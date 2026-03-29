package version

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input      string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantErr    bool
	}{
		{"1.2.3", 1, 2, 3, "", false},
		{"v1.2.3", 1, 2, 3, "", false},
		{"0.0.0", 0, 0, 0, "", false},
		{"v4.1.2", 4, 1, 2, "", false},
		{"1.0.0-alpha", 1, 0, 0, "alpha", false},
		{"v2.0.0-rc.1", 2, 0, 0, "rc.1", false},
		{"1.0.0-beta.2+build.123", 1, 0, 0, "beta.2", false},
		{"3.2.0+incompatible", 3, 2, 0, "", false},
		{"1.0", 1, 0, 0, "", false},         // two-part version (e.g., GitHub Actions)
		{"v4", 0, 0, 0, "", true},           // single part is invalid
		{"notaversion", 0, 0, 0, "", true},
		{"v1.x.3", 0, 0, 0, "", true},
		{"", 0, 0, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want error", tt.input, v)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.input, err)
			}
			if v.Major != tt.wantMajor || v.Minor != tt.wantMinor || v.Patch != tt.wantPatch {
				t.Errorf("Parse(%q) = %d.%d.%d, want %d.%d.%d",
					tt.input, v.Major, v.Minor, v.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
			if v.Prerelease != tt.wantPre {
				t.Errorf("Parse(%q).Prerelease = %q, want %q", tt.input, v.Prerelease, tt.wantPre)
			}
			if v.Raw != tt.input {
				t.Errorf("Parse(%q).Raw = %q, want %q", tt.input, v.Raw, tt.input)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		// Prerelease sorts before release
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		// Prerelease ordering
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1},
		{"1.0.0-1", "1.0.0-2", -1},
		// Numeric sorts before string
		{"1.0.0-1", "1.0.0-alpha", -1},
		// Fewer fields sorts before more fields (when equal prefix)
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a := mustParse(t, tt.a)
			b := mustParse(t, tt.b)
			got := a.Compare(b)
			if got != tt.want {
				t.Errorf("(%s).Compare(%s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLatestForMajor(t *testing.T) {
	tests := []struct {
		name      string
		proposed  string
		available []string
		wantLatest string
		wantNewer  bool
	}{
		{
			name:       "proposed is latest",
			proposed:   "1.5.0",
			available:  []string{"1.3.0", "1.4.0", "1.5.0"},
			wantLatest: "1.5.0",
			wantNewer:  false,
		},
		{
			name:       "newer patch exists",
			proposed:   "1.5.0",
			available:  []string{"1.5.0", "1.5.1", "1.5.2"},
			wantLatest: "1.5.2",
			wantNewer:  true,
		},
		{
			name:       "newer minor exists",
			proposed:   "1.5.0",
			available:  []string{"1.5.0", "1.6.0"},
			wantLatest: "1.6.0",
			wantNewer:  true,
		},
		{
			name:       "only v2 is newer -- should not flag",
			proposed:   "1.5.0",
			available:  []string{"1.3.0", "1.5.0", "2.0.0"},
			wantLatest: "1.5.0",
			wantNewer:  false,
		},
		{
			name:       "skip prereleases",
			proposed:   "1.5.0",
			available:  []string{"1.5.0", "1.6.0-rc.1"},
			wantLatest: "1.5.0",
			wantNewer:  false,
		},
		{
			name:       "empty available list",
			proposed:   "1.5.0",
			available:  nil,
			wantLatest: "1.5.0",
			wantNewer:  false,
		},
		{
			name:       "no versions in same major",
			proposed:   "1.5.0",
			available:  []string{"2.0.0", "3.0.0"},
			wantLatest: "1.5.0",
			wantNewer:  false,
		},
		{
			name:       "proposed is prerelease and stable exists",
			proposed:   "1.5.0-beta",
			available:  []string{"1.5.0"},
			wantLatest: "1.5.0",
			wantNewer:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposed := mustParse(t, tt.proposed)
			var available []Version
			for _, s := range tt.available {
				available = append(available, mustParse(t, s))
			}

			latest, hasNewer := LatestForMajor(proposed, available)
			if latest.String() != tt.wantLatest {
				t.Errorf("LatestForMajor(%s, ...) latest = %s, want %s",
					tt.proposed, latest, tt.wantLatest)
			}
			if hasNewer != tt.wantNewer {
				t.Errorf("LatestForMajor(%s, ...) hasNewer = %v, want %v",
					tt.proposed, hasNewer, tt.wantNewer)
			}
		})
	}
}

func mustParse(t *testing.T, s string) Version {
	t.Helper()
	v, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", s, err)
	}
	return v
}
