package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndReplaceBlock(t *testing.T) {
	source := "before\n% @resume-block id=job.api type=experience title=\"API\"\nold\n% @end-resume-block\nafter\n"
	blocks, err := parseResumeBlocks(source, "main.tex")
	if err != nil || len(blocks) != 1 {
		t.Fatalf("parse failed: %v %#v", err, blocks)
	}
	b := blocks[0]
	if b.ID != "job.api" || b.Content != "old" || b.StartLine != 3 {
		t.Fatalf("unexpected block: %#v", b)
	}
	updated := source[:b.startOffset] + "new" + source[b.endOffset:]
	if updated != "before\n% @resume-block id=job.api type=experience title=\"API\"\nnew\n% @end-resume-block\nafter\n" {
		t.Fatalf("replace mismatch: %q", updated)
	}
}

func TestRejectsInvalidMarkers(t *testing.T) {
	_, err := parseResumeBlocks("% @resume-block id=x\nbody", "main.tex")
	if err == nil {
		t.Fatal("expected missing end marker error")
	}
}

func TestSaveBlockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	source := "% @resume-block id=summary type=summary\nold\n% @end-resume-block\n"
	if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	state, err := app.SaveBlock(dir, "summary", "new content")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Blocks) != 1 || state.Blocks[0].Content != "new content" {
		t.Fatalf("unexpected state: %#v", state)
	}
}
