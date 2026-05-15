package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// go.dev/dl/?mode=json lacks release dates, so we use the GitHub releases API.
const goReleasesAPI = "https://api.github.com/repos/golang/go/releases?per_page=100"

// Release is a Go toolchain release.
type Release struct {
	Version string    // e.g. "go1.22.3"
	Date    time.Time
	Stable  bool
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}

// FetchReleases fetches Go releases from the GitHub releases API.
// Pass apiURL="" to use the default endpoint; a non-empty value overrides it (useful in tests).
// Pass client=nil to use http.DefaultClient.
func FetchReleases(ctx context.Context, client *http.Client, apiURL string) ([]Release, error) {
	if apiURL == "" {
		apiURL = goReleasesAPI
	}
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("releases API returned HTTP %d", resp.StatusCode)
	}

	var raw []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}

	releases := make([]Release, 0, len(raw))
	for _, r := range raw {
		if r.Draft || !strings.HasPrefix(r.TagName, "go") {
			continue
		}
		releases = append(releases, Release{
			Version: r.TagName,
			Date:    r.PublishedAt,
			Stable:  !r.Prerelease,
		})
	}
	return releases, nil
}

// LatestStable returns the highest-versioned stable release, or nil if none.
func LatestStable(releases []Release) *Release {
	var latest *Release
	for i := range releases {
		r := &releases[i]
		if !r.Stable {
			continue
		}
		if latest == nil || compareGoVersions(r.Version, latest.Version) > 0 {
			latest = r
		}
	}
	return latest
}
