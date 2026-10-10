package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// main brings the server up in a fixed order, each step in startup.go and
// serve.go, waits for a signal and takes it down. The order is the one the
// comments below and in those steps date and justify; moving a step changes
// the server's behaviour.
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

	cfg, resolvedConfigPath := loadConfig(*configPath)

	// Create logger
	log := newLogger(cfg)
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
	adsbClient := buildADSBClient(cfg, log)

	sttSidecar := startSidecar(signalCtx, cfg, log)
	defer sttSidecar.Stop()

	// Create SQLite storage
	runtimeSettings, store := openStorage(cfg, *configPath, log)
	defer store.aircraft.Close()

	// Create WebSocket server
	wsServer := websocket.NewServer(log)

	// Start WebSocket server
	go wsServer.Run()

	adsbService := buildADSBService(cfg, adsbClient, store, wsServer, log)

	refService, refAirport, effectiveAirport := loadReference(cfg, runtimeSettings, adsbService, log)

	// Create and set WebSocket message handler for ADSB
	wsHandler := adsb.NewWebSocketHandler(adsbService, log)
	wsServer.SetMessageHandler(wsHandler)

	// Start ADS-B service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runDatabaseRetentionCleanup(ctx, store.dir, store.aircraft.GetDB(), runtimeSettings, log)

	if err := adsbService.Start(ctx); err != nil {
		log.Error("Failed to start ADS-B service", logger.Error(err))
		os.Exit(1)
	}

	weatherService := startWeather(cfg, effectiveAirport, refAirport, runtimeSettings, log)

	frequenciesService := startFrequencies(ctx, cfg, adsbService, refService, runtimeSettings, wsServer, store, log)

	router := buildRouter(cfg, adsbService, frequenciesService, weatherService, refService, wsServer, store, runtimeSettings, sttSidecar, log)

	servers := serve(cfg, router, log)

	// Wait for one of the signals armed at startup. A signal that arrived while
	// the services were still coming up is already recorded here.
	<-signalCtx.Done()

	shutdown(log, weatherService, frequenciesService, sttSidecar, adsbService, cancel, servers)
}
