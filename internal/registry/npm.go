package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jandubois/cooldown/internal/version"
)

// NPM fetches versions from the npm registry (registry.npmjs.org).
type NPM struct {
	Client  *http.Client
	BaseURL string // override for testing; defaults to https://registry.npmjs.org
}

func (n *NPM) baseURL() string {
	if n.BaseURL != "" {
		return n.BaseURL
	}
	return "https://registry.npmjs.org"
}

// Versions returns all available versions for an npm package.
func (n *NPM) Versions(ctx context.Context, name string) ([]version.Version, error) {
	url := fmt.Sprintf("%s/%s", n.baseURL(), name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Use abbreviated metadata to reduce response size
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching versions for %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ErrNotFound{Name: name}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d from npm for %s: %s", resp.StatusCode, name, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response for %s: %w", name, err)
	}

	// The abbreviated response has a "versions" object keyed by version string.
	var doc struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing npm response for %s: %w", name, err)
	}

	var versions []version.Version
	for vStr := range doc.Versions {
		v, err := version.Parse(vStr)
		if err != nil {
			continue // skip unparseable versions
		}
		versions = append(versions, v)
	}

	return versions, nil
}
