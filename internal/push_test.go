package internal

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func bumpedRunner(t *testing.T) (*runner, *[][]string, *[]string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-100 * 24 * time.Hour)
	var gitCalls [][]string
	var shellCmds []string

	r := &runner{
		cfg: Config{
			Soak:     90 * 24 * time.Hour,
			Force:    true,
			TestCmd:  "echo ok",
			Push:     true,
			Protected: "main,master,trunk",
		},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) error { return nil },
		runShell: func(_, cmd string) error {
			shellCmds = append(shellCmds, cmd)
			return nil
		},
		git: func(_ string, args ...string) (string, error) {
			gitCalls = append(gitCalls, append([]string{}, args...))
			return "", nil
		},
	}
	return r, &gitCalls, &shellCmds
}

func TestPush_stagesCommitsAndPushes(t *testing.T) {
	r, gitCalls, _ := bumpedRunner(t)

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	var hasAdd, hasCommit, hasPush bool
	for _, call := range *gitCalls {
		switch {
		case call[0] == "add" && slices.Contains(call, "go.mod"):
			hasAdd = true
		case call[0] == "commit":
			hasCommit = true
		case call[0] == "push":
			hasPush = true
		}
	}
	if !hasAdd {
		t.Error("git add go.mod go.sum was not called")
	}
	if !hasCommit {
		t.Error("git commit was not called")
	}
	if !hasPush {
		t.Error("git push was not called")
	}
}

func TestPush_commitMessageIsConventional(t *testing.T) {
	r, gitCalls, _ := bumpedRunner(t)
	r.run(context.Background())

	for _, call := range *gitCalls {
		if call[0] == "commit" {
			if !slices.Contains(call, "chore: gobump updates") {
				t.Errorf("unexpected commit message in: %v", call)
			}
			return
		}
	}
	t.Error("no git commit call found")
}

func TestPush_skippedWhenFlagNotSet(t *testing.T) {
	r, gitCalls, _ := bumpedRunner(t)
	r.cfg.Push = false

	r.run(context.Background())

	for _, call := range *gitCalls {
		if call[0] == "commit" || call[0] == "push" {
			t.Errorf("unexpected git %s called without -push", call[0])
		}
	}
}

func TestPR_runsAfterPush(t *testing.T) {
	r, _, shellCmds := bumpedRunner(t)
	r.cfg.PR = "gh pr create --fill"

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	if len(*shellCmds) < 2 {
		t.Fatalf("expected at least 2 shell commands (test + pr), got %v", *shellCmds)
	}
	last := (*shellCmds)[len(*shellCmds)-1]
	if last != "gh pr create --fill" {
		t.Errorf("last shell command = %q, want pr command", last)
	}
}

func TestPR_skippedWhenEmpty(t *testing.T) {
	r, _, shellCmds := bumpedRunner(t)
	r.cfg.PR = ""

	r.run(context.Background())

	for _, cmd := range *shellCmds {
		if cmd == "gh pr create --fill" {
			t.Error("pr command should not run when -pr is empty")
		}
	}
}
