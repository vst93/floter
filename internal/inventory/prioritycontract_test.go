package inventory

import (
	"path/filepath"
	"strings"
	"testing"
)

// The ranking is a sum of eight signals, and the sum is the contract: only the
// ordering it induces is meaningful, but the ordering is produced by these
// numbers (`src-tauri/src/extensions/inventory.rs:324-368`). Asserting the
// values — not just "git beats a52dec" — is what makes a changed coefficient
// fail, which is the mutation this file exists to catch.
//
// Every row states its own arithmetic in the comment.
func TestPriorityArithmetic(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		description string
		sources     []DiscoverySource
		want        int
		why         string
	}{
		{
			name:    "git",
			path:    "/usr/bin/git",
			sources: []DiscoverySource{SourcePath},
			want:    1300,
			why:     "base 1000 + curated 300",
		},
		{
			name:    "a52dec",
			path:    "/usr/bin/a52dec",
			sources: []DiscoverySource{SourcePath},
			want:    1000,
			why:     "base only: short, not curated, no evidence",
		},
		{
			name:    "gcc-12",
			path:    "/usr/bin/gcc-12",
			sources: []DiscoverySource{SourcePath},
			want:    800,
			why:     "base 1000 − variant 200 (a version-looking suffix)",
		},
		{
			name:    "xml2-config",
			path:    "/usr/bin/xml2-config",
			sources: []DiscoverySource{SourcePath},
			want:    796,
			why:     "base 1000 − variant 200 − length 4 (11 chars, one past ten)",
		},
		{
			name:    "x",
			path:    "/usr/bin/x",
			sources: []DiscoverySource{SourcePath},
			want:    900,
			why:     "base 1000 − one-character 100",
		},
		{
			name:    "my-very-long-tool-name",
			path:    "/usr/bin/my-very-long-tool-name",
			sources: []DiscoverySource{SourcePath},
			want:    952,
			why:     "base 1000 − length 48 (22 chars, twelve past ten)",
		},
		{
			name:    "a52dec",
			path:    "/usr/share/applications/a52dec.desktop",
			sources: []DiscoverySource{SourceDesktop},
			want:    1200,
			why:     "base 1000 + desktop 200: the OS published a name for it",
		},
		{
			name:        "a52dec",
			path:        "/usr/bin/a52dec",
			description: "a free ATSC A/52 stream decoder",
			sources:     []DiscoverySource{SourcePath},
			want:        1060,
			why:         "base 1000 + description 60",
		},
		{
			name:        "a52dec",
			path:        "/usr/share/applications/a52dec.desktop",
			description: "a free ATSC A/52 stream decoder",
			sources:     []DiscoverySource{SourceDesktop, SourcePath},
			want:        1260,
			why:         "base 1000 + desktop 200 + description 60: the terms add, they do not replace",
		},
	}
	for _, test := range cases {
		candidate := Candidate{
			Name:        test.name,
			Path:        test.path,
			Description: test.description,
			Sources:     test.sources,
			Quality:     QualityAutoDetected,
		}
		if got := Priority(candidate); got != test.want {
			t.Errorf("Priority(%s at %s) = %d, want %d (%s)", test.name, test.path, got, test.want, test.why)
		}
	}
}

// fixtureHome pins the home directory the user-directory bonus is measured
// against, so the rows below are the same on every machine. `os.UserHomeDir`
// reads `HOME` on unix and `USERPROFILE` on Windows; both are set.
func fixtureHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(string(filepath.Separator), "fixture-home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// The user-owned bonus is +40, and which directories earn it is a table. The
// reference's four come first (`inventory.rs:410-418`); `~/bin` was missing
// from this build until P10-q, which cost a `~/bin` tool 40 points against the
// same tool in `~/.local/bin`.
func TestPriorityUserOwnedDirectories(t *testing.T) {
	home := fixtureHome(t)
	earns := []string{
		filepath.Join(home, ".local", "bin"),                                // reference
		filepath.Join(home, ".cargo", "bin"),                                // reference
		filepath.Join(home, ".nix-profile", "bin"),                          // reference
		filepath.Join(home, "bin"),                                          // reference — P10-q
		filepath.Join(home, "go", "bin"),                                    // this build
		filepath.Join(home, ".local", "share", "flatpak", "exports", "bin"), // this build
	}
	for _, dir := range earns {
		candidate := Candidate{Name: "a52dec", Path: filepath.Join(dir, "a52dec"), Sources: []DiscoverySource{SourcePath}}
		if got := Priority(candidate); got != 1040 {
			t.Errorf("Priority(%s) = %d, want 1040 (base 1000 + user directory 40)", candidate.Path, got)
		}
	}

	// A prefix that is not a directory boundary is not the directory. The
	// reference's bare `starts_with` (`inventory.rs:412`) would credit both of
	// these; this build's separator rule does not.
	earnsNothing := []string{
		filepath.Join(home, ".local", "bin-old", "a52dec"),
		filepath.Join(home, "bin-old", "a52dec"),
		filepath.Join(home, "go", "bin-old", "a52dec"),
		"/usr/bin/a52dec",
	}
	for _, path := range earnsNothing {
		candidate := Candidate{Name: "a52dec", Path: path, Sources: []DiscoverySource{SourcePath}}
		if got := Priority(candidate); got != 1000 {
			t.Errorf("Priority(%s) = %d, want 1000 (no user directory bonus)", path, got)
		}
	}

	// A path that *is* the directory is not inside it.
	self := Candidate{Name: "a52dec", Path: filepath.Join(home, "bin"), Sources: []DiscoverySource{SourcePath}}
	if got := Priority(self); got != 1000 {
		t.Errorf("Priority(%s) = %d, want 1000 (the directory itself is not under itself)", self.Path, got)
	}
}

// The sum is clamped at zero, so a heavily penalised name can never wrap around
// into a high score. Reachable: a 300-character variant name scores
// 1000 − 1160 (length) − 200 (variant) = −360 before the clamp.
func TestPriorityClampsAtZero(t *testing.T) {
	name := "x-" + strings.Repeat("1", 298)
	candidate := Candidate{Name: name, Path: "/usr/bin/" + name, Sources: []DiscoverySource{SourcePath}}
	if got := Priority(candidate); got != 0 {
		t.Errorf("Priority(%d-character variant) = %d, want 0 (clamped)", len(name), got)
	}
	for _, name := range []string{"x", "x-1", "x-config", "aclocal-1.18", ""} {
		candidate := Candidate{Name: name, Path: "/usr/bin/" + name, Sources: []DiscoverySource{SourcePath}}
		if got := Priority(candidate); got < 0 {
			t.Errorf("Priority(%q) = %d, want non-negative", name, got)
		}
	}
}

// The ordering is total and in one place: quality first (the existing
// contract — a `native-support` entry outranks a bare PATH hit even when the
// PATH hit is curated), then priority descending, then the name so equal rows
// have a stable order (`inventory.rs:377-382`).
func TestRankedOrdersByQualityThenPriorityThenName(t *testing.T) {
	byPath := map[string]*Candidate{
		"/usr/bin/zzz": {ID: "/usr/bin/zzz", Name: "zzz", Path: "/usr/bin/zzz", Quality: QualityAutoDetected, Sources: []DiscoverySource{SourcePath}},
		"/usr/bin/aaa": {ID: "/usr/bin/aaa", Name: "aaa", Path: "/usr/bin/aaa", Quality: QualityAutoDetected, Sources: []DiscoverySource{SourcePath}},
		"/usr/bin/git": {ID: "/usr/bin/git", Name: "git", Path: "/usr/bin/git", Quality: QualityAutoDetected, Sources: []DiscoverySource{SourcePath}},
		"/opt/yyy":     {ID: "/opt/yyy", Name: "yyy", Path: "/opt/yyy", Quality: QualityNativeSupport, Sources: []DiscoverySource{SourceNix}},
	}
	var got []string
	for _, candidate := range ranked(byPath) {
		got = append(got, candidate.Name)
	}
	want := []string{"yyy", "git", "aaa", "zzz"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ranked order = %v, want %v (quality, then priority, then name)", got, want)
	}
}
