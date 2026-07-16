package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	revision int64
	lastPDF  map[string]string
}

type ResumeBlock struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	File        string `json:"file"`
	StartLine   int    `json:"startLine"`
	EndLine     int    `json:"endLine"`
	Content     string `json:"content"`
	startOffset int
	endOffset   int
}

type ProjectState struct {
	Path     string        `json:"path"`
	Name     string        `json:"name"`
	MainFile string        `json:"mainFile"`
	Source   string        `json:"source"`
	Blocks   []ResumeBlock `json:"blocks"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type CompileResult struct {
	Revision    int64        `json:"revision"`
	Success     bool         `json:"success"`
	Stale       bool         `json:"stale"`
	Engine      string       `json:"engine"`
	DurationMS  int64        `json:"durationMs"`
	PDFBase64   string       `json:"pdfBase64,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func NewApp() *App { return &App{lastPDF: make(map[string]string)} }

func (a *App) OpenProject() (ProjectState, error) {
	dir, err := application.Get().Dialog.OpenFile().
		SetTitle("选择 LaTeX 简历项目").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil || dir == "" {
		return ProjectState{}, err
	}
	return a.LoadProject(dir)
}

func (a *App) LoadProject(dir string) (ProjectState, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ProjectState{}, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return ProjectState{}, fmt.Errorf("项目目录不存在: %s", abs)
	}
	mainFile, err := findMainTex(abs)
	if err != nil {
		return ProjectState{}, err
	}
	data, err := os.ReadFile(filepath.Join(abs, mainFile))
	if err != nil {
		return ProjectState{}, err
	}
	source := string(data)
	blocks, err := parseResumeBlocks(source, mainFile)
	if err != nil {
		return ProjectState{}, err
	}
	return ProjectState{Path: abs, Name: filepath.Base(abs), MainFile: mainFile, Source: source, Blocks: blocks}, nil
}

func (a *App) CreateDemoProject() (ProjectState, error) {
	dir := filepath.Join(os.TempDir(), "resume-studio-demo-v2")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ProjectState{}, err
	}
	path := filepath.Join(dir, "main.tex")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(demoTex), 0644); err != nil {
			return ProjectState{}, err
		}
	}
	return a.LoadProject(dir)
}

func (a *App) SaveBlock(projectPath, blockID, content string) (ProjectState, error) {
	state, err := a.LoadProject(projectPath)
	if err != nil {
		return ProjectState{}, err
	}
	var target *ResumeBlock
	for i := range state.Blocks {
		if state.Blocks[i].ID == blockID {
			target = &state.Blocks[i]
			break
		}
	}
	if target == nil {
		return ProjectState{}, fmt.Errorf("未找到段落 %q，请刷新项目", blockID)
	}
	clean := strings.Trim(content, "\r\n")
	updated := state.Source[:target.startOffset] + clean + state.Source[target.endOffset:]
	if err := atomicWrite(filepath.Join(projectPath, state.MainFile), []byte(updated)); err != nil {
		return ProjectState{}, err
	}
	return a.LoadProject(projectPath)
}

func (a *App) SaveSource(projectPath, source string) (ProjectState, error) {
	state, err := a.LoadProject(projectPath)
	if err != nil {
		return ProjectState{}, err
	}
	if _, err := parseResumeBlocks(source, state.MainFile); err != nil {
		return ProjectState{}, err
	}
	if err := atomicWrite(filepath.Join(projectPath, state.MainFile), []byte(source)); err != nil {
		return ProjectState{}, err
	}
	return a.LoadProject(projectPath)
}

func (a *App) Compile(projectPath string) CompileResult {
	state, err := a.LoadProject(projectPath)
	if err != nil {
		return CompileResult{Diagnostics: []Diagnostic{{Severity: "error", Message: err.Error()}}}
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.revision++
	revision := a.revision
	a.mu.Unlock()
	result := compileLatex(ctx, state, revision)
	a.mu.Lock()
	defer a.mu.Unlock()
	result.Stale = revision != a.revision
	if result.Success && !result.Stale {
		a.lastPDF[projectPath] = result.PDFBase64
	}
	if !result.Success {
		result.PDFBase64 = a.lastPDF[projectPath]
	}
	return result
}

func (a *App) ExportPDF(projectPath string) (string, error) {
	state, err := a.LoadProject(projectPath)
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	encoded := a.lastPDF[projectPath]
	a.mu.Unlock()
	if encoded == "" {
		return "", errors.New("尚无可导出的 PDF，请先成功编译一次")
	}

	filename := strings.TrimSuffix(state.MainFile, filepath.Ext(state.MainFile)) + ".pdf"
	dialog := application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:                "导出 PDF",
		Directory:            state.Path,
		Filename:             filename,
		ButtonText:           "导出",
		CanCreateDirectories: true,
		Filters: []application.FileFilter{
			{DisplayName: "PDF 文件", Pattern: "*.pdf"},
		},
	})
	destination, err := dialog.PromptForSingleSelection()
	if err != nil || destination == "" {
		return "", err
	}
	if filepath.Ext(destination) == "" {
		destination += ".pdf"
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("读取待导出的 PDF 失败: %w", err)
	}
	if err := os.WriteFile(destination, data, 0644); err != nil {
		return "", fmt.Errorf("导出 PDF 失败: %w", err)
	}
	return destination, nil
}

func (a *App) LocateBlock(projectPath, blockID string) (PDFLocation, error) {
	state, err := a.LoadProject(projectPath)
	if err != nil {
		return PDFLocation{}, err
	}
	for _, block := range state.Blocks {
		if block.ID == blockID {
			return locatePDFBlock(context.Background(), state, block)
		}
	}
	return PDFLocation{}, fmt.Errorf("未找到段落 %q", blockID)
}

func findMainTex(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "main.tex")); err == nil {
		return "main.tex", nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".tex") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return "", errors.New("目录中没有找到 .tex 文件")
	}
	return files[0], nil
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".resume-studio.tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	backup := path + ".resume-studio.bak"
	_ = os.Remove(backup)
	if err := os.Rename(path, backup); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Rename(backup, path)
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func encodePDF(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

const demoTex = `\documentclass[10pt,a4paper]{article}
\usepackage[margin=1.5cm]{geometry}
\usepackage{enumitem}
\usepackage[colorlinks=true,urlcolor=blue]{hyperref}
\setlength{\parindent}{0pt}
\begin{document}
{\LARGE \textbf{Lin Chen}} \hfill Backend Engineer\\
lin@example.com \quad github.com/linchen

\section*{Experience}
% @resume-block id=experience.cloud type=experience title="Cloud Platform"
\textbf{Cloud Service Management Platform} \hfill 2023--Present
\begin{itemize}[leftmargin=1.4em]
  \item Designed the order orchestration module with Go, MySQL and Redis.
  \item Reduced a ten-million-row query from 49 seconds to 0.9 seconds.
\end{itemize}
% @end-resume-block

\section*{Projects}
% @resume-block id=project.ai type=project title="AI Coding Assistant"
\textbf{Local AI Coding Assistant}
\begin{itemize}[leftmargin=1.4em]
  \item Built a context pipeline and tool execution layer for repository-aware coding.
\end{itemize}
% @end-resume-block

% @resume-block id=skills type=skills title="Skills"
\section*{Skills}
Go, Java, TypeScript, MySQL, Redis, Docker, Kubernetes
% @end-resume-block
\end{document}
`
