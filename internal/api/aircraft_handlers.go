package api

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/pkg/logger"
)

// GetAllAircraft returns all aircraft
func (h *Handler) GetAllAircraft(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Debug("Starting GetAllAircraft API call")

	// Parse query parameters
	minAltitude, maxAltitude, callsign, status, lastSeenMinutes,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore, distanceNM,
		refLat, refLon, refHex, refFlight, excludeOtherAirportsGrounded, simple := parseAircraftFilters(r)

	// Get aircraft data
	aircraft, usedDBLastSeenFilter := h.fetchAircraft(minAltitude, maxAltitude, status, lastSeenMinutes,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore, simple)

	// Filter by callsign if provided
	if callsign != "" {
		aircraft = filterByCallsign(aircraft, callsign)
	}

	// Filter by last seen time if provided (only needed if not already filtered at DB level)
	if lastSeenMinutes > 0 && !usedDBLastSeenFilter {
		aircraft = filterByLastSeen(aircraft, lastSeenMinutes)
	}

	// Apply distance filter if provided
	if distanceNM > 0 {
		aircraft = h.filterByDistance(aircraft, distanceNM, refLat, refLon, refHex, refFlight)
	}

	// Apply exclude_other_airports_grounded filter if requested
	if excludeOtherAirportsGrounded {
		aircraft = h.excludeOtherAirportsGrounded(aircraft)
	}

	// Update zero values with last non-zero values from position history
	h.annotateAircraft(aircraft, distanceNM, refLat, refLon, refHex)

	// Calculate counts by ground/air and active/total
	counts := countAircraft(aircraft)

	// Populate what voice has said about each aircraft. One grouped query serves
	// them all, unlike the clearance loop below, which asks once per target.
	h.attachVoice(aircraft)

	// Populate clearances for each aircraft
	h.attachClearances(aircraft)

	// Return simplified response if simple=1
	if simple {
		simpleAircraft := h.simplifyAircraft(aircraft)

		simpleResponse := adsb.AircraftSimpleResponse{
			Timestamp: time.Now().UTC(),
			Count:     len(simpleAircraft),
			Aircraft:  simpleAircraft,
		}
		WriteJSON(w, http.StatusOK, simpleResponse)

		totalDuration := time.Since(start)
		h.logger.Debug("GetAllAircraft API call completed (simple mode)",
			logger.Duration("total_duration", totalDuration),
			logger.Int("final_aircraft_count", len(simpleAircraft)))
		return
	}

	// Create response
	response := adsb.AircraftResponse{
		Timestamp: time.Now().UTC(), // Use UTC for response timestamp
		Count:     len(aircraft),
		Counts:    counts,
		Aircraft:  aircraft,
	}

	// Write response
	WriteJSON(w, http.StatusOK, response)

	totalDuration := time.Since(start)
	h.logger.Debug("GetAllAircraft API call completed",
		logger.Duration("total_duration", totalDuration),
		logger.Int("final_aircraft_count", len(aircraft)))
}

// The steps of GetAllAircraft, in the order it takes them.

// fetchAircraft reads the aircraft from the service, filtered at the source
// when the query allows it, and says whether last_seen was already applied.
func (h *Handler) fetchAircraft(minAltitude, maxAltitude float64, status []string, lastSeenMinutes int,
	tookOffAfter, tookOffBefore, landedAfter, landedBefore *time.Time, simple bool) ([]*adsb.Aircraft, bool) {
	dataFetchStart := time.Now()
	var aircraft []*adsb.Aircraft
	// Track whether we used database-level last_seen filtering
	usedDBLastSeenFilter := false

	if minAltitude > 0 || maxAltitude < 60000 || len(status) > 0 ||
		tookOffAfter != nil || tookOffBefore != nil ||
		landedAfter != nil || landedBefore != nil {
		// Use the enhanced GetFiltered method with date filters
		aircraft = h.adsbService.GetFilteredAircraft(
			minAltitude, maxAltitude,
			status,
			tookOffAfter, tookOffBefore, landedAfter, landedBefore,
		)
	} else if simple {
		// Use minimal mode for simple API - skips phase history and date queries
		aircraft = h.adsbService.GetAllAircraftMinimal(lastSeenMinutes)
		usedDBLastSeenFilter = lastSeenMinutes > 0
	} else if lastSeenMinutes > 0 {
		// Use database-level filtering for last_seen - much faster on large databases
		aircraft = h.adsbService.GetAllAircraftWithLastSeenFilter(lastSeenMinutes)
		usedDBLastSeenFilter = true
	} else {
		aircraft = h.adsbService.GetAllAircraft()
	}

	dataFetchDuration := time.Since(dataFetchStart)
	h.logger.Debug("Aircraft data fetch completed",
		logger.Duration("duration", dataFetchDuration),
		logger.Int("aircraft_count", len(aircraft)),
		logger.Bool("used_db_last_seen_filter", usedDBLastSeenFilter))

	return aircraft, usedDBLastSeenFilter
}

// filterByCallsign keeps the aircraft whose flight contains callsign, in any case.
func filterByCallsign(aircraft []*adsb.Aircraft, callsign string) []*adsb.Aircraft {
	filtered := make([]*adsb.Aircraft, 0)
	for _, a := range aircraft {
		if strings.Contains(strings.ToUpper(a.Flight), strings.ToUpper(callsign)) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// filterByLastSeen keeps the aircraft seen in the last lastSeenMinutes.
func filterByLastSeen(aircraft []*adsb.Aircraft, lastSeenMinutes int) []*adsb.Aircraft {
	now := time.Now().UTC() // Use UTC for cutoff time
	cutoffTime := now.Add(-time.Duration(lastSeenMinutes) * time.Minute)

	filtered := make([]*adsb.Aircraft, 0)
	for _, a := range aircraft {
		if a.LastSeen.After(cutoffTime) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// filterByDistance keeps the active airborne aircraft within distanceNM of a
// reference -- coordinates, an aircraft, or a flight, in that order of
// priority -- sorted by distance from it. A reference that cannot be resolved
// is logged and the aircraft are returned unfiltered.
func (h *Handler) filterByDistance(aircraft []*adsb.Aircraft, distanceNM, refLat, refLon float64, refHex, refFlight string) []*adsb.Aircraft {
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
			if lat, lon, ok := refAircraft.ADSB.Position(); ok {
				refLatitude = lat
				refLongitude = lon
			}
			refHeading = adsb.NumberOrZero(refAircraft.ADSB.TrueHeading)
			if refHeading == 0 {
				refHeading = adsb.NumberOrZero(refAircraft.ADSB.Track) // Use track if true heading is not available
			}
			refAltitude = refAircraft.ADSB.AltBaro.Float64()
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
			if a.ADSB == nil || !a.ADSB.HasPosition() {
				continue
			}
			lat, lon, _ := a.ADSB.Position()

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
			distMeters := adsb.Haversine(lat, lon, refLatitude, refLongitude)
			distNM := adsb.MetersToNM(distMeters)
			distNM = math.Round(distNM*10) / 10 // Round to 1 decimal place

			// Add to filtered list if within range
			if distNM <= distanceNM {
				// For proximity queries, we need to distinguish between:
				// 1. Distance from station (regular distance field)
				// 2. Distance from reference aircraft (relative distance field)

				// Calculate distance from station for each aircraft
				if a.ADSB != nil && a.ADSB.HasPosition() {
					stationDistMeters := adsb.Haversine(lat, lon, h.config.Station.Latitude, h.config.Station.Longitude)
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
						lat, lon)
					a.RelativeBearing = &bearing

					// Calculate relative altitude
					if refAltitude > 0 && a.ADSB.AltBaro.Float64() > 0 {
						relAlt := a.ADSB.AltBaro.Float64() - refAltitude
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
	return aircraft
}

// excludeOtherAirportsGrounded drops the aircraft on the ground away from the
// station's airport.
func (h *Handler) excludeOtherAirportsGrounded(aircraft []*adsb.Aircraft) []*adsb.Aircraft {
	filtered := make([]*adsb.Aircraft, 0)
	airportRangeNM := h.config.Station.AirportRangeNM
	if airportRangeNM == 0 {
		airportRangeNM = 5.0 // Default to 5.0 NM if not configured
	}

	for _, a := range aircraft {
		// Include all aircraft that are not on ground, or grounded aircraft within airport range
		if !a.OnGround {
			filtered = append(filtered, a)
		} else if a.ADSB != nil && a.ADSB.HasPosition() {
			lat, lon, _ := a.ADSB.Position()
			// Calculate distance from station for grounded aircraft
			distMeters := adsb.Haversine(lat, lon, h.config.Station.Latitude, h.config.Station.Longitude)
			distNM := adsb.MetersToNM(distMeters)
			if distNM <= airportRangeNM {
				filtered = append(filtered, a)
			}
		}
	}
	return filtered
}

// annotateAircraft adds what is computed per aircraft: distance from the
// station, the ATC-derived metrics, and no history for a proximity query.
func (h *Handler) annotateAircraft(aircraft []*adsb.Aircraft, distanceNM, refLat, refLon float64, refHex string) {
	for _, a := range aircraft {
		updateZeroValuesFromHistory(a)

		// Calculate distance from station for each aircraft
		if a.ADSB != nil && a.ADSB.HasPosition() {
			lat, lon, _ := a.ADSB.Position()
			distMeters := adsb.Haversine(lat, lon, h.config.Station.Latitude, h.config.Station.Longitude)
			distNM := adsb.MetersToNM(distMeters)
			distNM = math.Round(distNM*10) / 10 // Round to 1 decimal place
			a.Distance = &distNM
		}

		// Check if this is a proximity query (ref_hex or ref_lat/ref_lon with distance_nm)
		isProximityQuery := (refHex != "" || (refLat != 0 && refLon != 0)) && distanceNM > 0

		// For proximity queries, don't include history data to reduce payload size
		if isProximityQuery {
			a.History = nil
		}

		// Future array is now populated by the prediction algorithm
		adsb.AttachATCDerivedMetrics(a)
	}
}

// countAircraft counts the aircraft on the ground and in the air, active and in total.
func countAircraft(aircraft []*adsb.Aircraft) adsb.AircraftCounts {
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

	return adsb.AircraftCounts{
		GroundActive: groundActive,
		GroundTotal:  groundTotal,
		AirActive:    airActive,
		AirTotal:     airTotal,
	}
}

// attachVoice puts on each aircraft what the radio has said about its callsign.
func (h *Handler) attachVoice(aircraft []*adsb.Aircraft) {
	if h.transcriptionStorage != nil {
		if summaries, err := h.transcriptionStorage.VoiceSummaries(); err != nil {
			h.logger.Error("Failed to summarise voice by callsign", logger.Error(err))
		} else {
			for _, a := range aircraft {
				if v, ok := summaries[strings.TrimSpace(a.Flight)]; ok {
					a.Voice = &adsb.VoiceData{
						Transmissions: v.Transmissions,
						LastHeard:     v.LastHeard,
						LastText:      v.LastText,
					}
				}
			}
		}
	}
}

// attachClearances puts on each aircraft its last ten clearances.
func (h *Handler) attachClearances(aircraft []*adsb.Aircraft) {
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
}

// simplifyAircraft is the simple=1 shape of the aircraft: one flat record each.
func (h *Handler) simplifyAircraft(aircraft []*adsb.Aircraft) []*adsb.AircraftSimple {
	simpleAircraft := make([]*adsb.AircraftSimple, 0, len(aircraft))
	for _, a := range aircraft {
		airlineName := strings.TrimSpace(a.Airline)
		if airlineName == "" && h.refService != nil {
			flight := strings.TrimSpace(strings.ToUpper(a.Flight))
			if len(flight) >= 3 {
				icaoCode := flight[:3]
				if icaoCode[0] >= 'A' && icaoCode[0] <= 'Z' &&
					icaoCode[1] >= 'A' && icaoCode[1] <= 'Z' &&
					icaoCode[2] >= 'A' && icaoCode[2] <= 'Z' {
					if resolved := strings.TrimSpace(h.refService.LookupAirline(icaoCode)); resolved != "" {
						airlineName = resolved
					}
				}
			}
		}

		sa := &adsb.AircraftSimple{
			Hex:      a.Hex,
			Callsign: a.Flight,
			Airline:  airlineName,
			Distance: a.Distance,
			Status:   a.Status,
		}
		// Add BSDB data if available
		if a.BSDB != nil {
			sa.Registration = a.BSDB.Registration
			sa.AircraftType = a.BSDB.ICAOTypeCode
			sa.Manufacturer = a.BSDB.Manufacturer
			sa.RegisteredOwners = a.BSDB.RegisteredOwners
		}
		// Add ADSB data if available
		if a.ADSB != nil {
			sa.Lat = a.ADSB.Lat
			sa.Lon = a.ADSB.Lon
			sa.AltBaro = math.Round(a.ADSB.AltBaro.Float64()/100) * 100
			if a.ADSB.GS != nil {
				v := math.Round(*a.ADSB.GS)
				sa.GroundSpeed = &v
			}
			if a.ADSB.TAS != nil {
				v := math.Round(*a.ADSB.TAS)
				sa.TrueAirspeed = &v
			}
			if a.ADSB.Track != nil {
				v := math.Round(*a.ADSB.Track)
				sa.Track = &v
			}
			if a.ADSB.MagHeading != nil {
				v := math.Round(*a.ADSB.MagHeading)
				sa.MagHeading = &v
			}
			if a.ADSB.BaroRate != nil {
				v := math.Round(*a.ADSB.BaroRate/100) * 100
				sa.VerticalRate = &v
			}
			sa.Squawk = a.ADSB.Squawk
			sa.Category = a.ADSB.Category
			// Use ADSB type if BSDB type not available
			if sa.AircraftType == "" {
				sa.AircraftType = a.ADSB.AircraftType
			}
			if sa.Registration == "" {
				sa.Registration = a.ADSB.Registration
			}
		}
		// Add current phase if available
		if a.Phase != nil && len(a.Phase.Current) > 0 {
			sa.Phase = a.Phase.Current[0].Phase
		}
		simpleAircraft = append(simpleAircraft, sa)
	}
	return simpleAircraft
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
	if aircraft.ADSB != nil && aircraft.ADSB.HasPosition() {
		lat, lon, _ := aircraft.ADSB.Position()
		distMeters := haversine(lat, lon, h.config.Station.Latitude, h.config.Station.Longitude)
		distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
		aircraft.Distance = &distNM
	}

	adsb.AttachATCDerivedMetrics(aircraft)

	// Write response
	WriteJSON(w, http.StatusOK, aircraft)
}

// positionDedupHeading returns the best available heading (mag→track→true priority).
// Returns -1 if no heading is available.
func positionDedupHeading(p adsb.Position) float64 {
	if p.MagHeading != nil {
		return *p.MagHeading
	}
	if p.Track != nil {
		return *p.Track
	}
	if p.TrueHeading != nil {
		return *p.TrueHeading
	}
	return -1
}

// positionsMatchForDedup returns true if two positions have effectively the same
// displayed values (altitude rounded to 100ft, heading, TAS, GS) within a
// tolerance of 1. Distance is exempt.
func positionsMatchForDedup(a, b adsb.Position) bool {
	altA := math.Round(adsb.NumberOrZero(a.Altitude)/100) * 100
	altB := math.Round(adsb.NumberOrZero(b.Altitude)/100) * 100
	if a.Altitude != nil || b.Altitude != nil {
		if a.Altitude == nil || b.Altitude == nil {
			return false
		}
	}
	if math.Abs(altA-altB) > 100 {
		return false
	}

	hdgA := positionDedupHeading(a)
	hdgB := positionDedupHeading(b)
	if hdgA < 0 && hdgB < 0 {
		// both missing — match
	} else if hdgA < 0 || hdgB < 0 {
		return false
	} else if math.Abs(math.Round(hdgA)-math.Round(hdgB)) > 1 {
		return false
	}

	if (a.SpeedTrue == nil) != (b.SpeedTrue == nil) {
		return false
	}
	if math.Abs(math.Round(adsb.NumberOrZero(a.SpeedTrue))-math.Round(adsb.NumberOrZero(b.SpeedTrue))) > 1 {
		return false
	}
	if (a.SpeedGS == nil) != (b.SpeedGS == nil) {
		return false
	}
	if math.Abs(math.Round(adsb.NumberOrZero(a.SpeedGS))-math.Round(adsb.NumberOrZero(b.SpeedGS))) > 1 {
		return false
	}

	return true
}

// deduplicateHistory removes consecutive positions with effectively identical
// displayed values (tolerance of 1) and annotates remaining positions with
// skip counts for the UI to show dividers.
// SkippedBefore: N duplicate positions were omitted between the previous kept position and this one.
// SkippedAfter: N trailing duplicate positions were omitted after the last kept position.
// For small datasets (<60 positions), all positions are returned without grouping.
func deduplicateHistory(positions []adsb.Position) []adsb.Position {
	if len(positions) < 60 {
		return positions
	}

	result := make([]adsb.Position, 0, len(positions))
	skippedCount := 0

	for i := 0; i < len(positions); i++ {
		if i > 0 && positionsMatchForDedup(positions[i], result[len(result)-1]) {
			skippedCount++
			continue
		}

		// New distinct row — annotate it with how many were skipped before it
		pos := positions[i]
		if skippedCount > 0 {
			pos.SkippedBefore = skippedCount
			skippedCount = 0
		}

		result = append(result, pos)
	}

	// Trailing duplicates at the end — annotate the last kept position
	if skippedCount > 0 && len(result) > 0 {
		result[len(result)-1].SkippedAfter = skippedCount
	}

	return result
}

func roundFloat(value float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(value*pow) / pow
}

func roundedFloatPtr(value *float64, decimals int) *float64 {
	if value == nil {
		return nil
	}
	rounded := roundFloat(*value, decimals)
	return &rounded
}

func deriveVerticalSpeedAt(positions []adsb.Position, index int) (*float64, bool) {
	if index < 0 || index >= len(positions) || positions[index].Altitude == nil {
		return nil, false
	}
	current := positions[index]
	neighborIndexes := []int{index - 1, index + 1}
	for _, neighborIndex := range neighborIndexes {
		if neighborIndex < 0 || neighborIndex >= len(positions) {
			continue
		}
		neighbor := positions[neighborIndex]
		if neighbor.Altitude == nil {
			continue
		}
		deltaMinutes := current.Timestamp.Sub(neighbor.Timestamp).Minutes()
		if math.Abs(deltaMinutes) < 1e-6 {
			continue
		}
		verticalSpeed := (*current.Altitude - *neighbor.Altitude) / deltaMinutes
		return &verticalSpeed, true
	}
	return nil, false
}

func normalizeTrackPositions(positions []adsb.Position) []adsb.Position {
	normalized := make([]adsb.Position, len(positions))
	copy(normalized, positions)

	for i := range normalized {
		if normalized[i].VerticalSpeed == nil {
			if derivedVerticalSpeed, ok := deriveVerticalSpeedAt(normalized, i); ok {
				normalized[i].VerticalSpeed = derivedVerticalSpeed
			}
		}

		normalized[i].Lat = roundedFloatPtr(normalized[i].Lat, 6)
		normalized[i].Lon = roundedFloatPtr(normalized[i].Lon, 6)
		normalized[i].Altitude = roundedFloatPtr(normalized[i].Altitude, 0)
		normalized[i].SpeedTrue = roundedFloatPtr(normalized[i].SpeedTrue, 0)
		normalized[i].SpeedGS = roundedFloatPtr(normalized[i].SpeedGS, 0)
		normalized[i].Track = roundedFloatPtr(normalized[i].Track, 0)
		normalized[i].TrueHeading = roundedFloatPtr(normalized[i].TrueHeading, 0)
		normalized[i].MagHeading = roundedFloatPtr(normalized[i].MagHeading, 0)
		normalized[i].VerticalSpeed = roundedFloatPtr(normalized[i].VerticalSpeed, 0)
	}

	return normalized
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
	filteredHistory := make([]adsb.Position, 0, len(history))
	for i := range history {
		if history[i].Lat == nil || history[i].Lon == nil {
			continue
		}
		if *history[i].Lat == 0 && *history[i].Lon == 0 {
			continue
		}
		distMeters := haversine(*history[i].Lat, *history[i].Lon, h.config.Station.Latitude, h.config.Station.Longitude)
		distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
		history[i].Distance = &distNM
		filteredHistory = append(filteredHistory, history[i])
	}
	history = filteredHistory

	// Deduplicate consecutive positions with identical displayed values
	history = deduplicateHistory(history)
	history = normalizeTrackPositions(history)

	// Calculate current distance from station
	var distance *float64
	if aircraft.ADSB != nil && aircraft.ADSB.HasPosition() {
		lat, lon, _ := aircraft.ADSB.Position()
		distMeters := haversine(lat, lon, h.config.Station.Latitude, h.config.Station.Longitude)
		distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
		distance = &distNM
	}

	// Calculate distance for each future position
	future := aircraft.Future
	for i := range future {
		if future[i].Lat != nil && future[i].Lon != nil {
			distMeters := haversine(*future[i].Lat, *future[i].Lon, h.config.Station.Latitude, h.config.Station.Longitude)
			distNM := math.Round(distMeters/1852.0*10) / 10 // Convert meters to nautical miles and round to 1 decimal place
			future[i].Distance = &distNM
		}
	}
	future = normalizeTrackPositions(future)

	hindcast := normalizeTrackPositions(aircraft.Hindcast)

	// Fetch phase history
	phaseHistory, err := h.adsbService.GetPhaseHistory(hex)
	if err != nil {
		h.logger.Error("Failed to get phase history",
			logger.Error(err),
			logger.String("hex", hex))
		phaseHistory = []adsb.PhaseChange{}
	}

	// Create response
	response := adsb.AircraftTracksResponse{
		Hex:          aircraft.Hex,
		Flight:       aircraft.Flight,
		Distance:     distance,
		History:      history,
		Future:       future,
		Hindcast:     hindcast,
		PhaseHistory: phaseHistory,
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
				logger.Float64("mag_heading", adsb.NumberOrZero(pos.MagHeading)),
				logger.Float64("true_heading", adsb.NumberOrZero(pos.TrueHeading)),
				logger.String("timestamp", pos.Timestamp.Format(time.RFC3339)))
		}
	}

	// Write response
	WriteJSON(w, http.StatusOK, response)
}

// parseAircraftFilters parses aircraft filter parameters from the request
func parseAircraftFilters(r *http.Request) (float64, float64, string, []string, int, *time.Time, *time.Time, *time.Time, *time.Time, float64, float64, float64, string, string, bool, bool) {
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
	simple := false // Simple mode returns lightweight response

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

	// Parse simple parameter for lightweight response
	if simpleStr := r.URL.Query().Get("simple"); simpleStr != "" {
		if s, err := strconv.ParseBool(simpleStr); err == nil {
			simple = s
		} else if simpleStr == "1" {
			simple = true
		}
	}

	return minAltitude, maxAltitude, callsign, status, lastSeenMinutes,
		tookOffAfter, tookOffBefore, landedAfter, landedBefore, distanceNM,
		refLat, refLon, refHex, refFlight, excludeOtherAirportsGrounded, simple
}

// getRefAircraft gets the reference aircraft by hex code
func (h *Handler) getRefAircraft(hexCode string) (*adsb.Aircraft, error) {
	// Look up aircraft by hex code
	aircraft, found := h.adsbService.GetAircraftByHex(hexCode)
	if !found {
		return nil, fmt.Errorf("aircraft with hex %s not found", hexCode)
	}

	if aircraft.ADSB == nil || !aircraft.ADSB.HasPosition() {
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

			if !a.ADSB.HasPosition() {
				return 0, 0, fmt.Errorf("aircraft with flight %s has no position data", flight)
			}
			lat, lon, _ := a.ADSB.Position()
			return lat, lon, nil
		}
	}
	return 0, 0, fmt.Errorf("aircraft with flight %s not found", flight)
}

// updateZeroValuesFromHistory updates zero values in the aircraft with the last non-zero values from position history
func updateZeroValuesFromHistory(aircraft *adsb.Aircraft) {
	// This function is no longer needed since we're using ADSB data directly
	// We keep it as a no-op for backward compatibility
	_ = aircraft
}

// haversine is a wrapper around adsb.Haversine for backward compatibility
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	return adsb.Haversine(lat1, lon1, lat2, lon2)
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
