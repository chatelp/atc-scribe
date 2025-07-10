package adsb

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexibleField can hold either a string or a number
type FlexibleField struct {
	value interface{}
}

// UnmarshalJSON implements custom JSON unmarshaling for FlexibleField
func (f *FlexibleField) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a number first
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		f.value = num
		return nil
	}

	// If that fails, try to unmarshal as a string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		f.value = str
		return nil
	}

	// If both fail, try to unmarshal as a boolean
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		f.value = b
		return nil
	}

	// If all fail, return an error
	return fmt.Errorf("cannot unmarshal %s into FlexibleField", data)
}

// Float64 returns the value as a float64
func (f *FlexibleField) Float64() float64 {
	switch v := f.value.(type) {
	case float64:
		return v
	case string:
		if v == "" || v == "ground" {
			return 0
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return f
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Int returns the value as an int
func (f *FlexibleField) Int() int {
	switch v := f.value.(type) {
	case float64:
		return int(v)
	case string:
		if v == "" {
			return 0
		}
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0
		}
		return i
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// String returns the value as a string
func (f *FlexibleField) String() string {
	switch v := f.value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

// ExternalADSBTarget represents a single aircraft in the external ADS-B API response
// It uses FlexibleField to handle fields that can be either string or numeric
type ExternalADSBTarget struct {
	Hex            string        `json:"hex"`
	Type           string        `json:"type"`
	Flight         string        `json:"flight"`
	Registration   string        `json:"r"` // External API specific field
	AircraftType   string        `json:"t"` // External API specific field
	AltBaro        FlexibleField `json:"alt_baro"`
	AltGeom        FlexibleField `json:"alt_geom"`
	GS             FlexibleField `json:"gs"`
	IAS            FlexibleField `json:"ias"`
	TAS            FlexibleField `json:"tas"`
	Mach           FlexibleField `json:"mach"`
	WD             FlexibleField `json:"wd"`
	WS             FlexibleField `json:"ws"`
	OAT            FlexibleField `json:"oat"`
	TAT            FlexibleField `json:"tat"`
	Track          FlexibleField `json:"track"`
	TrackRate      FlexibleField `json:"track_rate"`
	Roll           FlexibleField `json:"roll"`
	MagHeading     FlexibleField `json:"mag_heading"`
	TrueHeading    FlexibleField `json:"true_heading"`
	BaroRate       FlexibleField `json:"baro_rate"`
	GeomRate       FlexibleField `json:"geom_rate"`
	Squawk         string        `json:"squawk"`
	Category       string        `json:"category"`
	NavQNH         FlexibleField `json:"nav_qnh"`
	NavAltitudeMCP FlexibleField `json:"nav_altitude_mcp"`
	NavAltitudeFMS FlexibleField `json:"nav_altitude_fms"`
	NavHeading     FlexibleField `json:"nav_heading"`
	Lat            FlexibleField `json:"lat"`
	Lon            FlexibleField `json:"lon"`
	NIC            FlexibleField `json:"nic"`
	RC             FlexibleField `json:"rc"`
	SeenPos        FlexibleField `json:"seen_pos"`
	RDst           FlexibleField `json:"r_dst"`
	RDir           FlexibleField `json:"r_dir"`
	Version        FlexibleField `json:"version"`
	NICBaro        FlexibleField `json:"nic_baro"`
	NACP           FlexibleField `json:"nac_p"`
	NACV           FlexibleField `json:"nac_v"`
	SIL            FlexibleField `json:"sil"`
	SILType        string        `json:"sil_type"`
	GVA            FlexibleField `json:"gva"`
	SDA            FlexibleField `json:"sda"`
	Alert          FlexibleField `json:"alert"`
	SPI            FlexibleField `json:"spi"`
	MLAT           []string      `json:"mlat"`
	TISB           []string      `json:"tisb"`
	Messages       FlexibleField `json:"messages"`
	Seen           FlexibleField `json:"seen"`
	RSSI           FlexibleField `json:"rssi"`
}

// ExternalAPIResponse represents the raw JSON data from the external ADS-B API
type ExternalAPIResponse struct {
	Now      float64              `json:"now,omitempty"`
	Messages int                  `json:"messages,omitempty"`
	AC       []ExternalADSBTarget `json:"ac"`
}

// Convert converts an ExternalADSBTarget to the standard ADSBTarget format
func (e *ExternalADSBTarget) Convert() ADSBTarget {
	target := ADSBTarget{
		Hex:          e.Hex,
		Type:         e.Type,
		Flight:       e.Flight,
		Registration: e.Registration, // Copy registration field
		AircraftType: e.AircraftType, // Copy aircraft type field
		Squawk:       e.Squawk,
		Category:     e.Category,
		SILType:      e.SILType,
		MLAT:         e.MLAT,
		TISB:         e.TISB,
		SourceType:   "external", // Mark as coming from external source
	}

	// Convert numeric fields
	target.AltBaro = e.AltBaro.Float64()
	target.AltGeom = e.AltGeom.Float64()
	target.GS = e.GS.Float64()
	target.IAS = e.IAS.Float64()
	target.TAS = e.TAS.Float64()
	target.Mach = e.Mach.Float64()
	target.WD = e.WD.Float64()
	target.WS = e.WS.Float64()
	target.OAT = e.OAT.Float64()
	target.TAT = e.TAT.Float64()
	target.Track = e.Track.Float64()
	target.TrackRate = e.TrackRate.Float64()
	target.Roll = e.Roll.Float64()
	target.MagHeading = e.MagHeading.Float64()
	target.TrueHeading = e.TrueHeading.Float64()
	target.BaroRate = e.BaroRate.Float64()
	target.GeomRate = e.GeomRate.Float64()
	target.NavQNH = e.NavQNH.Float64()
	target.NavAltitudeMCP = e.NavAltitudeMCP.Float64()
	target.NavAltitudeFMS = e.NavAltitudeFMS.Float64()
	target.NavHeading = e.NavHeading.Float64()
	target.Lat = e.Lat.Float64()
	target.Lon = e.Lon.Float64()
	target.SeenPos = e.SeenPos.Float64()
	target.RDst = e.RDst.Float64()
	target.RDir = e.RDir.Float64()
	target.RSSI = e.RSSI.Float64()
	target.Seen = e.Seen.Float64()

	// Convert integer fields
	target.NIC = e.NIC.Int()
	target.RC = e.RC.Int()
	target.Version = e.Version.Int()
	target.NICBaro = e.NICBaro.Int()
	target.NACP = e.NACP.Int()
	target.NACV = e.NACV.Int()
	target.SIL = e.SIL.Int()
	target.GVA = e.GVA.Int()
	target.SDA = e.SDA.Int()
	target.Alert = e.Alert.Int()
	target.SPI = e.SPI.Int()
	target.Messages = e.Messages.Int()

	return target
}
