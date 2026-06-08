package internal_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josvazg/gobump/internal"
)

func TestFullStack(t *testing.T) {
	latest := installedGoVersion(t)
	latestVer := strings.TrimPrefix(latest, "go")
	old := time.Now().Add(-100 * day)
	fresh := time.Now().Add(-5 * day)

	tests := []struct {
		name       string
		startVer   string
		commitDate time.Time
		args       []string
		vulnFail   bool
		wantFail   bool
		wantVer    string
	}{
		{
			name:       "bump",
			startVer:   "1.21.0",
			commitDate: old,
			args:       []string{"-soak=1d", "-test=echo ok"},
			wantVer:    latestVer,
		},
		{
			name:       "soak not elapsed",
			startVer:   "1.21.0",
			commitDate: fresh,
			args:       []string{"-soak=90d", "-test=echo ok"},
			wantVer:    "1.21.0",
		},
		{
			name:       "govulncheck fails",
			startVer:   "1.21.0",
			commitDate: old,
			args:       []string{"-soak=1d", "-test=echo ok"},
			vulnFail:   true,
			wantFail:   true,
			wantVer:    latestVer, // bumped before govulncheck ran; no revert, VCS handles it
		},
		{
			name:       "already at latest",
			startVer:   latestVer,
			commitDate: old,
			args:       []string{"-soak=1d", "-test=echo ok"},
			wantVer:    latestVer,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			versions := []map[string]any{{"version": latest, "stable": true}}
			fw := newFakeWorld(t, "module example.com/m\n\ngo "+tc.startVer+"\n", versions, tc.commitDate)
			if tc.vulnFail {
				t.Setenv("FAKE_VULN_EXIT_CODE", "1")
			}

			code := internal.Run(context.Background(), append(tc.args, fw.dir), fw.env)

			if tc.wantFail && code == 0 {
				t.Fatal("Run returned 0; want non-zero")
			}
			if !tc.wantFail && code != 0 {
				t.Fatalf("Run returned %d; want 0", code)
			}
			if got := fw.goVer(t); got != tc.wantVer {
				t.Errorf("go version = %q, want %q", got, tc.wantVer)
			}
		})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// fakeWorld is a fully simulated e2e environment:
//   - a fake HTTP server standing in for go.dev/dl and the GitHub commits API
//   - a fake govulncheck binary whose exit code is controlled by the
//     FAKE_VULN_EXIT_CODE environment variable (default: 0 = clean)
type fakeWorld struct {
	dir    string
	srvURL string
}

// env maps GOBUMP_DL_URL / GOBUMP_COMMIT_URL to the fake server so the tool
// never hits the real internet.  All other keys fall through to os.Getenv.
func (fw *fakeWorld) env(key string) string {
	switch key {
	case "GOBUMP_DL_URL":
		return fw.srvURL
	case "GOBUMP_COMMIT_URL":
		return fw.srvURL + "/commits"
	}
	return os.Getenv(key)
}

// goVer reads the go directive from go.mod in fw.dir.
func (fw *fakeWorld) goVer(t *testing.T) string {
	t.Helper()
	v, err := internal.ReadGoVersion(filepath.Join(fw.dir, "go.mod"))
	if err != nil {
		t.Fatalf("reading go version: %v", err)
	}
	return v
}

// newFakeWorld builds the complete simulated environment:
//
//   - writes modContent to go.mod (plus an empty go.sum)
//   - installs a fake govulncheck script at the front of PATH
//   - starts a fake HTTP release server (reuses fakeDLServer from gorelease_test.go)
//
// All t.Setenv calls are automatically cleaned up by the test framework.
func newFakeWorld(t *testing.T, modContent string, versions []map[string]any, commitDate time.Time) *fakeWorld {
	t.Helper()

	dir := t.TempDir()
	binDir := t.TempDir()

	// Module files.
	write(t, filepath.Join(dir, "go.mod"), modContent)
	write(t, filepath.Join(dir, "go.sum"), "")

	// Fake govulncheck: exits with FAKE_VULN_EXIT_CODE (default 0).
	fakeBin := filepath.Join(binDir, "govulncheck")
	write(t, fakeBin, "#!/bin/sh\nexit ${FAKE_VULN_EXIT_CODE:-0}\n")
	if err := os.Chmod(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	// Prevent go mod tidy from downloading a different toolchain.
	t.Setenv("GOTOOLCHAIN", "local")

	// Fake HTTP release server.
	srv, _ := fakeDLServer(t, versions, commitDate)
	t.Cleanup(srv.Close)

	return &fakeWorld{dir: dir, srvURL: srv.URL}
}

// installedGoVersion returns the installed Go toolchain version as "go1.X.Y".
// Using the real installed version as the fake "latest" avoids go mod tidy
// trying to switch toolchains mid-test.
func installedGoVersion(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		t.Fatalf("go version: %v", err)
	}
	f := strings.Fields(string(out))
	if len(f) < 3 || !strings.HasPrefix(f[2], "go") {
		t.Fatalf("unexpected go version output: %q", out)
	}
	return f[2] // e.g. "go1.26.3"
}
