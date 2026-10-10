package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yegors/co-atc/pkg/logger"
)

// GetAllFrequencies returns all frequencies with recent transcriptions
func (h *Handler) GetAllFrequencies(w http.ResponseWriter, r *http.Request) {
	// Get all frequencies
	frequencies := h.frequenciesService.GetAllFrequencies()

	// Fetch last 100 transcriptions per frequency
	transcriptionsByFreq := make(map[string]interface{})
	if h.transcriptionStorage != nil {
		for _, freq := range frequencies {
			txns, err := h.transcriptionStorage.GetTranscriptionsByFrequency(freq.ID, 100, 0)
			if err != nil {
				h.logger.Error("Failed to fetch transcriptions for frequency",
					logger.String("frequency_id", freq.ID),
					logger.Error(err))
				continue
			}
			if len(txns) > 0 {
				transcriptionsByFreq[freq.ID] = txns
			}
		}
	}

	// Create response
	response := map[string]interface{}{
		"timestamp":      time.Now().UTC(),
		"count":          len(frequencies),
		"frequencies":    frequencies,
		"transcriptions": transcriptionsByFreq,
	}

	// Write response
	WriteJSON(w, http.StatusOK, response)
}

// GetFrequencyByID returns a frequency by its ID
func (h *Handler) GetFrequencyByID(w http.ResponseWriter, r *http.Request) {
	// Get frequency ID from URL
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing frequency ID", http.StatusBadRequest)
		return
	}

	// Get frequency data
	frequency, found := h.frequenciesService.GetFrequencyByID(id)
	if !found {
		http.Error(w, "Frequency not found", http.StatusNotFound)
		return
	}

	// Write response
	WriteJSON(w, http.StatusOK, frequency)
}
