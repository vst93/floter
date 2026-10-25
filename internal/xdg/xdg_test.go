package xdg

import (
	"path/filepath"
	"reflect"
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
