package extensions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestTheSearchPathAppendsThePerUserToolDirectories pins the two directories a
// Finder/Dock launch never inherits but a user's own installers write: rustup's
// `~/.cargo/bin` and the XDG `~/.local/bin`. The reference appends both
// (runtime_path.rs:131-132, HOME_BASELINE_PATH_DIRS = [".cargo/bin",
// ".local/bin"]); a `homeBaselineDirs` that dropped `.cargo/bin` would silently
// hand `cargo`/`rustc` discovery the same broken answer as the bare launchd
// PATH, while every containment test above still passed.
func TestTheSearchPathAppendsThePerUserToolDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows installers write the process PATH themselves, so the home
		// list is empty by contract (the reference's #[cfg(unix)] gate).
		if got := homeBaselineDirs(); got != nil {
			t.Fatalf("homeBaselineDirs on Windows = %v, want nil", got)
		}
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to expand")
	}
	got := homeBaselineDirs()
	cargo := filepath.Join(home, ".cargo", "bin")
	local := filepath.Join(home, ".local", "bin")
	if len(got) != 2 || got[0] != cargo || got[1] != local {
		t.Fatalf("homeBaselineDirs = %v, want [%s %s]", got, cargo, local)
	}
}

// TestTheSearchPathMergeKeepsTheProcessEntriesFirst pins the append chain's
// order and deduplication: whatever the process inherited stays first (and
// therefore keeps its priority), then the login shell's answer, then the
// conventional baseline, then the home directories. Duplicates keep their first
// position and empty entries are dropped. The chain is asserted through the pure
// mergeSearchPath so the test never has to mutate the process environment.
func TestTheSearchPathMergeKeepsTheProcessEntriesFirst(t *testing.T) {
	merged := mergeSearchPath(
		[]string{"/usr/bin", "/bin", "/usr/bin", ""}, // a duplicated and an empty entry
		[]string{"/shell/bin"},
		[]string{"/opt/homebrew/bin", "/usr/bin"}, // /usr/bin already present
		[]string{"/home/u/.cargo/bin", "/home/u/.local/bin"},
	)
	want := []string{
		"/usr/bin",
		"/bin",
		"/shell/bin",
		"/opt/homebrew/bin",
		"/home/u/.cargo/bin",
		"/home/u/.local/bin",
	}
	if len(merged) != len(want) {
		t.Fatalf("merged = %v, want %v", merged, want)
	}
	for i := range want {
		if merged[i] != want[i] {
			t.Fatalf("merged[%d] = %q, want %q (whole: %v)", i, merged[i], want[i], merged)
		}
	}

	// A process with no PATH still gets the baseline, and no home list means no
	// home entry — the Windows shape the reference hands the same chain.
	onlyBaseline := mergeSearchPath(nil, nil, []string{"/usr/local/bin"}, nil)
	if len(onlyBaseline) != 1 || onlyBaseline[0] != "/usr/local/bin" {
		t.Fatalf("baseline-only merge = %v", onlyBaseline)
	}
}
