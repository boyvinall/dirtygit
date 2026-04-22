package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePorcelainStatus(t *testing.T) {
	input := strings.Join([]string{
		" M scanner/scan.go",
		"R  old/name.go -> new/name.go",
		"?? scanner/scan_test.go",
	}, "\n")

	st, err := ParsePorcelainStatus(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePorcelainStatus() error = %v", err)
	}

	if len(st.Entries) != 3 {
		t.Fatalf("ParsePorcelainStatus() entries = %d, want 3", len(st.Entries))
	}

	if st.Entries[0].Staging != ' ' || st.Entries[0].Worktree != 'M' || st.Entries[0].Path != "scanner/scan.go" {
		t.Fatalf("unexpected first entry: %+v", st.Entries[0])
	}

	if st.Entries[1].Staging != 'R' || st.Entries[1].Path != "new/name.go" || st.Entries[1].OriginalPath != "old/name.go" {
		t.Fatalf("unexpected rename entry: %+v", st.Entries[1])
	}

	if st.Entries[2].Staging != '?' || st.Entries[2].Worktree != '?' || st.Entries[2].Path != "scanner/scan_test.go" {
		t.Fatalf("unexpected untracked entry: %+v", st.Entries[2])
	}
}

func TestParsePorcelainStatusRejectsMalformedLine(t *testing.T) {
	_, err := ParsePorcelainStatus(strings.NewReader("bad"))
	if err == nil {
		t.Fatal("ParsePorcelainStatus() expected error for malformed line")
	}
}

func TestPorcelainStatusToGitStatus(t *testing.T) {
	st := PorcelainStatus{
		Entries: []PorcelainEntry{
			{Staging: 'M', Worktree: ' ', Path: "a.go"},
			{Staging: ' ', Worktree: 'D', Path: "b.go"},
		},
	}

	got := st.ToGitStatus()
	if len(got) != 2 {
		t.Fatalf("ToGitStatus() len = %d, want 2", len(got))
	}
	if got["a.go"].Staging != 'M' || got["a.go"].Worktree != ' ' {
		t.Fatalf("unexpected status for a.go: %+v", got["a.go"])
	}
	if got["b.go"].Staging != ' ' || got["b.go"].Worktree != 'D' {
		t.Fatalf("unexpected status for b.go: %+v", got["b.go"])
	}
}

func TestParseConfigFileFallsBackToDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "missing.yml")
	defaultCfg := `
scandirs:
  include:
    - /tmp
  exclude:
    - /tmp/nope
gitignore:
  fileglob:
    - "*.log"
  dirglob:
    - node_modules
followsymlinks: true
`

	cfg, err := ParseConfigFile(cfgPath, defaultCfg)
	if err != nil {
		t.Fatalf("ParseConfigFile() error = %v", err)
	}
	if len(cfg.ScanDirs.Include) != 1 || cfg.ScanDirs.Include[0] != "/tmp" {
		t.Fatalf("unexpected include dirs: %+v", cfg.ScanDirs.Include)
	}
	if !cfg.FollowSymlinks {
		t.Fatal("expected followsymlinks=true from default config")
	}
}

func TestParseConfigFileReadsFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	content := `
scandirs:
  include:
    - /opt/repos
gitignore:
  fileglob:
    - "*.tmp"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseConfigFile(cfgPath, "scandirs: {include: [/fallback]}")
	if err != nil {
		t.Fatalf("ParseConfigFile() error = %v", err)
	}
	if len(cfg.ScanDirs.Include) != 1 || cfg.ScanDirs.Include[0] != "/opt/repos" {
		t.Fatalf("unexpected include dirs: %+v", cfg.ScanDirs.Include)
	}
}
