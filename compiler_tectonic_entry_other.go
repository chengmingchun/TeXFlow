//go:build !windows

package main

func prepareTectonicProject(project ProjectState, _ int64) (ProjectState, func(), error) {
	return project, func() {}, nil
}

func promoteTectonicArtifacts(_, _, _ string, _ bool) {}
