// Package checker orchestrates version checks across dependencies.
package checker

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/jandubois/cooldown/internal/metadata"
	"github.com/jandubois/cooldown/internal/registry"
	"github.com/jandubois/cooldown/internal/version"
)

// Result describes the outcome of checking a single dependency.
type Result struct {
	Name      string
	Ecosystem string
	Proposed  version.Version
	Latest    version.Version
	Stale     bool   // true if a newer version exists within the same major
	Skipped   bool   // true if the ecosystem is unsupported or a lookup error occurred
	Reason    string // explanation when skipped
}

// Checker verifies that Dependabot PRs propose the latest version.
type Checker struct {
	HTTPClient  *http.Client
	GitHubToken string
}

// Check examines each dependency and reports whether its proposed version
// is the latest available within the same major version line.
func (c *Checker) Check(ctx context.Context, deps []metadata.Dependency) []Result {
	var results []Result

	for _, dep := range deps {
		r := c.checkOne(ctx, dep)
		results = append(results, r)
	}

	return results
}

// HasStale reports whether any result indicates a stale dependency.
func HasStale(results []Result) bool {
	for _, r := range results {
		if r.Stale {
			return true
		}
	}
	return false
}

func (c *Checker) checkOne(ctx context.Context, dep metadata.Dependency) Result {
	result := Result{
		Name:      dep.Name,
		Ecosystem: dep.Ecosystem,
	}

	reg := registry.ForEcosystem(dep.Ecosystem, c.HTTPClient, c.GitHubToken)
	if reg == nil {
		result.Skipped = true
		result.Reason = fmt.Sprintf("unsupported ecosystem %q", dep.Ecosystem)
		log.Printf("SKIP %s: %s", dep.Name, result.Reason)
		return result
	}

	proposed, err := version.Parse(dep.NewVersion)
	if err != nil {
		result.Skipped = true
		result.Reason = fmt.Sprintf("cannot parse proposed version %q: %v", dep.NewVersion, err)
		log.Printf("SKIP %s: %s", dep.Name, result.Reason)
		return result
	}
	result.Proposed = proposed

	available, err := reg.Versions(ctx, dep.Name)
	if err != nil {
		// Fail open: registry errors should not block PRs
		result.Skipped = true
		result.Reason = fmt.Sprintf("registry lookup failed: %v", err)
		log.Printf("SKIP %s: %s", dep.Name, result.Reason)
		return result
	}

	latest, hasNewer := version.LatestForMajor(proposed, available)
	result.Latest = latest
	result.Stale = hasNewer

	if hasNewer {
		log.Printf("STALE %s: proposed %s, latest %s", dep.Name, proposed, latest)
	} else {
		log.Printf("OK %s: %s is the latest in v%d.x", dep.Name, proposed, proposed.Major)
	}

	return result
}
