// Package pathlist answers the two questions about a PATH-shaped string that
// this repository used to answer three different ways:
//
//   - Split: which directories does this value name?
//   - Lookup: where does this program live on PATH?
//
// The reference answers both with `std::env::split_paths` at every call site
// (`main:src-tauri/src/commands/apps/linux.rs:203,479`,
// `main:src-tauri/src/extensions/catalog.rs:1003`,
// `main:src-tauri/src/extensions/inventory.rs:505`). Here the answer is
// `path/filepath`'s own splitter, for one reason: on Windows it strips the
// surrounding double quotes an installer is allowed to write around a PATH
// entry (`"C:\Program Files\Git\cmd"`). `strings.Split` on the list separator
// leaves those quotes on the directory name, so the joined path does not
// exist and the program is not found — on Windows only, which is exactly the
// cross-platform difference this package exists to remove.
package pathlist

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"floter/internal/progtype"
)

// Split is the one way a PATH-shaped value becomes directories: the platform's
// list separator, with the empty entries dropped.
//
//   - The empty string names no directories (`filepath.SplitList` returns an
//     empty slice, where `strings.Split` would return one empty entry).
//   - An empty entry in the middle (`a::b`) is dropped too. A shell reads it as
//     "the current directory", but that rule belongs to the one caller that
//     resolves a program — Lookup — and it gets it from `exec.LookPath`.
//     A scanner listing a PATH must not treat floter's own working directory as
//     a place the user's programs live.
//   - Quoting is the platform's business: Windows strips the quotes, Unix has
//     no quoting to strip.
func Split(value string) []string {
	var out []string
	for _, dir := range filepath.SplitList(value) {
		if dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

// Lookup is the one answer to "where does this program live on PATH": the
// first directory of the search path that carries it as a program, and then
// Go's own `exec.LookPath` as the last word.
//
// A name that carries a path separator is a path, not a PATH entry, and is
// judged on its own (`progtype.Host`) without being joined to any directory.
// The reference short-circuits the same way (`is_executable`,
// `main:src-tauri/src/commands/apps/linux.rs:476-478`); joining it to every
// directory instead makes `<PATH dir>/bin/foo` a hit for the name `bin/foo`,
// which is not a hit at all.
func Lookup(name string) (string, bool) { return lookupIn(os.Getenv("PATH"), name) }

// lookupIn is Lookup against an explicit PATH value: the seam a test resolves
// against a fixture instead of the machine it runs on.
func lookupIn(path, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	if strings.ContainsRune(name, os.PathSeparator) || strings.ContainsRune(name, '/') {
		if progtype.Host(name) {
			return name, true
		}
		return "", false
	}
	for _, dir := range Split(path) {
		candidate := filepath.Join(dir, name)
		if progtype.Host(candidate) {
			return candidate, true
		}
	}
	// The fallback is not a second opinion about "is this a program": it is
	// the platform's own answer for the two cases Split cannot express — a
	// `PATHEXT` extension the caller left off, and the empty entry that means
	// "the working directory".
	if found, err := exec.LookPath(name); err == nil {
		return found, true
	}
	return "", false
}
