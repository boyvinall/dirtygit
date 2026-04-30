package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// openTerminalInCurrentRepo starts a new terminal window (or tab-capable equivalent)
// with the working directory set to the selected repository. The parent terminal is
// inferred from TERM_PROGRAM when possible; otherwise a platform default is used.
func (m *model) openTerminalInCurrentRepo() {
	repo := m.currentRepo()
	if repo == "" {
		return
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		log.Printf("terminal: %v", err)
		return
	}
	termProg := os.Getenv("TERM_PROGRAM")

	// Ghostty on macOS: prefer AppleScript (PR #11208) so we open a new tab in the
	// front window with cwd set; fall back to CLI new-window if osascript fails.
	if runtime.GOOS == "darwin" && isGhosttyTerminal(termProg) {
		if err := runDarwinGhosttyNewTabAppleScript(abs); err != nil {
			log.Printf("terminal: Ghostty AppleScript: %v", err)
			cmd := exec.Command("open", "-na", "Ghostty.app", "--args", "+new-window", "--working-directory="+abs)
			if err := cmd.Start(); err != nil {
				log.Printf("terminal: Ghostty CLI fallback: %v", err)
			}
		}
		return
	}

	name, args := terminalArgv(abs, termProg, runtime.GOOS)
	if name == "" {
		log.Printf("terminal: no launcher (TERM_PROGRAM=%q GOOS=%s)", termProg, runtime.GOOS)
		return
	}
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		log.Printf("terminal %q: %v", name, err)
	}
}

// appleScriptQuotedString returns an AppleScript double-quoted string literal for text.
func appleScriptQuotedString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ghosttyDarwinNewTabAppleScript matches Ghostty's AppleScript API (merged in ghostty#11208):
// new tab in front window with configuration, using surface configuration's initial working directory.
// If there is no front window, try/new window covers first launch.
func ghosttyDarwinNewTabAppleScript(absPath string) string {
	q := appleScriptQuotedString(absPath)
	return fmt.Sprintf(`tell application "Ghostty"
  activate
  set cfg to new surface configuration
  set initial working directory of cfg to %s
  try
    new tab in front window with configuration cfg
  on error
    new window with configuration cfg
  end try
end tell`, q)
}

func runDarwinGhosttyNewTabAppleScript(abs string) error {
	script := ghosttyDarwinNewTabAppleScript(abs)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// terminalArgv returns argv for launching a terminal at abs. The first return value is
// the executable name or path; it is empty when no strategy applies.
func terminalArgv(abs, termProg, goos string) (string, []string) {
	switch goos {
	case "darwin":
		return darwinTerminalArgv(abs, termProg)
	case "windows":
		return windowsTerminalArgv(abs, termProg)
	default:
		return unixTerminalArgv(abs, termProg)
	}
}

// isGhosttyTerminal reports whether TERM_PROGRAM refers to Ghostty. Values differ by
// platform and version (e.g. "ghostty", "com.mitchellh.ghostty").
func isGhosttyTerminal(termProg string) bool {
	switch termProg {
	case "com.mitchellh.ghostty", "ghostty", "Ghostty":
		return true
	default:
		return strings.Contains(strings.ToLower(termProg), "ghostty")
	}
}

func darwinTerminalArgv(abs, termProg string) (string, []string) {
	switch termProg {
	case "Apple_Terminal":
		return "open", []string{"-a", "Terminal", abs}
	case "iTerm.app":
		return "open", []string{"-a", "iTerm", abs}
	case "WarpTerminal":
		return "open", []string{"-a", "Warp", abs}
	case "WezTerm":
		return "wezterm", []string{"start", "--cwd", abs}
	case "kitty":
		return "kitty", []string{"--directory", abs}
	default:
		// Default: new Terminal.app window with cwd set to abs (path argument).
		return "open", []string{"-a", "Terminal", abs}
	}
}

func unixTerminalArgv(abs, termProg string) (string, []string) {
	switch termProg {
	case "WezTerm":
		return "wezterm", []string{"start", "--cwd", abs}
	case "kitty":
		return "kitty", []string{"--directory", abs}
	case "com.mitchellh.ghostty":
		return "ghostty", []string{"+new-window", "--working-directory=" + abs}
	}
	if strings.Contains(strings.ToLower(termProg), "ghostty") {
		return "ghostty", []string{"+new-window", "--working-directory=" + abs}
	}

	try := []struct {
		exe  string
		args []string
	}{
		{"gnome-terminal", []string{"--working-directory", abs}},
		{"konsole", []string{"--workdir", abs}},
		{"xfce4-terminal", []string{"--working-directory", abs}},
		{"alacritty", []string{"--working-directory", abs}},
		{"kitty", []string{"--directory", abs}},
		{"wezterm", []string{"start", "--cwd", abs}},
		{"x-terminal-emulator", []string{"--working-directory", abs}},
	}
	for _, t := range try {
		if path, err := exec.LookPath(t.exe); err == nil {
			return path, t.args
		}
	}
	return "", nil
}

func windowsTerminalArgv(abs string, _ string) (string, []string) {
	if _, err := exec.LookPath("wt"); err == nil {
		return "wt", []string{"-d", abs}
	}
	// New console window: cd then interactive shell.
	q := batchQuoteArg(abs)
	return "cmd", []string{"/c", "start", "cmd", "/k", "cd /d " + q + " && title dirtygit"}
}

// batchQuoteArg quotes a path for cmd.exe /k when it contains spaces or special chars.
func batchQuoteArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"&<>|^") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
