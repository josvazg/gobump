package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func vulnRunner(t *testing.T, vulnErr error, skip string) (*runner, *bool) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	called := false
	r := &runner{
		cfg:       Config{Soak: 90 * 24 * time.Hour, Force: true, TestCmd: "echo ok", Skip: skip},
		skipSteps: parseSkip(skip),
		path:      dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:    func(string, ...string) (string, error) { return "", nil },
		runShell: func(string, string) error { return nil },
		git:      func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) error {
			called = true
			return vulnErr
		},
	}
	return r, &called
}

func TestGovulncheck_runsAfterBump(t *testing.T) {
	r, called := vulnRunner(t, nil, "")
	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	if !*called {
		t.Error("govulncheck was not called after bump")
	}
}

func TestGovulncheck_failsAndRevertsOnVulns(t *testing.T) {
	r, _ := vulnRunner(t, errors.New("vulnerabilities found"), "")
	var reverted bool
	r.git = func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "checkout" {
			reverted = true
		}
		return "", nil
	}

	if code := r.run(context.Background()); code != 1 {
		t.Fatalf("expected exit 1 when govulncheck finds vulns, got %d", code)
	}
	if !reverted {
		t.Error("changes should be reverted when govulncheck fails")
	}
}

func TestGovulncheck_skippedWhenSoakingNotAtLatest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := time.Now().Add(-10 * 24 * time.Hour)
	called := false
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, Force: true},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: fresh, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		runShell:    func(string, string) error { return nil },
		git:         func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) error { called = true; return nil },
	}
	r.run(context.Background())
	if called {
		t.Error("govulncheck should not run when soak blocks bump and toolchain is not at latest")
	}
}

func TestGovulncheck_skippedByFlag(t *testing.T) {
	r, called := vulnRunner(t, errors.New("would fail"), "govulncheck")
	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d; -skip=govulncheck should suppress it", code)
	}
	if *called {
		t.Error("govulncheck should not run when skipped")
	}
}
