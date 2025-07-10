package adsb

import (
	"encoding/json"

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
func (h *WebSocketHandler) handleFilterUpdate(client *websocket.Client, data map[string]interface{}) error {
	h.logger.Debug("Handling filter update request")

	// Parse filter data
	var filters websocket.ClientFilters

	if showAir, ok := data["show_air"].(bool); ok {
		filters.ShowAir = showAir
	}

	if showGround, ok := data["show_ground"].(bool); ok {
		filters.ShowGround = showGround
	}

	if phases, ok := data["phases"].(map[string]interface{}); ok {
		filters.Phases = make(map[string]bool)
		for phase, enabled := range phases {
			if enabledBool, ok := enabled.(bool); ok {
				filters.Phases[phase] = enabledBool
			}
		}
	}

	// Parse selected aircraft hex
	if selectedHex, ok := data["selected_aircraft_hex"].(string); ok {
		filters.SelectedAircraftHex = selectedHex
	}

	// Update client filters
	client.UpdateFilters(&filters)

	h.logger.Info("Updated client filters",
		logger.Bool("show_air", filters.ShowAir),
		logger.Bool("show_ground", filters.ShowGround),
		logger.Int("phase_count", len(filters.Phases)))

	// Convert filters to service format and get filtered aircraft
	serviceFilters := make(map[string]interface{})
	serviceFilters["show_air"] = filters.ShowAir
	serviceFilters["show_ground"] = filters.ShowGround

	// Convert phases map to array of enabled phases
	if len(filters.Phases) > 0 {
		enabledPhases := make([]string, 0)
		for phase, enabled := range filters.Phases {
			if enabled {
				enabledPhases = append(enabledPhases, phase)
			}
		}
		serviceFilters["phases"] = enabledPhases
	}

	// Parse additional filters from the request (same as handleBulkRequest)
	if val, ok := data["min_altitude"].(float64); ok {
		serviceFilters["min_altitude"] = val
	}
	if val, ok := data["max_altitude"].(float64); ok {
		serviceFilters["max_altitude"] = val
	}
	if val, ok := data["last_seen_minutes"].(float64); ok {
		serviceFilters["last_seen_minutes"] = val
	}
	if val, ok := data["exclude_other_airports_grounded"].(bool); ok {
		serviceFilters["exclude_other_airports_grounded"] = val
	}

	// Get filtered aircraft data
	response, err := h.service.HandleBulkRequest(serviceFilters)
	if err != nil {
		h.logger.Error("Failed to get filtered aircraft data", logger.Error(err))
		return err
	}

	// Send filtered aircraft data back to client
	message := &websocket.Message{
		Type: "aircraft_bulk_response",
		Data: map[string]interface{}{
			"aircraft": response.Aircraft,
			"count":    response.Count,
			"counts":   response.Counts,
		},
	}

	// Send to specific client (not broadcast)
	return h.sendToClient(client, message)
}

// sendToClient sends a message to a specific client
func (h *WebSocketHandler) sendToClient(client *websocket.Client, message *websocket.Message) error {
	messageData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.logger.Debug("Sending message to client",
		logger.String("type", message.Type),
		logger.Int("data_size", len(messageData)))

	// Send message to the specific client
	if client.SendMessage(message) {
		return nil
	} else {
		h.logger.Warn("Client send channel full, dropping message")
		return nil
	}
}
