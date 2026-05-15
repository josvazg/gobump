package internal

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds the resolved CLI configuration.
type Config struct {
	Push      bool
	PR        string
	TestCmd   string
	Soak      time.Duration
	Protected string
	Force     bool
	Skip      string
	Custom    string
}

// ParseFlags parses CLI args into a Config and an optional target path.
// The path is the first non-flag argument, if present.
func ParseFlags(args []string) (Config, string, error) {
	fs := flag.NewFlagSet("gobump", flag.ContinueOnError)

	var cfg Config
	var soak dayDuration

	fs.BoolVar(&cfg.Push, "push", false, "commit and push changes")
	fs.StringVar(&cfg.PR, "pr", "", "shell command to run after push")
	fs.StringVar(&cfg.TestCmd, "test", "go test ./...", "validation command")
	fs.Var(&soak, "soak", "soak duration before bumping (e.g. 90d)")
	fs.StringVar(&cfg.Protected, "protected", "main,master,trunk", "protected branches")
	fs.BoolVar(&cfg.Force, "force", false, "override branch protection and dirty-tree checks")
	fs.StringVar(&cfg.Skip, "skip", "", "skip steps: all|major|govulncheck|custom")
	fs.StringVar(&cfg.Custom, "custom", "", "extra command to run after bumping, before testing")

	if err := fs.Parse(args); err != nil {
		return Config{}, "", err
	}

	// Default soak to 90 days if not set.
	if soak == 0 {
		cfg.Soak = 90 * 24 * time.Hour
	} else {
		cfg.Soak = time.Duration(soak)
	}

	var path string
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	return cfg, path, nil
}

// dayDuration is a flag.Value that parses "Nd" strings into time.Duration.
type dayDuration time.Duration

func (d *dayDuration) Set(s string) error {
	s = strings.TrimSuffix(s, "d")
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid day duration %q: must be a positive integer followed by 'd'", s+"d")
	}
	*d = dayDuration(time.Duration(n) * 24 * time.Hour)
	return nil
}

func (d dayDuration) String() string {
	days := time.Duration(d) / (24 * time.Hour)
	return fmt.Sprintf("%dd", days)
}
