package internal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
		cfg:       Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok", Skip: skip},
		skipSteps: parseSkip(skip),
		path:      dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:    func(string, ...string) (string, error) { return "", nil },
		runShell: func(string, string) error { return nil },
		checkVulns: func(string) (VulnReport, error) {
			called = true
			return VulnReport{}, vulnErr
		},
	}
	return r, &called
}

func TestGovulncheck_notCalledAfterBumpToLatest(t *testing.T) {
	r, called := vulnRunner(t, nil, "")
	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	if *called {
		t.Error("govulncheck should not run after a successful bump to latest")
	}
}

func TestGovulncheck_failsAndExitsOnVulns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := time.Now().Add(-10 * 24 * time.Hour)
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: fresh, Stable: true}}, nil
		},
		goCmd:      func(string, ...string) (string, error) { return "", nil },
		runShell:   func(string, string) error { return nil },
		checkVulns: func(string) (VulnReport, error) { return VulnReport{}, errors.New("vulnerabilities found") },
	}

	if code := r.run(context.Background()); code != 1 {
		t.Fatalf("expected exit 1 when govulncheck finds vulns, got %d", code)
	}
}

func TestGovulncheck_runsWhileSoakingNotAtLatest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := time.Now().Add(-10 * 24 * time.Hour)
	called := false
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: fresh, Stable: true}}, nil
		},
		goCmd:      func(string, ...string) (string, error) { return "", nil },
		runShell:   func(string, string) error { return nil },
		checkVulns: func(string) (VulnReport, error) { called = true; return VulnReport{}, nil },
	}
	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	if !called {
		t.Error("govulncheck should run while soak blocks bump and toolchain is not at latest")
	}
}

// TestGovulncheck_handlesFindingsWithNilError is a regression test: a
// govulncheck invocation may exit successfully (nil error) while still
// reporting findings in its JSON output. The gate must inspect the report
// and apply fixes instead of treating nil as "clean".
func TestGovulncheck_handlesFindingsWithNilError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var vulnCalls atomic.Int32
	var getArgs []string
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour},
		path: dir,
		goCmd: func(_ string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "get" {
				getArgs = append(getArgs, args[1])
			}
			return "", nil
		},
		checkVulns: func(string) (VulnReport, error) {
			if vulnCalls.Add(1) == 1 {
				return VulnReport{Findings: []Finding{{
					OSV:          "GO-2024-0001",
					Module:       "example.com/dependency",
					FixedVersion: "v1.2.0",
				}}}, nil
			}
			return VulnReport{}, nil
		},
	}

	if err := r.fixVulns(context.Background(), filepath.Join(dir, "go.mod"), dir); err != nil {
		t.Fatalf("fixVulns returned error: %v", err)
	}
	if vulnCalls.Load() != 2 {
		t.Errorf("govulncheck calls = %d, want 2 (initial + re-run after fix)", vulnCalls.Load())
	}
	if len(getArgs) != 1 || getArgs[0] != "example.com/dependency@v1.2.0" {
		t.Errorf("go get calls = %v, want [example.com/dependency@v1.2.0]", getArgs)
	}
}

// TestGovulncheck_failsWhenFixLeavesFindings is a regression test: after an
// automated fix, the re-run govulncheck may exit successfully (nil error)
// while still reporting findings. The gate must fail instead of treating the
// nil error as "clean".
func TestGovulncheck_failsWhenFixLeavesFindings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var vulnCalls atomic.Int32
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour},
		path: dir,
		goCmd: func(string, ...string) (string, error) {
			return "", nil
		},
		checkVulns: func(string) (VulnReport, error) {
			if vulnCalls.Add(1) == 1 {
				// First call: a library finding plus an error so the
				// existing fixer path runs (goCmd succeeds).
				return VulnReport{Findings: []Finding{{
					OSV:          "GO-2024-0001",
					Module:       "example.com/dependency",
					FixedVersion: "v1.2.0",
				}}}, errors.New("vulnerabilities found")
			}
			// Second call: exits zero (nil error) but still has findings.
			return VulnReport{Findings: []Finding{{
				OSV:    "GO-2024-0002",
				Module: "example.com/other",
			}}}, nil
		},
	}

	if err := r.fixVulns(context.Background(), filepath.Join(dir, "go.mod"), dir); err == nil {
		t.Fatal("fixVulns returned nil; want non-nil when post-fix re-run still reports findings")
	}
}

// TestGovulncheck_reportsFindingCountsForEachScan proves fixVulns reports
// the finding count for every govulncheck scan it runs, including a zero
// count for the clean recheck after an automated fix.
func TestGovulncheck_reportsFindingCountsForEachScan(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := captureStderr(t)
	defer restore()

	var vulnCalls atomic.Int32
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour},
		path: dir,
		goCmd: func(string, ...string) (string, error) {
			return "", nil
		},
		checkVulns: func(string) (VulnReport, error) {
			if vulnCalls.Add(1) == 1 {
				// First scan: one finding plus an error so the existing
				// fixer path runs (goCmd succeeds).
				return VulnReport{Findings: []Finding{{
					OSV:          "GO-2024-0001",
					Module:       "example.com/dependency",
					FixedVersion: "v1.2.0",
				}}}, errors.New("vulnerabilities found")
			}
			// Second scan: clean after the automated go get fix.
			return VulnReport{}, nil
		},
	}

	if err := r.fixVulns(context.Background(), filepath.Join(dir, "go.mod"), dir); err != nil {
		t.Fatalf("fixVulns returned error: %v", err)
	}
	if vulnCalls.Load() != 2 {
		t.Errorf("govulncheck calls = %d, want 2 (initial + recheck after fix)", vulnCalls.Load())
	}

	out := restore()
	if !strings.Contains(out, "govulncheck findings: 1") {
		t.Errorf("stderr %q does not contain one-finding count for initial scan %q", out, "govulncheck findings: 1")
	}
	if !strings.Contains(out, "govulncheck findings: 0") {
		t.Errorf("stderr %q does not contain zero count for clean recheck %q", out, "govulncheck findings: 0")
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

// captureStderr redirects os.Stderr to a pipe and returns a restore func.
// Calling the returned func stops capturing and yields everything written
// to stderr so far. The restore is idempotent: only the first call actually
// stops the capture; subsequent calls return the same output. It is also
// registered with t.Cleanup so the pipe is always drained and stderr always
// restored, even when the test fails before calling restore itself.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Errorf("captureStderr: io.Copy: %v", err)
		}
		done <- buf.String()
	}()
	var (
		once  sync.Once
		out   string
		drain = func() {
			os.Stderr = old
			if err := w.Close(); err != nil {
				t.Errorf("captureStderr: close stderr pipe writer: %v", err)
			}
			out = <-done
			if err := r.Close(); err != nil {
				t.Errorf("captureStderr: close stderr pipe reader: %v", err)
			}
		}
	)
	t.Cleanup(func() { once.Do(drain) })
	return func() string {
		once.Do(drain)
		return out
	}
}
