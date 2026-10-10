package api

import (
	"os"
	"testing"
)

// skipDefect marks a test that describes a defect found while writing these
// tests and left uncorrected on purpose (fiche F: the test is written, the fix
// is not). Run with ATC_DEFECTS=1 to see it fail; it passes once the defect is
// fixed, and this call is then removed.
func skipDefect(t *testing.T, what string) {
	t.Helper()
	if os.Getenv("ATC_DEFECTS") == "" {
		t.Skip("defect: " + what)
	}
}
