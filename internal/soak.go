package internal

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ShouldBumpGo reports whether the go directive in go.mod should be updated.
// goModVersion is the bare version from go.mod (e.g. "1.22.3" or "1.22").
// now is injectable so tests don't depend on the wall clock.
func ShouldBumpGo(goModVersion string, latest *Release, soakDur time.Duration, now time.Time) (bool, string) {
	if latest == nil {
		return false, "no releases available"
	}

	// Normalize go.mod version to "go1.x.y" for comparison.
	current := "go" + goModVersion
	if compareGoVersions(current, latest.Version) >= 0 {
		return false, fmt.Sprintf("already at %s", current)
	}

	age := now.Sub(latest.Date)
	if age < soakDur {
		remaining := soakDur - age
		return false, fmt.Sprintf("%s released %.0f days ago, %.0f days remaining in soak period",
			latest.Version, age.Hours()/24, remaining.Hours()/24)
	}

	return true, fmt.Sprintf("bump go %s → %s (released %.0f days ago)", goModVersion, latest.Version, age.Hours()/24)
}

// compareGoVersions compares two Go version strings, which may be in either
// "go1.22.3" or "1.22.3" or "1.22" format. Returns -1, 0, or 1.
func compareGoVersions(a, b string) int {
	return compareVersionParts(splitGoVersion(a), splitGoVersion(b))
}

func splitGoVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "go")
	parts := strings.SplitN(v, ".", 3)
	var r [3]int
	for i, p := range parts {
		r[i], _ = strconv.Atoi(p)
	}
	return r
}

func compareVersionParts(a, b [3]int) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
