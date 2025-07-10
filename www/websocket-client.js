// WebSocket Client for Co-ATC
class WebSocketClient {
    constructor(url) {
        this.url = url;
        this.connection = null;
        this.reconnectTimeout = null;
        this.isReconnecting = false;
        this.autoReconnect = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 5000;
        this.listeners = {
            transcription: [],
            transcription_update: [],
            aircraft: [],
            aircraft_added: [],         // NEW
            aircraft_update: [],        // NEW
            aircraft_removed: [],       // NEW
            aircraft_bulk_response: [], // NEW - for bulk data responses
            status_update: [], // Add new listener type for status updates
            phase_change: [], // Add new listener type for phase changes
            clearance_issued: [], // Add new listener type for clearance events
            open: [],
            close: [],
            error: []
        };
    }

    // Connect to the WebSocket server
    connect() {
        // Prevent multiple simultaneous connection attempts
        if (this.isReconnecting) {
            console.log('WebSocket: Connection attempt already in progress');
            return;
        }

        // Close existing connection if any
        if (this.connection) {
            this.connection.close();
        }

        this.isReconnecting = true;

        // Create new WebSocket connection
        this.connection = new WebSocket(this.url);

        // Connection opened
        this.connection.addEventListener('open', (event) => {
            console.log('WebSocket connection established');
            this.isReconnecting = false;
            this.reconnectAttempts = 0; // Reset retry counter
            this._notifyListeners('open', event);
        });

        // Connection closed
        this.connection.addEventListener('close', (event) => {
            console.log('WebSocket connection closed');
            this.isReconnecting = false;
            this._notifyListeners('close', event);
            
            // Only attempt auto-reconnect if enabled and under max attempts
            if (this.autoReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
                this.reconnectAttempts++;
                console.log(`WebSocket: Attempting reconnect ${this.reconnectAttempts}/${this.maxReconnectAttempts}`);
                
                this.reconnectTimeout = setTimeout(() => {
                    this.connect();
                }, this.reconnectDelay);
            } else if (this.reconnectAttempts >= this.maxReconnectAttempts) {
                console.error('WebSocket: Max reconnection attempts reached. Stopping auto-reconnect.');
            }
        });

        // Connection error
        this.connection.addEventListener('error', (event) => {
            console.error('WebSocket error:', event);
            this.isReconnecting = false;
            this._notifyListeners('error', event);
        });

        // Listen for messages - CRITICAL FIX: Process asynchronously to prevent main thread blocking
        this.connection.addEventListener('message', (event) => {
            // IMMEDIATELY yield control back to browser to prevent blocking HTTP requests
            setTimeout(() => {
                try {
                    const message = JSON.parse(event.data);
                    
                    // Handle aircraft streaming messages
                    if (message.type === 'aircraft_added') {
                        console.log(`Aircraft ADDED: ${message.data.aircraft?.flight || message.data.hex}`);
                        this._notifyListeners('aircraft_added', message.data);
                    } else if (message.type === 'aircraft_update') {
                        const changeKeys = Object.keys(message.data.changes || {}).join(', ');
                        //console.log(`Aircraft UPDATED: ${message.data.hex} - ${changeKeys}`);
                        this._notifyListeners('aircraft_update', message.data);
                    } else if (message.type === 'aircraft_removed') {
                        console.log(`Aircraft REMOVED: ${message.data.hex}`);
                        this._notifyListeners('aircraft_removed', message.data);
                    } else if (message.type === 'aircraft_bulk_response') {
                        console.log(`BULK DATA: Received ${message.data.count} aircraft`);
                        this._notifyListeners('aircraft_bulk_response', message.data);
                    } else if (message.type === 'transcription') {
                        this._notifyListeners('transcription', message.data);
                    } else if (message.type === 'transcription_update') {
                        this._notifyListeners('transcription_update', message.data);
                    } else if (message.type === 'aircraft') {
                        // Log aircraft movement messages to console
                        if (message.data && message.data.movement) {
                            console.log(`Aircraft ${message.data.movement.toUpperCase()}: ${message.data.flight || message.data.hex}`, message.data);
                            
                            // Call the Alpine.js store method to handle the aircraft message
                            if (window.Alpine && Alpine.store('atc')) {
                                Alpine.store('atc').handleAircraftMessage(message.data);
                            }
                        }
                        this._notifyListeners('aircraft', message.data);
                    } else if (message.type === 'status_update') {
                        // Log status update messages to console
                        console.log(`Aircraft STATUS CHANGE: ${message.data.flight || message.data.hex} -> ${message.data.new_status.toUpperCase()}`, message.data);
                        
                        // Call the Alpine.js store method to handle the status update
                        if (window.Alpine && Alpine.store('atc')) {
                            Alpine.store('atc').handleStatusUpdateMessage(message.data);
                        }
                        
                        this._notifyListeners('status_update', message.data);
                    } else if (message.type === 'phase_change') {
                        // Log phase change messages to console
                        console.log(`Phase Change: ${message.data.flight || message.data.hex} ${message.data.transition}`, message.data);
                        
                        this._notifyListeners('phase_change', message.data);
                    }
                } catch (error) {
                    console.error('Error parsing WebSocket message:', error);
                }
            }, 0); // Yield to browser event loop immediately
        });
    }

    // Close the WebSocket connection
    disconnect() {
        this.autoReconnect = false; // Disable auto-reconnect when manually disconnecting
        
        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
            this.reconnectTimeout = null;
        }

        if (this.connection) {
            this.connection.close();
            this.connection = null;
        }
        
        this.isReconnecting = false;
    }

    // Enable auto-reconnect
    enableAutoReconnect() {
        this.autoReconnect = true;
    }

    // Disable auto-reconnect
    disableAutoReconnect() {
        this.autoReconnect = false;
    }

    // Reset reconnection attempts
    resetReconnectAttempts() {
        this.reconnectAttempts = 0;
    }

    // Add event listener
    addEventListener(type, callback) {
        if (this.listeners[type]) {
            this.listeners[type].push(callback);
        }
    }

    // Remove event listener
    removeEventListener(type, callback) {
        if (this.listeners[type]) {
            this.listeners[type] = this.listeners[type].filter(cb => cb !== callback);
        }
    }

    // Method to request bulk aircraft data via WebSocket
    requestBulkAircraftData(filters = {}) {
        if (this.connection && this.connection.readyState === WebSocket.OPEN) {
            const message = {
                type: 'aircraft_bulk_request',
                data: {
                    filters: filters
                }
            };
            
            console.log('Requesting bulk aircraft data via WebSocket...', filters);
            this.connection.send(JSON.stringify(message));
        } else {
            console.error('WebSocket not connected, cannot request bulk data');
        }
    }

    // Method to send filter updates to server
    updateFilters(filters) {
        if (this.connection && this.connection.readyState === WebSocket.OPEN) {
            const message = {
                type: 'filter_update',
                data: filters
            };
            
            console.log('Sending filter update:', filters);
            this.connection.send(JSON.stringify(message));
        } else {
            console.warn('WebSocket not connected, cannot send filter update');
        }
    }

    // Notify all listeners of an event
    _notifyListeners(type, data) {
        if (this.listeners[type]) {
            this.listeners[type].forEach(callback => {
                try {
                    callback(data);
                } catch (error) {
                    console.error(`Error in ${type} listener:`, error);
                }
            });
        }
    }
}

// Export the WebSocketClient class
window.WebSocketClient = WebSocketClient;