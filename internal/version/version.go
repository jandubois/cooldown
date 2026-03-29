// Package version provides semver parsing, comparison, and filtering.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a parsed semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string // empty for stable releases
	Raw        string // original string before parsing
}

// Parse parses a semver string into a Version. Accepts optional "v" prefix.
// Returns an error if the string is not a valid semver.
func Parse(s string) (Version, error) {
	raw := s
	s = strings.TrimPrefix(s, "v")

	// Split off prerelease (and build metadata)
	s, _ = strings.CutSuffix(s, "+incompatible") // Go module compat suffix
	base, pre, _ := strings.Cut(s, "-")

	// Strip build metadata from prerelease if present
	if idx := strings.Index(pre, "+"); idx >= 0 {
		pre = pre[:idx]
	}

	parts := strings.Split(base, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid semver: %q", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version in %q: %w", raw, err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version in %q: %w", raw, err)
	}

	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version in %q: %w", raw, err)
		}
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: pre,
		Raw:        raw,
	}, nil
}

// IsPrerelease reports whether this version has a prerelease tag.
func (v Version) IsPrerelease() bool {
	return v.Prerelease != ""
}

// Compare returns -1, 0, or 1 comparing v to other by semver precedence.
// Prerelease versions sort before their release counterpart.
func (v Version) Compare(other Version) int {
	if c := cmpInt(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmpInt(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmpInt(v.Patch, other.Patch); c != 0 {
		return c
	}
	return cmpPrerelease(v.Prerelease, other.Prerelease)
}

// Less reports whether v is strictly less than other.
func (v Version) Less(other Version) bool {
	return v.Compare(other) < 0
}

// String returns the original raw string used to create this version.
func (v Version) String() string {
	return v.Raw
}

// LatestForMajor finds the latest stable version with the same major version
// as proposed. Returns the latest version found and whether a newer version
// than proposed exists. If no versions match, returns proposed and false.
func LatestForMajor(proposed Version, available []Version) (latest Version, hasNewer bool) {
	latest = proposed
	for _, v := range available {
		if v.Major != proposed.Major {
			continue
		}
		if v.IsPrerelease() {
			continue
		}
		if v.Compare(latest) > 0 {
			latest = v
		}
	}
	return latest, latest.Compare(proposed) > 0
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// cmpPrerelease compares prerelease strings per semver spec:
// - no prerelease > any prerelease (stable beats prerelease)
// - otherwise lexicographic comparison of dot-separated identifiers
func cmpPrerelease(a, b string) int {
	if a == b {
		return 0
	}
	// Stable (empty prerelease) has higher precedence than any prerelease
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := range max(len(aParts), len(bParts)) {
		if i >= len(aParts) {
			return -1 // a has fewer fields, so a < b
		}
		if i >= len(bParts) {
			return 1 // b has fewer fields, so a > b
		}
		if c := cmpIdentifier(aParts[i], bParts[i]); c != 0 {
			return c
		}
	}
	return 0
}

// cmpIdentifier compares two prerelease identifiers per semver spec:
// numeric identifiers sort numerically, string identifiers sort lexically,
// numeric always sorts before string.
func cmpIdentifier(a, b string) int {
	aNum, aErr := strconv.Atoi(a)
	bNum, bErr := strconv.Atoi(b)

	switch {
	case aErr == nil && bErr == nil:
		return cmpInt(aNum, bNum)
	case aErr == nil:
		return -1 // numeric < string
	case bErr == nil:
		return 1 // string > numeric
	default:
		return strings.Compare(a, b)
	}
}
