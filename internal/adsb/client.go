package adsb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/pkg/logger"
)

var defaultReadsbDirs = []string{
	"/run/readsb",
	"/var/run/readsb",
	"/run/dump1090-fa",
	"/run/dump1090-mutability",
}

type openSkyOAuth2Credentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type openSkyOAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type openSkyStatesResponse struct {
	Time   int64           `json:"time"`
	States [][]interface{} `json:"states"`
}

// Client is responsible for fetching ADS-B data from the configured source.
type Client struct {
	httpClient *http.Client
	logger     *logger.Logger

	sourceType string

	externalSourceURL string
	apiHost           string
	apiKey            string
	openSkyBaseURL    string
	openSkyTokenURL   string
	openSkyAuthMode   string
	openSkyCredsPath  string
	stationLat        float64
	stationLon        float64
	searchRadiusNM    float64

	tar1090BaseURL string
	readsbAPIURL   string
	readsbDataDir  string

	mu                 sync.RWMutex
	readsbResolvedDir  string
	sourceStatus       SourceStatus
	openSkyAccessToken string
	openSkyTokenExpiry time.Time
}

// NewClient creates a new ADS-B client.
func NewClient(
	adsbCfg config.ADSBConfig,
	stationLat float64,
	stationLon float64,
	timeout time.Duration,
	logger *logger.Logger,
) *Client {
	client := &Client{
		sourceType:        adsbCfg.SourceType,
		externalSourceURL: adsbCfg.ExternalSourceURL,
		apiHost:           adsbCfg.APIHost,
		apiKey:            adsbCfg.APIKey,
		openSkyBaseURL:    adsbCfg.OpenSkyBaseURL,
		openSkyTokenURL:   adsbCfg.OpenSkyTokenURL,
		openSkyAuthMode:   adsbCfg.OpenSkyAuthMode,
		openSkyCredsPath:  adsbCfg.OpenSkyOAuth2CredentialsPath,
		stationLat:        stationLat,
		stationLon:        stationLon,
		searchRadiusNM:    float64(adsbCfg.SearchRadiusNM),
		tar1090BaseURL:    adsbCfg.Tar1090BaseURL,
		readsbAPIURL:      adsbCfg.ReadsbAPIURL,
		readsbDataDir:     adsbCfg.ReadsbDataDir,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger.Named("adsb-cli"),
	}

	now := time.Now().UTC()
	client.sourceStatus = SourceStatus{
		SourceType: adsbCfg.SourceType,
		Mode:       adsbCfg.SourceType,
		Status:     "initializing",
		Aircraft: SourceChannelStatus{
			Available: false,
			Data:      nil,
		},
		Receiver: SourceChannelStatus{
			Available: false,
			Data:      nil,
		},
		Stats: SourceChannelStatus{
			Available: false,
			Data:      nil,
		},
		UpdatedAt: &now,
	}

	return client
}

// ValidateSource performs a startup probe and returns error if source is unreachable or invalid.
func (c *Client) ValidateSource(ctx context.Context) error {
	_, err := c.fetchBySource(ctx)
	if err != nil {
		return err
	}
	return nil
}

// FetchData fetches ADS-B data from the configured source.
func (c *Client) FetchData(ctx context.Context) (*RawAircraftData, error) {
	data, err := c.fetchBySource(ctx)
	if err != nil {
		c.setAircraftError(err)
		return nil, err
	}

	return data, nil
}

// GetSourceStatus returns the latest source status snapshot.
func (c *Client) GetSourceStatus() SourceStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := c.sourceStatus
	status.Receiver.Data = cloneAny(status.Receiver.Data)
	status.Stats.Data = cloneAny(status.Stats.Data)
	return status
}

func (c *Client) fetchBySource(ctx context.Context) (*RawAircraftData, error) {
	switch c.sourceType {
	case SourceTypeExternalAPI:
		return c.fetchExternalData(ctx)
	case SourceTypeExternalOpenSky:
		return c.fetchOpenSkyData(ctx)
	case SourceTypeTar1090:
		return c.fetchTar1090Data(ctx)
	case SourceTypeReadsbAPI:
		return c.fetchReadsbAPIData(ctx)
	case SourceTypeReadsbFile:
		return c.fetchReadsbFileData()
	default:
		return nil, fmt.Errorf("unknown source type: %s", c.sourceType)
	}
}

func (c *Client) fetchTar1090Data(ctx context.Context) (*RawAircraftData, error) {
	base := normalizeBaseURL(c.tar1090BaseURL)
	aircraftURL := base + "aircraft.json"
	receiverURL := base + "receiver.json"
	statsURL := base + "stats.json"

	data, err := c.fetchStandardAircraftURL(ctx, aircraftURL, SourceTypeTar1090)
	if err != nil {
		return nil, err
	}

	receiverData, err := c.fetchJSONObjectURL(ctx, receiverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load tar1090 receiver.json: %w", err)
	}

	statsData, err := c.fetchJSONObjectURL(ctx, statsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load tar1090 stats.json: %w", err)
	}

	c.setSuccessStatus(data, receiverData, statsData)
	return data, nil
}

func (c *Client) fetchReadsbAPIData(ctx context.Context) (*RawAircraftData, error) {
	data, err := c.fetchStandardAircraftURL(ctx, c.readsbAPIURL, SourceTypeReadsbAPI)
	if err != nil {
		return nil, err
	}

	c.setSuccessStatus(data, nil, nil)
	return data, nil
}

func (c *Client) fetchReadsbFileData() (*RawAircraftData, error) {
	dir, err := c.resolveReadsbDir()
	if err != nil {
		return nil, err
	}

	aircraftPath := filepath.Join(dir, "aircraft.json")
	receiverPath := filepath.Join(dir, "receiver.json")
	statsPath := filepath.Join(dir, "stats.json")

	aircraftBody, err := os.ReadFile(aircraftPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read readsb aircraft.json from %s: %w", aircraftPath, err)
	}

	var data RawAircraftData
	if err := json.Unmarshal(aircraftBody, &data); err != nil {
		return nil, fmt.Errorf("failed to parse readsb aircraft.json from %s: %w", aircraftPath, err)
	}
	for i := range data.Aircraft {
		data.Aircraft[i].SourceType = SourceTypeReadsbFile
	}

	receiverBody, err := os.ReadFile(receiverPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read readsb receiver.json from %s: %w", receiverPath, err)
	}
	receiverData, err := parseJSONObject(receiverBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse readsb receiver.json from %s: %w", receiverPath, err)
	}

	statsBody, err := os.ReadFile(statsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read readsb stats.json from %s: %w", statsPath, err)
	}
	statsData, err := parseJSONObject(statsBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse readsb stats.json from %s: %w", statsPath, err)
	}

	c.setSuccessStatus(&data, receiverData, statsData)
	return &data, nil
}

func (c *Client) fetchStandardAircraftURL(ctx context.Context, sourceURL string, sourceType string) (*RawAircraftData, error) {
	body, err := c.fetchURL(ctx, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch aircraft JSON from %s: %w", sourceURL, err)
	}

	var data RawAircraftData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse aircraft JSON from %s: %w", sourceURL, err)
	}

	for i := range data.Aircraft {
		data.Aircraft[i].SourceType = sourceType
	}

	return &data, nil
}

func (c *Client) fetchExternalData(ctx context.Context) (*RawAircraftData, error) {
	requestURL := fmt.Sprintf(c.externalSourceURL, c.stationLat, c.stationLon, c.searchRadiusNM)
	headers := map[string]string{
		"Accept":          "application/json",
		"x-rapidapi-host": c.apiHost,
		"x-rapidapi-key":  c.apiKey,
	}

	body, err := c.fetchURL(ctx, requestURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch external API data from %s: %w", requestURL, err)
	}

	var externalData ExternalAPIResponse
	if err := json.Unmarshal(body, &externalData); err != nil {
		var data RawAircraftData
		if err2 := json.Unmarshal(body, &data); err2 != nil {
			return nil, fmt.Errorf("failed to parse external API JSON from %s: %w", requestURL, err)
		}
		for i := range data.Aircraft {
			data.Aircraft[i].SourceType = SourceTypeExternalAPI
		}
		c.setSuccessStatus(&data, nil, nil)
		return &data, nil
	}

	aircraft := make([]ADSBTarget, 0, len(externalData.AC))
	for _, extTarget := range externalData.AC {
		aircraft = append(aircraft, extTarget.Convert())
	}

	data := &RawAircraftData{
		Now:      float64(time.Now().Unix()),
		Messages: externalData.Messages,
		Aircraft: aircraft,
	}
	if data.Aircraft == nil {
		data.Aircraft = []ADSBTarget{}
	}

	c.setSuccessStatus(data, nil, nil)
	return data, nil
}

func (c *Client) fetchOpenSkyData(ctx context.Context) (*RawAircraftData, error) {
	requestURL, err := c.buildOpenSkyStatesURL()
	if err != nil {
		return nil, err
	}

	headers, err := c.getOpenSkyAuthHeaders(ctx)
	if err != nil {
		return nil, err
	}
	body, err := c.fetchURL(ctx, requestURL, headers)
	if err != nil && strings.EqualFold(strings.TrimSpace(c.openSkyAuthMode), "oauth2") && strings.Contains(err.Error(), "unexpected status code: 401") {
		c.mu.Lock()
		c.openSkyAccessToken = ""
		c.openSkyTokenExpiry = time.Time{}
		c.mu.Unlock()

		headers, headerErr := c.getOpenSkyAuthHeaders(ctx)
		if headerErr == nil {
			body, err = c.fetchURL(ctx, requestURL, headers)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenSky states from %s: %w", requestURL, err)
	}

	var payload openSkyStatesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse OpenSky response from %s: %w", requestURL, err)
	}
	if payload.States == nil {
		return nil, fmt.Errorf("invalid OpenSky response from %s: missing states", requestURL)
	}

	aircraft := make([]ADSBTarget, 0, len(payload.States))
	for _, state := range payload.States {
		target, ok := mapOpenSkyStateToTarget(state)
		if !ok {
			continue
		}
		aircraft = append(aircraft, target)
	}

	now := float64(payload.Time)
	if payload.Time <= 0 {
		now = float64(time.Now().Unix())
	}

	data := &RawAircraftData{
		Now:      now,
		Messages: 0,
		Aircraft: aircraft,
	}

	c.setSuccessStatus(data, nil, nil)
	return data, nil
}

func (c *Client) buildOpenSkyStatesURL() (string, error) {
	base := normalizeBaseURL(c.openSkyBaseURL)
	if base == "" {
		return "", fmt.Errorf("opensky base URL is empty")
	}
	baseURL, err := url.Parse(base + "states/all")
	if err != nil {
		return "", fmt.Errorf("invalid opensky base URL: %w", err)
	}

	deltaLat := c.searchRadiusNM / 60.0
	cosLat := math.Cos(c.stationLat * math.Pi / 180.0)
	if math.Abs(cosLat) < 0.01 {
		cosLat = 0.01
	}
	deltaLon := c.searchRadiusNM / (60.0 * math.Abs(cosLat))

	lamin := math.Max(-90.0, c.stationLat-deltaLat)
	lamax := math.Min(90.0, c.stationLat+deltaLat)
	lomin := math.Max(-180.0, c.stationLon-deltaLon)
	lomax := math.Min(180.0, c.stationLon+deltaLon)

	q := baseURL.Query()
	q.Set("lamin", fmt.Sprintf("%.6f", lamin))
	q.Set("lomin", fmt.Sprintf("%.6f", lomin))
	q.Set("lamax", fmt.Sprintf("%.6f", lamax))
	q.Set("lomax", fmt.Sprintf("%.6f", lomax))
	baseURL.RawQuery = q.Encode()

	return baseURL.String(), nil
}

func (c *Client) getOpenSkyAuthHeaders(ctx context.Context) (map[string]string, error) {
	mode := strings.TrimSpace(strings.ToLower(c.openSkyAuthMode))
	switch mode {
	case "", "anonymous":
		return map[string]string{"Accept": "application/json"}, nil
	case "oauth2":
		token, err := c.getOpenSkyBearerToken(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"Accept":        "application/json",
			"Authorization": "Bearer " + token,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported opensky auth mode: %s", c.openSkyAuthMode)
	}
}

func (c *Client) getOpenSkyBearerToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	cachedToken := c.openSkyAccessToken
	cachedExpiry := c.openSkyTokenExpiry
	c.mu.RUnlock()

	if cachedToken != "" && time.Now().UTC().Before(cachedExpiry.Add(-1*time.Minute)) {
		return cachedToken, nil
	}

	credentialsBlob, err := os.ReadFile(c.openSkyCredsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read opensky oauth2 credentials file %s: %w", c.openSkyCredsPath, err)
	}

	var creds openSkyOAuth2Credentials
	if err := json.Unmarshal(credentialsBlob, &creds); err != nil {
		return "", fmt.Errorf("failed to parse opensky oauth2 credentials file %s: %w", c.openSkyCredsPath, err)
	}
	if strings.TrimSpace(creds.ClientID) == "" || strings.TrimSpace(creds.ClientSecret) == "" {
		return "", fmt.Errorf("opensky oauth2 credentials file %s must include clientId and clientSecret", c.openSkyCredsPath)
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", creds.ClientID)
	form.Set("client_secret", creds.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.openSkyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create opensky oauth2 token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("opensky oauth2 token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read opensky oauth2 token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("opensky oauth2 token request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp openSkyOAuth2TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse opensky oauth2 token response: %w", err)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("opensky oauth2 token response did not include access_token")
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1800
	}

	expiry := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	c.mu.Lock()
	c.openSkyAccessToken = tokenResp.AccessToken
	c.openSkyTokenExpiry = expiry
	c.mu.Unlock()

	return tokenResp.AccessToken, nil
}

func mapOpenSkyStateToTarget(state []interface{}) (ADSBTarget, bool) {
	hex := strings.ToLower(strings.TrimSpace(getStateString(state, 0)))
	if hex == "" {
		return ADSBTarget{}, false
	}

	target := ADSBTarget{
		Hex:        hex,
		Flight:     strings.TrimSpace(getStateString(state, 1)),
		Squawk:     strings.TrimSpace(getStateString(state, 14)),
		SourceType: SourceTypeExternalOpenSky,
		AltBaro:    NullFlexibleFloat64(),
		AltGeom:    NullFlexibleFloat64(),
	}

	if lon, ok := getStateFloat(state, 5); ok {
		target.Lon = NumberPtr(lon)
	}
	if lat, ok := getStateFloat(state, 6); ok {
		target.Lat = NumberPtr(lat)
	}
	if altM, ok := getStateFloat(state, 7); ok {
		target.AltBaro = FlexibleFloat64(metersToFeet(altM))
	}
	if onGround, ok := getStateBool(state, 8); ok {
		target.OnGroundReported = boolPtr(onGround)
	}
	if velMS, ok := getStateFloat(state, 9); ok {
		target.GS = NumberPtr(msToKnots(velMS))
	}
	if track, ok := getStateFloat(state, 10); ok {
		target.Track = NumberPtr(track)
	}
	if vrMS, ok := getStateFloat(state, 11); ok {
		target.BaroRate = NumberPtr(msToFeetPerMin(vrMS))
	}
	if geoAltM, ok := getStateFloat(state, 13); ok {
		target.AltGeom = FlexibleFloat64(metersToFeet(geoAltM))
	}
	if spi, ok := getStateBool(state, 15); ok {
		if spi {
			target.SPI = IntPtr(1)
		} else {
			target.SPI = IntPtr(0)
		}
	}
	if category, ok := getStateFloat(state, 17); ok {
		target.Category = fmt.Sprintf("%d", int(category))
	}

	return target, true
}

func getStateString(state []interface{}, idx int) string {
	if idx < 0 || idx >= len(state) || state[idx] == nil {
		return ""
	}
	if value, ok := state[idx].(string); ok {
		return value
	}
	return fmt.Sprintf("%v", state[idx])
}

func getStateFloat(state []interface{}, idx int) (float64, bool) {
	if idx < 0 || idx >= len(state) || state[idx] == nil {
		return 0, false
	}
	switch value := state[idx].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func getStateBool(state []interface{}, idx int) (bool, bool) {
	if idx < 0 || idx >= len(state) || state[idx] == nil {
		return false, false
	}
	value, ok := state[idx].(bool)
	if !ok {
		return false, false
	}
	return value, true
}

func metersToFeet(meters float64) float64 {
	return meters * 3.28084
}

func msToKnots(metersPerSec float64) float64 {
	return metersPerSec * 1.943844492
}

func msToFeetPerMin(metersPerSec float64) float64 {
	return metersPerSec * 196.850394
}

func boolPtr(value bool) *bool {
	return &value
}

func (c *Client) fetchJSONObjectURL(ctx context.Context, sourceURL string) (map[string]interface{}, error) {
	body, err := c.fetchURL(ctx, sourceURL, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	return parseJSONObject(body)
}

func (c *Client) fetchURL(ctx context.Context, sourceURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *Client) resolveReadsbDir() (string, error) {
	c.mu.RLock()
	cached := c.readsbResolvedDir
	c.mu.RUnlock()
	if cached != "" {
		if hasReadsbFiles(cached) {
			return cached, nil
		}
	}

	candidates := make([]string, 0, len(defaultReadsbDirs)+1)
	if strings.TrimSpace(c.readsbDataDir) != "" {
		candidates = append(candidates, strings.TrimSpace(c.readsbDataDir))
	}
	candidates = append(candidates, defaultReadsbDirs...)

	for _, candidate := range candidates {
		if hasReadsbFiles(candidate) {
			c.mu.Lock()
			c.readsbResolvedDir = candidate
			c.mu.Unlock()
			return candidate, nil
		}
	}

	return "", fmt.Errorf("readsb-file mode could not auto-detect required files (aircraft.json, receiver.json, stats.json)")
}

func hasReadsbFiles(dir string) bool {
	required := []string{"aircraft.json", "receiver.json", "stats.json"}
	for _, fileName := range required {
		filePath := filepath.Join(dir, fileName)
		if info, err := os.Stat(filePath); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func parseJSONObject(body []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("JSON payload is not an object")
	}
	return data, nil
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		if strings.HasSuffix(trimmed, "/") {
			return trimmed
		}
		return trimmed + "/"
	}

	parsed.Path = path.Clean(parsed.Path)
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.String()
}

func cloneAny(value any) any {
	if value == nil {
		return nil
	}
	blob, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(blob, &cloned); err != nil {
		return value
	}
	return cloned
}

func (c *Client) setSuccessStatus(data *RawAircraftData, receiverData map[string]interface{}, statsData map[string]interface{}) {
	now := time.Now().UTC()
	status := SourceStatus{
		SourceType: c.sourceType,
		Mode:       c.sourceType,
		Status:     "ok",
		Aircraft: SourceChannelStatus{
			Available:     true,
			LastSuccessAt: &now,
			Data: map[string]interface{}{
				"messages":       data.Messages,
				"aircraft_count": len(data.Aircraft),
			},
		},
		Receiver: SourceChannelStatus{
			Available: false,
			Data:      nil,
		},
		Stats: SourceChannelStatus{
			Available: false,
			Data:      nil,
		},
		UpdatedAt: &now,
	}

	if c.sourceType == SourceTypeTar1090 || c.sourceType == SourceTypeReadsbFile {
		status.Receiver.Available = receiverData != nil
		status.Receiver.LastSuccessAt = &now
		status.Receiver.Data = cloneAny(receiverData)
		status.Stats.Available = statsData != nil
		status.Stats.LastSuccessAt = &now
		status.Stats.Data = cloneAny(statsData)
	}

	c.mu.Lock()
	c.sourceStatus = status
	c.mu.Unlock()
}

func (c *Client) setAircraftError(err error) {
	now := time.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()

	status := c.sourceStatus
	status.SourceType = c.sourceType
	status.Mode = c.sourceType
	status.Status = "error"
	status.Aircraft.Available = false
	status.Aircraft.LastError = err.Error()
	status.UpdatedAt = &now
	c.sourceStatus = status
}

// UpdateStationCoords updates the station coordinates used for external API calls.
func (c *Client) UpdateStationCoords(lat, lon float64) {
	c.stationLat = lat
	c.stationLon = lon

	c.logger.Debug("Station coordinates updated",
		logger.Float64("latitude", lat),
		logger.Float64("longitude", lon))
}
