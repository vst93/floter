package inventory

import (
	"reflect"
	"testing"
)

// The tool inventory reads the same desktop-entry roots the launcher's
// application scan reads, in the same order. This package used to keep its own
// copy of the table, and the copy had drifted: the fixed system roots came back
// reversed (`/usr/share` before `/usr/local/share`, so a local administrator's
// override lost to the distribution's copy) and after `$XDG_DATA_DIRS` instead
// of before it, and the two system package-export roots were missing. Pinning
// the whole table by value is what keeps the two answers one answer.
func TestTheToolInventoryReadsTheLaunchersDesktopRoots(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "/data/user")
	t.Setenv("XDG_DATA_DIRS", "/opt/extra/share")

	got := linuxDesktopRoots()
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
		t.Fatalf("linuxDesktopRoots =\n %v\nwant\n %v", got, want)
	}
}

// The roots follow the environment: a machine that publishes its applications
// under a custom prefix (`$XDG_DATA_HOME`), or that adds one through
// `$XDG_DATA_DIRS`, is where a hard-coded list finds nothing. The user's own
// directory still comes first, so its entries win.
func TestTheToolInventoryFollowsTheEnvironment(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "/data/user")
	t.Setenv("XDG_DATA_DIRS", "")

	got := linuxDesktopRoots()
	if len(got) == 0 || got[0] != "/data/user/applications" {
		t.Fatalf("linuxDesktopRoots = %v, want the user's applications first", got)
	}
	for _, root := range got {
		if root == "/opt/extra/share/applications" {
			t.Fatalf("linuxDesktopRoots = %v, want no root from an empty $XDG_DATA_DIRS", got)
		}
	}

	t.Setenv("XDG_DATA_DIRS", "/opt/extra/share")
	if got := linuxDesktopRoots(); got[len(got)-1] != "/opt/extra/share/applications" {
		t.Fatalf("linuxDesktopRoots = %v, want $XDG_DATA_DIRS last", got)
	}
}
