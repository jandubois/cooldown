package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jandubois/cooldown/internal/version"
)

// GitHub fetches versions from GitHub releases and tags for GitHub Actions.
type GitHub struct {
	Client  *http.Client
	Token   string // optional; improves rate limits
	BaseURL string // override for testing; defaults to https://api.github.com
}

func (g *GitHub) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return "https://api.github.com"
}

// Versions returns all release versions for a GitHub Action (owner/repo).
// Falls back to tags if no releases exist.
func (g *GitHub) Versions(ctx context.Context, name string) ([]version.Version, error) {
	owner, repo, ok := strings.Cut(name, "/")
	if !ok {
		return nil, fmt.Errorf("github action name %q must be owner/repo", name)
	}

	versions, err := g.releaseTags(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	if len(versions) > 0 {
		return versions, nil
	}

	// Fallback to tags API
	return g.tags(ctx, owner, repo)
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

func (g *GitHub) releaseTags(ctx context.Context, owner, repo string) ([]version.Version, error) {
	var allVersions []version.Version

	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100&page=%d",
			g.baseURL(), owner, repo, page)

		body, status, err := g.doGet(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("fetching releases for %s/%s: %w", owner, repo, err)
		}
		if status == http.StatusNotFound {
			return nil, &ErrNotFound{Name: owner + "/" + repo}
		}

		var releases []ghRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, fmt.Errorf("parsing releases for %s/%s: %w", owner, repo, err)
		}

		if len(releases) == 0 {
			break
		}

		for _, r := range releases {
			if r.Draft {
				continue
			}
			v, err := version.Parse(r.TagName)
			if err != nil {
				continue // skip non-semver tags
			}
			if r.Prerelease {
				// Mark as prerelease even if the tag itself doesn't have a pre suffix
				if !v.IsPrerelease() {
					v.Prerelease = "prerelease"
				}
			}
			allVersions = append(allVersions, v)
		}

		page++
	}

	return allVersions, nil
}

type ghTag struct {
	Name string `json:"name"`
}

func (g *GitHub) tags(ctx context.Context, owner, repo string) ([]version.Version, error) {
	var allVersions []version.Version

	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100&page=%d",
			g.baseURL(), owner, repo, page)

		body, status, err := g.doGet(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("fetching tags for %s/%s: %w", owner, repo, err)
		}
		if status == http.StatusNotFound {
			return nil, &ErrNotFound{Name: owner + "/" + repo}
		}

		var tags []ghTag
		if err := json.Unmarshal(body, &tags); err != nil {
			return nil, fmt.Errorf("parsing tags for %s/%s: %w", owner, repo, err)
		}

		if len(tags) == 0 {
			break
		}

		for _, tag := range tags {
			v, err := version.Parse(tag.Name)
			if err != nil {
				continue // skip non-semver tags (e.g., "v4" floating tags)
			}
			allVersions = append(allVersions, v)
		}

		page++
	}

	return allVersions, nil
}

func (g *GitHub) doGet(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, resp.StatusCode, nil
}
