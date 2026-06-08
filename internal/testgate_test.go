package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTestGate_runsAfterBump(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-100 * 24 * time.Hour)
	var testRan bool

	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) (VulnReport, error) { return VulnReport{}, nil },
		runShell:    func(string, string) error { testRan = true; return nil },
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	if !testRan {
		t.Error("test command was not run after bump")
	}
}

func TestTestGate_failsWithDirtyFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-100 * 24 * time.Hour)

	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "false"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) (VulnReport, error) { return VulnReport{}, nil },
		runShell:    func(string, string) error { return errors.New("tests failed") },
	}

	if code := r.run(context.Background()); code != 1 {
		t.Fatalf("expected exit 1 on test failure, got %d", code)
	}
	// go.mod is left modified; the caller's VCS handles any rollback.
	got, _ := ReadGoVersion(filepath.Join(dir, "go.mod"))
	if got != "1.22.3" {
		t.Errorf("go.mod should be bumped even after test failure, got %q", got)
	}
}

func TestTestGate_skippedWhenNothingBumped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.22.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-100 * 24 * time.Hour)
	var testRan bool

	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) (VulnReport, error) { return VulnReport{}, nil },
		runShell:    func(string, string) error { testRan = true; return nil },
	}

	r.run(context.Background())
	if testRan {
		t.Error("test command should not run when nothing was bumped")
	}
}
