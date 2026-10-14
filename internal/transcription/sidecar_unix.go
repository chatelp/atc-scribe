//go:build !windows

package transcription

import (
	"os/exec"
	"syscall"
)

// startInOwnGroup puts the child in a process group of its own, so Stop can
// reach the children too.
func startInOwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateGroup asks the whole process group to stop. The sidecar's
// dependencies fork helpers of their own -- Silero leaves a multiprocessing
// resource_tracker behind -- and signalling only the parent orphans them.
func terminateGroup(cmd *exec.Cmd) {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
}

// killGroup kills the whole process group.
func killGroup(cmd *exec.Cmd) {
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// inGroupOf reports whether pid belongs to the process group led by leader,
// which every descendant of the child we started shares.
func inGroupOf(pid, leader int) bool {
	pgid, err := syscall.Getpgid(pid)
	return err == nil && pgid == leader
}
