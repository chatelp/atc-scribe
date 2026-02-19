package adsb

import (
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
	case websocket.MessageTypeSimulationControlUpdate:
		return h.handleSimulationControlUpdate(client, data)
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

	// Send response back to client
	message := &websocket.Message{
		Type: websocket.MessageTypeAircraftBulkResponse,
		Data: map[string]interface{}{
			"aircraft": response.Aircraft,
			"count":    response.Count,
			"counts":   response.Counts,
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

// handleSimulationControlUpdate processes simulation control update messages
func (h *WebSocketHandler) handleSimulationControlUpdate(client *websocket.Client, data map[string]interface{}) error {
	h.logger.Debug("Handling simulation control update")

	// Parse required fields
	hex, ok := data["hex"].(string)
	if !ok || hex == "" {
		h.logger.Warn("Missing or invalid hex in simulation control update")
		return nil
	}

	heading, _ := data["heading"].(float64)
	speed, _ := data["speed"].(float64)
	verticalRate, _ := data["vertical_rate"].(float64)

	// Update simulation controls via the service
	if err := h.service.UpdateSimulationControls(hex, heading, speed, verticalRate); err != nil {
		h.logger.Error("Failed to update simulation controls",
			logger.String("hex", hex),
			logger.Error(err))
		return err
	}

	h.logger.Info("Updated simulation controls via WebSocket",
		logger.String("hex", hex),
		logger.Float64("heading", heading),
		logger.Float64("speed", speed),
		logger.Float64("vertical_rate", verticalRate))

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
