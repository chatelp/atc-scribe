package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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

// Handler contains the API handlers
type Handler struct {
	adsbService          *adsb.Service
	frequenciesService   *frequencies.Service
	weatherService       *weather.Service
	atcChatService       *atcchat.Service
	simulationService    *simulation.Service
	config               *config.Config
	logger               *logger.Logger
	wsServer             *websocket.Server
	transcriptionStorage *sqlite.TranscriptionStorage
	clearanceStorage     *sqlite.ClearanceStorage
}

// NewHandler creates a new API handler
func NewHandler(adsbService *adsb.Service, frequenciesService *frequencies.Service, weatherService *weather.Service, atcChatService *atcchat.Service, simulationService *simulation.Service, config *config.Config, logger *logger.Logger, wsServer *websocket.Server, transcriptionStorage *sqlite.TranscriptionStorage, clearanceStorage *sqlite.ClearanceStorage) *Handler {
	return &Handler{
		adsbService:          adsbService,
		frequenciesService:   frequenciesService,
		weatherService:       weatherService,
		atcChatService:       atcChatService,
		simulationService:    simulationService,
		config:               config,
		logger:               logger.Named("api-handler"),
		wsServer:             wsServer,
		transcriptionStorage: transcriptionStorage,
		clearanceStorage:     clearanceStorage,
	}
}

// GetAllAircraft returns all aircraft
func (h *Handler) GetAllAircraft(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Debug("Starting GetAllAircraft API call")

	// Parse query parameters
	minAltitude, maxAltitude, callsign, status, lastSeenMinutes,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore, distanceNM,
		refLat, refLon, refHex, refFlight, excludeOtherAirportsGrounded := parseAircraftFilters(r)

	// Get aircraft data
	dataFetchStart := time.Now()
	var aircraft []*adsb.Aircraft
	if minAltitude > 0 || maxAltitude < 60000 || len(status) > 0 ||
		tookOffAfter != nil || tookOffBefore != nil ||
		landedAfter != nil || landedBefore != nil {
		// Use the enhanced GetFiltered method with date filters
		aircraft = h.adsbService.GetFilteredAircraft(
			minAltitude, maxAltitude,
			status,
			tookOffAfter, tookOffBefore, landedAfter, landedBefore,
		)
	} else {
		aircraft = h.adsbService.GetAllAircraft()
	}

	dataFetchDuration := time.Since(dataFetchStart)
	h.logger.Debug("Aircraft data fetch completed",
		logger.Duration("duration", dataFetchDuration),
		logger.Int("aircraft_count", len(aircraft)))

	// Filter by callsign if provided
	if callsign != "" {
		filtered := make([]*adsb.Aircraft, 0)
		for _, a := range aircraft {
			if strings.Contains(strings.ToUpper(a.Flight), strings.ToUpper(callsign)) {
				filtered = append(filtered, a)
			}
		}
		aircraft = filtered
	}

	// Filter by last seen time if provided
	if lastSeenMinutes > 0 {
		now := time.Now().UTC() // Use UTC for cutoff time
		cutoffTime := now.Add(-time.Duration(lastSeenMinutes) * time.Minute)

		filtered := make([]*adsb.Aircraft, 0)
		for _, a := range aircraft {
			if a.LastSeen.After(cutoffTime) {
				filtered = append(filtered, a)
			}
		}
		aircraft = filtered
	}

	// Apply distance filter if provided
	if distanceNM > 0 {
		var refLatitude, refLongitude float64
		var refHeading, refAltitude float64
		var err error
		var refType string
		var refAircraft *adsb.Aircraft

		// Determine which reference to use (in order of priority)
		if refLat != 0 && refLon != 0 {
			// Use provided coordinates
			refLatitude, refLongitude = refLat, refLon
			refType = "coordinates"
			err = nil
		} else if refHex != "" {
			// Use aircraft hex code
			refAircraft, err = h.getRefAircraft(refHex)
			if err == nil && refAircraft != nil && refAircraft.ADSB != nil {
				refLatitude = refAircraft.ADSB.Lat
				refLongitude = refAircraft.ADSB.Lon
				refHeading = refAircraft.ADSB.TrueHeading
				if refHeading == 0 {
					refHeading = refAircraft.ADSB.Track // Use track if true heading is not available
				}
				refAltitude = refAircraft.ADSB.AltBaro
			}
			refType = "hex"
		} else if refFlight != "" {
			// Use flight number
			refLatitude, refLongitude, err = h.getFlightCoordinates(refFlight)
			refType = "flight"
		} else {
			// No valid reference provided
			err = fmt.Errorf("no valid reference coordinates provided")
			refType = "none"
		}

		if err == nil {
			filtered := make([]*adsb.Aircraft, 0)
			for _, a := range aircraft {
				// Skip aircraft with no position data
				if a.ADSB == nil || (a.ADSB.Lat == 0 && a.ADSB.Lon == 0) {
					continue
				}

				// Skip grounded aircraft for proximity queries
				if a.OnGround {
					continue
				}

				// Skip the reference aircraft itself
				if refHex != "" && a.Hex == refHex {
					continue
				}

				// For proximity queries, only include active aircraft
				if a.Status != "active" {
					continue
				}

				// Calculate distance
				distMeters := adsb.Haversine(a.ADSB.Lat, a.ADSB.Lon, refLatitude, refLongitude)
				distNM := adsb.MetersToNM(distMeters)
				distNM = math.Round(distNM*10) / 10 // Round to 1 decimal place

				// Add to filtered list if within range
				if distNM <= distanceNM {
					// For proximity queries, we need to distinguish between:
					// 1. Distance from station (regular distance field)
					// 2. Distance from reference aircraft (relative distance field)

					// Calculate distance from station for each aircraft
					if a.ADSB != nil && a.ADSB.Lat != 0 && a.ADSB.Lon != 0 {
						stationDistMeters := adsb.Haversine(a.ADSB.Lat, a.ADSB.Lon, h.config.Station.Latitude, h.config.Station.Longitude)
						stationDistNM := adsb.MetersToNM(stationDistMeters)
						stationDistNM = math.Round(stationDistNM*10) / 10 // Round to 1 decimal place
						a.Distance = &stationDistNM
					}

					// Store the calculated relative distance
					a.RelativeDistance = &distNM

					// If we have a reference aircraft with heading, calculate relative bearing
					if refAircraft != nil && refHeading > 0 {
						bearing := adsb.CalculateRelativeBearing(
							refLatitude, refLongitude, refHeading,
							a.ADSB.Lat, a.ADSB.Lon)
						a.RelativeBearing = &bearing

						// Calculate relative altitude
						if refAltitude > 0 && a.ADSB.AltBaro > 0 {
							relAlt := a.ADSB.AltBaro - refAltitude
							a.RelativeAlt = &relAlt
						}
					}

					filtered = append(filtered, a)
				}
			}

			// Sort aircraft by relative distance (ascending)
			sort.Slice(filtered, func(i, j int) bool {
				// Handle nil cases (shouldn't happen, but just in case)
				if filtered[i].RelativeDistance == nil {
					return false
				}
				if filtered[j].RelativeDistance == nil {
					return true
				}
				return *filtered[i].RelativeDistance < *filtered[j].RelativeDistance
			})

			aircraft = filtered
		} else {
			h.logger.Error("Failed to resolve reference coordinates",
				logger.Error(err),
				logger.String("reference_type", refType),
				logger.String("ref_hex", refHex),
				logger.String("ref_flight", refFlight))
		}
	}

	// Apply exclude_other_airports_grounded filter if requested
	if excludeOtherAirportsGrounded {
		filtered := make([]*adsb.Aircraft, 0)
		airportRangeNM := h.config.Station.AirportRangeNM
		if airportRangeNM == 0 {
			airportRangeNM = 5.0 // Default to 5.0 NM if not configured
		}

		for _, a := range aircraft {
			// Include all aircraft that are not on ground, or grounded aircraft within airport range
			if !a.OnGround {
				filtered = append(filtered, a)
			} else if a.ADSB != nil && a.ADSB.Lat != 0 && a.ADSB.Lon != 0 {
				// Calculate distance from station for grounded aircraft
				distMeters := adsb.Haversine(a.ADSB.Lat, a.ADSB.Lon, h.config.Station.Latitude, h.config.Station.Longitude)
				distNM := adsb.MetersToNM(distMeters)
				if distNM <= airportRangeNM {
					filtered = append(filtered, a)
				}
			}
		}
		aircraft = filtered
	}

	// Update zero values with last non-zero values from position history
	for _, a := range aircraft {
		updateZeroValuesFromHistory(a)

		// Calculate distance from station for each aircraft
		if a.ADSB != nil && a.ADSB.Lat != 0 && a.ADSB.Lon != 0 {
			distMeters := adsb.Haversine(a.ADSB.Lat, a.ADSB.Lon, h.config.Station.Latitude, h.config.Station.Longitude)
			distNM := adsb.MetersToNM(distMeters)
			distNM = math.Round(distNM*10) / 10 // Round to 1 decimal place
			a.Distance = &distNM
		}

		// Check if this is a proximity query (ref_hex or ref_lat/ref_lon with distance_nm)
		isProximityQuery := (refHex != "" || (refLat != 0 && refLon != 0)) && distanceNM > 0

		// For proximity queries, don't include history data to reduce payload size
		if isProximityQuery {
			a.History = nil
		} else {
			// Limit the number of positions returned in the API response
			if len(a.History) > h.config.Storage.MaxPositionsInAPI {
				// Keep only the most recent positions up to the limit
				a.History = a.History[len(a.History)-h.config.Storage.MaxPositionsInAPI:]
			}
		}

		// Future array is now populated by the prediction algorithm
	}

	// Calculate counts by ground/air and active/total
	groundActive := 0
	groundTotal := 0
	airActive := 0
	airTotal := 0

	for _, a := range aircraft {
		if a.OnGround {
			// Ground aircraft
			groundTotal++
			if a.Status == "active" {
				groundActive++
			}
		} else {
			// Air aircraft
			airTotal++
			if a.Status == "active" {
				airActive++
			}
		}
	}

	// Populate clearances for each aircraft
	for _, aircraft := range aircraft {
		clearances, err := h.clearanceStorage.GetClearancesByCallsign(aircraft.Flight, 10) // Last 10 clearances
		if err != nil {
			h.logger.Error("Failed to get clearances for aircraft",
				logger.String("callsign", aircraft.Flight),
				logger.Error(err))
			continue
		}

		// Convert to API format
		aircraft.Clearances = h.convertClearancesToAPIFormat(clearances)
	}

	// Create response
	response := adsb.AircraftResponse{
		Timestamp: time.Now().UTC(), // Use UTC for response timestamp
		Count:     len(aircraft),
		Counts: adsb.AircraftCounts{
			GroundActive: groundActive,
			GroundTotal:  groundTotal,
			AirActive:    airActive,
			AirTotal:     airTotal,
		},
		Aircraft: aircraft,
	}

	// Write response
	WriteJSON(w, http.StatusOK, response)

	totalDuration := time.Since(start)
	h.logger.Debug("GetAllAircraft API call completed",
		logger.Duration("total_duration", totalDuration),
		logger.Int("final_aircraft_count", len(aircraft)))
}

// GetAircraftByHex returns an aircraft by its hex ID
func (h *Handler) GetAircraftByHex(w http.ResponseWriter, r *http.Request) {
	// Get hex ID from URL
	hex := chi.URLParam(r, "id")
	if hex == "" {
		http.Error(w, "Missing aircraft ID", http.StatusBadRequest)
		return
	}

	// Get aircraft data
	aircraft, found := h.adsbService.GetAircraftByHex(hex)
	if !found {
		http.Error(w, "Aircraft not found", http.StatusNotFound)
		return
	}

	// Update zero values with last non-zero values from position history
	updateZeroValuesFromHistory(aircraft)

	// Calculate distance from station
	if aircraft.ADSB != nil && aircraft.ADSB.Lat != 0 && aircraft.ADSB.Lon != 0 {
		distMeters := haversine(aircraft.ADSB.Lat, aircraft.ADSB.Lon, h.config.Station.Latitude, h.config.Station.Longitude)
		distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
		aircraft.Distance = &distNM
	}

	// Write response
	WriteJSON(w, http.StatusOK, aircraft)
}

// GetAircraftTracks returns both history and future tracks for an aircraft
func (h *Handler) GetAircraftTracks(w http.ResponseWriter, r *http.Request) {
	// Get hex ID from URL
	hex := chi.URLParam(r, "id")
	if hex == "" {
		http.Error(w, "Missing aircraft ID", http.StatusBadRequest)
		return
	}

	// Get limit parameter (default to 1000)
	limitStr := r.URL.Query().Get("limit")
	limit := 1000
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get aircraft data for basic info
	aircraft, found := h.adsbService.GetAircraftByHex(hex)
	if !found {
		http.Error(w, "Aircraft not found", http.StatusNotFound)
		return
	}

	// Get position history with limit
	history, err := h.adsbService.GetPositionHistoryWithLimit(hex, limit)
	if err != nil {
		h.logger.Error("Failed to get position history",
			logger.Error(err),
			logger.String("hex", hex),
			logger.Int("limit", limit))
		http.Error(w, "Failed to get position history", http.StatusInternalServerError)
		return
	}

	// Calculate distance for each historical position
	for i := range history {
		if history[i].Lat != 0 && history[i].Lon != 0 {
			distMeters := haversine(history[i].Lat, history[i].Lon, h.config.Station.Latitude, h.config.Station.Longitude)
			distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
			history[i].Distance = &distNM
		}
	}

	// Calculate current distance from station
	var distance *float64
	if aircraft.ADSB != nil && aircraft.ADSB.Lat != 0 && aircraft.ADSB.Lon != 0 {
		distMeters := haversine(aircraft.ADSB.Lat, aircraft.ADSB.Lon, h.config.Station.Latitude, h.config.Station.Longitude)
		distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
		distance = &distNM
	}

	// Calculate distance for each future position
	future := aircraft.Future
	for i := range future {
		if future[i].Lat != 0 && future[i].Lon != 0 {
			distMeters := haversine(future[i].Lat, future[i].Lon, h.config.Station.Latitude, h.config.Station.Longitude)
			distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
			future[i].Distance = &distNM
		}
	}

	// Create response
	response := adsb.AircraftTracksResponse{
		Hex:      aircraft.Hex,
		Flight:   aircraft.Flight,
		Distance: distance,
		History:  history,
		Future:   future,
	}

	// Debug: Print some mag_heading values from history
	h.logger.Debug("GetAircraftTracks response",
		logger.String("hex", hex),
		logger.Int("history_count", len(response.History)),
		logger.Int("future_count", len(response.Future)))

	if len(response.History) > 0 {
		for i, pos := range response.History[:min(3, len(response.History))] {
			h.logger.Debug("History position",
				logger.Int("index", i),
				logger.Float64("mag_heading", pos.MagHeading),
				logger.Float64("true_heading", pos.TrueHeading),
				logger.String("timestamp", pos.Timestamp.Format(time.RFC3339)))
		}
	}

	// Write response
	WriteJSON(w, http.StatusOK, response)
}

// GetHealth returns the health status of the API
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	lastFetch, status := h.adsbService.GetStatus()

	response := map[string]interface{}{
		"status":         status,
		"last_fetch":     lastFetch,
		"aircraft_count": len(h.adsbService.GetAllAircraft()),
	}

	WriteJSON(w, http.StatusOK, response)
}

// GetConfig returns the public configuration
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	// Create a sanitized config with only public values
	publicConfig := map[string]interface{}{
		"adsb": map[string]interface{}{
			"fetch_interval_seconds":     h.config.ADSB.FetchIntervalSecs,
			"websocket_aircraft_updates": h.config.ADSB.WebSocketAircraftUpdates,
		},
		"storage": map[string]interface{}{
			"sqlite_base_path":     h.config.Storage.SQLiteBasePath,
			"max_positions_in_api": h.config.Storage.MaxPositionsInAPI,
		},
		"frequencies": map[string]interface{}{
			"buffer_size_kb":          h.config.Frequencies.BufferSizeKB,
			"stream_timeout_secs":     h.config.Frequencies.StreamTimeoutSecs,
			"reconnect_interval_secs": h.config.Frequencies.ReconnectIntervalSecs,
		},
		"atc_chat": map[string]interface{}{
			"enabled": h.config.ATCChat.Enabled,
		},
	}

	WriteJSON(w, http.StatusOK, publicConfig)
}

// GetStationConfig returns the station configuration (latitude, longitude, elevation)
func (h *Handler) GetStationConfig(w http.ResponseWriter, r *http.Request) {
	// Get effective coordinates (override if set, otherwise config)
	effectiveLat, effectiveLon := h.adsbService.GetEffectiveStationCoords()

	stationCfg := struct {
		Latitude      float64     `json:"latitude"`
		Longitude     float64     `json:"longitude"`
		ElevationFeet int         `json:"elevation_feet"`
		AirportCode   string      `json:"airport_code"`
		Runways       interface{} `json:"runways,omitempty"`
		FetchErrors   []string    `json:"fetch_errors,omitempty"`
		// Weather configuration flags
		FetchMETAR  bool `json:"fetch_metar"`
		FetchTAF    bool `json:"fetch_taf"`
		FetchNOTAMs bool `json:"fetch_notams"`
		// Station override information
		OverrideActive bool `json:"override_active"`
	}{
		Latitude:       effectiveLat,
		Longitude:      effectiveLon,
		ElevationFeet:  h.config.Station.ElevationFeet,
		AirportCode:    h.config.Station.AirportCode,
		FetchMETAR:     h.config.Weather.FetchMETAR,
		FetchTAF:       h.config.Weather.FetchTAF,
		FetchNOTAMs:    h.config.Weather.FetchNOTAMs,
		OverrideActive: effectiveLat != h.config.Station.Latitude || effectiveLon != h.config.Station.Longitude,
	}

	// Track if we have any data fetch failures
	var fetchErrors []string

	// Fetch runway data if path is configured
	if h.config.Station.RunwaysDBPath != "" {
		runwayData, err := h.fetchRunwayData(h.config.Station.RunwaysDBPath)
		if err == nil {
			stationCfg.Runways = runwayData
		} else {
			h.logger.Error("Failed to fetch runway data",
				logger.String("path", h.config.Station.RunwaysDBPath),
				logger.Error(err))
			fetchErrors = append(fetchErrors, fmt.Sprintf("Runways: %s", err.Error()))

			// Set empty object instead of null for better client handling
			stationCfg.Runways = map[string]interface{}{}
		}
	}

	// Add fetch errors to response if any occurred
	if len(fetchErrors) > 0 {
		stationCfg.FetchErrors = fetchErrors
	}

	WriteJSON(w, http.StatusOK, stationCfg)
}

// SetStationOverride sets or clears station coordinate override
func (h *Handler) SetStationOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Latitude  *float64 `json:"latitude"`  // nil to clear override
		Longitude *float64 `json:"longitude"` // nil to clear override
	}

	// Parse request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse station override request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate coordinates if provided
	if req.Latitude != nil && req.Longitude != nil {
		lat, lon := *req.Latitude, *req.Longitude

		// Basic coordinate validation
		if lat < -90 || lat > 90 {
			http.Error(w, "Invalid latitude: must be between -90 and 90", http.StatusBadRequest)
			return
		}
		if lon < -180 || lon > 180 {
			http.Error(w, "Invalid longitude: must be between -180 and 180", http.StatusBadRequest)
			return
		}

		// Set override coordinates
		h.adsbService.SetStationOverride(lat, lon)
		h.logger.Info("Station override coordinates set via API",
			logger.Float64("latitude", lat),
			logger.Float64("longitude", lon))

		response := struct {
			Success   bool    `json:"success"`
			Message   string  `json:"message"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}{
			Success:   true,
			Message:   "Station override coordinates set successfully",
			Latitude:  lat,
			Longitude: lon,
		}
		WriteJSON(w, http.StatusOK, response)
	} else {
		// Clear override coordinates
		h.adsbService.ClearStationOverride()
		h.logger.Info("Station override coordinates cleared via API")

		response := struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}{
			Success: true,
			Message: "Station override coordinates cleared successfully",
		}
		WriteJSON(w, http.StatusOK, response)
	}
}

// GetWeatherData returns cached weather data (METAR, TAF, NOTAMs)
func (h *Handler) GetWeatherData(w http.ResponseWriter, r *http.Request) {
	if h.weatherService == nil {
		// Weather service not available
		weatherData := struct {
			METAR       interface{} `json:"metar,omitempty"`
			TAF         interface{} `json:"taf,omitempty"`
			NOTAMs      interface{} `json:"notams,omitempty"`
			LastUpdated string      `json:"last_updated"`
			FetchErrors []string    `json:"fetch_errors,omitempty"`
		}{
			LastUpdated: time.Now().Format(time.RFC3339),
			FetchErrors: []string{"Weather service not available"},
		}
		WriteJSON(w, http.StatusOK, weatherData)
		return
	}

	// Get weather data from the service
	weatherData := h.weatherService.GetWeatherData()
	WriteJSON(w, http.StatusOK, weatherData)
}

// fetchRunwayData loads runway data from the specified file and calculates extended centerlines
func (h *Handler) fetchRunwayData(filePath string) (interface{}, error) {
	// Read the runway data file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runway data file: %w", err)
	}

	// Parse the JSON data
	var runwayData struct {
		Airport          string `json:"airport"`
		RunwayThresholds map[string]map[string]struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"runway_thresholds"`
	}
	if err := json.Unmarshal(data, &runwayData); err != nil {
		return nil, fmt.Errorf("failed to parse runway data: %w", err)
	}

	// Define the point structure with distance field
	type Point struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Distance  float64 `json:"distance,omitempty"` // Include distance for markers
	}

	// Create the response structure with extended centerlines
	response := struct {
		Airport          string `json:"airport"`
		RunwayThresholds map[string]map[string]struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"runway_thresholds"`
		RunwayExtensions map[string]map[string][]Point `json:"runway_extensions"`
	}{
		Airport:          runwayData.Airport,
		RunwayThresholds: runwayData.RunwayThresholds,
		RunwayExtensions: make(map[string]map[string][]Point),
	}

	// Calculate extended centerlines for each runway
	for runwayID, thresholds := range runwayData.RunwayThresholds {
		response.RunwayExtensions[runwayID] = make(map[string][]Point)

		// Process each end of the runway
		for endID, threshold := range thresholds {
			// Find the opposite end
			var oppositeThreshold struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}
			for otherEndID, otherThreshold := range thresholds {
				if otherEndID != endID {
					oppositeThreshold = otherThreshold
					break
				}
			}

			// Calculate the bearing from this threshold to the opposite threshold
			bearing := calculateBearing(
				threshold.Latitude, threshold.Longitude,
				oppositeThreshold.Latitude, oppositeThreshold.Longitude,
			)

			// Calculate the opposite bearing (for the extension)
			oppositeBearing := math.Mod(bearing+180, 360)

			// Create points for the extended centerline (10 nm from threshold)
			extensionPoints := []Point{
				// Start with the threshold point
				{
					Latitude:  threshold.Latitude,
					Longitude: threshold.Longitude,
					Distance:  0.0,
				},
			}

			// Get the configured runway extension length (default to 5 nm if not set)
			extensionLengthNM := 5.0
			if h.config.Station.RunwayExtensionLengthNM > 0 {
				extensionLengthNM = h.config.Station.RunwayExtensionLengthNM
			}

			// Add points at 1 nm intervals up to the configured length
			for distance := 1.0; distance <= extensionLengthNM; distance += 1.0 {
				lat, lon := calculateDestinationPoint(
					threshold.Latitude, threshold.Longitude,
					oppositeBearing, distance,
				)
				extensionPoints = append(extensionPoints, Point{
					Latitude:  lat,
					Longitude: lon,
					Distance:  distance,
				})
			}

			// Add the extension points to the response
			response.RunwayExtensions[runwayID][endID] = extensionPoints
		}
	}

	return response, nil
}

// calculateBearing calculates the initial bearing from point 1 to point 2
func calculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	// Convert to radians
	lat1 = lat1 * math.Pi / 180
	lon1 = lon1 * math.Pi / 180
	lat2 = lat2 * math.Pi / 180
	lon2 = lon2 * math.Pi / 180

	// Calculate bearing
	y := math.Sin(lon2-lon1) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
	bearing := math.Atan2(y, x) * 180 / math.Pi

	// Normalize to 0-360
	return math.Mod(math.Mod(bearing, 360)+360, 360)
}

// calculateDestinationPoint calculates a destination point given a starting point, bearing, and distance
func calculateDestinationPoint(lat, lon, bearing, distanceNM float64) (float64, float64) {
	// Convert to radians
	lat = lat * math.Pi / 180
	lon = lon * math.Pi / 180
	bearing = bearing * math.Pi / 180

	// Earth radius in nautical miles
	earthRadius := 3440.065 // 6371 km / 1.852 km/nm

	// Calculate destination point
	distRatio := distanceNM / earthRadius
	lat2 := math.Asin(math.Sin(lat)*math.Cos(distRatio) + math.Cos(lat)*math.Sin(distRatio)*math.Cos(bearing))
	lon2 := lon + math.Atan2(
		math.Sin(bearing)*math.Sin(distRatio)*math.Cos(lat),
		math.Cos(distRatio)-math.Sin(lat)*math.Sin(lat2),
	)

	// Convert back to degrees
	lat2 = lat2 * 180 / math.Pi
	lon2 = lon2 * 180 / math.Pi

	return lat2, lon2
}

// fetchMetarData fetches METAR data from the Windy API with retry logic
func (h *Handler) fetchMetarData(airportCode string) (interface{}, error) {
	url := fmt.Sprintf("https://node.windy.com/airports/metar/%s", airportCode)

	// Create a new HTTP client with increased timeout
	client := &http.Client{
		Timeout: 10 * time.Second, // Increased from 5 to 10 seconds
	}

	// Retry configuration
	maxRetries := 2
	var lastErr error
	var metarData interface{}

	// Try to fetch with retries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff between retries
			backoffDuration := time.Duration(500*(1<<uint(attempt-1))) * time.Millisecond
			h.logger.Info("Retrying METAR data fetch",
				logger.String("airport", airportCode),
				logger.Int("attempt", attempt),
				logger.String("backoff", backoffDuration.String()))
			time.Sleep(backoffDuration)
		}

		// Make the request
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("error making request to Windy API: %w", err)
			h.logger.Warn("METAR API request failed, may retry",
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Ensure response body is closed
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			h.logger.Warn("METAR API returned non-OK status, may retry",
				logger.String("airport", airportCode),
				logger.Int("status_code", resp.StatusCode),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Read and parse the response
		if err := json.NewDecoder(resp.Body).Decode(&metarData); err != nil {
			lastErr = fmt.Errorf("error decoding METAR data: %w", err)
			h.logger.Warn("Failed to decode METAR data, may retry",
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Success - return the data
		if attempt > 0 {
			h.logger.Info("Successfully fetched METAR data after retries",
				logger.String("airport", airportCode),
				logger.Int("attempts_needed", attempt+1))
		}
		return metarData, nil
	}

	// If we get here, all attempts failed
	h.logger.Error("All attempts to fetch METAR data failed",
		logger.String("airport", airportCode),
		logger.Error(lastErr),
		logger.Int("max_attempts", maxRetries+1))
	return nil, lastErr
}

// fetchTAFData fetches TAF data from the Windy API with retry logic
func (h *Handler) fetchTAFData(airportCode string) (interface{}, error) {
	url := fmt.Sprintf("https://node.windy.com/airports/taf/%s", airportCode)

	// Create a new HTTP client with increased timeout
	client := &http.Client{
		Timeout: 10 * time.Second, // Increased from 5 to 10 seconds
	}

	// Retry configuration
	maxRetries := 2
	var lastErr error
	var tafData interface{}

	// Try to fetch with retries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff between retries
			backoffDuration := time.Duration(500*(1<<uint(attempt-1))) * time.Millisecond
			h.logger.Info("Retrying TAF data fetch",
				logger.String("airport", airportCode),
				logger.Int("attempt", attempt),
				logger.String("backoff", backoffDuration.String()))
			time.Sleep(backoffDuration)
		}

		// Make the request
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("error making request to Windy API: %w", err)
			h.logger.Warn("TAF API request failed, may retry",
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Ensure response body is closed
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			h.logger.Warn("TAF API returned non-OK status, may retry",
				logger.String("airport", airportCode),
				logger.Int("status_code", resp.StatusCode),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Read and parse the response
		if err := json.NewDecoder(resp.Body).Decode(&tafData); err != nil {
			lastErr = fmt.Errorf("error decoding TAF data: %w", err)
			h.logger.Warn("Failed to decode TAF data, may retry",
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Success - return the data
		if attempt > 0 {
			h.logger.Info("Successfully fetched TAF data after retries",
				logger.String("airport", airportCode),
				logger.Int("attempts_needed", attempt+1))
		}
		return tafData, nil
	}

	// If we get here, all attempts failed
	h.logger.Error("All attempts to fetch TAF data failed",
		logger.String("airport", airportCode),
		logger.Error(lastErr),
		logger.Int("max_attempts", maxRetries+1))
	return nil, lastErr
}

// fetchNOTAMData fetches NOTAM data from the Windy API with retry logic
func (h *Handler) fetchNOTAMData(airportCode string) (interface{}, error) {
	url := fmt.Sprintf("https://node.windy.com/airports/notams/%s", airportCode)

	// Create a new HTTP client with increased timeout
	client := &http.Client{
		Timeout: 10 * time.Second, // Increased from 5 to 10 seconds
	}

	// Retry configuration
	maxRetries := 2
	var lastErr error
	var notamData interface{}

	// Try to fetch with retries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff between retries
			backoffDuration := time.Duration(500*(1<<uint(attempt-1))) * time.Millisecond
			h.logger.Info("Retrying NOTAM data fetch",
				logger.String("airport", airportCode),
				logger.Int("attempt", attempt),
				logger.String("backoff", backoffDuration.String()))
			time.Sleep(backoffDuration)
		}

		// Make the request
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("error making request to Windy API: %w", err)
			h.logger.Warn("NOTAM API request failed, may retry",
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Ensure response body is closed
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			h.logger.Warn("NOTAM API returned non-OK status, may retry",
				logger.String("airport", airportCode),
				logger.Int("status_code", resp.StatusCode),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Read and parse the response
		if err := json.NewDecoder(resp.Body).Decode(&notamData); err != nil {
			lastErr = fmt.Errorf("error decoding NOTAM data: %w", err)
			h.logger.Warn("Failed to decode NOTAM data, may retry",
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", maxRetries+1))
			continue
		}

		// Success - return the data
		if attempt > 0 {
			h.logger.Info("Successfully fetched NOTAM data after retries",
				logger.String("airport", airportCode),
				logger.Int("attempts_needed", attempt+1))
		}
		return notamData, nil
	}

	// If we get here, all attempts failed
	h.logger.Error("All attempts to fetch NOTAM data failed",
		logger.String("airport", airportCode),
		logger.Error(lastErr),
		logger.Int("max_attempts", maxRetries+1))
	return nil, lastErr
}

// GetAllFrequencies returns all frequencies
func (h *Handler) GetAllFrequencies(w http.ResponseWriter, r *http.Request) {
	// Get all frequencies
	frequencies := h.frequenciesService.GetAllFrequencies()

	// Create response
	response := map[string]interface{}{
		"timestamp":   time.Now().UTC(), // Use UTC for response timestamp
		"count":       len(frequencies),
		"frequencies": frequencies,
	}

	// Write response
	WriteJSON(w, http.StatusOK, response)
}

// GetFrequencyByID returns a frequency by its ID
func (h *Handler) GetFrequencyByID(w http.ResponseWriter, r *http.Request) {
	// Get frequency ID from URL
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing frequency ID", http.StatusBadRequest)
		return
	}

	// Get frequency data
	frequency, found := h.frequenciesService.GetFrequencyByID(id)
	if !found {
		http.Error(w, "Frequency not found", http.StatusNotFound)
		return
	}

	// Write response
	WriteJSON(w, http.StatusOK, frequency)
}

// StreamAudio streams audio for a frequency
func (h *Handler) StreamAudio(w http.ResponseWriter, r *http.Request) {
	// Get frequency ID from URL
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing frequency ID", http.StatusBadRequest)
		return
	}

	// Get client ID from query parameter
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		// Generate a random client ID if not provided
		clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	clientRemoteAddr := r.RemoteAddr

	// Set binary streaming headers
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Connection", "keep-alive")                // Add keep-alive
	w.Header().Set("Keep-Alive", "timeout=86400, max=604800") // Add keep-alive timeout

	// For HEAD requests, just return the headers
	if r.Method == "HEAD" {
		return
	}

	// Use the request's context directly - it will be canceled when the client disconnects
	ctx := r.Context()

	h.logger.Debug("Client requesting audio stream",
		logger.String("id", id),
		logger.String("client_id", clientID),
		logger.String("remote_addr", clientRemoteAddr))

	// Get audio stream with client ID
	stream, contentType, err := h.frequenciesService.GetAudioStream(ctx, id, clientID)
	if err != nil {
		// Check if the error is due to client already being connected
		if strings.Contains(err.Error(), "client already connected") {
			h.logger.Debug("Client already connected, returning success",
				logger.String("id", id),
				logger.String("client_id", clientID),
				logger.String("remote_addr", clientRemoteAddr))

			// Return a minimal response indicating the client is already connected
			w.Header().Set("X-Already-Connected", "true")
			w.WriteHeader(http.StatusOK)
			return
		}

		h.logger.Error("Failed to get audio stream",
			logger.String("id", id),
			logger.String("client_id", clientID),
			logger.String("remote_addr", clientRemoteAddr),
			logger.Error(err),
		)
		http.Error(w, "Stream unavailable", http.StatusServiceUnavailable)
		return
	}
	defer stream.Close() // Crucial: Ensures ClientStreamReader.Close() is called

	h.logger.Debug("Client connected to audio stream",
		logger.String("id", id),
		logger.String("client_id", clientID),
		logger.String("remote_addr", clientRemoteAddr),
		logger.String("content_type", contentType),
	)

	// Connection monitoring setup
	connectionStartTime := time.Now()

	// Use a buffer to improve performance
	buf := make([]byte, 4096)

	// Track consecutive errors for client disconnect detection
	consecutiveErrors := 0
	bytesWritten := 0

	// Stream data to client
	lastProgressLog := time.Now()
	for {
		// Check if client has disconnected
		select {
		case <-ctx.Done():
			h.logger.Info("Client context done, stopping stream",
				logger.String("id", id),
				logger.String("client_id", clientID),
				logger.String("remote_addr", clientRemoteAddr),
				logger.String("reason", ctx.Err().Error()),
				logger.Int("total_bytes_written", bytesWritten),
				logger.String("connection_duration", time.Since(connectionStartTime).String()),
			)
			return
		default:
			// Continue streaming
		}

		// Read from stream - this will timeout after 5 seconds if no data
		n, err := stream.Read(buf)

		if err != nil {
			if err == io.EOF {
				h.logger.Warn("Stream EOF reached unexpectedly",
					logger.String("id", id),
					logger.String("client_id", clientID),
					logger.Int("bytes_written_before_eof", bytesWritten),
					logger.String("connection_duration", time.Since(connectionStartTime).String()))
				return
			}

			h.logger.Warn("Error reading from stream",
				logger.String("id", id),
				logger.String("client_id", clientID),
				logger.String("error_type", fmt.Sprintf("%T", err)),
				logger.Error(err),
				logger.Int("consecutive_errors", consecutiveErrors+1))

			consecutiveErrors++
			if consecutiveErrors > 3 {
				h.logger.Error("Too many consecutive read errors, closing stream",
					logger.String("id", id),
					logger.String("client_id", clientID),
					logger.Int("total_consecutive_errors", consecutiveErrors),
					logger.Int("bytes_written_before_failure", bytesWritten),
					logger.String("connection_duration", time.Since(connectionStartTime).String()))
				return
			}

			// Brief pause before retrying
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Reset error counter on successful read
		consecutiveErrors = 0

		// If we got data, write it to the client
		if n > 0 {
			_, err = w.Write(buf[:n])
			if err != nil {
				h.logger.Warn("Error writing to client, closing stream",
					logger.String("id", id),
					logger.String("client_id", clientID),
					logger.String("error_type", fmt.Sprintf("%T", err)),
					logger.Error(err),
					logger.Int("bytes_written_before_error", bytesWritten),
					logger.String("connection_duration", time.Since(connectionStartTime).String()))
				return
			}

			bytesWritten += n

			// Flush data immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// Log every 100KB of data or every 60 seconds, whichever comes first
			if bytesWritten%102400 < n || time.Since(lastProgressLog) > 60*time.Second {
				h.logger.Debug("Streaming progress",
					logger.String("id", id),
					logger.String("client_id", clientID),
					logger.Int("bytes_written", bytesWritten),
					logger.String("connection_duration", time.Since(connectionStartTime).String()))
				lastProgressLog = time.Now()
			}
		}
	}
}

// parseAircraftFilters parses aircraft filter parameters from the request
func parseAircraftFilters(r *http.Request) (float64, float64, string, []string, int, *time.Time, *time.Time, *time.Time, *time.Time, float64, float64, float64, string, string, bool) {
	minAltitude := 0.0
	maxAltitude := 60000.0
	callsign := ""
	var status []string
	lastSeenMinutes := 0 // Default to 0 (no filtering)

	// New filter parameters
	var tookOffAfter, tookOffBefore, landedAfter, landedBefore *time.Time
	distanceNM := 0.0
	refLat, refLon := 0.0, 0.0
	refHex := ""
	refFlight := ""

	// Parse existing filters
	if minStr := r.URL.Query().Get("min_altitude"); minStr != "" {
		if min, err := strconv.ParseFloat(minStr, 64); err == nil {
			minAltitude = min
		}
	}

	if maxStr := r.URL.Query().Get("max_altitude"); maxStr != "" {
		if max, err := strconv.ParseFloat(maxStr, 64); err == nil {
			maxAltitude = max
		}
	}

	callsign = r.URL.Query().Get("callsign")

	// Parse status filter
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status = strings.Split(statusStr, ",")
		for i, s := range status {
			status[i] = strings.TrimSpace(s)
		}
	}

	// Parse last_seen_minutes filter
	if lastSeenStr := r.URL.Query().Get("last_seen_minutes"); lastSeenStr != "" {
		if lastSeen, err := strconv.Atoi(lastSeenStr); err == nil && lastSeen > 0 {
			lastSeenMinutes = lastSeen
		}
	}

	// Parse new takeoff time filters
	if tookOffAfterStr := r.URL.Query().Get("took_off_after"); tookOffAfterStr != "" {
		if t, err := time.Parse(time.RFC3339, tookOffAfterStr); err == nil {
			tookOffAfter = &t
		}
	}

	if tookOffBeforeStr := r.URL.Query().Get("took_off_before"); tookOffBeforeStr != "" {
		if t, err := time.Parse(time.RFC3339, tookOffBeforeStr); err == nil {
			tookOffBefore = &t
		}
	}

	// Parse new landing time filters
	if landedAfterStr := r.URL.Query().Get("landed_after"); landedAfterStr != "" {
		if t, err := time.Parse(time.RFC3339, landedAfterStr); err == nil {
			landedAfter = &t
		}
	}

	if landedBeforeStr := r.URL.Query().Get("landed_before"); landedBeforeStr != "" {
		if t, err := time.Parse(time.RFC3339, landedBeforeStr); err == nil {
			landedBefore = &t
		}
	}

	// Parse distance filter
	if distanceStr := r.URL.Query().Get("distance_nm"); distanceStr != "" {
		if dist, err := strconv.ParseFloat(distanceStr, 64); err == nil && dist > 0 {
			distanceNM = dist
		}
	}

	// Parse reference coordinate parameters
	if latStr := r.URL.Query().Get("ref_lat"); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			refLat = lat
		}
	}

	if lonStr := r.URL.Query().Get("ref_lon"); lonStr != "" {
		if lon, err := strconv.ParseFloat(lonStr, 64); err == nil {
			refLon = lon
		}
	}

	// Parse reference hex parameter
	refHex = r.URL.Query().Get("ref_hex")

	// Parse reference flight parameter
	refFlight = r.URL.Query().Get("ref_flight")

	// Parse exclude_other_airports_grounded parameter
	excludeOtherAirportsGrounded := false
	if excludeStr := r.URL.Query().Get("exclude_other_airports_grounded"); excludeStr != "" {
		if exclude, err := strconv.ParseBool(excludeStr); err == nil {
			excludeOtherAirportsGrounded = exclude
		} else if excludeStr == "1" {
			excludeOtherAirportsGrounded = true
		}
	}

	return minAltitude, maxAltitude, callsign, status, lastSeenMinutes,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore, distanceNM,
		refLat, refLon, refHex, refFlight, excludeOtherAirportsGrounded
}

// getHexCoordinates gets coordinates from an aircraft hex code
func (h *Handler) getHexCoordinates(hexCode string) (float64, float64, error) {
	// Look up aircraft by hex code
	aircraft, found := h.adsbService.GetAircraftByHex(hexCode)
	if !found {
		return 0, 0, fmt.Errorf("aircraft with hex %s not found", hexCode)
	}

	if aircraft.ADSB == nil || (aircraft.ADSB.Lat == 0 && aircraft.ADSB.Lon == 0) {
		return 0, 0, fmt.Errorf("aircraft with hex %s has no position data", hexCode)
	}
	return aircraft.ADSB.Lat, aircraft.ADSB.Lon, nil
}

// getRefAircraft gets the reference aircraft by hex code
func (h *Handler) getRefAircraft(hexCode string) (*adsb.Aircraft, error) {
	// Look up aircraft by hex code
	aircraft, found := h.adsbService.GetAircraftByHex(hexCode)
	if !found {
		return nil, fmt.Errorf("aircraft with hex %s not found", hexCode)
	}

	if aircraft.ADSB == nil || (aircraft.ADSB.Lat == 0 && aircraft.ADSB.Lon == 0) {
		return nil, fmt.Errorf("aircraft with hex %s has no position data", hexCode)
	}

	return aircraft, nil
}

// getFlightCoordinates gets coordinates from a flight number or tail number
func (h *Handler) getFlightCoordinates(flight string) (float64, float64, error) {
	// Look up aircraft by flight number
	// First, get all aircraft
	allAircraft := h.adsbService.GetAllAircraft()

	// Find the one with matching flight number
	for _, a := range allAircraft {
		if strings.EqualFold(strings.TrimSpace(a.Flight), strings.TrimSpace(flight)) {
			if a.ADSB == nil {
				return 0, 0, fmt.Errorf("aircraft with flight %s has no ADSB data", flight)
			}

			if a.ADSB.Lat == 0 && a.ADSB.Lon == 0 {
				return 0, 0, fmt.Errorf("aircraft with flight %s has no position data", flight)
			}

			return a.ADSB.Lat, a.ADSB.Lon, nil
		}
	}
	return 0, 0, fmt.Errorf("aircraft with flight %s not found", flight)
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// updateZeroValuesFromHistory updates zero values in the aircraft with the last non-zero values from position history
func updateZeroValuesFromHistory(aircraft *adsb.Aircraft) {
	// This function is no longer needed since we're using ADSB data directly
	// We keep it as a no-op for backward compatibility
	return
}

// haversine is a wrapper around adsb.Haversine for backward compatibility
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	return adsb.Haversine(lat1, lon1, lat2, lon2)
}

// This function has been replaced by getHexCoordinates and getFlightCoordinates

// CreateATCChatSession creates a new ATC chat session
func (h *Handler) CreateATCChatSession(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	session, err := h.atcChatService.CreateSession(r.Context())
	if err != nil {
		// Check if this is a missing API key error - handle gracefully
		if strings.Contains(err.Error(), "OpenAI API key is required") {
			h.logger.Warn("ATC Chat session creation failed - API key not configured")
			http.Error(w, "ATC Chat requires OpenAI API key configuration", http.StatusServiceUnavailable)
			return
		}

		// For other errors, log at error level with stack trace
		h.logger.Error("Failed to create ATC chat session", logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Created ATC chat session",
		logger.String("session_id", session.ID))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		h.logger.Error("Failed to encode session response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// EndATCChatSession terminates an ATC chat session
func (h *Handler) EndATCChatSession(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	if err := h.atcChatService.EndSession(r.Context(), sessionID); err != nil {
		h.logger.Error("Failed to end ATC chat session",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to end session: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Ended ATC chat session",
		logger.String("session_id", sessionID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"session_id": sessionID,
		"message":    "Session ended successfully",
	})
}

// HandleATCChatWebSocket handles WebSocket connections for ATC chat
func (h *Handler) HandleATCChatWebSocket(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Create ATC chat handlers and delegate to them
	atcChatHandlers := NewATCChatHandlers(h.atcChatService, h.logger)

	// Update the URL parameter to match what the ATC chat handler expects
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionID", sessionID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	atcChatHandlers.WebSocketHandler(w, r)
}

// GetATCChatSessionStatus returns the status of an ATC chat session
func (h *Handler) GetATCChatSessionStatus(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	status, err := h.atcChatService.GetSessionStatus(sessionID)
	if err != nil {
		h.logger.Error("Failed to get session status",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to get session status: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.Error("Failed to encode status response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetATCChatSessions returns all active ATC chat sessions
func (h *Handler) GetATCChatSessions(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	sessions := h.atcChatService.ListActiveSessions()

	response := map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
		"status":   "success",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode sessions response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetATCChatAirspaceStatus returns current airspace status for ATC chat
func (h *Handler) GetATCChatAirspaceStatus(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	status := h.atcChatService.GetAirspaceStatus()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.Error("Failed to encode airspace status response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// UpdateATCChatSessionContext updates the session context with fresh airspace data
func (h *Handler) UpdateATCChatSessionContext(w http.ResponseWriter, r *http.Request) {
	if h.atcChatService == nil {
		http.Error(w, "ATC Chat service not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	h.logger.Debug("Received request to update session context",
		logger.String("session_id", sessionID))

	// Generate the system prompt and variables that will be sent to AI
	promptWithVars, err := h.atcChatService.GenerateSystemPromptWithVariables(sessionID)
	if err != nil {
		h.logger.Error("Failed to generate system prompt for context update",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to generate system prompt: %v", err), http.StatusInternalServerError)
		return
	}

	// Update session context with fresh airspace data
	if err := h.atcChatService.UpdateSessionContextOnDemand(sessionID); err != nil {
		h.logger.Error("Failed to update session context",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to update session context: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Session context updated successfully",
		logger.String("session_id", sessionID))

	// Return success response with the actual instructions sent to AI and individual variables
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "success",
		"message":       "Session context updated with fresh airspace data",
		"instructions":  promptWithVars.Prompt,
		"prompt_length": len(promptWithVars.Prompt),
		"variables":     promptWithVars.Variables,
	})
}

// convertClearancesToAPIFormat converts clearance records to API format
func (h *Handler) convertClearancesToAPIFormat(clearances []*sqlite.ClearanceRecord) []adsb.ClearanceData {
	result := make([]adsb.ClearanceData, len(clearances))
	now := time.Now().UTC()

	for i, c := range clearances {
		result[i] = adsb.ClearanceData{
			ID:              c.ID,
			Type:            c.ClearanceType,
			Text:            c.ClearanceText,
			Runway:          c.Runway,
			Timestamp:       c.Timestamp,
			Status:          c.Status,
			TimeSinceIssued: h.formatTimeSince(now.Sub(c.Timestamp)),
		}
	}

	return result
}

// formatTimeSince formats a duration into a human-readable string
func (h *Handler) formatTimeSince(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

// CreateSimulatedAircraft creates a new simulated aircraft
func (h *Handler) CreateSimulatedAircraft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lat          float64 `json:"lat"`
		Lon          float64 `json:"lon"`
		Altitude     float64 `json:"altitude"`
		Heading      float64 `json:"heading"`
		Speed        float64 `json:"speed"`
		VerticalRate float64 `json:"vertical_rate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 {
		http.Error(w, "Invalid coordinates", http.StatusBadRequest)
		return
	}

	if req.Altitude < 0 || req.Altitude > 60000 {
		http.Error(w, "Invalid altitude (0-60000 ft)", http.StatusBadRequest)
		return
	}

	if req.Heading < 0 || req.Heading >= 360 {
		http.Error(w, "Invalid heading (0-359 degrees)", http.StatusBadRequest)
		return
	}

	if req.Speed < 0 || req.Speed > 500 {
		http.Error(w, "Invalid speed (0-500 knots)", http.StatusBadRequest)
		return
	}

	if req.VerticalRate < -3000 || req.VerticalRate > 3000 {
		http.Error(w, "Invalid vertical rate (-3000 to +3000 fpm)", http.StatusBadRequest)
		return
	}

	aircraft, err := h.simulationService.CreateAircraft(
		req.Lat, req.Lon, req.Altitude,
		req.Heading, req.Speed, req.VerticalRate,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("Created simulated aircraft via API",
		logger.String("hex", aircraft.Hex),
		logger.String("flight", aircraft.Flight))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"aircraft": aircraft,
	})
}

// UpdateSimulationControls updates the control parameters for a simulated aircraft
func (h *Handler) UpdateSimulationControls(w http.ResponseWriter, r *http.Request) {
	hex := chi.URLParam(r, "hex")
	if hex == "" {
		http.Error(w, "Missing hex parameter", http.StatusBadRequest)
		return
	}

	var req struct {
		Heading      float64 `json:"heading"`
		Speed        float64 `json:"speed"`
		VerticalRate float64 `json:"vertical_rate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Heading < 0 || req.Heading >= 360 {
		http.Error(w, "Invalid heading (0-359 degrees)", http.StatusBadRequest)
		return
	}

	if req.Speed < 0 || req.Speed > 500 {
		http.Error(w, "Invalid speed (0-500 knots)", http.StatusBadRequest)
		return
	}

	if req.VerticalRate < -3000 || req.VerticalRate > 3000 {
		http.Error(w, "Invalid vertical rate (-3000 to +3000 fpm)", http.StatusBadRequest)
		return
	}

	err := h.simulationService.UpdateControls(hex, req.Heading, req.Speed, req.VerticalRate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.logger.Debug("Updated simulation controls via API",
		logger.String("hex", hex),
		logger.Float64("heading", req.Heading),
		logger.Float64("speed", req.Speed),
		logger.Float64("vertical_rate", req.VerticalRate))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
	})
}

// RemoveSimulatedAircraft removes a simulated aircraft
func (h *Handler) RemoveSimulatedAircraft(w http.ResponseWriter, r *http.Request) {
	hex := chi.URLParam(r, "hex")
	if hex == "" {
		http.Error(w, "Missing hex parameter", http.StatusBadRequest)
		return
	}

	err := h.simulationService.RemoveAircraft(hex)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.logger.Info("Removed simulated aircraft via API",
		logger.String("hex", hex))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
	})
}

// GetSimulatedAircraft returns all simulated aircraft
func (h *Handler) GetSimulatedAircraft(w http.ResponseWriter, r *http.Request) {
	aircraft := h.simulationService.GetAllAircraft()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aircraft)
}
