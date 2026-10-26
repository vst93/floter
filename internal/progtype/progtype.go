// Package progtype answers the one question four packages used to answer four
// different ways: "is this path something the user can run?"
//
// There is one Unix rule and two Windows strategies, because Windows has no
// execute bit and the two callers want opposite defaults:
//
//   - the extension strategy (For, Host): a Windows program is a file named
//     .exe, .cmd, .bat or .com, and a plain notes.txt is not one whatever its
//     mode. This is the rule for a name probed on the user's behalf, where a
//     wrong "yes" puts a non-program in a list (the tool inventory's export
//     walk, the PATH probes, a `.desktop` entry's `TryExec=`).
//   - the permissive strategy (ForLenient, HostLenient): on Windows every file
//     is runnable, because Windows decides by content and by association, not
//     by name. This is the rule for a path the user picked themselves, where a
//     wrong "no" would hide the program they just chose (a custom extension's
//     runtime).
//
// The two strategies are deliberately not merged: they disagree on a plain
// file, and that disagreement is the point (see TestTheStrategiesDisagree).
//
// Unix is one rule for both strategies: a regular file with any execute bit
// set. `os.Stat` follows symlinks, so a link to such a file is one too.
package progtype

import (
	"os"
	"runtime"

	"floter/internal/progname"
)

// Host reports whether path is a program on the machine running floter: the
// host's own rule, which on Windows is the extension strategy. It is
// For(runtime.GOOS, path) and nothing else.
func Host(path string) bool { return For(runtime.GOOS, path) }

// For reports whether path is a program on platform, using the extension
// strategy for Windows.
//
// platform is a `runtime.GOOS` spelling ("windows", "darwin", "linux") or a
// `tools.Platform` one ("windows", "macos", "linux"): only "windows" is
// special, every other value takes the Unix rule. An empty platform means the
// host, so a caller with no key still gets the local answer.
//
// The file is always read from the local filesystem — the platform only picks
// the rule, which is what lets a Linux host pin the Windows ladder.
func For(platform, path string) bool {
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform == "windows" {
		return windowsProgram(path)
	}
	return unixProgram(path)
}

// HostLenient is ForLenient(runtime.GOOS, path): the permissive strategy on
// the machine running floter.
func HostLenient(path string) bool { return ForLenient(runtime.GOOS, path) }

// ForLenient is For with the permissive Windows strategy: on Windows every
// file is runnable, so only existence and "not a directory" are asked. The
// Unix half is identical to For's — there is nothing permissive to be about a
// real execute bit.
func ForLenient(platform, path string) bool {
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform == "windows" {
		return windowsFile(path)
	}
	return unixProgram(path)
}

// file is the read every rule starts from: the path must exist and must not be
// a directory. `os.Stat` (not `Lstat`) is deliberate — a symlink to a program
// is a program.
func file(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, false
	}
	return info, true
}

// unixProgram is the Unix rule: a regular file with any of the three execute
// bits. The regular-file half keeps a FIFO or a device node that happens to
// carry an execute bit out of the answer.
func unixProgram(path string) bool {
	info, ok := file(path)
	if !ok {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// windowsProgram is the extension strategy. The four suffixes themselves live
// in `progname`, which also generates the probe ladder and strips the suffix
// back off: one table, three directions.
func windowsProgram(path string) bool {
	if _, ok := file(path); !ok {
		return false
	}
	return progname.IsLauncher(path)
}

// windowsFile is the permissive strategy: any file will do.
func windowsFile(path string) bool {
	_, ok := file(path)
	return ok
}
