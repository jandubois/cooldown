package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubVersions(t *testing.T) {
	releases := []ghRelease{
		{TagName: "v4.1.2", Prerelease: false, Draft: false},
		{TagName: "v4.1.1", Prerelease: false, Draft: false},
		{TagName: "v4.0.0", Prerelease: false, Draft: false},
		{TagName: "v3.6.0", Prerelease: false, Draft: false},
		{TagName: "v5.0.0-beta", Prerelease: true, Draft: false},
		{TagName: "v4.2.0-rc.1", Prerelease: true, Draft: false},
		{TagName: "v4.1.3", Prerelease: false, Draft: true}, // draft, should be skipped
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			json.NewEncoder(w).Encode([]ghRelease{}) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(releases) //nolint:errcheck
	}))
	defer srv.Close()

	g := &GitHub{Client: srv.Client(), BaseURL: srv.URL}
	versions, err := g.Versions(context.Background(), "actions/checkout")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	// Should have: v4.1.2, v4.1.1, v4.0.0, v3.6.0, v5.0.0-beta, v4.2.0-rc.1
	// Draft v4.1.3 excluded; v5.0.0-beta marked as prerelease via tag
	// v4.2.0-rc.1 marked as prerelease via GitHub flag
	if len(versions) != 6 {
		t.Fatalf("got %d versions, want 6: %v", len(versions), versions)
	}

	// Check that the prerelease-flagged release without a pre tag gets marked
	for _, v := range versions {
		if v.Raw == "v5.0.0-beta" && !v.IsPrerelease() {
			t.Error("v5.0.0-beta should be prerelease")
		}
		if v.Raw == "v4.2.0-rc.1" && !v.IsPrerelease() {
			t.Error("v4.2.0-rc.1 should be prerelease")
		}
	}
}

func TestGitHubFallbackToTags(t *testing.T) {
	tags := []ghTag{
		{Name: "v1.2.3"},
		{Name: "v1.2.2"},
		{Name: "v1"},     // floating tag, should be skipped (not valid semver with 2+ parts)
		{Name: "latest"}, // not semver, should be skipped
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/repos/owner/repo/releases" {
			json.NewEncoder(w).Encode([]ghRelease{}) //nolint:errcheck
			return
		}
		if r.URL.Query().Get("page") == "2" {
			json.NewEncoder(w).Encode([]ghTag{}) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(tags) //nolint:errcheck
	}))
	defer srv.Close()

	g := &GitHub{Client: srv.Client(), BaseURL: srv.URL}
	versions, err := g.Versions(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2: %v", len(versions), versions)
	}
}

func TestGitHubNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	g := &GitHub{Client: srv.Client(), BaseURL: srv.URL}
	_, err := g.Versions(context.Background(), "nonexistent/repo")

	if _, ok := err.(*ErrNotFound); !ok {
		t.Fatalf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestGitHubAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ghRelease{}) //nolint:errcheck
	}))
	defer srv.Close()

	g := &GitHub{Client: srv.Client(), BaseURL: srv.URL, Token: "test-token"}
	g.Versions(context.Background(), "owner/repo") //nolint:errcheck

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
}
