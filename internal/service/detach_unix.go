//go:build unix

package service

import (
	"os/exec"
	"syscall"
)

// detach starts cmd in a session of its own, so that it outlives the
// terminal that started it.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
