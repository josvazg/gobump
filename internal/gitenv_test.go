package internal

import (
	"testing"
)

func cleanGit(dir string, args ...string) (string, error) {
	switch args[0] {
	case "status":
		return "", nil // clean tree
	case "rev-parse":
		return "feature/bump\n", nil // non-protected branch
	}
	return "", nil
}

func dirtyGit(dir string, args ...string) (string, error) {
	switch args[0] {
	case "status":
		return " M go.mod\n", nil // dirty
	case "rev-parse":
		return "feature/bump\n", nil
	}
	return "", nil
}

func mainBranchGit(dir string, args ...string) (string, error) {
	switch args[0] {
	case "status":
		return "", nil
	case "rev-parse":
		return "main\n", nil
	}
	return "", nil
}

func TestCheckGitEnv_clean(t *testing.T) {
	r := &runner{cfg: Config{Protected: "main,master,trunk"}, git: cleanGit}
	if err := r.checkGitEnv(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckGitEnv_dirty(t *testing.T) {
	r := &runner{cfg: Config{Protected: "main,master,trunk"}, git: dirtyGit}
	if err := r.checkGitEnv(); err == nil {
		t.Error("expected error for dirty tree")
	}
}

func TestCheckGitEnv_dirtyForce(t *testing.T) {
	r := &runner{cfg: Config{Protected: "main,master,trunk", Force: true}, git: dirtyGit}
	if err := r.checkGitEnv(); err != nil {
		t.Errorf("force should bypass dirty check, got %v", err)
	}
}

func TestCheckGitEnv_protectedBranch(t *testing.T) {
	r := &runner{cfg: Config{Protected: "main,master,trunk"}, git: mainBranchGit}
	if err := r.checkGitEnv(); err == nil {
		t.Error("expected error on protected branch")
	}
}

func TestCheckGitEnv_protectedBranchForce(t *testing.T) {
	r := &runner{cfg: Config{Protected: "main,master,trunk", Force: true}, git: mainBranchGit}
	if err := r.checkGitEnv(); err != nil {
		t.Errorf("force should bypass branch protection, got %v", err)
	}
}

func TestCheckGitEnv_customProtected(t *testing.T) {
	onMainGit := func(dir string, args ...string) (string, error) {
		if args[0] == "rev-parse" {
			return "main\n", nil
		}
		return "", nil
	}
	// "main" not in protected list
	r := &runner{cfg: Config{Protected: "production,staging"}, git: onMainGit}
	if err := r.checkGitEnv(); err != nil {
		t.Errorf("main should not be protected here, got %v", err)
	}
}
