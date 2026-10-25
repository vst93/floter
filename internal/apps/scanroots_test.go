//go:build !windows

package apps

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// A desktop-file id is unique across the whole search path, so the same entry
// in two roots is one application — and the row that survives is the one from
// the earlier root, which is what makes a user's own copy an override.
func TestALinuxDesktopIDIsUniqueAcrossRoots(t *testing.T) {
	userRoot := t.TempDir()
	systemRoot := t.TempDir()
	write(t, filepath.Join(userRoot, "firefox.desktop"), `
[Desktop Entry]
Type=Application
Name=Firefox (mine)
Exec=/usr/bin/firefox --my-flags
`)
	write(t, filepath.Join(systemRoot, "firefox.desktop"), `
[Desktop Entry]
Type=Application
Name=Firefox
Exec=/usr/bin/firefox
`)
	write(t, filepath.Join(systemRoot, "gimp.desktop"), `
[Desktop Entry]
Type=Application
Name=GIMP
Exec=/usr/bin/gimp
`)

	for _, tc := range []struct {
		name string
		root []string
		want string
	}{
		{"user first", []string{userRoot, systemRoot}, "Firefox (mine)"},
		{"system first", []string{systemRoot, userRoot}, "Firefox"},
	} {
		got := scanLinuxRoots(tc.root)
		if names := appNames(got); !reflect.DeepEqual(names, sortedNames(tc.want, "GIMP")) {
			t.Errorf("%s: rows = %v, want one Firefox + GIMP", tc.name, names)
		}
		var exec []string
		for _, app := range got {
			if strings.HasPrefix(app.Name, "Firefox") {
				exec = app.Exec
			}
		}
		want := []string{"/usr/bin/firefox"}
		if tc.want == "Firefox (mine)" {
			want = []string{"/usr/bin/firefox", "--my-flags"}
		}
		if !reflect.DeepEqual(exec, want) {
			t.Errorf("%s: Firefox exec = %#v, want %#v", tc.name, exec, want)
		}
	}
}

// The id is the path *below* the applications directory with the separators
// turned into dashes, so a nested entry (`kde4/konsole.desktop`) collides with
// its system copy exactly as a top-level one does. Comparing file names would
// still pass the test above and fail this one.
func TestANestedDesktopIDIsThePathWithDashes(t *testing.T) {
	userRoot := t.TempDir()
	systemRoot := t.TempDir()
	write(t, filepath.Join(userRoot, "kde4", "konsole.desktop"), `
[Desktop Entry]
Type=Application
Name=Konsole (mine)
Exec=/usr/bin/konsole
`)
	write(t, filepath.Join(systemRoot, "kde4", "konsole.desktop"), `
[Desktop Entry]
Type=Application
Name=Konsole
Exec=/usr/bin/konsole
`)

	got := scanLinuxRoots([]string{userRoot, systemRoot})
	if names := appNames(got); !reflect.DeepEqual(names, []string{"Konsole (mine)"}) {
		t.Errorf("rows = %v, want only the user's copy", names)
	}
	if id := desktopFileID(userRoot, filepath.Join(userRoot, "kde4", "konsole.desktop")); id != "kde4-konsole.desktop" {
		t.Errorf("desktopFileID = %q, want kde4-konsole.desktop", id)
	}
}

// One root on its own keeps every entry it finds: ScanLinux has no second root
// to shadow anything, and its behaviour must not change.
func TestScanLinuxOnOneRootKeepsEveryEntry(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.desktop"), "[Desktop Entry]\nType=Application\nName=Alpha\nExec=/bin/alpha\n")
	write(t, filepath.Join(root, "b.desktop"), "[Desktop Entry]\nType=Application\nName=Beta\nExec=/bin/beta\n")

	if names := appNames(ScanLinux(root)); !reflect.DeepEqual(names, []string{"Alpha", "Beta"}) {
		t.Errorf("ScanLinux rows = %v", names)
	}
}

// The Linux search path is read from the environment, in precedence order: the
// user's data directory first, then the system's, then the package exports,
// then whatever $XDG_DATA_DIRS advertises.
func TestLinuxRootsFollowTheSearchPath(t *testing.T) {
	got := linuxRoots("/data/user", []string{"/usr/local/share", "/usr/share", "/opt/extra/share", "  "})
	want := []string{
		"/data/user/applications",
		"/data/user/flatpak/exports/share/applications",
		"/usr/local/share/applications",
		"/usr/share/applications",
		"/var/lib/flatpak/exports/share/applications",
		"/var/lib/snapd/desktop/applications",
		"/opt/extra/share/applications",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxRoots =\n %v\nwant\n %v", got, want)
	}
}

// $XDG_DATA_HOME is honoured when it is absolute, and the spec's own fallback
// (~/.local/share) is used when it is missing or relative.
func TestLinuxDataHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	cases := []struct {
		value string
		want  string
	}{
		{"/data/elsewhere", "/data/elsewhere"},
		{"", filepath.Join("/home/tester", ".local", "share")},
		{"relative/dir", filepath.Join("/home/tester", ".local", "share")},
	}
	for _, tc := range cases {
		t.Setenv("XDG_DATA_HOME", tc.value)
		if got := linuxDataHome(); got != tc.want {
			t.Errorf("linuxDataHome with %q = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// The Windows shortcut scan's two identity keys are shared across roots, so one
// program's shortcut in the user's Start Menu and in the machine's is one row —
// and the user's row is the one that survives. `scanWindowsRoots` is the merge
// `Scan` itself runs, so this pins the sharing and not just one root.
func TestAWindowsShortcutIsOneRowAcrossRoots(t *testing.T) {
	userRoot := t.TempDir()
	machineRoot := t.TempDir()
	target := `C:\Program Files\WXWork\WXWork.exe`
	writeShortcut(t, userRoot, "WeChat.lnk", buildShortcut(target, "", ""))
	writeShortcut(t, machineRoot, "WeChat.lnk", buildShortcut(target, "", ""))
	writeShortcut(t, machineRoot, "Paint.lnk", buildShortcut(`C:\Windows\mspaint.exe`, "", ""))

	found := scanWindowsRoots([]string{userRoot, machineRoot})

	names := sortedNames(appNames(found)...)
	if !reflect.DeepEqual(names, sortedNames("WeChat", "Paint")) {
		t.Fatalf("rows = %v, want one WeChat + Paint", names)
	}
	for _, app := range found {
		if app.Name != "WeChat" {
			continue
		}
		if !strings.HasPrefix(app.Path, userRoot) {
			t.Errorf("WeChat path = %q, want the user root's copy", app.Path)
		}
	}
}

// appNames is the rows' names in the order the scan returned them.
func appNames(found []App) []string {
	out := make([]string, 0, len(found))
	for _, app := range found {
		out = append(out, app.Name)
	}
	return out
}

// sortedNames is the scan's own order (by name, then path) for a fixed set.
func sortedNames(names ...string) []string {
	out := append([]string(nil), names...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// The whole-root entry point reaches the same rule: `Scan`'s root merge is the
// Linux branch on this host, so a two-root list is the user-override case end
// to end. Off Linux the same two roots hold nothing the platform's scanner
// reads (no `.app` bundle, no `.lnk`), which is the control that says the rows
// below come from the Linux scan.
func TestScanKeepsOneRowPerDesktopID(t *testing.T) {
	userRoot := t.TempDir()
	systemRoot := t.TempDir()
	write(t, filepath.Join(userRoot, "editor.desktop"), "[Desktop Entry]\nType=Application\nName=Editor\nExec=/bin/editor\n")
	write(t, filepath.Join(systemRoot, "editor.desktop"), "[Desktop Entry]\nType=Application\nName=Editor\nExec=/bin/editor\n")

	got := appNames(Scan([]string{userRoot, systemRoot}))
	if runtime.GOOS != "linux" {
		if len(got) != 0 {
			t.Fatalf("Scan rows = %v, want none on %s", got, runtime.GOOS)
		}
		return
	}
	if !reflect.DeepEqual(got, []string{"Editor"}) {
		t.Errorf("Scan rows = %v, want one Editor", got)
	}
}
