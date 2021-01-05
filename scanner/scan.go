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

func ParseConfigFile(filename string) (*Config, error) {
	b, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
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
	err := os.Chdir(d)
	if err != nil {
		return nil, errors.Wrap(err, d)
	}

	cmd := exec.Command("git", "status", "--porcelain")
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

// Scan finds all "dirty" git repositories specified by config
func Scan(config *Config) (MultiGitStatus, error) {
	ex, e := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)
	if e != nil {
		return nil, e
	}

	ctx := context.Background()
	repositories := make(chan string, 1000)

	type walkResult struct {
		err      error
		duration time.Duration
	}
	ch := make(chan walkResult)
	go func() {
		start := time.Now()
		err := Walk(ctx, config, repositories)
		ch <- walkResult{
			err:      err,
			duration: time.Since(start),
		}
	}()

	results := make(MultiGitStatus)
	totalStatusDuration := time.Duration(0)
	for d := range repositories {
		start := time.Now()

		st, err := GitStatus(d)
		if err != nil {
			return nil, err
		}
		st = ex.FilterGitStatus(st)

		duration := time.Since(start)
		log.Println(d, duration)

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
