package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boyvinall/dirtygit/scanner"
)

// openCurrentRepo runs config edit.command for the selected repository.
func (m *model) openCurrentRepo() {
	repo := m.currentRepo()
	if repo == "" || m.config == nil {
		return
	}
	argv, err := m.config.EditArgv(repo)
	if err != nil {
		log.Printf("edit: %v", err)
		return
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Run(); err != nil {
		log.Printf("edit %q: %v", argv[0], err)
	}
}

func gitAdd(repo, path string) error {
	if repo == "" {
		return fmt.Errorf("no repository selected")
	}
	cmd := exec.Command("git", "add", "--", path)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return fmt.Errorf("%w: %s", err, s)
		}
		return err
	}
	return nil
}

// statusPathUnderRepo resolves a git status path (slash-separated, relative to the repository
// root) to an absolute filesystem path that must remain inside the repository directory.
func statusPathUnderRepo(repo, gitRelPath string) (string, error) {
	if repo == "" || strings.TrimSpace(gitRelPath) == "" {
		return "", fmt.Errorf("empty repo or path")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	rel := filepath.FromSlash(strings.TrimSpace(gitRelPath))
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "" || rel == "." {
		return "", fmt.Errorf("invalid path")
	}
	joined := filepath.Join(absRepo, rel)
	cleaned := filepath.Clean(joined)
	outRel, err := filepath.Rel(absRepo, cleaned)
	if err != nil {
		return "", err
	}
	if outRel == ".." || strings.HasPrefix(outRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository")
	}
	return cleaned, nil
}

// removeStatusPathOnDisk deletes a repo-relative path from the working tree using os.RemoveAll.
func removeStatusPathOnDisk(repo, gitRelPath string) error {
	abs, err := statusPathUnderRepo(repo, gitRelPath)
	if err != nil {
		return err
	}
	return os.RemoveAll(abs)
}

func gitResetPath(repo, path string) error {
	if repo == "" {
		return fmt.Errorf("no repository selected")
	}
	cmd := exec.Command("git", "reset", "HEAD", "--", path)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return fmt.Errorf("%w: %s", err, s)
		}
		return err
	}
	return nil
}

// gitCheckoutHeadPath restores the path's index and working tree content from HEAD
// (same as `git checkout HEAD -- <path>`).
func gitCheckoutHeadPath(repo, path string) error {
	if repo == "" {
		return fmt.Errorf("no repository selected")
	}
	cmd := exec.Command("git", "checkout", "HEAD", "--", path)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return fmt.Errorf("%w: %s", err, s)
		}
		return err
	}
	return nil
}

// refreshRepoStatusAfterGit re-runs status for the current repo so the UI matches git.
func (m *model) refreshRepoStatusAfterGit() {
	repo := m.currentRepo()
	if repo == "" || m.config == nil {
		return
	}
	rs, include, err := scanner.StatusForRepo(m.config, repo)
	if err != nil {
		log.Printf("refresh repo status: %v", err)
		return
	}
	if include {
		m.repositories.Set(repo, rs)
	} else {
		m.repositories.Delete(repo)
		m.repoList = m.repositories.SortedRepoPaths()
		if m.cursor >= len(m.repoList) {
			m.cursor = max(0, len(m.repoList)-1)
		}
	}
}
