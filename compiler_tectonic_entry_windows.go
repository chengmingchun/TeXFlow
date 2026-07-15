//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ctexClassPattern   = regexp.MustCompile(`(?s)\\documentclass\s*(?:\[([^]]*)\])?\s*\{(ctex(?:art|rep|book|beamer))\}`)
	ctexPackagePattern = regexp.MustCompile(`(?s)\\usepackage\s*(?:\[([^]]*)\])?\s*\{ctex\}`)
	ctexFontsetPattern = regexp.MustCompile(`(?i)(?:fontset\s*=|winfonts|adobefonts|nofonts)`)
)

func prepareTectonicProject(project ProjectState, revision int64) (ProjectState, func(), error) {
	kind, name, explicit := detectCtexDocument(project.Source)
	if kind == "" || explicit {
		return project, func() {}, nil
	}

	wrapperName := fmt.Sprintf("resume-studio-compile-%d.tex", revision)
	wrapperPath := filepath.Join(project.Path, wrapperName)
	optionTarget := "Package"
	if kind == "class" {
		optionTarget = "Class"
	}
	wrapper := fmt.Sprintf("\\PassOptionsTo%s{fontset=windows}{%s}\n\\input{\\detokenize{%s}}\n", optionTarget, name, filepath.ToSlash(project.MainFile))
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0644); err != nil {
		return project, func() {}, err
	}

	prepared := project
	prepared.MainFile = wrapperName
	return prepared, func() { _ = os.Remove(wrapperPath) }, nil
}

func detectCtexDocument(source string) (kind, name string, explicit bool) {
	if match := ctexClassPattern.FindStringSubmatch(source); len(match) > 0 {
		return "class", match[2], ctexFontsetPattern.MatchString(match[1])
	}
	if match := ctexPackagePattern.FindStringSubmatch(source); len(match) > 0 {
		return "package", "ctex", ctexFontsetPattern.MatchString(match[1])
	}
	return "", "", false
}

func promoteTectonicArtifacts(outputDir, generatedMain, originalMain string, success bool) {
	generatedStem := strings.TrimSuffix(filepath.Base(generatedMain), filepath.Ext(generatedMain))
	originalStem := strings.TrimSuffix(filepath.Base(originalMain), filepath.Ext(originalMain))
	extensions := []string{".log"}
	if success {
		extensions = append(extensions, ".pdf", ".synctex.gz")
	}
	for _, extension := range extensions {
		from := filepath.Join(outputDir, generatedStem+extension)
		to := filepath.Join(outputDir, originalStem+extension)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		_ = os.Remove(to)
		_ = os.Rename(from, to)
	}
}
