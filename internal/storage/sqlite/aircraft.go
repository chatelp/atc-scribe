package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/pkg/logger"
	_ "modernc.org/sqlite"
)

// AircraftRecord represents an aircraft record for context
type AircraftRecord struct {
	Callsign     string
	Altitude     int
	TrueAirspeed int
}

// AircraftStorage is a SQLite-based storage for aircraft data
type AircraftStorage struct {
	db                *sql.DB
	logger            *logger.Logger
	maxPositionsInAPI int
}

// NewAircraftStorage creates a new SQLite-based aircraft storage
func NewAircraftStorage(dbPath string, maxPositionsInAPI int, log *logger.Logger) (*AircraftStorage, error) {
	storageLogger := log.Named("sqlite")

	storageLogger.Info("Initializing SQLite storage",
		logger.String("path", dbPath))

	// Open the database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool limits
	db.SetMaxOpenConns(1) // SQLite only supports one writer at a time
	db.SetMaxIdleConns(1)

	// Set pragmas for better performance and concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to set journal mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA cache_size=10000"); err != nil {
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	// Create tables if they don't exist
	if err := initDatabase(db, storageLogger); err != nil {
		db.Close()
		return nil, err
	}

	storage := &AircraftStorage{
		db:                db,
		logger:            storageLogger,
		maxPositionsInAPI: maxPositionsInAPI,
	}

	return storage, nil
}

// Close closes the database connection
func (s *AircraftStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetDB returns the database connection
func (s *AircraftStorage) GetDB() *sql.DB {
	return s.db
}

// initDatabase initializes the database schema
func initDatabase(db *sql.DB, log *logger.Logger) error {
	log.Info("Initializing database schema")

	// Create aircraft table with essential fields
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS aircraft (
			hex TEXT PRIMARY KEY,
			flight TEXT,
			airline TEXT,
			status TEXT,
			last_seen TIMESTAMP,
			on_ground INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create aircraft table: %w", err)
	}

	// Create adsb_targets table with all possible fields from both local and external APIs
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS adsb_targets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			aircraft_hex TEXT,
			hex TEXT,
			type TEXT,
			flight TEXT,
			registration TEXT,      -- External API specific field (r)
			aircraft_type TEXT,     -- External API specific field (t)
			alt_baro REAL,
			alt_geom REAL,
			gs REAL,
			ias REAL,
			tas REAL,
			mach REAL,
			wd REAL,
			ws REAL,
			oat REAL,
			tat REAL,
			track REAL,
			track_rate REAL,
			roll REAL,
			mag_heading REAL,
			true_heading REAL,
			baro_rate REAL,
			geom_rate REAL,
			squawk TEXT,
			emergency TEXT,
			category TEXT,
			nav_qnh REAL,
			nav_altitude_mcp REAL,
			nav_altitude_fms REAL,
			nav_heading REAL,
			nav_modes TEXT,
			lat REAL,
			lon REAL,
			nic INTEGER,
			rc INTEGER,
			seen_pos REAL,
			r_dst REAL,
			r_dir REAL,
			version INTEGER,
			nic_baro INTEGER,
			nac_p INTEGER,
			nac_v INTEGER,
			sil INTEGER,
			sil_type TEXT,
			gva INTEGER,
			sda INTEGER,
			alert INTEGER,
			spi INTEGER,
			mlat TEXT,
			tisb TEXT,
			messages INTEGER,
			seen REAL,
			rssi REAL,
			timestamp TIMESTAMP,
			raw_data TEXT,
			source_type TEXT,       -- Indicates whether data came from "local" or "external" source
			FOREIGN KEY (aircraft_hex) REFERENCES aircraft(hex) ON DELETE CASCADE,
			UNIQUE(aircraft_hex, lat, lon, alt_baro, gs, tas, track)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create adsb_targets table: %w", err)
	}

	// Create phase_changes table for tracking flight phase transitions
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS phase_changes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hex TEXT NOT NULL,
			flight TEXT,
			phase TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			adsb_id INTEGER,
			FOREIGN KEY (adsb_id) REFERENCES adsb_targets(id),
			FOREIGN KEY (hex) REFERENCES aircraft(hex) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create phase_changes table: %w", err)
	}

	// Create indexes for efficient querying
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_adsb_targets_aircraft_hex ON adsb_targets(aircraft_hex)`)
	if err != nil {
		return fmt.Errorf("failed to create index on adsb_targets.aircraft_hex: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_adsb_targets_timestamp ON adsb_targets(timestamp)`)
	if err != nil {
		return fmt.Errorf("failed to create index on adsb_targets.timestamp: %w", err)
	}

	// Critical composite index for efficient latest record queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_adsb_targets_hex_timestamp ON adsb_targets(aircraft_hex, timestamp DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create index on adsb_targets.aircraft_hex_timestamp: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_aircraft_status ON aircraft(status)`)
	if err != nil {
		return fmt.Errorf("failed to create index on aircraft.status: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_aircraft_last_seen ON aircraft(last_seen)`)
	if err != nil {
		return fmt.Errorf("failed to create index on aircraft.last_seen: %w", err)
	}

	// Create indexes for phase_changes table
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_phase_changes_hex_timestamp ON phase_changes(hex, timestamp)`)
	if err != nil {
		return fmt.Errorf("failed to create index on phase_changes.hex_timestamp: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_phase_changes_phase_timestamp ON phase_changes(phase, timestamp)`)
	if err != nil {
		return fmt.Errorf("failed to create index on phase_changes.phase_timestamp: %w", err)
	}

	log.Info("Database schema initialized successfully")
	return nil
}

// GetAll returns all aircraft
func (s *AircraftStorage) GetAll() []*adsb.Aircraft {
	aircraft, err := s.getAllAircraft()
	if err != nil {
		s.logger.Error("Failed to get all aircraft", logger.Error(err))
		return []*adsb.Aircraft{}
	}

	return aircraft
}

// getAllAircraft retrieves all aircraft from the database
func (s *AircraftStorage) getAllAircraft() ([]*adsb.Aircraft, error) {
	start := time.Now()
	s.logger.Debug("Starting getAllAircraft query")

	// Query all aircraft
	rows, err := s.db.Query(`
		SELECT hex, flight, airline, status, last_seen,
		on_ground, created_at
		FROM aircraft
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query aircraft: %w", err)
	}
	defer rows.Close()

	// Map to store aircraft by hex
	aircraftMap := make(map[string]*adsb.Aircraft)

	// Process aircraft rows
	for rows.Next() {
		var a adsb.Aircraft
		var lastSeen, createdAt string
		var onGround int

		if err := rows.Scan(
			&a.Hex, &a.Flight, &a.Airline, &a.Status, &lastSeen,
			&onGround, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan aircraft row: %w", err)
		}

		// Convert integer to boolean
		a.OnGround = onGround != 0

		// Parse last_seen timestamp
		t, err := time.Parse(time.RFC3339, lastSeen)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last_seen timestamp: %w", err)
		}
		a.LastSeen = t

		// Parse created_at timestamp
		createdTime, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at timestamp: %w", err)
		}
		a.CreatedAt = createdTime

		// Initialize empty history and future slices (not populated in main aircraft endpoint)
		a.History = []adsb.PositionMinimal{}
		a.Future = []adsb.Position{}

		// Add to map
		aircraftMap[a.Hex] = &a
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating aircraft rows: %w", err)
	}

	// If no aircraft found, return empty slice
	if len(aircraftMap) == 0 {
		return []*adsb.Aircraft{}, nil
	}

	// For each aircraft, get the latest ADSB data (fast individual queries)
	adsbStart := time.Now()
	s.logger.Debug("Starting ADSB data population", logger.Int("aircraft_count", len(aircraftMap)))

	for hex, aircraft := range aircraftMap {
		// Get the latest ADSB data
		adsbData, err := s.getLatestADSBData(hex)
		if err == nil && adsbData != nil {
			aircraft.ADSB = adsbData
		}

		// History and future data are not populated in the main aircraft endpoint
		// Use the combined /aircraft/{hex}/tracks endpoint instead
	}

	adsbDuration := time.Since(adsbStart)
	s.logger.Debug("ADSB data population completed", logger.Duration("duration", adsbDuration))

	// Get current phases for all aircraft in a single batch query
	phaseStart := time.Now()
	s.logger.Debug("Starting phase data population", logger.Int("aircraft_count", len(aircraftMap)))

	hexCodes := make([]string, 0, len(aircraftMap))
	for hex := range aircraftMap {
		hexCodes = append(hexCodes, hex)
	}

	currentPhases, err := s.GetCurrentPhasesBatch(hexCodes)
	if err != nil {
		s.logger.Error("Failed to get current phases batch", logger.Error(err))
	} else {
		// Get recent phase history for all aircraft in batch (last 5 changes per aircraft)
		recentHistory, err := s.getRecentPhaseHistoryBatch(hexCodes, 5)
		if err != nil {
			s.logger.Error("Failed to get recent phase history batch", logger.Error(err))
			recentHistory = make(map[string][]adsb.PhaseChange) // Empty fallback
		}

		// Assign current phases and recent history to aircraft
		for hex, aircraft := range aircraftMap {
			if phase, exists := currentPhases[hex]; exists {
				history := recentHistory[hex] // Will be empty slice if not found
				aircraft.Phase = &adsb.PhaseData{
					Current: []adsb.PhaseChange{*phase},
					History: history,
				}
			}
		}
	}

	phaseDuration := time.Since(phaseStart)
	s.logger.Debug("Phase data population completed", logger.Duration("duration", phaseDuration))

	// Populate DateLanded and DateTookoff fields from phase_changes table
	dateStart := time.Now()
	s.logger.Debug("Starting date_landed/date_tookoff population", logger.Int("aircraft_count", len(aircraftMap)))

	for hex, aircraft := range aircraftMap {
		// Get latest takeoff time
		takeoffTime, err := s.GetLatestTakeoffTime(hex)
		if err != nil {
			s.logger.Error("Failed to get latest takeoff time", logger.Error(err), logger.String("hex", hex))
		} else {
			aircraft.DateTookoff = takeoffTime
		}

		// Get latest landing time
		landingTime, err := s.GetLatestLandingTime(hex)
		if err != nil {
			s.logger.Error("Failed to get latest landing time", logger.Error(err), logger.String("hex", hex))
		} else {
			aircraft.DateLanded = landingTime
		}
	}

	dateDuration := time.Since(dateStart)
	s.logger.Debug("Date population completed", logger.Duration("duration", dateDuration))

	// Convert map to slice
	aircraft := make([]*adsb.Aircraft, 0, len(aircraftMap))
	for _, a := range aircraftMap {
		aircraft = append(aircraft, a)
	}

	totalDuration := time.Since(start)
	s.logger.Debug("getAllAircraft completed",
		logger.Duration("total_duration", totalDuration),
		logger.Int("aircraft_count", len(aircraft)))

	return aircraft, nil
}

// getLatestADSBData returns the latest ADSB data for an aircraft
func (s *AircraftStorage) getLatestADSBData(hex string) (*adsb.ADSBTarget, error) {
	row := s.db.QueryRow(`
		SELECT raw_data, source_type, registration, aircraft_type FROM adsb_targets
		WHERE aircraft_hex = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`, hex)

	var rawDataJSON, sourceType, registration, aircraftType string
	if err := row.Scan(&rawDataJSON, &sourceType, &registration, &aircraftType); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var rawData adsb.ADSBTarget
	if err := json.Unmarshal([]byte(rawDataJSON), &rawData); err != nil {
		return nil, err
	}

	// Set the source type, registration, and aircraft type fields
	rawData.SourceType = sourceType
	rawData.Registration = registration
	rawData.AircraftType = aircraftType

	return &rawData, nil
}

// getLatestADSBDataBatch returns the latest ADSB data for multiple aircraft in a single query
func (s *AircraftStorage) getLatestADSBDataBatch(hexCodes []string) (map[string]*adsb.ADSBTarget, error) {
	start := time.Now()
	s.logger.Info("Starting batch ADSB query", logger.Int("hex_count", len(hexCodes)))

	if len(hexCodes) == 0 {
		return make(map[string]*adsb.ADSBTarget), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes))
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}

	// Simple and fast query - just get latest timestamp for each aircraft
	query := fmt.Sprintf(`
		SELECT
			aircraft_hex,
			raw_data,
			source_type,
			registration,
			aircraft_type
		FROM adsb_targets a1
		WHERE aircraft_hex IN (%s)
		AND timestamp = (
			SELECT MAX(timestamp)
			FROM adsb_targets a2
			WHERE a2.aircraft_hex = a1.aircraft_hex
		)
	`, strings.Join(placeholders, ","))

	allArgs := args

	rows, err := s.db.Query(query, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest ADSB data batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*adsb.ADSBTarget)

	for rows.Next() {
		var hex, rawDataJSON, sourceType, registration, aircraftType string
		if err := rows.Scan(&hex, &rawDataJSON, &sourceType, &registration, &aircraftType); err != nil {
			return nil, fmt.Errorf("failed to scan ADSB data row: %w", err)
		}

		var rawData adsb.ADSBTarget
		if err := json.Unmarshal([]byte(rawDataJSON), &rawData); err != nil {
			s.logger.Error("Failed to unmarshal ADSB data", logger.Error(err), logger.String("hex", hex))
			continue
		}

		// Set the source type, registration, and aircraft type fields
		rawData.SourceType = sourceType
		rawData.Registration = registration
		rawData.AircraftType = aircraftType

		result[hex] = &rawData
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ADSB data rows: %w", err)
	}

	duration := time.Since(start)
	s.logger.Debug("Batch ADSB query completed",
		logger.Duration("duration", duration),
		logger.Int("requested_count", len(hexCodes)),
		logger.Int("returned_count", len(result)))

	return result, nil
}

// getRecentPhaseHistoryBatch returns recent phase history for multiple aircraft in a single query
func (s *AircraftStorage) getRecentPhaseHistoryBatch(hexCodes []string, limit int) (map[string][]adsb.PhaseChange, error) {
	if len(hexCodes) == 0 {
		return make(map[string][]adsb.PhaseChange), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes)+1)
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}
	args[len(hexCodes)] = limit // Add limit as last parameter

	// Query to get recent phase history for each aircraft
	query := fmt.Sprintf(`
		SELECT hex, id, phase, timestamp, adsb_id
		FROM (
			SELECT
				hex, id, phase, timestamp, adsb_id,
				ROW_NUMBER() OVER (PARTITION BY hex ORDER BY timestamp DESC) as rn
			FROM phase_changes
			WHERE hex IN (%s)
		) ranked
		WHERE rn <= ?
		ORDER BY hex, timestamp DESC
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent phase history batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]adsb.PhaseChange)

	for rows.Next() {
		var hex, phase, timestampStr string
		var id int
		var adsbId sql.NullInt64

		if err := rows.Scan(&hex, &id, &phase, &timestampStr, &adsbId); err != nil {
			return nil, fmt.Errorf("failed to scan phase history row: %w", err)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			s.logger.Error("Failed to parse phase timestamp", logger.Error(err), logger.String("hex", hex))
			continue
		}

		phaseChange := adsb.PhaseChange{
			ID:        id,
			Phase:     phase,
			Timestamp: timestamp,
		}

		if adsbId.Valid {
			adsbIdInt := int(adsbId.Int64)
			phaseChange.ADSBId = &adsbIdInt
		}

		result[hex] = append(result[hex], phaseChange)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating phase history rows: %w", err)
	}

	return result, nil
}

// getPositionHistoryMinimal returns minimal position history (lat, lon, alt_baro, timestamp) for map trails
func (s *AircraftStorage) getPositionHistoryMinimal(hex string, maxPositions int) ([]adsb.PositionMinimal, error) {
	//s.logger.Debug("Getting minimal position history",
	//	logger.String("hex", hex),
	//	logger.Int("maxPositions", maxPositions))

	rows, err := s.db.Query(`
		SELECT lat, lon, alt_baro, timestamp
		FROM adsb_targets
		WHERE aircraft_hex = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, hex, maxPositions)

	if err != nil {
		s.logger.Error("Error querying position history", logger.Error(err), logger.String("hex", hex))
		return nil, err
	}
	defer rows.Close()

	positions := []adsb.PositionMinimal{}
	for rows.Next() {
		var pos adsb.PositionMinimal
		var timestamp string

		if err := rows.Scan(&pos.Lat, &pos.Lon, &pos.AltBaro, &timestamp); err != nil {
			s.logger.Error("Error scanning position row", logger.Error(err), logger.String("hex", hex))
			return nil, err
		}

		t, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			s.logger.Error("Error parsing timestamp", logger.Error(err), logger.String("hex", hex))
			return nil, err
		}
		pos.Timestamp = t

		positions = append(positions, pos)
	}

	// Reverse the order to be chronological
	for i, j := 0, len(positions)-1; i < j; i, j = i+1, j-1 {
		positions[i], positions[j] = positions[j], positions[i]
	}

	return positions, nil
}

// getPositionHistory returns the full position history for an aircraft
func (s *AircraftStorage) getPositionHistory(hex string, maxPositions int) ([]adsb.Position, error) {
	// Use the configured maxPositions parameter
	rows, err := s.db.Query(`
		SELECT id, lat, lon, alt_baro, gs, tas, track, timestamp, registration, aircraft_type, source_type
		FROM adsb_targets
		WHERE aircraft_hex = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, hex, maxPositions)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := []adsb.Position{}
	for rows.Next() {
		var pos adsb.Position
		var id int
		var timestamp, registration, aircraftType, sourceType string

		if err := rows.Scan(&id, &pos.Lat, &pos.Lon, &pos.Altitude, &pos.SpeedGS, &pos.SpeedTrue, &pos.TrueHeading, &timestamp,
			&registration, &aircraftType, &sourceType); err != nil {
			return nil, err
		}

		// Set the ID field
		pos.ID = &id

		t, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, err
		}
		pos.Timestamp = t

		// Add metadata to position
		metadata := make(map[string]string)
		if registration != "" {
			metadata["registration"] = registration
		}
		if aircraftType != "" {
			metadata["aircraft_type"] = aircraftType
		}
		if sourceType != "" {
			metadata["source_type"] = sourceType
		}

		// Log external data for debugging
		if sourceType == "external" && (registration != "" || aircraftType != "") {
			s.logger.Debug("Position with external data",
				logger.String("hex", hex),
				logger.String("registration", registration),
				logger.String("aircraft_type", aircraftType),
				logger.Time("timestamp", t))
		}

		positions = append(positions, pos)
	}

	// Reverse the order to be chronological
	for i, j := 0, len(positions)-1; i < j; i, j = i+1, j-1 {
		positions[i], positions[j] = positions[j], positions[i]
	}

	return positions, nil
}

// GetAllPositionHistory returns position history for an aircraft from the last 1 hour in descending order by timestamp
func (s *AircraftStorage) GetAllPositionHistory(hex string) ([]adsb.Position, error) {
	// Calculate 1 hour ago timestamp in RFC3339 format (same format used when storing)
	oneHourAgo := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)

	// Query positions for the aircraft from the last 1 hour, ordered by timestamp descending (newest first)
	rows, err := s.db.Query(`
		SELECT id, lat, lon, alt_baro, gs, tas, true_heading, mag_heading, baro_rate, timestamp, registration, aircraft_type, source_type
		FROM adsb_targets
		WHERE aircraft_hex = ? AND timestamp >= ?
		ORDER BY timestamp DESC
	`, hex, oneHourAgo)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := []adsb.Position{}
	for rows.Next() {
		var pos adsb.Position
		var id int
		var timestamp, registration, aircraftType, sourceType string

		if err := rows.Scan(&id, &pos.Lat, &pos.Lon, &pos.Altitude, &pos.SpeedGS, &pos.SpeedTrue, &pos.TrueHeading, &pos.MagHeading, &pos.VerticalSpeed, &timestamp,
			&registration, &aircraftType, &sourceType); err != nil {
			return nil, err
		}

		// Set the ID field
		pos.ID = &id

		t, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, err
		}
		pos.Timestamp = t

		// Add metadata to position
		metadata := make(map[string]string)
		if registration != "" {
			metadata["registration"] = registration
		}
		if aircraftType != "" {
			metadata["aircraft_type"] = aircraftType
		}
		if sourceType != "" {
			metadata["source_type"] = sourceType
		}

		// Log external data for debugging
		if sourceType == "external" && (registration != "" || aircraftType != "") {
			s.logger.Debug("Position with external data",
				logger.String("hex", hex),
				logger.String("registration", registration),
				logger.String("aircraft_type", aircraftType),
				logger.Time("timestamp", t))
		}

		positions = append(positions, pos)
	}

	return positions, nil
}

// GetPositionHistoryWithLimit returns position history for an aircraft with a specified limit in descending order by timestamp
func (s *AircraftStorage) GetPositionHistoryWithLimit(hex string, limit int) ([]adsb.Position, error) {
	// Calculate 1 hour ago timestamp in RFC3339 format (same format used when storing)
	oneHourAgo := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)

	// Query positions for the aircraft from the last 1 hour, ordered by timestamp descending (newest first) with limit
	rows, err := s.db.Query(`
		SELECT id, lat, lon, alt_baro, gs, tas, true_heading, mag_heading, baro_rate, timestamp, registration, aircraft_type, source_type
		FROM adsb_targets
		WHERE aircraft_hex = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, hex, oneHourAgo, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := []adsb.Position{}
	for rows.Next() {
		var pos adsb.Position
		var id int
		var timestamp, registration, aircraftType, sourceType string

		if err := rows.Scan(&id, &pos.Lat, &pos.Lon, &pos.Altitude, &pos.SpeedGS, &pos.SpeedTrue, &pos.TrueHeading, &pos.MagHeading, &pos.VerticalSpeed, &timestamp,
			&registration, &aircraftType, &sourceType); err != nil {
			return nil, err
		}

		// Set the ID field
		pos.ID = &id

		t, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, err
		}
		pos.Timestamp = t

		// Note: Position struct doesn't have metadata field, so we skip metadata for now
		_ = registration // Avoid unused variable warnings
		_ = aircraftType
		_ = sourceType

		positions = append(positions, pos)
	}

	return positions, nil
}

// GetByHex returns an aircraft by its hex ID
func (s *AircraftStorage) GetByHex(hex string) (*adsb.Aircraft, bool) {

	// Query aircraft
	row := s.db.QueryRow(`
		SELECT hex, flight, airline, status, last_seen, on_ground
		FROM aircraft
		WHERE hex = ?
	`, hex)

	var a adsb.Aircraft
	var lastSeen string
	var onGround int

	if err := row.Scan(
		&a.Hex, &a.Flight, &a.Airline, &a.Status, &lastSeen, &onGround,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		s.logger.Error("Failed to scan aircraft row", logger.Error(err), logger.String("hex", hex))
		return nil, false
	}

	// Parse last_seen timestamp
	t, err := time.Parse(time.RFC3339, lastSeen)
	if err != nil {
		s.logger.Error("Failed to parse last_seen timestamp", logger.Error(err), logger.String("hex", hex))
		return nil, false
	}
	a.LastSeen = t

	// Convert integer to boolean
	a.OnGround = onGround != 0

	// Get the latest ADSB data
	adsbData, err := s.getLatestADSBData(hex)
	if err == nil && adsbData != nil {
		a.ADSB = adsbData
	}

	// Get minimal position history for map trails
	minimalPositions, err := s.getPositionHistoryMinimal(hex, s.maxPositionsInAPI)
	if err == nil {
		a.History = minimalPositions
	} else {
		a.History = []adsb.PositionMinimal{}
		s.logger.Error("Failed to get position history", logger.Error(err), logger.String("hex", hex))
	}

	// Calculate future positions if we have the necessary data
	if a.ADSB != nil && a.ADSB.Lat != 0 && a.ADSB.Lon != 0 && a.ADSB.AltBaro != 0 {
		// Get heading (use true_heading, track, or mag_heading, whichever is available)
		heading := a.ADSB.TrueHeading
		if heading == 0 {
			heading = a.ADSB.Track
		}
		if heading == 0 {
			heading = a.ADSB.MagHeading
		}

		// Get speed (use TAS or GS, whichever is available)
		speed := a.ADSB.TAS
		if speed == 0 {
			speed = a.ADSB.GS
		}

		// Get vertical rate (use baro_rate or geom_rate, whichever is available)
		verticalRate := a.ADSB.BaroRate
		if verticalRate == 0 {
			verticalRate = a.ADSB.GeomRate
		}

		// Only predict if we have valid heading and speed
		if heading != 0 && speed != 0 {
			// Calculate future positions
			// Get magnetic heading for predictions
			magHeading := a.ADSB.MagHeading
			if magHeading == 0 {
				magHeading = heading // fallback to whatever heading we found
			}

			a.Future = adsb.PredictFuturePositions(
				a.ADSB.Lat,
				a.ADSB.Lon,
				a.ADSB.AltBaro,
				heading,    // true heading
				magHeading, // magnetic heading
				speed,
				verticalRate,
			)
		} else {
			// Initialize empty future slice
			a.Future = []adsb.Position{}
		}
	} else {
		// Initialize empty future slice
		a.Future = []adsb.Position{}
	}

	// Populate phase data for this aircraft
	if err := s.populatePhaseData(&a); err != nil {
		s.logger.Error("Failed to populate phase data", logger.Error(err), logger.String("hex", hex))
		// Continue returning the aircraft even if phase data fails
	}

	// Populate DateLanded and DateTookoff fields from phase_changes table
	takeoffTime, err := s.GetLatestTakeoffTime(hex)
	if err != nil {
		s.logger.Error("Failed to get latest takeoff time", logger.Error(err), logger.String("hex", hex))
	} else {
		a.DateTookoff = takeoffTime
	}

	landingTime, err := s.GetLatestLandingTime(hex)
	if err != nil {
		s.logger.Error("Failed to get latest landing time", logger.Error(err), logger.String("hex", hex))
	} else {
		a.DateLanded = landingTime
	}

	return &a, true
}

// Upsert updates or inserts an aircraft
func (s *AircraftStorage) Upsert(aircraft *adsb.Aircraft) {
	// Ensure all timestamps are in UTC
	aircraft.LastSeen = aircraft.LastSeen.UTC()

	// Try to begin a transaction with retries
	var tx *sql.Tx
	var err error

	// Retry up to 3 times with exponential backoff
	for i := 0; i < 3; i++ {
		tx, err = s.db.Begin()
		if err == nil {
			break
		}

		s.logger.Warn("Failed to begin transaction, retrying...",
			logger.Error(err),
			logger.String("hex", aircraft.Hex),
			logger.Int("attempt", i+1))

		// Exponential backoff: 100ms, 200ms, 400ms
		time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
	}

	if err != nil {
		s.logger.Error("Failed to begin transaction after retries", logger.Error(err), logger.String("hex", aircraft.Hex))
		return
	}
	if err != nil {
		s.logger.Error("Failed to begin transaction", logger.Error(err))
		return
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				s.logger.Error("Failed to rollback transaction", logger.Error(rollbackErr))
			} else {
				s.logger.Error("Transaction rolled back", logger.Error(err))
			}
		}
	}()

	// Check if aircraft already exists and get current status
	var exists bool
	var currentStatus string
	err = tx.QueryRow("SELECT 1, status FROM aircraft WHERE hex = ?", aircraft.Hex).Scan(&exists, &currentStatus)
	if err != nil && err != sql.ErrNoRows {
		s.logger.Error("Failed to check if aircraft exists", logger.Error(err), logger.String("hex", aircraft.Hex))
		return
	}

	// Set status to active for new data
	if aircraft.Status == "" {
		aircraft.Status = "active"
	}

	if err == sql.ErrNoRows {
		// Insert new aircraft with UTC timestamps
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`
			INSERT INTO aircraft (
				hex, flight, airline, status, last_seen,
				on_ground, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			aircraft.Hex, aircraft.Flight, aircraft.Airline, aircraft.Status,
			aircraft.LastSeen.Format(time.RFC3339),
			boolToInt(aircraft.OnGround),
			now, now,
		)
		if err != nil {
			s.logger.Error("Failed to insert aircraft", logger.Error(err), logger.String("hex", aircraft.Hex))
			return
		}
	} else {
		// Update existing aircraft with UTC timestamp for updated_at
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`
			UPDATE aircraft SET
				flight = ?, airline = ?, status = ?, last_seen = ?, on_ground = ?, updated_at = ?
			WHERE hex = ?
		`,
			aircraft.Flight, aircraft.Airline, aircraft.Status, aircraft.LastSeen.Format(time.RFC3339),
			boolToInt(aircraft.OnGround), now, aircraft.Hex,
		)
		if err != nil {
			s.logger.Error("Failed to update aircraft", logger.Error(err), logger.String("hex", aircraft.Hex))
			return
		}
	}

	// Check if this is a unique ADSB target
	isUnique, err := s.isUniqueADSBTarget(tx, aircraft)
	if err != nil {
		s.logger.Error("Failed to check for unique ADSB target", logger.Error(err), logger.String("hex", aircraft.Hex))
		return
	}

	if isUnique && aircraft.ADSB != nil {
		// Convert ADSB data to JSON
		rawData, err := json.Marshal(aircraft.ADSB)
		if err != nil {
			s.logger.Error("Failed to marshal ADSB data", logger.Error(err), logger.String("hex", aircraft.Hex))
			return
		}

		// Get source type and registration/aircraft type directly from the ADSB data
		sourceType := "local"
		registration := ""
		aircraftType := ""

		if aircraft.ADSB != nil {
			// Use the fields directly from the ADSBTarget struct
			if aircraft.ADSB.SourceType != "" {
				sourceType = aircraft.ADSB.SourceType
			}

			if aircraft.ADSB.Registration != "" {
				registration = aircraft.ADSB.Registration
			}

			if aircraft.ADSB.AircraftType != "" {
				aircraftType = aircraft.ADSB.AircraftType
			}

			//s.logger.Debug("ADSB data source info",
			//	logger.String("hex", aircraft.Hex),
			//	logger.String("source_type", sourceType),
			//	logger.String("registration", registration),
			//	logger.String("aircraft_type", aircraftType))
		}

		// Insert the ADSB target
		_, err = tx.Exec(`
			INSERT INTO adsb_targets (
				aircraft_hex, hex, type, flight, registration, aircraft_type, alt_baro, alt_geom, gs, ias, tas, mach, wd, ws, oat, tat,
				track, track_rate, roll, mag_heading, true_heading, baro_rate, geom_rate, squawk, emergency,
				category, nav_qnh, nav_altitude_mcp, nav_altitude_fms, nav_heading, nav_modes, lat, lon,
				nic, rc, seen_pos, r_dst, r_dir, version, nic_baro, nac_p, nac_v, sil, sil_type, gva, sda,
				alert, spi, mlat, tisb, messages, seen, rssi, timestamp, raw_data, source_type
			) VALUES (
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
			)
		`,
			aircraft.Hex, aircraft.ADSB.Hex, aircraft.ADSB.Type, aircraft.ADSB.Flight,
			registration, aircraftType, // Registration and AircraftType (populated for external API)
			aircraft.ADSB.AltBaro, aircraft.ADSB.AltGeom, aircraft.ADSB.GS, aircraft.ADSB.IAS,
			aircraft.ADSB.TAS, aircraft.ADSB.Mach, aircraft.ADSB.WD, aircraft.ADSB.WS,
			aircraft.ADSB.OAT, aircraft.ADSB.TAT, aircraft.ADSB.Track, aircraft.ADSB.TrackRate,
			aircraft.ADSB.Roll, aircraft.ADSB.MagHeading, aircraft.ADSB.TrueHeading,
			aircraft.ADSB.BaroRate, aircraft.ADSB.GeomRate, aircraft.ADSB.Squawk,
			"", aircraft.ADSB.Category, aircraft.ADSB.NavQNH,
			aircraft.ADSB.NavAltitudeMCP, aircraft.ADSB.NavAltitudeFMS, aircraft.ADSB.NavHeading,
			"", aircraft.ADSB.Lat, aircraft.ADSB.Lon,
			aircraft.ADSB.NIC, aircraft.ADSB.RC, aircraft.ADSB.SeenPos, aircraft.ADSB.RDst,
			aircraft.ADSB.RDir, aircraft.ADSB.Version, aircraft.ADSB.NICBaro, aircraft.ADSB.NACP,
			aircraft.ADSB.NACV, aircraft.ADSB.SIL, aircraft.ADSB.SILType, aircraft.ADSB.GVA,
			aircraft.ADSB.SDA, aircraft.ADSB.Alert, aircraft.ADSB.SPI,
			"", "", // MLAT and TISB as strings (we'll store them as empty strings for now)
			aircraft.ADSB.Messages, aircraft.ADSB.Seen, aircraft.ADSB.RSSI,
			aircraft.LastSeen.Format(time.RFC3339), string(rawData), sourceType,
		)
		if err != nil {
			s.logger.Error("Failed to insert ADSB target", logger.Error(err), logger.String("hex", aircraft.Hex))
			return
		}
	}

	// Commit transaction with retries
	for i := 0; i < 3; i++ {
		err = tx.Commit()
		if err == nil {
			break
		}

		s.logger.Warn("Failed to commit transaction, retrying...",
			logger.Error(err),
			logger.String("hex", aircraft.Hex),
			logger.Int("attempt", i+1))

		// Exponential backoff: 100ms, 200ms, 400ms
		time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
	}

	if err != nil {
		s.logger.Error("Failed to commit transaction after retries", logger.Error(err), logger.String("hex", aircraft.Hex))
	}
}

// isUniqueADSBTarget checks if the ADSB target represents a unique position/state
func (s *AircraftStorage) isUniqueADSBTarget(tx *sql.Tx, aircraft *adsb.Aircraft) (bool, error) {
	if aircraft.ADSB == nil {
		return false, nil
	}

	var count int
	err := tx.QueryRow(`
		SELECT COUNT(*) FROM adsb_targets
		WHERE aircraft_hex = ? AND lat = ? AND lon = ? AND alt_baro = ? AND gs = ? AND tas = ? AND track = ?
	`, aircraft.Hex, aircraft.ADSB.Lat, aircraft.ADSB.Lon, aircraft.ADSB.AltBaro, aircraft.ADSB.GS, aircraft.ADSB.TAS, aircraft.ADSB.Track).Scan(&count)

	if err != nil {
		s.logger.Error("Error checking for unique ADSB target",
			logger.Error(err),
			logger.String("hex", aircraft.Hex))
		return false, err
	}

	//s.logger.Debug("Checked for unique ADSB target",
	//	logger.String("hex", aircraft.Hex),
	//	logger.Int("count", count),
	//	logger.Bool("is_unique", count == 0))

	return count == 0, nil
}

// Count returns the number of aircraft in the database
func (s *AircraftStorage) Count() int {

	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM aircraft").Scan(&count)
	if err != nil {
		s.logger.Error("Failed to count aircraft", logger.Error(err))
		return 0
	}

	return count
}

// GetFiltered returns aircraft filtered by altitude, status, and date ranges
func (s *AircraftStorage) GetFiltered(
	minAltitude, maxAltitude float64,
	status []string,
	tookOffAfter, tookOffBefore, landedAfter, landedBefore *time.Time,
) []*adsb.Aircraft {

	// Build the query with placeholders
	query := `
		SELECT hex, flight, airline, status, last_seen, on_ground
		FROM aircraft
		WHERE 1=1`

	// Create a slice to hold query arguments
	args := []interface{}{}

	// Add status filter if provided
	if len(status) > 0 {
		query += " AND status IN (" + strings.Repeat("?,", len(status)-1) + "?)"
		for _, s := range status {
			args = append(args, s)
		}
	}

	// TODO: Add date filters using JOINs to phase_changes table for T/O and T/D phases
	// For now, we'll ignore the date filters since they need to be implemented with the new phase_changes table
	_ = tookOffAfter
	_ = tookOffBefore
	_ = landedAfter
	_ = landedBefore

	// Execute the query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		s.logger.Error("Failed to query filtered aircraft", logger.Error(err))
		return []*adsb.Aircraft{}
	}
	defer rows.Close()

	// Map to store aircraft by hex
	aircraftMap := make(map[string]*adsb.Aircraft)

	// Process aircraft rows
	for rows.Next() {
		var a adsb.Aircraft
		var lastSeen string
		var onGround int

		if err := rows.Scan(
			&a.Hex, &a.Flight, &a.Airline, &a.Status, &lastSeen, &onGround,
		); err != nil {
			s.logger.Error("Failed to scan aircraft row", logger.Error(err))
			continue
		}

		// Parse last_seen timestamp
		t, err := time.Parse(time.RFC3339, lastSeen)
		if err != nil {
			s.logger.Error("Failed to parse last_seen timestamp", logger.Error(err))
			continue
		}
		a.LastSeen = t

		// Convert integer to boolean
		a.OnGround = onGround != 0

		// Initialize empty history slice
		a.History = []adsb.PositionMinimal{}

		// Add to map
		aircraftMap[a.Hex] = &a
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating aircraft rows", logger.Error(err))
		return []*adsb.Aircraft{}
	}

	// If no aircraft found, return empty slice
	if len(aircraftMap) == 0 {
		return []*adsb.Aircraft{}
	}

	// For each aircraft, get the latest ADSB data and position history
	for hex, aircraft := range aircraftMap {
		// Get the latest ADSB data
		adsbData, err := s.getLatestADSBData(hex)
		if err == nil && adsbData != nil {
			aircraft.ADSB = adsbData
		}

		// History data is not populated in filtered aircraft endpoint
		// Use the combined /aircraft/{hex}/tracks endpoint instead

		// Populate phase data for this aircraft
		if err := s.populatePhaseData(aircraft); err != nil {
			s.logger.Error("Failed to populate phase data", logger.Error(err), logger.String("hex", hex))
			// Continue processing other aircraft even if phase data fails
		}
	}

	// Convert map to slice
	aircraft := make([]*adsb.Aircraft, 0, len(aircraftMap))
	for _, a := range aircraftMap {
		aircraft = append(aircraft, a)
	}

	return aircraft
}

// boolToInt converts a boolean to an integer (1 for true, 0 for false)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// formatNullableTime formats a nullable time.Time for SQL
func formatNullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

// marshalStringArray converts a string array to a JSON string for storage
func marshalStringArray(arr []string) string {
	if arr == nil || len(arr) == 0 {
		return ""
	}

	data, err := json.Marshal(arr)
	if err != nil {
		return ""
	}

	return string(data)
}

// GetActiveAircraft retrieves active aircraft data
func (s *AircraftStorage) GetActiveAircraft() ([]*AircraftRecord, error) {

	// Query active aircraft with their latest position data
	rows, err := s.db.Query(`
		SELECT a.flight, t.alt_baro, t.tas
		FROM aircraft a
		LEFT JOIN (
			SELECT aircraft_hex, alt_baro, tas,
				ROW_NUMBER() OVER (PARTITION BY aircraft_hex ORDER BY timestamp DESC) as rn
			FROM adsb_targets
		) t ON t.aircraft_hex = a.hex AND t.rn = 1
		WHERE a.status = 'active'
		ORDER BY a.flight ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query active aircraft: %w", err)
	}
	defer rows.Close()

	// Parse records
	var aircraft []*AircraftRecord
	for rows.Next() {
		var record AircraftRecord
		var callsign sql.NullString
		var altitude, trueAirspeed sql.NullFloat64

		if err := rows.Scan(&callsign, &altitude, &trueAirspeed); err != nil {
			return nil, fmt.Errorf("failed to scan aircraft: %w", err)
		}

		// Handle nullable fields
		if callsign.Valid {
			record.Callsign = callsign.String
		}
		if altitude.Valid {
			record.Altitude = int(altitude.Float64)
		}
		if trueAirspeed.Valid {
			record.TrueAirspeed = int(trueAirspeed.Float64)
		}

		aircraft = append(aircraft, &record)
	}

	return aircraft, nil
}

// InsertPhaseChange inserts a new phase change record
func (s *AircraftStorage) InsertPhaseChange(hex, flight, phase string, timestamp time.Time, adsbId *int) error {

	_, err := s.db.Exec(`
		INSERT INTO phase_changes (hex, flight, phase, timestamp, adsb_id)
		VALUES (?, ?, ?, ?, ?)
	`, hex, flight, phase, timestamp.Format(time.RFC3339), adsbId)

	if err != nil {
		s.logger.Error("Failed to insert phase change", logger.Error(err),
			logger.String("hex", hex), logger.String("phase", phase))
		return fmt.Errorf("failed to insert phase change: %w", err)
	}

	return nil
}

// GetPhaseHistory returns all phase changes for an aircraft in descending order by timestamp
func (s *AircraftStorage) GetPhaseHistory(hex string) ([]adsb.PhaseChange, error) {

	rows, err := s.db.Query(`
		SELECT id, phase, timestamp, adsb_id
		FROM phase_changes
		WHERE hex = ?
		ORDER BY timestamp DESC
	`, hex)
	if err != nil {
		return nil, fmt.Errorf("failed to query phase history: %w", err)
	}
	defer rows.Close()

	var phases []adsb.PhaseChange
	for rows.Next() {
		var phase adsb.PhaseChange
		var timestampStr string
		var adsbId sql.NullInt64

		if err := rows.Scan(&phase.ID, &phase.Phase, &timestampStr, &adsbId); err != nil {
			return nil, fmt.Errorf("failed to scan phase change row: %w", err)
		}

		// Debug logging to see what we're getting from the database
		//s.logger.Debug("Scanned phase change from database",
		//	logger.String("hex", hex),
		//	logger.Int("id", phase.ID),
		//	logger.String("phase", phase.Phase),
		//	logger.String("timestamp", timestampStr))

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}
		phase.Timestamp = timestamp

		// Handle nullable adsb_id
		if adsbId.Valid {
			id := int(adsbId.Int64)
			phase.ADSBId = &id
		}

		phases = append(phases, phase)
	}

	return phases, nil
}

// GetCurrentPhase returns the latest phase for an aircraft
func (s *AircraftStorage) GetCurrentPhase(hex string) (*adsb.PhaseChange, error) {

	row := s.db.QueryRow(`
		SELECT id, phase, timestamp, adsb_id
		FROM phase_changes
		WHERE hex = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`, hex)

	var phase adsb.PhaseChange
	var timestampStr string
	var adsbId sql.NullInt64

	if err := row.Scan(&phase.ID, &phase.Phase, &timestampStr, &adsbId); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No phase changes found
		}
		return nil, fmt.Errorf("failed to scan current phase: %w", err)
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	phase.Timestamp = timestamp

	// Handle nullable adsb_id
	if adsbId.Valid {
		id := int(adsbId.Int64)
		phase.ADSBId = &id
	}

	return &phase, nil
}

// GetLatestTakeoffTime returns the latest takeoff time for an aircraft from phase_changes
func (s *AircraftStorage) GetLatestTakeoffTime(hex string) (*time.Time, error) {

	row := s.db.QueryRow(`
		SELECT timestamp
		FROM phase_changes
		WHERE hex = ? AND phase = 'T/O'
		ORDER BY timestamp DESC
		LIMIT 1
	`, hex)

	var timestampStr string
	if err := row.Scan(&timestampStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No takeoff found
		}
		return nil, fmt.Errorf("failed to scan takeoff time: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse takeoff timestamp: %w", err)
	}

	return &timestamp, nil
}

// GetLatestLandingTime returns the latest landing time for an aircraft from phase_changes
func (s *AircraftStorage) GetLatestLandingTime(hex string) (*time.Time, error) {

	row := s.db.QueryRow(`
		SELECT timestamp
		FROM phase_changes
		WHERE hex = ? AND phase = 'T/D'
		ORDER BY timestamp DESC
		LIMIT 1
	`, hex)

	var timestampStr string
	if err := row.Scan(&timestampStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No landing found
		}
		return nil, fmt.Errorf("failed to scan landing time: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse landing timestamp: %w", err)
	}

	return &timestamp, nil
}

// populatePhaseData populates phase data for an aircraft from the phase_changes table
func (s *AircraftStorage) populatePhaseData(aircraft *adsb.Aircraft) error {
	start := time.Now()

	// Get phase history for this aircraft
	phaseHistory, err := s.GetPhaseHistory(aircraft.Hex)
	if err != nil {
		return fmt.Errorf("failed to get phase history: %w", err)
	}

	// Debug logging to see what we got from GetPhaseHistory
	//s.logger.Debug("Phase history retrieved",
	//	logger.String("hex", aircraft.Hex),
	//	logger.Int("count", len(phaseHistory)))

	// Create phase data structure
	phaseData := &adsb.PhaseData{
		Current: []adsb.PhaseChange{},
		History: phaseHistory,
	}

	// Set current phase (first item in history, or empty if no history)
	if len(phaseHistory) > 0 {
		phaseData.Current = []adsb.PhaseChange{phaseHistory[0]}
		s.logger.Debug("Set current phase",
			logger.String("hex", aircraft.Hex),
			logger.Int("current_id", phaseHistory[0].ID),
			logger.String("current_phase", phaseHistory[0].Phase))
	}

	aircraft.Phase = phaseData

	// Get takeoff and landing times from phase_changes table
	takeoffTime, err := s.GetLatestTakeoffTime(aircraft.Hex)
	if err != nil {
		s.logger.Error("Failed to get takeoff time", logger.Error(err), logger.String("hex", aircraft.Hex))
	} else {
		aircraft.DateTookoff = takeoffTime
	}

	landingTime, err := s.GetLatestLandingTime(aircraft.Hex)
	if err != nil {
		s.logger.Error("Failed to get landing time", logger.Error(err), logger.String("hex", aircraft.Hex))
	} else {
		aircraft.DateLanded = landingTime
	}

	duration := time.Since(start)
	if duration > 10*time.Millisecond {
		s.logger.Debug("Slow phase data population",
			logger.String("hex", aircraft.Hex),
			logger.Duration("duration", duration))
	}

	return nil
}

// GetLatestADSBTargetID returns the ID of the latest ADSB target record for an aircraft
func (s *AircraftStorage) GetLatestADSBTargetID(hex string) (*int, error) {

	row := s.db.QueryRow(`
		SELECT id
		FROM adsb_targets
		WHERE aircraft_hex = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`, hex)

	var id int
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No ADSB target found
		}
		return nil, fmt.Errorf("failed to scan ADSB target ID: %w", err)
	}

	return &id, nil
}

// GetCurrentPhasesBatch returns the current phases for multiple aircraft in a single query
func (s *AircraftStorage) GetCurrentPhasesBatch(hexCodes []string) (map[string]*adsb.PhaseChange, error) {
	if len(hexCodes) == 0 {
		return make(map[string]*adsb.PhaseChange), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes))
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}

	query := fmt.Sprintf(`
		WITH latest_phases AS (
			SELECT hex, id, phase, timestamp, adsb_id,
				   ROW_NUMBER() OVER (PARTITION BY hex ORDER BY timestamp DESC) as rn
			FROM phase_changes
			WHERE hex IN (%s)
		)
		SELECT hex, id, phase, timestamp, adsb_id
		FROM latest_phases
		WHERE rn = 1
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query current phases batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*adsb.PhaseChange)
	for rows.Next() {
		var hex, phase, timestampStr string
		var id int
		var adsbId *int

		if err := rows.Scan(&hex, &id, &phase, &timestampStr, &adsbId); err != nil {
			return nil, fmt.Errorf("failed to scan phase row: %w", err)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}

		result[hex] = &adsb.PhaseChange{
			ID:        id,
			Phase:     phase,
			Timestamp: timestamp,
			ADSBId:    adsbId,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating phase rows: %w", err)
	}

	return result, nil
}

// GetLatestADSBTargetIDsBatch returns the latest ADSB target IDs for multiple aircraft in a single query
func (s *AircraftStorage) GetLatestADSBTargetIDsBatch(hexCodes []string) (map[string]*int, error) {
	if len(hexCodes) == 0 {
		return make(map[string]*int), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes))
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}

	query := fmt.Sprintf(`
		WITH latest_targets AS (
			SELECT aircraft_hex, id,
				   ROW_NUMBER() OVER (PARTITION BY aircraft_hex ORDER BY timestamp DESC) as rn
			FROM adsb_targets
			WHERE aircraft_hex IN (%s)
		)
		SELECT aircraft_hex, id
		FROM latest_targets
		WHERE rn = 1
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest ADSB target IDs batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*int)
	for rows.Next() {
		var hex string
		var id int

		if err := rows.Scan(&hex, &id); err != nil {
			return nil, fmt.Errorf("failed to scan ADSB target ID row: %w", err)
		}

		result[hex] = &id
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ADSB target ID rows: %w", err)
	}

	return result, nil
}

// InsertPhaseChangesBatch inserts multiple phase changes in a single transaction
func (s *AircraftStorage) InsertPhaseChangesBatch(changes []adsb.PhaseChangeInsert) error {
	if len(changes) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare the insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO phase_changes (hex, flight, phase, timestamp, adsb_id)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare phase change insert statement: %w", err)
	}
	defer stmt.Close()

	// Insert all phase changes
	for _, change := range changes {
		_, err := stmt.Exec(
			change.Hex,
			change.Flight,
			change.Phase,
			change.Timestamp.Format(time.RFC3339),
			change.ADSBId,
		)
		if err != nil {
			return fmt.Errorf("failed to insert phase change for %s: %w", change.Hex, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit phase changes batch: %w", err)
	}

	s.logger.Debug("Inserted phase changes batch",
		logger.Int("count", len(changes)))

	return nil
}
