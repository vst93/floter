package extensions

import (
	"path/filepath"
	"runtime"

	"floter/internal/progname"
	"floter/internal/progtype"
)

// ExecutableCandidate is one program the search path offers for a system
// runtime — the reference's `ToolCandidate`, cut to the discovery source this
// build has (the search path). A row lists them when a manifest's declared
// executable names resolve to more than one program, so the user picks the one
// the integration should run: the reference's `ResolveResult::Ambiguous` and
// the `chooseSystemTool` dialog (`src/ExtensionsPanel.tsx:1393-1401,1975-1988`
// build `toolCandidates` and the chooser).
type ExecutableCandidate struct {
	// Name is the executable file's own name, the way the row titles it
	// (`candidate.name`).
	Name string
	// Path is the absolute path to the program (`locator.path`).
	Path string
	// Sources names where the program was found. The reference's inventory
	// knew several (Launch Services, desktop entries, the search path); this
	// build's only discovery source is the search path, so it is always
	// `["path"]`.
	Sources []string
}

// LookTools lists every runnable file in the search path whose file name is
// one of names, in search-path order and deduplicated by resolved path. It is
// the many-match form of LookTool, for the flows that must ask which program
// the user means rather than silently taking the first.
func LookTools(names ...string) []ExecutableCandidate {
	return lookToolsIn(runtime.GOOS, SearchDirectories(), names...)
}

// lookTools is the seam the candidate enumeration goes through, so a test can
// resolve against a fixture instead of the machine's search path. It mirrors
// `lookTool`.
var lookTools = LookTools

// StubToolCandidates replaces the candidate enumeration for a test, and returns
// the function that puts it back. It is the seam a test uses to make a
// manifest's declared names resolve to several programs, the state the
// chooser exists for.
func StubToolCandidates(lookup func(names ...string) []ExecutableCandidate) func() {
	previous := lookTools
	lookTools = lookup
	return func() { lookTools = previous }
}

// lookToolsIn is LookTools over an explicit directory list and platform key,
// so a test can resolve against a fixture. The join is `progname.Ladder` for
// the reason spelled out on `lookToolIn`: a name is a probe, and on Windows the
// program a bare `rg` asks for is `rg.exe`.
//
// `Name` is the file name that was found (`rg.exe`), not the name that was
// asked for (`rg`), because that is what the candidate *is*: the reference's
// PATH discovery names a candidate `path.file_name()`
// (src-tauri/src/extensions/inventory.rs:434-451), this build's inventory does
// the same (`merge(path, entry.Name(), …)`, inventory.go:239), and the register
// link names the integration `filepath.Base` of the path it resolved
// (shell/system.go:269). The chooser row, the Detected row and the register card
// therefore title one program one way.
func lookToolsIn(platform string, dirs []string, names ...string) []ExecutableCandidate {
	var out []ExecutableCandidate
	seen := map[string]bool{}
	for _, dir := range dirs {
		for _, name := range names {
			if name == "" {
				continue
			}
			for _, probe := range progname.Ladder(platform, name) {
				path := filepath.Join(dir, probe)
				if !progtype.ForLenient(platform, path) {
					continue
				}
				key := path
				if resolved, err := filepath.EvalSymlinks(path); err == nil {
					key = resolved
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, ExecutableCandidate{Name: probe, Path: path, Sources: []string{"path"}})
			}
		}
	}
	return out
}

// SystemRuntimeCandidates returns the programs a system manifest's runtime
// offers, in resolution order. The recorded program is the reference's
// `preferred_locator`: when it is still runnable it is the answer and the list
// is just it, so a caller never asks about a binding the integration already
// has (the reference's history score made it the unique best match). Otherwise
// every search-path match is a candidate, and more than one means the caller
// must ask which one.
//
// A non-system runtime (a bundled or script provider) has nothing to choose
// from: its program ships with the package or is the interpreter the run path
// resolves.
func SystemRuntimeCandidates(manifest Manifest, recordedPath string) []ExecutableCandidate {
	if manifest.Runtime.Type != "system" {
		return nil
	}
	names := manifest.Runtime.ExecutableNames
	if len(names) == 0 {
		names = []string{manifest.ID}
	}
	candidates := lookTools(names...)
	if recordedPath != "" {
		for _, candidate := range candidates {
			if sameExecutable(candidate.Path, recordedPath) {
				return []ExecutableCandidate{candidate}
			}
		}
	}
	return candidates
}

// sameExecutable reports whether two paths name the same program, following
// symlinks when the filesystem can (a version manager's shim and the binary it
// points at are the same tool).
func sameExecutable(a, b string) bool {
	if a == b {
		return true
	}
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && resolvedA == resolvedB
}
