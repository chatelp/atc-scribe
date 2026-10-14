//go:build windows

package transcription

import "os/exec"

// Windows has no process groups in the POSIX sense and no SIGTERM. The child is
// started as is and stopped by killing that one process once Stop's delay has
// run out. Known limit: a grandchild -- a helper the sidecar forked -- is not
// reached and would outlive it.

func startInOwnGroup(cmd *exec.Cmd) {}

// terminateGroup has no graceful signal to send: os.Process.Signal implements
// only Kill on Windows. Stop waits out its delay, in case the child is already
// on its way out, then calls killGroup.
func terminateGroup(cmd *exec.Cmd) {}

func killGroup(cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
}

// inGroupOf cannot tell without process groups: only the child itself, matched
// by pid, is recognised as ours. A command that is a wrapper around the real
// sidecar is therefore refused on Windows.
func inGroupOf(pid, leader int) bool {
	return false
}
