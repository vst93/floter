// Package homepath answers one question: what does a path the user typed or
// dropped mean? A leading `~` is shell syntax, and the launcher is not a shell,
// so every surface that takes a path has to expand it itself — and every
// surface has to give the same answer.
//
// The rule is the reference's `resolve_path`
// (`src-tauri/src/commands/actions.rs:92-105`), which the action bar and the
// open-path action share there:
//
//   - `~` and `~/…` are the home directory. The separators come off before
//     joining, because joining a path that starts with one discards everything
//     to its left and yields the filesystem root.
//   - `~alice` is *not* another user's home: nothing portable can resolve it,
//     and a directory literally called `~alice` is the likelier thing a
//     launcher is being asked about — so it is joined to home rather than
//     treated as a user.
//   - A path that is already absolute is left alone; a relative one is joined
//     to home by Resolve (Expand leaves it relative, because a dropped path
//     resolves against the drop's own directory instead).
//
// The separator test is the platform's own (`os.IsPathSeparator`, the
// reference's `std::path::is_separator`): `~\x` is a home-relative path on
// Windows and a directory literally named `~\x` on POSIX. That is one answer
// per platform, which is what the reference does; it is not this package's job
// to invent a third one.
//
// Before this package there were four implementations of the rule and two
// answers. The action bar expanded `~/x` with `filepath.FromSlash` followed by
// `strings.CutPrefix(expanded, "~/")` — but `FromSlash` had already turned the
// `/` into `\` on Windows, so the cut missed and the row offered
// `C:\Users\<u>\~\x` as the path to open and to copy. The connect link's
// manifest expanded `~alice` to `home/alice`, while the other three sites (and
// the reference) answered `home/~alice`.
package homepath

import (
	"os"
	"path/filepath"
	"strings"
)

// Expand replaces a leading `~` with home. The second result is false when the
// path needs a home directory and there is none: joining against "" would
// quietly turn the value into a working-directory relative path, which is a
// different file.
//
// A path that does not start with `~` is returned unchanged — including a
// relative one, which stays relative so each caller can join it to the base it
// means (home for a typed path, the drop's directory for a dropped one).
func Expand(path, home string) (string, bool) {
	rest, tilde := strings.CutPrefix(path, "~")
	if !tilde {
		return path, true
	}
	if home == "" {
		return "", false
	}
	// `~alice` is a directory called `~alice` under home, not a user's home.
	if rest != "" && !os.IsPathSeparator(rest[0]) {
		return filepath.Join(home, path), true
	}
	// `~`, `~/` and `~//x` all land in the same place: the separators are
	// removed before the join, never handed to it.
	for rest != "" && os.IsPathSeparator(rest[0]) {
		rest = rest[1:]
	}
	if rest == "" {
		return home, true
	}
	return filepath.Join(home, rest), true
}

// Resolve is Expand plus the reference's second half: a path that is still
// relative after the tilde is joined to home. This is the answer a *typed*
// path gets (the action bar's path row, the open-path action, a link printed in
// the terminal); a dropped path wants Expand alone.
func Resolve(path, home string) (string, bool) {
	expanded, ok := Expand(path, home)
	if !ok {
		return "", false
	}
	if filepath.IsAbs(expanded) {
		return expanded, true
	}
	if home == "" {
		return "", false
	}
	return filepath.Join(home, expanded), true
}
