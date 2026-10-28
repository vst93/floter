// Package versioning decides which version the next release carries.
//
// The release workflow used to answer that question from `mygo.json` alone:
// take its patch, add one, publish. But the branch's `mygo.json` is *not* the
// last release — the workflow writes the version into its own workspace and
// never commits it back (see `.github/workflows/release-mygo.yml`, 「版本号只写在
// 这次构建的工作区里」). So the file sat at 0.3.24 across a week of releases,
// and a dispatch that ran without typing a version computed 0.3.25 — a version
// that was not only already published, but one users had already installed.
// `gh release upload --clobber` would then have replaced that release's assets
// with newer binaries: the same version, different bytes. The updater compares
// versions, so every one of those users would have been told they were current
// and would never have received the fix. The 「修好的东西发不出去」 family the
// release notes already carry twice (prerelease-0.3.19, BSD-sed on macOS), in
// its third shape: correct build, silent version collision.
//
// So the next version is a function of what actually exists — the release tags
// on the remote — with the branch's own file only ever able to raise the floor
// (a hand-edited jump, a rebase onto a newer base), never to lower it.
//
// The package is pure: no git, no HTTP, no clock. The caller hands in the tags
// it read and gets back the version to build. That is what lets the rule be
// pinned by tests on all three platforms instead of by a workflow run.
package versioning

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"floter/internal/semver"
)

// Version is a semantic version's numeric core plus its prerelease suffix.
//
// Build metadata (`+sha.1234`) is dropped: it takes no part in ordering here,
// and a tag that carries it names the same release.
type Version struct {
	Major int
	Minor int
	Patch int
	// Pre is the prerelease suffix *without* the leading dash ("preview.1"),
	// empty for a final release.
	Pre string
}

// String is the version as a tag would spell it.
func (v Version) String() string {
	base := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre == "" {
		return base
	}
	return base + "-" + v.Pre
}

// Base is the version without its prerelease suffix.
func (v Version) Base() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Less orders two versions by their numeric core, then by prerelease.
//
// The rule is semantic versioning 2.0.0's and it lives in `internal/semver`
// (P11-u), because this package used to answer the same question differently
// from `internal/extensions`: the prerelease suffixes were compared as strings,
// so `0.3.26-preview.10` came out *older* than `0.3.26-preview.9` here and newer
// there. Only the core decides what `Next` computes, so the wrong answer never
// reached a release — but two answers to "which version is newer" is the same
// family of bug this package exists to prevent.
func (v Version) Less(other Version) bool {
	return semver.Compare(v.semver(), other.semver()) < 0
}

// semver is the ordering view of a version: the numeric core and the
// prerelease suffix, which are all `internal/semver` reads.
func (v Version) semver() semver.Version {
	return semver.Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch, Prerelease: v.Pre}
}

// Parse reads a version out of the spellings the repository actually contains:
// `0.3.25` (a release's number), `mygo-v0.3.25` (this branch's tags),
// `v0.3.25`, and prereleases of any of them (`mygo-v0.3.26-preview.1`).
//
// The accepted grammar is the one the workflow validates against (semantic
// versioning 2.0.0's core and prerelease, without build metadata), so a value
// this rejects is one the release step would have rejected too.
func Parse(s string) (Version, error) {
	// `^{}` is what `git ls-remote` appends to an annotated tag's peeled ref.
	text := strings.TrimSuffix(strings.TrimSpace(s), "^{}")
	text = strings.TrimPrefix(text, "refs/tags/")
	if rest, ok := strings.CutPrefix(text, "mygo-v"); ok {
		text = rest
	} else {
		// A bare `v` is the Tauri-era tag spelling; anything else is taken as
		// written (a release number, or a manifest's own value).
		text = strings.TrimPrefix(text, "v")
	}
	if text == "" {
		return Version{}, errors.New("versioning: empty version")
	}
	// Build metadata takes no part in the decision.
	if plus := strings.IndexByte(text, '+'); plus >= 0 {
		text = text[:plus]
	}
	core := text
	if dash := strings.IndexByte(text, '-'); dash >= 0 {
		core, text = text[:dash], text[dash+1:]
		if text == "" {
			return Version{}, fmt.Errorf("versioning: %q has an empty prerelease", s)
		}
		for _, ident := range strings.Split(text, ".") {
			if ident == "" {
				return Version{}, fmt.Errorf("versioning: %q has an empty prerelease identifier", s)
			}
			if !prereleaseIdent(ident) {
				return Version{}, fmt.Errorf("versioning: %q has an invalid prerelease identifier %q", s, ident)
			}
		}
	} else {
		text = ""
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("versioning: %q is not major.minor.patch", s)
	}
	numbers := [3]int{}
	for i, part := range parts {
		// `0|[1-9][0-9]*`: no leading zeros, no signs, no empty field.
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return Version{}, fmt.Errorf("versioning: %q has an invalid numeric field %q", s, part)
		}
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return Version{}, fmt.Errorf("versioning: %q has an invalid numeric field %q", s, part)
			}
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("versioning: %q has an invalid numeric field %q", s, part)
		}
		numbers[i] = value
	}
	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2], Pre: text}, nil
}

// prereleaseIdent is semantic versioning's identifier rule: alphanumerics and
// hyphens, non-empty.
func prereleaseIdent(ident string) bool {
	for _, r := range ident {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '-':
		default:
			return false
		}
	}
	return ident != ""
}

// Next is the version the next release should carry, given the branch's
// `mygo.json` value and every version that already exists (release tags, from
// `git ls-remote --tags origin 'mygo-v*'` and the local ones alike).
//
// The rule, in the order it applies:
//
//   - the floor is the higher of `mygo.json`'s core and the highest existing
//     core, so a stale file can never name a taken version and a hand-edited
//     jump is honoured;
//   - the candidate is that floor's patch + 1 (the branch has released within a
//     minor line for its whole history, so a patch bump is what a release is);
//   - the candidate is skipped while *any* existing version shares its core —
//     a preview of 0.3.26 makes 0.3.26 ambiguous for the updater, which is the
//     whole failure this package exists to prevent.
//
// Unparsable entries are ignored rather than fatal: a stray tag on the remote
// (a typo, a `mygo-vnext`) must not block a release. An unparsable *current*
// value is fatal, because that is a branch state the release cannot derive from.
func Next(current string, existing []string) (string, error) {
	cur, err := Parse(current)
	if err != nil {
		return "", fmt.Errorf("versioning: mygo.json: %w", err)
	}
	taken := make(map[string]bool, len(existing))
	floor := cur
	for _, raw := range existing {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		version, err := Parse(raw)
		if err != nil {
			continue
		}
		taken[version.Base()] = true
		if floor.Less(version) {
			floor = version
		}
	}
	next := Version{Major: floor.Major, Minor: floor.Minor, Patch: floor.Patch + 1}
	for taken[next.Base()] {
		next.Patch++
	}
	// A candidate that has run past this minor line is still the right answer:
	// `taken` only holds real releases, so the loop above ends within the
	// number of releases that exist.
	return next.Base(), nil
}
