package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	GitHubOwner = "elkoNDe"
	GitHubRepo  = "AudioDeejSwitch"
)

// CheckUpdateFunc is the function signature for checking updates.
type CheckUpdateFunc func(context.Context) (bool, string, string, error)

// Version is the current application version. Set at build time via -ldflags.
var Version = "dev"

// ReleaseInfo represents a GitHub release.
type ReleaseInfo struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	Prerelease  bool    `json:"prerelease"`
	Draft       bool    `json:"draft"`
	Assets      []Asset `json:"assets"`
	PublishedAt string  `json:"published_at"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for a newer version on GitHub.
// Returns (hasUpdate, latestVersion, releaseURL, error).
func CheckUpdate(ctx context.Context) (bool, string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "AudioDeejSwitch-Updater")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, "", "", fmt.Errorf("GitHub API error: %d %s", resp.StatusCode, string(body))
	}

	var rel ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return false, "", "", err
	}

	if rel.Draft || rel.Prerelease {
		return false, "", "", nil
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	cmp, err := compareVersions(current, latest)
	if err != nil {
		return false, "", "", err
	}

	if cmp < 0 {
		return true, latest, releaseDownloadURL(rel.Assets), nil
	}
	return false, latest, "", nil
}

// releaseDownloadURL prefers the Windows executable asset and falls back to the
// first asset. Returns an empty string when the release has no assets.
func releaseDownloadURL(assets []Asset) string {
	for _, a := range assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			return a.BrowserDownloadURL
		}
	}
	if len(assets) > 0 {
		return assets[0].BrowserDownloadURL
	}
	return ""
}

// compareVersions compares two semantic version strings (major.minor.patch).
// Returns -1 if v1 < v2, 0 if equal, 1 if v1 > v2.
func compareVersions(v1, v2 string) (int, error) {
	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")
	for i := 0; i < 3; i++ {
		var n1, n2 int
		if i < len(p1) {
			n1, _ = strconv.Atoi(p1[i])
		}
		if i < len(p2) {
			n2, _ = strconv.Atoi(p2[i])
		}
		if n1 < n2 {
			return -1, nil
		}
		if n1 > n2 {
			return 1, nil
		}
	}
	return 0, nil
}

// DownloadUpdate downloads the update asset to the given writer.
// Returns the number of bytes written and error.
func DownloadUpdate(ctx context.Context, url string, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "AudioDeejSwitch-Updater")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	return io.Copy(w, resp.Body)
}
