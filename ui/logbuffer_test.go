package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogBufferWriteSkipsEmptyAndTrailingNewline(t *testing.T) {
	b := &logBuffer{max: 10}
	n, err := b.Write([]byte("\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("Write n = %d, want 2", n)
	}
	if b.String() != "" {
		t.Fatalf("expected no lines, got %q", b.String())
	}
}

func TestLogBufferWriteSplitsLinesAndTrims(t *testing.T) {
	b := &logBuffer{max: 10}
	_, err := b.Write([]byte("a\nb\nc\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := b.String()
	want := "a\nb\nc"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestLogBufferWriteDropsOldestWhenOverMax(t *testing.T) {
	b := &logBuffer{max: 3}
	for i := range 5 {
		if _, err := fmt.Fprintf(b, "%d\n", i); err != nil {
			t.Fatal(err)
		}
	}
	lines := strings.Split(b.String(), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines kept, got %d: %q", len(lines), b.String())
	}
	if lines[0] != "2" || lines[1] != "3" || lines[2] != "4" {
		t.Fatalf("expected newest three lines 2, 3, 4, got %v", lines)
	}
}

func TestLogBufferStringJoinsLines(t *testing.T) {
	b := &logBuffer{max: 100}
	_, _ = b.Write([]byte("one\ntwo\n"))
	if s := b.String(); s != "one\ntwo" {
		t.Fatalf("got %q", s)
	}
}
