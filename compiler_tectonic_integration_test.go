package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTectonicCompilesCtexDocument(t *testing.T) {
	if os.Getenv("TEXFLOW_RUN_TECTONIC_TEST") == "" {
		t.Skip("set TEXFLOW_RUN_TECTONIC_TEST=1 to run the bundled Tectonic integration test")
	}
	root := t.TempDir()
	source := `\documentclass[UTF8,10pt,a4paper]{ctexart}
\begin{document}
TeXFlow
\end{document}
`
	if fixture := os.Getenv("TEXFLOW_CTEXT_FIXTURE"); fixture != "" {
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		source = string(data)
	}
	if err := os.WriteFile(filepath.Join(root, "main.tex"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	result := compileLatex(context.Background(), ProjectState{Path: root, MainFile: "main.tex", Source: source}, 1)
	if !result.Success {
		t.Fatalf("ctex compilation failed with %s: %+v", result.Engine, result.Diagnostics)
	}
}

func TestIsCompletePDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "complete.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.5\nbody\nstartxref\n4\n%%EOF\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !isCompletePDF(path) {
		t.Fatal("expected a structurally complete PDF")
	}
	if err := os.WriteFile(path, []byte("%PDF-1.5\ntruncated"), 0644); err != nil {
		t.Fatal(err)
	}
	if isCompletePDF(path) {
		t.Fatal("a truncated PDF must not be accepted")
	}
}
