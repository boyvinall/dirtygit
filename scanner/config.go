package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

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
		b = []byte(defaultConfig)
	default:
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return nil, err
	}

	if err := compileLocalOnlyHideRegexes(&config); err != nil {
		return nil, err
	}
	prepareBranchDefaultSet(&config)

	return &config, nil
}

func compileLocalOnlyHideRegexes(c *Config) error {
	patterns := c.Branches.HideLocalOnly.Regex
	if len(patterns) == 0 {
		return nil
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for i, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("branches.hidelocalonly.regex[%d] %q: %w", i, p, err)
		}
		out = append(out, re)
	}
	c.localOnlyHideCompiled = out
	return nil
}

func prepareBranchDefaultSet(c *Config) {
	names := c.Branches.Default
	if len(names) == 0 {
		c.branchDefaultAlways = nil
		return
	}
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		m[n] = struct{}{}
	}
	if len(m) == 0 {
		c.branchDefaultAlways = nil
		return
	}
	c.branchDefaultAlways = m
}
