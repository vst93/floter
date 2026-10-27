package settings

import (
	"path/filepath"
	"testing"
)

// P10-u · one answer per question: "where is the per-user data directory" is
// `xdg.DataHome`'s, including the specification's rule that a *relative*
// `$XDG_DATA_HOME` is ignored. The migration used to spell the Linux paths out
// by hand without that rule, so `XDG_DATA_HOME=rel/dir` made the legacy import
// read a directory relative to the process's working directory.
//
// The table is a pure function of its arguments, so this pin holds on any host:
// it asserts each platform's arm, not just the one this machine compiles.
func TestDataDirForFollowsThePlatformTable(t *testing.T) {
	cases := []struct {
		name         string
		goos         string
		home         string
		localAppData string
		dataHome     string
		want         string
	}{
		{
			name:         "windows uses LOCALAPPDATA",
			goos:         "windows",
			home:         filepath.Join("C:", "Users", "tester"),
			localAppData: filepath.Join("C:", "Users", "tester", "AppData", "Local"),
			dataHome:     "/srv/data",
			want:         filepath.Join("C:", "Users", "tester", "AppData", "Local"),
		},
		{
			name:         "windows without LOCALAPPDATA answers nothing",
			goos:         "windows",
			home:         filepath.Join("C:", "Users", "tester"),
			localAppData: "",
			dataHome:     "/srv/data",
			want:         "",
		},
		{
			name:         "macOS uses Application Support",
			goos:         "darwin",
			home:         "/Users/tester",
			localAppData: "",
			dataHome:     "/srv/data",
			want:         "/Users/tester/Library/Application Support",
		},
		{
			name:         "macOS without a home answers nothing",
			goos:         "darwin",
			home:         "",
			localAppData: "",
			dataHome:     "/srv/data",
			want:         "",
		},
		{
			name:         "linux is the data home it was given",
			goos:         "linux",
			home:         "/home/tester",
			localAppData: "",
			dataHome:     "/srv/data",
			want:         "/srv/data",
		},
		{
			// The point of delegating: a data home is used as given, never
			// re-derived from the home directory. A `~/.local/share` answer
			// here is the bug this round removes.
			name:         "linux does not re-derive the fallback",
			goos:         "linux",
			home:         "/home/tester",
			localAppData: "",
			dataHome:     "/home/tester/.local/share",
			want:         "/home/tester/.local/share",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := dataDirFor(testCase.goos, testCase.home, testCase.localAppData, testCase.dataHome)
			if got != testCase.want {
				t.Errorf("dataDirFor(%q, %q, %q, %q) = %q, want %q",
					testCase.goos, testCase.home, testCase.localAppData, testCase.dataHome, got, testCase.want)
			}
		})
	}
}
