package transcription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// OpenAIClient handles communication with OpenAI's Realtime Transcription API
type OpenAIClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
	logger     *logger.Logger
}

// OpenAIWebSocketConn represents a WebSocket connection to OpenAI
type OpenAIWebSocketConn struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	closed    bool
	closeChan chan struct{}
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model string, timeoutSeconds int, logger *logger.Logger) *OpenAIClient {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second // Default to 2 minutes if not specified
	}

	if apiKey == "" {
		logger.Warn("OpenAI API key is empty - transcription and post-processing features will not work")
	}

	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		logger: logger.Named("openai"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// CreateSession creates a new transcription session
func (c *OpenAIClient) CreateSession(ctx context.Context, config Config) (string, string, error) {
	// Check if OpenAI API key is provided - fail fast if missing
	if c.apiKey == "" {
		return "", "", fmt.Errorf("OpenAI API key is required for transcription sessions")
	}

	c.logger.Info("Creating new OpenAI transcription session",
		logger.String("model", c.model),
		logger.String("language", config.Language),
		logger.String("noise_reduction", config.NoiseReduction))
	// Create request body using the exact same structure as the POC
	type InputAudioNoiseReduction struct {
		Type string `json:"type"`
	}

	type InputAudioTranscription struct {
		Model    string `json:"model"`
		Language string `json:"language,omitempty"`
		Prompt   string `json:"prompt,omitempty"`
	}

	type TurnDetection struct {
		Type              string   `json:"type,omitempty"`
		PrefixPaddingMs   *int     `json:"prefix_padding_ms,omitempty"`
		SilenceDurationMs *int     `json:"silence_duration_ms,omitempty"`
		Threshold         *float64 `json:"threshold,omitempty"`
	}

	type TranscriptionSessionRequest struct {
		InputAudioFormat         string                    `json:"input_audio_format"`
		InputAudioTranscription  *InputAudioTranscription  `json:"input_audio_transcription"`
		InputAudioNoiseReduction *InputAudioNoiseReduction `json:"input_audio_noise_reduction,omitempty"`
		TurnDetection            *TurnDetection            `json:"turn_detection,omitempty"`
	}

	// Create the request body
	reqBody := TranscriptionSessionRequest{
		InputAudioFormat: "pcm16",
		InputAudioTranscription: &InputAudioTranscription{
			Model:    c.model,
			Language: config.Language,
			Prompt:   config.Prompt,
		},
	}

	// Add noise reduction if specified
	if config.NoiseReduction != "" {
		reqBody.InputAudioNoiseReduction = &InputAudioNoiseReduction{
			Type: config.NoiseReduction,
		}
	}

	// Add turn detection if specified
	if config.TurnDetectionType != "" {
		prefixPaddingMs := config.PrefixPaddingMs
		silenceDurationMs := config.SilenceDurationMs
		threshold := config.VADThreshold

		reqBody.TurnDetection = &TurnDetection{
			Type: config.TurnDetectionType,
		}

		// Only add non-zero values
		if prefixPaddingMs > 0 {
			reqBody.TurnDetection.PrefixPaddingMs = &prefixPaddingMs
		}

		if silenceDurationMs > 0 {
			reqBody.TurnDetection.SilenceDurationMs = &silenceDurationMs
		}

		if threshold > 0 {
			reqBody.TurnDetection.Threshold = &threshold
		}
	}

	// Marshal request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/realtime/transcription_sessions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("openai-beta", "realtime=v1")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		SessionID    string `json:"session_id"`
		ClientSecret struct {
			Value     string `json:"value"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"client_secret"`
	}

	// Read the response body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the response body
	c.logger.Debug("OpenAI API response",
		logger.String("response", string(bodyBytes)))

	// Parse the response
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Log the parsed result
	c.logger.Debug("Parsed OpenAI API response",
		logger.String("session_id", result.SessionID),
		logger.String("client_secret_value_prefix", result.ClientSecret.Value[:10]+"..."),
		logger.Int64("client_secret_expires_at", result.ClientSecret.ExpiresAt))

	return result.SessionID, result.ClientSecret.Value, nil
}

// ConnectWebSocket establishes a WebSocket connection to the transcription API with reconnection logic
func (c *OpenAIClient) ConnectWebSocket(ctx context.Context, sessionID, clientSecret string) (*OpenAIWebSocketConn, error) {
	// Create WebSocket URL
	wsURL := fmt.Sprintf("wss://api.openai.com/v1/realtime?session_id=%s", url.QueryEscape(sessionID))
	c.logger.Debug("Connecting to OpenAI WebSocket", logger.String("url", wsURL))

	// Create WebSocket dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}

	// Set headers
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", clientSecret))
	headers.Set("openai-beta", "realtime=v1")

	// Connect to WebSocket with retry logic
	var conn *websocket.Conn
	var resp *http.Response
	var err error

	maxRetries := 3
	retryInterval := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		c.logger.Debug("Attempting to connect to OpenAI WebSocket",
			logger.Int("attempt", attempt+1),
			logger.Int("max_attempts", maxRetries))

		conn, resp, err = dialer.DialContext(ctx, wsURL, headers)
		if err == nil {
			c.logger.Debug("Successfully connected to OpenAI WebSocket",
				logger.String("status", resp.Status))
			break
		}

		c.logger.Error("Failed to connect to OpenAI WebSocket",
			logger.Int("attempt", attempt+1),
			logger.Error(err))

		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("failed to connect to WebSocket after %d attempts: %w", maxRetries, err)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
			// Continue with retry
		}
	}

	// Create WebSocket connection
	wsConn := &OpenAIWebSocketConn{
		conn:      conn,
		closeChan: make(chan struct{}),
	}

	return wsConn, nil
}

// Send sends a message to the WebSocket
func (ws *OpenAIWebSocketConn) Send(message string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return fmt.Errorf("WebSocket connection is closed")
	}

	return ws.conn.WriteMessage(websocket.TextMessage, []byte(message))
}

// Receive receives a message from the WebSocket
func (ws *OpenAIWebSocketConn) Receive() (string, error) {
	_, message, err := ws.conn.ReadMessage()
	if err != nil {
		return "", err
	}

	return string(message), nil
}

// Close closes the WebSocket connection
func (ws *OpenAIWebSocketConn) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return nil
	}

	ws.closed = true
	close(ws.closeChan)
	return ws.conn.Close()
}

// PostProcessTranscription sends a transcription to OpenAI for post-processing
func (c *OpenAIClient) PostProcessTranscription(ctx context.Context, content string, systemPrompt string, model string) (*PostProcessingResult, error) {
	// Check if OpenAI API key is provided - fail fast if missing
	if c.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for post-processing")
	}

	c.logger.Debug("Post-processing transcription",
		logger.String("content", content),
		logger.String("model", model))

	// Create request body
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type Request struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
		Temperature float64   `json:"temperature"`
	}

	// Create messages
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: content,
		},
	}

	// Create request
	request := Request{
		Model:       model,
		Messages:    messages,
		MaxTokens:   2048, // Adjust as needed
		Temperature: 0.0,  // Set temperature to 0 for deterministic output
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we have choices
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Parse the content as JSON
	var processingResult PostProcessingResult
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &processingResult); err != nil {
		return nil, fmt.Errorf("failed to parse processing result: %w", err)
	}

	return &processingResult, nil
}

// PostProcessBatch sends a batch of transcriptions to OpenAI for post-processing
func (c *OpenAIClient) PostProcessBatch(ctx context.Context, systemPrompt string, userInput string, model string) ([]TranscriptionBatch, error) {
	// Check if OpenAI API key is provided - fail fast if missing
	if c.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for post-processing")
	}

	c.logger.Debug("Post-processing batch of transcriptions",
		logger.String("model", model),
		logger.Int("system_prompt_length", len(systemPrompt)),
		logger.Int("user_input_length", len(userInput)))

	// Create request body
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type Request struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
		Temperature float64   `json:"temperature"`
	}

	// Create messages
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userInput,
		},
	}

	// Create request
	request := Request{
		Model:       model,
		Messages:    messages,
		MaxTokens:   4096, // Increased for batch processing
		Temperature: 0.0,  // Set temperature to 0 for deterministic output
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the full request at info level for auditing
	prettyRequest, _ := json.MarshalIndent(request, "", "  ")
	c.logger.Debug("OpenAI post-processing request",
		logger.String("model", model),
		logger.Int("system_prompt_length", len(systemPrompt)),
		logger.Int("user_input_length", len(userInput)),
		logger.String("system_prompt", systemPrompt),
		logger.String("user_input", userInput),
		logger.String("full_request", string(prettyRequest)))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the response at info level for auditing
	c.logger.Info("OpenAI post-processing response",
		logger.Int("status_code", resp.StatusCode),
		logger.Int("response_length", len(bodyBytes)),
		logger.String("response", string(bodyBytes)))

	// Parse response
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we have choices
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Extract the content from the response
	content := result.Choices[0].Message.Content

	// Find the JSON array in the content (in case there's additional text)
	startIdx := strings.Index(content, "[")
	endIdx := strings.LastIndex(content, "]")

	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		// Log error with full response content for debugging
		c.logger.Error("Failed to find JSON array in OpenAI response - this indicates the LLM is not following the expected format",
			logger.String("full_response", content),
			logger.String("model", model))
		return nil, fmt.Errorf("OpenAI response does not contain valid JSON array: %s", content)
	}

	jsonContent := content[startIdx : endIdx+1]

	// Parse the content as JSON array of TranscriptionBatch
	var results []TranscriptionBatch
	if err := json.Unmarshal([]byte(jsonContent), &results); err != nil {
		// Log error with extracted JSON content for debugging
		c.logger.Error("Failed to unmarshal OpenAI response as JSON array",
			logger.String("error", err.Error()),
			logger.String("extracted_json", jsonContent),
			logger.String("full_response", content),
			logger.String("model", model))
		return nil, fmt.Errorf("failed to parse OpenAI response as JSON: %w", err)
	}

	// Log successful parsing with result count
	c.logger.Debug("Successfully parsed OpenAI response",
		logger.Int("result_count", len(results)),
		logger.String("model", model))

	return results, nil
}
