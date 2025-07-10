package adsb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Import the external.go file which contains the ExternalAPIResponse struct

// Client is responsible for fetching ADS-B data from the source
type Client struct {
	httpClient        *http.Client
	sourceType        string
	localSourceURL    string
	externalSourceURL string
	apiHost           string
	apiKey            string
	stationLat        float64
	stationLon        float64
	searchRadiusNM    float64
	logger            *logger.Logger
}

// NewClient creates a new ADS-B client
func NewClient(
	sourceType string,
	localSourceURL string,
	externalSourceURL string,
	apiHost string,
	apiKey string,
	stationLat float64,
	stationLon float64,
	searchRadiusNM float64,
	timeout time.Duration,
	logger *logger.Logger,
) *Client {
	return &Client{
		sourceType:        sourceType,
		localSourceURL:    localSourceURL,
		externalSourceURL: externalSourceURL,
		apiHost:           apiHost,
		apiKey:            apiKey,
		stationLat:        stationLat,
		stationLon:        stationLon,
		searchRadiusNM:    searchRadiusNM,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger.Named("adsb-cli"),
	}
}

// FetchData fetches ADS-B data from the configured source
func (c *Client) FetchData(ctx context.Context) (*RawAircraftData, error) {
	if c.sourceType == "local" {
		return c.fetchLocalData(ctx)
	} else if c.sourceType == "external" {
		return c.fetchExternalData(ctx)
	}
	return nil, fmt.Errorf("unknown source type: %s", c.sourceType)
}

// fetchLocalData fetches data from the local source
func (c *Client) fetchLocalData(ctx context.Context) (*RawAircraftData, error) {
	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.localSourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")

	// Execute the request
	c.logger.Debug("Fetching local ADS-B data",
		logger.String("url", c.localSourceURL),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON
	var data RawAircraftData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Mark all aircraft as coming from local source
	for i := range data.Aircraft {
		data.Aircraft[i].SourceType = "local"
	}

	c.logger.Debug("Successfully fetched local ADS-B data",
		logger.Int("aircraft_count", len(data.Aircraft)),
		logger.Int("message_count", data.Messages),
	)

	return &data, nil
}

// fetchExternalData fetches data from the external API
func (c *Client) fetchExternalData(ctx context.Context) (*RawAircraftData, error) {
	// Format URL with station coordinates and search radius
	url := fmt.Sprintf(c.externalSourceURL, c.stationLat, c.stationLon, c.searchRadiusNM)

	// Create request with authentication headers
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.logger.Error("Failed to create request", logger.Error(err), logger.String("url", url))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add required headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-rapidapi-host", c.apiHost)
	req.Header.Set("x-rapidapi-key", c.apiKey)

	// Execute the request
	c.logger.Debug("Fetching external ADS-B data",
		logger.String("url", url),
		logger.String("host", c.apiHost),
		logger.String("key_prefix", c.apiKey[:5]+"..."), // Log only prefix of API key for security
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to execute request", logger.Error(err), logger.String("url", url))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Unexpected status code",
			logger.Int("status_code", resp.StatusCode),
			logger.String("url", url))
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Failed to read response body", logger.Error(err))
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log a sample of the response for debugging
	bodyPreview := string(body)
	if len(bodyPreview) > 200 {
		bodyPreview = bodyPreview[:200] + "..."
	}
	c.logger.Debug("Response body preview", logger.String("body", bodyPreview))

	// First try to parse as external API format
	var externalData ExternalAPIResponse
	if err := json.Unmarshal(body, &externalData); err != nil {
		c.logger.Error("Failed to parse as external API format", logger.Error(err))

		// If that fails, try the standard format
		var data RawAircraftData
		if err2 := json.Unmarshal(body, &data); err2 != nil {
			c.logger.Error("Failed to parse as standard format", logger.Error(err2))
			return nil, fmt.Errorf("failed to parse JSON: %w (external format) and %w (standard format)", err, err2)
		}

		c.logger.Debug("Parsed as standard format",
			logger.Int("aircraft_count", len(data.Aircraft)))
		return &data, nil
	}

	c.logger.Debug("Parsed as external API format",
		logger.Int("aircraft_count", len(externalData.AC)))

	// Convert external API format to our standard format
	// Convert each ExternalADSBTarget to ADSBTarget
	aircraft := make([]ADSBTarget, 0, len(externalData.AC))
	for i, extTarget := range externalData.AC {
		// Log a sample of the conversion for debugging
		if i == 0 {
			c.logger.Debug("Sample aircraft conversion",
				logger.String("hex", extTarget.Hex),
				logger.String("flight", extTarget.Flight),
				logger.String("alt_baro_type", fmt.Sprintf("%T", extTarget.AltBaro.value)),
				logger.Float64("alt_baro_converted", extTarget.AltBaro.Float64()),
				logger.String("lat_type", fmt.Sprintf("%T", extTarget.Lat.value)),
				logger.Float64("lat_converted", extTarget.Lat.Float64()),
			)
		}
		aircraft = append(aircraft, extTarget.Convert())
	}

	data := &RawAircraftData{
		Now:      float64(time.Now().Unix()), // Use current time if not provided, convert to float64
		Messages: externalData.Messages,
		Aircraft: aircraft,
	}

	// Ensure Aircraft array is not nil
	if data.Aircraft == nil {
		data.Aircraft = []ADSBTarget{} // Initialize with empty array if nil
		c.logger.Warn("External API returned nil aircraft array, initializing empty array")
	}

	c.logger.Debug("Successfully fetched external ADS-B data",
		logger.Int("aircraft_count", len(data.Aircraft)),
		logger.String("source", "external API"),
	)

	return data, nil
}

// UpdateStationCoords updates the station coordinates used for external API calls
func (c *Client) UpdateStationCoords(lat, lon float64) {
	c.stationLat = lat
	c.stationLon = lon

	c.logger.Debug("Station coordinates updated",
		logger.Float64("latitude", lat),
		logger.Float64("longitude", lon))
}
