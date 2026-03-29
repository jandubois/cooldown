package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/jandubois/cooldown/internal/version"
)

// GoProxy fetches versions from the Go module proxy (proxy.golang.org).
type GoProxy struct {
	Client  *http.Client
	BaseURL string // override for testing; defaults to https://proxy.golang.org
}

func (g *GoProxy) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return "https://proxy.golang.org"
}

// Versions returns all available versions for a Go module.
func (g *GoProxy) Versions(ctx context.Context, name string) ([]version.Version, error) {
	encoded := encodeModulePath(name)
	url := fmt.Sprintf("%s/%s/@v/list", g.baseURL(), encoded)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching versions for %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, &ErrNotFound{Name: name}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d from Go proxy for %s: %s", resp.StatusCode, name, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response for %s: %w", name, err)
	}

	var versions []version.Version
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := version.Parse(line)
		if err != nil {
			continue // skip unparseable versions
		}
		versions = append(versions, v)
	}

	return versions, nil
}

// encodeModulePath encodes a Go module path for use in proxy URLs.
// Uppercase letters are escaped as !lowercase per the Go module proxy protocol.
func encodeModulePath(path string) string {
	var b strings.Builder
	for _, r := range path {
		if unicode.IsUpper(r) {
			b.WriteByte('!')
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
