package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func customRunner(t *testing.T, custom, skip string) (*runner, *[]string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	var shellCmds []string
	r := &runner{
		cfg: Config{
			Soak:    90 * 24 * time.Hour,
			Force:   true,
			TestCmd: "echo ok",
			Custom:  custom,
			Skip:    skip,
		},
		skipSteps: parseSkip(skip),
		path:      dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) error { return nil },
		git:         func(string, ...string) (string, error) { return "", nil },
		runShell: func(_, cmd string) error {
			shellCmds = append(shellCmds, cmd)
			return nil
		},
	}
	return r, &shellCmds
}

func TestCustom_runsBeforeTestGate(t *testing.T) {
	r, cmds := customRunner(t, "make generate", "")

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	var customIdx, testIdx = -1, -1
	for i, cmd := range *cmds {
		if cmd == "make generate" {
			customIdx = i
		}
		if cmd == "echo ok" {
			testIdx = i
		}
	}
	if customIdx == -1 {
		t.Fatal("custom command was not run")
	}
	if testIdx == -1 {
		t.Fatal("test command was not run")
	}
	if customIdx >= testIdx {
		t.Errorf("custom (idx %d) must run before test gate (idx %d)", customIdx, testIdx)
	}
}

func TestCustom_skippedWhenNothingBumped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.22.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	var customRan bool
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, Force: true, Custom: "make generate"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) error { return nil },
		git:         func(string, ...string) (string, error) { return "", nil },
		runShell:    func(_, cmd string) error { customRan = customRan || cmd == "make generate"; return nil },
	}
	r.run(context.Background())
	if customRan {
		t.Error("custom command should not run when nothing was bumped")
	}
}

func TestCustom_skippedByFlag(t *testing.T) {
	r, cmds := customRunner(t, "make generate", "custom")
	r.run(context.Background())
	for _, cmd := range *cmds {
		if cmd == "make generate" {
			t.Error("custom command should not run with -skip=custom")
		}
	}
}

func TestCustom_failureRevertsAndExits(t *testing.T) {
	r, _ := customRunner(t, "make generate", "")
	r.runShell = func(_, cmd string) error {
		if cmd == "make generate" {
			return errors.New("generate failed")
		}
		return nil
	}
	var reverted bool
	r.git = func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "checkout" {
			reverted = true
		}
		return "", nil
	}

	if code := r.run(context.Background()); code != 1 {
		t.Fatalf("expected exit 1 on custom failure, got %d", code)
	}
	if !reverted {
		t.Error("changes should be reverted when custom command fails")
	}
}
