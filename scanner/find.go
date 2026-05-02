package scanner

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"golang.org/x/sync/errgroup"
)

func isGitMetadataDir(path string, d fs.DirEntry) (bool, error) {
	if d.Name() != ".git" {
		return false, nil
	}
	if d.IsDir() {
		return true, nil
	}
	if d.Type()&os.ModeSymlink != 0 {
		fi, err := os.Stat(path)
		if err != nil {
			return false, err
		}
		return fi.IsDir(), nil
	}
	return false, nil
}

// walkone descends a single directory tree looking for git repos.
// onRepoFound is called for each discovered repo (may be concurrent across roots); nil is safe.
func walkone(ctx context.Context, dir string, config *Config, results chan string, onRepoFound func(string)) error {
	var walkDirFn fs.WalkDirFunc
	walkDirFn = func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			var pathErr *os.PathError
			if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrNotExist) {
				return nil
			}
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if errors.Is(err, syscall.ELOOP) {
				return nil
			}
			log.Printf("ERROR: %s: %v", path, err)
			return err
		}

		select {
		case <-ctx.Done():
			return filepath.SkipDir
		default:
		}

		if d.IsDir() {
			log.Printf("path %s", path)
		}

		if slices.Contains(config.ScanDirs.Exclude, path) {
			return filepath.SkipDir
		}

		if d.Type()&os.ModeSymlink != 0 && !config.FollowSymlinks {
			return nil
		}

		ok, metaErr := isGitMetadataDir(path, d)
		if metaErr != nil {
			if errors.Is(metaErr, os.ErrNotExist) || errors.Is(metaErr, syscall.ELOOP) {
				return nil
			}
			log.Printf("ERROR: %s: %v", path, metaErr)
			return metaErr
		}
		if ok {
			log.Printf("git %s", path)
			repo := filepath.Dir(path)
			if onRepoFound != nil {
				onRepoFound(repo)
			}
			results <- repo
			return filepath.SkipDir
		}

		if config.FollowSymlinks && d.Type()&os.ModeSymlink != 0 {
			fi, statErr := os.Stat(path)
			if statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) || errors.Is(statErr, syscall.ELOOP) {
					return nil
				}
				log.Printf("ERROR: %s: %v", path, statErr)
				return statErr
			}
			if fi.IsDir() {
				entries, rdErr := os.ReadDir(path)
				if rdErr != nil {
					if errors.Is(rdErr, os.ErrNotExist) || errors.Is(rdErr, syscall.ELOOP) {
						return nil
					}
					log.Printf("ERROR: %s: %v", path, rdErr)
					return rdErr
				}
				for _, ent := range entries {
					child := filepath.Join(path, ent.Name())
					if werr := filepath.WalkDir(child, walkDirFn); werr != nil {
						return werr
					}
				}
			}
		}

		return nil
	}

	return filepath.WalkDir(dir, walkDirFn)
}

// Walk finds all git repositories in the directories specified in config.
// onRepoFound is invoked once per discovered repository (from walker goroutines); nil is safe.
func Walk(ctx context.Context, config *Config, results chan string, onRepoFound func(string)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var eg errgroup.Group
	for i := range config.ScanDirs.Include {
		j := i // copy loop variable
		eg.Go(func() error {
			err := walkone(ctx, config.ScanDirs.Include[j], config, results, onRepoFound)
			if err == filepath.SkipDir {
				cancel()
			} else if err != nil {
				return err
			}
			return nil
		})
	}
	err := eg.Wait()
	close(results)
	return err
}
