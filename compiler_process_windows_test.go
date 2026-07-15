//go:build windows

package main

import (
	"os/exec"
	"testing"
)

func TestCompilerProcessIsHidden(t *testing.T) {
	command := exec.Command("cmd.exe", "/c", "exit", "0")
	configureCompilerCommand(command)
	if command.SysProcAttr == nil {
		t.Fatal("expected Windows process attributes")
	}
	if !command.SysProcAttr.HideWindow {
		t.Fatal("compiler process must hide its console window")
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("compiler process must use CREATE_NO_WINDOW")
	}
}
