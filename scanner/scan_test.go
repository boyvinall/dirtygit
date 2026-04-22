package scanner

import (
	"os"
	"os/exec"
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

func TestParseConfigFileInvalidHideLocalOnlyRegex(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "bad-regex.yml")
	content := `
scandirs:
  include:
    - /opt/repos
branches:
  hidelocalonly:
    regex:
      - "("
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := ParseConfigFile(cfgPath, "")
	if err == nil {
		t.Fatal("ParseConfigFile() expected error for invalid regex")
	}
}

func TestParseConfigFileCompilesHideLocalOnlyRegex(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "ok-regex.yml")
	content := `
scandirs:
  include:
    - /opt/repos
branches:
  hidelocalonly:
    regex:
      - "^wip/"
      - "junk$"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseConfigFile(cfgPath, "")
	if err != nil {
		t.Fatalf("ParseConfigFile() error = %v", err)
	}
	lb := LocalBranchRef{Name: "wip/foo", Locations: []BranchLocation{
		{Name: "local", Exists: true, TipHash: "a"},
		{Name: "origin", Exists: false},
	}}
	if !cfg.ShouldHideLocalOnlyBranch(lb) {
		t.Fatal("expected wip/foo to be hidden")
	}
	lb2 := LocalBranchRef{Name: "main", Locations: lb.Locations}
	if cfg.ShouldHideLocalOnlyBranch(lb2) {
		t.Fatal("expected main not to match hide patterns")
	}
	lb3 := LocalBranchRef{Name: "wip/foo", Locations: []BranchLocation{
		{Name: "local", Exists: true, TipHash: "a"},
		{Name: "origin", Exists: true, TipHash: "b"},
	}}
	if cfg.ShouldHideLocalOnlyBranch(lb3) {
		t.Fatal("expected branch with remote ref not to be hidden as local-only")
	}
}

func TestParseConfigFileDefaultBranches(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "defaults.yml")
	content := `
scandirs:
  include:
    - /opt/repos
branches:
  default:
    - main
    - master
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseConfigFile(cfgPath, "")
	if err != nil {
		t.Fatalf("ParseConfigFile() error = %v", err)
	}
	if !cfg.AlwaysListBranch("main") || !cfg.AlwaysListBranch("master") {
		t.Fatalf("AlwaysListBranch: main=%v master=%v", cfg.AlwaysListBranch("main"), cfg.AlwaysListBranch("master"))
	}
	if cfg.AlwaysListBranch("develop") {
		t.Fatal("AlwaysListBranch(develop) should be false")
	}
}

func TestStatusForRepo(t *testing.T) {
	tmp := t.TempDir()
	for _, argv := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "test"},
	} {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	rs, include, err := StatusForRepo(cfg, tmp)
	if err != nil {
		t.Fatalf("StatusForRepo: %v", err)
	}
	if !include {
		t.Fatal("expected repo with untracked file to be dirty")
	}
	if rs.IsClean() {
		t.Fatal("expected non-clean status")
	}

	cmd := exec.Command("git", "add", "--", "dirty.txt")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	rs2, include2, err := StatusForRepo(cfg, tmp)
	if err != nil {
		t.Fatalf("StatusForRepo after add: %v", err)
	}
	if !include2 {
		t.Fatal("expected staged-only change to remain dirty")
	}
	if len(rs2.Porcelain.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rs2.Porcelain.Entries))
	}
	if rs2.Porcelain.Entries[0].Staging != 'A' {
		t.Fatalf("want staged added, got %+v", rs2.Porcelain.Entries[0])
	}
}
