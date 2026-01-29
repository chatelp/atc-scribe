package bsdb

import (
	"database/sql"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// AircraftInfo represents aircraft data from BaseStation.sqb
type AircraftInfo struct {
	ModeS            string `json:"modes"`
	Registration     string `json:"registration,omitempty"`
	ICAOTypeCode     string `json:"icao_type_code,omitempty"`
	OperatorFlagCode string `json:"operator_flag_code,omitempty"`
	Manufacturer     string `json:"manufacturer,omitempty"`
	Type             string `json:"type,omitempty"`
	RegisteredOwners string `json:"registered_owners,omitempty"`
}

// Service provides lookup functionality for BaseStation.sqb database
type Service struct {
	db       *sql.DB
	cache    map[string]*AircraftInfo
	cacheMu  sync.RWMutex
	useCache bool
}

// NewService creates a new BSDB service
// If preload is true, loads entire database into memory for faster lookups
func NewService(dbPath string, preload bool) (*Service, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	s := &Service{
		db:       db,
		cache:    make(map[string]*AircraftInfo),
		useCache: preload,
	}

	if preload {
		if err := s.preloadCache(); err != nil {
			db.Close()
			return nil, err
		}
	}

	return s, nil
}

// preloadCache loads entire database into memory
func (s *Service) preloadCache() error {
	rows, err := s.db.Query(`
		SELECT ModeS, Registration, ICAOTypeCode, OperatorFlagCode, Manufacturer, Type, RegisteredOwners
		FROM Aircraft
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	for rows.Next() {
		var info AircraftInfo
		var registration, icaoTypeCode, operatorFlagCode, manufacturer, typ, registeredOwners sql.NullString

		err := rows.Scan(&info.ModeS, &registration, &icaoTypeCode, &operatorFlagCode, &manufacturer, &typ, &registeredOwners)
		if err != nil {
			return err
		}

		info.Registration = registration.String
		info.ICAOTypeCode = icaoTypeCode.String
		info.OperatorFlagCode = operatorFlagCode.String
		info.Manufacturer = manufacturer.String
		info.Type = typ.String
		info.RegisteredOwners = registeredOwners.String

		// Store with uppercase key for case-insensitive lookup
		s.cache[strings.ToUpper(info.ModeS)] = &info
	}

	return rows.Err()
}

// Lookup retrieves aircraft info by hex code (ModeS)
func (s *Service) Lookup(hex string) *AircraftInfo {
	hexUpper := strings.ToUpper(hex)

	if s.useCache {
		s.cacheMu.RLock()
		info := s.cache[hexUpper]
		s.cacheMu.RUnlock()
		return info
	}

	// Direct database query
	var info AircraftInfo
	var registration, icaoTypeCode, operatorFlagCode, manufacturer, typ, registeredOwners sql.NullString

	err := s.db.QueryRow(`
		SELECT ModeS, Registration, ICAOTypeCode, OperatorFlagCode, Manufacturer, Type, RegisteredOwners
		FROM Aircraft
		WHERE UPPER(ModeS) = ?
	`, hexUpper).Scan(&info.ModeS, &registration, &icaoTypeCode, &operatorFlagCode, &manufacturer, &typ, &registeredOwners)

	if err != nil {
		return nil
	}

	info.Registration = registration.String
	info.ICAOTypeCode = icaoTypeCode.String
	info.OperatorFlagCode = operatorFlagCode.String
	info.Manufacturer = manufacturer.String
	info.Type = typ.String
	info.RegisteredOwners = registeredOwners.String

	return &info
}

// LookupBatch retrieves aircraft info for multiple hex codes
func (s *Service) LookupBatch(hexCodes []string) map[string]*AircraftInfo {
	result := make(map[string]*AircraftInfo)

	for _, hex := range hexCodes {
		if info := s.Lookup(hex); info != nil {
			result[strings.ToUpper(hex)] = info
		}
	}

	return result
}

// Count returns the number of entries in the database
func (s *Service) Count() int {
	if s.useCache {
		s.cacheMu.RLock()
		defer s.cacheMu.RUnlock()
		return len(s.cache)
	}

	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM Aircraft").Scan(&count)
	return count
}

// Close closes the database connection
func (s *Service) Close() error {
	return s.db.Close()
}
