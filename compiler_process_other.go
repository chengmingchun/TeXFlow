//go:build !windows

package main

import "os/exec"

func configureCompilerCommand(_ *exec.Cmd) {}
