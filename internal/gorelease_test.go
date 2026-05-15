package internal_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/josvazg/gobump/internal"
)

// fakeDLServer serves a go.dev/dl-style version list on / and a GitHub
// commit-style date response on /commits/<version>.
func fakeDLServer(t *testing.T, versions []map[string]any, commitDate time.Time) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/commits/") {
			payload := map[string]any{
				"commit": map[string]any{
					"committer": map[string]any{
						"date": commitDate.Format(time.RFC3339),
					},
				},
			}
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(versions); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
}

func TestFetchReleases(t *testing.T) {
	published := time.Date(2024, 5, 1, 17, 0, 0, 0, time.UTC)
	versions := []map[string]any{
		{"version": "go1.22.3", "stable": true},
		{"version": "go1.22.2", "stable": true},
		{"version": "go1.23rc1", "stable": false},
	}

	srv := fakeDLServer(t, versions, published)
	defer srv.Close()

	releases, err := internal.FetchReleases(context.Background(), nil, srv.URL, srv.URL+"/commits")
	if err != nil {
		t.Fatalf("FetchReleases: %v", err)
	}

	stable := 0
	for _, r := range releases {
		if r.Stable {
			stable++
		}
	}
	if stable != 2 {
		t.Errorf("expected 2 stable releases, got %d", stable)
	}
	if len(releases) != 3 {
		t.Errorf("expected 3 total releases, got %d", len(releases))
	}

	// Latest stable (go1.22.3) should have its date populated.
	latest := internal.LatestStable(releases)
	if latest == nil {
		t.Fatal("expected a latest stable release")
	}
	if !latest.Date.Equal(published) {
		t.Errorf("latest.Date = %v, want %v", latest.Date, published)
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
