//go:build windows

package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configureTectonicEnvironment(env []string, projectPath, outputDir string) ([]string, error) {
	configDir := filepath.Join(outputDir, "fontconfig")
	cacheDir := filepath.Join(configDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return env, err
	}

	fontDirs := existingFontDirs(tectonicFontDirectories(projectPath))
	configPath := filepath.Join(configDir, "fonts.conf")
	if err := os.WriteFile(configPath, []byte(fontconfigDocument(fontDirs, cacheDir)), 0644); err != nil {
		return env, err
	}

	env = setEnvironmentValue(env, "FONTCONFIG_FILE", configPath)
	env = setEnvironmentValue(env, "FONTCONFIG_PATH", configDir)
	if len(fontDirs) > 0 {
		searchPaths := make([]string, 0, len(fontDirs))
		for _, dir := range fontDirs {
			searchPaths = append(searchPaths, filepath.ToSlash(dir)+"//")
		}
		env = setEnvironmentValue(env, "OSFONTDIR", strings.Join(searchPaths, ";"))
	}
	return env, nil
}

func tectonicFontDirectories(projectPath string) []string {
	var dirs []string
	if windowsDir := os.Getenv("WINDIR"); windowsDir != "" {
		dirs = append(dirs, filepath.Join(windowsDir, "Fonts"))
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		dirs = append(dirs, filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"))
	}
	if cacheDir, err := os.UserCacheDir(); err == nil {
		bundleData := filepath.Join(cacheDir, "TectonicProject", "Tectonic", "bundles", "data")
		dirs = append(dirs, bundleData)
		if entries, err := os.ReadDir(bundleData); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					dirs = append(dirs, filepath.Join(bundleData, entry.Name()))
				}
			}
		}
	}
	if executable, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(executable), "tools", "fonts"))
	}
	dirs = append(dirs,
		filepath.Join(projectPath, "fonts"),
		filepath.Join(projectPath, "assets", "fonts"),
	)
	return dirs
}

func existingFontDirs(candidates []string) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		key := strings.ToLower(filepath.Clean(absolute))
		if seen[key] {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && info.IsDir() {
			seen[key] = true
			dirs = append(dirs, absolute)
		}
	}
	return dirs
}

func fontconfigDocument(fontDirs []string, cacheDir string) string {
	var builder strings.Builder
	builder.WriteString("<?xml version=\"1.0\"?>\n")
	builder.WriteString("<!DOCTYPE fontconfig SYSTEM \"urn:fontconfig:fonts.dtd\">\n")
	builder.WriteString("<fontconfig>\n")
	for _, dir := range fontDirs {
		builder.WriteString("  <dir>")
		builder.WriteString(escapeFontconfigPath(dir))
		builder.WriteString("</dir>\n")
	}
	builder.WriteString("  <cachedir>")
	builder.WriteString(escapeFontconfigPath(cacheDir))
	builder.WriteString("</cachedir>\n")
	builder.WriteString("</fontconfig>\n")
	return builder.String()
}

func escapeFontconfigPath(path string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(filepath.ToSlash(path)))
	return escaped.String()
}

func setEnvironmentValue(env []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(strings.ToUpper(item), prefix) {
			continue
		}
		result = append(result, item)
	}
	return append(result, fmt.Sprintf("%s=%s", key, value))
}
