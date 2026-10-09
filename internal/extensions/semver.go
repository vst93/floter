package extensions

import (
	"errors"
	"strconv"
	"strings"
)

// Semver is a parsed semantic version: the subset the installer needs —
// major.minor.patch, an optional prerelease and build metadata.
type Semver struct {
	Major, Minor, Patch int
	Prerelease          string
	Build               string
	// Original is the text as it appeared, which is what a record keeps.
	Original string
}

// IsZero reports whether the version is the zero value.
func (v Semver) IsZero() bool {
	return v.Original == "" && v.Major == 0 && v.Minor == 0 && v.Patch == 0
}

// String renders the version as it was written.
func (v Semver) String() string {
	if v.Original != "" {
		return v.Original
	}
	out := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
	if v.Prerelease != "" {
		out += "-" + v.Prerelease
	}
	if v.Build != "" {
		out += "+" + v.Build
	}
	return out
}

// ParseSemver parses a version, tolerating a leading "v".
func ParseSemver(text string) (Semver, error) {
	original := strings.TrimSpace(text)
	trimmed := strings.TrimPrefix(original, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	if trimmed == "" {
		return Semver{}, errors.New("semver: empty version")
	}
	version := Semver{Original: original}

	if index := strings.IndexByte(trimmed, '+'); index >= 0 {
		version.Build = trimmed[index+1:]
		trimmed = trimmed[:index]
	}
	if index := strings.IndexByte(trimmed, '-'); index >= 0 {
		version.Prerelease = trimmed[index+1:]
		trimmed = trimmed[:index]
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) > 3 {
		return Semver{}, errors.New("semver: too many components in " + original)
	}
	numbers := [3]int{}
	for i, part := range parts {
		if part == "" {
			return Semver{}, errors.New("semver: empty component in " + original)
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return Semver{}, errors.New("semver: bad component " + part)
		}
		numbers[i] = value
	}
	version.Major, version.Minor, version.Patch = numbers[0], numbers[1], numbers[2]
	return version, nil
}

// Compare orders two versions, prereleases before their release.
func (v Semver) Compare(other Semver) int {
	for _, pair := range [][2]int{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case v.Prerelease == other.Prerelease:
		return 0
	case v.Prerelease == "":
		return 1 // a release is greater than any prerelease
	case other.Prerelease == "":
		return -1
	}
	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// comparePrerelease orders prerelease identifiers, numerically where both
// are numbers and lexically otherwise, as the specification says.
func comparePrerelease(left, right string) int {
	a, b := strings.Split(left, "."), strings.Split(right, ".")
	for i := 0; i < len(a) && i < len(b); i++ {
		an, aerr := strconv.Atoi(a[i])
		bn, berr := strconv.Atoi(b[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aerr == nil:
			return -1 // numeric identifiers are lower than alphanumeric
		case berr == nil:
			return 1
		default:
			if a[i] != b[i] {
				if a[i] < b[i] {
					return -1
				}
				return 1
			}
		}
	}
	switch {
	case len(a) == len(b):
		return 0
	case len(a) < len(b):
		return -1
	default:
		return 1
	}
}

// Satisfies reports whether a version is in a range: whitespace separates
// comparators that must all hold (an intersection), and "||" separates
// alternatives.
func Satisfies(version Semver, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" || strings.EqualFold(constraint, "latest") {
		return true
	}
	for _, alternative := range strings.Split(constraint, "||") {
		if satisfiesAll(version, alternative) {
			return true
		}
	}
	return false
}

func satisfiesAll(version Semver, alternative string) bool {
	fields := strings.Fields(strings.ReplaceAll(alternative, ",", " "))
	if len(fields) == 0 {
		return true
	}
	for _, field := range fields {
		if !satisfiesComparator(version, field) {
			return false
		}
	}
	return true
}

func satisfiesComparator(version Semver, comparator string) bool {
	comparator = strings.TrimSpace(comparator)
	if comparator == "" || comparator == "*" || comparator == "x" {
		return true
	}
	switch {
	case strings.HasPrefix(comparator, "^"):
		base, err := ParseSemver(comparator[1:])
		if err != nil {
			return false
		}
		return version.Compare(base) >= 0 && version.Compare(caretUpper(base)) < 0
	case strings.HasPrefix(comparator, "~"):
		base, err := ParseSemver(comparator[1:])
		if err != nil {
			return false
		}
		return version.Compare(base) >= 0 && version.Compare(tildeUpper(base)) < 0
	case strings.HasPrefix(comparator, ">="):
		base, err := ParseSemver(comparator[2:])
		return err == nil && version.Compare(base) >= 0
	case strings.HasPrefix(comparator, "<="):
		base, err := ParseSemver(comparator[2:])
		return err == nil && version.Compare(base) <= 0
	case strings.HasPrefix(comparator, ">"):
		base, err := ParseSemver(comparator[1:])
		return err == nil && version.Compare(base) > 0
	case strings.HasPrefix(comparator, "<"):
		base, err := ParseSemver(comparator[1:])
		return err == nil && version.Compare(base) < 0
	case strings.HasPrefix(comparator, "="):
		base, err := ParseSemver(comparator[1:])
		return err == nil && version.Compare(base) == 0
	default:
		base, err := ParseSemver(comparator)
		return err == nil && version.Compare(base) == 0
	}
}

// caretUpper is the exclusive bound of a caret range: leftmost non-zero part
// bumped, as npm's ^.
func caretUpper(base Semver) Semver {
	upper := Semver{Major: base.Major, Minor: base.Minor, Patch: base.Patch}
	switch {
	case base.Major > 0:
		upper.Major, upper.Minor, upper.Patch = base.Major+1, 0, 0
	case base.Minor > 0:
		upper.Minor, upper.Patch = base.Minor+1, 0
	default:
		upper.Patch = base.Patch + 1
	}
	return upper
}

// tildeUpper is the exclusive bound of a tilde range: the minor bumped, as
// npm's ~.
func tildeUpper(base Semver) Semver {
	return Semver{Major: base.Major, Minor: base.Minor + 1, Patch: 0}
}

// SelectVersion picks the highest version satisfying a constraint: a
// dist-tag resolves first (latest, beta), then a range among the versions
// that parse.
func SelectVersion(versions []string, distTags map[string]string, constraint string) (string, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		constraint = "latest"
	}
	if tagged, ok := distTags[constraint]; ok {
		return tagged, nil
	}

	// npm's rule, kept simple: a range matches stable versions only. A
	// prerelease is selected when the constraint itself names one (or a
	// dist-tag points at it, handled above), which is what keeps
	// `^1.0.0` from quietly installing `2.0.0-beta.1`.
	allowPrerelease := strings.Contains(constraint, "-")

	var best Semver
	for _, text := range versions {
		version, err := ParseSemver(text)
		if err != nil {
			continue // a dist-tag or a junk entry in the packument
		}
		if version.Prerelease != "" && !allowPrerelease {
			continue
		}
		if !Satisfies(version, constraint) {
			continue
		}
		if best.IsZero() || version.Compare(best) > 0 {
			best = version
		}
	}
	if best.IsZero() {
		return "", errors.New("extensions: no version satisfies " + constraint)
	}
	return best.Original, nil
}
