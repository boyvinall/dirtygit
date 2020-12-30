package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"

	"github.com/boyvinall/dirtygit/cui"
	"github.com/boyvinall/dirtygit/scanner"
)

func getDefaultConfigPath() string {
	home, err := homedir.Dir()
	_ = err // ignore
	return filepath.Join(home, ".dirtygit.yml")
}

func main() {
	app := cli.NewApp()
	app.Name = "dirtygit"
	app.Usage = "Finds git repos in need of commitment"
	app.EnableBashCompletion = true
	app.CommandNotFound = func(c *cli.Context, cmd string) {
		fmt.Printf("ERROR: Unknown command '%s'\n", cmd)
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Location of config file",
			Value:   getDefaultConfigPath(),
		},
	}
	app.Action = func(c *cli.Context) error {
		config, err := scanner.ParseConfigFile(c.String("config"))
		if err != nil {
			return err
		}
		// fmt.Printf("config %#v\n", config)
		// os.Exit(1)

		err = cui.Run(config)
		if err != nil {
			return err
		}
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}
