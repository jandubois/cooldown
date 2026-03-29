package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/jandubois/cooldown/internal/version"
)

// VersionRelease holds release notes for a single version.
type VersionRelease struct {
	Version version.Version
	Tag     string // git tag name (e.g., "v4.1.2")
	URL     string // link to the release page
	Body    string // release notes markdown
}

// Changelog holds release notes for all versions between proposed and latest.
type Changelog struct {
	Owner      string
	Repo       string
	CompareURL string             // GitHub compare URL (proposed...latest)
	Releases   []VersionRelease   // sorted oldest to newest
}

// ReleaseNotesFetcher fetches release notes from GitHub for any supported ecosystem.
type ReleaseNotesFetcher struct {
	Client        *http.Client
	GitHubToken   string
	GitHubBaseURL string // override for testing
	NPMBaseURL    string // override for testing
}

// FetchChangelog retrieves release notes for all versions between proposed
// (exclusive) and latest (inclusive). Returns nil when the ecosystem or
// GitHub repo cannot be resolved.
func (f *ReleaseNotesFetcher) FetchChangelog(ctx context.Context, ecosystem, name string, proposed, latest version.Version) (*Changelog, error) {
	owner, repo, ok := f.resolveGitHubRepo(ctx, ecosystem, name)
	if !ok {
		return nil, nil
	}

	releases, err := f.fetchGitHubReleases(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("fetching releases for %s/%s: %w", owner, repo, err)
	}

	// Filter to versions in the range (proposed, latest] within the same major
	var inRange []VersionRelease
	for _, r := range releases {
		if r.Version.Major != proposed.Major {
			continue
		}
		if r.Version.IsPrerelease() {
			continue
		}
		if r.Version.Compare(proposed) <= 0 {
			continue
		}
		if r.Version.Compare(latest) > 0 {
			continue
		}
		inRange = append(inRange, r)
	}

	// Sort oldest to newest
	slices.SortFunc(inRange, func(a, b VersionRelease) int {
		return a.Version.Compare(b.Version)
	})

	// Build compare URL between proposed and latest tags
	proposedTag := bestTag(proposed, releases)
	latestTag := bestTag(latest, releases)
	compareURL := fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s",
		owner, repo, proposedTag, latestTag)

	return &Changelog{
		Owner:      owner,
		Repo:       repo,
		CompareURL: compareURL,
		Releases:   inRange,
	}, nil
}

// bestTag finds the actual git tag for a version from the releases list.
// Falls back to common tag formats if not found.
func bestTag(v version.Version, releases []VersionRelease) string {
	for _, r := range releases {
		if r.Version.Compare(v) == 0 {
			return r.Tag
		}
	}
	// Fallback: use the raw version string (which often is the tag)
	return v.Raw
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

	if err := json.Unmarshal(raw, &repoURL); err != nil {
		var obj struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return "", "", false
		}
		repoURL = obj.URL
	}

	if !strings.Contains(repoURL, "/") {
		return "", "", false
	}
	if !strings.Contains(repoURL, "github") && !strings.Contains(repoURL, "://") {
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

type ghReleaseNotes struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// fetchGitHubReleases fetches all releases for a GitHub repo.
func (f *ReleaseNotesFetcher) fetchGitHubReleases(ctx context.Context, owner, repo string) ([]VersionRelease, error) {
	baseURL := f.githubBaseURL()
	var all []VersionRelease

	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100&page=%d",
			baseURL, owner, repo, page)

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

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d for releases %s/%s", resp.StatusCode, owner, repo)
		}
		if err != nil {
			return nil, err
		}

		var releases []ghReleaseNotes
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, err
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
				continue
			}
			if r.Prerelease && !v.IsPrerelease() {
				v.Prerelease = "prerelease"
			}
			all = append(all, VersionRelease{
				Version: v,
				Tag:     r.TagName,
				URL:     r.HTMLURL,
				Body:    r.Body,
			})
		}

		page++
	}

	return all, nil
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
