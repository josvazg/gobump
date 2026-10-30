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
	TestCmd string
	Soak    time.Duration
	Skip    string
	Custom  string
	DryRun  bool
}

// MaxSoakDays is the maximum allowed -soak duration in days. Go ships a new
// minor roughly every 6 months, so a soak longer than 165 days (about 5
// months 2 weeks) would leave less than ~2-3 weeks of headroom between the
// bump and the next minor release, defeating the purpose of soaking.
const MaxSoakDays = 165

// ParseFlags parses CLI args into a Config and an optional target path.
// The path is the first non-flag argument, if present.
func ParseFlags(args []string) (Config, string, error) {
	fs := flag.NewFlagSet("gobump", flag.ContinueOnError)

	var cfg Config
	var soak dayDuration

	fs.StringVar(&cfg.TestCmd, "test", "go test ./...", "validation command")
	fs.Var(&soak, "soak", "soak duration before bumping (e.g. 90d)")
	fs.BoolVar(&cfg.DryRun, "dryrun", false, "print what would be done without making changes")
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

	// Reject soak periods so long the release would barely settle before the
	// next minor release ships.
	if max := MaxSoakDays * 24 * time.Hour; cfg.Soak > max {
		return Config{}, "", fmt.Errorf("-soak=%dd exceeds the maximum of %dd (~5 months 2 weeks): Go ships a new minor roughly every 6 months, so a longer soak would leave almost no time on the release before the next one arrives",
			int(cfg.Soak/(24*time.Hour)), MaxSoakDays)
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
