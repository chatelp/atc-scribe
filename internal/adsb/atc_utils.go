package adsb

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/config"
)

// Constants for aviation calculations
const (
	// DEPRECATED: These constants are replaced by config values
	// FLYING_MIN_TAS = 50.0  // DEPRECATED: Use config.FlightPhasesConfig.FlyingMinTASKts instead
	// FLYING_MIN_ALT = 700.0 // DEPRECATED: Use config.FlightPhasesConfig.FlyingMinAltFt instead

	// Conversion factors
	METERS_PER_NM  = 1852.0  // Meters per nautical mile
	FEET_PER_NM    = 6076.12 // Feet per nautical mile
	FEET_PER_METER = 3.28084 // Feet per meter

	// Speed adjustment constants for trajectory prediction
	SPEED_ADJUST_RANGE_NM = 10.0 // Range in nautical miles where speed adjustments apply
	SPEED_ADJUST_PERCENT  = 0.25 // Maximum speed adjustment (25%)
)

// ValidateSensorData detects and corrects likely sensor errors when values suddenly drop to 0
// from previously high values, which commonly happens when aircraft leave ADS-B range.
// Enhanced to detect impossible altitude/speed drops that would indicate signal loss rather than actual flight changes.
func ValidateSensorData(currentTAS, currentGS, currentAlt, prevTAS, prevGS, prevAlt, aircraftLat, aircraftLon, stationLat, stationLon, airportRangeNM float64, config *config.FlightPhasesConfig) (float64, float64, float64) {
	correctedTAS := currentTAS
	correctedGS := currentGS
	correctedAlt := currentAlt

	// Calculate distance to airport for context
	distanceToAirportNM := MetersToNM(Haversine(aircraftLat, aircraftLon, stationLat, stationLon))

	// ENHANCED LOGIC: Detect impossible altitude drops regardless of distance to airport
	// Aircraft cannot go from cruise altitude to zero instantly - this indicates signal loss
	if currentAlt == 0 && prevAlt > config.ImpossibleAltDropThresholdFt {
		// Impossible drop from cruise altitude to zero - definitely sensor error
		correctedAlt = prevAlt
	} else if currentAlt == 0 && prevAlt > 5000 && distanceToAirportNM > airportRangeNM {
		// High altitude drop when far from airport - likely sensor error
		correctedAlt = prevAlt
	} else if currentAlt == 0 && prevAlt > 1000 && distanceToAirportNM > airportRangeNM {
		// Original logic: only apply when far from airport for lower altitudes
		correctedAlt = prevAlt
	}

	// ENHANCED LOGIC: Detect impossible speed drops
	// Aircraft at high altitude cannot suddenly have zero speed
	if currentTAS == 0 && prevTAS > config.ImpossibleSpeedDropThresholdKts && prevAlt > config.ImpossibleSpeedDropMinAltFt {
		// High speed aircraft at altitude cannot suddenly have zero TAS - sensor error
		correctedTAS = prevTAS
	} else if currentTAS == 0 && prevTAS > 42 && distanceToAirportNM > airportRangeNM {
		// Original logic: apply when far from airport
		correctedTAS = prevTAS
	}

	if currentGS == 0 && prevGS > config.ImpossibleSpeedDropThresholdKts && prevAlt > config.ImpossibleSpeedDropMinAltFt {
		// High speed aircraft at altitude cannot suddenly have zero GS - sensor error
		correctedGS = prevGS
	} else if currentGS == 0 && prevGS > 42 && distanceToAirportNM > airportRangeNM {
		// Original logic: apply when far from airport
		correctedGS = prevGS
	}

	// ADDITIONAL LOGIC: Detect impossible combined drops
	// If all three values (alt, TAS, GS) drop to zero simultaneously from high values,
	// this is almost certainly signal loss, not a legitimate flight state change
	if currentAlt == 0 && currentTAS == 0 && currentGS == 0 &&
		prevAlt > 1000 && (prevTAS > 50 || prevGS > 50) {
		// All values dropped to zero from flying state - definitely signal loss
		correctedAlt = prevAlt
		correctedTAS = prevTAS
		correctedGS = prevGS
	}

	return correctedTAS, correctedGS, correctedAlt
}

// IsFlying determines if an aircraft is considered to be flying based on speed and altitude
// If TAS (True Airspeed) is 0, it uses ground speed (GS) as a backup
// Also handles special case for helicopters (high altitude, lower speed)
func IsFlying(tas, gs, altitude float64, config *config.FlightPhasesConfig) bool {
	// If TAS is 0, use ground speed as a backup
	speed := tas
	if speed == 0 {
		speed = gs
	}

	// High altitude override: If altitude is very high, aircraft must be flying
	// regardless of speed data (handles bad ADSB speed data at cruise altitude)
	if altitude >= config.HighAltitudeOverrideFt {
		return true
	}

	// Normal case: speed and altitude both above thresholds
	if speed >= config.FlyingMinTASKts && altitude >= config.FlyingMinAltFt {
		return true
	}

	// Special case for helicopters: altitude is at least helicopterMultiplier x the threshold,
	// but speed is more than half the threshold
	if altitude >= (config.FlyingMinAltFt*config.HelicopterAltMultiplier) && speed > (config.FlyingMinTASKts/2) {
		return true
	}

	// Edge case: Very high speed with low/zero altitude likely indicates
	// sensor error or aircraft leaving ADS-B range - prioritize speed over altitude
	if speed >= config.HighSpeedThresholdKts {
		return true
	}

	return false
}

// Haversine calculates the distance in meters between two lat/lon points.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	rad := math.Pi / 180.0

	lat1Rad := lat1 * rad
	lon1Rad := lon1 * rad
	lat2Rad := lat2 * rad
	lon2Rad := lon2 * rad

	dlon := lon2Rad - lon1Rad
	dlat := lat2Rad - lat1Rad

	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(dlon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// CalculateBearing calculates the bearing in degrees from point 1 to point 2
// Returns a value between 0 and 360 degrees (0 = North, 90 = East, etc.)
func CalculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180.0
	lon1Rad := lon1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	lon2Rad := lon2 * math.Pi / 180.0

	// Calculate the bearing
	y := math.Sin(lon2Rad-lon1Rad) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(lon2Rad-lon1Rad)
	bearing := math.Atan2(y, x) * 180.0 / math.Pi

	// Convert to 0-360 degrees
	bearing = math.Mod(bearing+360.0, 360.0)

	return bearing
}

// CalculateRelativeBearing calculates the relative bearing from aircraft 1 to aircraft 2
// based on aircraft 1's heading. Returns a value between 0 and 360 degrees.
// This is the standard aviation "clock position" relative to the aircraft's heading.
func CalculateRelativeBearing(lat1, lon1, heading1, lat2, lon2 float64) float64 {
	// Calculate absolute bearing from aircraft 1 to aircraft 2
	absoluteBearing := CalculateBearing(lat1, lon1, lat2, lon2)

	// Calculate relative bearing
	relativeBearing := absoluteBearing - heading1

	// Normalize to 0-360 degrees
	relativeBearing = math.Mod(relativeBearing+360.0, 360.0)

	return relativeBearing
}

// MetersToNM converts meters to nautical miles
func MetersToNM(meters float64) float64 {
	return meters / METERS_PER_NM
}

// NMToMeters converts nautical miles to meters
func NMToMeters(nm float64) float64 {
	return nm * METERS_PER_NM
}

// FeetToMeters converts feet to meters
func FeetToMeters(feet float64) float64 {
	return feet / FEET_PER_METER
}

// MetersToFeet converts meters to feet
func MetersToFeet(meters float64) float64 {
	return meters * FEET_PER_METER
}

// ParseCoordinates parses a string in the format "lat,lon" to float64 values
func ParseCoordinates(coordStr string) (float64, float64, error) {
	parts := strings.Split(coordStr, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid coordinate format, expected 'lat,lon'")
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %w", err)
	}

	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %w", err)
	}

	return lat, lon, nil
}

// IsHexCode checks if a string is a valid hex code (ICAO address)
func IsHexCode(s string) bool {
	hexPattern := regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	return hexPattern.MatchString(s)
}

// IsFlightNumber checks if a string is likely a flight number
func IsFlightNumber(s string) bool {
	// Most flight numbers are 2-3 letters followed by 1-4 digits
	flightPattern := regexp.MustCompile(`^[A-Za-z]{2,3}[0-9]{1,4}$`)
	return flightPattern.MatchString(s)
}

// IsTailNumber checks if a string is likely a tail/registration number
func IsTailNumber(s string) bool {
	// Common tail number patterns:
	// N-numbers (US): N followed by 1-5 digits or 1-4 digits followed by 1-2 letters
	// C-XXXX (Canada): C- followed by 4 characters
	// G-XXXX (UK): G- followed by 4 characters
	tailPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^N[0-9]{1,5}$`),              // N12345
		regexp.MustCompile(`^N[0-9]{1,4}[A-Za-z]{1,2}$`), // N123AB
		regexp.MustCompile(`^[A-Z]-[A-Z0-9]{4}$`),        // C-FKWZ, G-ABCD
		regexp.MustCompile(`^[A-Z]{2}-[A-Z0-9]{3,4}$`),   // VH-ABC, JA-8089
		regexp.MustCompile(`^[A-Z]{3}[0-9]{1,4}[A-Z]?$`), // Various other formats
	}

	for _, pattern := range tailPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}

	return false
}

// --- ICAO to Tail Number Conversion Functions ---

// Constants for ICAO-to-tail number conversion
const (
	icaoSize = 6 // Size of ICAO hex address

	usCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ" // alphabet without I and O
	digitset  = "0123456789"

	caAlphabet      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	caAlphabetLen   = 26
	caMax3LetterVal = caAlphabetLen * caAlphabetLen * caAlphabetLen // 17576 (for C-Fxxx or C-Gxxx each)
)

var usAllChars = usCharset + digitset

// Precomputed constants for US conversion
var (
	usSuffixSize  int
	usBucket4Size int
	usBucket3Size int
	usBucket2Size int
	usBucket1Size int

	usCharsetLen  int
	usDigitsetLen int
	usAllCharsLen int
)

func init() {
	// Initialize US conversion constants
	usCharsetLen = len(usCharset)
	usDigitsetLen = len(digitset)
	usAllCharsLen = len(usAllChars)

	usSuffixSize = 1 + usCharsetLen*(1+usCharsetLen)
	usBucket4Size = 1 + usCharsetLen + usDigitsetLen
	usBucket3Size = usDigitsetLen*usBucket4Size + usSuffixSize
	usBucket2Size = usDigitsetLen*usBucket3Size + usSuffixSize
	usBucket1Size = usDigitsetLen*usBucket2Size + usSuffixSize
}

// CleanFlightName removes whitespace and null characters from flight names
func CleanFlightName(flight string) string {
	return strings.TrimSpace(strings.ReplaceAll(flight, "\\x00", ""))
}

// getSuffixUS computes the suffix for the US tail number given an offset.
func getSuffixUS(offset int) string {
	if offset == 0 {
		return ""
	}
	char0Idx := (offset - 1) / (usCharsetLen + 1)
	if char0Idx < 0 || char0Idx >= usCharsetLen {
		return fmt.Sprintf("!ERR_IDX_%d!", char0Idx)
	}
	char0 := string(usCharset[char0Idx])

	rem := (offset - 1) % (usCharsetLen + 1)
	if rem == 0 {
		return char0
	}
	if rem-1 < 0 || rem-1 >= usCharsetLen {
		return fmt.Sprintf("!ERR_REM_IDX_%d!", rem-1)
	}
	return char0 + string(usCharset[rem-1])
}

// USIcaoToN converts a US ICAO address to its N-Number.
func USIcaoToN(icaoUpper string) (string, error) {
	valHex := icaoUpper[1:]
	parsedVal, err := strconv.ParseInt(valHex, 16, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse US ICAO hex '%s': %v", valHex, err)
	}
	idx := int(parsedVal) - 1

	if idx < 0 || idx > 915398 { // Valid US range: A00001 to ADF7C7
		return "", fmt.Errorf("ICAO value %s (idx %d) out of valid range for US N-Number mapping (A00001-ADF7C7)", icaoUpper, idx)
	}

	output := "N"
	dig1 := (idx / usBucket1Size) + 1
	rem1 := idx % usBucket1Size
	output += strconv.Itoa(dig1)

	if rem1 < usSuffixSize {
		return output + getSuffixUS(rem1), nil
	}

	rem1 -= usSuffixSize
	dig2 := rem1 / usBucket2Size
	rem2 := rem1 % usBucket2Size
	output += strconv.Itoa(dig2)

	if rem2 < usSuffixSize {
		return output + getSuffixUS(rem2), nil
	}

	rem2 -= usSuffixSize
	dig3 := rem2 / usBucket3Size
	rem3 := rem2 % usBucket3Size
	output += strconv.Itoa(dig3)

	if rem3 < usSuffixSize {
		return output + getSuffixUS(rem3), nil
	}

	rem3 -= usSuffixSize
	dig4 := rem3 / usBucket4Size
	rem4 := rem3 % usBucket4Size
	output += strconv.Itoa(dig4)

	if rem4 == 0 {
		return output, nil
	}
	if rem4-1 < 0 || rem4-1 >= usAllCharsLen {
		return "", fmt.Errorf("internal error: invalid rem4 index %d for usAllChars (len %d)", rem4-1, usAllCharsLen)
	}
	return output + string(usAllChars[rem4-1]), nil
}

// CAIcaoToN converts a Canadian ICAO address to its Tail Number.
func CAIcaoToN(icaoUpper string) (string, error) {
	valHex := icaoUpper[1:]
	d, err := strconv.ParseInt(valHex, 16, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse Canadian ICAO hex '%s': %v", valHex, err)
	}

	var prefix string
	dEff := 0

	if d >= 1 && d <= caMax3LetterVal {
		prefix = "C-F"
		dEff = int(d) - 1
	} else if d >= (caMax3LetterVal+1) && d <= (caMax3LetterVal*2) {
		prefix = "C-G"
		dEff = int(d) - 1 - caMax3LetterVal
	} else {
		return "", fmt.Errorf("Canadian ICAO value %s (decimal %d) out of range for C-Fxxx or C-Gxxx mapping", icaoUpper, d)
	}

	if dEff < 0 || dEff >= caMax3LetterVal {
		return "", fmt.Errorf("internal error: dEff %d out of expected range [0, %d) for Canadian tail letters", dEff, caMax3LetterVal)
	}

	l1Idx := dEff % caAlphabetLen
	dEff /= caAlphabetLen
	l2Idx := dEff % caAlphabetLen
	dEff /= caAlphabetLen
	l3Idx := dEff % caAlphabetLen

	if l1Idx < 0 || l1Idx >= caAlphabetLen ||
		l2Idx < 0 || l2Idx >= caAlphabetLen ||
		l3Idx < 0 || l3Idx >= caAlphabetLen {
		return "", fmt.Errorf("internal error: calculated letter index out of bounds for Canadian tail letters")
	}

	tailLetters := string(caAlphabet[l3Idx]) + string(caAlphabet[l2Idx]) + string(caAlphabet[l1Idx])
	return prefix + tailLetters, nil
}

// IcaoToTailNumber converts an ICAO hex address to a tail number.
func IcaoToTailNumber(icao string) (string, error) {
	if len(icao) != icaoSize {
		return "", fmt.Errorf("ICAO hex address must be %d characters long, got %d for '%s'", icaoSize, len(icao), icao)
	}
	icaoUpper := strings.ToUpper(icao)

	for i := 1; i < icaoSize; i++ {
		isHex := false
		char := rune(icaoUpper[i])
		if (char >= '0' && char <= '9') || (char >= 'A' && char <= 'F') {
			isHex = true
		}
		if !isHex {
			return "", fmt.Errorf("ICAO hex address '%s' contains non-hex character '%c' at position %d", icao, icaoUpper[i], i+1)
		}
	}

	firstChar := icaoUpper[0]
	switch firstChar {
	case 'A':
		return USIcaoToN(icaoUpper)
	case 'C':
		return CAIcaoToN(icaoUpper)
	default:
		return "", fmt.Errorf("unsupported ICAO prefix '%c' in '%s'. Only 'A' (US) and 'C' (Canada) are supported", firstChar, icao)
	}
}

// PredictFuturePositions calculates predicted future positions for an aircraft
// based on its current position, heading, speed, and vertical rate.
// It returns an array of predicted positions at 1-minute intervals for the next 5 minutes.
// The function also adjusts speed based on proximity to the airport (station).
func PredictFuturePositions(lat, lon, altBaro, trueHeading, magHeading, speedKnots, verticalRateFtMin float64) []Position {
	predictions := make([]Position, 5) // 5 predictions (1-5 minutes ahead)
	now := time.Now().UTC()

	// Convert heading from degrees to radians for trigonometric calculations
	headingRad := trueHeading * math.Pi / 180.0

	// Calculate distance traveled per minute in degrees
	// 1 knot = 1 nautical mile per hour = 1.852 km per hour
	// 1 minute = 1/60 hour
	// Distance in km per minute = speedKnots * 1.852 / 60
	speedKmPerMin := speedKnots * 1.852 / 60

	// Approximate degrees per km (varies by latitude, but this is a reasonable approximation)
	// 1 degree of latitude = ~111 km
	// 1 degree of longitude = ~111 km * cos(latitude)
	latKmPerDegree := 111.0
	lonKmPerDegree := 111.0 * math.Cos(lat*math.Pi/180.0)

	// Get station coordinates from config
	// For now, we'll use a placeholder function that will be replaced with actual config values
	stationLat, stationLon := GetStationCoordinates()

	// Calculate initial distance to station in nautical miles (used for logging/debugging)
	_ = Haversine(lat, lon, stationLat, stationLon) / METERS_PER_NM

	// Determine if we're approaching or departing from the station based on heading
	// Calculate bearing to station
	bearingToStation := Bearing(lat, lon, stationLat, stationLon)

	// Calculate the absolute angular difference between aircraft heading and bearing to station
	// If the difference is less than 90 degrees, the aircraft is heading toward the station
	// If the difference is more than 90 degrees, the aircraft is heading away from the station
	headingDiff := math.Abs(trueHeading - bearingToStation)
	if headingDiff > 180 {
		headingDiff = 360 - headingDiff
	}

	approachingStation := headingDiff < 90

	for i := 0; i < 5; i++ {
		minutesAhead := float64(i + 1)

		// Start with the original speed
		adjustedSpeed := speedKnots
		adjustedSpeedKmPerMin := speedKmPerMin

		// Calculate new position
		latChange := (adjustedSpeedKmPerMin * minutesAhead * math.Cos(headingRad)) / latKmPerDegree
		lonChange := (adjustedSpeedKmPerMin * minutesAhead * math.Sin(headingRad)) / lonKmPerDegree

		newLat := lat + latChange
		newLon := lon + lonChange

		// Calculate distance of the predicted position to the station
		predictedDistanceToStationNM := Haversine(newLat, newLon, stationLat, stationLon) / METERS_PER_NM

		// Adjust speed based on proximity to airport if within range
		if predictedDistanceToStationNM < SPEED_ADJUST_RANGE_NM {
			// Calculate adjustment factor (0-1) based on how close we are to the airport
			adjustmentFactor := (SPEED_ADJUST_RANGE_NM - predictedDistanceToStationNM) / SPEED_ADJUST_RANGE_NM

			// Apply the adjustment based on whether we're approaching or departing
			if approachingStation {
				// Decrease speed when approaching
				adjustedSpeed = speedKnots * (1.0 - (SPEED_ADJUST_PERCENT * adjustmentFactor))
			} else {
				// Increase speed when departing
				adjustedSpeed = speedKnots * (1.0 + (SPEED_ADJUST_PERCENT * adjustmentFactor))
			}
		}

		// Calculate new altitude based on vertical rate
		// Vertical rate is in feet per minute
		newAltitude := altBaro + (verticalRateFtMin * minutesAhead)

		// If we're approaching the station and altitude is predicted to be negative,
		// adjust it to be at ground level (0 feet)
		if approachingStation && newAltitude < 0 {
			// Keep the negative value for UI warning purposes, but cap it at -100 feet
			// This allows the UI to show a warning icon while preventing extreme negative values
			if newAltitude < -100 {
				newAltitude = -100
			}
		}

		// Create prediction
		timestamp := now.Add(time.Duration(minutesAhead) * time.Minute)

		predictions[i] = Position{
			Lat:         newLat,
			Lon:         newLon,
			Altitude:    newAltitude,
			SpeedTrue:   adjustedSpeed,
			SpeedGS:     adjustedSpeed,
			TrueHeading: trueHeading, // Assuming constant true heading
			MagHeading:  magHeading,  // Assuming constant magnetic heading
			Timestamp:   timestamp,
		}
	}

	return predictions
}

// GetStationCoordinates returns the latitude and longitude of the station (airport)
// from the config. If the config is not available, it returns default values.
func GetStationCoordinates() (float64, float64) {
	// Get the config from the service
	cfg := GetConfig()
	if cfg != nil && cfg.Station.Latitude != 0 && cfg.Station.Longitude != 0 {
		return cfg.Station.Latitude, cfg.Station.Longitude
	}

	// Default to Toronto Pearson International Airport coordinates
	return 43.6777, -79.6248
}

// GetConfig returns the current configuration
// This is a placeholder that should be replaced with actual config access
var configInstance *Config

func GetConfig() *Config {
	return configInstance
}

// Config represents the application configuration
type Config struct {
	Station struct {
		Latitude  float64
		Longitude float64
	}
}

// SetConfig sets the configuration for testing purposes
func SetConfig(cfg *Config) {
	configInstance = cfg
}

// Bearing calculates the initial bearing from point 1 to point 2
func Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	// Convert to radians
	lat1 = lat1 * math.Pi / 180.0
	lon1 = lon1 * math.Pi / 180.0
	lat2 = lat2 * math.Pi / 180.0
	lon2 = lon2 * math.Pi / 180.0

	// Calculate bearing
	y := math.Sin(lon2-lon1) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
	bearing := math.Atan2(y, x) * 180.0 / math.Pi

	// Normalize to 0-360
	if bearing < 0 {
		bearing += 360.0
	}

	return bearing
}

// RunwayThreshold represents a single runway threshold with its coordinates
type RunwayThreshold struct {
	ID        string  `json:"id"` // e.g., "05", "23", "06L", "24R"
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// RunwayData represents the structure of runway data from runways.json
type RunwayData struct {
	Airport          string                         `json:"airport"`
	RunwayThresholds map[string]map[string]struct { // e.g., "05-23" -> "05" -> {lat, lon}
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"runway_thresholds"`
}

// DetectRunwayApproach determines if aircraft is on approach to any runway
func DetectRunwayApproach(lat, lon, heading, altitude float64, runways RunwayData, config config.FlightPhasesConfig) *RunwayApproachInfo {
	var bestApproach *RunwayApproachInfo
	minDistance := float64(config.ApproachMaxDistanceNM) + 1 // Start with distance beyond max

	// Check each runway threshold
	for runwayPair, thresholds := range runways.RunwayThresholds {
		for thresholdID, threshold := range thresholds {
			// Calculate distance to threshold
			distanceMeters := Haversine(lat, lon, threshold.Latitude, threshold.Longitude)
			distanceNM := MetersToNM(distanceMeters)

			// Skip if too far from threshold
			if distanceNM > float64(config.ApproachMaxDistanceNM) {
				continue
			}

			// Calculate runway heading (from opposite threshold to this threshold)
			var runwayHeading float64
			oppositeThresholdID := getOppositeThreshold(thresholdID, runwayPair)
			if oppositeThreshold, exists := thresholds[oppositeThresholdID]; exists {
				runwayHeading = CalculateBearing(oppositeThreshold.Latitude, oppositeThreshold.Longitude,
					threshold.Latitude, threshold.Longitude)
			} else {
				// If we can't find opposite threshold, skip this one
				continue
			}

			// Calculate heading alignment
			headingDiff := math.Abs(heading - runwayHeading)
			if headingDiff > 180 {
				headingDiff = 360 - headingDiff
			}

			// Skip if heading not aligned
			if headingDiff > float64(config.ApproachHeadingToleranceDeg) {
				continue
			}

			// Calculate distance from runway centerline
			runwayThreshold := RunwayThreshold{
				ID:        thresholdID,
				Latitude:  threshold.Latitude,
				Longitude: threshold.Longitude,
			}
			centerlineDistance := CalculateRunwayCenterlineDistance(lat, lon, runwayThreshold, runwayHeading)

			// Check if within centerline tolerance
			if centerlineDistance <= config.ApproachCenterlineToleranceNM {
				// This is a valid approach - check if it's the closest
				if distanceNM < minDistance {
					minDistance = distanceNM
					bestApproach = &RunwayApproachInfo{
						RunwayID:               runwayPair + "/" + thresholdID,
						DistanceToThreshold:    distanceNM,
						DistanceFromCenterline: centerlineDistance,
						HeadingAlignment:       headingDiff,
						OnApproach:             true,
					}
				}
			}
		}
	}

	return bestApproach
}

// CalculateRunwayCenterlineDistance calculates distance from aircraft to runway centerline
func CalculateRunwayCenterlineDistance(aircraftLat, aircraftLon float64, threshold RunwayThreshold, runwayHeading float64) float64 {
	// Calculate the bearing from threshold to aircraft
	bearingToAircraft := CalculateBearing(threshold.Latitude, threshold.Longitude, aircraftLat, aircraftLon)

	// Calculate the distance from threshold to aircraft
	distanceToAircraft := MetersToNM(Haversine(threshold.Latitude, threshold.Longitude, aircraftLat, aircraftLon))

	// Calculate the angle between runway heading and bearing to aircraft
	angleDiff := math.Abs(runwayHeading - bearingToAircraft)
	if angleDiff > 180 {
		angleDiff = 360 - angleDiff
	}

	// Calculate perpendicular distance (distance from centerline)
	// Using sine rule: perpendicular distance = hypotenuse * sin(angle)
	centerlineDistance := distanceToAircraft * math.Sin(angleDiff*math.Pi/180.0)

	return centerlineDistance
}

// IsOnRunwayApproach determines if aircraft meets approach criteria for a specific runway
func IsOnRunwayApproach(aircraftLat, aircraftLon, heading, altitude float64, threshold RunwayThreshold, config config.FlightPhasesConfig) bool {
	// Calculate distance to threshold
	distanceMeters := Haversine(aircraftLat, aircraftLon, threshold.Latitude, threshold.Longitude)
	distanceNM := MetersToNM(distanceMeters)

	// Check distance constraint
	if distanceNM > float64(config.ApproachMaxDistanceNM) {
		return false
	}

	// For approach phase, we also need to be below a certain altitude (5000 feet as per plan)
	if altitude > 5000 {
		return false
	}

	// Calculate runway heading (this is simplified - in real implementation we'd need the opposite threshold)
	// For now, we'll assume we have the runway heading available
	// This would need to be enhanced with actual runway data

	return true
}

// getOppositeThreshold returns the opposite threshold ID for a given threshold
func getOppositeThreshold(thresholdID, runwayPair string) string {
	// Split runway pair (e.g., "05-23" -> ["05", "23"])
	parts := strings.Split(runwayPair, "-")
	if len(parts) != 2 {
		return ""
	}

	// Return the opposite threshold
	if thresholdID == parts[0] {
		return parts[1]
	}
	return parts[0]
}

// DetectRunwayDeparture determines if aircraft is departing from any runway
// Unlike approach detection, departure detection is more lenient:
// - Aircraft don't need to be on centerline (they deviate quickly after takeoff)
// - We check if aircraft is moving away from the airport/runways
// - Distance tolerance is larger since aircraft spread out after departure
func DetectRunwayDeparture(lat, lon, heading float64, runways RunwayData, stationLat, stationLon float64, config config.FlightPhasesConfig) *RunwayDepartureInfo {
	var bestDeparture *RunwayDepartureInfo
	minDistance := float64(config.ApproachMaxDistanceNM) + 1 // Start with distance beyond max

	// Check each runway threshold
	for runwayPair, thresholds := range runways.RunwayThresholds {
		for thresholdID, threshold := range thresholds {
			// Calculate distance from threshold
			distanceMeters := Haversine(lat, lon, threshold.Latitude, threshold.Longitude)
			distanceNM := MetersToNM(distanceMeters)

			// Skip if too far from threshold (use larger tolerance for departures)
			maxDepartureDistance := float64(config.ApproachMaxDistanceNM) * 1.5 // 1.5x approach distance
			if distanceNM > maxDepartureDistance {
				continue
			}

			// Calculate runway heading (from this threshold outbound)
			var runwayHeading float64
			oppositeThresholdID := getOppositeThreshold(thresholdID, runwayPair)
			if oppositeThreshold, exists := thresholds[oppositeThresholdID]; exists {
				runwayHeading = CalculateBearing(threshold.Latitude, threshold.Longitude,
					oppositeThreshold.Latitude, oppositeThreshold.Longitude)
			} else {
				// If we can't find opposite threshold, skip this one
				continue
			}

			// Calculate heading alignment (more lenient for departures)
			headingDiff := math.Abs(heading - runwayHeading)
			if headingDiff > 180 {
				headingDiff = 360 - headingDiff
			}

			// More lenient heading tolerance for departures (aircraft deviate quickly)
			departureHeadingTolerance := float64(config.ApproachHeadingToleranceDeg) * 2.0 // 2x approach tolerance
			if headingDiff <= departureHeadingTolerance {
				// Check if aircraft is moving away from the station
				// Calculate bearing from aircraft to station
				bearingToStation := CalculateBearing(lat, lon, stationLat, stationLon)

				// If aircraft heading is roughly opposite to bearing to station, it's moving away
				awayHeadingDiff := math.Abs(heading - bearingToStation)
				if awayHeadingDiff > 180 {
					awayHeadingDiff = 360 - awayHeadingDiff
				}

				// Aircraft is moving away if heading is roughly opposite to station bearing
				// Allow 90 degrees tolerance (aircraft can be moving perpendicular and still be departing)
				if awayHeadingDiff >= 90 {
					// This is a valid departure - check if it's the closest
					if distanceNM < minDistance {
						minDistance = distanceNM
						bestDeparture = &RunwayDepartureInfo{
							RunwayID:              runwayPair + "/" + thresholdID,
							DistanceFromThreshold: distanceNM,
							HeadingAlignment:      headingDiff,
							OnDeparture:           true,
						}
					}
				}
			}
		}
	}

	return bestDeparture
}
