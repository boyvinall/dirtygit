package scanner

import (
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

type Excluder struct {
	files []string
	dirs  []string
}

func (e Excluder) IsExcluded(path string) bool {
	dir, base := filepath.Split(path)
	dirs := strings.Split(filepath.ToSlash(dir), string(filepath.Separator))
	for _, pattern := range e.files {
		m, _ := filepath.Match(pattern, base)
		if m {
			return true
		}
	}
	for _, pattern := range e.dirs {
		for _, d := range dirs {
			m, _ := filepath.Match(pattern, d)
			if m {
				return true
			}
		}
	}
	return false
}

func (e Excluder) FilterGitStatus(st git.Status) git.Status {
	newStatus := make(git.Status, len(st))
	for path, status := range st {
		if !e.IsExcluded(path) {
			newStatus[path] = status
		}
	}
	return newStatus
}

func NewExcluder(files, dirs []string) (Excluder, error) {
	return Excluder{
		files: files,
		dirs:  dirs,
	}, nil
}
