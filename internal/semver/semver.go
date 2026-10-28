// Package semver answers one question: of two version numbers, which is the
// newer? It is the one answer to that question in this repository.
//
// The question used to have two implementations. `internal/extensions` ordered
// a manifest's version the way semantic versioning 2.0.0 says (numeric
// prerelease identifiers compare as numbers), and `internal/versioning`
// ordered a release tag's version by comparing the prerelease suffix as a
// string — so the same pair came out both ways depending on which package was
// asked: `1.2.3-preview.9` was newer than `1.2.3-preview.10` to one of them and
// older to the other. `internal/versioning` exists precisely because a version
// number with two meanings already cost this project a release (see its
// package comment); its own ordering rule must not be the second meaning.
//
// What stays out of here is *parsing*. A release tag (`mygo-v0.3.25`, with a
// `refs/tags/` prefix and a peeled `^{}`) and an extension manifest's version
// (`1.0`, two components, tolerated) are two grammars for two different
// documents, and each package keeps its own. Only the ordering is shared —
// which is why this package takes a `Version` and never reads a string.
package semver

import (
	"strconv"
	"strings"
)

// Version is a version's numeric core plus its prerelease suffix. Build
// metadata (`+sha.1234`) takes no part in ordering, so it has no field here.
type Version struct {
	Major int
	Minor int
	Patch int
	// Prerelease is the suffix without the leading dash ("preview.1"), empty
	// for a final release.
	Prerelease string
}

// Compare returns -1 when a is older than b, 0 when they are the same version,
// and 1 when a is newer.
func Compare(a, b Version) int {
	for _, pair := range [][2]int{
		{a.Major, b.Major},
		{a.Minor, b.Minor},
		{a.Patch, b.Patch},
	} {
		switch {
		case pair[0] < pair[1]:
			return -1
		case pair[0] > pair[1]:
			return 1
		}
	}
	return ComparePrerelease(a.Prerelease, b.Prerelease)
}

// ComparePrerelease orders two prerelease suffixes, empty meaning "this is the
// final release" — which is newer than any prerelease of the same core.
//
// The rules are semantic versioning 2.0.0's, clause 11: identifiers are split
// on `.`; two numeric identifiers compare as numbers; a numeric identifier is
// older than an alphanumeric one; two alphanumeric identifiers compare
// byte-wise; and a longer suffix whose prefix is equal is newer
// (`preview.1` < `preview.1.1`).
func ComparePrerelease(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1 // a release is greater than any prerelease
	case b == "":
		return -1
	}
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) && i < len(right); i++ {
		ln, lerr := strconv.Atoi(left[i])
		rn, rerr := strconv.Atoi(right[i])
		switch {
		case lerr == nil && rerr == nil:
			switch {
			case ln < rn:
				return -1
			case ln > rn:
				return 1
			}
		case lerr == nil:
			return -1 // numeric identifiers are lower than alphanumeric
		case rerr == nil:
			return 1
		default:
			switch {
			case left[i] < right[i]:
				return -1
			case left[i] > right[i]:
				return 1
			}
		}
	}
	switch {
	case len(left) == len(right):
		return 0
	case len(left) < len(right):
		return -1
	default:
		return 1
	}
}
