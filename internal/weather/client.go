package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Client handles HTTP requests to weather APIs
type Client struct {
	config     WeatherConfig
	httpClient *http.Client
	logger     *logger.Logger
}

// NewClient creates a new weather API client
func NewClient(config WeatherConfig, logger *logger.Logger) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.RequestTimeoutSeconds) * time.Second,
		},
		logger: logger.Named("weather-client"),
	}
}

// FetchMETAR fetches METAR data for the specified airport
func (c *Client) FetchMETAR(airportCode string) (interface{}, error) {
	url := fmt.Sprintf("%s/metar/%s", c.config.APIBaseURL, airportCode)
	return c.fetchWithRetry(url, WeatherTypeMETAR, airportCode)
}

// FetchTAF fetches TAF data for the specified airport
func (c *Client) FetchTAF(airportCode string) (interface{}, error) {
	url := fmt.Sprintf("%s/taf/%s", c.config.APIBaseURL, airportCode)
	return c.fetchWithRetry(url, WeatherTypeTAF, airportCode)
}

// FetchNOTAMs fetches NOTAM data for the specified airport
func (c *Client) FetchNOTAMs(airportCode string) (interface{}, error) {
	url := fmt.Sprintf("%s/notams/%s", c.config.APIBaseURL, airportCode)
	return c.fetchWithRetry(url, WeatherTypeNOTAMs, airportCode)
}

// fetchWithRetry performs HTTP request with retry logic and exponential backoff
func (c *Client) fetchWithRetry(url string, weatherType WeatherType, airportCode string) (interface{}, error) {
	var lastErr error
	var data interface{}

	// Try to fetch with retries
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff between retries
			backoffDuration := time.Duration(500*(1<<uint(attempt-1))) * time.Millisecond
			c.logger.Info("Retrying weather data fetch",
				logger.String("type", string(weatherType)),
				logger.String("airport", airportCode),
				logger.Int("attempt", attempt),
				logger.String("backoff", backoffDuration.String()))
			time.Sleep(backoffDuration)
		}

		// Make the request
		resp, err := c.httpClient.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("error making request to weather API: %w", err)
			c.logger.Warn("Weather API request failed, may retry",
				logger.String("type", string(weatherType)),
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", c.config.MaxRetries+1))
			continue
		}

		// Ensure response body is closed
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			c.logger.Warn("Weather API returned non-OK status, may retry",
				logger.String("type", string(weatherType)),
				logger.String("airport", airportCode),
				logger.Int("status_code", resp.StatusCode),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", c.config.MaxRetries+1))
			continue
		}

		// Read and parse the response
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			lastErr = fmt.Errorf("error decoding weather data: %w", err)
			c.logger.Warn("Failed to decode weather data, may retry",
				logger.String("type", string(weatherType)),
				logger.String("airport", airportCode),
				logger.Error(err),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", c.config.MaxRetries+1))
			continue
		}

		// Success - return the data
		if attempt > 0 {
			c.logger.Info("Successfully fetched weather data after retries",
				logger.String("type", string(weatherType)),
				logger.String("airport", airportCode),
				logger.Int("attempts_needed", attempt+1))
		}
		return data, nil
	}

	// If we get here, all attempts failed
	c.logger.Error("All attempts to fetch weather data failed",
		logger.String("type", string(weatherType)),
		logger.String("airport", airportCode),
		logger.Error(lastErr),
		logger.Int("max_attempts", c.config.MaxRetries+1))
	return nil, lastErr
}

// FetchAll fetches all enabled weather data types concurrently
func (c *Client) FetchAll(airportCode string) []FetchResult {
	results := make(chan FetchResult, 3)
	var fetchCount int

	// Start concurrent fetches for enabled weather types
	if c.config.FetchMETAR {
		fetchCount++
		go func() {
			data, err := c.FetchMETAR(airportCode)
			results <- FetchResult{Type: WeatherTypeMETAR, Data: data, Err: err}
		}()
	}

	if c.config.FetchTAF {
		fetchCount++
		go func() {
			data, err := c.FetchTAF(airportCode)
			results <- FetchResult{Type: WeatherTypeTAF, Data: data, Err: err}
		}()
	}

	if c.config.FetchNOTAMs {
		fetchCount++
		go func() {
			data, err := c.FetchNOTAMs(airportCode)
			results <- FetchResult{Type: WeatherTypeNOTAMs, Data: data, Err: err}
		}()
	}

	// Collect results
	var fetchResults []FetchResult
	for i := 0; i < fetchCount; i++ {
		result := <-results
		fetchResults = append(fetchResults, result)
	}

	return fetchResults
}
