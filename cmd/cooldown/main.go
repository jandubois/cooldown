// Command cooldown checks whether a Dependabot PR proposes the latest
// available version for each dependency. It exits non-zero if any
// dependency has a newer version within the same major line.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jandubois/cooldown/internal/checker"
	"github.com/jandubois/cooldown/internal/metadata"
	"github.com/jandubois/cooldown/internal/reporting"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("cooldown: ")

	if len(os.Args) < 2 || os.Args[1] != "check" {
		fmt.Fprintf(os.Stderr, "Usage: cooldown check\n")
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  DEPENDENCIES_JSON  JSON from dependabot/fetch-metadata (required)\n")
		fmt.Fprintf(os.Stderr, "  GITHUB_TOKEN       GitHub token for API rate limits (optional)\n")
		os.Exit(2)
	}

	os.Exit(run())
}

func run() int {
	depsJSON := os.Getenv("DEPENDENCIES_JSON")
	if depsJSON == "" {
		log.Println("DEPENDENCIES_JSON is empty; nothing to check")
		return 0
	}

	deps, err := metadata.Parse(depsJSON)
	if err != nil {
		log.Printf("failed to parse dependencies: %v", err)
		return 2
	}

	if len(deps) == 0 {
		log.Println("no dependencies to check")
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c := &checker.Checker{
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
	}

	results := c.Check(ctx, deps)

	// Print summary to stdout
	fmt.Println()
	for _, r := range results {
		switch {
		case r.Skipped:
			fmt.Printf("  SKIP  %s (%s)\n", r.Name, r.Reason)
		case r.Stale:
			fmt.Printf("  STALE %s: proposed %s, latest %s\n", r.Name, r.Proposed, r.Latest)
		default:
			fmt.Printf("  OK    %s @ %s\n", r.Name, r.Proposed)
		}
	}
	fmt.Println()

	// GitHub Actions annotations (::error, ::warning)
	reporting.WriteAnnotations(os.Stdout, results)

	// GitHub Actions job summary (markdown table)
	if err := reporting.WriteJobSummary(results); err != nil {
		log.Printf("failed to write job summary: %v", err)
	}

	if checker.HasStale(results) {
		log.Println("one or more dependencies are not at the latest version")
		return 1
	}

	log.Println("all dependencies are at the latest version")
	return 0
}
