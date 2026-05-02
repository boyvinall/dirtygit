package scanner

import (
	"context"
	"log"
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
func Scan(config *Config) (*MultiGitStatus, error) {
	return ScanWithProgress(config, nil)
}

// ScanWithProgress runs the same scan as [Scan] and invokes onProgress from concurrent
// discovery and the status loop. Callbacks should be non-blocking (e.g. small channel send).
func ScanWithProgress(config *Config, onProgress func(ScanProgress)) (*MultiGitStatus, error) {
	ex, e := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)
	if e != nil {
		return nil, e
	}

	ctx := context.Background()
	repositories := make(chan string, 1000)

	var found, checked atomic.Uint64

	type walkResult struct {
		err      error
		duration time.Duration
	}
	ch := make(chan walkResult)
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
	var totalStatusDuration int64 // nanoseconds; atomic sum from concurrent workers
	var eg errgroup.Group

	for d := range repositories {
		eg.Go(func() error {
			// About to run git status for this repo; set CurrentPath so the scan modal
			// shows which directory is active (and keeps showing it through the rest
			// of this iteration via the update after GitStatus returns).
			reportProgress(onProgress, ScanProgress{
				ReposFound:   int(found.Load()),
				ReposChecked: int(checked.Load()),
				CurrentPath:  d,
			})

			start := time.Now()

			porcelain, err := GitStatus(d)
			if err != nil {
				return err
			}
			porcelain = ex.FilterPorcelainStatus(porcelain)
			st := porcelain.ToGitStatus()

			duration := time.Since(start)
			log.Println(d, duration)

			n := checked.Add(1)
			// Git status finished for this repo; advance ReposChecked and retain
			// CurrentPath until the next channel receive so the path line does not
			// flicker to empty while GitBranchStatus and filtering still run.
			reportProgress(onProgress, ScanProgress{
				ReposFound:   int(found.Load()),
				ReposChecked: int(n),
				CurrentPath:  d,
			})

			branches, err := GitBranchStatus(d)
			if err != nil {
				log.Printf("branch status scan failed for %s: %v", d, err)
			}
			branches.FilterLocalOnlyForConfig(config)
			if !st.IsClean() || branches.HasLocalRemoteMismatch() {
				atomic.AddInt64(&totalStatusDuration, duration.Nanoseconds())
				results.AddResult(d, RepoStatus{
					Status:    st,
					Porcelain: porcelain,
					Branches:  branches,
					ScanTime:  duration,
				})
			}
			return nil
		})
	}

	statusErr := eg.Wait()
	w := <-ch
	if statusErr != nil {
		return nil, statusErr
	}
	log.Println("walkDuration:", w.duration)
	log.Println("statusDuration:", time.Duration(atomic.LoadInt64(&totalStatusDuration)))
	return results, w.err
}

// StatusForRepo returns fresh status for a single repository directory using the
// same porcelain filtering and branch metadata as [ScanWithProgress]. The bool
// is whether this repo should appear in the dirty list (!clean or remote mismatch).
func StatusForRepo(config *Config, dir string) (RepoStatus, bool, error) {
	ex, err := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)
	if err != nil {
		return RepoStatus{}, false, err
	}
	start := time.Now()
	porcelain, err := GitStatus(dir)
	if err != nil {
		return RepoStatus{}, false, err
	}
	porcelain = ex.FilterPorcelainStatus(porcelain)
	st := porcelain.ToGitStatus()

	branches, berr := GitBranchStatus(dir)
	if berr != nil {
		log.Printf("branch status scan failed for %s: %v", dir, berr)
	}
	branches.FilterLocalOnlyForConfig(config)

	rs := RepoStatus{
		Status:    st,
		Porcelain: porcelain,
		Branches:  branches,
		ScanTime:  time.Since(start),
	}
	include := !st.IsClean() || branches.HasLocalRemoteMismatch()
	return rs, include, nil
}
