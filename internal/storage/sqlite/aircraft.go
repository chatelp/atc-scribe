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

	// Open the database with pragmas in the connection string so every pooled connection gets them
	connStr := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=cache_size(10000)",
		dbPath,
	)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Allow multiple concurrent readers; writes are serialized by package-level sqliteWriteMu
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

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

	// Index for phase + hex queries (used in takeoff/landing time lookups)
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_phase_changes_hex_phase_timestamp ON phase_changes(hex, phase, timestamp DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create index on phase_changes.hex_phase_timestamp: %w", err)
	}

	// Covering index for the uniqueness check query in isUniqueADSBTarget
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_adsb_targets_unique_check ON adsb_targets(aircraft_hex, lat, lon, alt_baro, gs, tas, track)`)
	if err != nil {
		return fmt.Errorf("failed to create index on adsb_targets unique check: %w", err)
	}

	log.Info("Database schema initialized successfully")
	return nil
}

// GetAll returns all aircraft
func (s *AircraftStorage) GetAll() []*adsb.Aircraft {
	aircraft, err := s.getAllAircraftInternal(0, false)
	if err != nil {
		s.logger.Error("Failed to get all aircraft", logger.Error(err))
		return []*adsb.Aircraft{}
	}

	return aircraft
}

// GetAllWithLastSeenFilter returns aircraft seen within the last N minutes
// This filters at the database level for much better performance on large databases
func (s *AircraftStorage) GetAllWithLastSeenFilter(lastSeenMinutes int) []*adsb.Aircraft {
	aircraft, err := s.getAllAircraftInternal(lastSeenMinutes, false)
	if err != nil {
		s.logger.Error("Failed to get aircraft with last seen filter", logger.Error(err))
		return []*adsb.Aircraft{}
	}

	return aircraft
}

// GetAllMinimal returns aircraft with minimal data (skips phase history and date queries)
// This is optimized for the simple=1 API endpoint
func (s *AircraftStorage) GetAllMinimal(lastSeenMinutes int) []*adsb.Aircraft {
	aircraft, err := s.getAllAircraftInternal(lastSeenMinutes, true)
	if err != nil {
		s.logger.Error("Failed to get aircraft minimal", logger.Error(err))
		return []*adsb.Aircraft{}
	}

	return aircraft
}

// getAllAircraftInternal retrieves aircraft from the database with optional filtering
// If lastSeenMinutes > 0, only returns aircraft seen within that time window
// If minimal is true, skips phase history and takeoff/landing time queries (faster for simple API)
func (s *AircraftStorage) getAllAircraftInternal(lastSeenMinutes int, minimal bool) ([]*adsb.Aircraft, error) {
	start := time.Now()
	s.logger.Debug("Starting getAllAircraftInternal query", logger.Int("lastSeenMinutes", lastSeenMinutes))

	// Build query with optional last_seen filter
	var rows *sql.Rows
	var err error

	if lastSeenMinutes > 0 {
		// Use parameterized cutoff time - this uses idx_aircraft_last_seen index
		cutoffTime := time.Now().UTC().Add(-time.Duration(lastSeenMinutes) * time.Minute)
		rows, err = s.db.Query(`
			SELECT hex, flight, airline, status, last_seen,
			on_ground, created_at
			FROM aircraft
			WHERE last_seen >= ?
		`, cutoffTime.Format(time.RFC3339))
	} else {
		// Query all aircraft (original behavior)
		rows, err = s.db.Query(`
			SELECT hex, flight, airline, status, last_seen,
			on_ground, created_at
			FROM aircraft
		`)
	}
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
		a.Hindcast = []adsb.Position{}

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

	// Build hex codes list once for all batch operations
	hexCodes := make([]string, 0, len(aircraftMap))
	for hex := range aircraftMap {
		hexCodes = append(hexCodes, hex)
	}

	// PERFORMANCE: Use batch query for ADSB data instead of N individual queries
	adsbStart := time.Now()
	s.logger.Debug("Starting ADSB data population (batch)", logger.Int("aircraft_count", len(aircraftMap)))

	adsbDataMap, err := s.GetLatestADSBDataBatch(hexCodes)
	if err != nil {
		s.logger.Error("Failed to get ADSB data batch", logger.Error(err))
	} else {
		for hex, aircraft := range aircraftMap {
			if adsbData, exists := adsbDataMap[hex]; exists {
				aircraft.ADSB = adsbData
			}
		}
	}

	adsbDuration := time.Since(adsbStart)
	s.logger.Debug("ADSB data population completed", logger.Duration("duration", adsbDuration))

	// Get current phases for all aircraft in a single batch query
	phaseStart := time.Now()
	s.logger.Debug("Starting phase data population", logger.Int("aircraft_count", len(aircraftMap)), logger.Bool("minimal", minimal))

	currentPhases, err := s.GetCurrentPhasesBatch(hexCodes)
	if err != nil {
		s.logger.Error("Failed to get current phases batch", logger.Error(err))
	} else {
		// In minimal mode, skip phase history query (only need current phase)
		var recentHistory map[string][]adsb.PhaseChange
		if !minimal {
			// Get recent phase history for all aircraft in batch (last 5 changes per aircraft)
			recentHistory, err = s.getRecentPhaseHistoryBatch(hexCodes, 5)
			if err != nil {
				s.logger.Error("Failed to get recent phase history batch", logger.Error(err))
				recentHistory = make(map[string][]adsb.PhaseChange) // Empty fallback
			}
		} else {
			recentHistory = make(map[string][]adsb.PhaseChange)
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

	// PERFORMANCE: Batch query for takeoff and landing times (skip in minimal mode)
	if !minimal {
		dateStart := time.Now()
		s.logger.Debug("Starting date_landed/date_tookoff population (batch)", logger.Int("aircraft_count", len(aircraftMap)))

		takeoffTimes, err := s.GetLatestTakeoffTimesBatch(hexCodes)
		if err != nil {
			s.logger.Error("Failed to get takeoff times batch", logger.Error(err))
		} else {
			for hex, aircraft := range aircraftMap {
				if takeoffTime, exists := takeoffTimes[hex]; exists {
					aircraft.DateTookoff = takeoffTime
				}
			}
		}

		landingTimes, err := s.GetLatestLandingTimesBatch(hexCodes)
		if err != nil {
			s.logger.Error("Failed to get landing times batch", logger.Error(err))
		} else {
			for hex, aircraft := range aircraftMap {
				if landingTime, exists := landingTimes[hex]; exists {
					aircraft.DateLanded = landingTime
				}
			}
		}

		dateDuration := time.Since(dateStart)
		s.logger.Debug("Date population completed (batch)", logger.Duration("duration", dateDuration))
	}

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

// GetLatestADSBDataBatch returns the latest ADSB data for multiple aircraft in a single query
func (s *AircraftStorage) GetLatestADSBDataBatch(hexCodes []string) (map[string]*adsb.ADSBTarget, error) {
	start := time.Now()
	s.logger.Debug("Starting batch ADSB query", logger.Int("hex_count", len(hexCodes)))

	if len(hexCodes) == 0 {
		return make(map[string]*adsb.ADSBTarget), nil
	}

	// Create placeholders for the IN clause (need two copies for the query)
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes)*2) // Double args: once for subquery, once for main query
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex               // First set for subquery
		args[i+len(hexCodes)] = hex // Second set for outer WHERE
	}

	// Use GROUP BY + JOIN pattern - more efficient than correlated subquery on large tables
	// 1. Subquery groups to find MAX(timestamp) per aircraft (small result set)
	// 2. JOIN fetches the actual data rows using the index
	query := fmt.Sprintf(`
		SELECT
			a.aircraft_hex,
			a.raw_data,
			a.source_type,
			a.registration,
			a.aircraft_type
		FROM adsb_targets a
		INNER JOIN (
			SELECT aircraft_hex, MAX(timestamp) as max_ts
			FROM adsb_targets
			WHERE aircraft_hex IN (%s)
			GROUP BY aircraft_hex
		) latest ON a.aircraft_hex = latest.aircraft_hex AND a.timestamp = latest.max_ts
		WHERE a.aircraft_hex IN (%s)
	`, strings.Join(placeholders, ","), strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
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
		var lat, lon, alt sql.NullFloat64

		if err := rows.Scan(&lat, &lon, &alt, &timestamp); err != nil {
			s.logger.Error("Error scanning position row", logger.Error(err), logger.String("hex", hex))
			return nil, err
		}

		// Skip rows without actual position instead of inventing zero coordinates.
		if !lat.Valid || !lon.Valid {
			continue
		}
		pos.Lat = lat.Float64
		pos.Lon = lon.Float64
		if alt.Valid {
			pos.AltBaro = alt.Float64
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
		SELECT id, lat, lon, alt_baro, gs, tas, track, true_heading, mag_heading, timestamp, registration, aircraft_type, source_type
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
		var lat, lon, altitude sql.NullFloat64
		var speedGS, speedTrue sql.NullFloat64
		var track, trueHeading, magHeading sql.NullFloat64

		if err := rows.Scan(&id, &lat, &lon, &altitude, &speedGS, &speedTrue, &track, &trueHeading, &magHeading, &timestamp,
			&registration, &aircraftType, &sourceType); err != nil {
			return nil, err
		}

		if !lat.Valid || !lon.Valid {
			continue
		}
		pos.Lat = nullFloatPtr(lat)
		pos.Lon = nullFloatPtr(lon)
		if altitude.Valid {
			pos.Altitude = nullFloatPtr(altitude)
		}

		if speedGS.Valid {
			pos.SpeedGS = nullFloatPtr(speedGS)
		}
		if speedTrue.Valid {
			pos.SpeedTrue = nullFloatPtr(speedTrue)
		}

		if track.Valid {
			v := track.Float64
			pos.Track = &v
		}
		if trueHeading.Valid {
			v := trueHeading.Float64
			pos.TrueHeading = &v
		}
		if magHeading.Valid {
			v := magHeading.Float64
			pos.MagHeading = &v
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
		if sourceType == "external-rapidapi" && (registration != "" || aircraftType != "") {
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
		var lat, lon, altitude sql.NullFloat64
		var speedGS, speedTrue, trueHeading, magHeading, verticalSpeed sql.NullFloat64

		if err := rows.Scan(&id, &lat, &lon, &altitude, &speedGS, &speedTrue, &trueHeading, &magHeading, &verticalSpeed, &timestamp,
			&registration, &aircraftType, &sourceType); err != nil {
			return nil, err
		}

		if !lat.Valid || !lon.Valid {
			continue
		}
		pos.Lat = nullFloatPtr(lat)
		pos.Lon = nullFloatPtr(lon)
		if altitude.Valid {
			pos.Altitude = nullFloatPtr(altitude)
		}

		if speedGS.Valid {
			pos.SpeedGS = nullFloatPtr(speedGS)
		}
		if speedTrue.Valid {
			pos.SpeedTrue = nullFloatPtr(speedTrue)
		}
		if trueHeading.Valid {
			v := trueHeading.Float64
			pos.TrueHeading = &v
		}
		if magHeading.Valid {
			v := magHeading.Float64
			pos.MagHeading = &v
		}
		if verticalSpeed.Valid {
			pos.VerticalSpeed = nullFloatPtr(verticalSpeed)
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
		if sourceType == "external-rapidapi" && (registration != "" || aircraftType != "") {
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
		SELECT id, lat, lon, alt_baro, gs, tas, track, true_heading, mag_heading, baro_rate, timestamp, registration, aircraft_type, source_type
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
		var lat, lon, altitude sql.NullFloat64
		var speedGS, speedTrue, verticalSpeed sql.NullFloat64
		var track, trueHeading, magHeading sql.NullFloat64

		if err := rows.Scan(&id, &lat, &lon, &altitude, &speedGS, &speedTrue, &track, &trueHeading, &magHeading, &verticalSpeed, &timestamp,
			&registration, &aircraftType, &sourceType); err != nil {
			return nil, err
		}

		if !lat.Valid || !lon.Valid {
			continue
		}
		pos.Lat = nullFloatPtr(lat)
		pos.Lon = nullFloatPtr(lon)
		if altitude.Valid {
			pos.Altitude = nullFloatPtr(altitude)
		}

		if speedGS.Valid {
			pos.SpeedGS = nullFloatPtr(speedGS)
		}
		if speedTrue.Valid {
			pos.SpeedTrue = nullFloatPtr(speedTrue)
		}

		if track.Valid {
			v := track.Float64
			pos.Track = &v
		}
		if trueHeading.Valid {
			v := trueHeading.Float64
			pos.TrueHeading = &v
		}
		if magHeading.Valid {
			v := magHeading.Float64
			pos.MagHeading = &v
		}
		if verticalSpeed.Valid {
			pos.VerticalSpeed = nullFloatPtr(verticalSpeed)
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
	if a.ADSB != nil && a.ADSB.HasPosition() && a.ADSB.AltBaro.Float64() != 0 {
		lat, lon, _ := a.ADSB.Position()
		// Get heading (use true_heading, track, or mag_heading, whichever is available)
		heading := adsb.NumberOrZero(a.ADSB.TrueHeading)
		if heading == 0 {
			heading = adsb.NumberOrZero(a.ADSB.Track)
		}
		if heading == 0 {
			heading = adsb.NumberOrZero(a.ADSB.MagHeading)
		}

		// Get speed (use TAS or GS, whichever is available)
		speed := adsb.NumberOrZero(a.ADSB.TAS)
		if speed == 0 {
			speed = adsb.NumberOrZero(a.ADSB.GS)
		}

		// Get vertical rate (use baro_rate or geom_rate, whichever is available)
		verticalRate := adsb.NumberOrZero(a.ADSB.BaroRate)
		if verticalRate == 0 {
			verticalRate = adsb.NumberOrZero(a.ADSB.GeomRate)
		}

		// Only predict if we have valid heading and speed
		if heading != 0 && speed != 0 {
			// Calculate future positions
			// Get magnetic heading for predictions
			magHeading := adsb.NumberOrZero(a.ADSB.MagHeading)
			if magHeading == 0 {
				magHeading = heading // fallback to whatever heading we found
			}

			a.Future = adsb.PredictFuturePositions(
				lat,
				lon,
				a.ADSB.AltBaro.Float64(),
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
	a.Hindcast = []adsb.Position{}

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
	lockSQLiteWrite()
	defer unlockSQLiteWrite()

	const maxBusyRetries = 3
	for attempt := 0; attempt <= maxBusyRetries; attempt++ {
		err := s.upsertOnce(aircraft)
		if err == nil {
			return
		}

		if !isSQLiteBusyError(err) || attempt == maxBusyRetries {
			s.logger.Error("Failed to upsert aircraft",
				logger.Error(err),
				logger.String("hex", aircraft.Hex),
				logger.Int("attempt", attempt+1))
			return
		}

		backoff := time.Duration((attempt+1)*100) * time.Millisecond
		s.logger.Warn("SQLite busy during aircraft upsert, retrying",
			logger.String("hex", aircraft.Hex),
			logger.Int("attempt", attempt+1),
			logger.Int("backoff_ms", int(backoff/time.Millisecond)))
		time.Sleep(backoff)
	}
}

func (s *AircraftStorage) upsertOnce(aircraft *adsb.Aircraft) (err error) {
	// Ensure all timestamps are in UTC
	aircraft.LastSeen = aircraft.LastSeen.UTC()

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.logger.Error("Failed to rollback transaction", logger.Error(rollbackErr), logger.String("hex", aircraft.Hex))
		}
	}()

	// Check if aircraft already exists
	var exists bool
	err = tx.QueryRow("SELECT 1 FROM aircraft WHERE hex = ?", aircraft.Hex).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check if aircraft exists: %w", err)
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
			return fmt.Errorf("insert aircraft: %w", err)
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
			return fmt.Errorf("update aircraft: %w", err)
		}
	}

	if aircraft.ADSB != nil {
		// Convert ADSB data to JSON
		rawData, err := json.Marshal(aircraft.ADSB)
		if err != nil {
			return fmt.Errorf("marshal ADSB data: %w", err)
		}

		// Get source type and registration/aircraft type directly from the ADSB data
		sourceType := "local"
		registration := ""
		aircraftType := ""

		if aircraft.ADSB.SourceType != "" {
			sourceType = aircraft.ADSB.SourceType
		}

		if aircraft.ADSB.Registration != "" {
			registration = aircraft.ADSB.Registration
		}

		if aircraft.ADSB.AircraftType != "" {
			aircraftType = aircraft.ADSB.AircraftType
		}

		// Insert the ADSB target (OR IGNORE handles dedup via UNIQUE constraint)
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO adsb_targets (
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
			registration, aircraftType,
			nullableFlexibleFloatValue(aircraft.ADSB.AltBaro), nullableFlexibleFloatValue(aircraft.ADSB.AltGeom), nullableFloatValue(aircraft.ADSB.GS), nullableFloatValue(aircraft.ADSB.IAS),
			nullableFloatValue(aircraft.ADSB.TAS), nullableFloatValue(aircraft.ADSB.Mach), nullableFloatValue(aircraft.ADSB.WD), nullableFloatValue(aircraft.ADSB.WS),
			nullableFloatValue(aircraft.ADSB.OAT), nullableFloatValue(aircraft.ADSB.TAT), aircraft.ADSB.Track, nullableFloatValue(aircraft.ADSB.TrackRate),
			nullableFloatValue(aircraft.ADSB.Roll), aircraft.ADSB.MagHeading, aircraft.ADSB.TrueHeading,
			nullableFloatValue(aircraft.ADSB.BaroRate), nullableFloatValue(aircraft.ADSB.GeomRate), aircraft.ADSB.Squawk,
			"", aircraft.ADSB.Category, nullableFloatValue(aircraft.ADSB.NavQNH),
			nullableFloatValue(aircraft.ADSB.NavAltitudeMCP), nullableFloatValue(aircraft.ADSB.NavAltitudeFMS), nullableFloatValue(aircraft.ADSB.NavHeading),
			"", nullableFloatValue(aircraft.ADSB.Lat), nullableFloatValue(aircraft.ADSB.Lon),
			nullableIntValue(aircraft.ADSB.NIC), nullableIntValue(aircraft.ADSB.RC), nullableFloatValue(aircraft.ADSB.SeenPos), nullableFloatValue(aircraft.ADSB.RDst),
			nullableFloatValue(aircraft.ADSB.RDir), nullableIntValue(aircraft.ADSB.Version), nullableIntValue(aircraft.ADSB.NICBaro), nullableIntValue(aircraft.ADSB.NACP),
			nullableIntValue(aircraft.ADSB.NACV), nullableIntValue(aircraft.ADSB.SIL), aircraft.ADSB.SILType, nullableIntValue(aircraft.ADSB.GVA),
			nullableIntValue(aircraft.ADSB.SDA), nullableIntValue(aircraft.ADSB.Alert), nullableIntValue(aircraft.ADSB.SPI),
			"", "",
			nullableIntValue(aircraft.ADSB.Messages), nullableFloatValue(aircraft.ADSB.Seen), nullableFloatValue(aircraft.ADSB.RSSI),
			aircraft.LastSeen.Format(time.RFC3339), string(rawData), sourceType,
		)
		if err != nil {
			return fmt.Errorf("insert ADSB target: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
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

func nullableFloatValue(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullableIntValue(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullableFlexibleFloatValue(v adsb.FlexibleFloat64) interface{} {
	return v.NullableValue()
}

func nullFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
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
	lockSQLiteWrite()
	defer unlockSQLiteWrite()

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

	// Create placeholders for the IN clause (need two copies for the query)
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes)*2)
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
		args[i+len(hexCodes)] = hex
	}

	// Use GROUP BY + JOIN pattern instead of ROW_NUMBER() for better performance
	query := fmt.Sprintf(`
		SELECT p.hex, p.id, p.phase, p.timestamp, p.adsb_id
		FROM phase_changes p
		INNER JOIN (
			SELECT hex, MAX(timestamp) as max_ts
			FROM phase_changes
			WHERE hex IN (%s)
			GROUP BY hex
		) latest ON p.hex = latest.hex AND p.timestamp = latest.max_ts
		WHERE p.hex IN (%s)
	`, strings.Join(placeholders, ","), strings.Join(placeholders, ","))

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

	// Create placeholders for the IN clause (need two copies for the query)
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes)*2)
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
		args[i+len(hexCodes)] = hex
	}

	// Use GROUP BY + JOIN pattern instead of ROW_NUMBER() for better performance
	query := fmt.Sprintf(`
		SELECT a.aircraft_hex, a.id
		FROM adsb_targets a
		INNER JOIN (
			SELECT aircraft_hex, MAX(timestamp) as max_ts
			FROM adsb_targets
			WHERE aircraft_hex IN (%s)
			GROUP BY aircraft_hex
		) latest ON a.aircraft_hex = latest.aircraft_hex AND a.timestamp = latest.max_ts
		WHERE a.aircraft_hex IN (%s)
	`, strings.Join(placeholders, ","), strings.Join(placeholders, ","))

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

	lockSQLiteWrite()
	defer unlockSQLiteWrite()

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

// GetLatestTakeoffTimesBatch returns the latest takeoff times for multiple aircraft in a single query
func (s *AircraftStorage) GetLatestTakeoffTimesBatch(hexCodes []string) (map[string]*time.Time, error) {
	if len(hexCodes) == 0 {
		return make(map[string]*time.Time), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes))
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}

	// Use GROUP BY with MAX - simple and efficient for getting latest timestamp per hex
	// Uses idx_phase_changes_hex_phase_timestamp index
	query := fmt.Sprintf(`
		SELECT hex, MAX(timestamp) as timestamp
		FROM phase_changes
		WHERE hex IN (%s) AND phase = 'T/O'
		GROUP BY hex
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest takeoff times batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*time.Time)
	for rows.Next() {
		var hex, timestampStr string

		if err := rows.Scan(&hex, &timestampStr); err != nil {
			return nil, fmt.Errorf("failed to scan takeoff time row: %w", err)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			s.logger.Error("Failed to parse takeoff timestamp", logger.Error(err), logger.String("hex", hex))
			continue
		}

		result[hex] = &timestamp
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating takeoff time rows: %w", err)
	}

	return result, nil
}

// GetLatestLandingTimesBatch returns the latest landing times for multiple aircraft in a single query
func (s *AircraftStorage) GetLatestLandingTimesBatch(hexCodes []string) (map[string]*time.Time, error) {
	if len(hexCodes) == 0 {
		return make(map[string]*time.Time), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes))
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}

	// Use GROUP BY with MAX - simple and efficient for getting latest timestamp per hex
	// Uses idx_phase_changes_hex_phase_timestamp index
	query := fmt.Sprintf(`
		SELECT hex, MAX(timestamp) as timestamp
		FROM phase_changes
		WHERE hex IN (%s) AND phase = 'T/D'
		GROUP BY hex
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest landing times batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*time.Time)
	for rows.Next() {
		var hex, timestampStr string

		if err := rows.Scan(&hex, &timestampStr); err != nil {
			return nil, fmt.Errorf("failed to scan landing time row: %w", err)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			s.logger.Error("Failed to parse landing timestamp", logger.Error(err), logger.String("hex", hex))
			continue
		}

		result[hex] = &timestamp
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating landing time rows: %w", err)
	}

	return result, nil
}

// GetStaleActiveAircraft returns aircraft that are no longer broadcasting and need status updates.
// Filters to: status='active', hex NOT IN activeHexCodes, last_seen < cutoff.
// Returns aircraft with their latest ADSB data populated.
func (s *AircraftStorage) GetStaleActiveAircraft(activeHexCodes []string, cutoff time.Time) ([]*adsb.Aircraft, error) {
	// Build NOT IN clause
	args := make([]interface{}, 0, len(activeHexCodes)+1)
	notInClause := ""
	if len(activeHexCodes) > 0 {
		placeholders := make([]string, len(activeHexCodes))
		for i, hex := range activeHexCodes {
			placeholders[i] = "?"
			args = append(args, hex)
		}
		notInClause = fmt.Sprintf("AND hex NOT IN (%s)", strings.Join(placeholders, ","))
	}
	args = append(args, cutoff.Format(time.RFC3339))

	query := fmt.Sprintf(`
		SELECT hex, flight, airline, status, last_seen, on_ground
		FROM aircraft
		WHERE status = 'active' %s AND last_seen < ?
	`, notInClause)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale active aircraft: %w", err)
	}
	defer rows.Close()

	var aircraft []*adsb.Aircraft
	var hexCodes []string
	for rows.Next() {
		var a adsb.Aircraft
		var lastSeen string
		var onGround int
		if err := rows.Scan(&a.Hex, &a.Flight, &a.Airline, &a.Status, &lastSeen, &onGround); err != nil {
			return nil, fmt.Errorf("failed to scan stale aircraft row: %w", err)
		}
		a.OnGround = onGround != 0
		t, err := time.Parse(time.RFC3339, lastSeen)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last_seen: %w", err)
		}
		a.LastSeen = t
		aircraft = append(aircraft, &a)
		hexCodes = append(hexCodes, a.Hex)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating stale aircraft rows: %w", err)
	}
	rows.Close()

	// Batch-fetch ADSB data after closing the rows cursor (avoids SQLite single-connection deadlock)
	if len(hexCodes) > 0 {
		adsbDataMap, err := s.GetLatestADSBDataBatch(hexCodes)
		if err == nil {
			for _, a := range aircraft {
				if adsbData, exists := adsbDataMap[a.Hex]; exists {
					a.ADSB = adsbData
				}
			}
		}
	}

	return aircraft, nil
}

// GetAircraftOnGroundBatch returns the on_ground state for multiple aircraft in a single query.
// The returned map contains hex -> on_ground for aircraft that exist in the database.
// Missing keys indicate the aircraft does not exist yet.
func (s *AircraftStorage) GetAircraftOnGroundBatch(hexCodes []string) (map[string]bool, error) {
	if len(hexCodes) == 0 {
		return make(map[string]bool), nil
	}

	placeholders := make([]string, len(hexCodes))
	args := make([]interface{}, len(hexCodes))
	for i, hex := range hexCodes {
		placeholders[i] = "?"
		args[i] = hex
	}

	query := fmt.Sprintf(`
		SELECT hex, on_ground FROM aircraft WHERE hex IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query aircraft on_ground batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var hex string
		var onGround int
		if err := rows.Scan(&hex, &onGround); err != nil {
			return nil, fmt.Errorf("failed to scan aircraft on_ground row: %w", err)
		}
		result[hex] = onGround != 0
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating aircraft on_ground rows: %w", err)
	}

	return result, nil
}
