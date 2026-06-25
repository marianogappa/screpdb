package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	repoOwner = "marianogappa"
	repoName  = "screpdb"

	checksumsAsset  = "SHA256SUMS"
	signatureAsset  = "SHA256SUMS.minisig"
	maxDownloadSize = 200 << 20 // 200 MiB ceiling guards against a runaway response.
)

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Assets  []ghAsset `json:"assets"`
}

func (r ghRelease) asset(name string) (ghAsset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return ghAsset{}, false
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// fetchLatestRelease queries the GitHub Releases API for the newest published
// release. This is the only outbound call made during a launch-time update check
// and is read-only.
func fetchLatestRelease(ctx context.Context) (ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ghRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("github releases API returned %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return ghRelease{}, fmt.Errorf("decode github release: %w", err)
	}
	if rel.TagName == "" {
		return ghRelease{}, fmt.Errorf("github release has no tag name")
	}
	return rel, nil
}

// downloadAsset fetches an asset's bytes. GitHub redirects asset downloads to a
// CDN host; the redirect chain is trusted because integrity is guaranteed by the
// minisign-signed SHA256SUMS, not by the transport.
func downloadAsset(ctx context.Context, a ghAsset) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s returned %s", a.Name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", a.Name, err)
	}
	return data, nil
}
