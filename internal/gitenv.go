package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

// checkGitEnv verifies the git environment is safe to proceed.
// It checks for a clean working tree and that the current branch is not
// in the protected list. Both checks are bypassed by -force.
func (r *runner) checkGitEnv() error {
	if r.cfg.Force {
		return nil
	}

	dir := r.rootDir()

	out, err := r.git(dir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("git tree is dirty; commit or stash changes first (or use -force)")
	}

	branch, err := r.git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	branch = strings.TrimSpace(branch)

	for _, p := range strings.Split(r.cfg.Protected, ",") {
		if strings.TrimSpace(p) == branch {
			return fmt.Errorf("on protected branch %q; switch to a feature branch or use -force", branch)
		}
	}

	return nil
}

func defaultGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
