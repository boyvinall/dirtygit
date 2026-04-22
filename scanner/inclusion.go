package scanner

import "fmt"

// RepoInclusionReasons returns human-readable lines explaining why a repository
// is included in the result list, using the same rules as [ScanWithProgress] and
// [StatusForRepo] (uncommitted work and/or a local/remote branch mismatch for the
// current branch).
func RepoInclusionReasons(rs RepoStatus) []string {
	var out []string
	if !rs.IsClean() {
		n := len(rs.Porcelain.Entries)
		if n == 0 {
			n = len(rs.Status)
		}
		if n == 0 {
			out = append(out, "The working tree or index is not clean (uncommitted change).")
		} else {
			out = append(out, fmt.Sprintf("Uncommitted changes: %d path(s) in the working tree and/or index (after .dirtygit config filters).", n))
		}
	}
	if r := rs.Branches.LocalRemoteMismatchReasons(); len(r) > 0 {
		out = append(out, r...)
	}
	return out
}
