package scanner

import (
	"context"
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

func Scan(config *Config) (MultiGitStatus, error) {
	e, err := NewExcluder(config.GitIgnore.FileGlob, config.GitIgnore.DirGlob)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	repositories := make(chan string, 10)
	go Walk(ctx, config, repositories)

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

		st = e.FilterGitStatus(st)
		if !st.IsClean() {
			results[d] = st
		}
	}
	return results, nil
}
