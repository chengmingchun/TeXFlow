//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureTectonicEnvironmentCreatesFontconfig(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "build")
	env, err := configureTectonicEnvironment([]string{"FONTCONFIG_FILE=old"}, root, out)
	if err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(out, "fontconfig", "fonts.conf")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	if !strings.Contains(config, "<fontconfig>") || !strings.Contains(config, "<cachedir>") {
		t.Fatalf("invalid fontconfig document: %s", config)
	}

	var values []string
	for _, item := range env {
		if strings.HasPrefix(item, "FONTCONFIG_FILE=") {
			values = append(values, item)
		}
	}
	if len(values) != 1 || values[0] != "FONTCONFIG_FILE="+configPath {
		t.Fatalf("unexpected FONTCONFIG_FILE values: %v", values)
	}
	if !environmentContainsKey(env, "OSFONTDIR") {
		t.Fatalf("OSFONTDIR was not configured: %v", env)
	}
}

func environmentContainsKey(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func TestFontconfigDocumentEscapesPaths(t *testing.T) {
	config := fontconfigDocument([]string{`C:\fonts & assets`}, `C:\cache`)
	if !strings.Contains(config, "C:/fonts &amp; assets") {
		t.Fatalf("font path was not escaped: %s", config)
	}
}
