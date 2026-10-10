package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/storage/sqlite"
	"github.com/yegors/co-atc/pkg/logger"
)

func runDatabaseRetentionCleanup(ctx context.Context, dbDir string, db *sqlite.DB, rt *config.Runtime, log *logger.Logger) {
	// Two cadences. Rotation is checked every minute so the switch lands close
	// to midnight; retention is an hourly sweep, which is as often as it can
	// possibly matter.
	rotateTicker := time.NewTicker(1 * time.Minute)
	defer rotateTicker.Stop()
	retentionTicker := time.NewTicker(1 * time.Hour)
	defer retentionTicker.Stop()

	sweep := func() {
		// Read on every pass, not captured once: a retention changed from the
		// settings panel takes effect on the next sweep rather than at the next
		// restart.
		capGB := rt.DBRetentionGB()
		if err := cleanupOldDailyDatabases(dbDir, db.Path(), capGB, log); err != nil {
			log.Warn("Periodic database retention cleanup failed",
				logger.Error(err),
				logger.String("path", dbDir),
				logger.Float64("retention_gb", capGB))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-rotateTicker.C:
			// The whole point of the loop. Before this, the file opened at
			// startup was written to for the life of the process and retention
			// skipped it as the active one, so a server left running simply grew
			// one database without limit.
			rotated, err := db.RotateIfNewDay(dbDir, time.Now())
			if err != nil {
				log.Error("Failed to rotate to today's database",
					logger.Error(err), logger.String("dir", dbDir))
				continue
			}
			if rotated {
				// Yesterday's file is closed now, so it is eligible for deletion
				// without waiting for the hourly sweep.
				sweep()
			}

		case <-retentionTicker.C:
			sweep()
		}
	}
}

// cleanupOldDailyDatabases keeps the daily databases within capGB together
// (28/09, D65). Newest first: the files that fit are kept, and from the first one
// that does not, it and every older file are deleted -- never a newer day for an
// older one. Today's file is never deleted, and counts. A file's -wal and -shm
// count with it and go with it.
func cleanupOldDailyDatabases(dbDir, activeDBPath string, capGB float64, log *logger.Logger) error {
	if capGB <= 0 {
		capGB = config.DefaultDBRetentionGB
	}
	capBytes := int64(capGB * (1 << 30))

	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return fmt.Errorf("read db directory: %w", err)
	}

	type daily struct {
		path  string
		date  time.Time
		bytes int64
	}
	// The same file can be spelled two ways: relative and absolute, or through a
	// symbolic link (/var is one on macOS, to /private/var). Compared by name
	// only, today's file passed for an old one there, and went when over the
	// cap. It is now recognised by name or by identity.
	absActiveDBPath, _ := filepath.Abs(activeDBPath)
	activeInfo, _ := os.Stat(activeDBPath)
	isActive := func(path string) bool {
		if abs, _ := filepath.Abs(path); abs == absActiveDBPath {
			return true
		}
		if activeInfo == nil {
			return false
		}
		info, err := os.Stat(path)
		return err == nil && os.SameFile(info, activeInfo)
	}
	var files []daily
	var total int64
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(fileName, "co-atc-") || !strings.HasSuffix(fileName, ".db") {
			continue
		}
		fileDate, parseErr := time.Parse("2006-01-02", strings.TrimSuffix(strings.TrimPrefix(fileName, "co-atc-"), ".db"))
		if parseErr != nil {
			continue
		}
		path := filepath.Join(dbDir, fileName)
		var size int64
		for _, p := range []string{path, path + "-wal", path + "-shm"} {
			if info, err := os.Stat(p); err == nil {
				size += info.Size()
			}
		}
		if isActive(path) {
			total += size // kept whatever its size
			continue
		}
		files = append(files, daily{path, fileDate, size})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].date.After(files[j].date) })

	deleted, over := 0, false
	var freed int64
	for _, f := range files {
		if !over && total+f.bytes <= capBytes {
			total += f.bytes
			continue
		}
		over = true
		for _, p := range []string{f.path, f.path + "-wal", f.path + "-shm"} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove old db '%s': %w", p, err)
			}
		}
		deleted++
		freed += f.bytes
		log.Info("Deleted old database file",
			logger.String("path", f.path),
			logger.String("file_date", f.date.Format("2006-01-02")),
			logger.Int64("bytes", f.bytes))
	}

	if deleted > 0 {
		log.Info("Database retention cleanup complete",
			logger.Int("deleted_files", deleted),
			logger.Int64("freed_bytes", freed),
			logger.Int64("kept_bytes", total),
			logger.Float64("retention_gb", capGB))
	}
	return nil
}
