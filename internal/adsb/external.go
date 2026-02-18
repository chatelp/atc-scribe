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

func (f *FlexibleField) HasValue() bool {
	if f == nil || f.value == nil {
		return false
	}
	if s, ok := f.value.(string); ok {
		return s != ""
	}
	return true
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
	v, ok := f.Float64OK()
	if !ok {
		return 0
	}
	return v
}

func (f *FlexibleField) Float64OK() (float64, bool) {
	switch v := f.value.(type) {
	case float64:
		return v, true
	case string:
		if v == "" {
			return 0, false
		}
		if v == "ground" {
			return 0, true
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func (f *FlexibleField) Float64Ptr() *float64 {
	if !f.HasValue() {
		return nil
	}
	v, ok := f.Float64OK()
	if !ok {
		return nil
	}
	return &v
}

// Int returns the value as an int
func (f *FlexibleField) Int() int {
	v, ok := f.IntOK()
	if !ok {
		return 0
	}
	return v
}

func (f *FlexibleField) IntOK() (int, bool) {
	if !f.HasValue() {
		return 0, false
	}

	switch v := f.value.(type) {
	case float64:
		return int(v), true
	case string:
		if v == "" {
			return 0, false
		}
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return i, true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func (f *FlexibleField) IntPtr() *int {
	v, ok := f.IntOK()
	if !ok {
		return nil
	}
	return &v
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
		SourceType:   SourceTypeExternalAPI, // Mark as coming from external source
	}

	// Convert numeric fields
	if v, ok := e.AltBaro.Float64OK(); ok {
		target.AltBaro = FlexibleFloat64(v)
	} else {
		target.AltBaro = NullFlexibleFloat64()
	}
	if v, ok := e.AltGeom.Float64OK(); ok {
		target.AltGeom = FlexibleFloat64(v)
	} else {
		target.AltGeom = NullFlexibleFloat64()
	}
	target.GS = e.GS.Float64Ptr()
	target.IAS = e.IAS.Float64Ptr()
	target.TAS = e.TAS.Float64Ptr()
	target.Mach = e.Mach.Float64Ptr()
	target.WD = e.WD.Float64Ptr()
	target.WS = e.WS.Float64Ptr()
	target.OAT = e.OAT.Float64Ptr()
	target.TAT = e.TAT.Float64Ptr()
	target.Track = e.Track.Float64Ptr()
	target.TrackRate = e.TrackRate.Float64Ptr()
	target.Roll = e.Roll.Float64Ptr()
	target.MagHeading = e.MagHeading.Float64Ptr()
	target.TrueHeading = e.TrueHeading.Float64Ptr()
	target.BaroRate = e.BaroRate.Float64Ptr()
	target.GeomRate = e.GeomRate.Float64Ptr()
	target.NavQNH = e.NavQNH.Float64Ptr()
	target.NavAltitudeMCP = e.NavAltitudeMCP.Float64Ptr()
	target.NavAltitudeFMS = e.NavAltitudeFMS.Float64Ptr()
	target.NavHeading = e.NavHeading.Float64Ptr()
	target.Lat = e.Lat.Float64Ptr()
	target.Lon = e.Lon.Float64Ptr()
	target.SeenPos = e.SeenPos.Float64Ptr()
	target.RDst = e.RDst.Float64Ptr()
	target.RDir = e.RDir.Float64Ptr()
	target.RSSI = e.RSSI.Float64Ptr()
	target.Seen = e.Seen.Float64Ptr()

	// Convert integer fields
	target.NIC = e.NIC.IntPtr()
	target.RC = e.RC.IntPtr()
	target.Version = e.Version.IntPtr()
	target.NICBaro = e.NICBaro.IntPtr()
	target.NACP = e.NACP.IntPtr()
	target.NACV = e.NACV.IntPtr()
	target.SIL = e.SIL.IntPtr()
	target.GVA = e.GVA.IntPtr()
	target.SDA = e.SDA.IntPtr()
	target.Alert = e.Alert.IntPtr()
	target.SPI = e.SPI.IntPtr()
	target.Messages = e.Messages.IntPtr()

	return target
}
