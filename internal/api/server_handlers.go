package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/pkg/logger"
)

// ServerState is what the settings panel shows about the running server.
//
// It exists because the only way to know how co-atc was actually behaving was to
// read its code: nothing surfaced that the active database grows without bound and
// is never rotated. A figure on screen finds that in a glance; a code reading finds
// it if someone happens to look.
type ServerState struct {
	Version       string                 `json:"version"`
	UptimeSeconds int64                  `json:"uptime_seconds"`
	Storage       StorageState           `json:"storage"`
	Logging       LoggingState           `json:"logging"`
	Settings      config.RuntimeSettings `json:"settings"`
	Writable      bool                   `json:"writable"`
}

type StorageState struct {
	Dir                string      `json:"dir"`
	ActiveDB           string      `json:"active_db"`
	ActiveDBBytes      int64       `json:"active_db_bytes"`
	GrowthBytesPerHour int64       `json:"growth_bytes_per_hour"` // -1 while still measuring
	Rotates            bool        `json:"rotates"`
	RetentionDays      int         `json:"retention_days"`
	DailyFiles         []DailyFile `json:"daily_files"`
	TotalBytes         int64       `json:"total_bytes"`
	DiskFreeBytes      int64       `json:"disk_free_bytes"`
	DiskTotalBytes     int64       `json:"disk_total_bytes"`
	DaysUntilFull      float64     `json:"days_until_full"` // -1 when not computable
}

type DailyFile struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	Active bool   `json:"active"`
}

type LoggingState struct {
	Level string `json:"level"`
}

// GetServerState reports how the server is actually running.
func (h *Handler) GetServerState(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		http.Error(w, "server state unavailable", http.StatusServiceUnavailable)
		return
	}

	st := ServerState{
		Version:       "0.1.0",
		UptimeSeconds: int64(time.Since(h.startedAt).Seconds()),
		Logging:       LoggingState{Level: logger.Level()},
		Settings:      h.runtime.Settings(),
		Writable:      false, // flips once authentication is in place
	}

	dir := h.config.Storage.SQLiteBasePath
	st.Storage = StorageState{
		Dir:           dir,
		ActiveDB:      h.db.Path(),
		RetentionDays: h.runtime.DBRetentionDays(),
		// True since the daily database began rotating at midnight. It used to be
		// opened once at startup and never reopened, so a long-running server
		// wrote one unbounded file that retention skipped as the active one. The
		// field stays because it is the difference between a database that is
		// bounded and one that is not, and a reader deserves to see which.
		Rotates:            true,
		GrowthBytesPerHour: -1,
		DaysUntilFull:      -1,
	}

	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "co-atc-") || !strings.HasSuffix(e.Name(), ".db") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			active := filepath.Base(h.db.Path()) == e.Name()
			st.Storage.DailyFiles = append(st.Storage.DailyFiles, DailyFile{e.Name(), info.Size(), active})
			st.Storage.TotalBytes += info.Size()
			if active {
				st.Storage.ActiveDBBytes = info.Size()
			}
		}
		sort.Slice(st.Storage.DailyFiles, func(i, j int) bool {
			return st.Storage.DailyFiles[i].Name > st.Storage.DailyFiles[j].Name
		})
	}

	var fs syscall.Statfs_t
	if err := syscall.Statfs(dir, &fs); err == nil {
		st.Storage.DiskFreeBytes = int64(fs.Bavail) * int64(fs.Bsize)
		st.Storage.DiskTotalBytes = int64(fs.Blocks) * int64(fs.Bsize)
	}

	// Growth is measured against a sample taken at start rather than divided by the
	// file's age: the file survives restarts, so its age says nothing about how fast
	// this process is filling it.
	if elapsed := time.Since(h.dbSampleAt); elapsed > 5*time.Minute && st.Storage.ActiveDBBytes > h.dbSampleBytes {
		perHour := float64(st.Storage.ActiveDBBytes-h.dbSampleBytes) / elapsed.Hours()
		st.Storage.GrowthBytesPerHour = int64(perHour)
		if perHour > 0 && st.Storage.DiskFreeBytes > 0 {
			st.Storage.DaysUntilFull = float64(st.Storage.DiskFreeBytes) / perHour / 24
		}
	}

	WriteJSON(w, http.StatusOK, st)
}

// AttachRuntime gives the handler the live settings and the database handle.
// Kept out of NewHandler so the two upstream constructors it sits between do
// not have to grow another parameter.
//
// It takes the handle rather than a path because the file changes: the daily
// database rotates at midnight, and a path captured here would name yesterday's.
func (h *Handler) AttachRuntime(rt *config.Runtime, db *sqlite.DB) {
	h.runtime = rt
	h.db = db
	if info, err := os.Stat(db.Path()); err == nil {
		h.dbSampleBytes = info.Size()
		h.dbSampleAt = time.Now()
	}
}
