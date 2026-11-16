//go:build windows

package main

import "os/exec"

// Windows has no process groups to signal, so the server is killed alone (agni issue 1595). A
// -wrap command's child may outlive it there.
func ownGroup(*exec.Cmd) {}

func killGroup(cmd *exec.Cmd) { _ = cmd.Process.Kill() }
