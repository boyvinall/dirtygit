package scanner

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
)

func ParsePorcelainStatus(r io.Reader) (PorcelainStatus, error) {
	st := PorcelainStatus{}
	lineScanner := bufio.NewScanner(r)
	for lineScanner.Scan() {
		entry, err := parsePorcelainLine(lineScanner.Text())
		if err != nil {
			return PorcelainStatus{}, err
		}
		st.Entries = append(st.Entries, entry)
	}
	if err := lineScanner.Err(); err != nil {
		return PorcelainStatus{}, err
	}
	return st, nil
}

func parsePorcelainLine(s string) (PorcelainEntry, error) {
	if len(s) < 4 || s[2] != ' ' {
		return PorcelainEntry{}, fmt.Errorf("unable to parse status line: %q", s)
	}

	entry := PorcelainEntry{
		Staging:  git.StatusCode(s[0]),
		Worktree: git.StatusCode(s[1]),
	}

	payload := s[3:]
	if oldPath, newPath, ok := strings.Cut(payload, " -> "); ok {
		entry.OriginalPath = oldPath
		entry.Path = newPath
	} else {
		entry.Path = payload
	}

	if entry.Path == "" {
		return PorcelainEntry{}, fmt.Errorf("unable to parse file path from status line: %q", s)
	}

	return entry, nil
}

// GitStatus invokes git executable to determine the git status for a directory.
func GitStatus(d string) (PorcelainStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = d
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PorcelainStatus{}, fmt.Errorf("%s: %w", d, err)
	}

	if err := cmd.Start(); err != nil {
		return PorcelainStatus{}, fmt.Errorf("%s: %w", d, err)
	}

	st, err := ParsePorcelainStatus(stdout)
	if err != nil {
		return PorcelainStatus{}, fmt.Errorf("%s: %w", d, err)
	}

	if err := cmd.Wait(); err != nil {
		return PorcelainStatus{}, fmt.Errorf("%s: %w", d, err)
	}

	return st, nil
}
