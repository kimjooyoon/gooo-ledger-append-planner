package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhysicalLinesCountsFinalUnterminatedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.go")
	if err := os.WriteFile(path, []byte("one\ntwo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := physicalLines(path); got != 2 {
		t.Fatalf("physicalLines() = %d, want 2", got)
	}
}

func TestPrecedenceIsFixed(t *testing.T) {
	want := []string{stateRefuted, stateUnknown, stateClosed}
	if !equalStringSlice(want, []string{"REFUTED", "UNKNOWN", "CLOSED"}) {
		t.Fatal("status precedence changed")
	}
}
