package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/atcchat"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/frequencies"
	"github.com/yegors/co-atc/internal/simulation"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// Router is the API router
type Router struct {
	handler    *Handler
	middleware *Middleware
	config     *config.Config
	logger     *logger.Logger
}

// NewRouter creates a new API router
func NewRouter(adsbService *adsb.Service, frequenciesService *frequencies.Service, weatherService *weather.Service, atcChatService *atcchat.Service, simulationService *simulation.Service, config *config.Config, logger *logger.Logger, wsServer *websocket.Server, transcriptionStorage *sqlite.TranscriptionStorage, clearanceStorage *sqlite.ClearanceStorage) *Router {
	return &Router{
		handler:    NewHandler(adsbService, frequenciesService, weatherService, atcChatService, simulationService, config, logger, wsServer, transcriptionStorage, clearanceStorage),
		middleware: NewMiddleware(logger),
		config:     config,
		logger:     logger.Named("api-router"),
	}
}

// Routes returns the API routes
func (r *Router) Routes() http.Handler {
	router := chi.NewRouter()

	// Middleware
	router.Use(r.middleware.RequestID)
	router.Use(r.middleware.Logger)
	router.Use(r.middleware.Recoverer)
	router.Use(r.middleware.CORS(r.config.Server.CORSAllowedOrigins))

	// API routes
	router.Route("/api/v1", func(router chi.Router) {
		// Aircraft routes
		router.Get("/aircraft", r.handler.GetAllAircraft)
		router.Get("/aircraft/{id}", r.handler.GetAircraftByHex)
		router.Get("/aircraft/{id}/tracks", r.handler.GetAircraftTracks)

		// Frequency routes
		router.Get("/frequencies", r.handler.GetAllFrequencies)
		router.Get("/frequencies/{id}", r.handler.GetFrequencyByID)

		// Audio stream route
		router.Get("/stream/{id}", r.handler.StreamAudio)
		router.Head("/stream/{id}", r.handler.StreamAudio) // Add support for HEAD requests

		// WebSocket route
		router.Get("/ws", r.handler.HandleWebSocket)

		// Transcription routes
		router.Get("/transcriptions", r.handler.GetAllTranscriptions)
		router.Get("/transcriptions/frequency/{id}", r.handler.GetTranscriptionsByFrequency)
		router.Get("/transcriptions/time-range", r.handler.GetTranscriptionsByTimeRange)
		router.Get("/transcriptions/speaker/{type}", r.handler.GetTranscriptionsBySpeaker)
		router.Get("/transcriptions/callsign/{callsign}", r.handler.GetTranscriptionsByCallsign)

		// Health check
		router.Get("/health", r.handler.GetHealth)

		// Configuration
		router.Get("/config", r.handler.GetConfig)

		// Station Configuration
		router.Get("/station", r.handler.GetStationConfig)    // New route for station config
		router.Post("/station", r.handler.SetStationOverride) // New route for station override

		// Weather Data
		router.Get("/wx", r.handler.GetWeatherData) // New route for weather data

		// ATC Chat routes
		router.Post("/atc-chat/session", r.handler.CreateATCChatSession)
		router.Delete("/atc-chat/session/{sessionId}", r.handler.EndATCChatSession)
		router.Get("/atc-chat/session/{sessionId}/status", r.handler.GetATCChatSessionStatus)
		router.Post("/atc-chat/session/{sessionId}/update-context", r.handler.UpdateATCChatSessionContext)
		router.Get("/atc-chat/sessions", r.handler.GetATCChatSessions)
		router.Get("/atc-chat/airspace-status", r.handler.GetATCChatAirspaceStatus)
		router.Get("/atc-chat/ws/{sessionId}", r.handler.HandleATCChatWebSocket)

		// Simulation routes
		router.Post("/simulation/aircraft", r.handler.CreateSimulatedAircraft)
		router.Put("/simulation/aircraft/{hex}/controls", r.handler.UpdateSimulationControls)
		router.Delete("/simulation/aircraft/{hex}", r.handler.RemoveSimulatedAircraft)
		router.Get("/simulation/aircraft", r.handler.GetSimulatedAircraft)
	})

	// Serve static files from the configured directory
	staticHandler := NewStaticFileHandler(r.config.Server.StaticFilesDir, r.logger)
	router.Handle("/*", staticHandler)

	return router
}
