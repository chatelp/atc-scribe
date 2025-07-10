package weather

import (
	"sync"
	"time"
)

// WeatherData represents the complete weather information for an airport
type WeatherData struct {
	METAR       interface{} `json:"metar,omitempty"`
	TAF         interface{} `json:"taf,omitempty"`
	NOTAMs      interface{} `json:"notams,omitempty"`
	LastUpdated time.Time   `json:"last_updated"`
	FetchErrors []string    `json:"fetch_errors,omitempty"`
}

// WeatherCache represents cached weather data with expiration
type WeatherCache struct {
	Data      *WeatherData
	ExpiresAt time.Time
	mu        sync.RWMutex
}

// WeatherConfig represents the weather service configuration
type WeatherConfig struct {
	RefreshIntervalMinutes int    `toml:"refresh_interval_minutes"`
	APIBaseURL             string `toml:"api_base_url"`
	RequestTimeoutSeconds  int    `toml:"request_timeout_seconds"`
	MaxRetries             int    `toml:"max_retries"`
	FetchMETAR             bool   `toml:"fetch_metar"`
	FetchTAF               bool   `toml:"fetch_taf"`
	FetchNOTAMs            bool   `toml:"fetch_notams"`
	CacheExpiryMinutes     int    `toml:"cache_expiry_minutes"`
}

// WeatherType represents the type of weather data
type WeatherType string

const (
	WeatherTypeMETAR  WeatherType = "metar"
	WeatherTypeTAF    WeatherType = "taf"
	WeatherTypeNOTAMs WeatherType = "notams"
)

// FetchResult represents the result of fetching weather data
type FetchResult struct {
	Type WeatherType
	Data interface{}
	Err  error
}

// IsExpired checks if the cached data has expired
func (wc *WeatherCache) IsExpired() bool {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return time.Now().After(wc.ExpiresAt)
}

// Get returns the cached weather data (thread-safe)
func (wc *WeatherCache) Get() *WeatherData {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.Data
}

// Set updates the cached weather data (thread-safe)
func (wc *WeatherCache) Set(data *WeatherData, expiryDuration time.Duration) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.Data = data
	wc.ExpiresAt = time.Now().Add(expiryDuration)
}

// NewWeatherCache creates a new weather cache instance
func NewWeatherCache() *WeatherCache {
	return &WeatherCache{
		Data: nil, // Start with no data instead of empty data
	}
}

// DefaultWeatherConfig returns the default weather configuration
func DefaultWeatherConfig() WeatherConfig {
	return WeatherConfig{
		RefreshIntervalMinutes: 10,
		APIBaseURL:             "https://node.windy.com/airports",
		RequestTimeoutSeconds:  10,
		MaxRetries:             2,
		FetchMETAR:             true,
		FetchTAF:               true,
		FetchNOTAMs:            true,
		CacheExpiryMinutes:     15,
	}
}

// ConfigWeatherConfig represents the config package's WeatherConfig
// This is used to avoid circular imports
type ConfigWeatherConfig struct {
	RefreshIntervalMinutes int    `toml:"refresh_interval_minutes"`
	APIBaseURL             string `toml:"api_base_url"`
	RequestTimeoutSeconds  int    `toml:"request_timeout_seconds"`
	MaxRetries             int    `toml:"max_retries"`
	FetchMETAR             bool   `toml:"fetch_metar"`
	FetchTAF               bool   `toml:"fetch_taf"`
	FetchNOTAMs            bool   `toml:"fetch_notams"`
	CacheExpiryMinutes     int    `toml:"cache_expiry_minutes"`
}

// FromConfigWeatherConfig converts a config.WeatherConfig to weather.WeatherConfig
func FromConfigWeatherConfig(cfg ConfigWeatherConfig) WeatherConfig {
	return WeatherConfig{
		RefreshIntervalMinutes: cfg.RefreshIntervalMinutes,
		APIBaseURL:             cfg.APIBaseURL,
		RequestTimeoutSeconds:  cfg.RequestTimeoutSeconds,
		MaxRetries:             cfg.MaxRetries,
		FetchMETAR:             cfg.FetchMETAR,
		FetchTAF:               cfg.FetchTAF,
		FetchNOTAMs:            cfg.FetchNOTAMs,
		CacheExpiryMinutes:     cfg.CacheExpiryMinutes,
	}
}
