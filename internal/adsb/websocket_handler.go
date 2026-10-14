package adsb

import (
	"time"

	"github.com/yegors/co-atc/internal/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// WebSocketHandler handles incoming WebSocket messages for ADSB data
type WebSocketHandler struct {
	service *Service
	logger  *logger.Logger
}

// NewWebSocketHandler creates a new WebSocket message handler
func NewWebSocketHandler(service *Service, logger *logger.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		service: service,
		logger:  logger.Named("adsb-ws-handler"),
	}
}

// HandleMessage handles incoming WebSocket messages
func (h *WebSocketHandler) HandleMessage(client *websocket.Client, messageType string, data map[string]interface{}) error {
	switch messageType {
	case websocket.MessageTypeAircraftBulkRequest:
		return h.handleBulkRequest(client, data)
	case websocket.MessageTypeFilterUpdate:
		return h.handleFilterUpdate(client, data)
	default:
		h.logger.Debug("Unhandled message type", logger.String("type", messageType))
		return nil
	}
}

// handleBulkRequest processes requests for bulk aircraft data
func (h *WebSocketHandler) handleBulkRequest(client *websocket.Client, data map[string]interface{}) error {
	h.logger.Debug("Handling bulk aircraft data request")

	// Parse filters from the request
	filters := make(map[string]interface{})
	if filtersData, ok := data["filters"].(map[string]interface{}); ok {
		filters = filtersData
	}

	// Get bulk aircraft data from service
	response, err := h.service.HandleBulkRequest(filters)
	if err != nil {
		h.logger.Error("Failed to get bulk aircraft data", logger.Error(err))
		return err
	}

	// Send response back to client. sent_at is the server's clock as it
	// answers: the page dates the aircraft's positions by it, as it dates the
	// stream's by observed_at; without it the page took its clock from the
	// aircraft's poll times, up to a second and a half late, and drew every
	// aircraft that much behind for the first seconds (fiche M).
	message := &websocket.Message{
		Type: websocket.MessageTypeAircraftBulkResponse,
		Data: map[string]interface{}{
			"aircraft": response.Aircraft,
			"count":    response.Count,
			"counts":   response.Counts,
			"sent_at":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	}

	// Send to specific client (not broadcast)
	return h.sendToClient(client, message)
}

// handleFilterUpdate processes filter update messages from clients
// Note: Server-side filtering has been removed. All filtering is done client-side.
// This handler is kept for backward compatibility but does nothing.
func (h *WebSocketHandler) handleFilterUpdate(client *websocket.Client, data map[string]interface{}) error {
	h.logger.Debug("Filter update received (filtering is client-side only)")
	return nil
}

// sendToClient sends a message to a specific client
func (h *WebSocketHandler) sendToClient(client *websocket.Client, message *websocket.Message) error {
	//messageData, err := json.Marshal(message)
	//if err != nil {
	//	return err
	//}

	//h.logger.Debug("Sending message to client",
	//	logger.String("type", message.Type),
	//	logger.Int("data_size", len(messageData)))

	// Send message to the specific client
	if client.SendMessage(message) {
		return nil
	} else {
		h.logger.Warn("Client send channel full, dropping message")
		return nil
	}
}
