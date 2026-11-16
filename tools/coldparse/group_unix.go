//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// ownGroup starts the server in a process group of its own, so killGroup stops a wrapper and the
// server under it together.
func ownGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func killGroup(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
