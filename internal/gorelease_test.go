package internal_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josvazg/gobump/internal"
)

func TestFetchReleases(t *testing.T) {
	published := time.Date(2024, 5, 1, 17, 0, 0, 0, time.UTC)
	fixture := []map[string]any{
		{"tag_name": "go1.22.3", "published_at": published.Format(time.RFC3339), "prerelease": false, "draft": false},
		{"tag_name": "go1.22.2", "published_at": published.Add(-30 * 24 * time.Hour).Format(time.RFC3339), "prerelease": false, "draft": false},
		{"tag_name": "go1.23rc1", "published_at": published.Add(24 * time.Hour).Format(time.RFC3339), "prerelease": true, "draft": false},
		{"tag_name": "go1.22.3", "published_at": published.Format(time.RFC3339), "prerelease": false, "draft": true}, // draft, must be excluded
		{"tag_name": "notago", "published_at": published.Format(time.RFC3339), "prerelease": false, "draft": false},  // non-go tag, excluded
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(fixture); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	releases, err := internal.FetchReleases(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatalf("FetchReleases: %v", err)
	}

	// 3 entries match go* prefix, minus 1 draft, minus 1 prerelease = 2 stable
	stable := 0
	for _, r := range releases {
		if r.Stable {
			stable++
		}
	}
	if stable != 2 {
		t.Errorf("expected 2 stable releases, got %d", stable)
	}
	if len(releases) != 3 { // 2 stable + 1 prerelease (rc1)
		t.Errorf("expected 3 total releases (excl draft/non-go), got %d", len(releases))
	}
}

func TestLatestStable(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	releases := []internal.Release{
		{Version: "go1.22.2", Date: base, Stable: true},
		{Version: "go1.22.3", Date: base.Add(30 * 24 * time.Hour), Stable: true},
		{Version: "go1.23rc1", Date: base.Add(60 * 24 * time.Hour), Stable: false},
		{Version: "go1.21.12", Date: base.Add(-30 * 24 * time.Hour), Stable: true},
	}

	got := internal.LatestStable(releases)
	if got == nil {
		t.Fatal("expected a latest stable release, got nil")
	}
	if got.Version != "go1.22.3" {
		t.Errorf("expected go1.22.3, got %s", got.Version)
	}
}

func TestLatestStable_empty(t *testing.T) {
	if got := internal.LatestStable(nil); got != nil {
		t.Errorf("expected nil for empty list, got %+v", got)
	}
}

func TestLatestStable_noStable(t *testing.T) {
	releases := []internal.Release{
		{Version: "go1.23rc1", Date: time.Now(), Stable: false},
	}
	if got := internal.LatestStable(releases); got != nil {
		t.Errorf("expected nil when no stable releases, got %+v", got)
	}
}
