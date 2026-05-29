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
	return newRunner(cfg, path, env).run(ctx)
}

type runner struct {
	cfg           Config
	path          string
	skipSteps     map[string]bool
	fetchReleases func(ctx context.Context) ([]Release, error)
	git           func(dir string, args ...string) (string, error)
	goCmd         func(dir string, args ...string) (string, error)
	runShell      func(dir, cmd string) error
	govulncheck   func(dir string) (VulnReport, error)
}

func parseSkip(s string) map[string]bool {
	m := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			m[p] = true
		}
	}
	return m
}

func (r *runner) rootDir() string {
	dir := strings.TrimSuffix(r.path, "/...")
	if dir == "" {
		return "."
	}
	return dir
}

func newRunner(cfg Config, path string, env func(string) string) *runner {
	return &runner{
		cfg:       cfg,
		path:      path,
		skipSteps: parseSkip(cfg.Skip),
		fetchReleases: func(ctx context.Context) ([]Release, error) {
			return FetchReleases(ctx, nil, env("GOBUMP_DL_URL"), env("GOBUMP_COMMIT_URL"))
		},
		git:      defaultGit,
		goCmd:    defaultGoCmd,
		runShell: defaultRunShell,
		govulncheck: defaultGovulncheck,
	}
}

func (r *runner) shouldSkip(step string) bool {
	return r.skipSteps["all"] || r.skipSteps[step]
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

	if r.cfg.Custom != "" && !r.shouldSkip("custom") {
		fmt.Printf("running: %s\n", r.cfg.Custom)
		if err := r.runShell(r.rootDir(), r.cfg.Custom); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: custom command failed: %v\n", err)
			r.revert(bumpedDirs)
			return 1
		}
	}

	fmt.Printf("running: %s\n", r.cfg.TestCmd)
	if err := r.runShell(r.rootDir(), r.cfg.TestCmd); err != nil {
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

// runGovulncheckGate runs govulncheck and attempts automated fixes:
//   - library findings: go get module@fixedVersion + go mod tidy
//   - stdlib findings: bump go directive to latest patch + go mod tidy
//
// If any fix was applied, govulncheck is re-run to confirm clean.
func (r *runner) runGovulncheckGate(ctx context.Context, modFile, modDir string) error {
	if r.shouldSkip("govulncheck") {
		return nil
	}
	report, firstErr := r.govulncheck(modDir)
	if firstErr == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "gobump: vulnerabilities found in %s: %v\n", modDir, firstErr)

	anyFixed := false

	// Fix library findings: go get module@fixedVersion.
	if libs := report.LibFindings(); len(libs) > 0 {
		for mod, ver := range libs {
			fmt.Fprintf(os.Stderr, "gobump: bumping library %s to %s\n", mod, ver)
			if _, err := r.goCmd(modDir, "get", mod+"@"+ver); err != nil {
				return fmt.Errorf("go get %s@%s: %w", mod, ver, err)
			}
		}
		if _, err := r.goCmd(modDir, "mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy in %s: %w", modDir, err)
		}
		anyFixed = true
	}

	// Fix stdlib findings: bump go directive if a newer patch is available.
	if report.HasStdlib() {
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
		if compareGoVersions(L2.Version, "go"+cur) > 0 {
			fmt.Fprintf(os.Stderr, "gobump: retrying with newer Go patch %s\n", strings.TrimPrefix(L2.Version, "go"))
			if err := WriteGoVersion(modFile, L2.Version); err != nil {
				return err
			}
			if _, err := r.goCmd(modDir, "mod", "tidy"); err != nil {
				return fmt.Errorf("go mod tidy in %s: %w", modDir, err)
			}
			anyFixed = true
		}
	}

	if !anyFixed {
		fmt.Fprintln(os.Stderr, "gobump: govulncheck still failing; no automated fix available (fix manually)")
		return fmt.Errorf("govulncheck")
	}

	if _, err := r.govulncheck(modDir); err != nil {
		fmt.Fprintln(os.Stderr, "gobump: govulncheck failed after automated fix (fix manually)")
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

	needsPatch, reason := ShouldBumpGo(current, latest, r.cfg.Soak, time.Now())
	if needsPatch && latest != nil && r.shouldSkip("major") && isMajorBump(current, latest.Version) {
		fmt.Printf("%s: skipping cross-minor bump %s → %s (-skip=major)\n",
			modFile, current, strings.TrimPrefix(latest.Version, "go"))
		return false, 0
	}
	fmt.Printf("%s: %s\n", modFile, reason)

	atLatest := latest != nil && compareGoVersions("go"+current, latest.Version) >= 0
	soaking := !needsPatch && !atLatest

	if soaking || r.cfg.DryRun {
		return false, 0
	}

	switch {
	case needsPatch:
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
	default: // atLatest: health-check tidy + govulncheck without a version change
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
