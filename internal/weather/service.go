package weather

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Service manages weather data fetching and caching
type Service struct {
	config      WeatherConfig
	airportCode string
	client      *Client
	cache       *Cache
	logger      *logger.Logger

	// Service lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	mu      sync.RWMutex

	// Initial data readiness
	initialDataReady chan struct{}
	initialDataOnce  sync.Once
}

// NewService creates a new weather service
func NewService(configWeather ConfigWeatherConfig, airportCode string, logger *logger.Logger) *Service {
	// Convert config to internal WeatherConfig type
	weatherConfig := FromConfigWeatherConfig(configWeather)

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		config:           weatherConfig,
		airportCode:      airportCode,
		client:           NewClient(weatherConfig, logger),
		cache:            NewCache(weatherConfig, logger),
		logger:           logger.Named("weather-service"),
		ctx:              ctx,
		cancel:           cancel,
		initialDataReady: make(chan struct{}),
	}
}

// Start begins the weather service background operations
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil // Already started
	}

	s.logger.Info("Starting weather service",
		logger.String("airport", s.airportCode),
		logger.Int("refresh_interval_minutes", s.config.RefreshIntervalMinutes))

	// Perform initial fetch
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.performInitialFetch()
	}()

	// Start background refresh goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.backgroundRefresh()
	}()

	s.started = true
	return nil
}

// Stop gracefully shuts down the weather service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil // Already stopped
	}

	s.logger.Info("Stopping weather service")

	// Cancel context to signal goroutines to stop
	s.cancel()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.started = false
	s.logger.Info("Weather service stopped")
	return nil
}

// GetWeatherData returns the current cached weather data
// Waits for initial data to be available if service just started
func (s *Service) GetWeatherData() *WeatherData {
	// Wait for initial data to be ready (with timeout)
	select {
	case <-s.initialDataReady:
		// Initial data is ready, proceed normally
	case <-time.After(30 * time.Second):
		// Timeout waiting for initial data, log warning and return error data
		s.logger.Warn("Timeout waiting for initial weather data")
		return &WeatherData{
			LastUpdated: time.Now(),
			FetchErrors: []string{"Weather data is still being fetched, please try again in a moment"},
		}
	}

	data := s.cache.Get()
	if data == nil {
		// This shouldn't happen after initial data is ready, but handle gracefully
		s.logger.Warn("No weather data available after initial fetch completed")
		return &WeatherData{
			LastUpdated: time.Now(),
			FetchErrors: []string{"Weather data temporarily unavailable"},
		}
	}

	return data
}

// RefreshNow triggers an immediate refresh of weather data
func (s *Service) RefreshNow() {
	s.logger.Info("Manual weather refresh triggered")
	go s.fetchAndUpdateCache()
}

// GetCacheStats returns cache statistics
func (s *Service) GetCacheStats() map[string]interface{} {
	return s.cache.GetStats()
}

// IsStarted returns whether the service is currently running
func (s *Service) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// performInitialFetch performs the first weather data fetch on service start
func (s *Service) performInitialFetch() {
	s.logger.Info("Performing initial weather data fetch",
		logger.String("airport", s.airportCode))

	s.fetchAndUpdateCache()

	// Signal that initial data is ready
	s.initialDataOnce.Do(func() {
		close(s.initialDataReady)
		s.logger.Info("Initial weather data fetch completed")
	})
}

// backgroundRefresh runs the periodic weather data refresh
func (s *Service) backgroundRefresh() {
	refreshInterval := time.Duration(s.config.RefreshIntervalMinutes) * time.Minute
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	s.logger.Info("Background weather refresh started",
		logger.String("interval", refreshInterval.String()))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Background weather refresh stopped")
			return
		case <-ticker.C:
			s.logger.Debug("Periodic weather refresh triggered")
			s.fetchAndUpdateCache()
		}
	}
}

// fetchAndUpdateCache fetches weather data and updates the cache
func (s *Service) fetchAndUpdateCache() {
	startTime := time.Now()

	s.logger.Debug("Fetching weather data",
		logger.String("airport", s.airportCode))

	// Fetch all enabled weather data types
	results := s.client.FetchAll(s.airportCode)

	// Update cache with results
	s.cache.Update(results, s.airportCode)

	duration := time.Since(startTime)
	s.logger.Info("Weather data fetch completed",
		logger.String("airport", s.airportCode),
		logger.String("duration", duration.String()),
		logger.Int("total_requests", len(results)))
}

// ValidateConfig validates the weather service configuration
func ValidateConfig(config WeatherConfig) error {
	if config.RefreshIntervalMinutes <= 0 {
		return fmt.Errorf("refresh_interval_minutes must be greater than 0")
	}

	if config.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("request_timeout_seconds must be greater than 0")
	}

	if config.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be 0 or greater")
	}

	if config.CacheExpiryMinutes <= 0 {
		return fmt.Errorf("cache_expiry_minutes must be greater than 0")
	}

	if config.APIBaseURL == "" {
		return fmt.Errorf("api_base_url cannot be empty")
	}

	// At least one weather type must be enabled
	if !config.FetchMETAR && !config.FetchTAF && !config.FetchNOTAMs {
		return fmt.Errorf("at least one weather type must be enabled (fetch_metar, fetch_taf, or fetch_notams)")
	}

	return nil
}
