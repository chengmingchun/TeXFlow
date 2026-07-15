//go:build !windows

package main

import "os"

func compilerEnvironment() []string { return os.Environ() }
