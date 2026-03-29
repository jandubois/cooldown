package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jandubois/cooldown/internal/version"
)

func mustParse(t *testing.T, s string) version.Version {
	t.Helper()
	v, err := version.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return v
}

func TestFetchChangelogGitHubActions(t *testing.T) {
	releases := []ghReleaseNotes{
		{TagName: "v4.3.0", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4.3.0", Body: "### v4.3.0\n* New feature"},
		{TagName: "v4.2.0", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4.2.0", Body: "### v4.2.0\n* Bug fix"},
		{TagName: "v4.1.0", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4.1.0", Body: "### v4.1.0\n* Old"},
		{TagName: "v3.6.0", HTMLURL: "https://github.com/actions/checkout/releases/tag/v3.6.0", Body: "v3"},
		{TagName: "v5.0.0-beta", HTMLURL: "url", Body: "beta", Prerelease: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			json.NewEncoder(w).Encode([]ghReleaseNotes{}) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(releases) //nolint:errcheck
	}))
	defer srv.Close()

	f := &ReleaseNotesFetcher{Client: srv.Client(), GitHubBaseURL: srv.URL}

	cl, err := f.FetchChangelog(context.Background(), "github_actions", "actions/checkout",
		mustParse(t, "v4.1.0"), mustParse(t, "v4.3.0"))
	if err != nil {
		t.Fatalf("FetchChangelog() error: %v", err)
	}
	if cl == nil {
		t.Fatal("FetchChangelog() returned nil")
	}

	// Should include v4.2.0 and v4.3.0 (not v4.1.0, v3.6.0, or v5.0.0-beta)
	if len(cl.Releases) != 2 {
		t.Fatalf("got %d releases, want 2: %v", len(cl.Releases), cl.Releases)
	}
	if cl.Releases[0].Tag != "v4.2.0" {
		t.Errorf("releases[0].Tag = %q, want v4.2.0", cl.Releases[0].Tag)
	}
	if cl.Releases[1].Tag != "v4.3.0" {
		t.Errorf("releases[1].Tag = %q, want v4.3.0", cl.Releases[1].Tag)
	}

	if cl.CompareURL != "https://github.com/actions/checkout/compare/v4.1.0...v4.3.0" {
		t.Errorf("CompareURL = %q", cl.CompareURL)
	}
}

func TestFetchChangelogGoModule(t *testing.T) {
	releases := []ghReleaseNotes{
		{TagName: "v1.10.0", HTMLURL: "url1", Body: "Release v1.10.0"},
		{TagName: "v1.9.0", HTMLURL: "url2", Body: "Release v1.9.0"},
		{TagName: "v1.8.0", HTMLURL: "url3", Body: "Release v1.8.0"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			json.NewEncoder(w).Encode([]ghReleaseNotes{}) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(releases) //nolint:errcheck
	}))
	defer srv.Close()

	f := &ReleaseNotesFetcher{Client: srv.Client(), GitHubBaseURL: srv.URL}

	cl, err := f.FetchChangelog(context.Background(), "gomod", "github.com/stretchr/testify",
		mustParse(t, "v1.8.0"), mustParse(t, "v1.10.0"))
	if err != nil {
		t.Fatalf("FetchChangelog() error: %v", err)
	}

	// Should include v1.9.0 and v1.10.0
	if len(cl.Releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(cl.Releases))
	}
	if cl.Releases[0].Tag != "v1.9.0" {
		t.Errorf("releases[0].Tag = %q, want v1.9.0", cl.Releases[0].Tag)
	}
}

func TestFetchChangelogGoModuleNonGitHub(t *testing.T) {
	f := &ReleaseNotesFetcher{Client: http.DefaultClient}

	cl, err := f.FetchChangelog(context.Background(), "gomod", "golang.org/x/net",
		mustParse(t, "v0.16.0"), mustParse(t, "v0.17.0"))
	if err != nil {
		t.Fatalf("FetchChangelog() error: %v", err)
	}
	if cl != nil {
		t.Errorf("expected nil for non-GitHub module, got %+v", cl)
	}
}

func TestFetchChangelogNPM(t *testing.T) {
	releases := []ghReleaseNotes{
		{TagName: "v4.22.1", HTMLURL: "url1", Body: "### 4.22.1\n* Security fix"},
		{TagName: "v4.22.0", HTMLURL: "url2", Body: "### 4.22.0\n* New features"},
		{TagName: "v4.21.0", HTMLURL: "url3", Body: "### 4.21.0\n* Old"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/express/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"repository": map[string]string{
				"type": "git",
				"url":  "git+https://github.com/expressjs/express.git",
			},
		}) //nolint:errcheck
	})
	mux.HandleFunc("/repos/expressjs/express/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			json.NewEncoder(w).Encode([]ghReleaseNotes{}) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(releases) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := &ReleaseNotesFetcher{
		Client:        srv.Client(),
		GitHubBaseURL: srv.URL,
		NPMBaseURL:    srv.URL,
	}

	cl, err := f.FetchChangelog(context.Background(), "npm_and_yarn", "express",
		mustParse(t, "4.21.0"), mustParse(t, "4.22.1"))
	if err != nil {
		t.Fatalf("FetchChangelog() error: %v", err)
	}
	if cl == nil {
		t.Fatal("FetchChangelog() returned nil")
	}

	// Should include v4.22.0 and v4.22.1 (not v4.21.0)
	if len(cl.Releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(cl.Releases))
	}
}

func TestFetchChangelogUnsupportedEcosystem(t *testing.T) {
	f := &ReleaseNotesFetcher{Client: http.DefaultClient}

	cl, err := f.FetchChangelog(context.Background(), "pip", "requests",
		mustParse(t, "2.30.0"), mustParse(t, "2.31.0"))
	if err != nil {
		t.Fatalf("FetchChangelog() error: %v", err)
	}
	if cl != nil {
		t.Errorf("expected nil for unsupported ecosystem, got %+v", cl)
	}
}

func TestParseGitHubRepoURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{
			name:      "object with https url",
			input:     `{"type":"git","url":"git+https://github.com/expressjs/express.git"}`,
			wantOwner: "expressjs", wantRepo: "express", wantOK: true,
		},
		{
			name:      "object with ssh url",
			input:     `{"type":"git","url":"git@github.com:lodash/lodash.git"}`,
			wantOwner: "lodash", wantRepo: "lodash", wantOK: true,
		},
		{
			name:      "string shorthand",
			input:     `"https://github.com/chalk/chalk"`,
			wantOwner: "chalk", wantRepo: "chalk", wantOK: true,
		},
		{
			name:      "non-github url",
			input:     `{"type":"git","url":"https://gitlab.com/foo/bar.git"}`,
			wantOwner: "", wantRepo: "", wantOK: false,
		},
		{
			name:      "empty",
			input:     ``,
			wantOwner: "", wantRepo: "", wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := parseGitHubRepoURL(json.RawMessage(tt.input))
			if owner != tt.wantOwner || repo != tt.wantRepo || ok != tt.wantOK {
				t.Errorf("parseGitHubRepoURL(%s) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input, owner, repo, ok, tt.wantOwner, tt.wantRepo, tt.wantOK)
			}
		})
	}
}

func TestGitHubRepoFromGoModule(t *testing.T) {
	tests := []struct {
		path      string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{"github.com/stretchr/testify", "stretchr", "testify", true},
		{"github.com/go-chi/chi/v5", "go-chi", "chi", true},
		{"golang.org/x/net", "", "", false},
		{"github.com/solo", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			owner, repo, ok := githubRepoFromGoModule(tt.path)
			if owner != tt.wantOwner || repo != tt.wantRepo || ok != tt.wantOK {
				t.Errorf("githubRepoFromGoModule(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.path, owner, repo, ok, tt.wantOwner, tt.wantRepo, tt.wantOK)
			}
		})
	}
}
