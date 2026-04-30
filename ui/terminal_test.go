package ui

import (
	"slices"
	"strings"
	"testing"
)

func TestTerminalArgvDarwin(t *testing.T) {
	const abs = "/tmp/repo"
	tests := []struct {
		term string
		want []string
	}{
		{"Apple_Terminal", []string{"open", "-a", "Terminal", abs}},
		{"iTerm.app", []string{"open", "-a", "iTerm", abs}},
		{"WarpTerminal", []string{"open", "-a", "Warp", abs}},
		{"WezTerm", []string{"wezterm", "start", "--cwd", abs}},
		{"kitty", []string{"kitty", "--directory", abs}},
	}
	for _, tt := range tests {
		name, args := terminalArgv(abs, tt.term, "darwin")
		got := append([]string{name}, args...)
		if !slices.Equal(got, tt.want) {
			t.Errorf("TERM_PROGRAM=%q: got %v want %v", tt.term, got, tt.want)
		}
	}
	name, args := terminalArgv(abs, "unknown-term", "darwin")
	if name != "open" || !slices.Equal(args, []string{"-a", "Terminal", abs}) {
		t.Errorf("unknown darwin TERM_PROGRAM: got %q %v", name, args)
	}
}

func TestAppleScriptQuotedString(t *testing.T) {
	if got := appleScriptQuotedString(`/tmp/a"b\c`); got != `"/tmp/a\"b\\c"` {
		t.Fatalf("got %s", got)
	}
}

func TestGhosttyDarwinNewTabAppleScript(t *testing.T) {
	const abs = "/Users/dev/my repo"
	script := ghosttyDarwinNewTabAppleScript(abs)
	if !strings.Contains(script, `tell application "Ghostty"`) {
		t.Fatalf("missing tell Ghostty")
	}
	if !strings.Contains(script, "new tab in front window with configuration cfg") {
		t.Fatalf("expected new tab with configuration")
	}
	if !strings.Contains(script, "new window with configuration cfg") {
		t.Fatalf("expected new window fallback in script")
	}
	if !strings.Contains(script, `"/Users/dev/my repo"`) {
		t.Fatalf("expected quoted path, got substrings of: %s", script)
	}
}

func TestTerminalArgvUnixExplicitTerms(t *testing.T) {
	const abs = "/srv/git/x"
	name, args := terminalArgv(abs, "WezTerm", "linux")
	if name != "wezterm" || !slices.Equal(args, []string{"start", "--cwd", abs}) {
		t.Fatalf("WezTerm linux: got %q %v", name, args)
	}
	name, args = terminalArgv(abs, "kitty", "linux")
	if name != "kitty" || !slices.Equal(args, []string{"--directory", abs}) {
		t.Fatalf("kitty linux: got %q %v", name, args)
	}
	// Linux uses the ghostty binary directly (unlike macOS `open`).
	name, args = terminalArgv(abs, "com.mitchellh.ghostty", "linux")
	if name != "ghostty" || !slices.Equal(args, []string{"+new-window", "--working-directory=" + abs}) {
		t.Fatalf("Ghostty linux: got %q %v", name, args)
	}
}

func TestBatchQuoteArg(t *testing.T) {
	if got := batchQuoteArg(`C:\a b\c`); got != `"C:\a b\c"` {
		t.Fatalf("quoted path: %q", got)
	}
	if got := batchQuoteArg(`C:\nospaces`); got != `C:\nospaces` {
		t.Fatalf("unquoted: %q", got)
	}
}
