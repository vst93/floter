package semver

import (
	"strconv"
	"strings"
	"testing"
)

// The ordering is semantic versioning 2.0.0's, so the table is the
// specification's own examples plus the one pair the two old implementations
// disagreed on (`preview.9` vs `preview.10`).
func TestCompareOrdersVersionsTheWayTheSpecificationSays(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// The core decides first.
		{"1.0.0", "2.0.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"1.0.10", "1.0.9", 1},
		{"1.2.3", "1.2.3", 0},
		// A prerelease is older than its release.
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		// Numeric identifiers compare as numbers, not as text: this is the
		// pair the lexical rule got backwards.
		{"1.0.0-preview.9", "1.0.0-preview.10", -1},
		{"1.0.0-preview.10", "1.0.0-preview.9", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		// Numeric identifiers are older than alphanumeric ones.
		{"1.0.0-1", "1.0.0-alpha", -1},
		{"1.0.0-alpha", "1.0.0-1", 1},
		// Alphanumeric identifiers compare byte-wise.
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		// A longer suffix with an equal prefix is newer.
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		// The core outranks the suffix: a newer core wins even against a
		// final release of the older one.
		{"0.3.27-preview.1", "0.3.26", 1},
	}
	for _, c := range cases {
		a, b := version(t, c.a), version(t, c.b)
		if got := Compare(a, b); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Compare(b, a); got != -c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d (the other direction)", c.b, c.a, got, -c.want)
		}
	}
}

// Compare is a total order: a version is never older than itself, and two
// answers about the same pair always mirror each other.
func TestCompareIsATotalOrder(t *testing.T) {
	versions := []Version{
		{},
		{Major: 1},
		{Major: 1, Prerelease: "alpha"},
		{Major: 1, Prerelease: "alpha.1"},
		{Major: 1, Prerelease: "1"},
		{Minor: 2},
	}
	for _, a := range versions {
		if got := Compare(a, a); got != 0 {
			t.Errorf("Compare(%+v, itself) = %d, want 0", a, got)
		}
		for _, b := range versions {
			if got, back := Compare(a, b), Compare(b, a); got != -back {
				t.Errorf("Compare(%+v, %+v) = %d but the reverse is %d", a, b, got, back)
			}
		}
	}
}

// version reads the `major.minor.patch[-prerelease]` spellings the table uses,
// so a typo in a fixture is a test failure rather than a silently-zero version.
func version(t *testing.T, text string) Version {
	t.Helper()
	core, prerelease, _ := strings.Cut(text, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		t.Fatalf("fixture %q is not major.minor.patch", text)
	}
	numbers := [3]int{}
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			t.Fatalf("fixture %q has a non-numeric core field: %v", text, err)
		}
		numbers[i] = value
	}
	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2], Prerelease: prerelease}
}
