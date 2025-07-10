package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/api"
	"github.com/yegors/co-atc/internal/atcchat"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/frequencies"
	"github.com/yegors/co-atc/internal/simulation"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/templating"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file (optional - will search in configs/ and root directory)")
	flag.Parse()

	// Load configuration with fallback logic
	cfg, err := config.LoadWithFallback(*configPath)
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
		Level:  cfg.Logging.Level,
		Format: "console", // Always use console format for better readability
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Co-ATC server",
		logger.String("version", "0.1.0"),
		logger.String("config_path", *configPath),
	)

	// Create ADS-B components
	adsbClient := adsb.NewClient(
		cfg.ADSB.SourceType,
		cfg.ADSB.LocalSourceURL,
		cfg.ADSB.ExternalSourceURL,
		cfg.ADSB.APIHost,
		cfg.ADSB.APIKey,
		cfg.Station.Latitude,
		cfg.Station.Longitude,
		float64(cfg.ADSB.SearchRadiusNM),
		time.Duration(cfg.Server.ReadTimeoutSecs)*time.Second,
		log,
	)
	// Processor has been moved into the service

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

	log.Info("Using daily database", logger.String("path", dbPath))

	// Create SQLite storage with no retention settings
	sqliteStorage, err := sqlite.NewAircraftStorage(
		dbPath,
		cfg.Storage.MaxPositionsInAPI,
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

	// Create simulation service
	simulationService := simulation.NewService(log)

	adsbService := adsb.NewService(
		adsbClient,
		adsbStorage,
		time.Duration(cfg.ADSB.FetchIntervalSecs)*time.Second,
		cfg.Storage.MaxPositionsInAPI,
		cfg.ADSB.AirlineDBPath,
		log,
		cfg.Station,
		cfg.ADSB,
		cfg.FlightPhases,
		wsServer,
		simulationService,
	)

	// Create and set WebSocket message handler for ADSB
	wsHandler := adsb.NewWebSocketHandler(adsbService, log)
	wsServer.SetMessageHandler(wsHandler)

	// Start ADS-B service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adsbService.Start(ctx); err != nil {
		log.Error("Failed to start ADS-B service", logger.Error(err))
		os.Exit(1)
	}

	// Create weather service first (needed for templating)
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
	weatherService := weather.NewService(weatherConfigConverted, cfg.Station.AirportCode, log)

	// Start weather service
	if err := weatherService.Start(); err != nil {
		log.Error("Failed to start weather service", logger.Error(err))
		os.Exit(1)
	}

	// Create templating service
	templateService := templating.NewService(
		adsbService,
		weatherService,
		transcriptionStorage,
		nil, // frequencies service not available yet
		cfg,
		log,
	)

	// Create frequencies service
	frequenciesService := frequencies.NewService(cfg, log, wsServer, transcriptionStorage, sqliteStorage, clearanceStorage, templateService)

	// Update templating service with frequencies service
	templateService = templating.NewService(
		adsbService,
		weatherService,
		transcriptionStorage,
		frequenciesService,
		cfg,
		log,
	)

	// Start frequencies service
	if err := frequenciesService.Start(ctx); err != nil {
		log.Error("Failed to start frequencies service", logger.Error(err))
		os.Exit(1)
	}

	// Create ATC Chat service (if enabled)
	var atcChatService *atcchat.Service
	if cfg.ATCChat.Enabled {
		log.Info("Creating ATC Chat service")
		atcChatService, err = atcchat.NewService(
			templateService,
			cfg,
			log,
		)
		if err != nil {
			log.Error("Failed to create ATC Chat service", logger.Error(err))
			// Continue without ATC Chat service rather than failing
			atcChatService = nil
		} else {
			log.Info("ATC Chat service created successfully")
		}
	} else {
		log.Info("ATC Chat service disabled in configuration")
	}

	// Create API router
	router := api.NewRouter(adsbService, frequenciesService, weatherService, atcChatService, simulationService, cfg, log, wsServer, transcriptionStorage, clearanceStorage)

	// --- Setup for multiple HTTP servers ---
	var servers []*http.Server
	allPorts := []int{cfg.Server.Port}       // Start with the primary port
	if len(cfg.Server.AdditionalPorts) > 0 { // Only append if there are additional ports
		allPorts = append(allPorts, cfg.Server.AdditionalPorts...)
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
			log.Info("Starting HTTP server", logger.String("addr", s.Addr))
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("HTTP server error on startup", logger.String("addr", s.Addr), logger.Error(err))
				// If one server fails to start, log the error. Depending on requirements,
				// you might want to os.Exit(1) here or implement more complex error handling.
			}
		}(server)
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Shutting down server...")

	// Stop background services first
	log.Info("Stopping weather service...")
	weatherService.Stop()
	log.Info("Weather service stopped.")

	log.Info("Stopping frequencies service...")
	frequenciesService.Stop()
	log.Info("Frequencies service stopped.")

	// Stop ATC Chat service if it was created
	if atcChatService != nil {
		log.Info("Stopping ATC Chat service...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := atcChatService.Shutdown(shutdownCtx); err != nil {
			log.Error("Error shutting down ATC Chat service", logger.Error(err))
		} else {
			log.Info("ATC Chat service stopped.")
		}
		shutdownCancel()
	}

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
