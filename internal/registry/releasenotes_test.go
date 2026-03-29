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

func TestFetchGitHubActions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/actions/checkout/releases/tags/v4.1.2" {
			json.NewEncoder(w).Encode(map[string]string{
				"html_url": "https://github.com/actions/checkout/releases/tag/v4.1.2",
				"body":     "## What's Changed\n* Fix sparse checkout",
			}) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &ReleaseNotesFetcher{
		Client:        srv.Client(),
		GitHubBaseURL: srv.URL,
	}

	info, err := f.Fetch(context.Background(), "github_actions", "actions/checkout", mustParse(t, "v4.1.2"))
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if info == nil {
		t.Fatal("Fetch() returned nil")
	}
	if info.URL != "https://github.com/actions/checkout/releases/tag/v4.1.2" {
		t.Errorf("URL = %q", info.URL)
	}
	if info.Body != "## What's Changed\n* Fix sparse checkout" {
		t.Errorf("Body = %q", info.Body)
	}
}

func TestFetchGoModule(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/stretchr/testify/releases/tags/v1.9.0" {
			json.NewEncoder(w).Encode(map[string]string{
				"html_url": "https://github.com/stretchr/testify/releases/tag/v1.9.0",
				"body":     "Release v1.9.0",
			}) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &ReleaseNotesFetcher{
		Client:        srv.Client(),
		GitHubBaseURL: srv.URL,
	}

	info, err := f.Fetch(context.Background(), "gomod", "github.com/stretchr/testify", mustParse(t, "v1.9.0"))
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if info == nil {
		t.Fatal("Fetch() returned nil")
	}
	if info.Body != "Release v1.9.0" {
		t.Errorf("Body = %q", info.Body)
	}
}

func TestFetchGoModuleNonGitHub(t *testing.T) {
	f := &ReleaseNotesFetcher{Client: http.DefaultClient}

	info, err := f.Fetch(context.Background(), "gomod", "golang.org/x/net", mustParse(t, "v0.17.0"))
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil for non-GitHub module, got %+v", info)
	}
}

func TestFetchNPM(t *testing.T) {
	mux := http.NewServeMux()

	// npm registry: return repository info
	mux.HandleFunc("/express/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"repository": map[string]string{
				"type": "git",
				"url":  "git+https://github.com/expressjs/express.git",
			},
		}) //nolint:errcheck
	})

	// GitHub API: return release for the tag
	mux.HandleFunc("/repos/expressjs/express/releases/tags/v4.22.1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"html_url": "https://github.com/expressjs/express/releases/tag/v4.22.1",
			"body":     "### 4.22.1\n* Security fix",
		}) //nolint:errcheck
	})

	// 404 for tag without v prefix
	mux.HandleFunc("/repos/expressjs/express/releases/tags/4.22.1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := &ReleaseNotesFetcher{
		Client:        srv.Client(),
		GitHubBaseURL: srv.URL,
		NPMBaseURL:    srv.URL,
	}

	info, err := f.Fetch(context.Background(), "npm_and_yarn", "express", mustParse(t, "4.22.1"))
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if info == nil {
		t.Fatal("Fetch() returned nil")
	}
	if info.Body != "### 4.22.1\n* Security fix" {
		t.Errorf("Body = %q", info.Body)
	}
}

func TestFetchUnsupportedEcosystem(t *testing.T) {
	f := &ReleaseNotesFetcher{Client: http.DefaultClient}

	info, err := f.Fetch(context.Background(), "pip", "requests", mustParse(t, "2.31.0"))
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil for unsupported ecosystem, got %+v", info)
	}
}

func TestCandidateTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"v4.1.2", []string{"v4.1.2", "4.1.2"}},
		{"4.17.21", []string{"v4.17.21", "4.17.21"}},
		{"v1.0.0-rc.1", []string{"v1.0.0-rc.1", "1.0.0-rc.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v := mustParse(t, tt.input)
			got := candidateTags(v)
			if len(got) != len(tt.want) {
				t.Fatalf("candidateTags(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("candidateTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
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

func TestFetchNoRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &ReleaseNotesFetcher{
		Client:        srv.Client(),
		GitHubBaseURL: srv.URL,
	}

	info, err := f.Fetch(context.Background(), "github_actions", "actions/checkout", mustParse(t, "v4.1.2"))
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if info == nil {
		t.Fatal("expected fallback URL, got nil")
	}
	if info.URL != "https://github.com/actions/checkout/releases" {
		t.Errorf("fallback URL = %q", info.URL)
	}
	if info.Body != "" {
		t.Errorf("expected empty body for fallback, got %q", info.Body)
	}
}
