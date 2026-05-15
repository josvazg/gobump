package internal

import (
	"fmt"
	"os"
)

// finalize commits and pushes changes after a successful test gate, then
// optionally runs the -pr command. It is a no-op when -push is not set.
func (r *runner) finalize(bumpedDirs []string) int {
	if !r.cfg.Push {
		return 0
	}

	rootDir := r.path
	if rootDir == "" {
		rootDir = "."
	}

	for _, dir := range bumpedDirs {
		if _, err := r.git(dir, "add", "go.mod", "go.sum"); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: git add in %s: %v\n", dir, err)
			return 1
		}
	}

	if _, err := r.git(rootDir, "commit", "-m", "chore: gobump updates"); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: git commit: %v\n", err)
		return 1
	}

	if _, err := r.git(rootDir, "push"); err != nil {
		fmt.Fprintf(os.Stderr, "gobump: git push: %v\n", err)
		return 1
	}

	if r.cfg.PR != "" {
		fmt.Printf("running: %s\n", r.cfg.PR)
		if err := r.runShell(rootDir, r.cfg.PR); err != nil {
			fmt.Fprintf(os.Stderr, "gobump: pr command: %v\n", err)
			return 1
		}
	}

	return 0
}
