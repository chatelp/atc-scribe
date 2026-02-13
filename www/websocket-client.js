// WebSocket Client for Co-ATC
class WebSocketClient {
    constructor(url) {
        this.url = url;
        this.connection = null;
        this.reconnectTimeout = null;
        this.isReconnecting = false;
        this.autoReconnect = false;
        this.reconnectAttempts = 0;
        this.baseReconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.reconnectJitterFactor = 0.25;
        this.intentionalClose = false;
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
            frequency_status: [], // Frequency connection status changes
            open: [],
            close: [],
            error: []
        };

        // Bound event handlers for proper cleanup
        this._boundOpenHandler = null;
        this._boundCloseHandler = null;
        this._boundErrorHandler = null;
        this._boundMessageHandler = null;
    }

    // Connect to the WebSocket server
    connect() {
        // This is an active connect request, not an intentional shutdown
        this.intentionalClose = false;

        // Prevent duplicate reconnect timers from triggering additional connect() calls
        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
            this.reconnectTimeout = null;
        }

        // Prevent multiple simultaneous connection attempts
        if (this.isReconnecting) {
            console.log('WebSocket: Connection attempt already in progress');
            return;
        }

        // Close existing connection and cleanup handlers
        if (this.connection) {
            this._removeConnectionHandlers();
            this.connection.close();
            this.connection = null;
        }

        this.isReconnecting = true;

        // Create new WebSocket connection
        this.connection = new WebSocket(this.url);

        // Create bound handlers for proper cleanup later
        this._boundOpenHandler = (event) => {
            console.log('WebSocket connection established');
            this.isReconnecting = false;
            this.reconnectAttempts = 0; // Reset retry counter
            this._notifyListeners('open', event);
        };

        this._boundCloseHandler = (event) => {
            console.log('WebSocket connection closed');
            this.isReconnecting = false;
            this._notifyListeners('close', event);

            // Reconnect forever unless intentionally disconnected
            if (this.autoReconnect && !this.intentionalClose) {
                this.reconnectAttempts++;
                const delayMs = this._getReconnectDelayMs();
                console.log(`WebSocket: Attempting reconnect #${this.reconnectAttempts} in ${delayMs}ms`);

                this.reconnectTimeout = setTimeout(() => {
                    this.reconnectTimeout = null;
                    this.connect();
                }, delayMs);
            }
        };

        this._boundErrorHandler = (event) => {
            console.error('WebSocket error:', event);
            this.isReconnecting = false;
            this._notifyListeners('error', event);
        };

        this._boundMessageHandler = (event) => {
            // Process messages directly - no setTimeout to avoid event loop flooding
            try {
                const message = JSON.parse(event.data);

                // Handle aircraft streaming messages
                if (message.type === 'aircraft_added') {
                    this._notifyListeners('aircraft_added', message.data);
                } else if (message.type === 'aircraft_update') {
                    this._notifyListeners('aircraft_update', message.data);
                } else if (message.type === 'aircraft_removed') {
                    this._notifyListeners('aircraft_removed', message.data);
                } else if (message.type === 'aircraft_bulk_response') {
                    this._notifyListeners('aircraft_bulk_response', message.data);
                } else if (message.type === 'transcription') {
                    this._notifyListeners('transcription', message.data);
                } else if (message.type === 'transcription_update') {
                    this._notifyListeners('transcription_update', message.data);
                } else if (message.type === 'aircraft') {
                    if (message.data && message.data.movement) {
                        // Call the Alpine.js store method to handle the aircraft message
                        if (window.Alpine && Alpine.store('atc')) {
                            Alpine.store('atc').handleAircraftMessage(message.data);
                        }
                    }
                    this._notifyListeners('aircraft', message.data);
                } else if (message.type === 'status_update') {
                    // Call the Alpine.js store method to handle the status update
                    if (window.Alpine && Alpine.store('atc')) {
                        Alpine.store('atc').handleStatusUpdateMessage(message.data);
                    }
                    this._notifyListeners('status_update', message.data);
                } else if (message.type === 'phase_change') {
                    this._notifyListeners('phase_change', message.data);
                } else if (message.type === 'frequency_status') {
                    this._notifyListeners('frequency_status', message.data);
                }
            } catch (error) {
                console.error('Error parsing WebSocket message:', error);
            }
        };

        // Add event listeners
        this.connection.addEventListener('open', this._boundOpenHandler);
        this.connection.addEventListener('close', this._boundCloseHandler);
        this.connection.addEventListener('error', this._boundErrorHandler);
        this.connection.addEventListener('message', this._boundMessageHandler);
    }

    _getReconnectDelayMs() {
        const exponent = Math.max(0, this.reconnectAttempts-1);
        const exponentialDelay = this.baseReconnectDelay * Math.pow(2, exponent);
        const cappedDelay = Math.min(this.maxReconnectDelay, exponentialDelay);
        const jitterRange = cappedDelay * this.reconnectJitterFactor;
        const jitter = (Math.random() * 2 - 1) * jitterRange;
        return Math.max(250, Math.round(cappedDelay + jitter));
    }

    // Remove event handlers from connection (prevents memory leaks)
    _removeConnectionHandlers() {
        if (this.connection) {
            if (this._boundOpenHandler) {
                this.connection.removeEventListener('open', this._boundOpenHandler);
            }
            if (this._boundCloseHandler) {
                this.connection.removeEventListener('close', this._boundCloseHandler);
            }
            if (this._boundErrorHandler) {
                this.connection.removeEventListener('error', this._boundErrorHandler);
            }
            if (this._boundMessageHandler) {
                this.connection.removeEventListener('message', this._boundMessageHandler);
            }
        }

        // Clear references
        this._boundOpenHandler = null;
        this._boundCloseHandler = null;
        this._boundErrorHandler = null;
        this._boundMessageHandler = null;
    }

    // Close the WebSocket connection
    disconnect() {
        this.autoReconnect = false; // Disable auto-reconnect when manually disconnecting
        this.intentionalClose = true;

        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
            this.reconnectTimeout = null;
        }

        // Remove event handlers before closing to prevent memory leaks
        this._removeConnectionHandlers();

        if (this.connection) {
            this.connection.close();
            this.connection = null;
        }

        this.isReconnecting = false;
    }

    // Clear all custom event listeners (for cleanup on page unload)
    clearAllListeners() {
        Object.keys(this.listeners).forEach(type => {
            this.listeners[type] = [];
        });
    }

    // Enable auto-reconnect
    enableAutoReconnect() {
        this.autoReconnect = true;
        this.intentionalClose = false;
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

    // Note: updateFilters method removed - server-side filtering is no longer used.
    // All filtering is done client-side.

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