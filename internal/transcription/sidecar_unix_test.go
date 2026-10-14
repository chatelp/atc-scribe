//go:build !windows

package transcription

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These tests check a process's death with signal 0 and reach its group by a
// negative pid, neither of which exists on Windows; and the grandchild test
// holds a guarantee the Windows path does not give (sidecar_windows.go).

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

// notOurs accepts a sidecar that answers from a descendant of our child, as when
// the command is a wrapper script: the descendant shares the group set at spawn.
func TestInGroupOfRecognisesTheChildsDescendants(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	cmd := exec.Command("sh", "-c", "sleep 60 & echo $! > "+pidFile+"; wait")
	startInOwnGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		killGroup(cmd)
		_ = cmd.Wait()
	}()

	var grandchild int
	for i := 0; i < 50 && grandchild == 0; i++ {
		if b, err := os.ReadFile(pidFile); err == nil {
			grandchild, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		time.Sleep(50 * time.Millisecond)
	}
	if grandchild == 0 {
		t.Fatal("grandchild never reported its pid")
	}
	if !inGroupOf(grandchild, cmd.Process.Pid) {
		t.Errorf("grandchild %d should be in the group of %d", grandchild, cmd.Process.Pid)
	}
	if inGroupOf(os.Getpid(), cmd.Process.Pid) {
		t.Errorf("the test process is not in the child's group")
	}
}
