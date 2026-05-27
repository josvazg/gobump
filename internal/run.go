package internal

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Run is the program entry point. Returns an OS exit code.
func Run(ctx context.Context, args []string, env func(string) string) int {
	cfg, path, err := ParseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: %v\n", err)
		return 1
	}
	return newRunner(cfg, path).run(ctx)
}

type runner struct {
	cfg           Config
	path          string
	fetchReleases func(ctx context.Context) ([]Release, error)
	git           func(dir string, args ...string) (string, error)
	goCmd         func(dir string, args ...string) (string, error)
	runShell      func(dir, cmd string) error
	govulncheck   func(dir string) error
}

func newRunner(cfg Config, path string) *runner {
	return &runner{
		cfg:  cfg,
		path: path,
		fetchReleases: func(ctx context.Context) ([]Release, error) {
			return FetchReleases(ctx, nil, "", "")
		},
		git:      defaultGit,
		goCmd:    defaultGoCmd,
		runShell: defaultRunShell,
		govulncheck: func(dir string) error {
			return defaultRunShell(dir, "govulncheck ./...")
		},
	}
}

func (r *runner) shouldSkip(step string) bool {
	for _, s := range strings.Split(r.cfg.Skip, ",") {
		s = strings.TrimSpace(s)
		if s == "all" || s == step {
			return true
		}
	}
	return false
}

func defaultGoCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func defaultRunShell(dir, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *runner) run(ctx context.Context) int {
	if err := r.checkGitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: %v\n", err)
		return 1
	}

	modFiles, err := FindModFiles(r.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: discovering modules: %v\n", err)
		return 1
	}
	if len(modFiles) == 0 {
		fmt.Println("gobump: no go.mod files found")
		return 0
	}

	var bumpedDirs []string
	for _, modFile := range modFiles {
		dir := filepath.Dir(modFile)
		dirty, code := r.processModule(ctx, modFile)
		if code != 0 {
			rev := append([]string{}, bumpedDirs...)
			if dirty {
				rev = append(rev, dir)
			}
			r.revert(rev)
			return code
		}
		if dirty {
			bumpedDirs = append(bumpedDirs, dir)
		}
	}

	if len(bumpedDirs) == 0 {
		return 0
	}

	rootDir := r.path
	if rootDir == "" {
		rootDir = "."
	}

	if r.cfg.Custom != "" && !r.shouldSkip("custom") {
		fmt.Printf("running: %s\n", r.cfg.Custom)
		if err := r.runShell(rootDir, r.cfg.Custom); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: custom command failed: %v\n", err)
			r.revert(bumpedDirs)
			return 1
		}
	}

	fmt.Printf("running: %s\n", r.cfg.TestCmd)
	if err := r.runShell(rootDir, r.cfg.TestCmd); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: tests failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "gobump: reverting changes")
		r.revert(bumpedDirs)
		return 1
	}

	return r.finalize(bumpedDirs)
}

func readModSumBytes(modDir string) (mod, sum []byte) {
	mod, _ = os.ReadFile(filepath.Join(modDir, "go.mod"))
	sumPath := filepath.Join(modDir, "go.sum")
	b, err := os.ReadFile(sumPath)
	if err != nil {
		return mod, nil
	}
	return mod, b
}

func modSnapChanged(modDir string, origMod, origSum []byte) bool {
	m, s := readModSumBytes(modDir)
	return !bytes.Equal(m, origMod) || !bytes.Equal(s, origSum)
}

// runGovulncheckGate runs govulncheck; on failure it refetches stable releases and,
// if a newer patch exists, bumps the go directive and runs govulncheck again.
func (r *runner) runGovulncheckGate(ctx context.Context, modFile, modDir string) error {
	if r.shouldSkip("govulncheck") {
		return nil
	}
	firstErr := r.govulncheck(modDir)
	if firstErr == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "gobump: vulnerabilities found in %s: %v\n", modDir, firstErr)

	releases, err := r.fetchReleases(ctx)
	if err != nil {
		return fmt.Errorf("refetching releases after govulncheck failure: %w", err)
	}
	L2 := LatestStable(releases)
	if L2 == nil {
		return fmt.Errorf("no stable Go release information after refetch")
	}
	cur, err := ReadGoVersion(modFile)
	if err != nil {
		return err
	}
	if compareGoVersions(L2.Version, "go"+cur) <= 0 {
		fmt.Fprintln(os.Stderr, "gobump: govulncheck still failing; no newer Go patch — library updates not yet automated (fix manually)")
		return fmt.Errorf("govulncheck")
	}
	fmt.Fprintf(os.Stderr, "gobump: retrying with newer Go patch %s\n", strings.TrimPrefix(L2.Version, "go"))
	if err := WriteGoVersion(modFile, L2.Version); err != nil {
		return err
	}
	if _, err := r.goCmd(modDir, "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy in %s: %w", modDir, err)
	}
	if err := r.govulncheck(modDir); err != nil {
		fmt.Fprintln(os.Stderr, "gobump: govulncheck failed after patch bump — library updates not yet automated (fix manually)")
		return err
	}
	return nil
}

// processModule updates a single go.mod when appropriate. It returns (dirty, exitCode)
// where dirty means go.mod or go.sum differs from the tree before this call.
func (r *runner) processModule(ctx context.Context, modFile string) (dirty bool, code int) {
	modDir := filepath.Dir(modFile)
	origMod, origSum := readModSumBytes(modDir)
	dirtyNow := func() bool { return modSnapChanged(modDir, origMod, origSum) }

	current, err := ReadGoVersion(modFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: reading %s: %v\n", modFile, err)
		return dirtyNow(), 1
	}

	releases, err := r.fetchReleases(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: fetching releases: %v\n", err)
		return dirtyNow(), 1
	}
	latest := LatestStable(releases)

	should, reason := ShouldBumpGo(current, latest, r.cfg.Soak, time.Now())
	if should && latest != nil && r.shouldSkip("major") && isMajorBump(current, latest.Version) {
		fmt.Printf("%s: skipping cross-minor bump %s → %s (-skip=major)\n",
			modFile, current, strings.TrimPrefix(latest.Version, "go"))
		return false, 0
	}
	fmt.Printf("%s: %s\n", modFile, reason)

	atLatest := latest != nil && compareGoVersions("go"+current, latest.Version) >= 0
	wantBump := should
	if !wantBump && !atLatest {
		return false, 0
	}

	if r.cfg.DryRun {
		return false, 0
	}

	if wantBump {
		if latest == nil {
			fmt.Fprintln(os.Stderr, "gobump: no stable release to bump to")
			return dirtyNow(), 1
		}
		if err := WriteGoVersion(modFile, latest.Version); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: updating %s: %v\n", modFile, err)
			return dirtyNow(), 1
		}
		if _, err := r.goCmd(modDir, "mod", "tidy"); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: go mod tidy in %s: %v\n", modDir, err)
			return dirtyNow(), 1
		}
		if err := r.runGovulncheckGate(ctx, modFile, modDir); err != nil {
			return dirtyNow(), 1
		}
		return dirtyNow(), 0
	}

	// Toolchain already matches latest stable: still tidy + govulncheck, then optional patch retry.
	if r.shouldSkip("govulncheck") {
		return dirtyNow(), 0
	}
	if _, err := r.goCmd(modDir, "mod", "tidy"); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: go mod tidy in %s: %v\n", modDir, err)
		return dirtyNow(), 1
	}
	if err := r.runGovulncheckGate(ctx, modFile, modDir); err != nil {
		return dirtyNow(), 1
	}
	return dirtyNow(), 0
}

// revert restores go.mod and go.sum in each bumped directory via git checkout.
func (r *runner) revert(dirs []string) {
	for _, dir := range dirs {
		if _, err := r.git(dir, "checkout", "--", "go.mod", "go.sum"); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: revert in %s: %v\n", dir, err)
		}
	}
}
