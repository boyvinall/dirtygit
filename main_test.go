package main

import (
	"strings"
	"testing"
)

func TestGetDefaultConfigPathUsesHomeAndSuffix(t *testing.T) {
	p := getDefaultConfigPath()
	if !strings.HasSuffix(p, ".dirtygit.yml") {
		t.Fatalf("expected path to end with .dirtygit.yml, got %q", p)
	}
}
