package shell

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// linkManifestValue turns a `connect` link's `manifest` value into the path the
// connect flow stats, or reports that the value cannot name a manifest source
// at all. It is the one answer to "is this a manifest value": the router calls
// it before the page opens (`system.go`), and `connectFromLink` receives its
// result rather than re-reading the query.
//
// The reference's `validate_manifest_source` (`main:src-tauri/src/deep_link.rs:288-338`)
// is the source of every rule below; each one is adopted or diverged on purpose:
//
//  1. Empty (after trimming) is refused — `Reject::MissingManifest` (`:290-292`).
//  2. A control character is refused — `ManifestInsecure` (`:296-299`), because a
//     NUL would truncate the value the moment it reached the filesystem, and
//     every other control character is a value nobody typed on purpose.
//  3. An explicit scheme is refused — `ManifestInsecure` (`:301-317`). The
//     reference accepts `https` there (a remote manifest it downloads and
//     reviews) and refuses every other transport. **Divergence**: this build has
//     no remote manifest install at all — `extensions.PrepareLocal`
//     (`internal/extensions/install.go:68`) is the connect flow's only entry and
//     it takes a package *directory*, while the npm path installs by name and
//     version constraint (`PrepareRegistry`, `install.go:83`), never by URL. So a
//     URL has nothing to land on here: it is refused before the page opens
//     instead of being carried to a `stat` that could only fail. Pinned in
//     `linkmanifest_test.go`.
//  4. `~` and a home-relative path are accepted — this build's own documented
//     convenience (`docs/mygo-rewrite-plan.md:690-695`: "支持 `~`，相对路径按 home
//     解析"), and the terminal spelling `floter connect ./tool.json`
//     (`linkargs.go:78-82`) folds to this same value. **Divergence**: the
//     reference refuses every relative path (`ManifestNotAbsolute`, `:323-326`)
//     because its link arrives from a web page; this build's link also arrives
//     from the user's own command line. Applying the reference rule here would
//     delete a documented feature, so it is pinned instead.
//  5. A `..` component is refused — `ManifestTraversal` (`:328-333`). The
//     reference walks `Path::components()`; that is one answer on POSIX and
//     another on Windows (a backslash is a separator on one and a filename
//     character on the other), so this build splits on both separators on every
//     platform: three platforms, one answer. A *filename* that merely starts
//     with two dots (`..notes.json`) is not a traversal — the reference pins that
//     too (`:1597-1600`).
//
// What is deliberately not here: the `.json` suffix (`ManifestNotJson`,
// `:334-336`). This build's connect value may name the package *directory* —
// documented, and three tests hand it a directory
// (`connectlink_test.go:68/115/177`) — and a directory has no suffix to check.
// The suffix rule therefore lives in `manifestFileFits`, on the file form only,
// after the stat has said which form the value is.
func linkManifestValue(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return "", false
	}
	if _, _, hasScheme := strings.Cut(value, "://"); hasScheme {
		return "", false
	}
	if manifestTraverses(value) {
		return "", false
	}
	// A home that cannot be resolved cannot produce the path the link promised:
	// joining against "" would quietly turn the value into a working-directory
	// relative path, which is a different file. `filepath.Join` cleans the
	// separator `~` leaves behind (`~/x` and `~\x` both land in the same place),
	// so no trimming branch is needed — the mutation that adds one back stays
	// green (report §5).
	if rest, ok := strings.CutPrefix(value, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, rest), true
	}
	if !filepath.IsAbs(value) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, value), true
	}
	return value, true
}

// manifestTraverses reports whether any component of the value is `..`, the
// reference's `Component::ParentDir` (`deep_link.rs:328-333`) spelled so that it
// answers the same on all three platforms (see `linkManifestValue`).
func manifestTraverses(value string) bool {
	for _, component := range strings.FieldsFunc(value, func(character rune) bool {
		return character == '/' || character == '\\'
	}) {
		if component == ".." {
			return true
		}
	}
	return false
}

// manifestFileFits reports whether a value that names a *file* may be the
// manifest: the reference's `.json` suffix rule (`deep_link.rs:334-336`), applied
// to the form this build can actually check. The connect flow takes the file's
// directory as the package, so without this rule any readable file name
// (`/tmp/notes.txt`) would connect whatever package happens to live next to it —
// a value the reference refuses (`ManifestNotJson`) and the user never meant.
//
// It is case-sensitive on purpose, like `str::ends_with` there: `.JSON` is a
// different spelling and this build has no reason to widen a refusal.
func manifestFileFits(path string) bool {
	return strings.HasSuffix(path, ".json")
}
