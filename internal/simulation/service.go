package simulation

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/pkg/logger"
)

const (
	MaxSimulatedAircraft = 10 // Hardcoded maximum number of simulated aircraft
)

// SimulatedAircraft represents a single simulated aircraft with its current state
type SimulatedAircraft struct {
	Hex                string    `json:"hex"`
	Flight             string    `json:"flight"`
	AircraftType       string    `json:"aircraft_type"`
	CurrentLat         float64   `json:"current_lat"`
	CurrentLon         float64   `json:"current_lon"`
	CurrentAltitude    float64   `json:"current_altitude"`
	TargetHeading      float64   `json:"target_heading"`
	TargetSpeed        float64   `json:"target_speed"`
	TargetVerticalRate float64   `json:"target_vertical_rate"`
	LastUpdate         time.Time `json:"last_update"`
	CreatedAt          time.Time `json:"created_at"`
}

// Service manages simulated aircraft
type Service struct {
	aircraft map[string]*SimulatedAircraft
	mutex    sync.RWMutex
	logger   *logger.Logger
}

// NewService creates a new simulation service
func NewService(logger *logger.Logger) *Service {
	return &Service{
		aircraft: make(map[string]*SimulatedAircraft),
		logger:   logger.Named("simulation"),
	}
}

// CreateAircraft creates a new simulated aircraft
func (s *Service) CreateAircraft(lat, lon, altitude, heading, speed, verticalRate float64) (*SimulatedAircraft, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if we've reached the maximum
	if len(s.aircraft) >= MaxSimulatedAircraft {
		return nil, fmt.Errorf("maximum number of simulated aircraft (%d) reached", MaxSimulatedAircraft)
	}

	// Generate unique identifiers
	hex := s.generateUniqueHex()
	flight := s.generateFlightNumber()

	aircraft := &SimulatedAircraft{
		Hex:                hex,
		Flight:             flight,
		AircraftType:       "SIM",
		CurrentLat:         lat,
		CurrentLon:         lon,
		CurrentAltitude:    altitude,
		TargetHeading:      heading,
		TargetSpeed:        speed,
		TargetVerticalRate: verticalRate,
		LastUpdate:         time.Now().UTC(),
		CreatedAt:          time.Now().UTC(),
	}

	s.aircraft[hex] = aircraft
	s.logger.Info(fmt.Sprintf("Created simulated aircraft hex=%s flight=%s lat=%.6f lon=%.6f", hex, flight, lat, lon))

	return aircraft, nil
}

// UpdateControls updates the control parameters for a simulated aircraft
func (s *Service) UpdateControls(hex string, heading, speed, verticalRate float64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	aircraft, exists := s.aircraft[hex]
	if !exists {
		return fmt.Errorf("simulated aircraft with hex %s not found", hex)
	}

	aircraft.TargetHeading = heading
	aircraft.TargetSpeed = speed
	aircraft.TargetVerticalRate = verticalRate

	s.logger.Debug(fmt.Sprintf("Updated simulation controls hex=%s heading=%.1f speed=%.1f vs=%.0f", hex, heading, speed, verticalRate))
	return nil
}

// RemoveAircraft removes a simulated aircraft
func (s *Service) RemoveAircraft(hex string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.aircraft[hex]; !exists {
		return fmt.Errorf("simulated aircraft with hex %s not found", hex)
	}

	delete(s.aircraft, hex)
	s.logger.Info(fmt.Sprintf("Removed simulated aircraft hex=%s", hex))
	return nil
}

// GetAircraft returns a simulated aircraft by hex code
func (s *Service) GetAircraft(hex string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	aircraft, exists := s.aircraft[hex]
	return aircraft, exists
}

// GetAllAircraft returns all simulated aircraft
func (s *Service) GetAllAircraft() interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]*SimulatedAircraft, 0, len(s.aircraft))
	for _, aircraft := range s.aircraft {
		result = append(result, aircraft)
	}
	return result
}

// UpdatePositions updates the positions of all simulated aircraft based on their control parameters
func (s *Service) UpdatePositions() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now().UTC()
	for _, aircraft := range s.aircraft {
		deltaTime := now.Sub(aircraft.LastUpdate).Seconds()
		if deltaTime > 0 {
			s.updateAircraftPosition(aircraft, deltaTime)
			aircraft.LastUpdate = now
		}
	}
}

// GenerateADSBData generates ADSB data for all simulated aircraft
func (s *Service) GenerateADSBData() []adsb.ADSBTarget {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	targets := make([]adsb.ADSBTarget, 0, len(s.aircraft))
	for _, aircraft := range s.aircraft {
		target := adsb.ADSBTarget{
			Hex:          aircraft.Hex,
			Type:         "sim", // Mark as simulated
			Flight:       aircraft.Flight,
			AircraftType: aircraft.AircraftType,
			Lat:          aircraft.CurrentLat,
			Lon:          aircraft.CurrentLon,
			AltBaro:      aircraft.CurrentAltitude,
			AltGeom:      aircraft.CurrentAltitude,
			TAS:          aircraft.TargetSpeed,
			GS:           aircraft.TargetSpeed, // Simplified: assume no wind
			Track:        aircraft.TargetHeading,
			MagHeading:   aircraft.TargetHeading,
			TrueHeading:  aircraft.TargetHeading,
			BaroRate:     aircraft.TargetVerticalRate,
			GeomRate:     aircraft.TargetVerticalRate,
			Seen:         0,   // Always current
			Messages:     100, // Fake message count
			RSSI:         -20, // Good signal strength
		}
		targets = append(targets, target)
	}

	return targets
}

// updateAircraftPosition updates a single aircraft's position using dead reckoning
func (s *Service) updateAircraftPosition(aircraft *SimulatedAircraft, deltaTime float64) {
	// Convert heading to radians (0° = North, clockwise)
	// Aviation: 0°=North, 90°=East, 180°=South, 270°=West
	// Math: 0°=East, 90°=North, 180°=West, 270°=South
	// Conversion: math_angle = 90° - aviation_heading
	headingRad := (90 - aircraft.TargetHeading) * math.Pi / 180

	// Calculate distance traveled (speed in knots, time in seconds)
	// 1 knot = 1 nautical mile per hour = 1/3600 nautical miles per second
	distanceNM := aircraft.TargetSpeed * deltaTime / 3600

	// Update position using basic trigonometry
	// 1 degree latitude ≈ 60 nautical miles
	// 1 degree longitude ≈ 60 * cos(latitude) nautical miles
	latChange := distanceNM * math.Sin(headingRad) / 60
	lonChange := distanceNM * math.Cos(headingRad) / (60 * math.Cos(aircraft.CurrentLat*math.Pi/180))

	aircraft.CurrentLat += latChange
	aircraft.CurrentLon += lonChange

	// Update altitude (vertical rate in feet per minute)
	aircraft.CurrentAltitude += aircraft.TargetVerticalRate * deltaTime / 60

	// Ensure altitude doesn't go below ground level
	if aircraft.CurrentAltitude < 0 {
		aircraft.CurrentAltitude = 0
		aircraft.TargetVerticalRate = 0 // Stop descent at ground level
	}
}

// generateUniqueHex generates a unique 6-character hex code
func (s *Service) generateUniqueHex() string {
	for {
		hex := fmt.Sprintf("%06X", rand.Intn(0xFFFFFF))
		// Ensure it doesn't conflict with existing aircraft
		if _, exists := s.aircraft[hex]; !exists {
			return hex
		}
	}
}

// generateFlightNumber generates a flight number in format SIM001-SIM999
func (s *Service) generateFlightNumber() string {
	return fmt.Sprintf("SIM%03d", rand.Intn(999)+1)
}

// IsSimulated checks if a hex code belongs to a simulated aircraft
func (s *Service) IsSimulated(hex string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	_, exists := s.aircraft[hex]
	return exists
}
