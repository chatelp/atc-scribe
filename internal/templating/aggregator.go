package templating

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/frequencies"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/pkg/logger"
)

const activeRunwayProbabilityThreshold = 0.15
const defaultTranscriptionHistorySeconds = 600

// DataAggregator collects and formats airspace data for template rendering
type DataAggregator struct {
	adsbService          *adsb.Service
	weatherService       *weather.Service
	transcriptionStorage *sqlite.TranscriptionStorage
	frequencyService     *frequencies.Service
	config               *config.Config
	logger               *logger.Logger
}

// NewDataAggregator creates a new data aggregator
func NewDataAggregator(
	adsbService *adsb.Service,
	weatherService *weather.Service,
	transcriptionStorage *sqlite.TranscriptionStorage,
	frequencyService *frequencies.Service,
	config *config.Config,
	logger *logger.Logger,
) *DataAggregator {
	return &DataAggregator{
		adsbService:          adsbService,
		weatherService:       weatherService,
		transcriptionStorage: transcriptionStorage,
		frequencyService:     frequencyService,
		config:               config,
		logger:               logger.Named("template-aggregator"),
	}
}

// GetTemplateContext aggregates all current airspace data for templating
func (da *DataAggregator) GetTemplateContext(opts FormattingOptions) (*TemplateContext, error) {
	// Override max aircraft with config value if available for ATC chat
	maxAircraft := opts.MaxAircraft
	if opts.IncludeTranscriptionHistory && da.config.ATCChat.MaxContextAircraft > 0 {
		maxAircraft = da.config.ATCChat.MaxContextAircraft
	}

	da.logger.Debug("Aggregating template context",
		logger.Int("max_aircraft", maxAircraft),
		logger.Int("config_max_aircraft", da.config.ATCChat.MaxContextAircraft),
		logger.Bool("include_weather", opts.IncludeWeather),
		logger.Bool("include_runways", opts.IncludeRunways),
		logger.Bool("include_transcription_history", opts.IncludeTranscriptionHistory))

	context := &TemplateContext{
		Timestamp: time.Now().UTC(),
		Airport:   da.getAirportInfo(),
	}

	// Get aircraft data
	aircraft, err := da.getAircraftData(maxAircraft)
	if err != nil {
		da.logger.Error("Failed to get aircraft data", logger.Error(err))
		// Continue with empty aircraft list rather than failing completely
		aircraft = []*adsb.Aircraft{}
	}
	context.Aircraft = aircraft

	// Get weather data if requested
	if opts.IncludeWeather {
		weatherData, err := da.getWeatherData()
		if err != nil {
			da.logger.Error("Failed to get weather data", logger.Error(err))
			// Continue with nil weather rather than failing completely
		}
		context.Weather = weatherData
	}

	// Get runway data if requested
	if opts.IncludeRunways {
		runways, err := da.getRunwayData()
		if err != nil {
			da.logger.Error("Failed to get runway data", logger.Error(err))
			// Continue with empty runways rather than failing completely
			runways = []RunwayInfo{}
		}
		context.Runways = runways
		context.ActiveRunways = da.getActiveRunwayScores(2)
	}

	// Get recent communications if requested (only for ATC Chat)
	if opts.IncludeTranscriptionHistory {
		communications, err := da.getRecentCommunications()
		if err != nil {
			da.logger.Error("Failed to get recent communications", logger.Error(err))
			// Continue with empty communications rather than failing completely
			communications = []TranscriptionSummary{}
		}
		context.TranscriptionHistory = communications
	}

	da.logger.Debug("Template context aggregated",
		logger.Int("aircraft_count", len(context.Aircraft)),
		logger.Int("runway_count", len(context.Runways)),
		logger.Int("communication_count", len(context.TranscriptionHistory)))

	return context, nil
}

func (da *DataAggregator) getActiveRunwayScores(maxRunways int) []adsb.RunwayScore {
	if da.adsbService == nil || maxRunways <= 0 {
		return nil
	}

	scores := da.adsbService.GetRunwayInUseScores(10)
	if len(scores) == 0 {
		return nil
	}

	detected := make([]adsb.RunwayScore, 0, maxRunways)
	for _, score := range scores {
		if score.Probability < activeRunwayProbabilityThreshold {
			continue
		}
		detected = append(detected, score)
		if len(detected) >= maxRunways {
			break
		}
	}

	return detected
}

// getAircraftData retrieves aircraft data with distance filtering
func (da *DataAggregator) getAircraftData(maxAircraft int) ([]*adsb.Aircraft, error) {
	// Get aircraft from ADSB service
	allAircraft := da.adsbService.GetAllAircraft()

	if len(allAircraft) == 0 {
		return []*adsb.Aircraft{}, nil
	}

	// First filter: Only include active aircraft (exclude signal_lost, stale, etc.)
	var activeAircraft []*adsb.Aircraft
	for _, ac := range allAircraft {
		if ac.Status == "active" {
			activeAircraft = append(activeAircraft, ac)
		}
	}

	da.logger.Debug("Filtered aircraft by status",
		logger.Int("total_aircraft", len(allAircraft)),
		logger.Int("active_aircraft", len(activeAircraft)))

	if len(activeAircraft) == 0 {
		return []*adsb.Aircraft{}, nil
	}

	// Filter by distance from airport
	airport := da.getAirportInfo()
	var aircraft []*adsb.Aircraft
	if len(airport.Coordinates) >= 2 {
		radius := da.config.Station.AirportRangeNM

		for _, ac := range activeAircraft {
			if ac.ADSB != nil && ac.ADSB.HasPosition() {
				lat, lon, _ := ac.ADSB.Position()
				distance := da.calculateDistance(lat, lon, airport.Coordinates[0], airport.Coordinates[1])

				// Include if within radius OR if airborne (preserve all airborne traffic)
				if distance <= radius || !ac.OnGround {
					ac.Distance = &distance
					adsb.AttachATCDerivedMetrics(ac)
					aircraft = append(aircraft, ac)
				}
			}
		}
	} else {
		// If no airport coordinates, just use active aircraft
		aircraft = activeAircraft
	}

	da.logger.Debug("Filtered aircraft by distance",
		logger.Int("active_aircraft", len(activeAircraft)),
		logger.Int("filtered_aircraft", len(aircraft)))

	// Limit the number of aircraft
	if len(aircraft) > maxAircraft {
		aircraft = aircraft[:maxAircraft]
	}

	return aircraft, nil
}

// getWeatherData retrieves current weather information
func (da *DataAggregator) getWeatherData() (*weather.WeatherData, error) {
	if da.weatherService == nil {
		return nil, fmt.Errorf("weather service not available")
	}

	weatherData := da.weatherService.GetWeatherData()
	if weatherData == nil {
		return nil, fmt.Errorf("no weather data available")
	}

	return weatherData, nil
}

// getRunwayData retrieves runway configuration from the reference data loaded at startup
func (da *DataAggregator) getRunwayData() ([]RunwayInfo, error) {
	if da.adsbService == nil {
		return nil, fmt.Errorf("adsb service not available")
	}

	rwData := da.adsbService.GetRunwayData()
	if len(rwData.RunwayThresholds) == 0 {
		return []RunwayInfo{}, nil
	}

	// Extract unique runway end identifiers from the threshold data
	var runways []RunwayInfo
	for _, thresholds := range rwData.RunwayThresholds {
		for endID := range thresholds {
			runways = append(runways, RunwayInfo{
				Name:       endID,
				Active:     true,
				Operations: []string{"departure", "arrival"},
			})
		}
	}
	return runways, nil
}

// getRecentCommunications retrieves recent radio communications
func (da *DataAggregator) getRecentCommunications() ([]TranscriptionSummary, error) {
	if da.transcriptionStorage == nil {
		return []TranscriptionSummary{}, nil
	}

	// Get recent transcriptions (default: last 10 minutes)
	timeWindowSeconds := defaultTranscriptionHistorySeconds
	if da.config.ATCChat.TranscriptionHistorySeconds > 0 {
		timeWindowSeconds = da.config.ATCChat.TranscriptionHistorySeconds
	}

	since := time.Now().UTC().Add(-time.Duration(timeWindowSeconds) * time.Second)
	endTime := time.Now().UTC()

	transcriptions, err := da.transcriptionStorage.GetTranscriptionsByTimeRange(since, endTime, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transcriptions: %w", err)
	}

	// Convert to TranscriptionSummary format
	var communications []TranscriptionSummary
	for _, t := range transcriptions {
		content := strings.TrimSpace(t.ContentProcessed)
		if content == "" {
			content = strings.TrimSpace(t.Content)
		}

		if content == "" {
			continue
		}

		// Get frequency name
		frequencyName := t.FrequencyID
		if da.frequencyService != nil {
			if freq, ok := da.frequencyService.GetFrequencyByID(t.FrequencyID); ok {
				frequencyName = freq.Name
			}
		}

		communications = append(communications, TranscriptionSummary{
			Timestamp: t.CreatedAt,
			Frequency: frequencyName,
			Content:   content,
			Speaker:   t.SpeakerType,
			Callsign:  t.Callsign,
		})
	}

	return communications, nil
}

// getAirportInfo returns airport information from config
func (da *DataAggregator) getAirportInfo() AirportInfo {
	// Generate airport name from code if not available in config
	airportName := da.config.Station.AirportCode
	if da.config.Station.AirportCode != "" {
		airportName = "Airport " + da.config.Station.AirportCode
	}

	return AirportInfo{
		Code:        da.config.Station.AirportCode,
		Name:        airportName,
		Coordinates: []float64{da.config.Station.Latitude, da.config.Station.Longitude},
		ElevationFt: int(da.config.Station.ElevationFeet),
	}
}

// calculateDistance calculates the distance between two points using Haversine formula
func (da *DataAggregator) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 3440.07 // Earth radius in nautical miles

	// Convert degrees to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
