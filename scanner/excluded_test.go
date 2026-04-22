package scanner

import (
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestExcluderIsExcluded(t *testing.T) {
	ex := Excluder{
		files: []string{"*.tmp", "*.log"},
		dirs:  []string{"node_modules", ".cache"},
	}

	cases := []struct {
		path string
		want bool
	}{
		{path: "app/main.go", want: false},
		{path: "app/debug.log", want: true},
		{path: "app/tmp/build.tmp", want: true},
		{path: "web/node_modules/react/index.js", want: true},
		{path: "web/src/.cache/state.json", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := ex.IsExcluded(tc.path)
			if got != tc.want {
				t.Fatalf("IsExcluded(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestExcluderFilterPorcelainStatus(t *testing.T) {
	ex := Excluder{
		files: []string{"*.log"},
		dirs:  []string{"vendor"},
	}

	in := PorcelainStatus{
		Entries: []PorcelainEntry{
			{Staging: 'M', Worktree: ' ', Path: "cmd/app/main.go"},
			{Staging: ' ', Worktree: 'M', Path: "cmd/app/debug.log"},
			{Staging: 'A', Worktree: ' ', Path: "vendor/pkg/file.go"},
		},
	}

	got := ex.FilterPorcelainStatus(in)
	if len(got.Entries) != 1 {
		t.Fatalf("FilterPorcelainStatus() entries = %d, want 1", len(got.Entries))
	}
	if got.Entries[0].Path != "cmd/app/main.go" {
		t.Fatalf("filtered path = %q, want cmd/app/main.go", got.Entries[0].Path)
	}
}

func TestExcluderFilterGitStatus(t *testing.T) {
	ex := Excluder{
		files: []string{"*.tmp"},
		dirs:  []string{"dist"},
	}
	st := git.Status{
		"src/main.go":            &git.FileStatus{Staging: 'M', Worktree: ' '},
		"src/generated.tmp":      &git.FileStatus{Staging: '?', Worktree: '?'},
		"web/dist/bundle.min.js": &git.FileStatus{Staging: 'A', Worktree: ' '},
	}

	got := ex.FilterGitStatus(st)
	if len(got) != 1 {
		t.Fatalf("FilterGitStatus() len = %d, want 1", len(got))
	}
	if _, ok := got["src/main.go"]; !ok {
		t.Fatal("expected src/main.go to remain after filtering")
	}
}
