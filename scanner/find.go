package scanner

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/karrick/godirwalk"
)

func skip(needle string, haystack []string) bool {
	for _, f := range haystack {
		if f == needle {
			return true
		}
	}
	return false
}

func walkone(ctx context.Context, dir string, config *Config, results chan string) error {
	err := godirwalk.Walk(dir, &godirwalk.Options{
		Unsorted:            true,
		ScratchBuffer:       make([]byte, godirwalk.MinimumScratchBufferSize),
		FollowSymbolicLinks: config.FollowSymlinks,
		Callback: func(path string, ent *godirwalk.Dirent) error {

			// early exit?

			select {
			case <-ctx.Done():
				return filepath.SkipDir
			default:
			}

			// process all the SkipThis rules first

			if skip(path, config.ScanDirs.Exclude) {
				return godirwalk.SkipThis
			}
			if ent.IsSymlink() && !config.FollowSymlinks {
				return godirwalk.SkipThis
			}

			// then process non-matching rules which still descend

			if ent.Name() != ".git" {
				return nil
			}
			isDir, _ := ent.IsDirOrSymlinkToDir()
			if !isDir {
				return nil
			}

			results <- filepath.Dir(path)
			return godirwalk.SkipThis // don't descend further
		},
	})
	return err
}

func Walk(ctx context.Context, config *Config, results chan string) {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i := range config.ScanDirs.Include {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			err := walkone(ctx, dir, config, results)
			if err == filepath.SkipDir {
				cancel()
			}
		}(config.ScanDirs.Include[i])
	}
	wg.Wait()
	close(results)
}
