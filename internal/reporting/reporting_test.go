package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jandubois/cooldown/internal/checker"
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
			Name:       "lodash",
			Ecosystem:  "npm_and_yarn",
			Proposed:   mustParse(t, "4.17.20"),
			Latest:     mustParse(t, "4.17.21"),
			Stale:      true,
			ReleaseURL: "https://github.com/lodash/lodash/releases/tag/v4.17.21",
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
	if !strings.Contains(output, "https://github.com/lodash/lodash/releases/tag/v4.17.21") {
		t.Errorf("missing release URL in annotation, got:\n%s", output)
	}
	if !strings.Contains(output, `::warning title=requests skipped::unsupported ecosystem "pip"`) {
		t.Errorf("missing warning annotation for requests, got:\n%s", output)
	}
	if strings.Contains(output, "express") {
		t.Errorf("should not annotate non-stale, non-skipped deps, got:\n%s", output)
	}
}

func TestWriteAnnotationsNoURL(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "lodash",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.17.20"),
			Latest:    mustParse(t, "4.17.21"),
			Stale:     true,
		},
	}

	var buf bytes.Buffer
	WriteAnnotations(&buf, results)
	output := buf.String()

	// Should not contain the " — " separator when there's no URL
	if strings.Contains(output, " — ") {
		t.Errorf("annotation should not have URL separator when no URL, got:\n%s", output)
	}
}

func TestWriteSummaryMarkdown(t *testing.T) {
	results := []checker.Result{
		{
			Name:        "lodash",
			Ecosystem:   "npm_and_yarn",
			Proposed:    mustParse(t, "4.17.20"),
			Latest:      mustParse(t, "4.17.21"),
			Stale:       true,
			ReleaseURL:  "https://github.com/lodash/lodash/releases/tag/v4.17.21",
			ReleaseBody: "## Changes\n* Fixed prototype pollution",
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
	if err := writeSummaryMarkdown(&buf, results); err != nil {
		t.Fatalf("writeSummaryMarkdown() error: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "newer versions available") {
		t.Error("title should indicate stale deps exist")
	}

	// Latest column should be a link
	if !strings.Contains(output, "[**4.17.21**](https://github.com/lodash/lodash/releases/tag/v4.17.21)") {
		t.Errorf("missing linked latest version, got:\n%s", output)
	}

	if !strings.Contains(output, "| express | npm_and_yarn | 4.18.2 | 4.18.2 | :white_check_mark: |") {
		t.Errorf("missing ok row, got:\n%s", output)
	}
	if !strings.Contains(output, ":warning:") {
		t.Errorf("missing skipped row, got:\n%s", output)
	}

	// Collapsed release notes
	if !strings.Contains(output, "<details>") {
		t.Error("missing collapsed release notes")
	}
	if !strings.Contains(output, "lodash 4.17.21 release notes") {
		t.Error("missing release notes summary text")
	}
	if !strings.Contains(output, "Fixed prototype pollution") {
		t.Error("missing release notes body")
	}
}

func TestWriteSummaryNoReleaseBody(t *testing.T) {
	results := []checker.Result{
		{
			Name:       "lodash",
			Ecosystem:  "npm_and_yarn",
			Proposed:   mustParse(t, "4.17.20"),
			Latest:     mustParse(t, "4.17.21"),
			Stale:      true,
			ReleaseURL: "https://github.com/lodash/lodash/releases/tag/v4.17.21",
			// No ReleaseBody
		},
	}

	var buf bytes.Buffer
	if err := writeSummaryMarkdown(&buf, results); err != nil {
		t.Fatalf("writeSummaryMarkdown() error: %v", err)
	}

	// Should have link in table but no <details> section
	if !strings.Contains(buf.String(), "[**4.17.21**]") {
		t.Error("missing linked version in table")
	}
	if strings.Contains(buf.String(), "<details>") {
		t.Error("should not have collapsed section when body is empty")
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
