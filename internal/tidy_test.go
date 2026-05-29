package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunner_runsModTidyAfterBump(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var tidyCalled bool
	old := time.Now().Add(-100 * 24 * time.Hour)

	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, Force: true},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd: func(goDir string, args ...string) (string, error) {
			if args[0] == "mod" && args[1] == "tidy" {
				tidyCalled = true
			}
			return "", nil
		},
		runShell:    func(string, string) error { return nil },
		git:         func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) (VulnReport, error) { return VulnReport{}, nil },
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	if !tidyCalled {
		t.Error("go mod tidy was not called after bumping")
	}
}

func TestRunner_noTidyWhenSoaking(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var tidyCalled bool
	// Fresh release: soak blocks bump and we are not yet on latest stable — no tidy.
	fresh := time.Now().Add(-10 * 24 * time.Hour)

	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, Force: true},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: fresh, Stable: true}}, nil
		},
		goCmd: func(_ string, args ...string) (string, error) {
			if args[0] == "mod" && args[1] == "tidy" {
				tidyCalled = true
			}
			return "", nil
		},
		runShell:    func(string, string) error { return nil },
		git:         func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) (VulnReport, error) { return VulnReport{}, nil },
	}

	r.run(context.Background())
	if tidyCalled {
		t.Error("go mod tidy should not run when soak blocks bump and toolchain is not at latest")
	}
}
