package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func skipRunner(t *testing.T, goVersion, latestVersion string, cfg Config) (*runner, string) {
	t.Helper()
	dir := t.TempDir()
	content := "module example.com/m\n\ngo " + goVersion + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	cfg.Force = true
	if cfg.TestCmd == "" {
		cfg.TestCmd = "echo ok"
	}
	cfg.Soak = 90 * 24 * time.Hour
	r := &runner{
		cfg:  cfg,
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: latestVersion, Date: old, Stable: true}}, nil
		},
		goCmd:       func(string, ...string) (string, error) { return "", nil },
		govulncheck: func(string) error { return nil },
		runShell:    func(string, string) error { return nil },
		git:         func(string, ...string) (string, error) { return "", nil },
	}
	return r, filepath.Join(dir, "go.mod")
}

// -skip=major tests

func TestSkipMajor_blocksCrossMinorBump(t *testing.T) {
	r, modPath := skipRunner(t, "1.26.3", "go1.27.0", Config{Skip: "major"})

	r.run(context.Background())

	got, _ := ReadGoVersion(modPath)
	if got != "1.26.3" {
		t.Errorf("go version should be unchanged with -skip=major, got %s", got)
	}
}

func TestSkipMajor_allowsPatchBump(t *testing.T) {
	r, modPath := skipRunner(t, "1.26.1", "go1.26.4", Config{Skip: "major"})

	r.run(context.Background())

	got, _ := ReadGoVersion(modPath)
	if got != "1.26.4" {
		t.Errorf("patch bump should proceed with -skip=major, got %s", got)
	}
}

func TestSkipMajor_allowsBumpWhenNotSet(t *testing.T) {
	r, modPath := skipRunner(t, "1.26.3", "go1.27.0", Config{})

	r.run(context.Background())

	got, _ := ReadGoVersion(modPath)
	if got != "1.27.0" {
		t.Errorf("cross-minor bump should proceed without -skip=major, got %s", got)
	}
}

func TestIsMajorBump(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"1.26.3", "go1.27.0", true},
		{"1.26.1", "go1.26.4", false},
		{"1.26.0", "go1.26.0", false},
		{"1.25.0", "go1.27.0", true},
	}
	for _, tc := range tests {
		got := isMajorBump(tc.current, tc.latest)
		if got != tc.want {
			t.Errorf("isMajorBump(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

// -dryrun tests

func TestDryRun_doesNotWriteGoMod(t *testing.T) {
	r, modPath := skipRunner(t, "1.26.0", "go1.27.0", Config{DryRun: true})

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("dry run returned %d", code)
	}

	got, _ := ReadGoVersion(modPath)
	if got != "1.26.0" {
		t.Errorf("dry run should not modify go.mod, got %s", got)
	}
}

func TestDryRun_doesNotRunTestGate(t *testing.T) {
	r, _ := skipRunner(t, "1.26.0", "go1.27.0", Config{DryRun: true, TestCmd: "false"})

	// -test=false would cause exit 1 if the test gate ran
	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("dry run should not run test gate, got exit %d", code)
	}
}

func TestDryRun_doesNotRunCustom(t *testing.T) {
	var customRan bool
	r, _ := skipRunner(t, "1.26.0", "go1.27.0", Config{DryRun: true, Custom: "make generate"})
	r.runShell = func(_, cmd string) error {
		if cmd == "make generate" {
			customRan = true
		}
		return nil
	}

	r.run(context.Background())
	if customRan {
		t.Error("dry run should not run custom command")
	}
}
