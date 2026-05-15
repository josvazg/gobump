package internal

import (
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

	releases, err := r.fetchReleases(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: fetching releases: %v\n", err)
		return 1
	}
	latest := LatestStable(releases)

	var bumpedDirs []string
	for _, modFile := range modFiles {
		bumped, code := r.processModule(modFile, latest)
		if bumped {
			bumpedDirs = append(bumpedDirs, filepath.Dir(modFile))
		}
		if code != 0 {
			r.revert(bumpedDirs)
			return code
		}
	}

	if len(bumpedDirs) == 0 {
		return 0
	}

	testDir := r.path
	if testDir == "" {
		testDir = "."
	}
	fmt.Printf("running: %s\n", r.cfg.TestCmd)
	if err := r.runShell(testDir, r.cfg.TestCmd); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: tests failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "gobump: reverting changes")
		r.revert(bumpedDirs)
		return 1
	}

	return r.finalize(bumpedDirs)
}

// processModule updates a single go.mod. Returns (bumped, exitCode).
func (r *runner) processModule(modFile string, latest *Release) (bool, int) {
	current, err := ReadGoVersion(modFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: reading %s: %v\n", modFile, err)
		return false, 1
	}

	should, reason := ShouldBumpGo(current, latest, r.cfg.Soak, time.Now())
	fmt.Printf("%s: %s\n", modFile, reason)
	if !should {
		return false, 0
	}

	if err := WriteGoVersion(modFile, latest.Version); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: updating %s: %v\n", modFile, err)
		return false, 1
	}

	modDir := filepath.Dir(modFile)

	if _, err := r.goCmd(modDir, "mod", "tidy"); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: go mod tidy in %s: %v\n", modDir, err)
		return true, 1
	}

	if !r.shouldSkip("govulncheck") {
		if err := r.govulncheck(modDir); err != nil {
			fmt.Fprintf(os.Stderr,
				"gobump: vulnerabilities found in %s: %v\n", modDir, err)
			fmt.Fprintf(os.Stderr,
				"gobump: library updates not yet automated — fix manually\n")
			return true, 1
		}
	}

	return true, 0
}

// revert restores go.mod and go.sum in each bumped directory via git checkout.
func (r *runner) revert(dirs []string) {
	for _, dir := range dirs {
		if _, err := r.git(dir, "checkout", "--", "go.mod", "go.sum"); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: revert in %s: %v\n", dir, err)
		}
	}
}
