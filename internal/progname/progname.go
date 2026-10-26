// Package progname is the one place that knows which suffixes Windows puts on
// a launcher: `.exe`, `.cmd`, `.bat`, `.com`.
//
// That table used to live in five files, read in three different directions:
//
//   - accepting: "is this file a program?" — `progtype`'s extension strategy
//     compared a path's extension against the list;
//   - generating: "which file names may a probe resolve to?" — the tool
//     inventory's Windows ladder appended each suffix to a bare name;
//   - stripping: "what is this program called?" — a connected program's
//     command word and the curated ranking both cut the suffix back off.
//
// One table now, read the same three ways (IsLauncher, Ladder, Stem/FromPath),
// so a suffix added or dropped here changes all three at once instead of three
// of five.
//
// Three callers keep a *narrower* set on purpose, because they ask a different
// question, and each of those is pinned where it lives rather than here:
//
//   - `apps.IsLaunchableTarget` accepts `.exe`/`.bat`/`.cmd` only — the Start
//     Menu's read-me and web links are dropped exactly as the reference does
//     (`src-tauri/src/commands/apps/windows.rs:539-549`), and a `.com`
//     shortcut is a leftover DOS entry;
//   - `extensions.ProgramIsCmdScript` accepts `.cmd`/`.bat` only — the question
//     there is whether the program needs `cmd.exe` to start, and `.exe`/`.com`
//     do not;
//   - `apps` reads registry icon paths as `.exe` only, because that is the only
//     spelling the uninstall keys write.
//
// The reference is deliberately not the model for this table: it has no single
// answer either (`executable_display_name` strips nothing,
// `executable_name` strips any extension with `file_stem`). Stripping exactly a
// launcher suffix is ours, and it is the better rule — `foo.tar.gz` is not
// renamed `foo.tar`.
package progname

import (
	"path/filepath"
	"runtime"
	"strings"
)

// launcherSuffixes are the four names Windows itself treats as a launcher.
// The only copy in the repository.
var launcherSuffixes = []string{".exe", ".cmd", ".bat", ".com"}

// IsLauncher reports whether name ends with a launcher suffix, ignoring case:
// `RG.EXE` and `rg.exe` both are, `rgexe` and `rg.exe.txt` are not. It is the
// accepting direction — the caller has already decided the path exists.
func IsLauncher(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range launcherSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// Stem is the word a name gives once its launcher suffix is gone, lowercased
// and trimmed: `RG.EXE` → `rg`, `  rg.cmd ` → `rg`, `rg` → `rg`. Exactly one
// suffix is dropped, so `rg.exe.txt` keeps its name.
func Stem(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range launcherSuffixes {
		if trimmed, ok := strings.CutSuffix(name, suffix); ok {
			return trimmed
		}
	}
	return name
}

// FromPath is Stem of the path's own file name: `/usr/bin/RG.EXE` → `rg`. The
// separator is the host's, because the path is one the host produced.
func FromPath(path string) string { return Stem(filepath.Base(path)) }

// Ladder is the file names one probe for name may resolve to on platform: on
// Windows each launcher suffix and then the bare name, in that order; on every
// other platform the bare name alone. An empty platform means the host.
//
// platform is read rather than `runtime.GOOS` so a Linux host can pin the
// Windows ladder.
func Ladder(platform, name string) []string {
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform != "windows" {
		return []string{name}
	}
	ladder := make([]string, 0, len(launcherSuffixes)+1)
	for _, suffix := range launcherSuffixes {
		ladder = append(ladder, name+suffix)
	}
	return append(ladder, name)
}
