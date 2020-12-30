package scanner

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type MultiGitStatus map[string]git.Status

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

// Scan finds all "dirty" git repositories specified by config
func Scan(config *Config) (MultiGitStatus, error) {
	ex, e := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)
	if e != nil {
		return nil, e
	}

	ctx := context.Background()
	repositories := make(chan string, 10)

	errCh := make(chan error)
	go func() {
		errCh <- Walk(ctx, config, repositories)
	}()

	results := make(MultiGitStatus)
	for d := range repositories {
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

		st = ex.FilterGitStatus(st)
		if !st.IsClean() {
			results[d] = st
		}
	}
	return results, <-errCh
}
