package scanner

import (
	"sort"
	"sync"
)

// MultiGitStatus holds per-repository scan results. The zero value is usable:
// reads treat a nil receiver as empty; the first AddResult or Set allocates
// the inner map. Do not copy a non-zero MultiGitStatus (it contains a sync.Mutex).
type MultiGitStatus struct {
	mu sync.Mutex
	m  map[string]RepoStatus
}

// NewMultiGitStatus returns an empty result set ready for concurrent AddResult calls.
func NewMultiGitStatus() *MultiGitStatus {
	return &MultiGitStatus{m: make(map[string]RepoStatus)}
}

// AddResult records a dirty or diverged repository; safe for concurrent use.
func (m *MultiGitStatus) AddResult(path string, rs RepoStatus) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = make(map[string]RepoStatus)
	}
	m.m[path] = rs
}

// Set replaces status for path (typically the UI thread after a scan completes).
func (m *MultiGitStatus) Set(path string, rs RepoStatus) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = make(map[string]RepoStatus)
	}
	m.m[path] = rs
}

// Delete removes path from the set.
func (m *MultiGitStatus) Delete(path string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, path)
}

// Get returns status for path, if present.
func (m *MultiGitStatus) Get(path string) (RepoStatus, bool) {
	if m == nil {
		return RepoStatus{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rs, ok := m.m[path]
	return rs, ok
}

// Len returns the number of repositories recorded.
func (m *MultiGitStatus) Len() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.m)
}

// SortedRepoPaths returns repository paths in stable alphabetical order.
func (m *MultiGitStatus) SortedRepoPaths() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	paths := make([]string, 0, len(m.m))
	for r := range m.m {
		paths = append(paths, r)
	}
	m.mu.Unlock()
	sort.Strings(paths)
	return paths
}
