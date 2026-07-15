package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type engineSpec struct {
	name, command string
	executable    string
	args          func(ProjectState, string) []string
}

func compileLatex(ctx context.Context, project ProjectState, revision int64) CompileResult {
	started := time.Now()
	result := CompileResult{Revision: revision, Diagnostics: []Diagnostic{}}
	outputDir := filepath.Join(project.Path, ".resume-studio", "build")
	_ = os.MkdirAll(outputDir, 0755)
	engines := []engineSpec{
		{name: "latexmk", command: "latexmk", args: func(p ProjectState, out string) []string {
			return []string{"-xelatex", "-synctex=1", "-interaction=nonstopmode", "-halt-on-error", "-file-line-error", "-outdir=" + out, p.MainFile}
		}},
		{name: "tectonic", command: "tectonic", args: func(p ProjectState, out string) []string {
			return []string{"--synctex", "--keep-logs", "-o", out, p.MainFile}
		}},
		{name: "xelatex", command: "xelatex", args: func(p ProjectState, out string) []string {
			return []string{"-synctex=1", "-interaction=nonstopmode", "-halt-on-error", "-file-line-error", "-output-directory=" + out, p.MainFile}
		}},
	}
	var chosen *engineSpec
	for i := range engines {
		if executable, ok := resolveCompiler(engines[i].command); ok {
			engines[i].executable = executable
			chosen = &engines[i]
			break
		}
	}
	if chosen == nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: "未检测到 LaTeX 编译器。请安装 latexmk + XeLaTeX，或 Tectonic，并加入 PATH。"})
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	result.Engine = chosen.name
	cmd := exec.CommandContext(ctx, chosen.executable, chosen.args(project, outputDir)...)
	configureCompilerCommand(cmd)
	cmd.Dir = project.Path
	cmd.Env = compilerEnvironment()
	output, err := cmd.CombinedOutput()
	result.DurationMS = time.Since(started).Milliseconds()
	if ctx.Err() != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "info", Message: "编译已被更新的版本取消"})
		return result
	}
	if err != nil {
		result.Diagnostics = parseCompileLog(string(output), project.MainFile)
		if len(result.Diagnostics) == 0 {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: tail(string(output), 1200)})
		}
		return result
	}
	pdf := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(project.MainFile), filepath.Ext(project.MainFile))+".pdf")
	encoded, err := encodePDF(pdf)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: fmt.Sprintf("读取 PDF 失败: %v", err)})
		return result
	}
	result.Success = true
	result.PDFBase64 = encoded
	return result
}

func resolveCompiler(command string) (string, bool) {
	if path, err := exec.LookPath(command); err == nil {
		return path, true
	}
	if command != "tectonic" {
		return "", false
	}
	name := "tectonic"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		base := filepath.Dir(executable)
		candidates = append(candidates, filepath.Join(base, "tools", name), filepath.Join(base, name))
	}
	if workingDir, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(workingDir, "tools", "tectonic", name), filepath.Join(workingDir, "tools", name))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

var logLine = regexp.MustCompile(`(?m)^([^:\r\n]+\.tex):(\d+):\s*(.+)$`)

func parseCompileLog(log, mainFile string) []Diagnostic {
	matches := logLine.FindAllStringSubmatch(log, -1)
	var result []Diagnostic
	for _, m := range matches {
		line := 0
		_, _ = fmt.Sscanf(m[2], "%d", &line)
		result = append(result, Diagnostic{Severity: "error", File: m[1], Line: line, Message: strings.TrimSpace(m[3])})
		if len(result) == 8 {
			break
		}
	}
	if len(result) == 0 && strings.Contains(log, "!") {
		result = append(result, Diagnostic{Severity: "error", File: mainFile, Message: tail(log, 1200)})
	}
	return result
}

func tail(value string, size int) string {
	if len(value) <= size {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(value[len(value)-size:])
}
