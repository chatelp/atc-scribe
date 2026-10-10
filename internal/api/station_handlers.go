package api

import (
	"encoding/json"
	"net/http"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/reference"
	"github.com/yegors/co-atc/pkg/logger"
)

// GetADSBSourceStatus returns ADS-B source mode, health, and optional receiver/stats payloads.
func (h *Handler) GetADSBSourceStatus(w http.ResponseWriter, r *http.Request) {
	status := h.adsbService.GetSourceStatus()
	WriteJSON(w, http.StatusOK, status)
}

// GetStationConfig returns the station configuration (latitude, longitude, elevation)
func (h *Handler) GetStationConfig(w http.ResponseWriter, r *http.Request) {
	// Get effective coordinates (override if set, otherwise config)
	effectiveLat, effectiveLon := h.adsbService.GetEffectiveStationCoords()

	stationCfg := struct {
		Latitude         float64            `json:"latitude"`
		Longitude        float64            `json:"longitude"`
		ElevationFeet    int                `json:"elevation_feet"`
		CruiseAltitudeFt int                `json:"cruise_altitude_ft"`
		AirportCode      string             `json:"airport_code"`
		Runways          interface{}        `json:"runways,omitempty"`
		RunwayInUse      []adsb.RunwayScore `json:"runway_in_use,omitempty"`
		// Every airport followed, the reference one first, with its runways and
		// runway in use; Runways and RunwayInUse above are the first's.
		Airports    []stationAirport `json:"airports,omitempty"`
		FetchErrors []string         `json:"fetch_errors,omitempty"`
		// Weather configuration flags
		FetchMETAR  bool `json:"fetch_metar"`
		FetchTAF    bool `json:"fetch_taf"`
		FetchNOTAMs bool `json:"fetch_notams"`
		// Station override information
		OverrideActive bool `json:"override_active"`
	}{
		Latitude:         effectiveLat,
		Longitude:        effectiveLon,
		ElevationFeet:    h.config.Station.ElevationFeet,
		CruiseAltitudeFt: h.config.FlightPhases.CruiseAltitudeFt,
		// The reference airport in force, which the settings panel can change;
		// [station] airport_code is only where it starts.
		AirportCode:    h.adsbService.PhaseReference().Airport,
		FetchMETAR:     h.config.Weather.FetchMETAR,
		FetchTAF:       h.config.Weather.FetchTAF,
		FetchNOTAMs:    h.config.Weather.FetchNOTAMs,
		OverrideActive: effectiveLat != h.config.Station.Latitude || effectiveLon != h.config.Station.Longitude,
	}

	// Track if we have any data fetch failures
	var fetchErrors []string

	// Build runway data from reference service
	if h.refService != nil {
		runwayData := h.buildRunwayResponse()
		stationCfg.Runways = runwayData
	}

	// Add runway-in-use scores
	if scores := h.adsbService.GetRunwayInUseScores(3); len(scores) > 0 {
		stationCfg.RunwayInUse = scores
	}
	if h.refService != nil {
		stationCfg.Airports = h.stationAirports()
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

// stationAirport is one followed airport as the map draws it.
type stationAirport struct {
	Code        string             `json:"code"`
	Name        string             `json:"name,omitempty"`
	Latitude    float64            `json:"latitude"`
	Longitude   float64            `json:"longitude"`
	Principal   bool               `json:"principal"`
	Runways     interface{}        `json:"runways"`
	RunwayInUse []adsb.RunwayScore `json:"runway_in_use,omitempty"`
}

// stationAirports lists every airport followed, the reference one first.
func (h *Handler) stationAirports() []stationAirport {
	var out []stationAirport
	for i, ref := range h.adsbService.PhaseReferences() {
		a := stationAirport{Code: ref.Airport, Latitude: ref.Lat, Longitude: ref.Lon, Principal: i == 0,
			RunwayInUse: h.adsbService.GetRunwayInUseScoresFor(ref.Airport, 3)}
		if i == 0 {
			a.Runways = h.buildRunwayResponse()
		} else if _, data, ext, err := h.refService.AirportRunways(ref.Airport); err == nil {
			a.Runways = runwayResponse(data, ext)
		}
		if info := h.refService.GetAirport(ref.Airport); info != nil {
			a.Name = info.Name
		}
		out = append(out, a)
	}
	return out
}

// buildRunwayResponse builds the runway JSON response from precomputed reference data.
// Matches the same JSON shape the frontend drawRunways() expects.
func (h *Handler) buildRunwayResponse() interface{} {
	return runwayResponse(h.refService.GetHomeRunwayData(), h.refService.GetHomeRunwayExtensions())
}

// runwayResponse puts an airport's runways in the shape drawRunways() reads.
func runwayResponse(homeData adsb.RunwayData, extensions map[string]map[string][]reference.RunwayExtensionPoint) interface{} {

	type Point struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Distance  float64 `json:"distance,omitempty"`
	}

	// Convert extensions to the Point type the frontend expects
	extResponse := make(map[string]map[string][]Point)
	for pairKey, ends := range extensions {
		extResponse[pairKey] = make(map[string][]Point)
		for endID, pts := range ends {
			points := make([]Point, len(pts))
			for i, p := range pts {
				points[i] = Point{Latitude: p.Latitude, Longitude: p.Longitude, Distance: p.Distance}
			}
			extResponse[pairKey][endID] = points
		}
	}

	return struct {
		Airport          string `json:"airport"`
		RunwayThresholds map[string]map[string]struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"runway_thresholds"`
		RunwayExtensions map[string]map[string][]Point `json:"runway_extensions"`
	}{
		Airport:          homeData.Airport,
		RunwayThresholds: homeData.RunwayThresholds,
		RunwayExtensions: extResponse,
	}
}
