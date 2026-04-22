package scanner

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type RepoStatus struct {
	git.Status

	ScanTime time.Duration
}

type MultiGitStatus map[string]RepoStatus

// ScanProgress reports coarse scan activity for UIs (discovery vs git status).
// ReposFound may increase while ReposChecked catches up; both match the final total when complete.
type ScanProgress struct {
	ReposFound   int
	ReposChecked int
	CurrentPath  string
}

type Config struct {
	ScanDirs struct {
		Include []string `yaml:"include"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"scandirs"`
	GitIgnore struct {
		FileGlob []string `yaml:"fileglob"`
		DirGlob  []string `yaml:"dirglob"`
	} `yaml:"gitignore"`
	FollowSymlinks bool `yaml:"followsymlinks"`
}

func DumpConfig(config *Config) error {
	b, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func ParseConfigFile(filename, defaultConfig string) (*Config, error) {
	b, err := ioutil.ReadFile(filepath.Clean(filename))
	switch {
	case err == nil:
	case os.IsNotExist(err):
		b = ([]byte)(defaultConfig)
	default:
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// GoGitStatus uses go-git package to determine the git status for a directory
func GoGitStatus(d string) (git.Status, error) {
	r, err := git.PlainOpen(d)
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	st, err := wt.Status()
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	return st, nil
}

// GitStatus invokes git executable to determine the git status for a directory
func GitStatus(d string) (git.Status, error) {
	st := make(git.Status)

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = d
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	errCh := make(chan error)
	go func() {
		errCh <- cmd.Start()
	}()
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var fst git.FileStatus
		var f string

		s := scanner.Text()
		if len(s) < 4 {
			return nil, errors.Errorf("unable to parse status: '%s' for %s", s, d)
		}
		fst.Staging = git.StatusCode(s[0])
		fst.Worktree = git.StatusCode(s[1])
		f = s[3:]
		st[f] = &fst
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, d)
	}

	return st, <-errCh
}

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

		st, err := GitStatus(d)
		if err != nil {
			return nil, err
		}
		st = ex.FilterGitStatus(st)

		duration := time.Since(start)
		log.Println(d, duration)

		n := checked.Add(1)
		reportProgress(onProgress, ScanProgress{
			ReposFound:   int(found.Load()),
			ReposChecked: int(n),
		})

		if !st.IsClean() {
			totalStatusDuration += duration
			results[d] = RepoStatus{
				Status:   st,
				ScanTime: duration,
			}
		}
	}

	w := <-ch
	log.Println("walkDuration:", w.duration)
	log.Println("statusDuration:", totalStatusDuration)
	return results, w.err
}
