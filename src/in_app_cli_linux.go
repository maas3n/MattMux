//go:build linux

package main

import "os/exec"

func hideCLIWindow(cmd *exec.Cmd) {}
