package reference

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// loadAircraftCSV parses aircraft.csv (semicolon-separated, no header)
// Format: Hex;Registration;TypeCode;??;ManufacturerModel;Year;Owner;??
func loadAircraftCSV(path string) (map[string]*AircraftInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open aircraft.csv: %w", err)
	}
	defer f.Close()

	m := make(map[string]*AircraftInfo, 500000) // pre-size for ~500k aircraft
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 512), 1024*1024) // 1MB line buffer
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ";", 8)
		if len(parts) < 3 {
			continue
		}

		hex := strings.ToUpper(strings.TrimSpace(parts[0]))
		if hex == "" {
			continue
		}

		info := &AircraftInfo{Hex: hex}
		if len(parts) > 1 {
			info.Registration = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			info.TypeCode = strings.TrimSpace(parts[2])
		}
		// parts[3] is unknown field, skip
		if len(parts) > 4 {
			info.ManufacturerModel = strings.TrimSpace(parts[4])
		}
		if len(parts) > 5 {
			info.Year = strings.TrimSpace(parts[5])
		}
		if len(parts) > 6 {
			info.Owner = strings.TrimSpace(parts[6])
		}

		m[hex] = info
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan aircraft.csv: %w", err)
	}
	return m, nil
}

// loadAirlineDAT parses airlines.dat (comma-separated, no header)
// Format: ID,Name,Alias,IATA,ICAO,Callsign,Country,Active
// Returns map[ICAO/IATA code] -> AirlineInfo (name + country)
func loadAirlineDAT(path string) (map[string]AirlineInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open airlines.dat: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // variable field count

	m := make(map[string]AirlineInfo, 7000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed lines
		}
		if len(record) < 7 {
			continue
		}

		name := cleanDATField(record[1])
		iata := cleanDATField(record[3])
		icao := cleanDATField(record[4])
		country := cleanDATField(record[6])

		if name == "" {
			continue
		}

		info := AirlineInfo{Name: name, Country: country}
		if icao != "" && icao != "N/A" {
			m[icao] = info
		}
		if iata != "" && iata != "-" && iata != "N/A" {
			m[iata] = info
		}
	}
	return m, nil
}

// cleanDATField handles \N (null) and trims whitespace from OpenFlights .dat fields.
func cleanDATField(s string) string {
	s = strings.TrimSpace(s)
	if s == `\N` || s == "" {
		return ""
	}
	return s
}

// loadAirportsCSV parses airports.csv (OurAirports, with header)
// Returns only airports within rangeNM of station, plus the slice and ident map.
func loadAirportsCSV(path string, stationLat, stationLon, rangeNM float64) ([]*AirportInfo, map[string]*AirportInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open airports.csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	// Read header
	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read airports.csv header: %w", err)
	}
	idx := buildIndex(header)

	var airports []*AirportInfo
	airportMap := make(map[string]*AirportInfo)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		lat := parseFloat(getField(record, idx, "latitude_deg"))
		lon := parseFloat(getField(record, idx, "longitude_deg"))
		if lat == 0 && lon == 0 {
			continue
		}

		dist := haversineNM(stationLat, stationLon, lat, lon)
		if dist > rangeNM {
			continue
		}

		apType := getField(record, idx, "type")
		if apType == "closed" {
			continue
		}

		ident := getField(record, idx, "ident")
		ap := &AirportInfo{
			ID:           parseInt(getField(record, idx, "id")),
			Ident:        ident,
			Type:         apType,
			Name:         getField(record, idx, "name"),
			Latitude:     lat,
			Longitude:    lon,
			ElevationFt:  parseFloat(getField(record, idx, "elevation_ft")),
			Continent:    getField(record, idx, "continent"),
			ISOCountry:   getField(record, idx, "iso_country"),
			ISORegion:    getField(record, idx, "iso_region"),
			Municipality: getField(record, idx, "municipality"),
			IATACode:     getField(record, idx, "iata_code"),
		}
		airports = append(airports, ap)
		if ident != "" {
			airportMap[ident] = ap
		}
	}
	return airports, airportMap, nil
}

// loadFrequenciesCSV parses airport-frequencies.csv and attaches frequencies to airports in the map.
func loadFrequenciesCSV(path string, airportMap map[string]*AirportInfo) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open airport-frequencies.csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read airport-frequencies.csv header: %w", err)
	}
	idx := buildIndex(header)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		ident := getField(record, idx, "airport_ident")
		ap, ok := airportMap[ident]
		if !ok {
			continue
		}

		freq := FrequencyInfo{
			ID:           parseInt(getField(record, idx, "id")),
			AirportIdent: ident,
			Type:         getField(record, idx, "type"),
			Description:  getField(record, idx, "description"),
			FrequencyMHz: parseFloat(getField(record, idx, "frequency_mhz")),
		}
		ap.Frequencies = append(ap.Frequencies, freq)
	}
	return nil
}

// loadRunwaysCSV parses runways.csv and returns geo-filtered runways + home airport runways.
func loadRunwaysCSV(path string, airportMap map[string]*AirportInfo, homeAirportCode string) ([]*RunwayInfo, []*RunwayInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open runways.csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read runways.csv header: %w", err)
	}
	idx := buildIndex(header)

	var allRunways []*RunwayInfo
	var homeRunways []*RunwayInfo

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		airportIdent := getField(record, idx, "airport_ident")
		_, inRange := airportMap[airportIdent]
		isHome := strings.EqualFold(airportIdent, homeAirportCode)

		if !inRange && !isHome {
			continue
		}

		// Skip closed runways
		if getField(record, idx, "closed") == "1" {
			continue
		}

		rwy := &RunwayInfo{
			ID:            parseInt(getField(record, idx, "id")),
			AirportIdent:  airportIdent,
			LengthFt:      parseInt(getField(record, idx, "length_ft")),
			WidthFt:       parseInt(getField(record, idx, "width_ft")),
			Surface:       getField(record, idx, "surface"),
			Lighted:       getField(record, idx, "lighted") == "1",
			Closed:        false,
			LEIdent:       getField(record, idx, "le_ident"),
			LELatitude:    parseFloat(getField(record, idx, "le_latitude_deg")),
			LELongitude:   parseFloat(getField(record, idx, "le_longitude_deg")),
			LEElevationFt: parseFloat(getField(record, idx, "le_elevation_ft")),
			LEHeadingDegT: parseFloat(getField(record, idx, "le_heading_degT")),
			LEDisplacedFt: parseFloat(getField(record, idx, "le_displaced_threshold_ft")),
			HEIdent:       getField(record, idx, "he_ident"),
			HELatitude:    parseFloat(getField(record, idx, "he_latitude_deg")),
			HELongitude:   parseFloat(getField(record, idx, "he_longitude_deg")),
			HEElevationFt: parseFloat(getField(record, idx, "he_elevation_ft")),
			HEHeadingDegT: parseFloat(getField(record, idx, "he_heading_degT")),
			HEDisplacedFt: parseFloat(getField(record, idx, "he_displaced_threshold_ft")),
		}

		// Only include runways that have at least one end with valid coordinates
		hasCoords := (rwy.LELatitude != 0 || rwy.LELongitude != 0) ||
			(rwy.HELatitude != 0 || rwy.HELongitude != 0)
		if !hasCoords {
			continue
		}

		allRunways = append(allRunways, rwy)
		if isHome {
			homeRunways = append(homeRunways, rwy)
		}
	}
	return allRunways, homeRunways, nil
}

// loadNavaidsCSV parses navaids.csv and returns navaids within rangeNM of station.
func loadNavaidsCSV(path string, stationLat, stationLon, rangeNM float64) ([]*NavaidInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open navaids.csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read navaids.csv header: %w", err)
	}
	idx := buildIndex(header)

	var navaids []*NavaidInfo

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		lat := parseFloat(getField(record, idx, "latitude_deg"))
		lon := parseFloat(getField(record, idx, "longitude_deg"))
		if lat == 0 && lon == 0 {
			continue
		}

		dist := haversineNM(stationLat, stationLon, lat, lon)
		if dist > rangeNM {
			continue
		}

		nav := &NavaidInfo{
			ID:                parseInt(getField(record, idx, "id")),
			Ident:             getField(record, idx, "ident"),
			Name:              getField(record, idx, "name"),
			Type:              getField(record, idx, "type"),
			FrequencyKHz:      parseFloat(getField(record, idx, "frequency_khz")),
			Latitude:          lat,
			Longitude:         lon,
			ElevationFt:       parseFloat(getField(record, idx, "elevation_ft")),
			ISOCountry:        getField(record, idx, "iso_country"),
			MagneticVariation: parseFloat(getField(record, idx, "magnetic_variation_deg")),
			UsageType:         getField(record, idx, "usageType"),
			Power:             getField(record, idx, "power"),
			AssociatedAirport: getField(record, idx, "associated_airport"),
		}
		navaids = append(navaids, nav)
	}
	return navaids, nil
}

// --- CSV parsing helpers ---

// buildIndex creates a column-name → index map from a CSV header row.
func buildIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

// getField safely retrieves a field from a CSV record by column name.
func getField(record []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}
