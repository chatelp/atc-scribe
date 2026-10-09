package weather

import (
	"fmt"
	"sync"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Cache manages weather data caching with thread-safe operations
type Cache struct {
	cache  *WeatherCache
	config WeatherConfig
	logger *logger.Logger
	mu     sync.RWMutex
}

// NewCache creates a new weather cache manager
func NewCache(config WeatherConfig, logger *logger.Logger) *Cache {
	return &Cache{
		cache:  NewWeatherCache(),
		config: config,
		logger: logger.Named("weather-cache"),
	}
}

// Get returns the current cached weather data
// Returns nil if no data has been fetched yet
func (c *Cache) Get() *WeatherData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := c.cache.Get()
	if data == nil {
		return nil
	}

	// Check if this is just the default empty data (no actual weather data fetched)
	if data.METAR == nil && data.TAF == nil && data.NOTAMs == nil && len(data.FetchErrors) == 0 {
		return nil
	}

	return data
}

// Update updates the cache with new fetch results
func (c *Cache) Update(results []FetchResult, airportCode string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get current data or create new
	currentData := c.cache.Get()
	if currentData == nil {
		currentData = &WeatherData{}
	}

	// Create new data structure
	newData := &WeatherData{
		METAR:       currentData.METAR,
		TAF:         currentData.TAF,
		NOTAMs:      currentData.NOTAMs,
		LastUpdated: time.Now(),
		FetchErrors: []string{},
	}

	// Process fetch results
	for _, result := range results {
		switch result.Type {
		case WeatherTypeMETAR:
			if result.Err != nil {
				newData.FetchErrors = append(newData.FetchErrors, fmt.Sprintf("METAR: %s", result.Err.Error()))
				c.logger.Warn("Failed to fetch METAR data",
					logger.String("airport", airportCode),
					logger.Error(result.Err))
			} else {
				newData.METAR = result.Data
				c.logger.Debug("METAR data updated",
					logger.String("airport", airportCode))
			}

		case WeatherTypeTAF:
			if result.Err != nil {
				newData.FetchErrors = append(newData.FetchErrors, fmt.Sprintf("TAF: %s", result.Err.Error()))
				c.logger.Warn("Failed to fetch TAF data",
					logger.String("airport", airportCode),
					logger.Error(result.Err))
			} else {
				newData.TAF = result.Data
				c.logger.Debug("TAF data updated",
					logger.String("airport", airportCode))
			}

		case WeatherTypeNOTAMs:
			if result.Err != nil {
				newData.FetchErrors = append(newData.FetchErrors, fmt.Sprintf("NOTAMs: %s", result.Err.Error()))
				c.logger.Warn("Failed to fetch NOTAM data",
					logger.String("airport", airportCode),
					logger.Error(result.Err))
			} else {
				newData.NOTAMs = result.Data
				c.logger.Debug("NOTAM data updated",
					logger.String("airport", airportCode))
			}
		}
	}

	// Update cache with new data
	expiryDuration := time.Duration(c.config.CacheExpiryMinutes) * time.Minute
	c.cache.Set(newData, expiryDuration)

	// Log cache update
	successCount := len(results) - len(newData.FetchErrors)
	c.logger.Info("Weather cache updated",
		logger.String("airport", airportCode),
		logger.Int("successful_fetches", successCount),
		logger.Int("failed_fetches", len(newData.FetchErrors)),
		logger.Time("expires_at", time.Now().Add(expiryDuration)))
}

// Invalidate clears the cache
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = NewWeatherCache()
	c.logger.Info("Weather cache invalidated")
}
