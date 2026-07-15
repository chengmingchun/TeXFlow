//go:build !windows

package main

func configureTectonicEnvironment(env []string, _, _ string) ([]string, error) {
	return env, nil
}
