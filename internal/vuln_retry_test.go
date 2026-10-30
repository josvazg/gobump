package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGovulncheck_retriesAfterNewerPatchFromRefetch(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/m\n\ngo 1.22.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)

	var fetchCalls atomic.Int32
	var vulnCalls atomic.Int32
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			n := fetchCalls.Add(1)
			if n == 1 {
				return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
			}
			return []Release{{Version: "go1.22.4", Date: old, Stable: true}}, nil
		},
		goCmd: func(string, ...string) (string, error) { return "", nil },
		runShell: func(string, string) error {
			return nil
		},
		checkVulns: func(string) (VulnReport, error) {
			v := vulnCalls.Add(1)
			if v == 1 {
				report := VulnReport{Findings: []Finding{{
					OSV: "GO-2023-1988", Module: "stdlib",
					Package: "net/http", FixedVersion: "go1.22.4",
				}}}
				return report, errors.New("vuln on first check")
			}
			return VulnReport{}, nil
		},
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	got, err := ReadGoVersion(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.22.4" {
		t.Errorf("go version after vuln retry = %q, want 1.22.4", got)
	}
	if fetchCalls.Load() < 2 {
		t.Errorf("expected at least 2 release fetches, got %d", fetchCalls.Load())
	}
}

// TestGovulncheck_stdlibRemediationDoesNotJumpLines is a regression test:
// when the refetched release feed has no stable release in the current
// major.minor line, stdlib remediation must fail with a clear error instead
// of falling back to the latest stable overall (which could pull in a
// release still in soak).
func TestGovulncheck_stdlibRemediationDoesNotJumpLines(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/m\n\ngo 1.22.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			// Only a stable release on a different major.minor line.
			return []Release{{Version: "go1.23.0", Date: old, Stable: true}}, nil
		},
		goCmd:    func(string, ...string) (string, error) { return "", nil },
		runShell: func(string, string) error { return nil },
		checkVulns: func(string) (VulnReport, error) {
			return VulnReport{Findings: []Finding{{
				OSV: "GO-2023-1988", Module: "stdlib",
				Package: "net/http", FixedVersion: "go1.22.4",
			}}}, errors.New("vuln")
		},
	}

	err := r.fixVulns(context.Background(), modPath, dir, "1.22.3")
	if err == nil {
		t.Fatal("fixVulns returned nil; want error when no stable release matches the current line")
	}
	const wantErr = "no stable Go release in line 1.22.3"
	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("error %q does not contain %q", err.Error(), wantErr)
	}
	got, err := ReadGoVersion(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.22.3" {
		t.Errorf("go version after failed stdlib remediation = %q, want unchanged 1.22.3", got)
	}
}

// TestGovulncheck_runsWhenToolchainAlreadyLatest ensures we still run govulncheck (+ tidy)
// when go.mod already matches latest stable (vulnerability-only path).

func TestGovulncheck_runsWhenToolchainAlreadyLatest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.22.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour)
	var vulnCalls atomic.Int32
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "echo ok"},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
		goCmd:    func(string, ...string) (string, error) { return "", nil },
		runShell: func(string, string) error { return nil },
		checkVulns: func(string) (VulnReport, error) {
			vulnCalls.Add(1)
			return VulnReport{}, nil
		},
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}
	if vulnCalls.Load() == 0 {
		t.Error("govulncheck should run when go.mod already matches latest stable")
	}
}
