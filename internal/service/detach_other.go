//go:build !unix

package service

import "os/exec"

func detach(*exec.Cmd) {}
