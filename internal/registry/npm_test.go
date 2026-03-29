package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNPMVersions(t *testing.T) {
	response := `{
		"versions": {
			"4.17.19": {},
			"4.17.20": {},
			"4.17.21": {},
			"5.0.0-beta.1": {}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.npm.install-v1+json" {
			t.Errorf("Accept header = %q, want abbreviated metadata", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response)) //nolint:errcheck
	}))
	defer srv.Close()

	n := &NPM{Client: srv.Client(), BaseURL: srv.URL}
	versions, err := n.Versions(context.Background(), "lodash")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	if len(versions) != 4 {
		t.Fatalf("got %d versions, want 4: %v", len(versions), versions)
	}
}

func TestNPMScopedPackage(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"versions": {"18.2.0": {}}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	n := &NPM{Client: srv.Client(), BaseURL: srv.URL}
	_, err := n.Versions(context.Background(), "@angular/core")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	if gotPath != "/@angular/core" {
		t.Errorf("request path = %q, want %q", gotPath, "/@angular/core")
	}
}

func TestNPMNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	n := &NPM{Client: srv.Client(), BaseURL: srv.URL}
	_, err := n.Versions(context.Background(), "nonexistent-package")

	if _, ok := err.(*ErrNotFound); !ok {
		t.Fatalf("expected ErrNotFound, got %T: %v", err, err)
	}
}
