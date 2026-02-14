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
	MessageTypeAircraftAdded           = "aircraft_added"
	MessageTypeAircraftUpdate          = "aircraft_update"
	MessageTypeAircraftRemoved         = "aircraft_removed"
	MessageTypeAircraftPredictedState  = "aircraft_predicted_state"
	MessageTypeAircraftBulkRequest     = "aircraft_bulk_request"     // Client requests bulk data
	MessageTypeAircraftBulkResponse    = "aircraft_bulk_response"    // Server sends bulk data
	MessageTypeFilterUpdate            = "filter_update"             // Client sends filter preferences
	MessageTypeSimulationControlUpdate = "simulation_control_update" // Client updates simulation controls
	MessageTypeFrequencyStatus         = "frequency_status"          // Frequency connection status changes
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

// Client represents a WebSocket client
type Client struct {
	conn      *websocket.Conn
	send      chan *Message
	server    *Server
	mu        sync.Mutex
	closed    bool
	closeChan chan struct{}
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
		register:   make(chan *Client, 32),   // Buffered to prevent goroutine blocking
		unregister: make(chan *Client, 32),   // Buffered to prevent goroutine blocking
		broadcast:  make(chan *Message, 512), // Buffered for high-throughput broadcasting
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

				// Send to all clients - filtering is done client-side
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

	// Aircraft movement updates should be dispatched immediately (no server-side queueing)
	if message.Type == MessageTypeAircraftAdded || message.Type == MessageTypeAircraftUpdate || message.Type == MessageTypeAircraftRemoved || message.Type == MessageTypeAircraftPredictedState {
		s.broadcastImmediate(message)
		return
	}

	s.broadcast <- message
}

// broadcastImmediate sends a message directly to all clients without using the broadcast queue.
func (s *Server) broadcastImmediate(message *Message) {
	s.mu.RLock()
	clientsToRemove := make([]*Client, 0)
	for client := range s.clients {
		client.mu.Lock()
		if client.closed {
			clientsToRemove = append(clientsToRemove, client)
			client.mu.Unlock()
			continue
		}
		client.mu.Unlock()

		if !client.SendMessage(message) {
			clientsToRemove = append(clientsToRemove, client)
		}
	}
	s.mu.RUnlock()

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

// Import logger functions
var (
	String = logger.String
	Error  = logger.Error
)
