//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareTectonicProjectInjectsWindowsCtexFontset(t *testing.T) {
	root := t.TempDir()
	project := ProjectState{
		Path:     root,
		MainFile: "main.tex",
		Source:   `\documentclass[UTF8,10pt]{ctexart}\begin{document}OK\end{document}`,
	}
	prepared, cleanup, err := prepareTectonicProject(project, 42)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.MainFile == project.MainFile {
		t.Fatal("expected a temporary Tectonic entry file")
	}
	wrapperPath := filepath.Join(root, prepared.MainFile)
	data, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `\PassOptionsToClass{fontset=windows}{ctexart}`) {
		t.Fatalf("fontset was not injected: %s", data)
	}
	cleanup()
	if _, err := os.Stat(wrapperPath); !os.IsNotExist(err) {
		t.Fatalf("temporary entry was not removed: %v", err)
	}
}

func TestPrepareTectonicProjectPreservesExplicitFontset(t *testing.T) {
	project := ProjectState{
		Path:     t.TempDir(),
		MainFile: "main.tex",
		Source:   `\documentclass[fontset=ubuntu]{ctexart}`,
	}
	prepared, cleanup, err := prepareTectonicProject(project, 1)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if prepared.MainFile != project.MainFile {
		t.Fatal("an explicit user fontset must not be overridden")
	}
}

func TestPrepareTectonicProjectSupportsCtexPackage(t *testing.T) {
	project := ProjectState{
		Path:     t.TempDir(),
		MainFile: "main.tex",
		Source:   `\documentclass{article}\usepackage[UTF8]{ctex}`,
	}
	prepared, cleanup, err := prepareTectonicProject(project, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(filepath.Join(project.Path, prepared.MainFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `\PassOptionsToPackage{fontset=windows}{ctex}`) {
		t.Fatalf("package fontset was not injected: %s", data)
	}
}
