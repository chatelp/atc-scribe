package api

import (
	"net/http"
	"time"
)

// GetWeatherData returns cached weather data (METAR, TAF, NOTAMs)
func (h *Handler) GetWeatherData(w http.ResponseWriter, r *http.Request) {
	if h.weatherService == nil {
		// Weather service not available
		weatherData := struct {
			METAR       interface{} `json:"metar,omitempty"`
			TAF         interface{} `json:"taf,omitempty"`
			NOTAMs      interface{} `json:"notams,omitempty"`
			LastUpdated string      `json:"last_updated"`
			FetchErrors []string    `json:"fetch_errors,omitempty"`
		}{
			LastUpdated: time.Now().Format(time.RFC3339),
			FetchErrors: []string{"Weather service not available"},
		}
		WriteJSON(w, http.StatusOK, weatherData)
		return
	}

	// Get weather data from the service
	weatherData := h.weatherService.GetWeatherData()
	WriteJSON(w, http.StatusOK, weatherData)
}
