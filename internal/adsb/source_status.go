package adsb

import "time"

const (
	SourceTypeExternalAPI = "external_api"
	SourceTypeTar1090     = "tar1090"
	SourceTypeReadsbAPI   = "readsb_api"
	SourceTypeReadsbFile  = "readsb_file"
)

type SourceChannelStatus struct {
	Available     bool       `json:"available"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	Data          any        `json:"data"`
}

type SourceStatus struct {
	SourceType string              `json:"source_type"`
	Mode       string              `json:"mode"`
	Status     string              `json:"status"`
	Aircraft   SourceChannelStatus `json:"aircraft"`
	Receiver   SourceChannelStatus `json:"receiver"`
	Stats      SourceChannelStatus `json:"stats"`
	UpdatedAt  *time.Time          `json:"updated_at,omitempty"`
}
