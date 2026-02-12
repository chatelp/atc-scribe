package adsb

/*
FLIGHT PHASE DETECTION SYSTEM
==============================

The Co-ATC system uses a two-tier approach for flight phase detection:

  PRIORITY 1 — Immediate ground state transitions (T/O and T/D)
    Detected instantly when the OnGround state flips via IsFlying().
    These are binary state changes that do not benefit from trajectory analysis.

  PRIORITY 2 — All other phases via trajectory analysis
    The TrajectoryTracker (trajectory.go, trajectory_phase.go) maintains a rolling
    window of ~90 seconds of ADS-B observations per aircraft and uses statistical
    analysis (OLS regression, median filtering, distance trends) to classify each
    aircraft into one of 10 phases:

      NEW — Newly detected or parked on ground
      TAX — Taxiing (1–50 kts ground speed)
      T/O — Takeoff (ground→air transition, preserved for UI visibility)
      CLB — Initial climb out (runway heading, near airport, hasRecentTakeoff)
      DEP — Departure (climbing away, below cruise — general)
      CRZ — Cruise (above cruise altitude)
      ARR — Arrival (approaching station, below cruise, descending/level)
      APP — Approach (on final, descending, runway-aligned)
      T/D — Touchdown (air→ground transition, preserved for UI visibility)
      UNK — Unknown (airborne but doesn't match any specific condition)

  ARR is NOT a catch-all — it requires the aircraft to be genuinely approaching
  the station. UNK is the honest catch-all for anything that doesn't match.

SPECIAL FEATURES:
  - Trajectory-based noise resistance: decisions use windowed trends, not single readings
  - Signal Lost Landing: auto-marks aircraft as landed when signal lost near airport,
    enhanced by trajectory descent trend analysis
  - Phase Preservation: T/O and T/D kept visible for configurable duration
  - Sensor Data Validation: corrects erroneous readings before trajectory ingestion
  - Data gap detection: handles coverage gaps without spurious phase flips
*/

import (
	"context"
	"encoding/json"
	"fmt"
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

// BSDBService defines the interface for BaseStation.sqb lookup service
type BSDBService interface {
	Lookup(hex string) *BSDBInfo
	LookupBatch(hexCodes []string) map[string]*BSDBInfo
}

// BSDBInfo represents aircraft info from BaseStation.sqb (interface type for dependency injection)
type BSDBInfo struct {
	Registration     string
	ICAOTypeCode     string
	OperatorFlagCode string
	Manufacturer     string
	Type             string
	RegisteredOwners string
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
	GetAllWithLastSeenFilter(lastSeenMinutes int) []*Aircraft
	GetAllMinimal(lastSeenMinutes int) []*Aircraft // Minimal mode: skips phase history and date queries
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

	// Batch methods for fetch hot path optimization
	GetAircraftOnGroundBatch(hexCodes []string) (map[string]bool, error)
	GetLatestADSBDataBatch(hexCodes []string) (map[string]*ADSBTarget, error)
	GetLatestTakeoffTimesBatch(hexCodes []string) (map[string]*time.Time, error)
	GetLatestLandingTimesBatch(hexCodes []string) (map[string]*time.Time, error)

	// Targeted query for inactive aircraft status updates
	GetStaleActiveAircraft(activeHexCodes []string, cutoff time.Time) ([]*Aircraft, error)
}

// SimulationService defines the interface for simulation service
type SimulationService interface {
	UpdatePositions()
	GenerateADSBData() []ADSBTarget
	IsSimulated(hex string) bool
	GetAllAircraft() interface{}                                       // Returns simulation aircraft data
	GetAircraft(hex string) (interface{}, bool)                        // Returns specific simulated aircraft
	UpdateControls(hex string, heading, speed, verticalRate float64) error // Update simulation controls
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
	bsdbService        BSDBService               // BaseStation.sqb lookup service
	trajectoryTracker  *TrajectoryTracker        // Trajectory-based phase detection
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

	// Always enable WebSocket streaming for aircraft updates
	logger.Info("Initializing WebSocket change detection for aircraft streaming")
	service.changeDetector = NewChangeDetector(logger)
	service.broadcastChan = make(chan []AircraftChange, 256) // Buffered to handle burst traffic
	service.startBroadcastWorker()

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

	// Initialize trajectory tracker for phase detection
	if flightPhasesConfig.Enabled {
		fetchIntervalSec := adsbCfg.FetchIntervalSecs
		if fetchIntervalSec < 1 {
			fetchIntervalSec = 1
		}
		bufferCap := flightPhasesConfig.TrajectoryBufferDurationSec/fetchIntervalSec + 10 // +10 margin
		service.trajectoryTracker = NewTrajectoryTracker(
			TrajectoryConfig{
				BufferDurationSec:       flightPhasesConfig.TrajectoryBufferDurationSec,
				BufferCapacity:          bufferCap,
				FetchIntervalSec:        fetchIntervalSec,
				MinPointsForAnalysis:    flightPhasesConfig.TrajectoryMinPoints,
				StaleTimeoutSec:         flightPhasesConfig.TrajectoryStaleTimeoutSec,
				CleanupIntervalSec:      flightPhasesConfig.TrajectoryCleanupIntervalSec,
				DescentVRThresholdFPM:   flightPhasesConfig.TrajectoryDescentThresholdFPM,
				ClimbVRThresholdFPM:     flightPhasesConfig.TrajectoryClimbThresholdFPM,
				LevelAltBandFt:          flightPhasesConfig.TrajectoryLevelBandFt,
				TurningRateThresholdDeg: flightPhasesConfig.TrajectoryTurningRateDeg,
				DecelerationThreshold:   -0.5,
				AccelerationThreshold:   0.5,
			},
			stationCfg.Latitude,
			stationCfg.Longitude,
			service.runwayData,
			&service.flightPhasesConfig,
			logger,
		)
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

	// For "added", send full aircraft object
	// For "updated", send only the delta (changed fields)
	// For "removed", just send hex
	if change.Aircraft != nil {
		data["aircraft"] = change.Aircraft
	}
	if change.Delta != nil {
		data["delta"] = change.Delta
	}

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
				Alt: aircraft.ADSB.AltBaro.Float64(),
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
			logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
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
	if s.trajectoryTracker != nil {
		s.trajectoryTracker.Stop()
	}
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

	// Collect all hex codes for batch operations
	hexCodes := make([]string, len(newAircraft))
	for i, a := range newAircraft {
		hexCodes[i] = a.Hex
	}

	// PERFORMANCE: Batch queries replace ~8000 per-aircraft queries with 6 batch queries per cycle
	existingOnGround, _ := s.storage.GetAircraftOnGroundBatch(hexCodes)
	existingADSB, _ := s.storage.GetLatestADSBDataBatch(hexCodes)
	takeoffTimes, _ := s.storage.GetLatestTakeoffTimesBatch(hexCodes)
	landingTimes, _ := s.storage.GetLatestLandingTimesBatch(hexCodes)
	currentPhases, _ := s.storage.GetCurrentPhasesBatch(hexCodes)
	adsbTargetIDs, _ := s.storage.GetLatestADSBTargetIDsBatch(hexCodes)

	// Process each aircraft for ground state determination and takeoff/landing detection
	for _, a := range newAircraft {
		// Check if aircraft exists in database using pre-fetched batch data
		_, found := existingOnGround[a.Hex]

		var prevTAS, prevGS, prevAlt float64
		if found {
			if adsbData, ok := existingADSB[a.Hex]; ok && adsbData != nil {
				prevTAS = adsbData.TAS
				prevGS = adsbData.GS
				prevAlt = adsbData.AltBaro.Float64()
			}
		}

		// Validate and correct sensor data for potential errors
		correctedTAS, correctedGS, correctedAlt := ValidateSensorData(
			a.ADSB.TAS, a.ADSB.GS, a.ADSB.AltBaro.Float64(),
			prevTAS, prevGS, prevAlt,
			a.ADSB.Lat, a.ADSB.Lon, s.stationLat, s.stationLon,
			s.flightPhasesConfig.AirportRangeNM,
			&s.flightPhasesConfig,
		)

		// Apply corrected values back to ADSB data so the entire pipeline
		// (phase detection, DB storage, change detector) uses consistent corrected data
		if correctedTAS != a.ADSB.TAS || correctedGS != a.ADSB.GS || correctedAlt != a.ADSB.AltBaro.Float64() {
			if found {
				s.logger.Debug("Sensor data corrected",
					logger.String("hex", a.Hex),
					logger.String("flight", a.Flight),
					logger.Float64("original_tas", a.ADSB.TAS),
					logger.Float64("corrected_tas", correctedTAS),
					logger.Float64("original_gs", a.ADSB.GS),
					logger.Float64("corrected_gs", correctedGS),
					logger.Float64("original_alt", a.ADSB.AltBaro.Float64()),
					logger.Float64("corrected_alt", correctedAlt),
				)
			}
			a.ADSB.TAS = correctedTAS
			a.ADSB.GS = correctedGS
			a.ADSB.AltBaro = FlexibleFloat64(correctedAlt)
		}

		// Determine if aircraft is currently flying using corrected values and config
		currentlyFlying := IsFlying(a.ADSB.TAS, a.ADSB.GS, a.ADSB.AltBaro.Float64(), &s.flightPhasesConfig)

		// Always set on_ground based on flying state
		a.OnGround = !currentlyFlying

		// Feed corrected data into trajectory tracker for phase analysis
		if s.trajectoryTracker != nil && a.ADSB != nil {
			snap := TrajectorySnapshotFromADSB(a.ADSB, a.OnGround, time.Now().UTC())
			s.trajectoryTracker.Ingest(a.Hex, snap)
		}

		if found {
			// Use pre-fetched takeoff and landing times
			a.DateTookoff = takeoffTimes[a.Hex]
			a.DateLanded = landingTimes[a.Hex]
		} else {
			// New aircraft — log and send WebSocket notification
			s.logger.Info("New aircraft detected",
				logger.String("hex", a.Hex),
				logger.String("flight", a.Flight),
				logger.Float64("altitude", a.ADSB.AltBaro.Float64()),
				logger.Bool("on_ground", a.OnGround),
			)
			if s.wsServer != nil {
				s.wsServer.Broadcast(&websocket.Message{
					Type: "status_update",
					Data: map[string]interface{}{
						"hex":        a.Hex,
						"flight":     a.Flight,
						"altitude":   a.ADSB.AltBaro.Float64(),
						"on_ground":  a.OnGround,
						"timestamp":  time.Now().UTC().Format(time.RFC3339),
						"new_status": "new_aircraft",
					},
				})
			}
		}
	}

	// PRIORITY 1: Handle immediate ground state transitions (takeoff/landing)
	immediatePhaseChanges := s.detectGroundStateTransitions(newAircraft, existingOnGround, currentPhases, adsbTargetIDs)
	if len(immediatePhaseChanges) > 0 {
		err := s.storage.InsertPhaseChangesBatch(immediatePhaseChanges)
		if err != nil {
			s.logger.Error("Failed to insert immediate ground transition phases", logger.Error(err))
		} else {
			s.sendImmediateGroundTransitionAlerts(immediatePhaseChanges)
		}
	}

	// Update all aircraft in the database
	for _, a := range newAircraft {
		s.storage.Upsert(a)
	}

	// Update status of existing aircraft that are no longer active
	s.updateAircraftStatus(activeAircraft)

	// PRIORITY 2: Handle all other phase changes (normal phase detection)
	newPhaseChanges := s.processPhaseChangesBatch(newAircraft, immediatePhaseChanges, currentPhases, adsbTargetIDs, takeoffTimes)

	s.setLastFetchTime(time.Now().UTC())

	// Detect and broadcast changes using enriched newAircraft (no DB read)
	if s.changeDetector != nil && s.broadcastChan != nil {
		// Enrich newAircraft with phase data from batch + new changes
		phaseMap := make(map[string]PhaseChange)
		for hex, phase := range currentPhases {
			if phase != nil {
				phaseMap[hex] = *phase
			}
		}
		// Override with immediate phase changes
		for _, change := range immediatePhaseChanges {
			phaseMap[change.Hex] = PhaseChange{Phase: change.Phase, Timestamp: change.Timestamp}
		}
		// Override with newly detected phase changes
		for _, change := range newPhaseChanges {
			phaseMap[change.Hex] = PhaseChange{Phase: change.Phase, Timestamp: change.Timestamp}
		}
		for _, a := range newAircraft {
			if phase, ok := phaseMap[a.Hex]; ok {
				a.Phase = &PhaseData{Current: []PhaseChange{phase}}
			}
		}

		// Enrich with BSDB and simulation data (in-memory lookups)
		s.enrichWithBSDB(newAircraft)
		s.updateSimulationFields(newAircraft)

		changes := s.changeDetector.DetectChanges(newAircraft)
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

// SetBSDBService sets the BaseStation.sqb lookup service
func (s *Service) SetBSDBService(bsdbService BSDBService) {
	s.bsdbService = bsdbService
}

// enrichWithBSDB enriches aircraft data with BaseStation.sqb information
func (s *Service) enrichWithBSDB(aircraft []*Aircraft) {
	if s.bsdbService == nil {
		return
	}

	for _, a := range aircraft {
		if info := s.bsdbService.Lookup(a.Hex); info != nil {
			a.BSDB = &BSDBData{
				Registration:     info.Registration,
				ICAOTypeCode:     info.ICAOTypeCode,
				OperatorFlagCode: info.OperatorFlagCode,
				Manufacturer:     info.Manufacturer,
				Type:             info.Type,
				RegisteredOwners: info.RegisteredOwners,
			}
		}
	}
}

// UpdateSimulationControls updates the control parameters for a simulated aircraft
func (s *Service) UpdateSimulationControls(hex string, heading, speed, verticalRate float64) error {
	if s.simulationService == nil {
		return fmt.Errorf("simulation service not available")
	}
	return s.simulationService.UpdateControls(hex, heading, speed, verticalRate)
}

// GetAllAircraft returns all aircraft
func (s *Service) GetAllAircraft() []*Aircraft {
	aircraft := s.storage.GetAll()
	s.updateSimulationFields(aircraft)
	s.enrichWithBSDB(aircraft)
	return aircraft
}

// GetAllAircraftWithLastSeenFilter returns aircraft seen within the last N minutes
// This uses database-level filtering for better performance on large databases
func (s *Service) GetAllAircraftWithLastSeenFilter(lastSeenMinutes int) []*Aircraft {
	aircraft := s.storage.GetAllWithLastSeenFilter(lastSeenMinutes)
	s.updateSimulationFields(aircraft)
	s.enrichWithBSDB(aircraft)
	return aircraft
}

// GetAllAircraftMinimal returns aircraft with minimal data (optimized for simple API)
// Skips phase history and takeoff/landing time queries for better performance
func (s *Service) GetAllAircraftMinimal(lastSeenMinutes int) []*Aircraft {
	aircraft := s.storage.GetAllMinimal(lastSeenMinutes)
	s.updateSimulationFields(aircraft)
	s.enrichWithBSDB(aircraft)
	return aircraft
}

// GetAircraftByHex returns an aircraft by its hex ID
func (s *Service) GetAircraftByHex(hex string) (*Aircraft, bool) {
	aircraft, found := s.storage.GetByHex(hex)
	if found && aircraft != nil {
		s.updateSimulationFields([]*Aircraft{aircraft})
		s.enrichWithBSDB([]*Aircraft{aircraft})
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

// GetPhaseHistory returns the phase change history for an aircraft (newest first)
func (s *Service) GetPhaseHistory(hex string) ([]PhaseChange, error) {
	return s.storage.GetPhaseHistory(hex)
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
	s.enrichWithBSDB(aircraft)
	return aircraft
}

// GetFilteredAircraftSimple is a simplified version for backward compatibility
func (s *Service) GetFilteredAircraftSimple(minAltitude, maxAltitude float64, status ...string) []*Aircraft {
	aircraft := s.storage.GetFiltered(minAltitude, maxAltitude, status, nil, nil, nil, nil)
	s.updateSimulationFields(aircraft)
	s.enrichWithBSDB(aircraft)
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

// updateAircraftStatus updates the status of aircraft that are no longer active.
// Uses a targeted query instead of GetAll() — only fetches aircraft that actually need status updates.
func (s *Service) updateAircraftStatus(activeAircraft map[string]bool) {
	now := time.Now().UTC()
	cutoff := now.Add(-s.signalLostTimeout)

	// Build list of active hex codes for exclusion
	activeHexCodes := make([]string, 0, len(activeAircraft))
	for hex := range activeAircraft {
		activeHexCodes = append(activeHexCodes, hex)
	}

	// Targeted query: only aircraft that are active, not currently broadcasting, and stale
	staleAircraft, err := s.storage.GetStaleActiveAircraft(activeHexCodes, cutoff)
	if err != nil {
		s.logger.Error("Failed to get stale active aircraft", logger.Error(err))
		return
	}

	var inactiveAircraft []*Aircraft

	for _, aircraft := range staleAircraft {
		aircraft.Status = "signal_lost"
		s.storage.Upsert(aircraft)

		// Only track aircraft with position for signal-lost landing detection
		hasPosition := aircraft.ADSB != nil && (aircraft.ADSB.Lat != 0 || aircraft.ADSB.Lon != 0)
		if hasPosition {
			inactiveAircraft = append(inactiveAircraft, aircraft)
		}

		timeSinceLastSeen := now.Sub(aircraft.LastSeen)
		s.logger.Info("Aircraft status updated",
			logger.String("hex", aircraft.Hex),
			logger.String("flight", aircraft.Flight),
			logger.String("new_status", "signal_lost"),
			logger.Bool("on_ground", aircraft.OnGround),
			logger.Duration("time_since_last_seen", timeSinceLastSeen),
		)

		// Send WebSocket message (skip for grounded aircraft)
		if s.wsServer != nil && !aircraft.OnGround {
			s.wsServer.Broadcast(&websocket.Message{
				Type: "status_update",
				Data: map[string]interface{}{
					"hex":                  aircraft.Hex,
					"flight":               aircraft.Flight,
					"new_status":           "signal_lost",
					"on_ground":            aircraft.OnGround,
					"time_since_last_seen": timeSinceLastSeen.Seconds(),
					"timestamp":            now.Format(time.RFC3339),
				},
			})
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

// detectGroundStateTransitions detects immediate takeoff/landing events.
// Uses pre-fetched batch data to avoid per-aircraft database queries.
func (s *Service) detectGroundStateTransitions(aircraft []*Aircraft, existingOnGround map[string]bool, currentPhases map[string]*PhaseChange, adsbTargetIDs map[string]*int) []PhaseChangeInsert {
	var immediatePhaseChanges []PhaseChangeInsert
	now := time.Now().UTC()

	for _, a := range aircraft {
		// Check if aircraft exists using pre-fetched batch data
		prevOnGround, found := existingOnGround[a.Hex]
		if !found {
			continue // New aircraft - will be handled by normal phase detection
		}

		// Check if ground state changed
		if prevOnGround != a.OnGround {
			var newPhase string
			var eventType string

			// Get current phase from pre-fetched batch data to prevent rapid T/O ↔ T/D flapping
			currentPhase := currentPhases[a.Hex]

			if !prevOnGround && a.OnGround {
				// Aircraft was airborne, now on ground = LANDING

				// Anti-flapping: Prevent T/O → T/D transition if T/O was recent
				if currentPhase != nil && currentPhase.Phase == "T/O" {
					timeSinceTakeoff := time.Since(currentPhase.Timestamp).Seconds()
					preservationThreshold := float64(s.flightPhasesConfig.PhasePreservationSeconds)
					if timeSinceTakeoff < preservationThreshold {
						s.logger.Warn("Preventing rapid T/O → T/D flapping",
							logger.String("hex", a.Hex),
							logger.String("flight", a.Flight),
							logger.Float64("time_since_takeoff", timeSinceTakeoff),
							logger.Float64("threshold_seconds", preservationThreshold),
							logger.Float64("altitude", a.ADSB.AltBaro.Float64()),
						)
						continue // Skip this transition
					}
				}

				newPhase = "T/D"
				eventType = "landing"

				s.logger.Info("IMMEDIATE LANDING DETECTED",
					logger.String("hex", a.Hex),
					logger.String("flight", a.Flight),
					logger.Bool("was_on_ground", prevOnGround),
					logger.Bool("now_on_ground", a.OnGround),
					logger.Float64("altitude", a.ADSB.AltBaro.Float64()),
					logger.Float64("ground_speed", a.ADSB.GS),
				)

			} else if prevOnGround && !a.OnGround {
				// Aircraft was on ground, now airborne = TAKEOFF

				// Guard against false takeoff from incomplete ADSB data:
				// When an aircraft first appears with no/zero altitude it gets marked on_ground=true.
				// If the next cycle delivers real altitude showing the aircraft well above ground level,
				// that's data becoming available — not an actual takeoff. Skip T/O and let the
				// trajectory tracker assign the correct phase (e.g. CRZ, ARR, UNK).
				alt := a.ADSB.AltBaro.Float64()
				if alt > float64(s.flightPhasesConfig.DepartureAltitudeFt) {
					s.logger.Info("Skipping false takeoff — altitude above departure threshold (incomplete data becoming available)",
						logger.String("hex", a.Hex),
						logger.String("flight", a.Flight),
						logger.Float64("altitude", alt),
						logger.Float64("departure_threshold_ft", float64(s.flightPhasesConfig.DepartureAltitudeFt)),
					)
					continue
				}

				// Anti-flapping: Prevent T/D → T/O transition if T/D was recent
				if currentPhase != nil && currentPhase.Phase == "T/D" {
					timeSinceLanding := time.Since(currentPhase.Timestamp).Seconds()
					preservationThreshold := float64(s.flightPhasesConfig.PhasePreservationSeconds)
					if timeSinceLanding < preservationThreshold {
						s.logger.Warn("Preventing rapid T/D → T/O flapping",
							logger.String("hex", a.Hex),
							logger.String("flight", a.Flight),
							logger.Float64("time_since_landing", timeSinceLanding),
							logger.Float64("threshold_seconds", preservationThreshold),
							logger.Float64("altitude", a.ADSB.AltBaro.Float64()),
						)
						continue // Skip this transition
					}
				}

				newPhase = "T/O"
				eventType = "takeoff"

				s.logger.Info("IMMEDIATE TAKEOFF DETECTED",
					logger.String("hex", a.Hex),
					logger.String("flight", a.Flight),
					logger.Bool("was_on_ground", prevOnGround),
					logger.Bool("now_on_ground", a.OnGround),
					logger.Float64("altitude", a.ADSB.AltBaro.Float64()),
					logger.Float64("ground_speed", a.ADSB.GS),
				)
			}

			if newPhase != "" {
				// Use pre-fetched ADSB target ID
				adsbId := adsbTargetIDs[a.Hex]

				immediatePhaseChanges = append(immediatePhaseChanges, PhaseChangeInsert{
					Hex:       a.Hex,
					Flight:    a.Flight,
					Phase:     newPhase,
					Timestamp: now,
					ADSBId:    adsbId,
					EventType: eventType,
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

		// Check if the trajectory shows the aircraft was descending toward the station.
		// This is a stronger signal than phase alone — if the trajectory buffer confirms
		// a steady descent toward the airport, we can be confident this is a landing.
		trajectoryConfirms := false
		if s.trajectoryTracker != nil {
			trajectoryConfirms = s.trajectoryTracker.WasDescendingTowardStation(aircraft.Hex)
		}

		// Accept aircraft in APP phase, low altitude ARR, or trajectory-confirmed descent
		if currentPhase.Phase != "APP" &&
			!(currentPhase.Phase == "ARR" && aircraft.ADSB.AltBaro.Float64() < 2000) &&
			!trajectoryConfirms {
			continue
		}

		// Check proximity to airport
		distanceFromStation := MetersToNM(Haversine(
			aircraft.ADSB.Lat, aircraft.ADSB.Lon,
			s.stationLat, s.stationLon,
		))

		// If aircraft was close to airport and low altitude when signal lost
		if distanceFromStation <= s.flightPhasesConfig.AirportRangeNM &&
			aircraft.ADSB.AltBaro.Float64() < s.flightPhasesConfig.SignalLostLandingMaxAltFt {

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
				logger.Float64("last_altitude", aircraft.ADSB.AltBaro.Float64()),
				logger.Float64("distance_from_airport", distanceFromStation),
			)
		}
	}

	return landingPhaseChanges
}

// processPhaseChangesBatch handles phase detection using pre-fetched batch data.
// Returns any new phase changes that were inserted (for downstream enrichment).
func (s *Service) processPhaseChangesBatch(aircraft []*Aircraft, immediatePhaseChanges []PhaseChangeInsert,
	currentPhases map[string]*PhaseChange, adsbTargetIDs map[string]*int, takeoffTimes map[string]*time.Time) []PhaseChangeInsert {
	if !s.flightPhasesConfig.Enabled {
		return nil
	}

	if len(aircraft) == 0 {
		return nil
	}

	// Create map of aircraft that just had immediate ground transitions
	immediateTransitions := make(map[string]string) // hex -> phase
	for _, change := range immediatePhaseChanges {
		immediateTransitions[change.Hex] = change.Phase
	}

	// Process each aircraft using pre-fetched batch data
	var phaseChanges []PhaseChangeInsert
	for _, a := range aircraft {
		// Skip aircraft that just had immediate ground transitions
		if _, hasImmediate := immediateTransitions[a.Hex]; hasImmediate {
			continue
		}

		currentPhase := currentPhases[a.Hex]
		newPhase := s.trajectoryTracker.DeterminePhase(a, currentPhase, takeoffTimes[a.Hex])

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

	// Batch insert all phase changes
	if len(phaseChanges) > 0 {
		err := s.storage.InsertPhaseChangesBatch(phaseChanges)
		if err != nil {
			s.logger.Error("Failed to insert phase changes batch", logger.Error(err))
			return nil
		}

		// Send WebSocket alerts and log changes
		s.sendPhaseChangeAlerts(phaseChanges, currentPhases)
	}

	return phaseChanges
}

// evaluatePhaseChange applies phase stability rules and determines if a phase change
// should be inserted into the database.
//
// The trajectory-based phase detection (TrajectoryTracker.DeterminePhase) already provides
// noise-resistant classification through trajectory window analysis, so airborne flapping
// prevention is no longer needed. This function handles only:
//
//  1. T/O preservation — Keep takeoff phase visible for PhasePreservationSeconds
//  2. T/D post-landing flow — Transition T/D → TAX or T/D → NEW based on ground speed
//  3. Ground phase protection — Prevent premature TAX→NEW and T/D→NEW transitions
//  4. Inactive aircraft timeout — Revert long-parked aircraft to NEW
func (s *Service) evaluatePhaseChange(aircraft *Aircraft, latestPhase *PhaseChange, newPhase string) (string, bool) {
	currentPhase := newPhase

	// ── T/D post-landing flow ────────────────────────────────────────
	// After landing, guide the aircraft through T/D → TAX → NEW gracefully.
	if latestPhase != nil && latestPhase.Phase == "T/D" {
		if currentPhase == "NEW" {
			if aircraft.OnGround && aircraft.ADSB != nil &&
				aircraft.ADSB.GS >= float64(s.flightPhasesConfig.TaxiingMinSpeedKts) &&
				aircraft.ADSB.GS <= float64(s.flightPhasesConfig.TaxiingMaxSpeedKts) {
				currentPhase = "TAX"
			} else {
				// Keep T/D visible for the preservation period
				timeSinceLanding := time.Since(latestPhase.Timestamp).Seconds()
				if timeSinceLanding < float64(s.flightPhasesConfig.PhasePreservationSeconds) {
					currentPhase = "T/D"
				} else if aircraft.ADSB != nil && aircraft.ADSB.GS >= float64(s.flightPhasesConfig.TaxiingMinSpeedKts) {
					currentPhase = "TAX"
				} else {
					currentPhase = "NEW"
				}
			}
		}
	}

	// ── T/O preservation ─────────────────────────────────────────────
	// Keep takeoff phase visible for at least PhasePreservationSeconds so
	// controllers can see it in the UI before it transitions to DEP.
	if latestPhase != nil && latestPhase.Phase == "T/O" {
		timeSinceTakeoff := time.Since(latestPhase.Timestamp).Seconds()
		if timeSinceTakeoff < float64(s.flightPhasesConfig.PhasePreservationSeconds) {
			currentPhase = "T/O"
		}
	}

	// ── Determine if a phase change should be recorded ───────────────
	var shouldInsert bool

	if latestPhase == nil {
		// Brand new aircraft — record initial phase
		shouldInsert = true
	} else if latestPhase.Phase != currentPhase {
		// Phase changed — check for premature ground transitions
		if currentPhase == "NEW" && (latestPhase.Phase == "T/D" || latestPhase.Phase == "TAX") {
			timeSinceLastPhase := time.Since(latestPhase.Timestamp).Seconds()
			if timeSinceLastPhase < float64(s.flightPhasesConfig.PhaseTransitionTimeoutSeconds) {
				// Too soon — keep the current ground phase
				currentPhase = latestPhase.Phase
			} else {
				shouldInsert = true
			}
		} else {
			shouldInsert = true
		}
	} else {
		// Phase unchanged — check inactive aircraft timeout
		if latestPhase.Phase != "NEW" {
			timeSinceLastPhase := time.Since(latestPhase.Timestamp).Seconds()
			if timeSinceLastPhase > float64(s.flightPhasesConfig.PhaseChangeTimeoutSeconds) {
				currentPhase = "NEW"
				shouldInsert = true
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
				logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
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
				logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
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
				"altitude":   aircraft.ADSB.AltBaro.Float64(),
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
				logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
				logger.Bool("on_ground", aircraft.OnGround),
			)

			// T/O phase change message is sent by the main phase change handler
		} else if change.Phase == "T/D" {
			s.logger.Info("Aircraft LANDED",
				logger.String("hex", aircraft.Hex),
				logger.String("flight", aircraft.Flight),
				logger.String("transition", previousPhase+" → T/D"),
				logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
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
				logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
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
					"altitude":   aircraft.ADSB.AltBaro.Float64(),
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
				logger.Float64("altitude", aircraft.ADSB.AltBaro.Float64()),
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
					"altitude":   aircraft.ADSB.AltBaro.Float64(),
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

	for _, raw := range rawData.Aircraft {
		// Skip aircraft without a hex identifier (unusable data)
		if raw.Hex == "" {
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

		// Sensor validation and OnGround determination handled by fetchAndProcess using batch data

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
			Status:             aircraftStatus,
			LastSeen:           now.Add(-time.Duration(raw.Seen) * time.Second),
			ADSB:               &raw,
			IsSimulated:        isSimulated,
			SimulationControls: simulationControls,
		}

		// Calculate future positions if we have the necessary data
		if raw.Lat != 0 && raw.Lon != 0 && raw.AltBaro.Float64() != 0 {
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
					raw.AltBaro.Float64(),
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
