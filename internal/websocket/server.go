package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/yegors/co-atc/pkg/logger"
)

// New message types for aircraft streaming
const (
	MessageTypeAircraftAdded        = "aircraft_added"
	MessageTypeAircraftUpdate       = "aircraft_update"
	MessageTypeAircraftRemoved      = "aircraft_removed"
	MessageTypeAircraftBulkRequest  = "aircraft_bulk_request"  // Client requests bulk data
	MessageTypeAircraftBulkResponse = "aircraft_bulk_response" // Server sends bulk data
	MessageTypeFilterUpdate         = "filter_update"          // Client sends filter preferences
)

// Message represents a WebSocket message
type Message struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// AircraftBulkRequest represents client request for bulk aircraft data
type AircraftBulkRequest struct {
	Filters map[string]interface{} `json:"filters"` // Filter parameters
}

// MessageHandler defines the interface for handling incoming WebSocket messages
type MessageHandler interface {
	HandleMessage(client *Client, messageType string, data map[string]interface{}) error
}

// ClientFilters represents the active filters for a WebSocket client
type ClientFilters struct {
	ShowAir             bool            `json:"show_air"`
	ShowGround          bool            `json:"show_ground"`
	Phases              map[string]bool `json:"phases"`                // phase -> enabled
	SelectedAircraftHex string          `json:"selected_aircraft_hex"` // hex of currently selected aircraft
}

// Client represents a WebSocket client
type Client struct {
	conn      *websocket.Conn
	send      chan *Message
	server    *Server
	mu        sync.Mutex
	closed    bool
	closeChan chan struct{}
	filters   *ClientFilters // Active filters for this client
}

// Server represents a WebSocket server
type Server struct {
	clients        map[*Client]bool
	register       chan *Client
	unregister     chan *Client
	broadcast      chan *Message
	upgrader       websocket.Upgrader
	logger         *logger.Logger
	mu             sync.RWMutex
	messageHandler MessageHandler // Handler for incoming messages
}

// NewServer creates a new WebSocket server
func NewServer(logger *logger.Logger) *Server {
	return &Server{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
		logger: logger.Named("web-socket"),
	}
}

// SetMessageHandler sets the message handler for incoming WebSocket messages
func (s *Server) SetMessageHandler(handler MessageHandler) {
	s.messageHandler = handler
}

// Run starts the WebSocket server
func (s *Server) Run() {
	s.logger.Info("Starting WebSocket server")

	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			clientCount := len(s.clients)
			s.mu.Unlock()
			s.logger.Debug("Client registered", String("client_count", fmt.Sprintf("%d", clientCount)))

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				// Mark client as closed first to prevent new messages
				client.mu.Lock()
				client.closed = true
				client.mu.Unlock()
				// Then close the channel
				close(client.send)
			}
			clientCount := len(s.clients)
			s.mu.Unlock()
			s.logger.Debug("Client unregistered", String("client_count", fmt.Sprintf("%d", clientCount)))

		case message := <-s.broadcast:
			s.mu.RLock()
			clientsToRemove := make([]*Client, 0)
			for client := range s.clients {
				// Check if client is still valid before sending
				client.mu.Lock()
				if client.closed {
					clientsToRemove = append(clientsToRemove, client)
					client.mu.Unlock()
					continue
				}
				client.mu.Unlock()

				// Filter aircraft updates based on client preferences
				shouldSend := s.shouldSendToClient(client, message)
				if !shouldSend {
					continue
				}

				select {
				case client.send <- message:
					// Message sent successfully
				default:
					// Channel is full, mark for removal
					clientsToRemove = append(clientsToRemove, client)
				}
			}
			s.mu.RUnlock()

			// Clean up failed clients
			if len(clientsToRemove) > 0 {
				s.mu.Lock()
				for _, client := range clientsToRemove {
					if _, ok := s.clients[client]; ok {
						delete(s.clients, client)
						client.mu.Lock()
						if !client.closed {
							client.closed = true
							close(client.send)
						}
						client.mu.Unlock()
					}
				}
				s.mu.Unlock()
			}
		}
	}
}

// HandleConnection handles a WebSocket connection
func (s *Server) HandleConnection(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Handling new WebSocket connection request",
		String("remote_addr", r.RemoteAddr),
		String("user_agent", r.UserAgent()))

	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade connection",
			Error(err),
			String("remote_addr", r.RemoteAddr))
		return
	}

	s.logger.Debug("Successfully upgraded connection to WebSocket",
		String("remote_addr", r.RemoteAddr))

	// Create client
	client := &Client{
		conn:      conn,
		send:      make(chan *Message, 256),
		server:    s,
		closeChan: make(chan struct{}),
	}

	// Register client
	s.register <- client

	// Start client goroutines
	go client.readPump()
	go client.writePump()
}

// Broadcast sends a message to all connected clients
func (s *Server) Broadcast(message *Message) {
	s.logger.Debug("Broadcasting message to all clients",
		String("message_type", message.Type),
		String("client_count", fmt.Sprintf("%d", len(s.clients))))

	// Log the message content for debugging
	if messageData, err := json.Marshal(message); err == nil {
		s.logger.Debug("Message content", String("content", string(messageData)))
	}

	s.broadcast <- message
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.mu.Lock()
		if !c.closed {
			c.closed = true
		}
		c.mu.Unlock()

		c.server.unregister <- c
		c.conn.Close()
	}()

	for {
		// Check if client is closed
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			break
		}
		c.mu.Unlock()

		// Read message
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				c.server.logger.Error("WebSocket read error", Error(err))
			}
			break
		}

		// Parse incoming message
		var message struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(messageBytes, &message); err != nil {
			c.server.logger.Error("Failed to parse WebSocket message", Error(err))
			continue
		}

		c.server.logger.Debug("Received WebSocket message",
			String("type", message.Type),
			String("client", c.conn.RemoteAddr().String()))

		// Handle message if handler is set
		if c.server.messageHandler != nil {
			if err := c.server.messageHandler.HandleMessage(c, message.Type, message.Data); err != nil {
				c.server.logger.Error("Failed to handle WebSocket message",
					Error(err),
					String("type", message.Type))
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	defer func() {
		c.mu.Lock()
		if !c.closed {
			c.closed = true
		}
		c.mu.Unlock()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.mu.Unlock()
				return
			}

			// Marshal message to JSON
			data, err := json.Marshal(message)
			if err != nil {
				c.server.logger.Error("Failed to marshal message", Error(err))
				c.mu.Unlock()
				continue
			}

			// Write message
			c.server.logger.Debug("Sending message to client",
				String("message_type", message.Type),
				String("message_length", fmt.Sprintf("%d bytes", len(data))))

			w.Write(data)

			// Close writer
			if err := w.Close(); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()

		case <-c.closeChan:
			return
		}
	}
}

// Close closes the client connection
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	close(c.closeChan)
	c.conn.Close()
}

// SendMessage sends a message to this specific client
func (c *Client) SendMessage(message *Message) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if client is closed
	if c.closed {
		return false
	}

	// Try to send message with non-blocking select
	select {
	case c.send <- message:
		return true
	default:
		// Channel is full, drop message
		return false
	}
}

// UpdateFilters updates the client's active filters
func (c *Client) UpdateFilters(filters *ClientFilters) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filters = filters
}

// GetFilters returns a copy of the client's current filters
func (c *Client) GetFilters() *ClientFilters {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.filters == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	filtersCopy := &ClientFilters{
		ShowAir:    c.filters.ShowAir,
		ShowGround: c.filters.ShowGround,
		Phases:     make(map[string]bool),
	}
	for phase, enabled := range c.filters.Phases {
		filtersCopy.Phases[phase] = enabled
	}
	return filtersCopy
}

// MatchesFilters checks if an aircraft matches the client's active filters
func (c *Client) MatchesFilters(aircraft map[string]interface{}) bool {
	filters := c.GetFilters()
	if filters == nil {
		// No filters set, show everything
		return true
	}

	// Get aircraft hex for debugging
	hex, _ := aircraft["hex"].(string)

	// Always allow updates for the selected aircraft, regardless of filters
	if filters.SelectedAircraftHex != "" && hex == filters.SelectedAircraftHex {
		c.server.logger.Info("Aircraft is selected, allowing update regardless of filters", String("hex", hex))
		return true
	}

	// Check Air/Ground scope - CRITICAL: if both are false, filter out everything
	onGround, _ := aircraft["on_ground"].(bool)

	if !filters.ShowAir && !filters.ShowGround {
		// Both air and ground are disabled, filter out everything
		c.server.logger.Info("Aircraft filtered out - both Air and Ground disabled", String("hex", hex))
		return false
	}

	if onGround && !filters.ShowGround {
		// Debug log
		c.server.logger.Info("Aircraft is grounded but ShowGround=false, filtering out", String("hex", hex))
		return false
	}
	if !onGround && !filters.ShowAir {
		// Debug log
		c.server.logger.Info("Aircraft is airborne but ShowAir=false, filtering out", String("hex", hex))
		return false
	}

	// Check phase filter - only if we have phase restrictions
	if len(filters.Phases) > 0 {
		// Check if ALL phases are disabled
		allPhasesDisabled := true
		for _, enabled := range filters.Phases {
			if enabled {
				allPhasesDisabled = false
				break
			}
		}

		if allPhasesDisabled {
			// All phases are disabled, filter out everything
			c.server.logger.Info("Aircraft filtered out - all phases disabled", String("hex", hex))
			return false
		}

		// Get current phase from aircraft
		var currentPhase string
		if phaseData, ok := aircraft["phase_data"].(map[string]interface{}); ok {
			if current, ok := phaseData["current"].(map[string]interface{}); ok {
				if phase, ok := current["phase"].(string); ok {
					currentPhase = phase
				}
			}
		}

		// If we have a phase, check if it's enabled
		if currentPhase != "" {
			if enabled, exists := filters.Phases[currentPhase]; exists && !enabled {
				c.server.logger.Info("Aircraft phase is disabled, filtering out", String("hex", hex), String("phase", currentPhase))
				return false
			}
		}
	}

	c.server.logger.Info("Aircraft PASSED filters", String("hex", hex))
	return true
}

// shouldSendToClient determines if a message should be sent to a specific client based on their filters
func (s *Server) shouldSendToClient(client *Client, message *Message) bool {
	//s.logger.Info("shouldSendToClient called", String("message_type", message.Type))

	// Always send non-aircraft messages (alerts, transcriptions, etc.)
	if message.Type != MessageTypeAircraftAdded &&
		message.Type != MessageTypeAircraftUpdate &&
		message.Type != MessageTypeAircraftRemoved {
		//s.logger.Info("Non-aircraft message, sending", String("message_type", message.Type))
		return true
	}

	// Debug: Log what's actually in the message data
	s.logger.Debug("Message data keys", String("keys", fmt.Sprintf("%v", getMapKeys(message.Data))))

	// For aircraft messages, check if the aircraft matches client filters
	if aircraftData, exists := message.Data["aircraft"]; exists {
		// Convert aircraft data to map[string]interface{} for filtering
		var data map[string]interface{}

		// Try direct type assertion first
		if directMap, ok := aircraftData.(map[string]interface{}); ok {
			data = directMap
		} else {
			// Convert struct to map using JSON marshaling/unmarshaling
			if jsonBytes, err := json.Marshal(aircraftData); err == nil {
				if err := json.Unmarshal(jsonBytes, &data); err == nil {
					s.logger.Info("Converted aircraft struct to map for filtering")
				} else {
					s.logger.Error("Failed to unmarshal aircraft data", Error(err))
					return true // Send if we can't filter
				}
			} else {
				s.logger.Error("Failed to marshal aircraft data", Error(err))
				return true // Send if we can't filter
			}
		}

		if data != nil {
			result := client.MatchesFilters(data)
			hex, _ := data["hex"].(string)
			s.logger.Info("Aircraft message filter result", String("hex", hex), String("result", fmt.Sprintf("%t", result)))
			return result
		}
	}

	// If no aircraft data, send the message (e.g., aircraft_removed might not have full data)
	s.logger.Info("No aircraft data in message, sending")
	return true
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Import logger functions
var (
	String = logger.String
	Error  = logger.Error
)
