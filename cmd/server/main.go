package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/api"
	"github.com/yegors/co-atc/internal/auth"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/frequencies"
	"github.com/yegors/co-atc/internal/reference"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/transcription"
	"github.com/yegors/co-atc/internal/transcription/phraseology"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// refAdapter wraps reference.Service to implement adsb.ReferenceService interface
type refAdapter struct {
	service *reference.Service
}

func (a *refAdapter) LookupAircraft(hex string) *adsb.ReferenceAircraftInfo {
	info := a.service.LookupAircraft(hex)
	if info == nil {
		return nil
	}
	return &adsb.ReferenceAircraftInfo{
		Hex:               info.Hex,
		Registration:      info.Registration,
		TypeCode:          info.TypeCode,
		ManufacturerModel: info.ManufacturerModel,
		Year:              info.Year,
		Owner:             info.Owner,
	}
}

func (a *refAdapter) LookupAirline(code string) string {
	return a.service.LookupAirline(code)
}

func (a *refAdapter) LookupAirlineCountry(code string) string {
	return a.service.LookupAirlineCountry(code)
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file (optional - will search in configs/ and root directory)")
	addUser := flag.String("add-user", "", "Print a configuration block for a new account, reading the password without echoing it")
	flag.Parse()

	if *addUser != "" {
		if err := printNewUser(*addUser); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Load configuration with fallback logic
	cfg, resolvedConfigPath, err := config.LoadWithFallbackAndPath(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	log, err := logger.New(logger.Config{
		Level:     cfg.Logging.Level,
		Format:    cfg.Logging.Format,
		File:      cfg.Logging.File,
		MaxSizeMB: cfg.Logging.MaxSizeMB,
		MaxFiles:  cfg.Logging.MaxFiles,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Co-ATC server",
		logger.String("version", "0.1.0"),
		logger.String("config_path", resolvedConfigPath),
	)
	// What the loader had to say about the file: a key written twice, a key
	// nothing reads any more. One line each; never a reason not to start.
	for _, w := range cfg.Warnings {
		log.Warn(w, logger.String("config_path", resolvedConfigPath))
	}

	// Arm the shutdown signals before anything is started, not after.
	//
	// They used to be armed once every service was up, which left a measured
	// 1.7 s window -- from spawning the STT sidecar to the end of startup --
	// where a signal took its default action and killed the server outright:
	// no deferred cleanup, and the sidecar orphaned. Arming first also lets a
	// long startup be interrupted: signalCtx is what the sidecar waits on, so
	// Ctrl-C during its health probe stops the wait and takes the child with it.
	//
	// SIGHUP is included because closing the terminal window sends it.
	signalCtx, stopSignals := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stopSignals()

	// Create ADS-B components
	adsbClient := adsb.NewClient(
		cfg.ADSB,
		cfg.Station.Latitude,
		cfg.Station.Longitude,
		time.Duration(cfg.Server.ReadTimeoutSecs)*time.Second,
		log,
	)

	probeCtx, probeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := adsbClient.ValidateSource(probeCtx); err != nil {
		probeCancel()
		fatalLog := log.WithOptions(zap.AddStacktrace(zapcore.PanicLevel))
		fatalLog.Fatal("ADS-B source validation failed", logger.Error(err), logger.String("source_type", cfg.ADSB.SourceType))
	}
	probeCancel()
	log.Info("ADS-B source validation succeeded", logger.String("source_type", cfg.ADSB.SourceType))

	// The other half of the product gets the same treatment. Without this the
	// server starts, serves the map and the audio, and transcribes nothing --
	// one error line per transmission and no transcript, which is a worse
	// failure than not starting at all.
	sttSidecar := transcription.NewSidecar(transcription.SidecarConfig{
		ServerURL:             cfg.Transcription.Local.ServerURL,
		Command:               cfg.Transcription.Local.Command,
		StartupTimeoutSeconds: cfg.Transcription.Local.StartupTimeoutSeconds,
	}, log)
	sidecarCtx, sidecarCancel := context.WithTimeout(signalCtx, 5*time.Minute)
	if err := sttSidecar.Start(sidecarCtx); err != nil {
		sidecarCancel()
		fatalLog := log.WithOptions(zap.AddStacktrace(zapcore.PanicLevel))
		fatalLog.Fatal("Local transcription sidecar unavailable", logger.Error(err))
	}
	sidecarCancel()
	// A sidecar that dies later is restarted, instead of leaving the server
	// transcribing nothing until someone notices (28/09).
	sttSidecar.Supervise(signalCtx)
	defer sttSidecar.Stop()

	// Create SQLite storage
	var adsbStorage adsb.Storage

	// Generate today's database filename
	today := time.Now().Format("2006-01-02")
	dbFilename := fmt.Sprintf("co-atc-%s.db", today)
	dbPath := filepath.Join(cfg.Storage.SQLiteBasePath, dbFilename)

	// Ensure the directory exists
	dbDir := cfg.Storage.SQLiteBasePath
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Error("Failed to create database directory", logger.Error(err), logger.String("path", dbDir))
		os.Exit(1)
	}

	// Values that can be changed while the server runs, from the settings panel.
	// Read before the first retention pass, so a size set from the panel is the
	// one that pass obeys, not the configured default.
	runtimeSettings := config.NewRuntime(cfg, *configPath, log)

	if cfg.Storage.DBRetentionDays > 0 {
		log.Warn("db_retention_days is no longer used: the daily databases are kept within db_retention_gb",
			logger.Int("db_retention_days", cfg.Storage.DBRetentionDays),
			logger.Float64("db_retention_gb", runtimeSettings.DBRetentionGB()))
	}
	if err := cleanupOldDailyDatabases(dbDir, dbPath, runtimeSettings.DBRetentionGB(), log); err != nil {
		log.Warn("Failed to clean up old database files",
			logger.Error(err),
			logger.String("path", dbDir),
			logger.Float64("retention_gb", runtimeSettings.DBRetentionGB()))
	}

	log.Info("Using daily database", logger.String("path", dbPath))

	// Create SQLite storage with no retention settings
	sqliteStorage, err := sqlite.NewAircraftStorage(
		dbPath,
		log,
	)
	if err != nil {
		log.Error("Failed to create SQLite storage", logger.Error(err))
		os.Exit(1)
	}
	defer sqliteStorage.Close()
	adsbStorage = sqliteStorage
	log.Info("Using SQLite storage", logger.String("path", dbPath))

	// Create transcription storage
	transcriptionStorage := sqlite.NewTranscriptionStorage(sqliteStorage.GetDB(), log)

	// Create clearance storage
	clearanceStorage := sqlite.NewClearanceStorage(sqliteStorage.GetDB(), log)

	// Create WebSocket server
	wsServer := websocket.NewServer(log)

	// Start WebSocket server
	go wsServer.Run()

	adsbService := adsb.NewService(
		adsbClient,
		adsbStorage,
		time.Duration(cfg.ADSB.FetchIntervalSecs)*time.Second,
		log,
		cfg.Station,
		cfg.ADSB,
		cfg.FlightPhases,
		wsServer,
	)

	// Load reference data (aircraft, airlines, airports, runways, navaids)
	refService, err := reference.NewService(reference.ServiceConfig{
		AircraftCSVPath:    cfg.Reference.AircraftCSVPath,
		AirlinesDATPath:    cfg.Reference.AirlinesDATPath,
		AirportsCSVPath:    cfg.Reference.AirportsCSVPath,
		FrequenciesCSVPath: cfg.Reference.FrequenciesCSVPath,
		RunwaysCSVPath:     cfg.Reference.RunwaysCSVPath,
		NavaidsCSVPath:     cfg.Reference.NavaidsCSVPath,
		StationLat:         cfg.Station.Latitude,
		StationLon:         cfg.Station.Longitude,
		HomeAirportCode:    cfg.Station.AirportCode,
		DisplayRangeNM:     cfg.Station.DisplayRangeNM,
		ExtensionLengthNM:  cfg.Station.RunwayExtensionLengthNM,
	}, log)
	// The reference airport: the one saved from the settings panel, else
	// [station] airport_code. Without reference data, phases keep being judged
	// from the receiver, as upstream did.
	var refAirport *referenceAirport
	effectiveAirport := cfg.Station.AirportCode
	if err != nil {
		log.Warn("Failed to load reference data", logger.Error(err))
	} else {
		adsbService.SetReferenceService(&refAdapter{service: refService})
		refAirport = &referenceAirport{ref: refService, adsb: adsbService, log: log}
		inForce := refAirport.start(runtimeSettings.Settings().ReferenceAirports(), cfg.Station.AirportCode)
		runtimeSettings.UseReferenceAirports(inForce)
		effectiveAirport = inForce[0]
		log.Info("Reference data loaded",
			logger.Int("aircraft_count", refService.AircraftCount()),
			logger.Int("airline_count", refService.AirlineCount()))
	}

	// Create and set WebSocket message handler for ADSB
	wsHandler := adsb.NewWebSocketHandler(adsbService, log)
	wsServer.SetMessageHandler(wsHandler)

	// Start ADS-B service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runDatabaseRetentionCleanup(ctx, dbDir, sqliteStorage.GetDB(), runtimeSettings, log)

	if err := adsbService.Start(ctx); err != nil {
		log.Error("Failed to start ADS-B service", logger.Error(err))
		os.Exit(1)
	}

	// Create weather service
	weatherConfigConverted := weather.ConfigWeatherConfig{
		RefreshIntervalMinutes: cfg.Weather.RefreshIntervalMinutes,
		APIBaseURL:             cfg.Weather.APIBaseURL,
		RequestTimeoutSeconds:  cfg.Weather.RequestTimeoutSeconds,
		MaxRetries:             cfg.Weather.MaxRetries,
		FetchMETAR:             cfg.Weather.FetchMETAR,
		FetchTAF:               cfg.Weather.FetchTAF,
		FetchNOTAMs:            cfg.Weather.FetchNOTAMs,
		CacheExpiryMinutes:     cfg.Weather.CacheExpiryMinutes,
	}
	weatherService := weather.NewService(weatherConfigConverted, effectiveAirport, log)

	// From here the panel can change the reference airport: the three services
	// that hold a piece of it exist.
	if refAirport != nil {
		refAirport.weather = weatherService
		runtimeSettings.SetReferenceAirportHook(config.ReferenceAirportHook{
			Validate: refAirport.validate,
			Apply:    refAirport.apply,
		})
	}

	// Start weather service
	if err := weatherService.Start(); err != nil {
		log.Error("Failed to start weather service", logger.Error(err))
		os.Exit(1)
	}

	// Create frequencies service.
	// The fleet adapter is what lets a spoken callsign be attached to a real
	// target.
	fleet := &adsbFleet{service: adsbService, lastSeenMinutes: cfg.PostProcessing.FleetLastSeenMinutes}

	// What the radio has said about each aircraft, carried on the aircraft itself
	// so the map can show which targets the controller is actually talking to.
	adsbService.SetVoiceIndex(newVoiceIndex(context.Background(), transcriptionStorage, 5*time.Second, log))
	frequenciesService := frequencies.NewService(cfg, log, wsServer, transcriptionStorage, clearanceStorage, fleet)
	// Each frequency's sector: its airport and kind, the sizes from the settings
	// panel, the airport's position from the reference data. Off unless the
	// panel says so, and for a frequency that names no airport.
	frequenciesService.SetSectors(func(id string) (phraseology.Sector, bool) {
		m := runtimeSettings.Matching()
		if !m.Sectors {
			return phraseology.Sector{}, false
		}
		code, kind := frequenciesService.SectorOf(id)
		size, known := m.Sector[kind]
		if code == "" || !known || refService == nil {
			return phraseology.Sector{}, false // no reference data: no airport to draw around
		}
		ap := refService.GetAirport(code)
		if ap == nil {
			return phraseology.Sector{}, false
		}
		return phraseology.Sector{Lat: ap.Latitude, Lon: ap.Longitude, RadiusNM: size.RadiusNM, MaxAltFt: size.MaxAltFt}, true
	})
	// The association rules from the settings panel, read for every transmission.
	frequenciesService.SetMatchingRules(func() phraseology.Rules {
		m := runtimeSettings.Matching()
		return phraseology.Rules{Letters: m.Letters, ApproxOperators: m.ApproxOperators,
			MinDigits: m.MinDigits, OneDigitOff: m.OneDigitOff,
			ContextLetters: m.ContextLetters, ContextDigits: m.ContextDigits, ContextNames: m.ContextNames,
			PrefixDigits: m.PrefixDigits}
	})

	// Start frequencies service
	if err := frequenciesService.Start(ctx); err != nil {
		log.Error("Failed to start frequencies service", logger.Error(err))
		os.Exit(1)
	}

	// Create API router
	router := api.NewRouter(adsbService, frequenciesService, weatherService, refService, cfg, log, wsServer, transcriptionStorage, clearanceStorage)
	router.Handler().AttachRuntime(runtimeSettings, sqliteStorage.GetDB(), sttSidecar)

	// --- Setup for multiple HTTP servers ---
	var servers []*http.Server
	allPorts := []int{cfg.Server.Port}       // Start with the primary port
	if len(cfg.Server.AdditionalPorts) > 0 { // Only append if there are additional ports
		allPorts = append(allPorts, cfg.Server.AdditionalPorts...)
	}

	// No account anywhere. On loopback that is a legitimate state and the setup
	// page handles it; bound to anything else it is a server about to serve the
	// receiver's data to whoever asks, which is precisely what upstream's README
	// warns against. Refusing to start is the only honest answer -- a warning
	// scrolls past and the server stays open.
	if router.Handler().Auth().NeedsSetup() {
		if !isLoopbackHost(cfg.Server.Host) {
			fatalLog := log.WithOptions(zap.AddStacktrace(zapcore.PanicLevel))
			fatalLog.Fatal("Refusing to start: no account exists and the server is not bound to loopback",
				logger.String("host", cfg.Server.Host),
				logger.String("fix", "set server.host to 127.0.0.1 and open the setup page, or create an account with: co-atc -add-user <name>"))
		}
		log.Warn("No account configured -- open the setup page to choose local-only or an account",
			logger.String("url", fmt.Sprintf("http://%s:%d/", cfg.Server.Host, cfg.Server.Port)))
	}

	log.Info("Configured listener ports", logger.Any("ports", allPorts))

	// Start a server for each configured port
	for _, port := range allPorts {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, port)
		server := &http.Server{
			Addr:         addr,
			Handler:      router.Routes(), // All servers use the same main router
			ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSecs) * time.Second,
			WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSecs) * time.Second,
			IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSecs) * time.Second,
		}
		servers = append(servers, server)

		go func(s *http.Server) {
			// TLS served by co-atc itself when a certificate is configured, and by
			// nothing when it is not. Behind a reverse proxy both stay empty and the
			// proxy terminates TLS -- neither shape is the privileged one.
			if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
				log.Info("Starting HTTPS server", logger.String("addr", s.Addr))
				if err := s.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil && err != http.ErrServerClosed {
					log.Error("HTTPS server error on startup", logger.String("addr", s.Addr), logger.Error(err))
				}
				return
			}
			log.Info("Starting HTTP server", logger.String("addr", s.Addr))
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("HTTP server error on startup", logger.String("addr", s.Addr), logger.Error(err))
				// If one server fails to start, log the error. Depending on requirements,
				// you might want to os.Exit(1) here or implement more complex error handling.
			}
		}(server)
	}

	// Wait for one of the signals armed at startup. A signal that arrived while
	// the services were still coming up is already recorded here.
	<-signalCtx.Done()

	log.Info("Shutting down server...")

	// Stop background services first
	log.Info("Stopping weather service...")
	weatherService.Stop()
	log.Info("Weather service stopped.")

	log.Info("Stopping frequencies service...")
	frequenciesService.Stop()
	log.Info("Frequencies service stopped.")

	// After the frequencies service: nothing will ask it to transcribe again.
	sttSidecar.Stop()

	// Stop any active transcription processors
	// This will be handled by the frequencies service when we integrate the transcription service

	log.Info("Stopping ADS-B service...")
	adsbService.Stop()
	log.Info("ADS-B service stopped.")

	// Cancel the main context
	cancel()

	// Shutdown all HTTP servers
	log.Info("Shutting down HTTP servers...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout slightly for multiple servers
	defer shutdownCancel()

	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(srv *http.Server) {
			defer wg.Done()
			log.Info("Attempting to shutdown HTTP server", logger.String("addr", srv.Addr))
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Error("HTTP server shutdown error", logger.String("addr", srv.Addr), logger.Error(err))
			} else {
				log.Info("HTTP server shutdown complete", logger.String("addr", srv.Addr))
			}
		}(s)
	}
	wg.Wait() // Wait for all server shutdowns to complete

	log.Info("All HTTP servers shutdown.")

	log.Info("Server fully stopped")
}

func runDatabaseRetentionCleanup(ctx context.Context, dbDir string, db *sqlite.DB, rt *config.Runtime, log *logger.Logger) {
	// Two cadences. Rotation is checked every minute so the switch lands close
	// to midnight; retention is an hourly sweep, which is as often as it can
	// possibly matter.
	rotateTicker := time.NewTicker(1 * time.Minute)
	defer rotateTicker.Stop()
	retentionTicker := time.NewTicker(1 * time.Hour)
	defer retentionTicker.Stop()

	sweep := func() {
		// Read on every pass, not captured once: a retention changed from the
		// settings panel takes effect on the next sweep rather than at the next
		// restart.
		capGB := rt.DBRetentionGB()
		if err := cleanupOldDailyDatabases(dbDir, db.Path(), capGB, log); err != nil {
			log.Warn("Periodic database retention cleanup failed",
				logger.Error(err),
				logger.String("path", dbDir),
				logger.Float64("retention_gb", capGB))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-rotateTicker.C:
			// The whole point of the loop. Before this, the file opened at
			// startup was written to for the life of the process and retention
			// skipped it as the active one, so a server left running simply grew
			// one database without limit.
			rotated, err := db.RotateIfNewDay(dbDir, time.Now())
			if err != nil {
				log.Error("Failed to rotate to today's database",
					logger.Error(err), logger.String("dir", dbDir))
				continue
			}
			if rotated {
				// Yesterday's file is closed now, so it is eligible for deletion
				// without waiting for the hourly sweep.
				sweep()
			}

		case <-retentionTicker.C:
			sweep()
		}
	}
}

// cleanupOldDailyDatabases keeps the daily databases within capGB together
// (28/09, D65). Newest first: the files that fit are kept, and from the first one
// that does not, it and every older file are deleted -- never a newer day for an
// older one. Today's file is never deleted, and counts. A file's -wal and -shm
// count with it and go with it.
func cleanupOldDailyDatabases(dbDir, activeDBPath string, capGB float64, log *logger.Logger) error {
	if capGB <= 0 {
		capGB = config.DefaultDBRetentionGB
	}
	capBytes := int64(capGB * (1 << 30))

	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return fmt.Errorf("read db directory: %w", err)
	}

	type daily struct {
		path  string
		date  time.Time
		bytes int64
	}
	// The same file can be spelled two ways: relative and absolute, or through a
	// symbolic link (/var is one on macOS, to /private/var). Compared by name
	// only, today's file passed for an old one there, and went when over the
	// cap. It is now recognised by name or by identity.
	absActiveDBPath, _ := filepath.Abs(activeDBPath)
	activeInfo, _ := os.Stat(activeDBPath)
	isActive := func(path string) bool {
		if abs, _ := filepath.Abs(path); abs == absActiveDBPath {
			return true
		}
		if activeInfo == nil {
			return false
		}
		info, err := os.Stat(path)
		return err == nil && os.SameFile(info, activeInfo)
	}
	var files []daily
	var total int64
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(fileName, "co-atc-") || !strings.HasSuffix(fileName, ".db") {
			continue
		}
		fileDate, parseErr := time.Parse("2006-01-02", strings.TrimSuffix(strings.TrimPrefix(fileName, "co-atc-"), ".db"))
		if parseErr != nil {
			continue
		}
		path := filepath.Join(dbDir, fileName)
		var size int64
		for _, p := range []string{path, path + "-wal", path + "-shm"} {
			if info, err := os.Stat(p); err == nil {
				size += info.Size()
			}
		}
		if isActive(path) {
			total += size // kept whatever its size
			continue
		}
		files = append(files, daily{path, fileDate, size})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].date.After(files[j].date) })

	deleted, over := 0, false
	var freed int64
	for _, f := range files {
		if !over && total+f.bytes <= capBytes {
			total += f.bytes
			continue
		}
		over = true
		for _, p := range []string{f.path, f.path + "-wal", f.path + "-shm"} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove old db '%s': %w", p, err)
			}
		}
		deleted++
		freed += f.bytes
		log.Info("Deleted old database file",
			logger.String("path", f.path),
			logger.String("file_date", f.date.Format("2006-01-02")),
			logger.Int64("bytes", f.bytes))
	}

	if deleted > 0 {
		log.Info("Database retention cleanup complete",
			logger.Int("deleted_files", deleted),
			logger.Int64("freed_bytes", freed),
			logger.Int64("kept_bytes", total),
			logger.Float64("retention_gb", capGB))
	}
	return nil
}

// printNewUser reads a password twice without echoing it and prints the TOML block
// to paste into the configuration.
//
// It prints rather than writes. Rewriting the configuration file would mean a
// program editing the document that carries every measured default and the reason
// for it; and a password that has been typed should reach exactly one place, the
// hash, with no copy left in a backup file along the way.
func printNewUser(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a user name is required")
	}

	first, err := readPassword("Password for " + name + ": ")
	if err != nil {
		return err
	}
	again, err := readPassword("Repeat: ")
	if err != nil {
		return err
	}
	if first != again {
		return fmt.Errorf("the two entries differ")
	}

	hash, err := auth.HashPassword(first)
	if err != nil {
		return err
	}

	fmt.Printf("\n# Add this to your configuration, under [auth]:\n\n")
	fmt.Printf("[auth]\nenabled = true\n\n[[auth.users]]\nname = %q\npassword_hash = %q\n\n", name, hash)
	fmt.Printf("# The password itself is not stored anywhere. Losing it means creating\n")
	fmt.Printf("# another account with this command; there is no recovery by design.\n")
	return nil
}

// stdinReader is shared across reads. A fresh bufio.Reader per call would lose
// whatever the previous one had already buffered, which makes the second prompt
// read EOF when the input is piped rather than typed.
var stdinReader = bufio.NewReader(os.Stdin)

var warnedAboutEcho bool

// readPassword turns off terminal echo through stty rather than pulling in a
// dependency for it. If echo cannot be turned off -- a pipe, a terminal that does
// not support it -- the caller is told plainly rather than typing a password into
// a visible line without knowing.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	stty := exec.Command("stty", "-echo")
	stty.Stdin = os.Stdin
	echoOff := stty.Run() == nil
	if !echoOff && !warnedAboutEcho {
		warnedAboutEcho = true
		fmt.Fprint(os.Stderr, "\n[!] this terminal will not hide input; type with that in mind\n"+prompt)
	}

	line, err := stdinReader.ReadString('\n')

	if echoOff {
		restore := exec.Command("stty", "echo")
		restore.Stdin = os.Stdin
		restore.Run()
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// isLoopbackHost reports whether a bind address can only be reached from this
// machine. An empty host means every interface, which is the one to get right.
func isLoopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}
