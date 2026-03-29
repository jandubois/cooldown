package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jandubois/cooldown/internal/checker"
	"github.com/jandubois/cooldown/internal/registry"
	"github.com/jandubois/cooldown/internal/version"
)

func mustParse(t *testing.T, s string) version.Version {
	t.Helper()
	v, err := version.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", s, err)
	}
	return v
}

func TestWriteAnnotations(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "lodash",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.17.20"),
			Latest:    mustParse(t, "4.17.21"),
			Stale:     true,
			Changelog: &registry.Changelog{
				CompareURL: "https://github.com/lodash/lodash/compare/v4.17.20...v4.17.21",
			},
		},
		{
			Name:      "express",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.18.2"),
			Latest:    mustParse(t, "4.18.2"),
			Stale:     false,
		},
		{
			Name:      "requests",
			Ecosystem: "pip",
			Skipped:   true,
			Reason:    `unsupported ecosystem "pip"`,
		},
	}

	var buf bytes.Buffer
	WriteAnnotations(&buf, results)
	output := buf.String()

	if !strings.Contains(output, "::error title=lodash is outdated::lodash 4.17.21 is available (proposed 4.17.20)") {
		t.Errorf("missing error annotation for lodash, got:\n%s", output)
	}
	if !strings.Contains(output, "https://github.com/lodash/lodash/compare/v4.17.20...v4.17.21") {
		t.Errorf("missing compare URL in annotation, got:\n%s", output)
	}
	if !strings.Contains(output, `::warning title=requests skipped::unsupported ecosystem "pip"`) {
		t.Errorf("missing warning annotation for requests, got:\n%s", output)
	}
	if strings.Contains(output, "express") {
		t.Errorf("should not annotate non-stale, non-skipped deps, got:\n%s", output)
	}
}

func TestWriteSummaryWithChangelog(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "lodash",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.17.20"),
			Latest:    mustParse(t, "4.17.21"),
			Stale:     true,
			Changelog: &registry.Changelog{
				Owner:      "lodash",
				Repo:       "lodash",
				CompareURL: "https://github.com/lodash/lodash/compare/v4.17.20...v4.17.21",
				Releases: []registry.VersionRelease{
					{
						Version: mustParse(t, "4.17.21"),
						Tag:     "v4.17.21",
						URL:     "https://github.com/lodash/lodash/releases/tag/v4.17.21",
						Body:    "## Changes\n* Fixed prototype pollution",
					},
				},
			},
		},
		{
			Name:      "express",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.18.2"),
			Latest:    mustParse(t, "4.18.2"),
			Stale:     false,
		},
	}

	var buf bytes.Buffer
	if err := writeSummaryMarkdown(&buf, results); err != nil {
		t.Fatalf("writeSummaryMarkdown() error: %v", err)
	}
	output := buf.String()

	// Table should have linked latest version
	if !strings.Contains(output, "[**4.17.21**](https://github.com/lodash/lodash/compare/v4.17.20...v4.17.21)") {
		t.Errorf("missing linked latest version, got:\n%s", output)
	}

	// Changelog section
	if !strings.Contains(output, "lodash: 4.17.20 → 4.17.21") {
		t.Errorf("missing changelog header, got:\n%s", output)
	}
	if !strings.Contains(output, "Release notes") {
		t.Errorf("missing release notes section, got:\n%s", output)
	}
	if !strings.Contains(output, "Fixed prototype pollution") {
		t.Errorf("missing release body, got:\n%s", output)
	}
	if !strings.Contains(output, "View changes on GitHub") {
		t.Errorf("missing compare link, got:\n%s", output)
	}
	if !strings.Contains(output, "lodash/lodash's releases") {
		t.Errorf("missing source attribution, got:\n%s", output)
	}
}

func TestWriteSummaryMultipleReleases(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "actions/checkout",
			Ecosystem: "github_actions",
			Proposed:  mustParse(t, "v4.1.0"),
			Latest:    mustParse(t, "v4.3.0"),
			Stale:     true,
			Changelog: &registry.Changelog{
				Owner:      "actions",
				Repo:       "checkout",
				CompareURL: "https://github.com/actions/checkout/compare/v4.1.0...v4.3.0",
				Releases: []registry.VersionRelease{
					{Version: mustParse(t, "v4.2.0"), Tag: "v4.2.0", URL: "url1", Body: "Bug fix release"},
					{Version: mustParse(t, "v4.3.0"), Tag: "v4.3.0", URL: "url2", Body: "Feature release"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := writeSummaryMarkdown(&buf, results); err != nil {
		t.Fatalf("writeSummaryMarkdown() error: %v", err)
	}
	output := buf.String()

	// Multiple releases should show count
	if !strings.Contains(output, "Release notes (2 versions)") {
		t.Errorf("missing multi-release header, got:\n%s", output)
	}
	if !strings.Contains(output, "Bug fix release") {
		t.Errorf("missing first release body, got:\n%s", output)
	}
	if !strings.Contains(output, "Feature release") {
		t.Errorf("missing second release body, got:\n%s", output)
	}
}

func TestWriteSummaryNoChangelog(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "lodash",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.17.20"),
			Latest:    mustParse(t, "4.17.21"),
			Stale:     true,
			// No Changelog
		},
	}

	var buf bytes.Buffer
	if err := writeSummaryMarkdown(&buf, results); err != nil {
		t.Fatalf("writeSummaryMarkdown() error: %v", err)
	}
	output := buf.String()

	// Should have the table but no changelog section
	if !strings.Contains(output, "**4.17.21**") {
		t.Error("missing bold latest in table")
	}
	if strings.Contains(output, "Release notes") {
		t.Error("should not have release notes when changelog is nil")
	}
}

func TestWriteSummaryAllCurrent(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "express",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.18.2"),
			Latest:    mustParse(t, "4.18.2"),
			Stale:     false,
		},
	}

	var buf bytes.Buffer
	if err := writeSummaryMarkdown(&buf, results); err != nil {
		t.Fatalf("writeSummaryMarkdown() error: %v", err)
	}

	if !strings.Contains(buf.String(), "all dependencies are current") {
		t.Error("title should indicate all deps are current")
	}
}
