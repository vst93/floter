package xdg

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// The desktop-entry roots are the reference's precedence order, by value: the
// user's data directory, then the system's (with `/usr/local/share` before
// `/usr/share`, the specification's own default order), then the two package
// exports, then whatever else the environment advertises.
func TestDesktopRootsFollowTheSearchPath(t *testing.T) {
	got := DesktopRoots("/data/user", []string{"/usr/local/share", "/usr/share", "/opt/extra/share", "  "})
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
		t.Fatalf("DesktopRoots =\n %v\nwant\n %v", got, want)
	}
}

// A directory the table already holds contributes nothing, so a machine whose
// `$XDG_DATA_DIRS` is the specification's default (`/usr/local/share:/usr/share`)
// reads each root once — and an empty `dataHome` drops the user's two roots
// without disturbing the order of the rest.
func TestDesktopRootsAreDeduplicated(t *testing.T) {
	got := DesktopRoots("/data/user", []string{"/usr/local/share", "/usr/share", "/data/user"})
	want := []string{
		"/data/user/applications",
		"/data/user/flatpak/exports/share/applications",
		"/usr/local/share/applications",
		"/usr/share/applications",
		"/var/lib/flatpak/exports/share/applications",
		"/var/lib/snapd/desktop/applications",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DesktopRoots with repeats =\n %v\nwant\n %v", got, want)
	}

	withoutHome := DesktopRoots("", nil)
	wantWithoutHome := []string{
		"/usr/local/share/applications",
		"/usr/share/applications",
		"/var/lib/flatpak/exports/share/applications",
		"/var/lib/snapd/desktop/applications",
	}
	if !reflect.DeepEqual(withoutHome, wantWithoutHome) {
		t.Fatalf("DesktopRoots with no dataHome =\n %v\nwant\n %v", withoutHome, wantWithoutHome)
	}
}

// `$XDG_DATA_HOME` is honoured when it is absolute, and the specification's own
// fallback (`~/.local/share`) is used when it is missing or relative.
func TestDataHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	cases := []struct {
		value string
		want  string
	}{
		{"/data/elsewhere", "/data/elsewhere"},
		{"", filepath.Join("/home/tester", ".local", "share")},
		{"relative/dir", filepath.Join("/home/tester", ".local", "share")},
		{"   ", filepath.Join("/home/tester", ".local", "share")},
	}
	for _, tc := range cases {
		t.Setenv("XDG_DATA_HOME", tc.value)
		if got := DataHome(); got != tc.want {
			t.Errorf("DataHome with %q = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// `$XDG_DATA_DIRS` is split on the platform's list separator, and an empty
// entry (a leading, trailing or doubled separator) contributes nothing rather
// than becoming a root of `""`.
func TestDataDirsDropsEmptyEntries(t *testing.T) {
	separator := string(filepath.ListSeparator)
	cases := []struct {
		value string
		want  []string
	}{
		{"/opt/a/share" + separator + "/opt/b/share", []string{"/opt/a/share", "/opt/b/share"}},
		{separator + "/opt/a/share" + separator + separator + "/opt/b/share" + separator, []string{"/opt/a/share", "/opt/b/share"}},
		{"  /opt/a/share  ", []string{"/opt/a/share"}},
		{"", nil},
	}
	for _, tc := range cases {
		t.Setenv("XDG_DATA_DIRS", tc.value)
		if got := DataDirs(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("DataDirs with %q = %v, want %v", tc.value, got, tc.want)
		}
	}
}

// DesktopRootsFromEnv is the table the environment describes: the two halves
// are not allowed to disagree.
func TestDesktopRootsFromEnvUsesBothVariables(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "/data/user")
	t.Setenv("XDG_DATA_DIRS", "/opt/extra/share")

	got := DesktopRootsFromEnv()
	want := DesktopRoots("/data/user", []string{"/opt/extra/share"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DesktopRootsFromEnv = %v, want %v", got, want)
	}
	for _, root := range []string{"/data/user/applications", "/opt/extra/share/applications"} {
		found := false
		for _, candidate := range got {
			if candidate == root {
				found = true
			}
		}
		if !found {
			t.Errorf("DesktopRootsFromEnv = %v, missing %q", got, root)
		}
	}
}

// writeEntry writes a file, creating its directories, and returns its path.
func writeEntry(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// walkEntries is WalkDesktopEntries as a sorted list of the paths it visited.
func walkEntries(root string, depth int) []string {
	var got []string
	WalkDesktopEntries(root, depth, func(path string) {
		got = append(got, path)
	})
	sort.Strings(got)
	return got
}

// The walk descends into subdirectories: a distribution nests entries a couple
// of levels deep (`kde4/`, `screensavers/`), and those entries have to be rows
// exactly as a top-level one is.
func TestWalkDesktopEntriesDescendsIntoSubdirectories(t *testing.T) {
	root := t.TempDir()
	top := writeEntry(t, filepath.Join(root, "top.desktop"), "[Desktop Entry]\n")
	nested := writeEntry(t, filepath.Join(root, "kde4", "konsole.desktop"), "[Desktop Entry]\n")
	deep := writeEntry(t, filepath.Join(root, "screensavers", "x", "y", "z.desktop"), "[Desktop Entry]\n")

	got := walkEntries(root, DesktopWalkDepth)
	want := []string{nested, deep, top}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked %v, want %v", got, want)
	}
}

// The depth cap is the reference's: a file three levels below the root is read,
// one four levels below is not (`collect_desktop_paths`, `if depth > 3`).
func TestWalkDesktopEntriesStopsAtTheReferenceDepth(t *testing.T) {
	root := t.TempDir()
	atThree := writeEntry(t, filepath.Join(root, "a", "b", "c", "kept.desktop"), "[Desktop Entry]\n")
	writeEntry(t, filepath.Join(root, "a", "b", "c", "d", "dropped.desktop"), "[Desktop Entry]\n")

	got := walkEntries(root, DesktopWalkDepth)
	if !reflect.DeepEqual(got, []string{atThree}) {
		t.Fatalf("walked %v, want only %q", got, atThree)
	}
}

// Only `*.desktop` files are visited: the walk answers "where are the desktop
// entries", not "what is in this tree", so a `README` or a `*.desktop.bak`
// beside an entry is not one.
func TestWalkDesktopEntriesVisitsOnlyDesktopFiles(t *testing.T) {
	root := t.TempDir()
	entry := writeEntry(t, filepath.Join(root, "app.desktop"), "[Desktop Entry]\n")
	writeEntry(t, filepath.Join(root, "README"), "not an entry\n")
	writeEntry(t, filepath.Join(root, "app.desktop.bak"), "[Desktop Entry]\n")

	got := walkEntries(root, DesktopWalkDepth)
	if !reflect.DeepEqual(got, []string{entry}) {
		t.Fatalf("walked %v, want only %q", got, entry)
	}
}
