package internal_test

import (
	"testing"
	"time"

	"github.com/josvazg/gobump/internal"
)

const day = 24 * time.Hour

func TestShouldBumpGo(t *testing.T) {
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	soak := 90 * day

	old := internal.Release{Version: "go1.22.3", Date: now.Add(-100 * day), Stable: true}
	fresh := internal.Release{Version: "go1.22.3", Date: now.Add(-30 * day), Stable: true}

	tests := []struct {
		name        string
		current     string
		latest      *internal.Release
		wantBump    bool
	}{
		{"bump when latest is older than soak and current is behind", "1.21.0", &old, true},
		{"no bump when latest is still soaking", "1.21.0", &fresh, false},
		{"no bump when already at latest", "1.22.3", &old, false},
		{"no bump when ahead of latest", "1.22.4", &old, false},
		{"no bump when no releases", "1.21.0", nil, false},
		{"go.mod version without patch is treated as x.y.0", "1.22", &old, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := internal.ShouldBumpGo(tc.current, tc.latest, soak, now)
			if got != tc.wantBump {
				t.Errorf("ShouldBumpGo(%q) = %v (%s), want %v", tc.current, got, reason, tc.wantBump)
			}
		})
	}
}
