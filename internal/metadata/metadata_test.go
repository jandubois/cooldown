package metadata

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("single dependency", func(t *testing.T) {
		input := `[{"dependency-name":"lodash","dependency-type":"direct:production","update-type":"version-update:semver-patch","package-ecosystem":"npm_and_yarn","directory":"/","new-version":"4.17.21","previous-version":"4.17.20","dependency-group":""}]`

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
			{"dependency-name":"actions/checkout","dependency-type":"direct:production","update-type":"version-update:semver-major","package-ecosystem":"github_actions","directory":"/","new-version":"4.1.2","previous-version":"3.6.0","dependency-group":"actions"},
			{"dependency-name":"actions/setup-go","dependency-type":"direct:production","update-type":"version-update:semver-minor","package-ecosystem":"github_actions","directory":"/","new-version":"5.1.0","previous-version":"5.0.0","dependency-group":"actions"}
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
