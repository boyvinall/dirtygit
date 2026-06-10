package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/boyvinall/dirtygit/scanner"
)

// reportLocationEntry mirrors scanner.BranchLocation for JSON output.
type reportLocationEntry struct {
	Name               string `json:"name"`
	Exists             bool   `json:"exists"`
	TipHash            string `json:"tip_hash"`
	TipUnix            int64  `json:"tip_unix"`
	UniqueCount        int    `json:"unique_count"`
	Incoming           int    `json:"incoming"`
	Outgoing           int    `json:"outgoing"`
	HistoriesUnrelated bool   `json:"histories_unrelated"`
}

// reportBranchEntry is one local branch row in the report.
type reportBranchEntry struct {
	Name string `json:"name"`
	// TipHash is the full object name of the local ref tip.
	TipHash string `json:"tip_hash"`
	TipUnix int64  `json:"tip_unix"`
	Current bool   `json:"current"`
	// ShownInTUI is true when this branch appears in the TUI Branches pane for its
	// repository (HasTipMismatchAcrossRemotes or listed under branches.default).
	ShownInTUI bool `json:"shown_in_tui"`
	// ExcludedByConfig is true when this branch was removed from the branch list by
	// FilterLocalOnlyForConfig (branches.hidelocalonly.regex matched and the branch
	// is not in branches.default). Such branches are never shown in the TUI.
	ExcludedByConfig bool                  `json:"excluded_by_config"`
	IsLocalOnly      bool                  `json:"is_local_only"`
	Locations        []reportLocationEntry `json:"locations"`
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
	Path string `json:"path"`
	// InclusionReasons lists why this repository appears in the TUI (non-clean or
	// local/remote mismatch). Mirrors scanner.RepoInclusionReasons.
	InclusionReasons []string `json:"inclusion_reasons"`
	IsClean          bool     `json:"is_clean"`
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

func buildReport(config *scanner.Config, mgs *scanner.MultiGitStatus) report {
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

		// Branches — get the unfiltered list so we can annotate ExcludedByConfig.
		unfiltered, err := scanner.GitBranchStatus(path)
		if err != nil {
			// Fall back to the already-filtered branches from the scan result.
			unfiltered = rs.Branches
		}

		// Build a set of branch names that survived FilterLocalOnlyForConfig.
		filteredSet := make(map[string]struct{}, len(rs.Branches.LocalBranches))
		for _, lb := range rs.Branches.LocalBranches {
			filteredSet[lb.Name] = struct{}{}
		}

		branches := make([]reportBranchEntry, 0, len(unfiltered.LocalBranches))
		for _, lb := range unfiltered.LocalBranches {
			_, survived := filteredSet[lb.Name]
			excludedByConfig := !survived

			always := config != nil && config.AlwaysListBranch(lb.Name)
			shownInTUI := !excludedByConfig && (lb.HasTipMismatchAcrossRemotes() || always)

			locs := make([]reportLocationEntry, 0, len(lb.Locations))
			for _, loc := range lb.Locations {
				locs = append(locs, reportLocationEntry{
					Name:               loc.Name,
					Exists:             loc.Exists,
					TipHash:            loc.TipHash,
					TipUnix:            loc.TipUnix,
					UniqueCount:        loc.UniqueCount,
					Incoming:           loc.Incoming,
					Outgoing:           loc.Outgoing,
					HistoriesUnrelated: loc.HistoriesUnrelated,
				})
			}

			branches = append(branches, reportBranchEntry{
				Name:             lb.Name,
				TipHash:          lb.TipHash,
				TipUnix:          lb.TipUnix,
				Current:          lb.Current,
				ShownInTUI:       shownInTUI,
				ExcludedByConfig: excludedByConfig,
				IsLocalOnly:      lb.IsLocalOnly(),
				Locations:        locs,
			})
		}

		repos = append(repos, reportRepo{
			Path:             path,
			InclusionReasons: scanner.RepoInclusionReasons(rs),
			IsClean:          rs.IsClean(),
			Files:            files,
			CurrentBranch:    rs.Branches.Branch,
			Detached:         rs.Branches.Detached,
			Branches:         branches,
		})
	}

	return report{Repos: repos}
}

func runReport(config *scanner.Config) error {
	mgs, err := scanner.Scan(config)
	if err != nil {
		return err
	}

	r := buildReport(config, mgs)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// validateReport checks a report for consistency against the given config.
// Each returned string is one violation message.
func validateReport(config *scanner.Config, r report) []string {
	var violations []string
	for _, repo := range r.Repos {
		hasShown := false
		for _, b := range repo.Branches {
			if b.ShownInTUI {
				hasShown = true
			}
			if b.ExcludedByConfig && b.ShownInTUI {
				violations = append(violations, fmt.Sprintf("%s: branch %q is excluded by config but shown_in_tui=true", repo.Path, b.Name))
			}
			if b.IsLocalOnly {
				lb := scanner.LocalBranchRef{
					Name:      b.Name,
					Locations: []scanner.BranchLocation{{Name: "local", Exists: true}},
				}
				if config.ShouldHideLocalOnlyBranch(lb) && !config.AlwaysListBranch(b.Name) && !b.ExcludedByConfig {
					violations = append(violations, fmt.Sprintf("%s: branch %q matches hidelocalonly regex but excluded_by_config is not set", repo.Path, b.Name))
				}
			}
		}
		if !hasShown {
			violations = append(violations, fmt.Sprintf("%s: no branch has shown_in_tui=true", repo.Path))
		}
	}
	return violations
}

func validateReportCommand() *cli.Command {
	return &cli.Command{
		Name:   "validate-report",
		Usage:  "Validate a JSON report file from the report command",
		Hidden: true,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("expected exactly one argument: <report.json>")
			}
			config, err := scanner.ParseConfigFile(cmd.Root().String("config"), defaultConfig)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(cmd.Args().First())
			if err != nil {
				return err
			}
			var r report
			if err := json.Unmarshal(data, &r); err != nil {
				return fmt.Errorf("failed to parse report: %w", err)
			}
			violations := validateReport(config, r)
			for _, v := range violations {
				fmt.Fprintln(os.Stderr, v)
			}
			if len(violations) > 0 {
				return cli.Exit("", 1)
			}
			return nil
		},
	}
}

func reportCommand() *cli.Command {
	return &cli.Command{
		Name:  "report",
		Usage: "Print a JSON report of all dirty/diverged repositories (equivalent to TUI state)",
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
			return runReport(config)
		},
	}
}
