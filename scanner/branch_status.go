package scanner

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// haveMergeBase reports whether git finds a common ancestor for the two commits.
func haveMergeBase(dir, commitA, commitB string) (bool, error) {
	cmd := exec.Command("git", "merge-base", commitA, commitB)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base: %w", err)
}

// listLocalBranches returns all refs/heads sorted by name. When detached is false,
// currentName is the checked-out branch name and that row has Current set.
func listLocalBranches(dir, currentName string, detached bool) ([]LocalBranchRef, error) {
	out, err := runGit(dir, "for-each-ref", "refs/heads", "--sort=refname",
		"--format=%(refname:short)\t%(objectname)\t%(committerdate:unix)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	refs := make([]LocalBranchRef, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected for-each-ref line: %q", line)
		}
		unix, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse committer date for branch %q: %w", parts[0], err)
		}
		name := parts[0]
		cur := !detached && name == currentName
		refs = append(refs, LocalBranchRef{
			Name:    name,
			TipHash: parts[1],
			TipUnix: unix,
			Current: cur,
		})
	}
	return refs, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func currentBranch(dir string) (name string, detached bool, err error) {
	// First try symbolic-ref to get the branch name
	name, err = runGit(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return name, false, nil
	}

	// If that fails, try rev-parse to see if we are in a detached HEAD state
	// head, err := runGit(dir, "rev-parse", "--short", "HEAD")
	head, err := runGit(dir, "rev-parse", "HEAD")
	if err == nil {
		return head, true, nil
	}

	// If both fail, return error
	return "", false, err
}

func listRemotes(dir string) ([]string, error) {
	out, err := runGit(dir, "remote")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	remotes := strings.Split(out, "\n")
	sort.Strings(remotes)
	return remotes, nil
}

func refTip(dir, ref string) (hash string, unix int64, exists bool, err error) {
	out, err := runGit(dir, "show", "-s", "--format=%H %ct", ref)
	if err != nil {
		// git show exits 128 for unknown refs; treat that as "doesn't exist" rather
		// than an error so callers correctly see Exists=false for missing remotes.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 128 {
			return "", 0, false, nil
		}
		return "", 0, false, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return "", 0, false, fmt.Errorf("unable to parse commit metadata for ref %s", ref)
	}
	unix, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, false, err
	}
	return parts[0], unix, true, nil
}

// branchLocationRef returns the full ref for a location row (local heads vs
// refs/remotes) for use with git commands; it is not stored on [BranchLocation].
func branchLocationRef(locationName, branchName string) string {
	if locationName == "local" {
		return "refs/heads/" + branchName
	}
	return "refs/remotes/" + locationName + "/" + branchName
}

func uniqueCommitCount(dir, ref string, otherRefs []string) (count int, err error) {
	if len(otherRefs) == 0 {
		return 0, nil
	}
	args := []string{"rev-list", "--count", ref, "--not"}
	args = append(args, otherRefs...)
	out, err := runGit(dir, args...)
	if err != nil {
		return 0, err
	}
	count, err = strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return count, nil
}

// computeBranchLocations compares refs/heads/<branchName> to refs/remotes/<r>/<branchName>
// for each configured remote and fills UniqueCount per location.
func computeBranchLocations(dir, branchName string, remotes []string) ([]BranchLocation, error) {
	locations := make([]BranchLocation, 0, 1+len(remotes))
	locations = append(locations, BranchLocation{Name: "local"})
	for _, remote := range remotes {
		locations = append(locations, BranchLocation{Name: remote})
	}

	for i := range locations {
		ref := branchLocationRef(locations[i].Name, branchName)
		hash, unix, exists, err := refTip(dir, ref)
		if err != nil {
			return nil, err
		}
		locations[i].Exists = exists
		locations[i].TipHash = hash
		locations[i].TipUnix = unix
	}

	existingRefs := make([]string, 0, len(locations))
	for _, loc := range locations {
		if loc.Exists {
			existingRefs = append(existingRefs, branchLocationRef(loc.Name, branchName))
		}
	}
	for i := range locations {
		if !locations[i].Exists {
			continue
		}
		ref := branchLocationRef(locations[i].Name, branchName)
		others := make([]string, 0, len(existingRefs)-1)
		for _, otherRef := range existingRefs {
			if otherRef != ref {
				others = append(others, otherRef)
			}
		}
		count, err := uniqueCommitCount(dir, ref, others)
		if err != nil {
			return nil, err
		}
		locations[i].UniqueCount = count
	}

	if len(locations) > 0 && locations[0].Exists {
		localRef := branchLocationRef("local", branchName)
		for i := 1; i < len(locations); i++ {
			if !locations[i].Exists {
				continue
			}
			related, err := haveMergeBase(dir, locations[0].TipHash, locations[i].TipHash)
			if err != nil {
				return nil, err
			}
			if !related {
				locations[i].HistoriesUnrelated = true
				continue
			}
			remoteRef := branchLocationRef(locations[i].Name, branchName)
			incoming, err := uniqueCommitCount(dir, remoteRef, []string{localRef})
			if err != nil {
				return nil, err
			}
			outgoing, err := uniqueCommitCount(dir, localRef, []string{remoteRef})
			if err != nil {
				return nil, err
			}
			locations[i].Incoming = incoming
			locations[i].Outgoing = outgoing
		}
	}

	return locations, nil
}

func tipFromLocalBranchLocation(locations []BranchLocation) (hash string, unix int64) {
	for _, loc := range locations {
		if loc.Name == "local" && loc.Exists {
			return loc.TipHash, loc.TipUnix
		}
	}
	return "", 0
}

func GitBranchStatus(dir string) (branch string, detached bool, locals []LocalBranchRef, err error) {
	branch, detached, err = currentBranch(dir)
	if err != nil {
		return
	}

	var remotes []string
	remotes, err = listRemotes(dir)
	if err != nil {
		return
	}

	locals, err = listLocalBranches(dir, branch, detached)
	if err != nil {
		return
	}

	// if !detached && len(locals) == 0 {
	if len(locals) == 0 {
		var locations []BranchLocation
		locations, err = computeBranchLocations(dir, branch, remotes)
		if err != nil {
			return
		}
		tipHash, tipUnix := tipFromLocalBranchLocation(locations)
		locals = []LocalBranchRef{{
			Name:      branch,
			TipHash:   tipHash,
			TipUnix:   tipUnix,
			Current:   true,
			Locations: locations,
		}}
	}

	for i := range locals {
		var locs []BranchLocation
		locs, err = computeBranchLocations(dir, locals[i].Name, remotes)
		if err != nil {
			return
		}
		locals[i].Locations = locs
	}

	if detached {
		unix := int64(0)
		if raw, e := runGit(dir, "log", "-1", "--format=%ct", "HEAD"); e == nil {
			unix, _ = strconv.ParseInt(raw, 10, 64)
		}
		locals = append([]LocalBranchRef{
			{
				Name:    "HEAD",
				TipHash: branch,
				TipUnix: unix,
				Current: true,
				Locations: []BranchLocation{
					{
						Name:    "local",
						Exists:  true,
						TipHash: branch,
						TipUnix: unix,
					},
				},
			},
		}, locals...) // avoid mutating the original slice since it may be shared with the caller
	}

	return
}
