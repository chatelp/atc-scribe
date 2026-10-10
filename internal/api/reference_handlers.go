package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yegors/co-atc/internal/reference"
)

// GetAirports returns all airports within the configured display range
func (h *Handler) GetAirports(w http.ResponseWriter, r *http.Request) {
	if h.refService == nil {
		WriteJSON(w, http.StatusOK, []interface{}{})
		return
	}
	WriteJSON(w, http.StatusOK, h.refService.GetAirportsOnly())
}

// GetHeliports returns all heliports within the configured display range
func (h *Handler) GetHeliports(w http.ResponseWriter, r *http.Request) {
	if h.refService == nil {
		WriteJSON(w, http.StatusOK, []interface{}{})
		return
	}
	WriteJSON(w, http.StatusOK, h.refService.GetHeliportsOnly())
}

// GetAirportByIdent returns a single airport with full details including frequencies
func (h *Handler) GetAirportByIdent(w http.ResponseWriter, r *http.Request) {
	ident := strings.ToUpper(chi.URLParam(r, "ident"))
	if h.refService == nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "reference service not available"})
		return
	}
	airport := h.refService.GetAirport(ident)
	if airport == nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "airport not found"})
		return
	}

	// Also include associated runways
	var airportRunways []*reference.RunwayInfo
	for _, rwy := range h.refService.GetRunways() {
		if strings.EqualFold(rwy.AirportIdent, ident) {
			airportRunways = append(airportRunways, rwy)
		}
	}

	response := struct {
		*reference.AirportInfo
		Runways []*reference.RunwayInfo `json:"runways,omitempty"`
	}{
		AirportInfo: airport,
		Runways:     airportRunways,
	}
	WriteJSON(w, http.StatusOK, response)
}

// GetNavaids returns all navaids within the configured display range
func (h *Handler) GetNavaids(w http.ResponseWriter, r *http.Request) {
	if h.refService == nil {
		WriteJSON(w, http.StatusOK, []interface{}{})
		return
	}
	WriteJSON(w, http.StatusOK, h.refService.GetNavaids())
}

// GetNavaidByIdent returns all navaids matching the given ident
func (h *Handler) GetNavaidByIdent(w http.ResponseWriter, r *http.Request) {
	ident := strings.ToUpper(chi.URLParam(r, "ident"))
	if h.refService == nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "reference service not available"})
		return
	}
	navaids := h.refService.GetNavaidsByIdent(ident)
	if len(navaids) == 0 {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "navaid not found"})
		return
	}
	WriteJSON(w, http.StatusOK, navaids)
}

// GetRunways returns all runways within the configured display range
func (h *Handler) GetRunways(w http.ResponseWriter, r *http.Request) {
	if h.refService == nil {
		WriteJSON(w, http.StatusOK, []interface{}{})
		return
	}
	WriteJSON(w, http.StatusOK, h.refService.GetRunways())
}
