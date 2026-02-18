package sqlite

import (
	"strings"
	"sync"
)

// sqliteWriteMu serializes writes across all sqlite storage modules that share
// the same DB handle. SQLite allows one writer at a time, so cross-module
// writes should use the same lock to avoid SQLITE_BUSY contention.
var sqliteWriteMu sync.Mutex

func lockSQLiteWrite() {
	sqliteWriteMu.Lock()
}

func unlockSQLiteWrite() {
	sqliteWriteMu.Unlock()
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked")
}
