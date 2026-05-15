package internal

import (
	"context"
	"fmt"
	"os"
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
}

func newRunner(cfg Config, path string) *runner {
	return &runner{
		cfg:  cfg,
		path: path,
		fetchReleases: func(ctx context.Context) ([]Release, error) {
			return FetchReleases(ctx, nil, "")
		},
	}
}

func (r *runner) run(ctx context.Context) int {
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

	for _, modFile := range modFiles {
		if code := r.processModule(modFile, latest); code != 0 {
			return code
		}
	}
	return 0
}

func (r *runner) processModule(modFile string, latest *Release) int {
	current, err := ReadGoVersion(modFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gobump: reading %s: %v\n", modFile, err)
		return 1
	}

	should, reason := ShouldBumpGo(current, latest, r.cfg.Soak, time.Now())
	fmt.Printf("%s: %s\n", modFile, reason)
	if !should {
		return 0
	}

	if err := WriteGoVersion(modFile, latest.Version); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: updating %s: %v\n", modFile, err)
		return 1
	}
	return 0
}
