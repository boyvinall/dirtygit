package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boyvinall/dirtygit/scanner"
)

func TestNewScanSpinnerNonNilView(t *testing.T) {
	s := newScanSpinner()
	v := s.View()
	if v == "" {
		t.Fatal("expected spinner view to be non-empty")
	}
}

func TestTickCmdReturnsTickMsgAfterDelay(t *testing.T) {
	cmd := tickCmd()
	if cmd == nil {
		t.Fatal("tickCmd returned nil")
	}
	done := make(chan tea.Msg, 1)
	go func() {
		done <- cmd()
	}()
	select {
	case msg := <-done:
		if _, ok := msg.(tickMsg); !ok {
			t.Fatalf("expected tickMsg, got %T", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tick (expected ~100ms)")
	}
}

func TestBeginScanNoOpWhenAlreadyScanning(t *testing.T) {
	m := newTestModel()
	m.config = &scanner.Config{}
	m.scanResultCh = make(chan scanResult, 1)
	m.scanning = true
	if cmd := m.beginScan(); cmd != nil {
		t.Fatalf("beginScan while already scanning should return nil, got %#v", cmd)
	}
}
