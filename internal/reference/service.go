package reference

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/pkg/logger"
)

// Service provides unified access to all reference data (aircraft, airlines, airports, runways, navaids).
// All data is loaded once at startup and geo-filtered where applicable.
type Service struct {
	logger *logger.Logger

	// Global (no geo-filter)
	aircraftMap map[string]*AircraftInfo // key: uppercase hex
	airlineMap  map[string]AirlineInfo   // key: ICAO or IATA code → airline info

	// Geo-filtered within display_range_nm of station
	airports   []*AirportInfo
	airportMap map[string]*AirportInfo // key: airport ident (ICAO)
	runways    []*RunwayInfo
	navaids    []*NavaidInfo

	// Home airport specific. It can be changed from the settings panel while
	// the server runs, so everything below is read and replaced under homeMu.
	homeMu               sync.RWMutex
	homeCode             string
	homeRunways          []*RunwayInfo
	homeRunwayData       adsb.RunwayData
	homeRunwayExtensions map[string]map[string][]RunwayExtensionPoint

	stationLat, stationLon float64 // the receiver, for candidate distances
	extensionLengthNM      float64
}

// NewService creates a new reference service, loading all CSV data at startup.
func NewService(cfg ServiceConfig, log *logger.Logger) (*Service, error) {
	s := &Service{
		logger:      log.Named("reference"),
		aircraftMap: make(map[string]*AircraftInfo),
		airlineMap:  make(map[string]AirlineInfo),
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
	s.stationLat, s.stationLon = cfg.StationLat, cfg.StationLon
	s.extensionLengthNM = cfg.ExtensionLengthNM
	s.homeCode = strings.ToUpper(strings.TrimSpace(cfg.HomeAirportCode))
	s.homeRunwayData = runwayDataFor(s.homeCode, s.homeRunways)
	s.homeRunwayExtensions = runwayExtensionsFor(s.homeRunwayData, s.extensionLengthNM)

	return s, nil
}

// --- Aircraft enrichment ---

// LookupAircraft retrieves aircraft info by hex code (case-insensitive).
func (s *Service) LookupAircraft(hex string) *AircraftInfo {
	return s.aircraftMap[strings.ToUpper(hex)]
}

// LookupAirline retrieves an airline name by ICAO or IATA code.
func (s *Service) LookupAirline(code string) string {
	return s.airlineMap[code].Name
}

// LookupAirlineCountry retrieves an airline's country by ICAO or IATA code.
func (s *Service) LookupAirlineCountry(code string) string {
	return s.airlineMap[code].Country
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

// GetAirportsOnly returns airports (excluding heliports) within the display range.
func (s *Service) GetAirportsOnly() []*AirportInfo {
	result := make([]*AirportInfo, 0)
	for _, ap := range s.airports {
		if ap.Type != "heliport" {
			result = append(result, ap)
		}
	}
	return result
}

// GetHeliportsOnly returns only heliports within the display range.
func (s *Service) GetHeliportsOnly() []*AirportInfo {
	result := make([]*AirportInfo, 0)
	for _, ap := range s.airports {
		if ap.Type == "heliport" {
			result = append(result, ap)
		}
	}
	return result
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
	s.homeMu.RLock()
	defer s.homeMu.RUnlock()
	return s.homeRunways
}

// GetHomeRunwayData returns the backward-compatible RunwayData struct for phase detection.
func (s *Service) GetHomeRunwayData() adsb.RunwayData {
	s.homeMu.RLock()
	defer s.homeMu.RUnlock()
	return s.homeRunwayData
}

// GetHomeRunwayExtensions returns the precomputed runway extension points for the home airport.
func (s *Service) GetHomeRunwayExtensions() map[string]map[string][]RunwayExtensionPoint {
	s.homeMu.RLock()
	defer s.homeMu.RUnlock()
	return s.homeRunwayExtensions
}

// HomeAirportCode is the airport phases are judged against, as currently set.
func (s *Service) HomeAirportCode() string {
	s.homeMu.RLock()
	defer s.homeMu.RUnlock()
	return s.homeCode
}

// usableRunways are an airport's runways with both thresholds located -- the
// only ones phase detection can use, since a runway's heading comes from its
// two ends.
func (s *Service) usableRunways(code string) []*RunwayInfo {
	var out []*RunwayInfo
	for _, r := range s.runways {
		if !strings.EqualFold(r.AirportIdent, code) || r.LEIdent == "" || r.HEIdent == "" {
			continue
		}
		if (r.LELatitude == 0 && r.LELongitude == 0) || (r.HELatitude == 0 && r.HELongitude == 0) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// SetHomeAirport makes another airport the one phases are judged against. It
// refuses rather than guesses: an unknown code, or an airport none of whose
// runways can be used, would silently turn every approach into "unknown", which
// is the failure this setting exists to cure.
//
// Only airports within the display range are known -- the reference data is
// loaded for that circle around the receiver -- which is also the only place a
// receiver could watch an airport from.
func (s *Service) SetHomeAirport(code string) (*AirportInfo, int, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	airport, runways, err := s.checkHomeAirport(code)
	if err != nil {
		return nil, 0, err
	}
	data := runwayDataFor(code, runways)
	ext := runwayExtensionsFor(data, s.extensionLengthNM)

	s.homeMu.Lock()
	s.homeCode, s.homeRunways, s.homeRunwayData, s.homeRunwayExtensions = code, runways, data, ext
	s.homeMu.Unlock()

	s.logger.Info("Home airport changed",
		logger.String("code", code), logger.String("name", airport.Name),
		logger.Int("runways", len(runways)))
	return airport, len(runways), nil
}

// CheckHomeAirport says whether SetHomeAirport would accept code, without
// changing anything -- so a settings change can be refused before any part of
// it is applied.
func (s *Service) CheckHomeAirport(code string) (*AirportInfo, error) {
	a, _, err := s.checkHomeAirport(strings.ToUpper(strings.TrimSpace(code)))
	return a, err
}

func (s *Service) checkHomeAirport(code string) (*AirportInfo, []*RunwayInfo, error) {
	airport := s.airportMap[code]
	if airport == nil {
		return nil, nil, fmt.Errorf("unknown airport %q, or beyond the display range", code)
	}
	runways := s.usableRunways(code)
	if len(runways) == 0 {
		return nil, nil, fmt.Errorf("airport %s has no runway with both ends located", code)
	}
	return airport, runways, nil
}

// AirportRunways returns an airport's position, its runways as phase detection
// reads them, and their extended centrelines -- for an airport followed besides
// the principal one, which keeps its own copy in the home fields. Refused on the
// same terms as SetHomeAirport.
func (s *Service) AirportRunways(code string) (*AirportInfo, adsb.RunwayData, map[string]map[string][]RunwayExtensionPoint, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	airport, runways, err := s.checkHomeAirport(code)
	if err != nil {
		return nil, adsb.RunwayData{}, nil, err
	}
	data := runwayDataFor(code, runways)
	return airport, data, runwayExtensionsFor(data, s.extensionLengthNM), nil
}

// AirportCandidate is an airport the settings panel can offer as reference.
type AirportCandidate struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	DistanceNM float64 `json:"distance_nm"`
	Runways    int     `json:"runways"`
}

// HomeAirportCandidates lists the airports within maxNM of the receiver that
// have at least one usable runway, nearest first. Heliports are left out: they
// have no runway to align an approach with.
func (s *Service) HomeAirportCandidates(maxNM float64) []AirportCandidate {
	var out []AirportCandidate
	for _, a := range s.airports {
		if a.Type == "heliport" || a.Type == "closed" {
			continue
		}
		d := haversineNM(s.stationLat, s.stationLon, a.Latitude, a.Longitude)
		if d > maxNM {
			continue
		}
		n := len(s.usableRunways(a.Ident))
		if n == 0 {
			continue
		}
		out = append(out, AirportCandidate{Code: a.Ident, Name: a.Name, Type: a.Type,
			DistanceNM: math.Round(d*10) / 10, Runways: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceNM < out[j].DistanceNM })
	return out
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

// runwayDataFor converts an airport's runways into the adsb.RunwayData format
// expected by DetectRunwayApproach/DetectRunwayDeparture.
func runwayDataFor(homeCode string, runways []*RunwayInfo) adsb.RunwayData {
	data := adsb.RunwayData{
		Airport:          homeCode,
		RunwayThresholds: make(map[string]map[string]thresholdEntry),
	}

	for _, rwy := range runways {
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
		data.RunwayThresholds[pairKey] = thresholds
	}
	return data
}

// runwayExtensionsFor precomputes the extended centrelines of an airport's runways.
func runwayExtensionsFor(data adsb.RunwayData, extensionLengthNM float64) map[string]map[string][]RunwayExtensionPoint {
	if extensionLengthNM <= 0 {
		extensionLengthNM = 10.0
	}

	ext := make(map[string]map[string][]RunwayExtensionPoint)

	for pairKey, thresholds := range data.RunwayThresholds {
		ext[pairKey] = make(map[string][]RunwayExtensionPoint)

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

			ext[pairKey][endID] = points
		}
	}
	return ext
}
