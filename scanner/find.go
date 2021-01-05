package scanner

import (
	"context"
	"path/filepath"

	"github.com/karrick/godirwalk"
	"golang.org/x/sync/errgroup"
)

func skip(needle string, haystack []string) bool {
	for _, f := range haystack {
		if f == needle {
			return true
		}
	}
	return false
}

// walkone descends a single directory tree looking for git repos
func walkone(ctx context.Context, dir string, config *Config, results chan string) error {
	err := godirwalk.Walk(dir, &godirwalk.Options{
		Unsorted:            true,
		ScratchBuffer:       make([]byte, godirwalk.MinimumScratchBufferSize),
		FollowSymbolicLinks: config.FollowSymlinks,
		ErrorCallback: func(path string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
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

// Walk finds all git repositories in the directories specified in config
func Walk(ctx context.Context, config *Config, results chan string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var errors errgroup.Group
	for i := range config.ScanDirs.Include {
		j := i // copy loop variable
		errors.Go(func() error {
			err := walkone(ctx, config.ScanDirs.Include[j], config, results)
			if err == filepath.SkipDir {
				cancel()
			} else if err != nil {
				return err
			}
			return nil
		})
	}
	err := errors.Wait()
	close(results)
	return err
}
