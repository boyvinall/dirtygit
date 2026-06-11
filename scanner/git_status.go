package scanner

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/go-git/go-git/v5"
)

// ParsePorcelainStatus parses NUL-delimited output from git status --porcelain -z.
// Rename/copy entries (staging code 'R' or 'C') consume an extra NUL-delimited
// token for the original path; all other entries are single-token records.
func ParsePorcelainStatus(r io.Reader) (PorcelainStatus, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return PorcelainStatus{}, err
	}
	st := PorcelainStatus{}
	i := 0
	for i < len(data) {
		j := bytes.IndexByte(data[i:], 0)
		var token []byte
		if j < 0 {
			token = data[i:]
			i = len(data)
		} else {
			token = data[i : i+j]
			i += j + 1
		}
		if len(token) == 0 {
			continue
		}
		if len(token) < 4 || token[2] != ' ' {
			return PorcelainStatus{}, fmt.Errorf("unable to parse status line: %q", token)
		}
		entry := PorcelainEntry{
			Staging:  git.StatusCode(token[0]),
			Worktree: git.StatusCode(token[1]),
			Path:     string(token[3:]),
		}
		if entry.Path == "" {
			return PorcelainStatus{}, fmt.Errorf("unable to parse file path from status line: %q", token)
		}
		if entry.Staging == 'R' || entry.Staging == 'C' {
			// Original path follows as the next NUL-delimited token.
			k := bytes.IndexByte(data[i:], 0)
			var orig []byte
			if k < 0 {
				orig = data[i:]
				i = len(data)
			} else {
				orig = data[i : i+k]
				i += k + 1
			}
			entry.OriginalPath = string(orig)
		}
		st.Entries = append(st.Entries, entry)
	}
	return st, nil
}

// GitStatus invokes git to return porcelain status for a directory.
func GitStatus(d string) (PorcelainStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain", "-z")
	cmd.Dir = d
	out, err := cmd.Output()
	if err != nil {
		return PorcelainStatus{}, fmt.Errorf("%s: %w", d, err)
	}
	return ParsePorcelainStatus(bytes.NewReader(out))
}
