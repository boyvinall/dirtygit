package scanner

import (
	"context"
	"log"
	"sync/atomic"
	"time"
)

func reportProgress(onProgress func(ScanProgress), p ScanProgress) {
	if onProgress != nil {
		onProgress(p)
	}
}

// Scan finds all "dirty" git repositories specified by config.
func Scan(config *Config) (MultiGitStatus, error) {
	return ScanWithProgress(config, nil)
}

// ScanWithProgress runs the same scan as [Scan] and invokes onProgress from concurrent
// discovery and the status loop. Callbacks should be non-blocking (e.g. small channel send).
func ScanWithProgress(config *Config, onProgress func(ScanProgress)) (MultiGitStatus, error) {
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

	results := make(MultiGitStatus)
	totalStatusDuration := time.Duration(0)
	for d := range repositories {
		reportProgress(onProgress, ScanProgress{
			ReposFound:   int(found.Load()),
			ReposChecked: int(checked.Load()),
			CurrentPath:  d,
		})

		start := time.Now()

		porcelain, err := GitStatus(d)
		if err != nil {
			return nil, err
		}
		porcelain = ex.FilterPorcelainStatus(porcelain)
		st := porcelain.ToGitStatus()

		duration := time.Since(start)
		log.Println(d, duration)

		n := checked.Add(1)
		reportProgress(onProgress, ScanProgress{
			ReposFound:   int(found.Load()),
			ReposChecked: int(n),
		})

		branches, err := GitBranchStatus(d)
		if err != nil {
			log.Printf("branch status scan failed for %s: %v", d, err)
		}

		if !st.IsClean() || branches.HasLocalRemoteMismatch() {
			totalStatusDuration += duration
			results[d] = RepoStatus{
				Status:    st,
				Porcelain: porcelain,
				Branches:  branches,
				ScanTime:  duration,
			}
		}
	}

	w := <-ch
	log.Println("walkDuration:", w.duration)
	log.Println("statusDuration:", totalStatusDuration)
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

	rs := RepoStatus{
		Status:    st,
		Porcelain: porcelain,
		Branches:  branches,
		ScanTime:  time.Since(start),
	}
	include := !st.IsClean() || branches.HasLocalRemoteMismatch()
	return rs, include, nil
}
