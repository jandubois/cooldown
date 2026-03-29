// Package reporting formats checker results for GitHub Actions output:
// workflow command annotations and job step summaries.
package reporting

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jandubois/cooldown/internal/checker"
)

// WriteAnnotations writes GitHub Actions workflow commands for each result.
// Stale dependencies produce ::error annotations; skipped ones produce ::warning.
// These appear in the "Annotations" section of the check run in the PR.
func WriteAnnotations(w io.Writer, results []checker.Result) {
	for _, r := range results {
		switch {
		case r.Stale:
			fmt.Fprintf(w, "::error title=%s is outdated::%s %s is available (proposed %s)\n",
				r.Name, r.Name, r.Latest, r.Proposed)
		case r.Skipped:
			fmt.Fprintf(w, "::warning title=%s skipped::%s\n",
				r.Name, r.Reason)
		}
	}
}

// WriteJobSummary writes a markdown table to GITHUB_STEP_SUMMARY if the
// environment variable is set. Outside GitHub Actions this is a no-op.
func WriteJobSummary(results []checker.Result) error {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening step summary file: %w", err)
	}
	defer f.Close()

	return writeSummaryMarkdown(f, results)
}

func writeSummaryMarkdown(w io.Writer, results []checker.Result) error {
	var b strings.Builder

	stale := hasStale(results)
	if stale {
		b.WriteString("### Cooldown: newer versions available\n\n")
	} else {
		b.WriteString("### Cooldown: all dependencies are current\n\n")
	}

	b.WriteString("| Dependency | Ecosystem | Proposed | Latest | Status |\n")
	b.WriteString("|---|---|---|---|---|\n")

	for _, r := range results {
		switch {
		case r.Stale:
			fmt.Fprintf(&b, "| %s | %s | %s | **%s** | :x: outdated |\n",
				r.Name, r.Ecosystem, r.Proposed, r.Latest)
		case r.Skipped:
			fmt.Fprintf(&b, "| %s | %s | | | :warning: %s |\n",
				r.Name, r.Ecosystem, r.Reason)
		default:
			fmt.Fprintf(&b, "| %s | %s | %s | %s | :white_check_mark: |\n",
				r.Name, r.Ecosystem, r.Proposed, r.Latest)
		}
	}

	b.WriteString("\n")

	_, err := io.WriteString(w, b.String())
	return err
}

func hasStale(results []checker.Result) bool {
	for _, r := range results {
		if r.Stale {
			return true
		}
	}
	return false
}
