package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestLibBump(t *testing.T) {
	net := Finding{OSV: "GO-2023-2041", FixedVersion: "v0.18.0", Module: "golang.org/x/net", Package: "golang.org/x/net/http2"}
	bar := Finding{OSV: "GO-2023-9000", FixedVersion: "v1.2.3", Module: "github.com/foo/bar", Package: "github.com/foo/bar/pkg"}
	netLow := Finding{OSV: "GO-2023-2040", FixedVersion: "v0.17.0", Module: "golang.org/x/net", Package: "golang.org/x/net/http2"}

	tests := []struct {
		name          string
		startVer      string
		latestVer     string
		firstFindings []Finding
		vulnPersists  bool   // govulncheck always returns error (re-run also fails)
		wantCode      int
		wantReverted  bool
		wantGetArgs   []string // expected "module@version" args to go get; nil = skip check
		wantMinCalls  int32    // minimum govulncheck invocation count
		wantGoVer     string   // expected go directive after run; "" = skip check
	}{
		{
			name:          "single library finding — go get called and govulncheck re-run",
			startVer:      "1.22.3",
			latestVer:     "go1.22.3",
			firstFindings: []Finding{net},
			wantGetArgs:   []string{"golang.org/x/net@v0.18.0"},
			wantMinCalls:  2,
		},
		{
			name:          "two different modules — go get for each",
			startVer:      "1.22.3",
			latestVer:     "go1.22.3",
			firstFindings: []Finding{net, bar},
			wantGetArgs:   []string{"golang.org/x/net@v0.18.0", "github.com/foo/bar@v1.2.3"},
		},
		{
			name:          "same module two CVEs — deduplicated at highest version",
			startVer:      "1.22.3",
			latestVer:     "go1.22.3",
			firstFindings: []Finding{netLow, net},
			wantGetArgs:   []string{"golang.org/x/net@v0.18.0"},
		},
		{
			// startVer < latestVer so processModule runs WriteGoVersion (dirtyNow=true),
			// enabling the revert assertion when re-run still fails.
			name:          "re-run still fails — reverts and exits 1",
			startVer:      "1.21.0",
			latestVer:     "go1.22.3",
			firstFindings: []Finding{net},
			vulnPersists:  true,
			wantCode:      1,
			wantReverted:  true,
		},
		{
			// processModule bumps go directive (needsPatch); runGovulncheckGate
			// then handles both lib go get and stdlib detection (already at latest,
			// so no extra bump needed).
			name:          "mixed stdlib and library — library go get applied",
			startVer:      "1.21.0",
			latestVer:     "go1.22.3",
			firstFindings: []Finding{
				{OSV: "GO-STDLIB", Module: "stdlib", FixedVersion: "go1.22.3", Package: "net/http"},
				net,
			},
			wantGetArgs: []string{"golang.org/x/net@v0.18.0"},
			wantGoVer:   "1.22.3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"),
				[]byte("module example.com/m\n\ngo "+tc.startVer+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-100 * 24 * time.Hour)

			var goCmds [][]string
			var reverted bool
			var calls atomic.Int32

			r := &runner{
				cfg:  Config{Soak: 90 * 24 * time.Hour, Force: true, TestCmd: "echo ok"},
				path: dir,
				fetchReleases: func(_ context.Context) ([]Release, error) {
					return []Release{{Version: tc.latestVer, Date: old, Stable: true}}, nil
				},
				goCmd: func(_ string, args ...string) (string, error) {
					goCmds = append(goCmds, append([]string{}, args...))
					return "", nil
				},
				runShell: func(string, string) error { return nil },
				git: func(_ string, args ...string) (string, error) {
					if len(args) > 0 && args[0] == "checkout" {
						reverted = true
					}
					return "", nil
				},
				govulncheck: func(string) (VulnReport, error) {
					n := calls.Add(1)
					if n == 1 || tc.vulnPersists {
						return VulnReport{Findings: tc.firstFindings}, errors.New("vulnerable")
					}
					return VulnReport{}, nil
				},
			}

			code := r.run(context.Background())

			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if tc.wantReverted && !reverted {
				t.Error("changes should be reverted")
			}
			if tc.wantMinCalls > 0 && calls.Load() < tc.wantMinCalls {
				t.Errorf("govulncheck calls = %d, want >= %d", calls.Load(), tc.wantMinCalls)
			}
			if tc.wantGetArgs != nil {
				var getArgs []string
				for _, cmd := range goCmds {
					if cmd[0] == "get" {
						getArgs = append(getArgs, cmd[1])
					}
				}
				if len(getArgs) != len(tc.wantGetArgs) {
					t.Fatalf("go get calls = %v, want %v", getArgs, tc.wantGetArgs)
				}
				for _, want := range tc.wantGetArgs {
					if !slices.Contains(getArgs, want) {
						t.Errorf("missing go get %s; got %v", want, getArgs)
					}
				}
			}
			if tc.wantGoVer != "" {
				got, _ := ReadGoVersion(filepath.Join(dir, "go.mod"))
				if got != tc.wantGoVer {
					t.Errorf("go version = %q, want %q", got, tc.wantGoVer)
				}
			}
		})
	}
}
