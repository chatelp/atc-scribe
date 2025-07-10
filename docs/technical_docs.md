# Co-ATC Technical Documentation

This document provides a comprehensive technical overview of the Co-ATC system architecture, implementation details, and internal workings.

## System Architecture

Co-ATC is built as a Go-based backend server with a web frontend, designed for real-time air traffic monitoring and AI-enhanced operations.

### Core Components

1. **Backend Server** (`cmd/server/main.go`)
   - Go-based HTTP/WebSocket server
   - Multi-port configuration support
   - Graceful shutdown with proper resource cleanup

2. **Frontend** (`www/`)
   - HTML5/CSS3/JavaScript web application
   - Alpine.js for reactive UI components
   - Leaflet.js for interactive mapping
   - Real-time WebSocket communication

3. **Data Storage**
   - SQLite database with pure Go driver (modernc.org/sqlite)
   - Daily database files: `co-atc-YYYY-MM-DD.db`
   - No CGO dependencies required

## Project Structure

```
co-atc/
├── cmd/                      # Application entry points
│   └── server/               # Main server application
├── internal/                 # Private application code
│   ├── adsb/                 # ADS-B data processing
│   │   ├── client.go         # Client for fetching ADS-B data
│   │   ├── atc_utils.go      # Aviation utilities and calculations
│   │   ├── external.go       # External ADS-B API integration
│   │   ├── models.go         # Data models for ADS-B
│   │   ├── service.go        # ADS-B service implementation
│   │   ├── change_detector.go # Aircraft change detection
│   │   └── websocket_handler.go # WebSocket message handling
│   ├── api/                  # API handlers and routes
│   │   ├── handlers.go       # API request handlers
│   │   ├── middleware.go     # API middleware
│   │   ├── routes.go         # API route definitions
│   │   ├── static.go         # Static file serving
│   │   ├── atc_chat_handlers.go # ATC chat API handlers
│   │   └── transcription_handlers.go # Transcription handlers
│   ├── atcchat/              # ATC Chat AI assistant
│   │   ├── models.go         # Chat data models
│   │   ├── realtime_client.go # OpenAI realtime API client
│   │   └── service.go        # Chat service implementation
│   ├── audio/                # Audio processing
│   │   ├── central_processor.go # Unified audio processing
│   │   ├── chunker.go        # Audio chunking for transcription
│   │   ├── multireader.go    # Multiple reader support
│   │   └── wavreader.go      # WAV format handling
│   ├── config/               # Configuration handling
│   │   └── config.go         # Configuration loading and validation
│   ├── frequencies/          # Frequency management
│   │   ├── client.go         # Audio stream client
│   │   ├── models.go         # Frequency data models
│   │   └── service.go        # Frequency service implementation
│   ├── simulation/           # Aircraft simulation
│   │   └── service.go        # Simulation service implementation
│   ├── storage/              # Data storage implementations
│   │   └── sqlite/           # SQLite storage
│   │       ├── aircraft.go   # Aircraft data storage
│   │       ├── clearances.go # ATC clearance storage
│   │       ├── clearance_models.go # Clearance data models
│   │       └── transcriptions.go # Transcription storage
│   ├── templating/           # Template system
│   │   ├── aggregator.go     # Data aggregation
│   │   ├── engine.go         # Template engine
│   │   ├── formatters.go     # Data formatters
│   │   ├── models.go         # Template models
│   │   └── templating.go     # Template utilities
│   ├── transcription/        # Audio transcription
│   │   ├── interface.go      # Transcription interfaces
│   │   ├── manager.go        # Transcription management
│   │   ├── models.go         # Transcription data models
│   │   ├── openai.go         # OpenAI API integration
│   │   ├── post_processor.go # LLM-based post-processing
│   │   └── processor.go      # Transcription processing
│   ├── weather/              # Weather data integration
│   │   ├── cache.go          # Weather data caching
│   │   ├── client.go         # Weather API client
│   │   ├── models.go         # Weather data models
│   │   └── service.go        # Weather service implementation
│   └── websocket/            # WebSocket server
│       └── server.go         # WebSocket server implementation
├── assets/                   # Static assets and prompts
│   ├── airlines.json         # Airline database
│   ├── airports.json         # Airport database
│   ├── runways.json          # Runway database
│   ├── atc_chat_prompt.txt   # ATC chat AI prompt
│   ├── post_processing_prompt.txt # Post-processing prompt
│   ├── transcription_prompt.txt # Transcription prompt
│   └── voice_assistant_prompt.txt # Voice assistant prompt
├── configs/                  # Configuration files
│   └── config.toml           # Main configuration file
└── www/                      # Frontend web application
    ├── index.html            # Main HTML page
    ├── style.css             # CSS styles
    ├── app.js                # Main application logic
    ├── map-manager.js        # Map visualization
    ├── websocket-client.js   # WebSocket client
    ├── aircraft-animation.js # Aircraft animation engine
    ├── atc-chat.js           # ATC chat interface
    ├── audio-client.js       # Audio streaming client
    └── sounds/               # Audio assets
```

## Background Workers and Goroutines

Co-ATC uses several background workers and goroutines to handle concurrent operations efficiently:

### 1. WebSocket Server
- **Location**: `internal/websocket/server.go`
- **Purpose**: Manages real-time communication with clients
- **Workers**:
  - Main WebSocket loop: Handles client registration, unregistration, and message broadcasting
  - Per-client read goroutine: Detects client disconnections
  - Per-client write goroutine: Sends messages to connected clients

### 2. ADS-B Service
- **Location**: `internal/adsb/service.go`
- **Purpose**: Processes aircraft tracking data
- **Workers**:
  - fetchLoop: Periodically fetches and processes ADS-B data at configured intervals
  - Detects aircraft takeoffs and landings
  - Updates aircraft status (active, stale, signal_lost)
  - Broadcasts aircraft events via WebSocket

### 3. Frequencies Service
- **Location**: `internal/frequencies/service.go`
- **Purpose**: Manages radio frequency audio streams
- **Workers**:
  - Per-frequency StreamProcessor: Manages audio stream for each configured frequency
  - cleanupInactiveClients: Periodically checks and removes inactive clients (runs every 30 seconds)
  - Parallel shutdown: Uses goroutines to stop stream processors concurrently during shutdown

### 4. Audio Processing System
- **Location**: `internal/audio/central_processor.go`, `internal/audio/multireader.go`, `internal/audio/wavreader.go`
- **Purpose**: Handles audio stream processing from audio sources
- **Components**:
  - **FFmpeg Manager (CentralAudioProcessor)**:
    - processFFmpegOutput: Reads audio data from ffmpeg and writes to MultiReader
    - startMonitoring: Monitors ffmpeg process health (runs every 5 seconds)
    - Reconnection timer: Automatically restarts ffmpeg after failures with configured delay
  - **Stream Manager (MultiReader)**:
    - Circular buffer implementation for efficient audio data sharing
    - Manages multiple concurrent readers from a single audio source
    - Per-reader goroutines that wait for new data with condition variables
    - Handles backpressure from slow clients without affecting other clients
  - **WAV Header Generator (WAVReader)**:
    - Dynamically generates WAV headers for browser compatibility
    - Ensures proper audio format for web clients

### 5. Transcription System
- **Location**: `internal/transcription/processor.go`, `internal/transcription/manager.go`
- **Purpose**: Transcribes ATC communications in real-time
- **Workers**:
  - processAudio: Reads audio data, chunks it, and sends to OpenAI
  - processTranscriptions: Receives and processes transcription events from OpenAI
  - Handles reconnection to OpenAI services when connections fail

### 6. Post-Processing
- **Location**: `internal/transcription/post_processor.go`
- **Purpose**: Enhances raw transcriptions with LLM processing
- **Workers**:
  - Background processing loop: Periodically processes batches of unprocessed transcriptions
  - Uses OpenAI to identify speakers, clean up content, and extract callsigns
  - Includes active aircraft data from the database as context for better processing
  - Broadcasts processed transcriptions via WebSocket

### 7. HTTP Servers
- **Location**: `cmd/server/main.go`
- **Purpose**: Serves API endpoints and static content
- **Workers**:
  - Multiple HTTP servers: One goroutine per configured port
  - Parallel shutdown: Uses goroutines to shut down HTTP servers concurrently with timeout

### 8. Graceful Shutdown
- **Location**: `cmd/server/main.go`
- **Purpose**: Ensures clean application termination
- **Process**:
  1. Captures interrupt signals (SIGINT, SIGTERM)
  2. Stops background services in order: frequencies → transcription → ADS-B
  3. Cancels main context to signal all goroutines to stop
  4. Shuts down HTTP servers with timeout
  5. Closes database connections

## Key Technical Components

### 1. Audio Processing System
- `audio/central_processor.go`: Manages a single ffmpeg process per frequency
- `audio/multireader.go`: Allows multiple clients to read from the same audio stream
- `audio/wavreader.go`: Handles WAV header generation for browser compatibility
- `audio/chunker.go`: Handles chunking of audio data for transcription

### 2. Transcription System
- `transcription/processor.go`: Processes audio for transcription
- `transcription/openai.go`: Integrates with OpenAI's Realtime Transcription API
- `transcription/manager.go`: Manages transcription processors for frequencies

### 3. Frequency Management
- `frequencies/service.go`: Manages audio streams for different frequencies
- `frequencies/client.go`: Handles connections to audio sources

### 4. ADS-B Data Processing
- `adsb/service.go`: Manages ADS-B data processing and processes raw data
- `adsb/atc_utils.go`: Provides aviation utilities and calculations
- `adsb/external.go`: Handles external ADS-B API integration
- `adsb/models.go`: Defines optimized data models with deduplication

### 5. API and WebSocket
- `api/routes.go`: Defines API endpoints
- `websocket/server.go`: Implements WebSocket server for real-time updates

### 6. Error Handling System
- Robust error handling throughout the application for better reliability
- WebSocket reconnection with exponential backoff
- API request retries with configurable parameters
- Graceful degradation when external services are unavailable
- Structured error reporting in API responses

## Database Schema

### Aircraft Table
- Stores aircraft position and telemetry data
- Optimized with composite indexes for performance
- Supports both real and simulated aircraft (`type` field)

### ADS-B Targets Table
- Raw ADS-B data storage with deduplication
- Composite index on (aircraft_hex, timestamp DESC)
- Stores position, altitude, speed, and heading data

### Phase Changes Table
- Tracks flight phase transitions (NEW, TAX, T/O, DEP, CRZ, ARR, APP, T/D)
- Anti-flapping logic prevents rapid phase transitions
- Configurable detection parameters

### Transcriptions Table
- Stores raw and processed transcription data
- Links to frequency information
- Supports post-processing workflow

### Clearances Table
- Stores extracted ATC clearances
- Links to transcription source
- Supports takeoff, landing, and approach clearances

## WebSocket Communication

### Message Types
- `aircraft_added`: New aircraft detected
- `aircraft_update`: Aircraft data changed
- `aircraft_removed`: Aircraft no longer tracked
- `aircraft_bulk_data`: Initial data load
- `phase_change`: Flight phase transition
- `clearance_issued`: ATC clearance extracted
- `filter_update`: Client filter preferences

### Client-Side Filtering
- Server-side filtering based on client preferences
- Reduces bandwidth by only sending relevant updates
- Supports Air/Ground scope, phase filters, and altitude ranges

## AI Integration

### ATC Chat Assistant
- OpenAI Realtime API integration
- Voice-based interaction with push-to-talk
- Real-time airspace context updates
- Templated system prompts with live data

### Post-Processing
- LLM-based transcription enhancement
- Speaker identification and callsign extraction
- ATC clearance detection and classification

### Templating System
- Unified data formatting for AI interactions
- Real-time aircraft, weather, and runway data
- Consistent context across all AI services

## Performance Optimizations

### WebSocket Optimizations
- Server-side filtering reduces client bandwidth
- Intelligent change detection prevents unnecessary updates
- Viewport culling for map rendering
- Batched message processing

### Database Optimizations
- Composite indexes for query performance
- Batch operations for phase data retrieval
- Connection pooling and prepared statements

### Frontend Optimizations
- Aircraft animation with vector extrapolation
- Smooth position interpolation
- Adaptive performance monitoring
- Memory management for large datasets

## Configuration System

The application uses TOML configuration with comprehensive documentation:
- Server settings (ports, timeouts, static files)
- ADS-B data sources and processing parameters
- Audio streaming configuration
- AI service integration settings
- Database and storage options
- Weather data integration
- Flight phase detection parameters

## Security Features

### Static File Serving
- Directory traversal protection
- File validation and security checks
- Configurable serving directory

### API Security
- Input validation and sanitization
- Structured error responses
- Rate limiting capabilities

### WebSocket Security
- Connection validation
- Message type verification
- Client state management

This architecture allows Co-ATC to handle multiple concurrent operations efficiently, including real-time data processing, audio streaming, and client communications, while maintaining clean shutdown procedures to prevent resource leaks.