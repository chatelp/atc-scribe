package transcription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New(logger.Config{Level: "error", Format: "console"})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return log
}

func healthServer(t *testing.T) *httptest.Server {
	t.Helper()
	// No pid: a sidecar that does not report one is tolerated, so these tests
	// exercise spawning and stopping without also asserting ownership.
	return healthServerSaying(t, `{"status":"ok"}`)
}

func healthServerSaying(t *testing.T, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// With no command configured, an unreachable sidecar must stop the server rather
// than let it run half deaf -- and the error has to say what to do about it.
func TestProbeOnlyFailsWhenNothingIsListening(t *testing.T) {
	s := NewSidecar(SidecarConfig{
		ServerURL:             "http://127.0.0.1:1",
		StartupTimeoutSeconds: 1,
	}, testLogger(t))

	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("expected an error when no sidecar is listening")
	}
	for _, want := range []string{"127.0.0.1:1", "server_url", "command"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestProbeOnlySucceedsAgainstARunningSidecar(t *testing.T) {
	srv := healthServer(t)
	s := NewSidecar(SidecarConfig{ServerURL: srv.URL, StartupTimeoutSeconds: 5}, testLogger(t))

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Nothing was spawned, so Stop must leave the foreign process alone.
	s.Stop()
	if err := s.probe(context.Background()); err != nil {
		t.Errorf("a sidecar we did not start must survive Stop: %v", err)
	}
}

func TestStartSpawnsAndStopTerminates(t *testing.T) {
	srv := healthServer(t)
	s := NewSidecar(SidecarConfig{
		ServerURL:             srv.URL,
		Command:               []string{"sh", "-c", "sleep 60"},
		StartupTimeoutSeconds: 5,
	}, testLogger(t))

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := s.cmd.Process.Pid
	if err := syscall.Kill(pid, syscall.Signal(0)); err != nil {
		t.Fatalf("child %d should be running: %v", pid, err)
	}

	s.Stop()
	if err := syscall.Kill(pid, syscall.Signal(0)); err == nil {
		t.Errorf("child %d should be gone after Stop", pid)
	}
}

// The reason Stop signals the process group: the real sidecar's dependencies
// fork helpers that a signal to the parent alone would orphan. We watched one --
// a multiprocessing resource_tracker -- survive its parent on this machine.
func TestStopReachesGrandchildren(t *testing.T) {
	srv := healthServer(t)
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")

	s := NewSidecar(SidecarConfig{
		ServerURL:             srv.URL,
		Command:               []string{"sh", "-c", "sleep 60 & echo $! > " + pidFile + "; wait"},
		StartupTimeoutSeconds: 5,
	}, testLogger(t))

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var grandchild int
	for i := 0; i < 50; i++ {
		if b, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				grandchild = pid
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if grandchild == 0 {
		t.Fatal("grandchild never reported its pid")
	}

	s.Stop()

	// SIGTERM travels the group, but reaping is not instantaneous.
	gone := false
	for i := 0; i < 40; i++ {
		if err := syscall.Kill(grandchild, syscall.Signal(0)); err != nil {
			gone = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !gone {
		_ = syscall.Kill(grandchild, syscall.SIGKILL)
		t.Errorf("grandchild %d survived Stop -- the process group was not signalled", grandchild)
	}
}

// A sidecar that dies on startup should be reported at once, with its own output,
// instead of making the operator wait out the whole timeout for a bare failure.
func TestChildThatExitsIsReportedWithItsOutput(t *testing.T) {
	s := NewSidecar(SidecarConfig{
		ServerURL:             "http://127.0.0.1:1",
		Command:               []string{"sh", "-c", "echo 'ModuleNotFoundError: mlx_whisper' >&2; exit 3"},
		StartupTimeoutSeconds: 30,
	}, testLogger(t))

	started := time.Now()
	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("expected an error when the sidecar exits at startup")
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Errorf("should fail as soon as the child exits, took %s", elapsed)
	}
	if !strings.Contains(err.Error(), "exited") {
		t.Errorf("error should say the sidecar exited, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ModuleNotFoundError") {
		t.Errorf("error should carry the sidecar's own output, got: %v", err)
	}
}

// A command that starts but never answers must fail on the timeout, not hang.
func TestSpawnedButSilentSidecarTimesOut(t *testing.T) {
	s := NewSidecar(SidecarConfig{
		ServerURL:             "http://127.0.0.1:1",
		Command:               []string{"sh", "-c", "sleep 60"},
		StartupTimeoutSeconds: 1,
	}, testLogger(t))

	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("expected a timeout")
	}
	if !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("error should name the timeout, got: %v", err)
	}
	// Start must not leave the process behind when it gives up.
	if s.cmd != nil && s.cmd.Process != nil {
		if err := syscall.Kill(s.cmd.Process.Pid, syscall.Signal(0)); err == nil {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
			t.Error("a sidecar that timed out should have been stopped")
		}
	}
}

const degradedBody = `{
  "status": "degraded",
  "degraded": ["fr"],
  "second_opinion": true,
  "second_opinion_ok": 12,
  "second_opinion_failed": 3,
  "models": {
    "en": {"id":"sfabriece/x","kind":"hub","available":true},
    "fr": {"id":"/Volumes/Crucial X8/m","kind":"path","available":false,
           "detail":"path not found -- an external drive may be unplugged"}
  }
}`

// The failure this reporting exists for: a model on an external drive goes away,
// transcription keeps working, and without this nothing anywhere says the station
// just lost its second opinion.
func TestStartReadsAndKeepsADegradedHealth(t *testing.T) {
	srv := healthServerSaying(t, degradedBody)
	s := NewSidecar(SidecarConfig{ServerURL: srv.URL, StartupTimeoutSeconds: 5}, testLogger(t))

	// A degraded sidecar must still start the server: the primary model answers.
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("a degraded sidecar must not stop startup: %v", err)
	}

	h := s.LastHealth()
	if h.Status != "degraded" {
		t.Errorf("status: got %q, want degraded", h.Status)
	}
	if len(h.Degraded) != 1 || h.Degraded[0] != "fr" {
		t.Errorf("degraded: got %v, want [fr]", h.Degraded)
	}
	if !h.SecondOpinion || h.SecondOpinionOK != 12 || h.SecondOpinionFailed != 3 {
		t.Errorf("counters not carried: %+v", h)
	}
	if m := h.Models["fr"]; m.Available || m.Detail == "" {
		t.Errorf("the unavailable model must carry its reason, got %+v", m)
	}
}

// Refresh failing is not the same as the sidecar being broken: it is busy
// decoding for seconds at a time. The last reading stands, and the caller is
// told the probe failed rather than being handed an invented outage.
func TestRefreshFailureKeepsTheLastReading(t *testing.T) {
	srv := healthServerSaying(t, degradedBody)
	s := NewSidecar(SidecarConfig{ServerURL: srv.URL, StartupTimeoutSeconds: 5}, testLogger(t))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	srv.Close() // the sidecar stops answering

	h, err := s.Refresh(context.Background())
	if err == nil {
		t.Error("Refresh should report that it could not reach the sidecar")
	}
	if h.Status != "degraded" || len(h.Degraded) != 1 {
		t.Errorf("the last known reading must survive a failed probe, got %+v", h)
	}
}

// How seventeen servers came to share one sidecar in a night: each spawned its
// own, each failed to bind the port, and each was reassured by the first one's
// answer to the health probe. A probe that succeeds says something is there --
// not that it is ours.
func TestRefusesToStartOnSomeoneElsesSidecar(t *testing.T) {
	// pid 1 is launchd: a pid that is certainly not the child we are about to
	// spawn. The real sidecar reports its own.
	srv := healthServerSaying(t, `{"status":"ok","pid":1}`)
	s := NewSidecar(SidecarConfig{
		ServerURL: srv.URL,
		// Our own child cannot bind, so it dies -- exactly what the sixteen did.
		Command:               []string{"sh", "-c", "echo 'address already in use' >&2; exit 1"},
		StartupTimeoutSeconds: 5,
	}, testLogger(t))

	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("starting on a sidecar we did not spawn must be refused")
	}
	for _, want := range []string{"not the sidecar we started", "command empty"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should say %q and what to do about it, got: %v", want, err)
		}
	}
	// And it should carry the child's own reason, which is the actionable part.
	if !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("the error should carry the child's output, got: %v", err)
	}
}

// The deliberate case stays allowed: no command means the operator runs the
// sidecar themselves, and a foreign one answering is the whole point.
func TestAForeignSidecarIsFineWhenNoCommandIsSet(t *testing.T) {
	srv := healthServer(t)
	s := NewSidecar(SidecarConfig{ServerURL: srv.URL, StartupTimeoutSeconds: 5}, testLogger(t))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("an empty command means using someone else's sidecar on purpose: %v", err)
	}
}
