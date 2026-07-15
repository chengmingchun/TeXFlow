package main

import "testing"

func TestIsCtexSource(t *testing.T) {
	for _, source := range []string{
		`\documentclass{ctexart}`,
		`\documentclass{article}\usepackage{ctex}`,
	} {
		if !isCtexSource(source) {
			t.Fatalf("expected ctex source to be detected: %s", source)
		}
	}
	if isCtexSource(`\documentclass{article}`) {
		t.Fatal("plain article must not be classified as ctex")
	}
}
