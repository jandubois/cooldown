package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGoProxyVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v1.0.0\nv1.1.0\nv1.2.0\nv1.3.0-rc.1\nv2.0.0\n")) //nolint:errcheck
	}))
	defer srv.Close()

	g := &GoProxy{Client: srv.Client(), BaseURL: srv.URL}
	versions, err := g.Versions(context.Background(), "golang.org/x/net")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	if len(versions) != 5 {
		t.Fatalf("got %d versions, want 5: %v", len(versions), versions)
	}
}

func TestGoProxyNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	g := &GoProxy{Client: srv.Client(), BaseURL: srv.URL}
	_, err := g.Versions(context.Background(), "nonexistent/module")

	if _, ok := err.(*ErrNotFound); !ok {
		t.Fatalf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestGoProxyEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("")) //nolint:errcheck
	}))
	defer srv.Close()

	g := &GoProxy{Client: srv.Client(), BaseURL: srv.URL}
	versions, err := g.Versions(context.Background(), "example.com/mod")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("got %d versions, want 0", len(versions))
	}
}

func TestEncodeModulePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"golang.org/x/net", "golang.org/x/net"},
		{"github.com/Azure/azure-sdk-for-go", "github.com/!azure/azure-sdk-for-go"},
		{"github.com/BurntSushi/toml", "github.com/!burnt!sushi/toml"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := encodeModulePath(tt.input)
			if got != tt.want {
				t.Errorf("encodeModulePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
