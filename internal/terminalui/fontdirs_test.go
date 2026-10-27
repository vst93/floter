package terminalui

import (
	"path/filepath"
	"testing"
)

// P10-u · the per-user data directory has one answer in this build
// (`xdg.DataHome`), and the font walk is one of its readers: the faces a user
// installs for themselves live in `$XDG_DATA_HOME/fonts`, so spelling the
// fallback (`~/.local/share/fonts`) into the table hides every font of a user
// whose `$XDG_DATA_HOME` is set to something else.
//
// The table is a pure function of its arguments, so this pin holds on any host.
func TestLinuxFontDirsFollowTheDataHome(t *testing.T) {
	const home = "/home/tester"
	const dataHome = "/srv/data"

	dirs := fontDirsFor("linux", home, "", "", dataHome)
	if len(dirs) == 0 {
		t.Fatal("no font directories")
	}
	if want := filepath.Join(dataHome, "fonts"); dirs[0] != want {
		t.Errorf("first font directory = %q, want %q (the data home's own fonts)", dirs[0], want)
	}
	stale := filepath.Join(home, ".local", "share", "fonts")
	for _, dir := range dirs {
		if dir == stale {
			t.Errorf("the data home's fallback %q is still in the table: %v", stale, dirs)
		}
	}
	// The rest of the Linux table is the user's legacy directory plus the two
	// system ones, unchanged.
	want := []string{
		filepath.Join(dataHome, "fonts"),
		filepath.Join(home, ".fonts"),
		"/usr/share/fonts",
		"/usr/local/share/fonts",
	}
	if len(dirs) != len(want) {
		t.Fatalf("font directories = %v, want %v", dirs, want)
	}
	for index := range want {
		if dirs[index] != want[index] {
			t.Errorf("font directory %d = %q, want %q", index, dirs[index], want[index])
		}
	}
}

// The other two platforms' tables are the ones that shipped, entry for entry:
// a data home is a Linux question and must not leak into either of them.
func TestFontDirTablesPerPlatform(t *testing.T) {
	darwin := fontDirsFor("darwin", "/Users/tester", "", "", "/srv/data")
	wantDarwin := []string{
		"/System/Library/Fonts",
		"/Library/Fonts",
		"/Users/tester/Library/Fonts",
	}
	if len(darwin) != len(wantDarwin) {
		t.Fatalf("darwin font directories = %v, want %v", darwin, wantDarwin)
	}
	for index := range wantDarwin {
		if darwin[index] != wantDarwin[index] {
			t.Errorf("darwin font directory %d = %q, want %q", index, darwin[index], wantDarwin[index])
		}
	}

	// The Windows table: the system root's Fonts, plus the per-user Fonts
	// directory only when LOCALAPPDATA says where it is.
	localAppData := filepath.Join("C:", "Users", "tester", "AppData", "Local")
	systemRoot := filepath.Join("C:", "Windows")
	windows := fontDirsFor("windows", filepath.Join("C:", "Users", "tester"), systemRoot, localAppData, "/srv/data")
	if len(windows) != 2 {
		t.Fatalf("windows font directories = %v, want the system and the per-user one", windows)
	}
	if want := filepath.Join(systemRoot, "Fonts"); windows[0] != want {
		t.Errorf("windows font directory 0 = %q, want %q", windows[0], want)
	}
	if want := filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"); windows[1] != want {
		t.Errorf("windows font directory 1 = %q, want %q", windows[1], want)
	}

	// No LOCALAPPDATA: the per-user directory is not guessed, and the system
	// root falls back to the shipped default.
	only := fontDirsFor("windows", filepath.Join("C:", "Users", "tester"), "", "", "/srv/data")
	if len(only) != 1 {
		t.Fatalf("windows font directories without LOCALAPPDATA = %v, want the system one only", only)
	}
	if want := filepath.Join(`C:\Windows`, "Fonts"); only[0] != want {
		t.Errorf("windows font directory 0 = %q, want %q", only[0], want)
	}
}
