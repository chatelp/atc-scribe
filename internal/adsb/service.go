package adsb

/*
FLIGHT PHASE DETECTION SYSTEM OVERVIEW
======================================

The Co-ATC system uses a unified landing/takeoff detection system that provides immediate,
accurate phase transitions for aircraft. This system is designed to solve the critical
timing issue where ground state changes (landing/takeoff) were detected several seconds
before the corresponding phase changes were recorded.

KEY CONCEPTS:

1. FLIGHT PHASES - The stages of an aircraft's journey:
   - NEW: Newly detected aircraft or parked/stationary on ground
   - TAX: Taxiing on ground (1-50 knots ground speed)
   - T/O: Takeoff phase (ground to air transition, preserved for 60 seconds)
   - DEP: Departure phase (climbing away from airport)
   - CRZ: Cruise phase (high altitude flight, typically above 10,000 ft)
   - ARR: Arrival phase (descending towards destination)
   - APP: Approach phase (aligned with runway, descending to land)
   - T/D: Touchdown/Landing phase (air to ground transition, preserved for 60 seconds)

2. PRIORITY-BASED DETECTION:
   The system uses a two-priority approach:
   - PRIORITY 1: Immediate ground state transitions (T/O and T/D)
     Detected instantly when OnGround state changes
   - PRIORITY 2: All other phase changes (TAX, DEP, CRZ, ARR, APP)
     Detected through normal phase detection logic

3. GROUND STATE DETERMINATION:
   Uses the IsFlying() function with configurable thresholds:
   - Minimum True Airspeed (TAS): 50 knots (configurable)
   - Minimum Altitude: 700 feet (configurable)
   - Special handling for helicopters and high-speed ground movements

4. SPECIAL FEATURES:
   - Hysteresis: Prevents rapid flapping between states
   - Signal Lost Landing Detection: Automatically marks aircraft as landed
     when signal is lost near airport at low altitude
   - Phase Preservation: T/O and T/D phases are preserved for 60 seconds
     to ensure they're visible in the UI
   - Sensor Data Validation: Corrects erroneous sensor readings

5. WEBSOCKET EVENTS:
   The system sends immediate WebSocket alerts for critical events:
   - "aircraft_event" with event="takeoff" for immediate takeoffs
   - "aircraft_event" with event="landing" for immediate landings
   - "phase_change" for all other phase transitions

This unified system ensures that the most critical aircraft events (takeoff/landing)
are detected and recorded with zero delay, providing accurate timestamps and immediate
user notifications.
*/

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// WebSocketServer defines the interface for a WebSocket server
type WebSocketServer interface {
	Broadcast(message *websocket.Message)
}

// Airline represents an airline from the airlines.json file
type Airline struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Alias    string `json:"alias"`
	IATA     string `json:"iata"`
	ICAO     string `json:"icao"`
	Callsign string `json:"callsign"`
	Country  string `json:"country"`
	Active   string `json:"active"`
}

// Storage defines the interface for aircraft data storage
type Storage interface {
	GetAll() []*Aircraft
	GetByHex(hex string) (*Aircraft, bool)
	GetFiltered(
		minAltitude, maxAltitude float64,
		status []string,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore *time.Time,
	) []*Aircraft
	Upsert(aircraft *Aircraft)
	Count() int
	GetAllPositionHistory(hex string) ([]Position, error)
	GetPositionHistoryWithLimit(hex string, limit int) ([]Position, error)

	// Phase change methods
	InsertPhaseChange(hex, flight, phase string, timestamp time.Time, adsbId *int) error
	GetPhaseHistory(hex string) ([]PhaseChange, error)
	GetCurrentPhase(hex string) (*PhaseChange, error)
	GetLatestTakeoffTime(hex string) (*time.Time, error)
	GetLatestLandingTime(hex string) (*time.Time, error)
	GetLatestADSBTargetID(hex string) (*int, error)

	// Batch phase change methods for performance optimization
	GetCurrentPhasesBatch(hexCodes []string) (map[string]*PhaseChange, error)
	GetLatestADSBTargetIDsBatch(hexCodes []string) (map[string]*int, error)
	InsertPhaseChangesBatch(changes []PhaseChangeInsert) error
}

// SimulationService defines the interface for simulation service
type SimulationService interface {
	UpdatePositions()
	GenerateADSBData() []ADSBTarget
	IsSimulated(hex string) bool
	GetAllAircraft() interface{}                // Returns simulation aircraft data
	GetAircraft(hex string) (interface{}, bool) // Returns specific simulated aircraft
}

// Service is the main service for ADS-B data processing
type Service struct {
	client             *Client
	storage            Storage
	fetchInterval      time.Duration
	maxPositionsInAPI  int // Maximum number of positions to return in the API response
	logger             *logger.Logger
	lastFetchTime      time.Time
	lastFetchStatus    bool
	mu                 sync.RWMutex
	stopCh             chan struct{}
	wg                 sync.WaitGroup
	airlineMap         map[string]string         // Map of ICAO code to airline name
	airlineDBPath      string                    // Path to airlines.json file
	stationLat         float64                   // Station latitude from config
	stationLon         float64                   // Station longitude from config
	stationElevFeet    float64                   // Station elevation in feet
	overrideLat        *float64                  // Override station latitude (nil = use config)
	overrideLon        *float64                  // Override station longitude (nil = use config)
	overrideMutex      sync.RWMutex              // Protect override coordinates
	wsServer           WebSocketServer           // WebSocket server for broadcasting events
	signalLostTimeout  time.Duration             // Time after which aircraft is marked as signal_lost
	runwayData         RunwayData                // Runway data for approach detection
	flightPhasesConfig config.FlightPhasesConfig // Flight phases configuration
	changeDetector     *ChangeDetector           // Tracks aircraft changes
	broadcastChan      chan []AircraftChange     // Channel for broadcasting changes
	simulationService  SimulationService         // Simulation service for simulated aircraft
}

// AircraftBulkResponse represents server response with bulk aircraft data
type AircraftBulkResponse struct {
	Aircraft []*Aircraft    `json:"aircraft"`
	Count    int            `json:"count"`
	Counts   AircraftCounts `json:"counts"`
}

// NewService creates a new ADS-B service
func NewService(
	client *Client,
	storage Storage,
	fetchInterval time.Duration,
	maxPositionsInAPI int,
	airlineDBPath string,
	logger *logger.Logger,
	stationCfg config.StationConfig,
	adsbCfg config.ADSBConfig,
	flightPhasesConfig config.FlightPhasesConfig,
	wsServer WebSocketServer,
	simulationService SimulationService,
) *Service {
	// Set default signal lost timeout if not configured
	signalLostTimeout := time.Duration(adsbCfg.SignalLostTimeoutSecs) * time.Second
	if signalLostTimeout == 0 {
		signalLostTimeout = 60 * time.Second // Default to 60 seconds
	}

	service := &Service{
		client:             client,
		storage:            storage,
		fetchInterval:      fetchInterval,
		maxPositionsInAPI:  maxPositionsInAPI,
		logger:             logger.Named("adsb"),
		stopCh:             make(chan struct{}),
		airlineMap:         make(map[string]string),
		airlineDBPath:      airlineDBPath,
		stationLat:         stationCfg.Latitude,
		stationLon:         stationCfg.Longitude,
		stationElevFeet:    float64(stationCfg.ElevationFeet),
		wsServer:           wsServer,
		signalLostTimeout:  signalLostTimeout,
		flightPhasesConfig: flightPhasesConfig,
		simulationService:  simulationService,
	}

	// CRITICAL FIX: Only enable WebSocket streaming if configured
	if adsbCfg.WebSocketAircraftUpdates {
		logger.Info("Aircraft streaming ENABLED - initializing WebSocket change detection")
		service.changeDetector = NewChangeDetector(logger)
		service.broadcastChan = make(chan []AircraftChange, 100)
		// Start broadcast worker
		service.startBroadcastWorker()
	} else {
		logger.Info("Aircraft streaming DISABLED - using HTTP polling only")
		// No change detection or broadcasting
		service.changeDetector = nil
		service.broadcastChan = nil
	}

	// Set the config for the prediction function
	predictionConfig := &Config{
		Station: struct {
			Latitude  float64
			Longitude float64
		}{
			Latitude:  stationCfg.Latitude,
			Longitude: stationCfg.Longitude,
		},
	}
	SetConfig(predictionConfig)

	// Load airline data
	if airlineDBPath != "" {
		if err := service.loadAirlineData(); err != nil {
			service.logger.Error("Failed to load airline data: " + err.Error())
		}
	}

	// Load runway data
	if stationCfg.RunwaysDBPath != "" {
		if err := service.loadRunwayData(stationCfg.RunwaysDBPath); err != nil {
			service.logger.Error("Failed to load runway data: " + err.Error())
		}
	}

	return service
}

// startBroadcastWorker starts the worker that broadcasts aircraft changes via WebSocket
func (s *Service) startBroadcastWorker() {
	go func() {
		for changes := range s.broadcastChan {
			for _, change := range changes {
				s.broadcastAircraftChange(change)
			}
		}
	}()
}

// broadcastAircraftChange broadcasts a single aircraft change via WebSocket
func (s *Service) broadcastAircraftChange(change AircraftChange) {
	var messageType string
	switch change.Type {
	case "added":
		messageType = "aircraft_added"
	case "updated":
		messageType = "aircraft_update"
	case "removed":
		messageType = "aircraft_removed"
	}

	data := map[string]interface{}{
		"type": change.Type,
		"hex":  change.Hex,
	}

	if change.Aircraft != nil {
		data["aircraft"] = change.Aircraft
	}

	// Removed "changes" field - we now always send full aircraft data
	// This aligns WebSocket payloads with HTTP API responses

	message := &websocket.Message{
		Type: messageType,
		Data: data,
	}

	if s.wsServer != nil {
		s.wsServer.Broadcast(message)
	}
}

// loadAirlineData loads airline data from the airlines.json file
func (s *Service) loadAirlineData() error {
	s.logger.Info("Loading airline data from: " + s.airlineDBPath)

	// Read the file
	data, err := os.ReadFile(s.airlineDBPath)
	if err != nil {
		return err
	}

	// Parse the JSON
	var airlines []Airline
	if err := json.Unmarshal(data, &airlines); err != nil {
		return err
	}

	// Create the mapping
	for _, airline := range airlines {
		// Map ICAO code to airline name
		if airline.ICAO != "" && airline.ICAO != "N/A" {
			s.airlineMap[airline.ICAO] = airline.Name
		}

		// Also map IATA code to airline name if available
		// This handles callsigns that use IATA codes (e.g., AA123 instead of AAL123)
		if airline.IATA != "" && airline.IATA != "-" && airline.IATA != "N/A" {
			s.airlineMap[airline.IATA] = airline.Name
		}
	}

	// Airline map is now used directly in ProcessRawData

	// Print the map size for debugging
	s.logger.Info("Airline map loaded", logger.Int("count", len(s.airlineMap)))

	s.logger.Info("Loaded airline data",
		logger.Int("count", len(s.airlineMap)))
	return nil
}

// loadRunwayData loads runway data from the runways.json file
func (s *Service) loadRunwayData(runwayDBPath string) error {
	s.logger.Info("Loading runway data from: " + runwayDBPath)

	// Read the file
	data, err := os.ReadFile(runwayDBPath)
	if err != nil {
		return err
	}

	// Parse the JSON
	if err := json.Unmarshal(data, &s.runwayData); err != nil {
		return err
	}

	s.logger.Info("Loaded runway data",
		logger.String("airport", s.runwayData.Airport),
		logger.Int("runway_count", len(s.runwayData.RunwayThresholds)))
	return nil
}

// sendPhaseChangeAlert sends a phase change alert via WebSocket
func (s *Service) sendPhaseChangeAlert(aircraft *Aircraft, fromPhase, toPhase string, runwayInfo *RunwayApproachInfo) {
	s.sendPhaseChangeAlertWithEvent(aircraft, fromPhase, toPhase, "phase_change", runwayInfo)
}

// sendPhaseChangeAlertWithEvent sends a phase change alert with event type via WebSocket
func (s *Service) sendPhaseChangeAlertWithEvent(aircraft *Aircraft, fromPhase, toPhase, eventType string, runwayInfo *RunwayApproachInfo) {
	if s.wsServer != nil {
		alert := PhaseChangeAlert{
			Type:      "phase_change",
			Hex:       aircraft.Hex,
			Flight:    aircraft.Flight,
			FromPhase: fromPhase,
			ToPhase:   toPhase,
			EventType: eventType,
			Timestamp: time.Now().UTC(),
			Location: struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
				Alt float64 `json:"alt"`
			}{
				Lat: aircraft.ADSB.Lat,
				Lon: aircraft.ADSB.Lon,
				Alt: aircraft.ADSB.AltBaro,
			},
			RunwayInfo: runwayInfo,
		}

		s.wsServer.Broadcast(&websocket.Message{
			Type: "phase_change",
			Data: map[string]interface{}{
				"alert": alert,
			},
		})

		s.logger.Info("Phase change alert sent",
			logger.String("hex", aircraft.Hex),
			logger.String("flight", aircraft.Flight),
			logger.String("transition", fromPhase+" → "+toPhase),
			logger.String("event_type", eventType),
			logger.Float64("altitude", aircraft.ADSB.AltBaro),
			logger.Bool("on_ground", aircraft.OnGround),
		)
	}
}

// Start starts the ADS-B service
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Starting ADS-B service",
		logger.Duration("fetch_interval", s.fetchInterval),
	)

	// Initial fetch
	if err := s.fetchAndProcess(ctx); err != nil {
		s.logger.Error("Failed to fetch initial ADS-B data", logger.Error(err))
		s.setFetchStatus(false)
	} else {
		s.setFetchStatus(true)
	}

	// Start background fetching
	s.wg.Add(1)
	go s.fetchLoop(ctx)

	return nil
}

// Stop stops the ADS-B service
func (s *Service) Stop() {
	s.logger.Info("Stopping ADS-B service")
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("ADS-B service stopped")
}

// fetchLoop periodically fetches and processes ADS-B data
func (s *Service) fetchLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.fetchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.fetchAndProcess(ctx); err != nil {
				s.logger.Error("Failed to fetch ADS-B data", logger.Error(err))
				s.setFetchStatus(false)
			} else {
				s.setFetchStatus(true)
			}
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// fetchAndProcess fetches and processes ADS-B data
func (s *Service) fetchAndProcess(ctx context.Context) error {
	// Fetch raw data
	rawData, err := s.client.FetchData(ctx)
	if err != nil {
		return err
	}

	// Update simulated aircraft positions and inject simulated data
	if s.simulationService != nil {
		s.simulationService.UpdatePositions()
		simulatedTargets := s.simulationService.GenerateADSBData()

		// Append simulated aircraft to raw data
		rawData.Aircraft = append(rawData.Aircraft, simulatedTargets...)

		s.logger.Debug("Injected simulated aircraft into ADSB data",
			logger.Int("count", len(simulatedTargets)))
	}

	// Process raw data (now includes simulated aircraft)
	newAircraft := s.ProcessRawData(rawData)

	// Create a map of active aircraft hex codes
	activeAircraft := make(map[string]bool)
	for _, a := range newAircraft {
		activeAircraft[a.Hex] = true
	}

	// Process each aircraft for ground state determination and takeoff/landing detection
	for _, a := range newAircraft {
		// Get previous state from database for sensor validation
		existingAircraft, found := s.storage.GetByHex(a.Hex)

		var prevTAS, prevGS, prevAlt float64
		if found && existingAircraft.ADSB != nil {
			prevTAS = existingAircraft.ADSB.TAS
			prevGS = existingAircraft.ADSB.GS
			prevAlt = existingAircraft.ADSB.AltBaro
		}

		// Validate and correct sensor data for potential errors
		correctedTAS, correctedGS, correctedAlt := ValidateSensorData(
			a.ADSB.TAS, a.ADSB.GS, a.ADSB.AltBaro,
			prevTAS, prevGS, prevAlt,
			a.ADSB.Lat, a.ADSB.Lon, s.stationLat, s.stationLon,
			s.flightPhasesConfig.AirportRangeNM,
			&s.flightPhasesConfig,
		)

		// Determine if aircraft is currently flying using corrected values and config
		currentlyFlying := IsFlying(correctedTAS, correctedGS, correctedAlt, &s.flightPhasesConfig)

		// Always set on_ground based on flying state
		a.OnGround = !currentlyFlying

		if found {
			// Get the latest takeoff and landing times from phase_changes table
			takeoffTime, _ := s.storage.GetLatestTakeoffTime(a.Hex)
			landingTime, _ := s.storage.GetLatestLandingTime(a.Hex)

			a.DateTookoff = takeoffTime
			a.DateLanded = landingTime

			// Flying state detection moved to after phase determination

			// Log sensor corrections if they occurred
			if correctedTAS != a.ADSB.TAS || correctedGS != a.ADSB.GS || correctedAlt != a.ADSB.AltBaro {
				s.logger.Debug("Sensor data corrected for flying determination",
					logger.String("hex", a.Hex),
					logger.String("flight", a.Flight),
					logger.Float64("original_tas", a.ADSB.TAS),
					logger.Float64("corrected_tas", correctedTAS),
					logger.Float64("original_gs", a.ADSB.GS),
					logger.Float64("corrected_gs", correctedGS),
					logger.Float64("original_alt", a.ADSB.AltBaro),
					logger.Float64("corrected_alt", correctedAlt),
				)
			}

			// Flying state detection moved to after phase determination
		}

		// DO NOT update the aircraft in the database yet - we need to detect ground state transitions first
		// s.storage.Upsert(a) -- MOVED TO AFTER GROUND STATE DETECTION

		// Populate phase data for the aircraft
		phaseHistory, err := s.storage.GetPhaseHistory(a.Hex)
		if err == nil && len(phaseHistory) > 0 {
			// Create phase data structure
			a.Phase = &PhaseData{
				Current: []PhaseChange{phaseHistory[0]},
				History: phaseHistory,
			}
		}
	}

	// PRIORITY 1: Handle immediate ground state transitions (takeoff/landing)
	immediatePhaseChanges := s.detectGroundStateTransitions(newAircraft)
	if len(immediatePhaseChanges) > 0 {
		err := s.storage.InsertPhaseChangesBatch(immediatePhaseChanges)
		if err != nil {
			s.logger.Error("Failed to insert immediate ground transition phases", logger.Error(err))
		} else {
			// Send immediate alerts for takeoff/landing events
			s.sendImmediateGroundTransitionAlerts(immediatePhaseChanges)

			// IMPORTANT: Update the phase data for aircraft that just had transitions
			// This ensures the Phase, DateTookoff, and DateLanded fields are populated
			for _, change := range immediatePhaseChanges {
				// Find the aircraft in our newAircraft slice
				for _, a := range newAircraft {
					if a.Hex == change.Hex {
						// Get the updated phase data from storage
						phaseHistory, err := s.storage.GetPhaseHistory(a.Hex)
						if err == nil && len(phaseHistory) > 0 {
							// Create phase data structure
							a.Phase = &PhaseData{
								Current: []PhaseChange{phaseHistory[0]},
								History: phaseHistory,
							}

							// Update takeoff/landing times based on phase
							if change.Phase == "T/O" {
								takeoffTime := change.Timestamp
								a.DateTookoff = &takeoffTime
							} else if change.Phase == "T/D" {
								landingTime := change.Timestamp
								a.DateLanded = &landingTime
							}

							// Update the aircraft in storage with the new phase data
							// s.storage.Upsert(a) -- MOVED TO AFTER ALL PROCESSING
						}
						break
					}
				}
			}
		}
	}

	// NOW update all aircraft in the database after ground state transitions have been detected
	for _, a := range newAircraft {
		s.storage.Upsert(a)
	}

	// Update status of existing aircraft that are no longer active
	s.updateAircraftStatus(activeAircraft)

	// PRIORITY 2: Handle all other phase changes (normal phase detection)
	s.processPhaseChangesBatch(newAircraft, immediatePhaseChanges)

	s.setLastFetchTime(time.Now().UTC()) // Use UTC for last fetch time

	// CRITICAL FIX: Only detect and broadcast changes if WebSocket streaming is enabled
	if s.changeDetector != nil && s.broadcastChan != nil {
		allAircraft := s.GetAllAircraft()
		changes := s.changeDetector.DetectChanges(allAircraft)

		if len(changes) > 0 {
			s.logger.Debug("Detected aircraft changes",
				logger.Int("change_count", len(changes)))

			select {
			case s.broadcastChan <- changes:
			default:
				s.logger.Warn("Broadcast channel full, dropping changes")
			}
		}
	}

	s.logger.Debug("Updated aircraft data",
		logger.Int("count", len(newAircraft)),
		logger.Int("total", s.storage.Count()),
	)

	return nil
}

// updateSimulationFields updates the IsSimulated field and simulation controls for aircraft
func (s *Service) updateSimulationFields(aircraft []*Aircraft) {
	for _, a := range aircraft {
		if a.ADSB != nil {
			a.IsSimulated = (s.simulationService != nil && s.simulationService.IsSimulated(a.Hex)) || a.ADSB.Type == "sim"

			// Update simulation controls if this is a simulated aircraft
			if a.IsSimulated && a.SimulationControls == nil {
				a.SimulationControls = &SimulationControls{
					TargetHeading:      a.ADSB.TrueHeading,
					TargetSpeed:        a.ADSB.TAS,
					TargetVerticalRate: a.ADSB.BaroRate,
				}
			}
		}
	}
}

// GetAllAircraft returns all aircraft
func (s *Service) GetAllAircraft() []*Aircraft {
	aircraft := s.storage.GetAll()
	s.updateSimulationFields(aircraft)
	return aircraft
}

// GetAircraftByHex returns an aircraft by its hex ID
func (s *Service) GetAircraftByHex(hex string) (*Aircraft, bool) {
	aircraft, found := s.storage.GetByHex(hex)
	if found && aircraft != nil {
		s.updateSimulationFields([]*Aircraft{aircraft})
	}
	return aircraft, found
}

// GetAllPositionHistory returns all position history for an aircraft
func (s *Service) GetAllPositionHistory(hex string) ([]Position, error) {
	return s.storage.GetAllPositionHistory(hex)
}

// GetPositionHistoryWithLimit returns position history for an aircraft with a specified limit
func (s *Service) GetPositionHistoryWithLimit(hex string, limit int) ([]Position, error) {
	return s.storage.GetPositionHistoryWithLimit(hex, limit)
}

// GetFilteredAircraft returns aircraft filtered by altitude, status, and date ranges
func (s *Service) GetFilteredAircraft(
	minAltitude, maxAltitude float64,
	status []string,
	tookOffAfter, tookOffBefore, landedAfter, landedBefore *time.Time,
) []*Aircraft {
	aircraft := s.storage.GetFiltered(
		minAltitude, maxAltitude,
		status,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore,
	)
	s.updateSimulationFields(aircraft)
	return aircraft
}

// GetFilteredAircraftSimple is a simplified version for backward compatibility
func (s *Service) GetFilteredAircraftSimple(minAltitude, maxAltitude float64, status ...string) []*Aircraft {
	aircraft := s.storage.GetFiltered(minAltitude, maxAltitude, status, nil, nil, nil, nil)
	s.updateSimulationFields(aircraft)
	return aircraft
}

// HandleBulkRequest processes client requests for bulk aircraft data
func (s *Service) HandleBulkRequest(filters map[string]interface{}) (*AircraftBulkResponse, error) {
	// Parse filters from the request
	minAltitude := 0.0
	maxAltitude := 60000.0
	var status []string
	lastSeenMinutes := 0
	excludeOtherAirportsGrounded := false
	showAir := true
	showGround := true
	var phases []string

	// Extract filters
	if val, ok := filters["min_altitude"].(float64); ok {
		minAltitude = val
	}
	if val, ok := filters["max_altitude"].(float64); ok {
		maxAltitude = val
	}
	if val, ok := filters["status"].([]interface{}); ok {
		for _, s := range val {
			if str, ok := s.(string); ok {
				status = append(status, str)
			}
		}
	}
	if val, ok := filters["last_seen_minutes"].(float64); ok {
		lastSeenMinutes = int(val)
	}
	if val, ok := filters["exclude_other_airports_grounded"].(bool); ok {
		excludeOtherAirportsGrounded = val
	}
	if val, ok := filters["show_air"].(bool); ok {
		showAir = val
	}
	if val, ok := filters["show_ground"].(bool); ok {
		showGround = val
	}
	if val, ok := filters["phases"].([]interface{}); ok {
		for _, p := range val {
			if str, ok := p.(string); ok {
				phases = append(phases, str)
			}
		}
	}

	// If both Air and Ground are disabled, return empty result
	if !showAir && !showGround {
		return &AircraftBulkResponse{
			Aircraft: []*Aircraft{},
			Count:    0,
			Counts: AircraftCounts{
				GroundActive: 0,
				GroundTotal:  0,
				AirActive:    0,
				AirTotal:     0,
			},
		}, nil
	}

	// Get filtered aircraft using existing filtering logic
	var aircraft []*Aircraft
	if minAltitude > 0 || maxAltitude < 60000 || len(status) > 0 {
		aircraft = s.GetFilteredAircraft(minAltitude, maxAltitude, status, nil, nil, nil, nil)
	} else {
		aircraft = s.GetAllAircraft()
	}

	// Apply additional filters
	if lastSeenMinutes > 0 {
		aircraft = s.filterByLastSeen(aircraft, lastSeenMinutes)
	}

	if excludeOtherAirportsGrounded {
		aircraft = s.filterByAirportGrounded(aircraft)
	}

	// Apply Air/Ground and Phase filters
	aircraft = s.filterByAirGroundAndPhases(aircraft, showAir, showGround, phases)

	// Calculate counts
	groundActive, groundTotal, airActive, airTotal := s.calculateCounts(aircraft)

	return &AircraftBulkResponse{
		Aircraft: aircraft,
		Count:    len(aircraft),
		Counts: AircraftCounts{
			GroundActive: groundActive,
			GroundTotal:  groundTotal,
			AirActive:    airActive,
			AirTotal:     airTotal,
		},
	}, nil
}

func (s *Service) filterByLastSeen(aircraft []*Aircraft, minutes int) []*Aircraft {
	cutoffTime := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	filtered := make([]*Aircraft, 0)
	for _, a := range aircraft {
		if a.LastSeen.After(cutoffTime) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// filterByAirGroundAndPhases filters aircraft based on Air/Ground scope and phases
func (s *Service) filterByAirGroundAndPhases(aircraft []*Aircraft, showAir, showGround bool, phases []string) []*Aircraft {
	filtered := make([]*Aircraft, 0)

	for _, a := range aircraft {
		// Apply Air/Ground filter
		if a.OnGround && !showGround {
			continue
		}
		if !a.OnGround && !showAir {
			continue
		}

		// Apply phase filter if phases are specified
		if len(phases) > 0 {
			phaseMatch := false
			if a.Phase != nil && len(a.Phase.Current) > 0 {
				currentPhase := a.Phase.Current[0].Phase
				for _, phase := range phases {
					if currentPhase == phase {
						phaseMatch = true
						break
					}
				}
			}
			if !phaseMatch {
				continue
			}
		}

		filtered = append(filtered, a)
	}

	return filtered
}

func (s *Service) filterByAirportGrounded(aircraft []*Aircraft) []*Aircraft {
	filtered := make([]*Aircraft, 0)
	airportRangeNM := 5.0 // Default range, should come from config

	for _, a := range aircraft {
		if !a.OnGround {
			filtered = append(filtered, a)
		} else if a.ADSB != nil && a.ADSB.Lat != 0 && a.ADSB.Lon != 0 {
			// Calculate distance from station and apply filter
			distMeters := Haversine(a.ADSB.Lat, a.ADSB.Lon, s.stationLat, s.stationLon)
			distNM := MetersToNM(distMeters)
			if distNM <= airportRangeNM {
				filtered = append(filtered, a)
			}
		}
	}
	return filtered
}

func (s *Service) calculateCounts(aircraft []*Aircraft) (int, int, int, int) {
	groundActive, groundTotal, airActive, airTotal := 0, 0, 0, 0

	for _, a := range aircraft {
		if a.OnGround {
			groundTotal++
			if a.Status == "active" {
				groundActive++
			}
		} else {
			airTotal++
			if a.Status == "active" {
				airActive++
			}
		}
	}

	return groundActive, groundTotal, airActive, airTotal
}

// GetStatus returns the service status
func (s *Service) GetStatus() (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastFetchTime, s.lastFetchStatus
}

// setLastFetchTime sets the last fetch time
func (s *Service) setLastFetchTime(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFetchTime = t
}

// setFetchStatus sets the fetch status
func (s *Service) setFetchStatus(status bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFetchStatus = status
}

// SetStationOverride sets override coordinates for station location
func (s *Service) SetStationOverride(lat, lon float64) {
	s.overrideMutex.Lock()
	defer s.overrideMutex.Unlock()

	s.overrideLat = &lat
	s.overrideLon = &lon

	// Update the client with new coordinates
	if s.client != nil {
		s.client.UpdateStationCoords(lat, lon)
	}

	s.logger.Info("Station override coordinates set",
		logger.Float64("latitude", lat),
		logger.Float64("longitude", lon))
}

// ClearStationOverride removes override coordinates, reverting to config values
func (s *Service) ClearStationOverride() {
	s.overrideMutex.Lock()
	defer s.overrideMutex.Unlock()

	s.overrideLat = nil
	s.overrideLon = nil

	// Restore client to original config coordinates
	if s.client != nil {
		s.client.UpdateStationCoords(s.stationLat, s.stationLon)
	}

	s.logger.Info("Station override coordinates cleared, using config values",
		logger.Float64("config_latitude", s.stationLat),
		logger.Float64("config_longitude", s.stationLon))
}

// GetEffectiveStationCoords returns the current effective station coordinates (override or config)
func (s *Service) GetEffectiveStationCoords() (lat, lon float64) {
	s.overrideMutex.RLock()
	defer s.overrideMutex.RUnlock()

	if s.overrideLat != nil && s.overrideLon != nil {
		return *s.overrideLat, *s.overrideLon
	}

	return s.stationLat, s.stationLon
}

// updateAircraftStatus updates the status of aircraft that are no longer active
func (s *Service) updateAircraftStatus(activeAircraft map[string]bool) {
	// Get all current aircraft
	allAircraft := s.storage.GetAll()
	now := time.Now().UTC() // Use UTC for current time

	var inactiveAircraft []*Aircraft

	for _, aircraft := range allAircraft {
		// Skip aircraft with no position data
		if aircraft.ADSB == nil || (aircraft.ADSB.Lat == 0 && aircraft.ADSB.Lon == 0) {
			continue
		}

		// Skip active aircraft (they're handled in fetchAndProcess)
		if activeAircraft[aircraft.Hex] {
			continue
		}

		// Handle inactive aircraft status updates
		timeSinceLastSeen := now.Sub(aircraft.LastSeen)
		newStatus := aircraft.Status // Default to current status

		// Apply new status logic:
		// - If last seen > configured timeout: signal_lost
		if timeSinceLastSeen > s.signalLostTimeout {
			newStatus = "signal_lost"
			inactiveAircraft = append(inactiveAircraft, aircraft)
		}

		// Only update if status changed
		if aircraft.Status != newStatus {
			aircraft.Status = newStatus
			s.storage.Upsert(aircraft)

			s.logger.Info("Aircraft status updated",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("new_status", aircraft.Status),
				logger.Bool("on_ground", aircraft.OnGround),
				logger.Duration("time_since_last_seen", timeSinceLastSeen),
			)

			// Send WebSocket message for status change event
			if s.wsServer != nil {
				// For signal_lost status, only send WebSocket message if aircraft is NOT on the ground
				// For other status changes, always send the message
				if newStatus != "signal_lost" || !aircraft.OnGround {
					// Create message data
					data := map[string]interface{}{
						"hex":                  aircraft.Hex,
						"flight":               aircraft.Flight,
						"new_status":           newStatus,
						"on_ground":            aircraft.OnGround,
						"time_since_last_seen": timeSinceLastSeen.Seconds(),
						"timestamp":            now.Format(time.RFC3339),
					}

					// Broadcast the message
					s.wsServer.Broadcast(&websocket.Message{
						Type: "status_update",
						Data: data,
					})
				} else {
					// Log that we're skipping the WebSocket message for a grounded aircraft
					s.logger.Debug("Skipping signal_lost WebSocket message for grounded aircraft",
						logger.String("hex", aircraft.Hex),
						logger.String("flight", aircraft.Flight),
						logger.Bool("on_ground", aircraft.OnGround),
					)
				}
			}
		}
	}

	// Check for signal lost landings
	landingPhaseChanges := s.detectSignalLostLandings(inactiveAircraft)
	if len(landingPhaseChanges) > 0 {
		err := s.storage.InsertPhaseChangesBatch(landingPhaseChanges)
		if err != nil {
			s.logger.Error("Failed to insert signal lost landing phases", logger.Error(err))
		} else {
			s.sendImmediateGroundTransitionAlerts(landingPhaseChanges)
		}
	}
}

// detectGroundStateTransitions detects immediate takeoff/landing events
func (s *Service) detectGroundStateTransitions(aircraft []*Aircraft) []PhaseChangeInsert {
	var immediatePhaseChanges []PhaseChangeInsert
	now := time.Now().UTC()

	for _, a := range aircraft {
		// Get previous state from database
		existingAircraft, found := s.storage.GetByHex(a.Hex)
		if !found {
			continue // New aircraft - will be handled by normal phase detection
		}

		// Check if ground state changed
		if existingAircraft.OnGround != a.OnGround {
			var newPhase string
			var eventType string

			// Get current phase to prevent rapid T/O ↔ T/D flapping
			currentPhase, err := s.storage.GetCurrentPhase(a.Hex)

			if !existingAircraft.OnGround && a.OnGround {
				// Aircraft was airborne, now on ground = LANDING

				// Anti-flapping: Prevent T/O → T/D transition if T/O was recent
				if err == nil && currentPhase != nil && currentPhase.Phase == "T/O" {
					timeSinceTakeoff := time.Since(currentPhase.Timestamp).Seconds()
					flappingThreshold := float64(s.flightPhasesConfig.PhaseFlappingPreventionSeconds)
					if timeSinceTakeoff < flappingThreshold {
						s.logger.Warn("Preventing rapid T/O → T/D flapping",
							logger.String("hex", a.Hex),
							logger.String("flight", a.Flight),
							logger.Float64("time_since_takeoff", timeSinceTakeoff),
							logger.Float64("threshold_seconds", flappingThreshold),
							logger.Float64("altitude", a.ADSB.AltBaro),
						)
						continue // Skip this transition
					}
				}

				newPhase = "T/D"
				eventType = "landing"

				s.logger.Info("IMMEDIATE LANDING DETECTED",
					logger.String("hex", a.Hex),
					logger.String("flight", a.Flight),
					logger.Bool("was_on_ground", existingAircraft.OnGround),
					logger.Bool("now_on_ground", a.OnGround),
					logger.Float64("altitude", a.ADSB.AltBaro),
					logger.Float64("ground_speed", a.ADSB.GS),
				)

			} else if existingAircraft.OnGround && !a.OnGround {
				// Aircraft was on ground, now airborne = TAKEOFF

				// Anti-flapping: Prevent T/D → T/O transition if T/D was recent
				if err == nil && currentPhase != nil && currentPhase.Phase == "T/D" {
					timeSinceLanding := time.Since(currentPhase.Timestamp).Seconds()
					flappingThreshold := float64(s.flightPhasesConfig.PhaseFlappingPreventionSeconds)
					if timeSinceLanding < flappingThreshold {
						s.logger.Warn("Preventing rapid T/D → T/O flapping",
							logger.String("hex", a.Hex),
							logger.String("flight", a.Flight),
							logger.Float64("time_since_landing", timeSinceLanding),
							logger.Float64("threshold_seconds", flappingThreshold),
							logger.Float64("altitude", a.ADSB.AltBaro),
						)
						continue // Skip this transition
					}
				}

				newPhase = "T/O"
				eventType = "takeoff"

				s.logger.Info("IMMEDIATE TAKEOFF DETECTED",
					logger.String("hex", a.Hex),
					logger.String("flight", a.Flight),
					logger.Bool("was_on_ground", existingAircraft.OnGround),
					logger.Bool("now_on_ground", a.OnGround),
					logger.Float64("altitude", a.ADSB.AltBaro),
					logger.Float64("ground_speed", a.ADSB.GS),
				)
			}

			if newPhase != "" {
				// Get ADSB target ID for this aircraft
				adsbId, _ := s.storage.GetLatestADSBTargetID(a.Hex)

				immediatePhaseChanges = append(immediatePhaseChanges, PhaseChangeInsert{
					Hex:       a.Hex,
					Flight:    a.Flight,
					Phase:     newPhase,
					Timestamp: now,
					ADSBId:    adsbId,
					EventType: eventType, // New field to track the type of transition
				})
			}
		}
	}

	return immediatePhaseChanges
}

// detectSignalLostLandings checks for aircraft that lost signal near the airport
// and marks them as landed if they meet certain criteria
func (s *Service) detectSignalLostLandings(inactiveAircraft []*Aircraft) []PhaseChangeInsert {
	var landingPhaseChanges []PhaseChangeInsert

	// Check if signal lost landing detection is enabled
	if !s.flightPhasesConfig.SignalLostLandingEnabled {
		return landingPhaseChanges
	}

	now := time.Now().UTC()

	for _, aircraft := range inactiveAircraft {
		// Skip if already on ground or no ADSB data
		if aircraft.OnGround || aircraft.ADSB == nil {
			continue
		}

		// Check if aircraft was in approach phase
		currentPhase, err := s.storage.GetCurrentPhase(aircraft.Hex)
		if err != nil || currentPhase == nil {
			continue
		}

		// Only consider aircraft that were in APP phase or low altitude ARR
		if currentPhase.Phase != "APP" &&
			!(currentPhase.Phase == "ARR" && aircraft.ADSB.AltBaro < 2000) {
			continue
		}

		// Check proximity to airport
		distanceFromStation := MetersToNM(Haversine(
			aircraft.ADSB.Lat, aircraft.ADSB.Lon,
			s.stationLat, s.stationLon,
		))

		// If aircraft was close to airport and low altitude when signal lost
		if distanceFromStation <= s.flightPhasesConfig.AirportRangeNM &&
			aircraft.ADSB.AltBaro < s.flightPhasesConfig.SignalLostLandingMaxAltFt {

			// Mark as landed
			aircraft.OnGround = true
			s.storage.Upsert(aircraft)

			// Create T/D phase record
			adsbId, _ := s.storage.GetLatestADSBTargetID(aircraft.Hex)
			landingPhaseChanges = append(landingPhaseChanges, PhaseChangeInsert{
				Hex:       aircraft.Hex,
				Flight:    aircraft.Flight,
				Phase:     "T/D",
				Timestamp: now,
				ADSBId:    adsbId,
				EventType: "signal_lost_landing",
			})

			s.logger.Info("Signal lost aircraft marked as landed",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.Float64("last_altitude", aircraft.ADSB.AltBaro),
				logger.Float64("distance_from_airport", distanceFromStation),
			)
		}
	}

	return landingPhaseChanges
}

// hasRecentTakeoff determines if an aircraft has taken off recently
// This is used to identify aircraft in the departure phase even if they're not
// perfectly aligned with a runway (e.g., after turning to their departure heading)
//
// Returns true if ANY of these conditions are met:
// 1. Aircraft has a T/O (takeoff) phase record within the timeout period
// 2. Database shows a recent ground-to-air transition time
// 3. Aircraft is low altitude and close to the airport (likely just departed)
func (s *Service) hasRecentTakeoff(aircraft *Aircraft) bool {
	config := s.flightPhasesConfig
	// Convert timeout from minutes to a Duration (typically 30 minutes)
	timeoutDuration := time.Duration(config.RecentTakeoffTimeoutMinutes) * time.Minute

	// METHOD 1: Check phase history for recent T/O phase
	// This is the most reliable method as T/O phases are recorded immediately on takeoff
	phaseHistory, err := s.storage.GetPhaseHistory(aircraft.Hex)
	if err == nil {
		for _, phase := range phaseHistory {
			if phase.Phase == "T/O" && time.Since(phase.Timestamp) <= timeoutDuration {
				return true
			}
		}
	}

	// METHOD 2: Check for recent ground-to-air transition in database
	// This catches cases where we might have missed the T/O phase recording
	takeoffTime, err := s.storage.GetLatestTakeoffTime(aircraft.Hex)
	if err == nil && takeoffTime != nil && time.Since(*takeoffTime) <= timeoutDuration {
		return true
	}

	// METHOD 3: Proximity and altitude check
	// Aircraft close to airport at low altitude are likely recent departures
	// This helps catch aircraft we just started tracking after they took off
	if aircraft.ADSB != nil {
		distanceFromStation := MetersToNM(Haversine(aircraft.ADSB.Lat, aircraft.ADSB.Lon, s.stationLat, s.stationLon))

		// Check if aircraft is:
		// - Within 2x normal airport range (e.g., 10 NM if airport range is 5 NM)
		// - Below 2x departure altitude (e.g., 6000 ft if departure altitude is 3000 ft)
		// These multipliers give us a larger catch zone for recent departures
		if distanceFromStation <= float64(config.AirportRangeNM)*2 && // Within 2x airport range
			aircraft.ADSB.AltBaro <= float64(config.DepartureAltitudeFt)*2 { // Within 2x departure altitude
			return true
		}
	}

	return false
}

// detectRunwayDeparture determines if an aircraft is departing from any runway
// This helps identify aircraft in the departure phase based on their position
// relative to runway centerlines and their direction of travel
//
// Returns RunwayDepartureInfo if aircraft is:
// - Aligned with a runway (within tolerance)
// - Moving away from the airport
// - At appropriate altitude for departure
//
// This is particularly useful for catching departures when we didn't see the
// actual takeoff event (e.g., started tracking after aircraft was airborne)
func (s *Service) detectRunwayDeparture(aircraft *Aircraft) *RunwayDepartureInfo {
	// Skip if no runway data is configured for this airport
	if s.runwayData.Airport == "" {
		return nil // No runway data available
	}

	// Delegate to the runway detection utility function
	// This checks all configured runways to see if aircraft is on a departure path
	return DetectRunwayDeparture(
		aircraft.ADSB.Lat,
		aircraft.ADSB.Lon,
		aircraft.ADSB.Track,
		s.runwayData,
		s.stationLat,
		s.stationLon,
		s.flightPhasesConfig,
	)
}

// determineFlightPhase determines the current flight phase based on simplified logic
//
// Flight phases represent different stages of an aircraft's journey:
// - NEW: Aircraft just appeared or is parked/stationary on ground
// - TAX: Aircraft is taxiing on ground (moving between 1-50 knots)
// - T/O: Takeoff phase (preserved for 60 seconds after ground->air transition)
// - DEP: Departure phase (climbing away from airport)
// - CRZ: Cruise phase (high altitude, typically above 10,000 ft)
// - ARR: Arrival phase (descending towards destination, default airborne phase)
// - APP: Approach phase (aligned with runway, descending to land)
// - T/D: Touchdown/Landing phase (preserved for 60 seconds after air->ground transition)
//
// The function uses a priority-based system where certain conditions override others
func (s *Service) determineFlightPhase(aircraft *Aircraft) string {
	// STEP 1: Data Validation
	// If we don't have ADS-B position/altitude data, we can't determine phase accurately
	if aircraft.ADSB == nil {
		return "NEW"
	}

	adsb := aircraft.ADSB
	config := s.flightPhasesConfig

	// STEP 2: Emergency Aircraft Detection
	// Check if aircraft is squawking emergency code (7500=hijack, 7600=radio fail, 7700=emergency)
	// We still determine phase normally but log the emergency for awareness
	for _, emergencyCode := range config.EmergencySquawkCodes {
		if adsb.Squawk == emergencyCode {
			s.logger.Warn("Emergency squawk detected",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("squawk", adsb.Squawk))
			// For emergency aircraft, determine phase normally but log the emergency
			break
		}
	}

	// STEP 3: GROUND PHASE DETERMINATION
	// Ground phases are determined first as they override any altitude-based logic
	if aircraft.OnGround {
		// Get the aircraft's current phase to make intelligent decisions
		latestPhase, err := s.storage.GetCurrentPhase(aircraft.Hex)

		// STEP 3A: Check if aircraft is taxiing
		// Taxiing = moving on ground between 1-50 knots ground speed
		// This covers aircraft moving to/from runway, between gates, etc.
		if adsb.GS >= float64(config.TaxiingMinSpeedKts) && adsb.GS <= float64(config.TaxiingMaxSpeedKts) {
			return "TAX"
		}

		// STEP 3B: Stationary aircraft handling
		// Aircraft on ground but not moving (parked at gate, holding short, etc.)
		if err != nil || latestPhase == nil {
			// This is a brand new aircraft we haven't seen before
			return "NEW"
		}

		// For existing aircraft, preserve TAX phase even when stopped
		// This prevents flapping between TAX and NEW when aircraft stops briefly
		if latestPhase.Phase == "TAX" {
			// Keep aircraft in TAX phase until takeoff or timeout
			// The timeout is handled in evaluatePhaseChange
			return "TAX"
		}

		// For all other phases, preserve the current phase
		return latestPhase.Phase
	}

	// STEP 4: AIRBORNE PHASE DETERMINATION
	// Aircraft is flying - determine which flight phase based on altitude, location, and behavior
	altitude := adsb.AltBaro      // Barometric altitude in feet
	verticalRate := adsb.BaroRate // Vertical speed in feet per minute

	// STEP 4A: CRUISE PHASE - Highest Priority
	// Aircraft at cruise altitude (typically 10,000+ ft) are in cruise phase
	// This overrides all other airborne phases as cruise is unambiguous
	if altitude >= float64(config.CruiseAltitudeFt) {
		return "CRZ"
	}

	// STEP 4B: APPROACH PHASE - Second Priority
	// Detect if aircraft is on final approach to land
	var onRunwayCenterline bool
	var approachingAirport bool

	// Only check approach phase for low altitude aircraft (below typical pattern altitude)
	if altitude <= float64(config.TakeoffAltitudeThresholdFt) {
		// Check if aircraft is aligned with any runway (extended centerline)
		runwayInfo := DetectRunwayApproach(adsb.Lat, adsb.Lon, adsb.Track, altitude, s.runwayData, config)
		if runwayInfo != nil && runwayInfo.OnApproach {
			onRunwayCenterline = true

			// IMPORTANT: Verify aircraft is flying TOWARDS the airport, not away
			// This prevents departing aircraft from being marked as approaching
			bearingToStation := CalculateBearing(adsb.Lat, adsb.Lon, s.stationLat, s.stationLon)
			headingDiff := math.Abs(adsb.Track - bearingToStation)
			if headingDiff > 180 {
				headingDiff = 360 - headingDiff
			}
			// Aircraft heading should be within 90° of direct path to airport
			approachingAirport = headingDiff <= 90
		}
	}

	// Confirm approach phase: must be aligned with runway, heading towards airport, and descending
	if onRunwayCenterline && approachingAirport && verticalRate <= float64(config.ApproachVerticalRateThresholdFPM) {
		return "APP"
	}

	// STEP 4C: DEPARTURE PHASE
	// Detect if aircraft is in initial climb after takeoff

	// Check two conditions for departure:
	// 1. Aircraft took off recently (within configured timeout, typically 30 minutes)
	hasRecentTakeoff := s.hasRecentTakeoff(aircraft)

	// 2. Aircraft is on runway heading climbing away from airport
	departureInfo := s.detectRunwayDeparture(aircraft)
	isMovingAwayFromStation := departureInfo != nil && departureInfo.OnDeparture

	// Aircraft qualifies for departure phase if it meets either condition above
	if hasRecentTakeoff || isMovingAwayFromStation {
		// For aircraft with recent takeoff, be more lenient with DEP phase detection
		if hasRecentTakeoff {
			// Aircraft that recently took off should be in DEP phase if:
			// - Below cruise altitude AND (climbing OR low altitude after takeoff)
			if altitude < float64(config.CruiseAltitudeFt) &&
				(verticalRate > 0 || altitude < float64(config.DepartureAltitudeFt)*2) {
				return "DEP"
			}
		}

		// For aircraft moving away from station, use stricter criteria:
		// - Above departure altitude and climbing, OR
		// - On runway centerline, climbing, and below pattern altitude
		if (altitude >= float64(config.DepartureAltitudeFt) && verticalRate > float64(config.ClimbingVerticalRateFPM)) ||
			(onRunwayCenterline && verticalRate > float64(config.ClimbingVerticalRateFPM) && altitude < float64(config.TakeoffAltitudeThresholdFt)) {
			return "DEP"
		}
	}

	// STEP 4D: ARRIVAL PHASE - Default
	// Any airborne aircraft that doesn't meet criteria for CRZ, APP, or DEP
	// This includes:
	// - Aircraft descending from cruise (but not yet on approach)
	// - Aircraft in holding patterns
	// - Aircraft maneuvering in terminal area
	// - General aviation aircraft below cruise altitude
	return "ARR"
}

// processPhaseChangesBatch handles phase detection using batch operations for better performance
func (s *Service) processPhaseChangesBatch(aircraft []*Aircraft, immediatePhaseChanges []PhaseChangeInsert) {
	if !s.flightPhasesConfig.Enabled {
		return // Phase detection is disabled
	}

	if len(aircraft) == 0 {
		return
	}

	// Create map of aircraft that just had immediate ground transitions
	immediateTransitions := make(map[string]string) // hex -> phase
	for _, change := range immediatePhaseChanges {
		immediateTransitions[change.Hex] = change.Phase
	}

	// Step 1: Get all aircraft hex codes
	hexCodes := make([]string, len(aircraft))
	aircraftMap := make(map[string]*Aircraft)
	for i, a := range aircraft {
		hexCodes[i] = a.Hex
		aircraftMap[a.Hex] = a
	}

	// Step 2: Batch query for current phases
	currentPhases, err := s.storage.GetCurrentPhasesBatch(hexCodes)
	if err != nil {
		s.logger.Error("Failed to get current phases batch", logger.Error(err))
		return
	}

	// Step 3: Batch query for ADSB target IDs
	adsbTargetIDs, err := s.storage.GetLatestADSBTargetIDsBatch(hexCodes)
	if err != nil {
		s.logger.Error("Failed to get ADSB target IDs batch", logger.Error(err))
		return
	}

	// Step 4: Process each aircraft with simple logic
	var phaseChanges []PhaseChangeInsert
	for _, a := range aircraft {
		// Skip aircraft that just had immediate ground transitions
		if _, hasImmediate := immediateTransitions[a.Hex]; hasImmediate {
			continue // Already handled by immediate ground transition detection
		}

		currentPhase := currentPhases[a.Hex]
		newPhase := s.determineFlightPhase(a)

		// Apply phase stability rules and determine if change needed
		finalPhase, shouldInsert := s.evaluatePhaseChange(a, currentPhase, newPhase)

		if shouldInsert {
			phaseChanges = append(phaseChanges, PhaseChangeInsert{
				Hex:       a.Hex,
				Flight:    a.Flight,
				Phase:     finalPhase,
				Timestamp: time.Now().UTC(),
				ADSBId:    adsbTargetIDs[a.Hex],
			})
		}
	}

	// Step 5: Batch insert all phase changes
	if len(phaseChanges) > 0 {
		err := s.storage.InsertPhaseChangesBatch(phaseChanges)
		if err != nil {
			s.logger.Error("Failed to insert phase changes batch", logger.Error(err))
			return
		}

		// Step 6: Send WebSocket alerts and log changes
		s.sendPhaseChangeAlerts(phaseChanges, currentPhases)
	}
}

// evaluatePhaseChange applies phase stability rules and determines if a phase change should be inserted
//
// TIMEOUT CONFIGURATION CLARIFICATION:
//
// 1. PhaseChangeTimeoutSeconds (default: 3600 seconds = 1 hour)
//   - Used for INACTIVE aircraft that haven't been seen recently
//   - If an aircraft hasn't changed phase for this duration, it reverts to NEW
//   - This handles aircraft that have been parked for a long time
//   - Example: Aircraft lands, taxis to gate, stays there for > 1 hour → NEW
//
// 2. PhaseTransitionTimeoutSeconds (default: 60 seconds, but 1800 = 30 min recommended)
//   - Used to prevent rapid flapping between certain phase transitions
//   - Specifically prevents TAX→NEW and T/D→NEW transitions from happening too quickly
//   - This handles brief stops during taxiing or after landing
//   - Example: Aircraft stops taxiing for 30 seconds → stays in TAX (not NEW)
//
// 3. PhasePreservationSeconds (default: 60 seconds)
//   - Used to preserve T/O and T/D phases for visibility
//   - These critical phases are kept for at least this duration
//   - Ensures pilots/controllers can see takeoff/landing events in the UI
//
// 4. PhaseFlappingPreventionSeconds (default: 300 seconds = 5 minutes)
//   - Used to prevent rapid flapping between airborne phases
//   - Specifically prevents DEP↔APP and ARR↔DEP transitions
//   - Much shorter than PhaseChangeTimeoutSeconds as these are active aircraft
//   - Example: Aircraft briefly meets APP criteria → stays in DEP for 5 minutes
//
// The key difference: PhaseChangeTimeoutSeconds is for long-term inactive aircraft,
// PhaseTransitionTimeoutSeconds prevents ground phase flapping, and
// PhaseFlappingPreventionSeconds prevents airborne phase flapping.
func (s *Service) evaluatePhaseChange(aircraft *Aircraft, latestPhase *PhaseChange, newPhase string) (string, bool) {
	currentPhase := newPhase

	// Apply phase stability rules to prevent flapping
	if latestPhase != nil {
		timeSinceLastPhase := time.Since(latestPhase.Timestamp).Seconds()

		// Phase flapping prevention
		shouldPreventFlapping := false
		var flappingType string

		if latestPhase.Phase == "DEP" && currentPhase == "APP" {
			shouldPreventFlapping = true
			flappingType = "DEP → APP"
		} else if (latestPhase.Phase == "ARR" && currentPhase == "DEP") ||
			(latestPhase.Phase == "DEP" && currentPhase == "ARR") {
			shouldPreventFlapping = true
			flappingType = "ARR ↔ DEP"
		}

		if shouldPreventFlapping && timeSinceLastPhase < float64(s.flightPhasesConfig.PhaseFlappingPreventionSeconds) {
			currentPhase = latestPhase.Phase // Keep current phase
			s.logger.Debug("Prevented phase flapping",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("flapping_type", flappingType),
				logger.String("prev_phase", latestPhase.Phase),
				logger.String("attempted_phase", newPhase),
				logger.Float64("time_since_last_phase", timeSinceLastPhase),
				logger.Int("timeout_seconds", s.flightPhasesConfig.PhaseFlappingPreventionSeconds))
		}
	}

	// Special handling for aircraft that just landed (T/D phase)
	if latestPhase != nil && latestPhase.Phase == "T/D" {
		if currentPhase == "NEW" {
			// Check if aircraft is moving on ground (taxiing)
			if aircraft.OnGround && aircraft.ADSB.GS >= float64(s.flightPhasesConfig.TaxiingMinSpeedKts) && aircraft.ADSB.GS <= float64(s.flightPhasesConfig.TaxiingMaxSpeedKts) {
				currentPhase = "TAX"
			} else {
				// Aircraft is stationary after landing, keep it as T/D for a minimum time
				timeSinceLanding := time.Since(latestPhase.Timestamp).Seconds()
				if timeSinceLanding < float64(s.flightPhasesConfig.PhasePreservationSeconds) { // Use config value
					currentPhase = "T/D" // Keep current phase
				} else {
					// After minimum time, allow transition to NEW only if truly stationary
					if aircraft.ADSB.GS < float64(s.flightPhasesConfig.TaxiingMinSpeedKts) {
						currentPhase = "NEW"
					} else {
						currentPhase = "TAX"
					}
				}
			}
		}
	}

	// Special handling for aircraft that just took off (T/O phase)
	if latestPhase != nil && latestPhase.Phase == "T/O" {
		// Preserve T/O phase for minimum time to ensure visibility
		timeSinceTakeoff := time.Since(latestPhase.Timestamp).Seconds()
		if timeSinceTakeoff < float64(s.flightPhasesConfig.PhasePreservationSeconds) {
			currentPhase = "T/O" // Keep T/O phase
			s.logger.Debug("Preserving T/O phase",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("attempted_phase", newPhase),
				logger.Float64("time_since_takeoff", timeSinceTakeoff),
				logger.Int("preservation_seconds", s.flightPhasesConfig.PhasePreservationSeconds))
		}
		// After preservation time, allow normal phase transitions (DEP, ARR, etc.)
	}

	// Check if phase has changed or if this is a new aircraft
	var shouldInsert bool

	if latestPhase == nil {
		// New aircraft - insert NEW phase
		shouldInsert = true
	} else {
		// Check if phase has changed
		if latestPhase.Phase != currentPhase {
			// Special handling for transitions to NEW phase
			if currentPhase == "NEW" {
				// Prevent immediate transitions to NEW from active phases
				// Apply timeout protection for T/D, TAX phases
				if latestPhase.Phase == "T/D" || latestPhase.Phase == "TAX" {
					timeSinceLastPhase := time.Since(latestPhase.Timestamp).Seconds()
					if timeSinceLastPhase < float64(s.flightPhasesConfig.PhaseTransitionTimeoutSeconds) { // Use config value
						currentPhase = latestPhase.Phase // Keep current phase
						s.logger.Debug("Prevented premature transition to NEW",
							logger.String("hex", aircraft.Hex),
							logger.String("flight", aircraft.Flight),
							logger.String("prev_phase", latestPhase.Phase),
							logger.Float64("time_since_last_phase", timeSinceLastPhase))
					} else {
						shouldInsert = true
					}
				} else {
					shouldInsert = true
				}
			} else {
				shouldInsert = true
			}
		} else {
			// Check timeout for inactive aircraft (revert to NEW phase)
			if latestPhase.Phase != "NEW" {
				timeSinceLastPhase := time.Since(latestPhase.Timestamp).Seconds()
				if timeSinceLastPhase > float64(s.flightPhasesConfig.PhaseChangeTimeoutSeconds) {
					currentPhase = "NEW"
					shouldInsert = true
				}
			}
		}
	}

	return currentPhase, shouldInsert
}

// sendPhaseChangeAlerts sends WebSocket alerts for phase changes
func (s *Service) sendPhaseChangeAlerts(phaseChanges []PhaseChangeInsert, currentPhases map[string]*PhaseChange) {
	for _, change := range phaseChanges {
		aircraft, found := s.storage.GetByHex(change.Hex)
		if !found || aircraft == nil {
			continue
		}

		var previousPhase string
		if prevPhase := currentPhases[change.Hex]; prevPhase != nil {
			previousPhase = prevPhase.Phase
		}

		// Log the phase change with detailed aircraft data for debugging
		distanceFromStation := MetersToNM(Haversine(aircraft.ADSB.Lat, aircraft.ADSB.Lon, s.stationLat, s.stationLon))

		if previousPhase == "" {
			s.logger.Info("New aircraft phase detected",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("phase", change.Phase),
				logger.Float64("altitude", aircraft.ADSB.AltBaro),
				logger.Float64("ground_speed", aircraft.ADSB.GS),
				logger.Float64("vertical_rate", aircraft.ADSB.BaroRate),
				logger.Float64("distance_from_station", distanceFromStation),
				logger.Bool("on_ground", aircraft.OnGround),
			)
		} else {
			s.logger.Info("Phase change detected",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("transition", previousPhase+" → "+change.Phase),
				logger.Float64("altitude", aircraft.ADSB.AltBaro),
				logger.Float64("ground_speed", aircraft.ADSB.GS),
				logger.Float64("vertical_rate", aircraft.ADSB.BaroRate),
				logger.Float64("distance_from_station", distanceFromStation),
				logger.Bool("on_ground", aircraft.OnGround),
			)
		}

		// Send WebSocket message for phase change
		if s.wsServer != nil {
			// Create message data for phase change
			data := map[string]interface{}{
				"hex":        aircraft.Hex,
				"flight":     aircraft.Flight,
				"phase":      change.Phase,
				"prev_phase": previousPhase,
				"transition": previousPhase + " → " + change.Phase,
				"altitude":   aircraft.ADSB.AltBaro,
				"on_ground":  aircraft.OnGround,
				"timestamp":  change.Timestamp.Format(time.RFC3339),
			}

			// Broadcast the phase change message
			s.wsServer.Broadcast(&websocket.Message{
				Type: "phase_change",
				Data: data,
			})
		}

		// Handle special takeoff/landing phases
		if change.Phase == "T/O" {
			s.logger.Info("Aircraft TOOK OFF",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("transition", previousPhase+" → T/O"),
				logger.Float64("altitude", aircraft.ADSB.AltBaro),
				logger.Bool("on_ground", aircraft.OnGround),
			)

			// T/O phase change message is sent by the main phase change handler
		} else if change.Phase == "T/D" {
			s.logger.Info("Aircraft LANDED",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("transition", previousPhase+" → T/D"),
				logger.Float64("altitude", aircraft.ADSB.AltBaro),
				logger.Bool("on_ground", aircraft.OnGround),
			)

			// T/D phase change message is sent by the main phase change handler
		}
	}
}

// sendImmediateGroundTransitionAlerts sends immediate WebSocket alerts for ground state transitions
func (s *Service) sendImmediateGroundTransitionAlerts(phaseChanges []PhaseChangeInsert) {
	for _, change := range phaseChanges {
		aircraft, found := s.storage.GetByHex(change.Hex)
		if !found || aircraft == nil {
			continue
		}

		// Get the previous phase for the transition message
		var previousPhase string
		phaseHistory, err := s.storage.GetPhaseHistory(change.Hex)
		if err == nil && len(phaseHistory) > 1 {
			// The first item is the current (just inserted), second is the previous
			previousPhase = phaseHistory[1].Phase
		}

		if change.Phase == "T/O" {
			s.logger.Info("Aircraft TOOK OFF (IMMEDIATE)",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.Float64("altitude", aircraft.ADSB.AltBaro),
				logger.String("timestamp", change.Timestamp.Format(time.RFC3339)),
			)

			// Send phase change message first
			if s.wsServer != nil {
				// Send phase_change message
				phaseData := map[string]interface{}{
					"hex":        aircraft.Hex,
					"flight":     aircraft.Flight,
					"phase":      change.Phase,
					"prev_phase": previousPhase,
					"transition": previousPhase + " → " + change.Phase,
					"altitude":   aircraft.ADSB.AltBaro,
					"on_ground":  aircraft.OnGround,
					"timestamp":  change.Timestamp.Format(time.RFC3339),
				}

				s.wsServer.Broadcast(&websocket.Message{
					Type: "phase_change",
					Data: phaseData,
				})

				// T/O phase change message already sent above
			}

		} else if change.Phase == "T/D" {
			s.logger.Info("Aircraft LANDED (IMMEDIATE)",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.Float64("altitude", aircraft.ADSB.AltBaro),
				logger.String("timestamp", change.Timestamp.Format(time.RFC3339)),
			)

			// Send phase change message first
			if s.wsServer != nil {
				// Send phase_change message
				phaseData := map[string]interface{}{
					"hex":        aircraft.Hex,
					"flight":     aircraft.Flight,
					"phase":      change.Phase,
					"prev_phase": previousPhase,
					"transition": previousPhase + " → " + change.Phase,
					"altitude":   aircraft.ADSB.AltBaro,
					"on_ground":  aircraft.OnGround,
					"timestamp":  change.Timestamp.Format(time.RFC3339),
				}

				s.wsServer.Broadcast(&websocket.Message{
					Type: "phase_change",
					Data: phaseData,
				})

				// T/D phase change message already sent above
			}
		}
	}
}

// ProcessRawData processes raw ADS-B data into aircraft objects
func (s *Service) ProcessRawData(rawData *RawAircraftData) []*Aircraft {
	s.logger.Debug("Processing raw ADS-B data",
		logger.Int("aircraft_count", len(rawData.Aircraft)),
	)

	aircraft := make([]*Aircraft, 0, len(rawData.Aircraft))
	now := time.Now().UTC() // Ensure we use UTC time

	// Create a map of active aircraft hex codes
	activeAircraft := make(map[string]bool)
	for _, rawItem := range rawData.Aircraft { // Renamed to avoid editor/linter confusion with field name
		activeAircraft[rawItem.Hex] = true
	}

	// Create a map of existing aircraft for quick lookup
	existingAircraftMap := make(map[string]bool)
	existingAircraft := s.storage.GetAll()
	for _, a := range existingAircraft {
		existingAircraftMap[a.Hex] = true
	}

	for _, raw := range rawData.Aircraft {
		// Skip aircraft without position data
		if raw.Lat == 0 && raw.Lon == 0 {
			continue
		}

		// Get the raw flight name and clean it
		flightRaw := raw.Flight
		flightName := strings.TrimSpace(CleanFlightName(flightRaw))

		// If flightName is empty but hex is available, try to derive tail number
		if flightName == "" && raw.Hex != "" {
			tailNumber, err := IcaoToTailNumber(raw.Hex) // Use exported function from atc_utils.go
			if err == nil && tailNumber != "" {
				flightName = tailNumber + "*" // Appended * to indicate derived tail number
				s.logger.Debug("Derived tail number from ICAO hex",
					logger.String("hex", raw.Hex),
					logger.String("tail_number", flightName))
			} else if err != nil {
				s.logger.Debug("Failed to derive tail number from ICAO hex",
					logger.String("hex", raw.Hex),
					logger.Error(err))
			}
		}

		// Determine airline from callsign only for valid flight numbers (3 letters + 1-4 numbers)
		var airlineName string
		if len(flightName) >= 4 && len(flightName) <= 7 {
			// Check if the first 3 characters are letters
			firstThree := strings.ToUpper(flightName[:3])
			isAllLetters := true
			for _, c := range firstThree {
				if c < 'A' || c > 'Z' {
					isAllLetters = false
					break
				}
			}

			// Check if the remaining characters are digits (1-4 digits)
			remainingChars := flightName[3:]
			isAllDigits := true
			for _, c := range remainingChars {
				if c < '0' || c > '9' {
					isAllDigits = false
					break
				}
			}

			// Only lookup airline if it's a valid flight number (3 letters + 1-4 numbers)
			if isAllLetters && isAllDigits && len(remainingChars) >= 1 && len(remainingChars) <= 4 {
				icaoCode := firstThree
				airlineName = s.airlineMap[icaoCode]
				s.logger.Debug("Detected valid flight number",
					logger.String("flight", flightName),
					logger.String("airline_code", icaoCode),
					logger.String("airline", airlineName))
			}
		}

		// Determine if aircraft is on ground based on speed and altitude
		// Get previous data for sensor validation if this aircraft exists
		var prevTAS, prevGS, prevAlt float64
		if existingAircraftMap[raw.Hex] {
			// Find the existing aircraft data for sensor validation
			for _, existing := range existingAircraft {
				if existing.Hex == raw.Hex && existing.ADSB != nil {
					prevTAS = existing.ADSB.TAS
					prevGS = existing.ADSB.GS
					prevAlt = existing.ADSB.AltBaro
					break
				}
			}
		}

		// Validate and correct sensor data for potential errors
		correctedTAS, correctedGS, correctedAlt := ValidateSensorData(
			raw.TAS, raw.GS, raw.AltBaro,
			prevTAS, prevGS, prevAlt,
			raw.Lat, raw.Lon, s.stationLat, s.stationLon,
			s.flightPhasesConfig.AirportRangeNM,
			&s.flightPhasesConfig,
		)

		// Determine ground state using corrected values
		onGround := !IsFlying(correctedTAS, correctedGS, correctedAlt, &s.flightPhasesConfig)

		// Log sensor corrections if they occurred
		if correctedTAS != raw.TAS || correctedGS != raw.GS || correctedAlt != raw.AltBaro {
			s.logger.Debug("Sensor data corrected in ProcessRawData",
				logger.String("hex", raw.Hex),
				logger.String("flight", flightName),
				logger.Float64("original_tas", raw.TAS),
				logger.Float64("corrected_tas", correctedTAS),
				logger.Float64("original_gs", raw.GS),
				logger.Float64("corrected_gs", correctedGS),
				logger.Float64("original_alt", raw.AltBaro),
				logger.Float64("corrected_alt", correctedAlt),
			)
		}

		// Check if this is a new aircraft (first time seen)
		isNewAircraft := !existingAircraftMap[raw.Hex]

		// Process aircraft data
		// Set status to "active" for aircraft that are currently transmitting
		// This ensures aircraft marked as "signal_lost" are restored to "active" when they reappear
		// Set status to "active" for aircraft that are currently transmitting
		aircraftStatus := "active" // Always set to active for aircraft in current ADSB data

		// Check if this is a simulated aircraft
		isSimulated := (s.simulationService != nil && s.simulationService.IsSimulated(raw.Hex)) || raw.Type == "sim"
		var simulationControls *SimulationControls

		if isSimulated {
			// For simulated aircraft, extract controls from the raw data type field
			// The simulation service will have already populated the ADSB data with current values
			simulationControls = &SimulationControls{
				TargetHeading:      raw.TrueHeading, // Use current heading as target
				TargetSpeed:        raw.TAS,         // Use current TAS as target
				TargetVerticalRate: raw.BaroRate,    // Use current vertical rate as target
			}
		}

		a := &Aircraft{
			Hex:                raw.Hex,
			Flight:             flightName,
			Airline:            airlineName,
			Status:             aircraftStatus,                                  // Set to active for aircraft in current ADSB data
			Phase:              nil,                                             // Phase will be handled separately
			LastSeen:           now.Add(-time.Duration(raw.Seen) * time.Second), // Already in UTC since now is UTC
			OnGround:           onGround,
			ADSB:               &raw,
			IsSimulated:        isSimulated,
			SimulationControls: simulationControls,
		}

		// TODO: Phase detection will be implemented separately using the new phase_changes table
		// For now, we just process the aircraft data without phase detection

		// If this is a new aircraft, log it and send a WebSocket message
		if isNewAircraft {
			s.logger.Info("New aircraft detected",
				logger.String("hex", a.Hex),
				logger.String("flight", a.Flight),
				logger.Float64("altitude", a.ADSB.AltBaro),
				logger.Bool("on_ground", a.OnGround),
			)

			// Send WebSocket message for new aircraft
			if s.wsServer != nil {
				// Create message data
				data := map[string]interface{}{
					"hex":        a.Hex,
					"flight":     a.Flight,
					"altitude":   a.ADSB.AltBaro,
					"on_ground":  a.OnGround,
					"timestamp":  time.Now().UTC().Format(time.RFC3339),
					"new_status": "new_aircraft",
				}

				// Broadcast the message
				s.wsServer.Broadcast(&websocket.Message{
					Type: "status_update",
					Data: data,
				})
			}
		}

		// Calculate future positions if we have the necessary data
		if raw.Lat != 0 && raw.Lon != 0 && raw.AltBaro != 0 {
			// Get heading (use true_heading, track, or mag_heading, whichever is available)
			heading := raw.TrueHeading
			if heading == 0 {
				heading = raw.Track
			}
			if heading == 0 {
				heading = raw.MagHeading
			}

			// Get speed (use TAS or GS, whichever is available)
			speed := raw.TAS
			if speed == 0 {
				speed = raw.GS
			}

			// Get vertical rate (use baro_rate or geom_rate, whichever is available)
			verticalRate := raw.BaroRate
			if verticalRate == 0 {
				verticalRate = raw.GeomRate
			}

			// Only predict if we have valid heading and speed
			if heading != 0 && speed != 0 {
				// Calculate future positions
				// Get magnetic heading for predictions
				magHeading := raw.MagHeading
				if magHeading == 0 {
					magHeading = heading // fallback to whatever heading we found
				}

				futurePredictions := PredictFuturePositions(
					raw.Lat,
					raw.Lon,
					raw.AltBaro,
					heading,    // true heading
					magHeading, // magnetic heading
					speed,
					verticalRate,
				)

				// Add future predictions to the aircraft
				a.Future = futurePredictions
			}
		}

		aircraft = append(aircraft, a)
	}

	s.logger.Debug("Processed ADS-B data",
		logger.Int("processed_count", len(aircraft)),
	)

	return aircraft
}
