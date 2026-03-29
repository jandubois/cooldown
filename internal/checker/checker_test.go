package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jandubois/cooldown/internal/metadata"
)

func TestCheckLatest(t *testing.T) {
	// npm registry mock returning lodash versions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"versions": {"4.17.19": {}, "4.17.20": {}, "4.17.21": {}}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Checker{HTTPClient: srv.Client()}

	deps := []metadata.Dependency{
		{
			Name:        "lodash",
			Ecosystem:   "npm_and_yarn",
			NewVersion:  "4.17.21",
			PrevVersion: "4.17.20",
		},
	}

	// Patch the registry to use our test server
	results := checkWithBaseURL(t, c, deps, srv.URL)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Stale {
		t.Error("expected not stale when proposed version is latest")
	}
	if results[0].Skipped {
		t.Errorf("expected not skipped, reason: %s", results[0].Reason)
	}
}

func TestCheckStale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"versions": {"4.17.19": {}, "4.17.20": {}, "4.17.21": {}}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := &Checker{HTTPClient: srv.Client()}

	deps := []metadata.Dependency{
		{
			Name:        "lodash",
			Ecosystem:   "npm_and_yarn",
			NewVersion:  "4.17.20",
			PrevVersion: "4.17.19",
		},
	}

	results := checkWithBaseURL(t, c, deps, srv.URL)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].Stale {
		t.Error("expected stale when newer version exists")
	}
	if results[0].Latest.String() != "4.17.21" {
		t.Errorf("Latest = %s, want 4.17.21", results[0].Latest)
	}
}

func TestCheckUnsupportedEcosystem(t *testing.T) {
	c := &Checker{}

	deps := []metadata.Dependency{
		{
			Name:       "requests",
			Ecosystem:  "pip",
			NewVersion: "2.31.0",
		},
	}

	results := c.Check(context.Background(), deps)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected skipped for unsupported ecosystem")
	}
	if results[0].Stale {
		t.Error("skipped dependencies must not be marked stale")
	}
}

func TestCheckRegistryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Checker{HTTPClient: srv.Client()}

	deps := []metadata.Dependency{
		{
			Name:       "lodash",
			Ecosystem:  "npm_and_yarn",
			NewVersion: "4.17.21",
		},
	}

	results := checkWithBaseURL(t, c, deps, srv.URL)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected skipped on registry error (fail open)")
	}
	if results[0].Stale {
		t.Error("should not be stale on registry error")
	}
}

func TestCheckGroupedDeps(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/lodash", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// lodash is up to date
		w.Write([]byte(`{"versions": {"4.17.21": {}}}`)) //nolint:errcheck
	})
	mux.HandleFunc("/express", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// express has a newer version
		w.Write([]byte(`{"versions": {"4.18.0": {}, "4.18.1": {}, "4.18.2": {}}}`)) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Checker{HTTPClient: srv.Client()}

	deps := []metadata.Dependency{
		{Name: "lodash", Ecosystem: "npm_and_yarn", NewVersion: "4.17.21", Group: "npm-deps"},
		{Name: "express", Ecosystem: "npm_and_yarn", NewVersion: "4.18.0", Group: "npm-deps"},
	}

	results := checkWithBaseURL(t, c, deps, srv.URL)

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Stale {
		t.Error("lodash should not be stale")
	}
	if !results[1].Stale {
		t.Error("express should be stale")
	}
	if !HasStale(results) {
		t.Error("HasStale should return true when any dep is stale")
	}
}

func TestHasStaleAllCurrent(t *testing.T) {
	results := []Result{
		{Stale: false},
		{Stale: false, Skipped: true},
	}
	if HasStale(results) {
		t.Error("HasStale should return false when no dep is stale")
	}
}

func TestAllSkipped(t *testing.T) {
	if !AllSkipped([]Result{{Skipped: true}, {Skipped: true}}) {
		t.Error("AllSkipped should return true when all are skipped")
	}
	if AllSkipped([]Result{{Skipped: true}, {Skipped: false}}) {
		t.Error("AllSkipped should return false when some are not skipped")
	}
	if AllSkipped([]Result{}) {
		t.Error("AllSkipped should return false for empty results")
	}
}

func TestHasSkipped(t *testing.T) {
	if !HasSkipped([]Result{{Skipped: false}, {Skipped: true}}) {
		t.Error("HasSkipped should return true when any is skipped")
	}
	if HasSkipped([]Result{{Skipped: false}, {Skipped: false}}) {
		t.Error("HasSkipped should return false when none are skipped")
	}
}

// checkWithBaseURL runs the checker with a custom base URL for all registries.
// This works because the checker creates registries via ForEcosystem, but we
// need to override the base URLs for testing. We do this by directly calling
// the registry and checking results, but for integration, we take a simpler
// approach: mock the HTTP server to respond to any path.
func checkWithBaseURL(t *testing.T, c *Checker, deps []metadata.Dependency, baseURL string) []Result {
	t.Helper()

	// We need to work around the checker creating registries internally.
	// The simplest approach: serialize deps to JSON, create a new checker
	// that intercepts HTTP calls at the base URL level.
	// Actually, we can just inject the test server URL by making the HTTP
	// client redirect all requests to our test server.

	// Better approach: test at a higher level by using the metadata + registry
	// directly. But the checker test should test the checker's orchestration.
	// Let's use a transport that rewrites URLs.
	originalTransport := c.HTTPClient.Transport
	c.HTTPClient.Transport = &rewriteTransport{
		baseURL:   baseURL,
		transport: originalTransport,
	}
	defer func() { c.HTTPClient.Transport = originalTransport }()

	return c.Check(context.Background(), deps)
}

// rewriteTransport redirects all HTTP requests to a test server.
type rewriteTransport struct {
	baseURL   string
	transport http.RoundTripper
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point at our test server, keeping the path
	newURL := rt.baseURL + req.URL.Path
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header

	transport := rt.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(newReq)
}

func TestCheckResultJSON(t *testing.T) {
	// Verify results can be serialized (useful for future output formatting)
	results := []Result{
		{Name: "lodash", Ecosystem: "npm_and_yarn", Stale: true},
	}
	_, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("failed to marshal results: %v", err)
	}
}
