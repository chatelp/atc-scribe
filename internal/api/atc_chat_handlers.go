package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/yegors/co-atc/internal/atcchat"
	"github.com/yegors/co-atc/pkg/logger"
)

// SafeWebSocketConn wraps a WebSocket connection with a mutex for thread-safe writes
type SafeWebSocketConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// WriteMessage safely writes a message to the WebSocket connection
func (s *SafeWebSocketConn) WriteMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(messageType, data)
}

// ReadMessage reads a message from the WebSocket connection (no mutex needed for reads)
func (s *SafeWebSocketConn) ReadMessage() (int, []byte, error) {
	return s.conn.ReadMessage()
}

// Close closes the WebSocket connection
func (s *SafeWebSocketConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}

// NewSafeWebSocketConn creates a new safe WebSocket connection wrapper
func NewSafeWebSocketConn(conn *websocket.Conn) *SafeWebSocketConn {
	return &SafeWebSocketConn{
		conn: conn,
	}
}

// ATCChatHandlers contains handlers for ATC chat functionality
type ATCChatHandlers struct {
	service  *atcchat.Service
	logger   *logger.Logger
	upgrader websocket.Upgrader
}

// NewATCChatHandlers creates new ATC chat handlers
func NewATCChatHandlers(service *atcchat.Service, logger *logger.Logger) *ATCChatHandlers {
	return &ATCChatHandlers{
		service: service,
		logger:  logger.Named("atc-chat-handlers"),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for now - in production, restrict this
				return true
			},
		},
	}
}

// CreateSession creates a new ATC chat session
func (h *ATCChatHandlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Creating new ATC chat session")

	session, err := h.service.CreateSession(r.Context())
	if err != nil {
		h.logger.Error("Failed to create session", logger.Error(err))
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		h.logger.Error("Failed to encode session response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully created session",
		logger.String("session_id", session.ID))
}

// GetSession retrieves an existing ATC chat session
func (h *ATCChatHandlers) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	session, err := h.service.GetSession(sessionID)
	if err != nil {
		h.logger.Error("Failed to get session",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		h.logger.Error("Failed to encode session response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// EndSession terminates an ATC chat session
func (h *ATCChatHandlers) EndSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	if err := h.service.EndSession(r.Context(), sessionID); err != nil {
		h.logger.Error("Failed to end session",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, "Failed to end session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.logger.Info("Successfully ended session",
		logger.String("session_id", sessionID))
}

// UpdateSessionContext updates the session context with fresh airspace data
// This is called when the user starts speaking (push-to-talk)
func (h *ATCChatHandlers) UpdateSessionContext(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	h.logger.Debug("Received request to update session context",
		logger.String("session_id", sessionID))

	// Update session context with fresh airspace data
	if err := h.service.UpdateSessionContextOnDemand(sessionID); err != nil {
		h.logger.Error("Failed to update session context",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to update session context: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Session context updated successfully",
		logger.String("session_id", sessionID))

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Session context updated with fresh airspace data",
	})
}

// WebSocketMessage represents a message sent over the WebSocket connection
type WebSocketMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// AudioData represents audio data in a WebSocket message
type AudioData struct {
	Format    string `json:"format"`
	Data      string `json:"data"` // Base64 encoded audio data
	Timestamp int64  `json:"timestamp"`
}

// WebSocketHandler handles WebSocket connections for realtime audio
func (h *ATCChatHandlers) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Verify session exists and is valid
	session, err := h.service.GetSession(sessionID)
	if err != nil {
		h.logger.Error("Session not found for WebSocket connection",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade to WebSocket",
			logger.String("session_id", sessionID),
			logger.Error(err))
		return
	}
	defer conn.Close()

	h.logger.Info("WebSocket connection established",
		logger.String("session_id", sessionID))

	// Create context for this connection
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start the realtime audio bridge
	if err := h.bridgeRealtimeAudio(ctx, conn, session); err != nil {
		// Only log unexpected WebSocket errors, not normal closures
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			h.logger.Error("Realtime audio bridge failed",
				logger.String("session_id", sessionID),
				logger.Error(err))
		} else {
			h.logger.Debug("Realtime audio bridge ended normally",
				logger.String("session_id", sessionID),
				logger.Error(err))
		}
	}

	h.logger.Info("WebSocket connection closed",
		logger.String("session_id", sessionID))
}

// bridgeRealtimeAudio handles the bidirectional audio streaming between client and OpenAI
func (h *ATCChatHandlers) bridgeRealtimeAudio(ctx context.Context, clientConn *websocket.Conn, session *atcchat.ChatSession) error {
	h.logger.Info("Starting realtime audio bridge",
		logger.String("session_id", session.ID))

	// Register this WebSocket connection for context updates
	updateChan := h.service.RegisterWebSocketConnection(session.ID)
	defer h.service.UnregisterWebSocketConnection(session.ID)

	// Send a ready message to client to confirm connection is established
	readyMsg := map[string]interface{}{
		"type":       "connection_ready",
		"session_id": session.ID,
	}
	if err := clientConn.WriteJSON(readyMsg); err != nil {
		return fmt.Errorf("failed to send ready message to client: %w", err)
	}

	// Use static prompt for initial connection - templated data will be sent via session.update
	systemPrompt := "You are an experienced Air Traffic Controller assistant. Real-time airspace data will be provided via system updates."

	// Get configured model from service
	model := h.service.GetRealtimeModel()
	if model == "" {
		model = "gpt-4o-realtime-preview-2024-12-17" // fallback
	}

	h.logger.Debug("Using model and prompt",
		logger.String("session_id", session.ID),
		logger.String("model", model),
		logger.String("prompt_length", fmt.Sprintf("%d chars", len(systemPrompt))))

	// Connect to OpenAI realtime WebSocket (this can take time)
	h.logger.Info("Connecting to OpenAI realtime API",
		logger.String("session_id", session.ID))

	rawOpenaiConn, err := h.connectToOpenAI(ctx, session, systemPrompt, model)
	if err != nil {
		// Send error message to client
		errorMsg := map[string]interface{}{
			"type":  "connection_error",
			"error": "Failed to connect to OpenAI API",
		}
		clientConn.WriteJSON(errorMsg)
		return fmt.Errorf("failed to connect to OpenAI: %w", err)
	}
	defer rawOpenaiConn.Close()

	// Send OpenAI connection ready message to client
	openaiReadyMsg := map[string]interface{}{
		"type":       "openai_ready",
		"session_id": session.ID,
	}
	if err := clientConn.WriteJSON(openaiReadyMsg); err != nil {
		return fmt.Errorf("failed to send OpenAI ready message to client: %w", err)
	}

	// Wrap with safe WebSocket connection for thread-safe writes
	openaiConn := NewSafeWebSocketConn(rawOpenaiConn)

	// Start bidirectional message forwarding
	errChan := make(chan error, 3)

	// Forward messages from client to OpenAI
	go func() {
		errChan <- h.forwardClientToOpenAI(ctx, clientConn, openaiConn, session)
	}()

	// Forward messages from OpenAI to client
	go func() {
		errChan <- h.forwardOpenAIToClient(ctx, openaiConn, clientConn, session, systemPrompt)
	}()

	// Handle context updates from service
	go func() {
		errChan <- h.handleContextUpdates(ctx, openaiConn, session, updateChan)
	}()

	// Wait for any goroutine to finish or error
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetSessionStatus returns the current status of a session
func (h *ATCChatHandlers) GetSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	status, err := h.service.GetSessionStatus(sessionID)
	if err != nil {
		h.logger.Error("Failed to get session status",
			logger.String("session_id", sessionID),
			logger.Error(err))
		http.Error(w, "Failed to get session status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.Error("Failed to encode status response", logger.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// connectToOpenAI establishes a WebSocket connection to OpenAI's realtime API
func (h *ATCChatHandlers) connectToOpenAI(ctx context.Context, session *atcchat.ChatSession, systemPrompt, model string) (*websocket.Conn, error) {
	// Get OpenAI API key from config
	config := h.service.GetConfig()
	if config.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	var url string
	var headers http.Header

	// Check if we have an OpenAI session created via REST API
	if session.OpenAISessionID != "" && session.ClientSecret != "" {
		// Use session-based WebSocket connection with model parameter
		url = fmt.Sprintf("wss://api.openai.com/v1/realtime?model=%s", model)
		headers = http.Header{}
		headers.Set("Authorization", "Bearer "+session.ClientSecret)
		headers.Set("OpenAI-Beta", "realtime=v1")

		h.logger.Info("Connecting to OpenAI realtime API using session credentials",
			logger.String("session_id", session.ID),
			logger.String("openai_session_id", session.OpenAISessionID),
			logger.String("url", url),
			logger.String("auth_type", "session_client_secret"))
	} else {
		// Fallback to direct WebSocket connection
		url = "wss://api.openai.com/v1/realtime?model=" + model
		headers = http.Header{}
		headers.Set("Authorization", "Bearer "+config.OpenAIAPIKey)
		headers.Set("OpenAI-Beta", "realtime=v1")

		h.logger.Warn("No OpenAI session found, using direct WebSocket connection",
			logger.String("session_id", session.ID),
			logger.String("model", model),
			logger.String("url", url),
			logger.String("auth_type", "api_key"))
	}

	h.logger.Debug("Connecting to OpenAI realtime API",
		logger.String("session_id", session.ID),
		logger.String("url", url))

	// Connect to OpenAI
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		// Log detailed error information
		if resp != nil {
			h.logger.Error("WebSocket handshake failed with HTTP response",
				logger.String("session_id", session.ID),
				logger.String("url", url),
				logger.Int("status_code", resp.StatusCode),
				logger.String("status", resp.Status))

			// Try to read response body for more details
			if resp.Body != nil {
				bodyBytes, readErr := io.ReadAll(resp.Body)
				if readErr == nil {
					h.logger.Error("WebSocket handshake error response body",
						logger.String("session_id", session.ID),
						logger.String("response_body", string(bodyBytes)))
				}
			}
		}
		return nil, fmt.Errorf("failed to connect to OpenAI: %w", err)
	}

	h.logger.Info("Connected to OpenAI realtime API",
		logger.String("session_id", session.ID),
		logger.String("openai_session_id", session.OpenAISessionID))

	return conn, nil
}

// sendSessionUpdate sends the session.update event with system prompt
func (h *ATCChatHandlers) sendSessionUpdate(conn *SafeWebSocketConn, session *atcchat.ChatSession, systemPrompt string) error {
	// Get config for session parameters
	config := h.service.GetConfig()

	sessionData := map[string]interface{}{
		"modalities":                 []string{"text", "audio"},
		"instructions":               systemPrompt,
		"voice":                      config.Voice,
		"input_audio_format":         config.InputAudioFormat,
		"output_audio_format":        config.OutputAudioFormat,
		"temperature":                config.Temperature,
		"speed":                      config.Speed,
		"max_response_output_tokens": config.MaxResponseTokens,
		"tool_choice":                "auto",
		"tools":                      []interface{}{},
	}

	// Add turn detection only if not disabled
	if config.TurnDetectionType != "" && config.TurnDetectionType != "none" {
		sessionData["turn_detection"] = map[string]interface{}{
			"type":                config.TurnDetectionType,
			"threshold":           config.VADThreshold,
			"prefix_padding_ms":   300,
			"silence_duration_ms": config.SilenceDurationMs,
			"create_response":     true,
			"interrupt_response":  true,
		}
	} else {
		// Explicitly set to null to disable turn detection
		sessionData["turn_detection"] = nil
	}

	sessionUpdate := map[string]interface{}{
		"type":    "session.update",
		"session": sessionData,
	}

	// Add input audio transcription
	sessionData["input_audio_transcription"] = map[string]interface{}{
		"model": "whisper-1",
	}

	updateData, err := json.Marshal(sessionUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal session update: %w", err)
	}

	h.logger.Info("Sending session.update with custom instructions",
		logger.String("session_id", session.ID),
		logger.Int("prompt_length", len(systemPrompt)),
		logger.String("prompt_preview", systemPrompt[:min(200, len(systemPrompt))]),
		logger.String("voice", config.Voice),
		logger.Float64("temperature", config.Temperature),
		logger.Float64("speed", config.Speed))

	// Log the full session update for debugging
	h.logger.Debug("Full session.update payload",
		logger.String("session_id", session.ID),
		logger.String("payload", string(updateData)))

	if err := conn.WriteMessage(websocket.TextMessage, updateData); err != nil {
		return fmt.Errorf("failed to send session update: %w", err)
	}

	h.logger.Info("Successfully sent session.update to OpenAI",
		logger.String("session_id", session.ID))

	return nil
}

// handleContextUpdates listens for context updates from the service and sends session.update events to OpenAI
func (h *ATCChatHandlers) handleContextUpdates(ctx context.Context, openaiConn *SafeWebSocketConn, session *atcchat.ChatSession, updateChan <-chan string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case updateMessage, ok := <-updateChan:
			if !ok {
				h.logger.Info("Context update channel closed", logger.String("session_id", session.ID))
				return nil
			}

			h.logger.Debug("Received context update, forwarding to OpenAI",
				logger.String("session_id", session.ID),
				logger.Int("message_length", len(updateMessage)))

			// The service sends a complete JSON message, so we need to parse it and send as raw message
			if err := openaiConn.WriteMessage(websocket.TextMessage, []byte(updateMessage)); err != nil {
				h.logger.Error("Failed to send context update to OpenAI",
					logger.String("session_id", session.ID),
					logger.Error(err))
				return fmt.Errorf("failed to send context update: %w", err)
			}

			h.logger.Debug("Successfully sent context update to OpenAI",
				logger.String("session_id", session.ID))
		}
	}
}

// forwardClientToOpenAI forwards messages from client to OpenAI
func (h *ATCChatHandlers) forwardClientToOpenAI(ctx context.Context, clientConn *websocket.Conn, openaiConn *SafeWebSocketConn, session *atcchat.ChatSession) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.logger.Error("Client WebSocket error", logger.Error(err))
				}
				return err
			}

			h.logger.Debug("Forwarding client message to OpenAI",
				logger.String("session_id", session.ID),
				logger.Int("message_type", messageType),
				logger.Int("size", len(message)))

			// Forward message to OpenAI
			if err := openaiConn.WriteMessage(messageType, message); err != nil {
				h.logger.Error("Failed to forward message to OpenAI", logger.Error(err))
				return err
			}
		}
	}
}

// forwardOpenAIToClient forwards messages from OpenAI to client
func (h *ATCChatHandlers) forwardOpenAIToClient(ctx context.Context, openaiConn *SafeWebSocketConn, clientConn *websocket.Conn, session *atcchat.ChatSession, systemPrompt string) error {
	sessionUpdateSent := false
	usingSessionBasedConnection := session.OpenAISessionID != "" && session.ClientSecret != ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			messageType, message, err := openaiConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.logger.Error("OpenAI WebSocket error", logger.Error(err))
				}
				return err
			}

			// Parse and log important events
			if messageType == websocket.TextMessage {
				var event map[string]interface{}
				if err := json.Unmarshal(message, &event); err == nil {
					if eventType, ok := event["type"].(string); ok {
						switch eventType {
						case "session.created":
							h.logger.Info("Received session.created from OpenAI",
								logger.String("session_id", session.ID),
								logger.Bool("using_session_based_connection", usingSessionBasedConnection))

							// ALWAYS send session.update after session.created, regardless of connection type
							// The REST API session creation doesn't actually apply instructions - they must be set via WebSocket
							if !sessionUpdateSent {
								h.logger.Info("Sending session.update to apply custom instructions",
									logger.String("session_id", session.ID),
									logger.Bool("using_session_based_connection", usingSessionBasedConnection))

								// Generate fresh templated prompt for the initial update
								templatedPrompt, err := h.service.GenerateSystemPrompt(session.ID)
								if err != nil {
									h.logger.Error("Failed to generate templated prompt for initial session update", logger.Error(err))
									// Fallback to static prompt
									templatedPrompt = systemPrompt
								}

								if err := h.sendSessionUpdate(openaiConn, session, templatedPrompt); err != nil {
									h.logger.Error("Failed to send session update after session.created", logger.Error(err))
									return err
								}
								sessionUpdateSent = true
							}

							// Log the instructions from the session.created event for comparison
							if sessionData, ok := event["session"].(map[string]interface{}); ok {
								if instructions, ok := sessionData["instructions"].(string); ok {
									h.logger.Info("Session instructions from session.created (before update)",
										logger.String("session_id", session.ID),
										logger.Int("instructions_length", len(instructions)),
										logger.String("instructions_preview", instructions[:min(200, len(instructions))]))

								}
							}

						case "session.updated":
							h.logger.Info("Received session.updated from OpenAI - instructions applied!",
								logger.String("session_id", session.ID))

							// Log the actual instructions that were applied
							if sessionData, ok := event["session"].(map[string]interface{}); ok {
								if instructions, ok := sessionData["instructions"].(string); ok {
									h.logger.Info("Confirmed instructions in session.updated",
										logger.String("session_id", session.ID),
										logger.Int("instructions_length", len(instructions)),
										logger.String("instructions_preview", instructions[:min(200, len(instructions))]))
								}
							}

						case "error":
							h.logger.Error("Received error from OpenAI",
								logger.String("session_id", session.ID),
								logger.Any("error", event))
						}
					}
				}
			}

			h.logger.Debug("Forwarding OpenAI message to client",
				logger.String("session_id", session.ID),
				logger.Int("message_type", messageType),
				logger.Int("size", len(message)))

			// Forward message to client
			if err := clientConn.WriteMessage(messageType, message); err != nil {
				h.logger.Error("Failed to forward message to client", logger.Error(err))
				return err
			}
		}
	}
}
