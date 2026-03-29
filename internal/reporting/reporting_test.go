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
			Name:      "lodash",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.17.20"),
			Latest:    mustParse(t, "4.17.21"),
			Stale:     true,
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
	if !strings.Contains(output, `::warning title=requests skipped::unsupported ecosystem "pip"`) {
		t.Errorf("missing warning annotation for requests, got:\n%s", output)
	}
	if strings.Contains(output, "express") {
		t.Errorf("should not annotate non-stale, non-skipped deps, got:\n%s", output)
	}
}

func TestWriteSummaryMarkdown(t *testing.T) {
	results := []checker.Result{
		{
			Name:      "lodash",
			Ecosystem: "npm_and_yarn",
			Proposed:  mustParse(t, "4.17.20"),
			Latest:    mustParse(t, "4.17.21"),
			Stale:     true,
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
	if !strings.Contains(output, "| lodash | npm_and_yarn | 4.17.20 | **4.17.21** | :x: outdated |") {
		t.Errorf("missing stale row, got:\n%s", output)
	}
	if !strings.Contains(output, "| express | npm_and_yarn | 4.18.2 | 4.18.2 | :white_check_mark: |") {
		t.Errorf("missing ok row, got:\n%s", output)
	}
	if !strings.Contains(output, ":warning:") {
		t.Errorf("missing skipped row, got:\n%s", output)
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
