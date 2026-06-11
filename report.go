package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/boyvinall/dirtygit/scanner"
)

// reportBranchEntry is one local branch row in the report.
type reportBranchEntry struct {
	scanner.LocalBranchRef

	// ShownInTUI is true when this branch appears in the TUI Branches pane for its repository
	ShownInTUI  bool `json:"shown_in_tui"`
	IsLocalOnly bool `json:"is_local_only"`
}

// reportFileEntry is one porcelain status entry for a file.
type reportFileEntry struct {
	// Staging and Worktree are the single-character git status codes (e.g. "M", "A", "?").
	Staging      string `json:"staging"`
	Worktree     string `json:"worktree"`
	Path         string `json:"path"`
	OriginalPath string `json:"original_path"`
}

// reportRepo is the per-repository section of the report.
type reportRepo struct {
	Path    string `json:"path"`
	IsClean bool   `json:"is_clean"`
	// Files are the uncommitted working-tree changes (porcelain entries).
	Files []reportFileEntry `json:"files"`
	// CurrentBranch is the checked-out branch short name, or the short HEAD hash when detached.
	CurrentBranch string `json:"current_branch"`
	Detached      bool   `json:"detached"`
	// Branches lists all local branches including those excluded by config (see ExcludedByConfig).
	Branches []reportBranchEntry `json:"branches"`
}

// report is the top-level JSON structure for the report subcommand.
type report struct {
	// Repos are the repositories shown in the TUI repository pane (dirty or diverged),
	// in alphabetical order.
	Repos []reportRepo `json:"repos"`
}

func buildReport(mgs *scanner.MultiGitStatus) report {
	paths := mgs.SortedRepoPaths()
	repos := make([]reportRepo, 0, len(paths))

	for _, path := range paths {
		rs, ok := mgs.Get(path)
		if !ok {
			continue
		}

		// Files
		files := make([]reportFileEntry, 0, len(rs.Porcelain.Entries))
		for _, e := range rs.Porcelain.Entries {
			files = append(files, reportFileEntry{
				Staging:      string(e.Staging),
				Worktree:     string(e.Worktree),
				Path:         e.Path,
				OriginalPath: e.OriginalPath,
			})
		}

		// Build a set of branch names that survived FilterLocalOnlyForConfig.
		filteredSet := make(map[string]struct{}, len(rs.FilteredBranches))
		for _, lb := range rs.FilteredBranches {
			filteredSet[lb.Name] = struct{}{}
		}

		branches := make([]reportBranchEntry, 0, len(rs.Branches))
		for _, lb := range rs.Branches {
			_, survived := filteredSet[lb.Name]

			branches = append(branches, reportBranchEntry{
				LocalBranchRef: lb,
				ShownInTUI:     survived,
				IsLocalOnly:    lb.IsLocalOnly(),
			})
		}

		repos = append(repos, reportRepo{
			Path:          path,
			IsClean:       rs.Porcelain.ToGitStatus().IsClean(),
			Files:         files,
			CurrentBranch: rs.Branch,
			Detached:      rs.Detached,
			Branches:      branches,
		})
	}

	return report{Repos: repos}
}

func runReport(config *scanner.Config, outputFile string) error {
	mgs, err := scanner.Scan(config)
	if err != nil {
		return err
	}

	r := buildReport(mgs)
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		err = enc.Encode(r)
		if err != nil {
			return err
		}
	}

	printReportSummary(r)
	return nil
}

func printReportSummary(r report) {
	if len(r.Repos) == 0 {
		fmt.Println("No dirty or diverged repositories found.")
		return
	}
	for _, repo := range r.Repos {
		fmt.Println(repo.Path)
		for _, b := range repo.Branches {
			if b.ShownInTUI {
				fmt.Printf("  %s\n", b.GetDisplayName())
			}
		}
	}
}

func reportCommand() *cli.Command {
	return &cli.Command{
		Name:  "report",
		Usage: "Report on dirty/diverged repositories (equivalent to TUI state)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output-file",
				Aliases: []string{"o"},
				Usage:   "Write json report to this file",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			config, err := scanner.ParseConfigFile(cmd.Root().String("config"), defaultConfig)
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
			return runReport(config, cmd.String("output-file"))
		},
	}
}
