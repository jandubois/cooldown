// Package metadata parses the JSON output of dependabot/fetch-metadata.
package metadata

import (
	"encoding/json"
	"fmt"
)

// Dependency represents a single dependency update from a Dependabot PR.
type Dependency struct {
	Name       string `json:"dependency-name"`
	Type       string `json:"dependency-type"`
	UpdateType string `json:"update-type"`
	Ecosystem  string `json:"package-ecosystem"`
	Directory  string `json:"directory"`
	NewVersion string `json:"new-version"`
	PrevVersion string `json:"previous-version"`
	Group      string `json:"dependency-group"`
}

// Parse parses the JSON array from the fetch-metadata updated-dependencies-json output.
func Parse(data string) ([]Dependency, error) {
	if data == "" {
		return nil, nil
	}

	var deps []Dependency
	if err := json.Unmarshal([]byte(data), &deps); err != nil {
		return nil, fmt.Errorf("parsing dependencies JSON: %w", err)
	}
	return deps, nil
}
