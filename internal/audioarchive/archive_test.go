package audioarchive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeDay lays out one day of archive as the sidecar does: the clips, and
// the manifest beside them.
func writeDay(t *testing.T, root, day string, lines ...string) {
	t.Helper()
	dir := filepath.Join(root, day)
	if err := os.MkdirAll(filepath.Join(dir, "124350"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "manifeste.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func touch(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, rel), []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestATransmissionFindsItsClip(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 10, 14, 48, 41, 0, time.UTC)
	day := at.Local().Format("20060102")
	// The 10/10 line carries created_at in local time with an offset; the
	// database holds the same instant in UTC.
	writeDay(t, root, day,
		`{"frequency_id": "124350", "created_at": "`+at.In(time.FixedZone("CEST", 7200)).Format(time.RFC3339)+`", "fichier": "`+day+`/124350/a.wav"}`,
		`{"frequency_id": "124350", "segment_at_utc": "2026-10-10T14:50:02.512Z", "fichier": "`+day+`/124350/b.wav"}`,
		`not json`,
		`{"frequency_id": "124350", "created_at": "2026-10-10T14:55:00Z", "fichier": "../../etc/passwd"}`,
	)
	touch(t, root, day+"/124350/a.wav")
	touch(t, root, day+"/124350/b.wav")
	a := New(root)

	if got, want := a.Find("124350", at), filepath.Join(root, day, "124350", "a.wav"); got != want {
		t.Errorf("by created_at: %q, want %q", got, want)
	}
	if got := a.Find("124350", time.Date(2026, 10, 10, 14, 50, 2, 0, time.UTC)); filepath.Base(got) != "b.wav" {
		t.Errorf("by the segment's moment, for lines older than created_at: %q", got)
	}
	if got := a.Find("126425", at); got != "" {
		t.Errorf("another frequency at the same second: %q, want none", got)
	}
	if got := a.Find("124350", time.Date(2026, 10, 10, 14, 55, 0, 0, time.UTC)); got != "" {
		t.Errorf("a path leaving the archive was served: %q", got)
	}
	if got := a.Find("124350", at.Add(time.Second)); got != "" {
		t.Errorf("one second off: %q, want none", got)
	}
}

func TestLinesWrittenLaterAreFound(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	day := at.Local().Format("20060102")
	writeDay(t, root, day)
	a := New(root)
	if got := a.Find("124350", at); got != "" {
		t.Fatalf("before the clip: %q", got)
	}
	writeDay(t, root, day, `{"frequency_id": "124350", "created_at": "2026-10-11T09:00:00Z", "fichier": "`+day+`/124350/c.wav"}`)
	touch(t, root, day+"/124350/c.wav")
	if got := a.Find("124350", at); filepath.Base(got) != "c.wav" {
		t.Errorf("after the sidecar appended its line: %q", got)
	}
}

func TestAMissingClipOrArchiveIsNone(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	day := at.Local().Format("20060102")
	writeDay(t, root, day, `{"frequency_id": "124350", "created_at": "2026-10-11T09:00:00Z", "fichier": "`+day+`/124350/gone.wav"}`)
	if got := New(root).Find("124350", at); got != "" {
		t.Errorf("a clip deleted from the disk: %q", got)
	}
	if New("") != nil || New("").Available() {
		t.Error("no directory, no archive")
	}
	if New(filepath.Join(root, "unplugged")).Available() {
		t.Error("a missing directory is not available")
	}
}

func TestTheArchiveIsWhereTheSidecarWritesIt(t *testing.T) {
	for _, c := range []struct {
		cmd  []string
		want string
	}{
		{[]string{"python", "w.py", "--save-audio", "/Volumes/X/a", "--preload"}, "/Volumes/X/a"},
		{[]string{"python", "w.py", "--save-audio=/data/a"}, "/data/a"},
		{[]string{"python", "w.py", "--save-audio-min-free-gb", "60"}, ""},
		{nil, ""},
	} {
		if got := FromSidecarCommand(c.cmd); got != c.want {
			t.Errorf("%v: %q, want %q", c.cmd, got, c.want)
		}
	}
}
