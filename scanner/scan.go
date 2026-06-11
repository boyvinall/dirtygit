package scanner

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

func reportProgress(onProgress func(ScanProgress), p ScanProgress) {
	if onProgress != nil {
		onProgress(p)
	}
}

// Scan finds all "dirty" git repositories specified by config.
func Scan(ctx context.Context, config *Config) (*MultiGitStatus, error) {
	return ScanWithProgress(ctx, config, nil)
}

// ScanWithProgress runs the same scan as [Scan] and invokes onProgress from concurrent
// discovery and the status loop. Callbacks should be non-blocking (e.g. small channel send).
func ScanWithProgress(ctx context.Context, config *Config, onProgress func(ScanProgress)) (*MultiGitStatus, error) {
	repositories := make(chan string, 1000)

	var found, checked atomic.Uint64

	type walkResult struct {
		err      error
		duration time.Duration
	}
	ch := make(chan walkResult, 1)
	go func() {
		start := time.Now()
		err := Walk(ctx, config, repositories, func(string) {
			n := found.Add(1)
			// Discovery found another .git directory; bump ReposFound so the UI can
			// show how far ahead the walk is versus status checks (ReposChecked).
			reportProgress(onProgress, ScanProgress{
				ReposFound:   int(n),
				ReposChecked: int(checked.Load()),
			})
		})
		ch <- walkResult{
			err:      err,
			duration: time.Since(start),
		}
	}()

	results := NewMultiGitStatus()
	var eg errgroup.Group
	ex := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)

	for d := range repositories {
		eg.Go(func() error {
			// About to run StatusForRepo for this path; set CurrentPath so the scan modal
			// shows which directory is active until this worker finishes and the UI updates.
			reportProgress(onProgress, ScanProgress{
				ReposFound:   int(found.Load()),
				ReposChecked: int(checked.Load()),
				CurrentPath:  d,
			})

			rs, include, err := statusForRepoWithExcluder(config, ex, d)
			if err != nil {
				return err
			}
			n := checked.Add(1)
			// Per-repo StatusForRepo finished; advance ReposChecked and retain
			// CurrentPath until the next progress event so the path line does not flicker.
			reportProgress(onProgress, ScanProgress{
				ReposFound:   int(found.Load()),
				ReposChecked: int(n),
				CurrentPath:  d,
			})

			if include {
				results.AddResult(d, rs)
			}
			return nil
		})
	}

	statusErr := eg.Wait()
	w := <-ch
	if statusErr != nil {
		return nil, statusErr
	}
	return results, w.err
}

// StatusForRepo returns fresh status for a single repository directory using the
// same porcelain filtering and branch metadata as [ScanWithProgress]. The bool
// is whether this repo should appear in the dirty list (!clean or remote mismatch).
func StatusForRepo(config *Config, dir string) (RepoStatus, bool, error) {
	ex := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)
	return statusForRepoWithExcluder(config, ex, dir)
}

// statusForRepoWithExcluder is the shared implementation used by [StatusForRepo]
// and [ScanWithProgress] (which builds the excluder once for all repos).
func statusForRepoWithExcluder(config *Config, ex Excluder, dir string) (RepoStatus, bool, error) {
	porcelain, err := GitStatus(dir)
	if err != nil {
		return RepoStatus{}, false, err
	}
	porcelain = ex.FilterPorcelainStatus(porcelain)
	branch, detached, branches, err := GitBranchStatus(dir)
	if err != nil {
		// Best-effort: a single repo's branch metadata failure should not abort the
		// whole scan. The repo will still appear if it has uncommitted working-tree
		// changes; it will just show no branch divergence information.
		slog.Warn("branch status scan failed", "dir", dir, "err", err)
	}

	rs := RepoStatus{
		Branch:    branch,
		Detached:  detached,
		Porcelain: porcelain,
		Branches:  branches,
	}
	rs.FilteredBranches = rs.Filter(config)
	include := !porcelain.ToGitStatus().IsClean() || rs.HasUnpushedChanges(config)
	return rs, include, nil
}
