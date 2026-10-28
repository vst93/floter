package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

// `~` in the action bar is the same rule the open-path action and a dropped
// file use (`internal/homepath`), and the row shows the path it will open and
// copy — so the answer is visible before Enter is pressed.
func TestThePathRowOffersTheExpandedPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("home: %v", err)
	}
	a := testApp()
	a.SetQuery("~")
	results := a.Results()
	bar := actionBarIndex(results)
	if bar < 0 {
		t.Fatalf("the query drew no action bar: %+v", results)
	}
	if got := results[bar].Title; got != home {
		t.Errorf("the ~ row offers %q, want the home directory %q", got, home)
	}
}

// The expansion itself, with an explicit home so the answer does not depend on
// the machine. This is the pin a Windows build needs: the local code used to
// normalise the separator (`filepath.FromSlash`) *before* looking for `~/`, so
// `~/notes` became `~\notes`, the cut missed, and the row offered
// `<home>\~\notes` as the path to open and to copy.
func TestExpandPathIsTheSameOnEveryPlatform(t *testing.T) {
	home := t.TempDir()
	cases := []struct{ in, want string }{
		{"~", home},
		{"~/notes", filepath.Join(home, "notes")},
		{"~//notes", filepath.Join(home, "notes")},
		{"~alice", filepath.Join(home, "~alice")},
		{"notes", filepath.Join(home, "notes")},
	}
	for _, c := range cases {
		if got := expandPath(c.in, home); got != c.want {
			t.Errorf("expandPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
