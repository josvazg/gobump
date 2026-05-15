package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// go.dev/dl has version stability info but no release dates.
// We get the date for the latest stable version from the GitHub commits API.
const (
	goDownloadsAPI    = "https://go.dev/dl/?mode=json&include=all"
	goCommitsBaseURL  = "https://api.github.com/repos/golang/go/commits"
)

// Release is a Go toolchain release.
type Release struct {
	Version string    // e.g. "go1.22.3"
	Date    time.Time // zero if unknown
	Stable  bool
}

type dlRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type ghCommit struct {
	Commit struct {
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

// FetchReleases fetches Go releases from go.dev/dl, then enriches the latest
// stable release with its date from the GitHub commits API.
// Pass empty strings for dlURL / commitBaseURL to use the defaults.
func FetchReleases(ctx context.Context, client *http.Client, dlURL, commitBaseURL string) ([]Release, error) {
	if dlURL == "" {
		dlURL = goDownloadsAPI
	}
	if commitBaseURL == "" {
		commitBaseURL = goCommitsBaseURL
	}
	if client == nil {
		client = http.DefaultClient
	}

	releases, err := fetchDLVersions(ctx, client, dlURL)
	if err != nil {
		return nil, err
	}

	// Soak time is measured from when the minor version first shipped (x.y.0),
	// not the latest patch — patches are just fixes within an already-soaked line.
	latest := LatestStable(releases)
	if latest != nil {
		origin := minorOrigin(latest.Version)
		date, err := fetchCommitDate(ctx, client, commitBaseURL+"/"+origin)
		if err != nil {
			return nil, fmt.Errorf("fetching date for %s: %w", origin, err)
		}
		latest.Date = date
	}

	return releases, nil
}

func fetchDLVersions(ctx context.Context, client *http.Client, url string) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go.dev/dl returned HTTP %d", resp.StatusCode)
	}

	var raw []dlRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding go.dev/dl response: %w", err)
	}

	releases := make([]Release, 0, len(raw))
	for _, r := range raw {
		releases = append(releases, Release{
			Version: r.Version,
			Stable:  r.Stable,
		})
	}
	return releases, nil
}

func fetchCommitDate(ctx context.Context, client *http.Client, url string) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("github commits API returned HTTP %d", resp.StatusCode)
	}

	var info ghCommit
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return time.Time{}, fmt.Errorf("decoding commit info: %w", err)
	}
	return info.Commit.Committer.Date, nil
}

// minorOrigin returns the x.y.0 tag for a given Go version, e.g.
// "go1.26.3" → "go1.26.0". Soak time is measured from this tag.
func minorOrigin(version string) string {
	p := splitGoVersion(version)
	return fmt.Sprintf("go%d.%d.0", p[0], p[1])
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
