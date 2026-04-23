package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// diffModeLabel returns the current Diff pane mode label.
func (m *model) diffModeLabel() string {
	if m.diffMode == diffModeStaged {
		return "Staged"
	}
	return "Worktree"
}

// diffPaneBorderTitle is the Diff pane top-border label: pane name when focused,
// plus Worktree and Staged with the active diff mode emphasized.
func (m *model) diffPaneBorderTitle() string {
	// Not lipgloss 214: that matches the focused-pane border accent (see view_test).
	return diffPaneTopBorderLabel(m.focus == paneDiff, m.diffMode == diffModeWorktree)
}

// refreshDiffContent reloads the visible diff text when needed.
func (m *model) refreshDiffContent() {
	if !m.diffNeedsRefresh {
		return
	}
	m.diffNeedsRefresh = false
	m.diffErr = nil

	repo := m.currentRepo()
	if repo == "" {
		m.diffContent = "(select a repository to view diffs)"
		return
	}

	path := m.selectedStatusPath()
	out, err := gitDiff(repo, path, m.diffMode == diffModeStaged)
	if err != nil {
		m.diffErr = err
		if strings.TrimSpace(out) == "" {
			m.diffContent = fmt.Sprintf("git diff failed: %v", err)
			return
		}
	}
	if strings.TrimSpace(out) == "" {
		target := "all changed files"
		if path != "" {
			target = path
		}
		m.diffContent = fmt.Sprintf("(no %s diff for %s)", strings.ToLower(m.diffModeLabel()), target)
		return
	}
	m.diffContent = styleDiffContent(out)
}

// styleDiffContent colorizes git diff output for terminal display.
func styleDiffContent(raw string) string {
	if raw == "" {
		return ""
	}

	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			lines[i] = diffStyleHeader.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = diffStyleHunk.Render(line)
		case strings.HasPrefix(line, "+++ "), strings.HasPrefix(line, "--- "):
			lines[i] = diffStyleFile.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = diffStyleAdded.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = diffStyleDeleted.Render(line)
		case strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "new file mode "),
			strings.HasPrefix(line, "deleted file mode "),
			strings.HasPrefix(line, "similarity index "),
			strings.HasPrefix(line, "rename from "),
			strings.HasPrefix(line, "rename to "):
			lines[i] = diffStyleMeta.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

// gitDiff runs git diff for a repository and optional file path.
func gitDiff(repo, path string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	if path != "" {
		args = append(args, "--", path)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
