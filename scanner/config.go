package scanner

import (
	"fmt"
	"os"
	"path/filepath"

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

	return &config, nil
}
