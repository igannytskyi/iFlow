//go:build !unix

package tui

import "os/exec"

func detach(*exec.Cmd) {}
