package api

import (
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yegors/co-atc/internal/audioarchive"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/pkg/logger"
)

// newAudioArchive is the sidecar's archive of clips: the directory the
// configuration names, or else the --save-audio of the sidecar's command.
func newAudioArchive(cfg *config.Config) *audioarchive.Archive {
	if cfg == nil {
		return nil
	}
	dir := cfg.Transcription.Local.AudioArchiveDir
	if dir == "" {
		dir = audioarchive.FromSidecarCommand(cfg.Transcription.Local.Command)
	}
	return audioarchive.New(dir)
}

// GetAudioArchive says whether transmissions can be listened to again: an
// archive is configured and its directory is there (an external disk may be
// unplugged). The page shows its play buttons only then.
func (h *Handler) GetAudioArchive(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]bool{
		"configured": h.audioArchive != nil,
		"available":  h.audioArchive.Available(),
	})
}

// GetTranscriptionAudio serves the clip of one transmission, as the sidecar
// archived it (WAV, as received), or 404 when none was kept: archiving off,
// the disk full or unplugged, or a transmission older than the archive.
func (h *Handler) GetTranscriptionAudio(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid transmission ID", http.StatusBadRequest)
		return
	}
	if !h.audioArchive.Available() {
		http.Error(w, "No audio archive", http.StatusNotFound)
		return
	}
	t, err := h.transcriptionStorage.GetTranscriptionByID(id)
	if err != nil {
		h.logger.Error("Failed to read transmission", logger.Error(err), logger.Int("id", int(id)))
		http.Error(w, "Failed to read transmission", http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.Error(w, "Transmission not found", http.StatusNotFound)
		return
	}
	path := h.audioArchive.Find(t.FrequencyID, t.CreatedAt)
	if path == "" {
		http.Error(w, "No audio kept for this transmission", http.StatusNotFound)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "No audio kept for this transmission", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to read audio", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, "", st.ModTime(), f)
}
