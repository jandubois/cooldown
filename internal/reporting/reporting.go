// Package reporting formats checker results for GitHub Actions output:
// workflow command annotations and job step summaries.
package reporting

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jandubois/cooldown/internal/checker"
	"github.com/jandubois/cooldown/internal/registry"
)

// WriteAnnotations writes GitHub Actions workflow commands for each result.
// Stale dependencies produce ::error annotations; skipped ones produce ::warning.
// These appear in the "Annotations" section of the check run in the PR.
func WriteAnnotations(w io.Writer, results []checker.Result) {
	for _, r := range results {
		switch {
		case r.Stale:
			msg := fmt.Sprintf("%s %s is available (proposed %s)", r.Name, r.Latest, r.Proposed)
			if r.Changelog != nil && r.Changelog.CompareURL != "" {
				msg += " — " + r.Changelog.CompareURL
			}
			fmt.Fprintf(w, "::error title=%s is outdated::%s\n", r.Name, msg)
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
			latestCol := formatLatestWithLink(r)
			fmt.Fprintf(&b, "| %s | %s | %s | %s | :x: outdated |\n",
				r.Name, r.Ecosystem, r.Proposed, latestCol)
		case r.Skipped:
			fmt.Fprintf(&b, "| %s | %s | | | :warning: %s |\n",
				r.Name, r.Ecosystem, r.Reason)
		default:
			fmt.Fprintf(&b, "| %s | %s | %s | %s | :white_check_mark: |\n",
				r.Name, r.Ecosystem, r.Proposed, r.Latest)
		}
	}

	b.WriteString("\n")

	// Append changelog sections for each stale dependency
	for _, r := range results {
		if !r.Stale || r.Changelog == nil {
			continue
		}
		writeChangelog(&b, r)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// writeChangelog formats release notes for a stale dependency, matching
// the style Dependabot uses in PR descriptions: release notes in collapsed
// sections, plus a compare link.
func writeChangelog(b *strings.Builder, r checker.Result) {
	cl := r.Changelog

	fmt.Fprintf(b, "---\n\n#### %s: %s → %s\n\n", r.Name, r.Proposed, r.Latest)

	if len(cl.Releases) > 0 {
		writeReleaseNotes(b, cl)
	}

	if cl.CompareURL != "" {
		fmt.Fprintf(b, "[View changes on GitHub](%s)\n\n", cl.CompareURL)
	}
}

func writeReleaseNotes(b *strings.Builder, cl *registry.Changelog) {
	// Single release: show inline. Multiple: each in a collapsed section.
	if len(cl.Releases) == 1 {
		r := cl.Releases[0]
		b.WriteString("<details>\n")
		fmt.Fprintf(b, "<summary>Release notes</summary>\n")
		fmt.Fprintf(b, "<em>Sourced from <a href=\"https://github.com/%s/%s/releases\">%s/%s's releases</a>.</em>\n",
			cl.Owner, cl.Repo, cl.Owner, cl.Repo)
		b.WriteString("<blockquote>\n\n")
		if r.Body != "" {
			b.WriteString(r.Body)
		} else {
			fmt.Fprintf(b, "*No release notes provided for %s.*", r.Tag)
		}
		b.WriteString("\n\n</blockquote>\n</details>\n\n")
		return
	}

	b.WriteString("<details>\n")
	fmt.Fprintf(b, "<summary>Release notes (%d versions)</summary>\n",
		len(cl.Releases))
	fmt.Fprintf(b, "<em>Sourced from <a href=\"https://github.com/%s/%s/releases\">%s/%s's releases</a>.</em>\n",
		cl.Owner, cl.Repo, cl.Owner, cl.Repo)
	b.WriteString("<blockquote>\n\n")

	for _, r := range cl.Releases {
		fmt.Fprintf(b, "**[%s](%s)**\n\n", r.Tag, r.URL)
		if r.Body != "" {
			b.WriteString(r.Body)
		} else {
			fmt.Fprintf(b, "*No release notes provided for %s.*", r.Tag)
		}
		b.WriteString("\n\n")
	}

	b.WriteString("</blockquote>\n</details>\n\n")
}

// formatLatestWithLink returns the latest version as a bold markdown link
// if a compare URL is available, or just bold text otherwise.
func formatLatestWithLink(r checker.Result) string {
	if r.Changelog != nil && r.Changelog.CompareURL != "" {
		return fmt.Sprintf("[**%s**](%s)", r.Latest, r.Changelog.CompareURL)
	}
	return fmt.Sprintf("**%s**", r.Latest)
}

func hasStale(results []checker.Result) bool {
	for _, r := range results {
		if r.Stale {
			return true
		}
	}
	return false
}
