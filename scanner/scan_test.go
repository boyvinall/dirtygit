package scanner

import (
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
