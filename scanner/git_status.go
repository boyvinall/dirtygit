package scanner

import (
	"bufio"
	"io"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
)

// GoGitStatus uses go-git package to determine the git status for a directory.
func GoGitStatus(d string) (git.Status, error) {
	r, err := git.PlainOpen(d)
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	st, err := wt.Status()
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	return st, nil
}

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
		return PorcelainEntry{}, errors.Errorf("unable to parse status line: %q", s)
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
		return PorcelainEntry{}, errors.Errorf("unable to parse file path from status line: %q", s)
	}

	return entry, nil
}

// GitStatus invokes git executable to determine the git status for a directory.
func GitStatus(d string) (PorcelainStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = d
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	if err := cmd.Start(); err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	st, err := ParsePorcelainStatus(stdout)
	if err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	if err := cmd.Wait(); err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	return st, nil
}
