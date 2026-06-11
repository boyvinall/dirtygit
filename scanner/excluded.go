package scanner

import (
	"path/filepath"
	"strings"
)

type Excluder struct {
	files []string
	dirs  []string
}

func (e Excluder) IsExcluded(path string) bool {
	dir, base := filepath.Split(path)
	dirs := strings.Split(filepath.ToSlash(dir), "/")
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

func (e Excluder) FilterPorcelainStatus(st PorcelainStatus) PorcelainStatus {
	filtered := PorcelainStatus{Entries: make([]PorcelainEntry, 0, len(st.Entries))}
	for _, entry := range st.Entries {
		if e.IsExcluded(entry.Path) {
			continue
		}
		filtered.Entries = append(filtered.Entries, entry)
	}
	return filtered
}

func NewExcluder(files, dirs []string) Excluder {
	return Excluder{
		files: files,
		dirs:  dirs,
	}
}
