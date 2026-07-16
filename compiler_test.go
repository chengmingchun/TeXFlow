package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompilerPrefersXeLaTeX(t *testing.T) {
	engines := compilerEngines()
	if len(engines) == 0 || engines[0].name != "xelatex" {
		t.Fatalf("expected xelatex to be preferred, got %#v", engines)
	}
}

func TestResolveBundledTectonic(t *testing.T) {
	path, ok := resolveCompiler("tectonic")
	if !ok {
		t.Fatal("expected bundled Tectonic to be detected")
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("invalid compiler path %q: %v", path, err)
	}
	if filepath.Base(path) != "tectonic.exe" {
		t.Fatalf("unexpected compiler: %s", path)
	}
}
