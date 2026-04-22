package scanner

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// haveMergeBase reports whether git finds a common ancestor for the two commits.
func haveMergeBase(d, commitA, commitB string) (bool, error) {
	cmd := exec.Command("git", "merge-base", commitA, commitB)
	cmd.Dir = d
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base: %w", err)
}

// listLocalBranches returns all refs/heads sorted by name. When detached is false,
// currentName is the checked-out branch name and that row has Current set.
func listLocalBranches(d, currentName string, detached bool) ([]LocalBranchRef, error) {
	out, err := runGit(d, "for-each-ref", "refs/heads", "--sort=refname",
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

func runGit(d string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = d
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", d, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func currentBranch(d string) (name string, detached bool, err error) {
	name, err = runGit(d, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return name, false, nil
	}
	head, headErr := runGit(d, "rev-parse", "--short", "HEAD")
	if headErr != nil {
		return "", false, headErr
	}
	return head, true, nil
}

func listRemotes(d string) ([]string, error) {
	out, err := runGit(d, "remote")
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

func refTip(d, ref string) (hash string, unix int64, exists bool, err error) {
	out, err := runGit(d, "show", "-s", "--format=%H %ct", ref)
	if err != nil {
		return "", 0, false, nil
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

func uniqueCommitInfo(d, ref string, otherRefs []string) (count int, newestUnix int64, err error) {
	if len(otherRefs) == 0 {
		return 0, 0, nil
	}
	args := []string{"rev-list", "--count", ref, "--not"}
	args = append(args, otherRefs...)
	out, err := runGit(d, args...)
	if err != nil {
		return 0, 0, err
	}
	count, err = strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, 0, err
	}
	if count == 0 {
		return 0, 0, nil
	}

	args = []string{"log", "-1", "--format=%ct", ref, "--not"}
	args = append(args, otherRefs...)
	out, err = runGit(d, args...)
	if err != nil {
		return count, 0, err
	}
	if strings.TrimSpace(out) == "" {
		return count, 0, nil
	}
	newestUnix, err = strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return count, 0, err
	}
	return count, newestUnix, nil
}

// computeBranchLocations compares refs/heads/<branchName> to refs/remotes/<r>/<branchName>
// for each configured remote and fills UniqueCount / NewestUniqueUnix per location.
func computeBranchLocations(d, branchName string, remotes []string) ([]BranchLocation, string, error) {
	locations := make([]BranchLocation, 0, 1+len(remotes))
	locations = append(locations, BranchLocation{
		Name: "local",
		Ref:  "refs/heads/" + branchName,
	})
	for _, remote := range remotes {
		locations = append(locations, BranchLocation{
			Name: remote,
			Ref:  "refs/remotes/" + remote + "/" + branchName,
		})
	}

	for i := range locations {
		hash, unix, exists, err := refTip(d, locations[i].Ref)
		if err != nil {
			return nil, "", err
		}
		locations[i].Exists = exists
		locations[i].TipHash = hash
		locations[i].TipUnix = unix
	}

	existingRefs := make([]string, 0, len(locations))
	for _, loc := range locations {
		if loc.Exists {
			existingRefs = append(existingRefs, loc.Ref)
		}
	}
	for i := range locations {
		if !locations[i].Exists {
			continue
		}
		others := make([]string, 0, len(existingRefs)-1)
		for _, ref := range existingRefs {
			if ref != locations[i].Ref {
				others = append(others, ref)
			}
		}
		count, newestUnix, err := uniqueCommitInfo(d, locations[i].Ref, others)
		if err != nil {
			return nil, "", err
		}
		locations[i].UniqueCount = count
		locations[i].NewestUniqueUnix = newestUnix
	}

	if len(locations) > 0 && locations[0].Exists {
		localRef := locations[0].Ref
		for i := 1; i < len(locations); i++ {
			if !locations[i].Exists {
				continue
			}
			related, err := haveMergeBase(d, locations[0].TipHash, locations[i].TipHash)
			if err != nil {
				return nil, "", err
			}
			if !related {
				locations[i].HistoriesUnrelated = true
				continue
			}
			incoming, _, err := uniqueCommitInfo(d, locations[i].Ref, []string{localRef})
			if err != nil {
				return nil, "", err
			}
			outgoing, _, err := uniqueCommitInfo(d, localRef, []string{locations[i].Ref})
			if err != nil {
				return nil, "", err
			}
			locations[i].Incoming = incoming
			locations[i].Outgoing = outgoing
		}
	}

	newestLocation := ""
	var newestUnix int64
	for _, loc := range locations {
		if loc.NewestUniqueUnix > newestUnix {
			newestUnix = loc.NewestUniqueUnix
			newestLocation = loc.Name
		}
	}
	return locations, newestLocation, nil
}

func GitBranchStatus(d string) (BranchStatus, error) {
	branch, detached, err := currentBranch(d)
	if err != nil {
		return BranchStatus{}, err
	}
	if detached {
		locals, listErr := listLocalBranches(d, branch, true)
		if listErr != nil {
			return BranchStatus{}, listErr
		}
		return BranchStatus{Branch: branch, Detached: true, LocalBranches: locals}, nil
	}

	remotes, err := listRemotes(d)
	if err != nil {
		return BranchStatus{}, err
	}

	locals, err := listLocalBranches(d, branch, false)
	if err != nil {
		return BranchStatus{}, err
	}

	if len(locals) == 0 {
		locations, newestLocation, err := computeBranchLocations(d, branch, remotes)
		if err != nil {
			return BranchStatus{}, err
		}
		return BranchStatus{
			Branch:         branch,
			Detached:       false,
			Locations:      locations,
			NewestLocation: newestLocation,
			LocalBranches:  nil,
		}, nil
	}

	var topLocations []BranchLocation
	var topNewest string
	for i := range locals {
		locs, newest, err := computeBranchLocations(d, locals[i].Name, remotes)
		if err != nil {
			return BranchStatus{}, err
		}
		locals[i].Locations = locs
		if locals[i].Current {
			topLocations = locs
			topNewest = newest
		}
	}

	if topLocations == nil {
		var err2 error
		topLocations, topNewest, err2 = computeBranchLocations(d, branch, remotes)
		if err2 != nil {
			return BranchStatus{}, err2
		}
	}

	return BranchStatus{
		Branch:         branch,
		Detached:       false,
		Locations:      topLocations,
		NewestLocation: topNewest,
		LocalBranches:  locals,
	}, nil
}
