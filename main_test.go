package main

import (
	"os"
	"strings"
	"testing"

	"github.com/boyvinall/dirtygit/scanner"
)

func expandEnvInConfig(config *scanner.Config) {
	for i := range config.ScanDirs.Include {
		config.ScanDirs.Include[i] = os.ExpandEnv(config.ScanDirs.Include[i])
	}
	for i := range config.ScanDirs.Exclude {
		config.ScanDirs.Exclude[i] = os.ExpandEnv(config.ScanDirs.Exclude[i])
	}
}

func TestGetDefaultConfigPathUsesHomeAndSuffix(t *testing.T) {
	p := getDefaultConfigPath()
	if !strings.HasSuffix(p, ".dirtygit.yml") {
		t.Fatalf("expected path to end with .dirtygit.yml, got %q", p)
	}
}

func TestValidateReport_Integration(t *testing.T) {
	configPath := getDefaultConfigPath()
	if _, err := os.Stat(configPath); err != nil {
		t.Skipf("default config not found at %s", configPath)
	}

	config, err := scanner.ParseConfigFile(configPath, defaultConfig)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	expandEnvInConfig(config)

	mgs, err := scanner.Scan(config)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	r := buildReport(config, mgs)

	for _, v := range validateReport(config, r) {
		t.Error(v)
	}
}
