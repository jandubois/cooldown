// Package registry provides clients that fetch available versions from
// package registries (GitHub Actions, Go modules, npm).
package registry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jandubois/cooldown/internal/version"
)

// Registry fetches available versions for a named dependency.
type Registry interface {
	// Versions returns all available versions for the given dependency.
	Versions(ctx context.Context, name string) ([]version.Version, error)
}

// ForEcosystem returns the appropriate registry client for a dependabot
// package-ecosystem identifier. Returns nil for unsupported ecosystems.
func ForEcosystem(ecosystem string, httpClient *http.Client, githubToken string) Registry {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	switch ecosystem {
	case "github_actions":
		return &GitHub{Client: httpClient, Token: githubToken}
	case "gomod":
		return &GoProxy{Client: httpClient}
	case "npm_and_yarn":
		return &NPM{Client: httpClient}
	default:
		return nil
	}
}

// ErrNotFound indicates a package was not found in the registry.
type ErrNotFound struct {
	Name string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("package %q not found", e.Name)
}
