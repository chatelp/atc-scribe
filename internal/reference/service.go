package reference

import (
	"fmt"
	"math"
	"strings"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/pkg/logger"
)

// Service provides unified access to all reference data (aircraft, airlines, airports, runways, navaids).
// All data is loaded once at startup and geo-filtered where applicable.
type Service struct {
	logger *logger.Logger

	// Global (no geo-filter)
	aircraftMap map[string]*AircraftInfo // key: uppercase hex
	airlineMap  map[string]string        // key: ICAO or IATA code → airline name

	// Geo-filtered within display_range_nm of station
	airports   []*AirportInfo
	airportMap map[string]*AirportInfo // key: airport ident (ICAO)
	runways    []*RunwayInfo
	navaids    []*NavaidInfo

	// Home airport specific
	homeRunways          []*RunwayInfo
	homeRunwayData       adsb.RunwayData
	homeRunwayExtensions map[string]map[string][]RunwayExtensionPoint
}

// NewService creates a new reference service, loading all CSV data at startup.
func NewService(cfg ServiceConfig, log *logger.Logger) (*Service, error) {
	s := &Service{
		logger:      log.Named("reference"),
		aircraftMap: make(map[string]*AircraftInfo),
		airlineMap:  make(map[string]string),
		airportMap:  make(map[string]*AirportInfo),
	}

	// 1. Load aircraft.csv (global)
	if cfg.AircraftCSVPath != "" {
		m, err := loadAircraftCSV(cfg.AircraftCSVPath)
		if err != nil {
			s.logger.Warn("Failed to load aircraft.csv: " + err.Error())
		} else {
			s.aircraftMap = m
			s.logger.Info("Aircraft data loaded",
				logger.Int("count", len(m)),
				logger.String("path", cfg.AircraftCSVPath))
		}
	}

	// 2. Load airlines.dat (global)
	if cfg.AirlinesDATPath != "" {
		m, err := loadAirlineDAT(cfg.AirlinesDATPath)
		if err != nil {
			s.logger.Warn("Failed to load airlines.dat: " + err.Error())
		} else {
			s.airlineMap = m
			s.logger.Info("Airline data loaded",
				logger.Int("count", len(m)),
				logger.String("path", cfg.AirlinesDATPath))
		}
	}

	// 3. Load airports.csv (geo-filtered)
	if cfg.AirportsCSVPath != "" {
		airports, airportMap, err := loadAirportsCSV(cfg.AirportsCSVPath, cfg.StationLat, cfg.StationLon, cfg.DisplayRangeNM)
		if err != nil {
			s.logger.Warn("Failed to load airports.csv: " + err.Error())
		} else {
			s.airports = airports
			s.airportMap = airportMap
			s.logger.Info("Airport data loaded",
				logger.Int("total_in_range", len(airports)),
				logger.Float64("range_nm", cfg.DisplayRangeNM))
		}
	}

	// 4. Load airport-frequencies.csv (attach to filtered airports)
	if cfg.FrequenciesCSVPath != "" && len(s.airportMap) > 0 {
		if err := loadFrequenciesCSV(cfg.FrequenciesCSVPath, s.airportMap); err != nil {
			s.logger.Warn("Failed to load airport-frequencies.csv: " + err.Error())
		} else {
			freqCount := 0
			for _, ap := range s.airports {
				freqCount += len(ap.Frequencies)
			}
			s.logger.Info("Airport frequency data loaded",
				logger.Int("count", freqCount))
		}
	}

	// 5. Load runways.csv (geo-filtered + home airport)
	if cfg.RunwaysCSVPath != "" {
		all, home, err := loadRunwaysCSV(cfg.RunwaysCSVPath, s.airportMap, cfg.HomeAirportCode)
		if err != nil {
			s.logger.Warn("Failed to load runways.csv: " + err.Error())
		} else {
			s.runways = all
			s.homeRunways = home
			s.logger.Info("Runway data loaded",
				logger.Int("total_in_range", len(all)),
				logger.Int("home_airport", len(home)),
				logger.String("home_code", cfg.HomeAirportCode))
		}
	}

	// 6. Load navaids.csv (geo-filtered)
	if cfg.NavaidsCSVPath != "" {
		navs, err := loadNavaidsCSV(cfg.NavaidsCSVPath, cfg.StationLat, cfg.StationLon, cfg.DisplayRangeNM)
		if err != nil {
			s.logger.Warn("Failed to load navaids.csv: " + err.Error())
		} else {
			s.navaids = navs
			s.logger.Info("Navaid data loaded",
				logger.Int("count", len(navs)),
				logger.Float64("range_nm", cfg.DisplayRangeNM))
		}
	}

	// 7. Build home runway data (backward-compatible format for phase detection)
	s.buildHomeRunwayData(cfg.HomeAirportCode)
	s.buildHomeRunwayExtensions(cfg.ExtensionLengthNM)

	return s, nil
}

// --- Aircraft enrichment ---

// LookupAircraft retrieves aircraft info by hex code (case-insensitive).
func (s *Service) LookupAircraft(hex string) *AircraftInfo {
	return s.aircraftMap[strings.ToUpper(hex)]
}

// LookupAirline retrieves an airline name by ICAO or IATA code.
func (s *Service) LookupAirline(code string) string {
	return s.airlineMap[code]
}

// AircraftCount returns the number of aircraft in the database.
func (s *Service) AircraftCount() int {
	return len(s.aircraftMap)
}

// AirlineCount returns the number of airline code mappings.
func (s *Service) AirlineCount() int {
	return len(s.airlineMap)
}

// --- Airports ---

// GetAirports returns all airports within the configured display range.
func (s *Service) GetAirports() []*AirportInfo {
	if s.airports == nil {
		return []*AirportInfo{}
	}
	return s.airports
}

// GetAirport returns a single airport by ICAO ident, or nil if not found.
func (s *Service) GetAirport(ident string) *AirportInfo {
	return s.airportMap[strings.ToUpper(ident)]
}

// --- Runways ---

// GetRunways returns all runways within the configured display range.
func (s *Service) GetRunways() []*RunwayInfo {
	if s.runways == nil {
		return []*RunwayInfo{}
	}
	return s.runways
}

// GetHomeRunways returns runways for the home airport only.
func (s *Service) GetHomeRunways() []*RunwayInfo {
	return s.homeRunways
}

// GetHomeRunwayData returns the backward-compatible RunwayData struct for phase detection.
func (s *Service) GetHomeRunwayData() adsb.RunwayData {
	return s.homeRunwayData
}

// GetHomeRunwayExtensions returns the precomputed runway extension points for the home airport.
func (s *Service) GetHomeRunwayExtensions() map[string]map[string][]RunwayExtensionPoint {
	return s.homeRunwayExtensions
}

// --- Navaids ---

// GetNavaids returns all navaids within the configured display range.
func (s *Service) GetNavaids() []*NavaidInfo {
	if s.navaids == nil {
		return []*NavaidInfo{}
	}
	return s.navaids
}

// GetNavaidsByIdent returns all navaids matching the given ident (there can be multiple, e.g. collocated VOR+DME).
func (s *Service) GetNavaidsByIdent(ident string) []*NavaidInfo {
	upper := strings.ToUpper(ident)
	var result []*NavaidInfo
	for _, n := range s.navaids {
		if strings.ToUpper(n.Ident) == upper {
			result = append(result, n)
		}
	}
	return result
}

// --- Internal builders ---

// thresholdEntry matches the anonymous struct type in adsb.RunwayData.RunwayThresholds
type thresholdEntry = struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// buildHomeRunwayData converts home runways into the adsb.RunwayData format
// expected by DetectRunwayApproach/DetectRunwayDeparture.
func (s *Service) buildHomeRunwayData(homeCode string) {
	s.homeRunwayData = adsb.RunwayData{
		Airport:          homeCode,
		RunwayThresholds: make(map[string]map[string]thresholdEntry),
	}

	for _, rwy := range s.homeRunways {
		if rwy.LEIdent == "" || rwy.HEIdent == "" {
			continue
		}
		// Both ends need valid coordinates for phase detection
		if (rwy.LELatitude == 0 && rwy.LELongitude == 0) || (rwy.HELatitude == 0 && rwy.HELongitude == 0) {
			continue
		}

		pairKey := fmt.Sprintf("%s-%s", rwy.LEIdent, rwy.HEIdent)
		thresholds := make(map[string]thresholdEntry)
		thresholds[rwy.LEIdent] = thresholdEntry{
			Latitude: rwy.LELatitude, Longitude: rwy.LELongitude,
		}
		thresholds[rwy.HEIdent] = thresholdEntry{
			Latitude: rwy.HELatitude, Longitude: rwy.HELongitude,
		}
		s.homeRunwayData.RunwayThresholds[pairKey] = thresholds
	}
}

// buildHomeRunwayExtensions precomputes runway extension points for the home airport.
func (s *Service) buildHomeRunwayExtensions(extensionLengthNM float64) {
	if extensionLengthNM <= 0 {
		extensionLengthNM = 10.0
	}

	s.homeRunwayExtensions = make(map[string]map[string][]RunwayExtensionPoint)

	for pairKey, thresholds := range s.homeRunwayData.RunwayThresholds {
		s.homeRunwayExtensions[pairKey] = make(map[string][]RunwayExtensionPoint)

		for endID, threshold := range thresholds {
			// Find opposite end
			var opposite thresholdEntry
			for otherID, otherThreshold := range thresholds {
				if otherID != endID {
					opposite = otherThreshold
					break
				}
			}

			// Bearing from this end to opposite end
			bearing := calculateBearing(
				threshold.Latitude, threshold.Longitude,
				opposite.Latitude, opposite.Longitude,
			)
			// Extension goes in the opposite direction
			oppositeBearing := math.Mod(bearing+180, 360)

			points := []RunwayExtensionPoint{
				{Latitude: threshold.Latitude, Longitude: threshold.Longitude, Distance: 0},
			}

			for d := 1.0; d <= extensionLengthNM; d += 1.0 {
				lat, lon := calculateDestinationPoint(
					threshold.Latitude, threshold.Longitude,
					oppositeBearing, d,
				)
				points = append(points, RunwayExtensionPoint{
					Latitude: lat, Longitude: lon, Distance: d,
				})
			}

			s.homeRunwayExtensions[pairKey][endID] = points
		}
	}
}
