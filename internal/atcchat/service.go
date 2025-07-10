package atcchat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/templating"
	"github.com/yegors/co-atc/pkg/logger"
)

// TemplatingService interface for shared templating functionality
type TemplatingService interface {
	RenderATCChatTemplate(templatePath string) (string, error)
	GetTemplateContext(opts FormattingOptions) (*TemplateContext, error)
}

// Import templating types
type FormattingOptions = templating.FormattingOptions
type TemplateContext = templating.TemplateContext

// ATCChatFormattingOptions returns formatting options for ATC chat
func ATCChatFormattingOptions() FormattingOptions {
	return templating.ATCChatFormattingOptions()
}

// Service manages ATC chat sessions and interactions
type Service struct {
	realtimeClient    *RealtimeClient
	templatingService TemplatingService
	config            *config.ATCChatConfig
	logger            *logger.Logger

	// Session management
	sessions   map[string]*ChatSession
	sessionsMu sync.RWMutex

	// WebSocket connection registry for sending updates
	wsConnections   map[string]chan string // sessionID -> update channel
	wsConnectionsMu sync.RWMutex

	// Background tasks
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new ATC chat service
func NewService(
	templatingService TemplatingService,
	config *config.Config,
	logger *logger.Logger,
) (*Service, error) {
	if !config.ATCChat.Enabled {
		return nil, fmt.Errorf("ATC chat is disabled in configuration")
	}

	// Create session config
	sessionConfig := SessionConfig{
		InputAudioFormat:  config.ATCChat.InputAudioFormat,
		OutputAudioFormat: config.ATCChat.OutputAudioFormat,
		SampleRate:        config.ATCChat.SampleRate,
		Channels:          config.ATCChat.Channels,
		MaxResponseTokens: config.ATCChat.MaxResponseTokens,
		Temperature:       config.ATCChat.Temperature,
		TurnDetectionType: config.ATCChat.TurnDetectionType,
		VADThreshold:      config.ATCChat.VADThreshold,
		SilenceDurationMs: config.ATCChat.SilenceDurationMs,
		Voice:             config.ATCChat.Voice,
		Model:             config.ATCChat.RealtimeModel,
	}

	// Create realtime client
	realtimeClient := NewRealtimeClient(
		config.ATCChat.OpenAIAPIKey,
		sessionConfig,
		logger,
	)

	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		realtimeClient:    realtimeClient,
		templatingService: templatingService,
		config:            &config.ATCChat,
		logger:            logger.Named("atc-chat-service"),
		sessions:          make(map[string]*ChatSession),
		wsConnections:     make(map[string]chan string),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Start background tasks
	service.startBackgroundTasks()

	return service, nil
}

// CreateSession creates a new chat session
func (s *Service) CreateSession(ctx context.Context) (*ChatSession, error) {
	s.logger.Info("Creating new ATC chat session")

	// Use templating service to render the ATC chat template
	staticPrompt, err := s.templatingService.RenderATCChatTemplate(s.config.SystemPromptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to render ATC chat template: %w", err)
	}

	// Create OpenAI session via REST API with static instructions
	s.logger.Info("Creating OpenAI session via REST API with static instructions",
		logger.Int("prompt_length", len(staticPrompt)))

	session, err := s.realtimeClient.CreateSession(ctx, staticPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI session: %w", err)
	}

	// Store session
	s.sessionsMu.Lock()
	s.sessions[session.ID] = session
	s.sessionsMu.Unlock()

	s.logger.Info("Successfully created ATC chat session with OpenAI session",
		logger.String("session_id", session.ID),
		logger.String("openai_session_id", session.OpenAISessionID),
		logger.Int("total_sessions", len(s.sessions)))

	return session, nil
}

// GetSession retrieves a session by ID
func (s *Service) GetSession(sessionID string) (*ChatSession, error) {
	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Validate session
	if !s.realtimeClient.ValidateSession(session) {
		return nil, fmt.Errorf("session is invalid or expired: %s", sessionID)
	}

	return session, nil
}

// EndSession terminates a chat session
func (s *Service) EndSession(ctx context.Context, sessionID string) error {
	s.logger.Info("Ending ATC chat session",
		logger.String("session_id", sessionID))

	s.sessionsMu.Lock()
	session, exists := s.sessions[sessionID]
	if exists {
		delete(s.sessions, sessionID)
	}
	s.sessionsMu.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Unregister WebSocket connection to stop receiving updates
	s.UnregisterWebSocketConnection(sessionID)

	// End OpenAI session
	if err := s.realtimeClient.EndSession(ctx, session.OpenAISessionID); err != nil {
		s.logger.Error("Failed to end OpenAI session",
			logger.String("session_id", sessionID),
			logger.Error(err))
		// Continue with cleanup even if OpenAI session termination fails
	}

	// Mark session as inactive
	session.Active = false

	s.logger.Info("Successfully ended ATC chat session",
		logger.String("session_id", sessionID),
		logger.Int("remaining_sessions", len(s.sessions)))

	return nil
}

// GetSessionStatus returns the status of a session
func (s *Service) GetSessionStatus(sessionID string) (SessionStatus, error) {
	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		return SessionStatus{
			ID:        sessionID,
			Active:    false,
			Connected: false,
			Error:     "Session not found",
		}, nil
	}

	return s.realtimeClient.GetSessionStatus(session), nil
}

// ListActiveSessions returns all active sessions
func (s *Service) ListActiveSessions() []*ChatSession {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	var activeSessions []*ChatSession
	for _, session := range s.sessions {
		// Check both OpenAI session validity AND WebSocket connection
		if s.realtimeClient.ValidateSession(session) && s.hasActiveWebSocketConnection(session.ID) {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions
}

// UpdateSessionContext updates the system prompt for a session with fresh airspace data
func (s *Service) UpdateSessionContext(ctx context.Context, sessionID string) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Render updated system prompt using shared templating service
	systemPrompt, err := s.templatingService.RenderATCChatTemplate(s.config.SystemPromptPath)
	if err != nil {
		return fmt.Errorf("failed to render system prompt: %w", err)
	}

	// Update session instructions
	if err := s.realtimeClient.UpdateSessionInstructions(ctx, session.OpenAISessionID, systemPrompt); err != nil {
		return fmt.Errorf("failed to update session instructions: %w", err)
	}

	// Update last activity
	session.LastActivity = time.Now().UTC()

	s.logger.Debug("Updated session context",
		logger.String("session_id", sessionID))

	return nil
}

// GetAirspaceStatus returns current airspace status
func (s *Service) GetAirspaceStatus() map[string]interface{} {
	s.sessionsMu.RLock()
	sessionCount := len(s.sessions)
	s.sessionsMu.RUnlock()

	return map[string]interface{}{
		"active_sessions":    sessionCount,
		"templating_enabled": true,
	}
}

// startBackgroundTasks starts background maintenance tasks
func (s *Service) startBackgroundTasks() {
	s.wg.Add(1)
	go s.sessionCleanupTask()

	// Start automatic system prompt refresh task if enabled
	if s.config.RefreshSystemPromptSecs > 0 {
		s.wg.Add(1)
		go s.systemPromptRefreshTask()
	}
}

// sessionCleanupTask periodically cleans up expired sessions
func (s *Service) sessionCleanupTask() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExpiredSessions()
		}
	}
}

// systemPromptRefreshTask periodically sends system prompt updates to all active sessions
func (s *Service) systemPromptRefreshTask() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.config.RefreshSystemPromptSecs) * time.Second)
	defer ticker.Stop()

	s.logger.Info("Started automatic system prompt refresh task",
		logger.Int("interval_seconds", s.config.RefreshSystemPromptSecs))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("System prompt refresh task shutting down")
			return
		case <-ticker.C:
			s.refreshAllActiveSessions()
		}
	}
}

// refreshAllActiveSessions sends system prompt updates to all active sessions
func (s *Service) refreshAllActiveSessions() {
	activeSessions := s.ListActiveSessions()

	if len(activeSessions) == 0 {
		s.logger.Debug("No active sessions to refresh")
		return
	}

	s.logger.Info("Refreshing system prompt for all active sessions",
		logger.Int("session_count", len(activeSessions)))

	for _, session := range activeSessions {
		if err := s.sendSystemPromptUpdate(session.ID); err != nil {
			s.logger.Error("Failed to send system prompt update to session",
				logger.String("session_id", session.ID),
				logger.Error(err))
		}
	}
}

// sendSystemPromptUpdate sends a system prompt update to a specific session with detailed logging
func (s *Service) sendSystemPromptUpdate(sessionID string) error {
	// Check if session still exists before processing
	_, err := s.GetSession(sessionID)
	if err != nil {
		s.logger.Debug("Skipping system prompt update for non-existent session",
			logger.String("session_id", sessionID),
			logger.Error(err))
		return nil // Don't treat this as an error, just skip
	}

	// Generate fresh system prompt with current airspace data and get variables
	promptWithVars, err := s.GenerateSystemPromptWithVariables(sessionID)
	if err != nil {
		return fmt.Errorf("failed to generate system prompt with variables: %w", err)
	}

	// Log all the templated variables at info level with proper formatting
	fmt.Printf("\n=== System prompt refresh - templated variables (Session: %s) ===\n", sessionID)
	fmt.Printf("\nAircraft Data:\n%v\n", promptWithVars.Variables["Aircraft"])
	fmt.Printf("\nWeather Data:\n%v\n", promptWithVars.Variables["Weather"])
	fmt.Printf("\nRunway Data:\n%v\n", promptWithVars.Variables["Runways"])
	fmt.Printf("\nTranscription History:\n%v\n", promptWithVars.Variables["TranscriptionHistory"])
	fmt.Printf("\nAirport Data:\n%v\n", promptWithVars.Variables["Airport"])
	fmt.Printf("=== End templated variables ===\n\n")

	// Create session.update message
	sessionUpdate := map[string]interface{}{
		"type": "session.update",
		"session": map[string]interface{}{
			"instructions": promptWithVars.Prompt,
		},
	}

	// Convert to JSON
	updateData, err := json.Marshal(sessionUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal session update: %w", err)
	}

	// Send update through WebSocket channel
	s.SendSessionUpdate(sessionID, string(updateData))

	s.logger.Info("Successfully sent automatic system prompt update",
		logger.String("session_id", sessionID),
		logger.Int("prompt_length", len(promptWithVars.Prompt)))

	return nil
}

// UpdateSessionContextOnDemand updates the context for a specific session with fresh airspace data
// This is called when the user starts speaking (push-to-talk) to ensure latest data
func (s *Service) UpdateSessionContextOnDemand(sessionID string) error {
	s.logger.Debug("Updating session context on-demand for user interaction",
		logger.String("session_id", sessionID))

	// Generate fresh system prompt with current airspace data
	systemPrompt, err := s.GenerateSystemPrompt(sessionID)
	if err != nil {
		s.logger.Error("Failed to generate system prompt for on-demand update",
			logger.String("session_id", sessionID),
			logger.Error(err))
		return fmt.Errorf("failed to generate system prompt: %w", err)
	}

	// Create session.update message
	sessionUpdate := map[string]interface{}{
		"type": "session.update",
		"session": map[string]interface{}{
			"instructions": systemPrompt,
		},
	}

	// Convert to JSON
	updateData, err := json.Marshal(sessionUpdate)
	if err != nil {
		s.logger.Error("Failed to marshal session update for on-demand update",
			logger.String("session_id", sessionID),
			logger.Error(err))
		return fmt.Errorf("failed to marshal session update: %w", err)
	}

	// Send update through WebSocket channel
	s.SendSessionUpdate(sessionID, string(updateData))

	s.logger.Info("Successfully sent on-demand context update",
		logger.String("session_id", sessionID))

	return nil
}

// cleanupExpiredSessions removes expired or invalid sessions
func (s *Service) cleanupExpiredSessions() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	var expiredSessions []string
	for sessionID, session := range s.sessions {
		if !s.realtimeClient.ValidateSession(session) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	for _, sessionID := range expiredSessions {
		delete(s.sessions, sessionID)
		s.logger.Debug("Cleaned up expired session",
			logger.String("session_id", sessionID))
	}

	if len(expiredSessions) > 0 {
		s.logger.Info("Cleaned up expired sessions",
			logger.Int("expired_count", len(expiredSessions)),
			logger.Int("remaining_sessions", len(s.sessions)))
	}
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down ATC chat service")

	// Cancel background tasks
	s.cancel()

	// Wait for background tasks to complete
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Debug("Background tasks completed")
	case <-ctx.Done():
		s.logger.Warn("Shutdown timeout reached, forcing exit")
	}

	// End all active sessions
	s.sessionsMu.Lock()
	sessionIDs := make([]string, 0, len(s.sessions))
	for sessionID := range s.sessions {
		sessionIDs = append(sessionIDs, sessionID)
	}
	s.sessionsMu.Unlock()

	for _, sessionID := range sessionIDs {
		if err := s.EndSession(ctx, sessionID); err != nil {
			s.logger.Error("Failed to end session during shutdown",
				logger.String("session_id", sessionID),
				logger.Error(err))
		}
	}

	s.logger.Info("ATC chat service shutdown complete")
	return nil
}

// GetSessionCount returns the number of active sessions
func (s *Service) GetSessionCount() int {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return len(s.sessions)
}

// GenerateSystemPrompt generates a templated system prompt with current airspace data
func (s *Service) GenerateSystemPrompt(sessionID string) (string, error) {
	s.logger.Debug("Generating system prompt for session",
		logger.String("session_id", sessionID))

	// Generate prompt using shared templating service
	prompt, err := s.templatingService.RenderATCChatTemplate(s.config.SystemPromptPath)
	if err != nil {
		s.logger.Error("Failed to generate prompt from template", logger.Error(err))
		return "", fmt.Errorf("failed to generate prompt: %w", err)
	}

	s.logger.Info("Generated system prompt for ATC chat",
		logger.String("session_id", sessionID),
		logger.Int("prompt_length", len(prompt)))

	// Log prompt generation without full content to reduce log verbosity
	s.logger.Debug("System prompt generated successfully",
		logger.String("session_id", sessionID))

	return prompt, nil
}

// PromptWithVariables contains both the rendered prompt and individual template variables
type PromptWithVariables struct {
	Prompt    string                 `json:"prompt"`
	Variables map[string]interface{} `json:"variables"`
}

// GenerateSystemPromptWithVariables generates a templated system prompt and returns individual variables
func (s *Service) GenerateSystemPromptWithVariables(sessionID string) (*PromptWithVariables, error) {
	s.logger.Debug("Generating system prompt with variables for session",
		logger.String("session_id", sessionID))

	// Generate prompt using shared templating service
	prompt, err := s.templatingService.RenderATCChatTemplate(s.config.SystemPromptPath)
	if err != nil {
		s.logger.Error("Failed to generate prompt from template", logger.Error(err))
		return nil, fmt.Errorf("failed to generate prompt: %w", err)
	}

	// Get the actual template context to return real variable data
	context, err := s.templatingService.GetTemplateContext(ATCChatFormattingOptions())
	if err != nil {
		s.logger.Error("Failed to get template context for variables", logger.Error(err))
		// Fall back to simplified variables if context retrieval fails
		variables := map[string]interface{}{
			"Aircraft":             "Error retrieving aircraft data",
			"Weather":              "Error retrieving weather data",
			"Runways":              "Error retrieving runway data",
			"TranscriptionHistory": "Error retrieving transcription data",
			"Airport":              "Error retrieving airport data",
		}
		return &PromptWithVariables{
			Prompt:    prompt,
			Variables: variables,
		}, nil
	}

	// Format the actual template variables for display
	variables := map[string]interface{}{
		"Aircraft":             templating.FormatAircraftData(context.Aircraft, context.Airport),
		"Weather":              templating.FormatWeatherData(context.Weather),
		"Runways":              templating.FormatRunwayData(context.Runways),
		"TranscriptionHistory": templating.FormatTranscriptionHistory(context.TranscriptionHistory),
		"Airport":              templating.FormatAirportData(context.Airport),
	}

	s.logger.Info("Generated system prompt with variables for ATC chat",
		logger.String("session_id", sessionID),
		logger.Int("prompt_length", len(prompt)))

	return &PromptWithVariables{
		Prompt:    prompt,
		Variables: variables,
	}, nil
}

// GetRealtimeModel returns the configured realtime model
func (s *Service) GetRealtimeModel() string {
	return s.config.RealtimeModel
}

// IsEnabled returns whether the ATC chat service is enabled
func (s *Service) IsEnabled() bool {
	return s.config.Enabled
}

// GetConfig returns the ATC chat configuration
func (s *Service) GetConfig() *config.ATCChatConfig {
	return s.config
}

// RegisterWebSocketConnection registers a WebSocket connection for session updates
func (s *Service) RegisterWebSocketConnection(sessionID string) chan string {
	s.wsConnectionsMu.Lock()
	defer s.wsConnectionsMu.Unlock()

	updateChan := make(chan string, 10) // Buffer for updates
	s.wsConnections[sessionID] = updateChan

	s.logger.Debug("Registered WebSocket connection for session updates",
		logger.String("session_id", sessionID))

	return updateChan
}

// UnregisterWebSocketConnection removes a WebSocket connection from the registry
func (s *Service) UnregisterWebSocketConnection(sessionID string) {
	s.wsConnectionsMu.Lock()
	defer s.wsConnectionsMu.Unlock()

	if updateChan, exists := s.wsConnections[sessionID]; exists {
		close(updateChan)
		delete(s.wsConnections, sessionID)

		s.logger.Debug("Unregistered WebSocket connection for session updates",
			logger.String("session_id", sessionID))
	}
}

// hasActiveWebSocketConnection checks if a session has an active WebSocket connection
func (s *Service) hasActiveWebSocketConnection(sessionID string) bool {
	s.wsConnectionsMu.RLock()
	defer s.wsConnectionsMu.RUnlock()

	_, exists := s.wsConnections[sessionID]
	return exists
}

// SendSessionUpdate sends a session update to a specific session's WebSocket connection
func (s *Service) SendSessionUpdate(sessionID string, updateMessage string) {
	s.wsConnectionsMu.RLock()
	updateChan, exists := s.wsConnections[sessionID]
	s.wsConnectionsMu.RUnlock()

	if exists {
		select {
		case updateChan <- updateMessage:
			s.logger.Debug("Sent session update to WebSocket",
				logger.String("session_id", sessionID))
		default:
			s.logger.Warn("Failed to send session update - channel full",
				logger.String("session_id", sessionID))
		}
	}
}
