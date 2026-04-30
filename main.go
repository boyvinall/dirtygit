package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v3"

	"github.com/boyvinall/dirtygit/scanner"
	"github.com/boyvinall/dirtygit/ui"
)

func getDefaultConfigPath() string {
	home, err := homedir.Dir()
	_ = err // ignore
	return filepath.Join(home, ".dirtygit.yml")
}

//go:embed .dirtygit.yml
var defaultConfig string

// Set at link time via GoReleaser (e.g. -ldflags "-X main.version=v1.2.3").
var version = "dev"

func main() {
	app := &cli.Command{
		Name:                  "dirtygit",
		Usage:                 "Finds git repos in need of commitment",
		Version:               version,
		EnableShellCompletion: true,
		CommandNotFound: func(ctx context.Context, cmd *cli.Command, name string) {
			fmt.Printf("ERROR: Unknown command '%s'\n", name)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Location of config file",
				Value:   getDefaultConfigPath(),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			config, err := scanner.ParseConfigFile(cmd.String("config"), defaultConfig)
			if err != nil {
				return err
			}
			if cmd.Args().Len() > 0 {
				config.ScanDirs.Include = cmd.Args().Slice()
			}
			for i := range config.ScanDirs.Include {
				config.ScanDirs.Include[i] = os.ExpandEnv(config.ScanDirs.Include[i])
			}
			for i := range config.ScanDirs.Exclude {
				config.ScanDirs.Exclude[i] = os.ExpandEnv(config.ScanDirs.Exclude[i])
			}

			return ui.Run(config)
		},
	}
	err := app.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}
