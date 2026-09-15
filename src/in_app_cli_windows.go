//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func hideCLIWindow(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
