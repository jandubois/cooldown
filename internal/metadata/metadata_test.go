package metadata

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("single dependency", func(t *testing.T) {
		// Matches the actual JSON format from dependabot/fetch-metadata
		input := `[{"dependencyName":"lodash","dependencyType":"direct:production","updateType":"version-update:semver-patch","packageEcosystem":"npm_and_yarn","directory":"/","newVersion":"4.17.21","prevVersion":"4.17.20","dependencyGroup":""}]`

		deps, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if len(deps) != 1 {
			t.Fatalf("got %d deps, want 1", len(deps))
		}

		d := deps[0]
		if d.Name != "lodash" {
			t.Errorf("Name = %q, want %q", d.Name, "lodash")
		}
		if d.Ecosystem != "npm_and_yarn" {
			t.Errorf("Ecosystem = %q, want %q", d.Ecosystem, "npm_and_yarn")
		}
		if d.NewVersion != "4.17.21" {
			t.Errorf("NewVersion = %q, want %q", d.NewVersion, "4.17.21")
		}
		if d.PrevVersion != "4.17.20" {
			t.Errorf("PrevVersion = %q, want %q", d.PrevVersion, "4.17.20")
		}
	})

	t.Run("grouped dependencies", func(t *testing.T) {
		input := `[
			{"dependencyName":"actions/checkout","dependencyType":"direct:production","updateType":"version-update:semver-major","packageEcosystem":"github_actions","directory":"/","newVersion":"4.1.2","prevVersion":"3.6.0","dependencyGroup":"actions"},
			{"dependencyName":"actions/setup-go","dependencyType":"direct:production","updateType":"version-update:semver-minor","packageEcosystem":"github_actions","directory":"/","newVersion":"5.1.0","prevVersion":"5.0.0","dependencyGroup":"actions"}
		]`

		deps, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if len(deps) != 2 {
			t.Fatalf("got %d deps, want 2", len(deps))
		}
		if deps[0].Name != "actions/checkout" {
			t.Errorf("deps[0].Name = %q, want %q", deps[0].Name, "actions/checkout")
		}
		if deps[1].Name != "actions/setup-go" {
			t.Errorf("deps[1].Name = %q, want %q", deps[1].Name, "actions/setup-go")
		}
		if deps[0].Group != "actions" {
			t.Errorf("deps[0].Group = %q, want %q", deps[0].Group, "actions")
		}
	})

	t.Run("real fetch-metadata output", func(t *testing.T) {
		// Exact JSON from a real Dependabot PR captured from CI logs
		input := `[{"dependencyName":"actions/setup-go","dependencyType":"direct:production","updateType":"version-update:semver-major","directory":"/","packageEcosystem":"github_actions","targetBranch":"main","prevVersion":"5.6.0","newVersion":"6.3.0","compatScore":0,"maintainerChanges":false,"dependencyGroup":"","alertState":"","ghsaId":"","cvss":0}]`

		deps, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if len(deps) != 1 {
			t.Fatalf("got %d deps, want 1", len(deps))
		}

		d := deps[0]
		if d.Name != "actions/setup-go" {
			t.Errorf("Name = %q, want %q", d.Name, "actions/setup-go")
		}
		if d.Ecosystem != "github_actions" {
			t.Errorf("Ecosystem = %q, want %q", d.Ecosystem, "github_actions")
		}
		if d.NewVersion != "6.3.0" {
			t.Errorf("NewVersion = %q, want %q", d.NewVersion, "6.3.0")
		}
		if d.PrevVersion != "5.6.0" {
			t.Errorf("PrevVersion = %q, want %q", d.PrevVersion, "5.6.0")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		deps, err := Parse("")
		if err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if len(deps) != 0 {
			t.Fatalf("got %d deps, want 0", len(deps))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := Parse("{not valid")
		if err == nil {
			t.Fatal("Parse() expected error for invalid JSON")
		}
	})
}
