package adsb

import (
	"time"
)

// RawAircraftData represents the raw JSON data from the ADS-B source
type RawAircraftData struct {
	Now      float64      `json:"now"`
	Messages int          `json:"messages"`
	Aircraft []ADSBTarget `json:"aircraft"`
}

// ADSBTarget represents a single aircraft in the raw ADS-B data
// This corresponds to entries in the adsb_targets table
type ADSBTarget struct {
	Hex            string   `json:"hex"`
	Type           string   `json:"type"`
	Flight         string   `json:"flight"`
	Registration   string   `json:"r,omitempty"` // External API specific field (r)
	AircraftType   string   `json:"t,omitempty"` // External API specific field (t)
	AltBaro        float64  `json:"alt_baro"`
	AltGeom        float64  `json:"alt_geom"`
	GS             float64  `json:"gs"`
	IAS            float64  `json:"ias"`
	TAS            float64  `json:"tas"`
	Mach           float64  `json:"mach"`
	WD             float64  `json:"wd"`
	WS             float64  `json:"ws"`
	OAT            float64  `json:"oat"`
	TAT            float64  `json:"tat"`
	Track          float64  `json:"track"`
	TrackRate      float64  `json:"track_rate"`
	Roll           float64  `json:"roll"`
	MagHeading     float64  `json:"mag_heading"`
	TrueHeading    float64  `json:"true_heading"`
	BaroRate       float64  `json:"baro_rate"`
	GeomRate       float64  `json:"geom_rate"`
	Squawk         string   `json:"squawk"`
	Category       string   `json:"category"`
	NavQNH         float64  `json:"nav_qnh"`
	NavAltitudeMCP float64  `json:"nav_altitude_mcp"`
	NavAltitudeFMS float64  `json:"nav_altitude_fms"`
	NavHeading     float64  `json:"nav_heading"`
	Lat            float64  `json:"lat"`
	Lon            float64  `json:"lon"`
	NIC            int      `json:"nic"`
	RC             int      `json:"rc"`
	SeenPos        float64  `json:"seen_pos"`
	RDst           float64  `json:"r_dst"`
	RDir           float64  `json:"r_dir"`
	Version        int      `json:"version"`
	NICBaro        int      `json:"nic_baro"`
	NACP           int      `json:"nac_p"`
	NACV           int      `json:"nac_v"`
	SIL            int      `json:"sil"`
	SILType        string   `json:"sil_type"`
	GVA            int      `json:"gva"`
	SDA            int      `json:"sda"`
	Alert          int      `json:"alert"`
	SPI            int      `json:"spi"`
	MLAT           []string `json:"mlat"`
	TISB           []string `json:"tisb"`
	Messages       int      `json:"messages"`
	Seen           float64  `json:"seen"`
	RSSI           float64  `json:"rssi"`
	SourceType     string   `json:"source_type,omitempty"` // Indicates whether data came from "local" or "external" source
}

// PositionMinimal represents a minimal historical position for map trails
type PositionMinimal struct {
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	AltBaro   float64   `json:"alt_baro"`
	Timestamp time.Time `json:"timestamp"`
}

// PhaseChange represents a single phase change record
type PhaseChange struct {
	ID        int       `json:"id"`
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	ADSBId    *int      `json:"adsb_id"`
}

// PhaseChangeInsert represents a phase change to be inserted in batch
type PhaseChangeInsert struct {
	Hex       string    `json:"hex"`
	Flight    string    `json:"flight"`
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	ADSBId    *int      `json:"adsb_id"`
	EventType string    `json:"event_type"` // "takeoff", "landing", or "" for normal phase changes
}

// PhaseData represents the phase information for an aircraft
type PhaseData struct {
	Current []PhaseChange `json:"current"` // Array with latest phase (same as first item in history)
	History []PhaseChange `json:"history"` // All phase changes in descending order by timestamp
}

// Aircraft represents a processed aircraft with essential fields and status
type Aircraft struct {
	Hex                string              `json:"hex"`
	Flight             string              `json:"flight"`
	Airline            string              `json:"airline"`
	Status             string              `json:"status"`
	LastSeen           time.Time           `json:"last_seen"`
	OnGround           bool                `json:"on_ground"`
	DateLanded         *time.Time          `json:"date_landed"`            // Derived from phase_changes table JOIN
	DateTookoff        *time.Time          `json:"date_tookoff"`           // Derived from phase_changes table JOIN
	CreatedAt          time.Time           `json:"created_at"`             // When the aircraft was first seen
	Distance           *float64            `json:"distance,omitempty"`     // Distance in NM from station
	RelativeDistance   *float64            `json:"rel_distance,omitempty"` // Distance in NM from reference aircraft
	RelativeBearing    *float64            `json:"rel_bearing,omitempty"`  // Relative bearing from reference aircraft (0 to 360)
	RelativeAlt        *float64            `json:"rel_altitude,omitempty"` // Relative altitude from reference aircraft (feet)
	ADSB               *ADSBTarget         `json:"adsb,omitempty"`
	History            []PositionMinimal   `json:"history,omitempty"`             // Minimal historical positions for map trails
	Future             []Position          `json:"future,omitempty"`              // Predicted future positions (placeholder for now)
	Phase              *PhaseData          `json:"phase,omitempty"`               // Phase information with current and history
	Clearances         []ClearanceData     `json:"clearances,omitempty"`          // Recent clearances for this aircraft
	IsSimulated        bool                `json:"is_simulated"`                  // Whether this is a simulated aircraft
	SimulationControls *SimulationControls `json:"simulation_controls,omitempty"` // Simulation control parameters
}

// SimulationControls represents the control parameters for simulated aircraft
type SimulationControls struct {
	TargetHeading      float64 `json:"target_heading"`       // Target heading in degrees (0-359)
	TargetSpeed        float64 `json:"target_speed"`         // Target true airspeed in knots
	TargetVerticalRate float64 `json:"target_vertical_rate"` // Target vertical rate in feet per minute
}

// ClearanceData represents clearance information in API responses
type ClearanceData struct {
	ID              int64     `json:"id"`
	Type            string    `json:"type"` // "takeoff" or "landing"
	Text            string    `json:"text"` // Full clearance text
	Runway          string    `json:"runway,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	Status          string    `json:"status"`            // "issued", "complied", "deviation"
	TimeSinceIssued string    `json:"time_since_issued"` // Human readable time since issued
}

// Position represents a historical position of an aircraft
type Position struct {
	ID            *int      `json:"id,omitempty"` // ADSB record ID
	Lat           float64   `json:"lat"`
	Lon           float64   `json:"lon"`
	Altitude      float64   `json:"altitude"`
	SpeedTrue     float64   `json:"speed_true"`
	SpeedGS       float64   `json:"speed_gs"`
	TrueHeading   float64   `json:"true_heading"`
	MagHeading    float64   `json:"mag_heading"`
	VerticalSpeed float64   `json:"vertical_speed"`
	Timestamp     time.Time `json:"timestamp"`
	Distance      *float64  `json:"distance,omitempty"` // Distance in NM from station
}

// AircraftMap is a map of aircraft keyed by hex ID
type AircraftMap map[string]*Aircraft

// AircraftCounts represents the counts of aircraft by ground/air and active/total
type AircraftCounts struct {
	GroundActive int `json:"ground_active"`
	GroundTotal  int `json:"ground_total"`
	AirActive    int `json:"air_active"`
	AirTotal     int `json:"air_total"`
}

// AircraftResponse represents the API response for aircraft data
type AircraftResponse struct {
	Timestamp time.Time      `json:"timestamp"`
	Count     int            `json:"count"`
	Counts    AircraftCounts `json:"counts"`
	Aircraft  []*Aircraft    `json:"aircraft"`
}

// AircraftHistoryResponse represents the API response for aircraft history
type AircraftHistoryResponse struct {
	Hex      string     `json:"hex"`
	Flight   string     `json:"flight"`
	Distance *float64   `json:"distance,omitempty"` // Distance in NM from station
	History  []Position `json:"history"`            // Historical positions (renamed from Positions)
}

// AircraftFutureResponse represents the API response for aircraft future predictions
type AircraftFutureResponse struct {
	Hex      string     `json:"hex"`
	Flight   string     `json:"flight"`
	Distance *float64   `json:"distance,omitempty"` // Distance in NM from station
	Future   []Position `json:"future"`             // Future predicted positions
}

// AircraftTracksResponse represents the API response for aircraft tracks (combined history and future)
type AircraftTracksResponse struct {
	Hex      string     `json:"hex"`
	Flight   string     `json:"flight"`
	Distance *float64   `json:"distance,omitempty"` // Distance in NM from station
	History  []Position `json:"history"`            // Historical positions
	Future   []Position `json:"future"`             // Future predicted positions
}

// RunwayApproachInfo contains information about aircraft's approach to a runway
type RunwayApproachInfo struct {
	RunwayID               string  `json:"runway_id"`
	DistanceToThreshold    float64 `json:"distance_to_threshold_nm"`
	DistanceFromCenterline float64 `json:"distance_from_centerline_nm"`
	HeadingAlignment       float64 `json:"heading_alignment_deg"`
	OnApproach             bool    `json:"on_approach"`
}

// RunwayDepartureInfo contains information about aircraft's departure from a runway
type RunwayDepartureInfo struct {
	RunwayID              string  `json:"runway_id"`
	DistanceFromThreshold float64 `json:"distance_from_threshold_nm"`
	HeadingAlignment      float64 `json:"heading_alignment_deg"`
	OnDeparture           bool    `json:"on_departure"`
}

// PhaseChangeAlert represents a flight phase change alert
type PhaseChangeAlert struct {
	Type      string    `json:"type"` // "phase_change"
	Hex       string    `json:"hex"`
	Flight    string    `json:"flight"`
	FromPhase string    `json:"from_phase"`
	ToPhase   string    `json:"to_phase"`
	EventType string    `json:"event_type"` // "takeoff", "landing", "phase_change"
	Timestamp time.Time `json:"timestamp"`
	Location  struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
		Alt float64 `json:"alt"`
	} `json:"location"`
	RunwayInfo *RunwayApproachInfo `json:"runway_info,omitempty"`
}
