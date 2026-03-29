package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/jandubois/cooldown/internal/version"
)

// ReleaseInfo holds release notes and URL for a specific version.
type ReleaseInfo struct {
	URL  string // link to the release page (or releases index as fallback)
	Body string // release notes markdown (may be empty)
}

// ReleaseNotesFetcher fetches release notes from GitHub for any supported ecosystem.
type ReleaseNotesFetcher struct {
	Client        *http.Client
	GitHubToken   string
	GitHubBaseURL string // override for testing
	NPMBaseURL    string // override for testing
}

// Fetch retrieves release notes for a dependency version.
// Returns nil (not an error) when the ecosystem or repo cannot be resolved.
func (f *ReleaseNotesFetcher) Fetch(ctx context.Context, ecosystem, name string, v version.Version) (*ReleaseInfo, error) {
	owner, repo, ok := f.resolveGitHubRepo(ctx, ecosystem, name)
	if !ok {
		return nil, nil
	}

	// Try candidate tags in order: raw version, with v prefix, without v prefix
	for _, tag := range candidateTags(v) {
		info, err := f.fetchGitHubRelease(ctx, owner, repo, tag)
		if err == nil {
			return info, nil
		}
	}

	// No matching release found; return a link to the releases page
	return &ReleaseInfo{
		URL: fmt.Sprintf("https://github.com/%s/%s/releases", owner, repo),
	}, nil
}

func (f *ReleaseNotesFetcher) resolveGitHubRepo(ctx context.Context, ecosystem, name string) (owner, repo string, ok bool) {
	switch ecosystem {
	case "github_actions":
		owner, repo, ok = strings.Cut(name, "/")
		return
	case "gomod":
		return githubRepoFromGoModule(name)
	case "npm_and_yarn":
		return f.githubRepoFromNPM(ctx, name)
	default:
		return "", "", false
	}
}

// githubRepoFromGoModule extracts owner/repo from a github.com module path.
// Returns false for non-GitHub modules.
func githubRepoFromGoModule(path string) (owner, repo string, ok bool) {
	if !strings.HasPrefix(path, "github.com/") {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "github.com/"), "/", 3)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// githubRepoFromNPM fetches the package metadata from npm to find the
// repository URL. Returns false if the repo is not on GitHub.
func (f *ReleaseNotesFetcher) githubRepoFromNPM(ctx context.Context, name string) (owner, repo string, ok bool) {
	baseURL := f.npmBaseURL()
	// Fetch the latest version manifest (small response) to get the repository field
	url := fmt.Sprintf("%s/%s/latest", baseURL, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", false
	}

	resp, err := f.Client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return "", "", false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", false
	}

	var doc struct {
		Repository json.RawMessage `json:"repository"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", false
	}

	return parseGitHubRepoURL(doc.Repository)
}

var githubRepoPattern = regexp.MustCompile(`github\.com[/:]([^/]+)/([^/.]+?)(?:\.git)?(?:/|$)`)

// parseGitHubRepoURL extracts owner/repo from an npm repository field.
// The field can be a string (shorthand) or an object with a "url" property.
func parseGitHubRepoURL(raw json.RawMessage) (owner, repo string, ok bool) {
	if len(raw) == 0 {
		return "", "", false
	}

	var repoURL string

	// Try as a string first (shorthand like "github:user/repo" or just "user/repo")
	if err := json.Unmarshal(raw, &repoURL); err != nil {
		// Try as an object with a "url" field
		var obj struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return "", "", false
		}
		repoURL = obj.URL
	}

	// Handle GitHub shorthand (e.g., "expressjs/express")
	if !strings.Contains(repoURL, "/") {
		return "", "", false
	}
	if !strings.Contains(repoURL, "github") && !strings.Contains(repoURL, "://") {
		// Might be "owner/repo" shorthand
		parts := strings.SplitN(repoURL, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
	}

	m := githubRepoPattern.FindStringSubmatch(repoURL)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// candidateTags returns possible git tags for a version, ordered by likelihood.
func candidateTags(v version.Version) []string {
	raw := v.Raw
	if strings.HasPrefix(raw, "v") {
		// Already has v prefix; try as-is, then without
		return []string{raw, strings.TrimPrefix(raw, "v")}
	}
	// No v prefix; try with v, then without
	return []string{"v" + raw, raw}
}

func (f *ReleaseNotesFetcher) fetchGitHubRelease(ctx context.Context, owner, repo, tag string) (*ReleaseInfo, error) {
	baseURL := f.githubBaseURL()
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", baseURL, owner, repo, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if f.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+f.GitHubToken)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for release %s/%s@%s", resp.StatusCode, owner, repo, tag)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release struct {
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	return &ReleaseInfo{
		URL:  release.HTMLURL,
		Body: release.Body,
	}, nil
}

func (f *ReleaseNotesFetcher) githubBaseURL() string {
	if f.GitHubBaseURL != "" {
		return f.GitHubBaseURL
	}
	return "https://api.github.com"
}

func (f *ReleaseNotesFetcher) npmBaseURL() string {
	if f.NPMBaseURL != "" {
		return f.NPMBaseURL
	}
	return "https://registry.npmjs.org"
}
