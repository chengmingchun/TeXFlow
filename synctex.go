package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type PDFLocation struct {
	Page   int     `json:"page"`
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type syncTeXPoint struct {
	page          int
	x, y, h, v, w float64
	height        float64
}

func locatePDFBlock(ctx context.Context, project ProjectState, block ResumeBlock) (PDFLocation, error) {
	syncTeX, ok := resolveSyncTeX()
	if !ok {
		return PDFLocation{}, errors.New("未检测到 SyncTeX，无法精确定位 PDF 段落")
	}

	base := strings.TrimSuffix(filepath.Base(project.MainFile), filepath.Ext(project.MainFile))
	pdf := filepath.Join(project.Path, ".resume-studio", "build", base+".pdf")
	if _, err := os.Stat(pdf); err != nil {
		return PDFLocation{}, fmt.Errorf("PDF 尚未生成: %w", err)
	}

	source := filepath.Join(project.Path, block.File)
	start, err := querySyncTeX(ctx, syncTeX, source, pdf, block.StartLine)
	if err != nil {
		return PDFLocation{}, err
	}
	end, endErr := querySyncTeX(ctx, syncTeX, source, pdf, block.EndLine)
	if endErr != nil || end.page != start.page {
		end = start
	}

	top := math.Min(start.v, end.v)
	bottom := math.Max(start.v+start.height, end.v+end.height)
	height := math.Max(24, bottom-top)
	left := math.Min(start.h, end.h)
	right := math.Max(start.h+start.w, end.h+end.w)
	return PDFLocation{
		Page:   start.page,
		Left:   math.Max(0, left),
		Top:    math.Max(0, top),
		Width:  math.Max(48, right-left),
		Height: height,
	}, nil
}

func querySyncTeX(ctx context.Context, executable, source, pdf string, line int) (syncTeXPoint, error) {
	input := fmt.Sprintf("%d:1:%s", line, source)
	cmd := exec.CommandContext(ctx, executable, "view", "-i", input, "-o", pdf)
	configureCompilerCommand(cmd)
	cmd.Dir = filepath.Dir(source)
	cmd.Env = compilerEnvironment()
	output, err := cmd.CombinedOutput()
	point, parseErr := parseSyncTeXView(string(output))
	if parseErr != nil {
		if err != nil {
			return syncTeXPoint{}, fmt.Errorf("SyncTeX 定位失败: %w", err)
		}
		return syncTeXPoint{}, parseErr
	}
	return point, nil
}

func parseSyncTeXView(output string) (syncTeXPoint, error) {
	point := syncTeXPoint{}
	seenPage := false
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		if key == "Page" {
			if seenPage {
				break
			}
			page, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return syncTeXPoint{}, fmt.Errorf("无效的 SyncTeX 页码: %q", value)
			}
			point.page = page
			seenPage = true
			continue
		}
		if !seenPage {
			continue
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			continue
		}
		switch key {
		case "x":
			point.x = number
		case "y":
			point.y = number
		case "h":
			point.h = number
		case "v":
			point.v = number
		case "W":
			point.w = number
		case "H":
			point.height = number
		}
	}
	if point.page < 1 {
		return syncTeXPoint{}, errors.New("SyncTeX 未返回可用的 PDF 坐标")
	}
	if point.h == 0 {
		point.h = point.x
	}
	if point.v == 0 {
		point.v = point.y
	}
	return point, nil
}

func resolveSyncTeX() (string, bool) {
	name := "synctex"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, true
	}
	if xelatex, ok := resolveCompiler("xelatex"); ok {
		candidate := filepath.Join(filepath.Dir(xelatex), name)
		if isExecutableFile(candidate) {
			return candidate, true
		}
	}

	var candidates []string
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, filepath.Join(localAppData, "Programs", "MiKTeX", "miktex", "bin", "x64", name))
	}
	if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
		candidates = append(candidates, filepath.Join(programFiles, "MiKTeX", "miktex", "bin", "x64", name))
		matches, _ := filepath.Glob(filepath.Join(programFiles, "texlive", "*", "bin", "windows", name))
		candidates = append(candidates, matches...)
	}
	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
