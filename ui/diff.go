package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// diffModeLabel returns the current Diff pane mode label.
func (m *model) diffModeLabel() string {
	if m.diffMode == diffModeStaged {
		return "Staged"
	}
	return "Worktree"
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

	added := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	deleted := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	hunk := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	file := lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			lines[i] = header.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = hunk.Render(line)
		case strings.HasPrefix(line, "+++ "), strings.HasPrefix(line, "--- "):
			lines[i] = file.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = added.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = deleted.Render(line)
		case strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "new file mode "),
			strings.HasPrefix(line, "deleted file mode "),
			strings.HasPrefix(line, "similarity index "),
			strings.HasPrefix(line, "rename from "),
			strings.HasPrefix(line, "rename to "):
			lines[i] = meta.Render(line)
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
