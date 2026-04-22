package scanner

import (
	"bufio"
	"context"
	"io"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type RepoStatus struct {
	git.Status

	Porcelain PorcelainStatus
	ScanTime time.Duration
}

type MultiGitStatus map[string]RepoStatus

type PorcelainEntry struct {
	Staging      git.StatusCode
	Worktree     git.StatusCode
	Path         string
	OriginalPath string
}

type PorcelainStatus struct {
	Entries []PorcelainEntry
}

func (p PorcelainStatus) ToGitStatus() git.Status {
	st := make(git.Status, len(p.Entries))
	for _, entry := range p.Entries {
		fst := &git.FileStatus{
			Staging:  entry.Staging,
			Worktree: entry.Worktree,
		}
		st[entry.Path] = fst
	}
	return st
}

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
	b, err := os.ReadFile(filepath.Clean(filename))
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

func ParsePorcelainStatus(r io.Reader) (PorcelainStatus, error) {
	st := PorcelainStatus{}
	lineScanner := bufio.NewScanner(r)
	for lineScanner.Scan() {
		entry, err := parsePorcelainLine(lineScanner.Text())
		if err != nil {
			return PorcelainStatus{}, err
		}
		st.Entries = append(st.Entries, entry)
	}
	if err := lineScanner.Err(); err != nil {
		return PorcelainStatus{}, err
	}
	return st, nil
}

func parsePorcelainLine(s string) (PorcelainEntry, error) {
	if len(s) < 4 || s[2] != ' ' {
		return PorcelainEntry{}, errors.Errorf("unable to parse status line: %q", s)
	}

	entry := PorcelainEntry{
		Staging:  git.StatusCode(s[0]),
		Worktree: git.StatusCode(s[1]),
	}

	payload := s[3:]
	if oldPath, newPath, ok := strings.Cut(payload, " -> "); ok {
		entry.OriginalPath = oldPath
		entry.Path = newPath
	} else {
		entry.Path = payload
	}

	if entry.Path == "" {
		return PorcelainEntry{}, errors.Errorf("unable to parse file path from status line: %q", s)
	}

	return entry, nil
}

// GitStatus invokes git executable to determine the git status for a directory.
func GitStatus(d string) (PorcelainStatus, error) {

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = d
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	if err := cmd.Start(); err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	st, err := ParsePorcelainStatus(stdout)
	if err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	if err := cmd.Wait(); err != nil {
		return PorcelainStatus{}, errors.Wrap(err, d)
	}

	return st, nil
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

		if !st.IsClean() {
			totalStatusDuration += duration
			results[d] = RepoStatus{
				Status:    st,
				Porcelain: porcelain,
				ScanTime:  duration,
			}
		}
	}

	w := <-ch
	log.Println("walkDuration:", w.duration)
	log.Println("statusDuration:", totalStatusDuration)
	return results, w.err
}
