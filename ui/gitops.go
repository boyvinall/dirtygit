package ui

import (
	"fmt"
	"log"
	"os/exec"
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
		m.repositories[repo] = rs
	} else {
		delete(m.repositories, repo)
		m.repoList = sortedRepoPaths(m.repositories)
		if m.cursor >= len(m.repoList) {
			m.cursor = max(0, len(m.repoList)-1)
		}
	}
}
