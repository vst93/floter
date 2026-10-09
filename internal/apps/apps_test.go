package apps

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitDesktopExec(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"firefox %u", []string{"firefox"}},
		{"firefox --new-window %U", []string{"firefox", "--new-window"}},
		{"env FOO=bar app", []string{"env", "FOO=bar", "app"}},
		{`sh -c "echo hi" %f`, []string{"sh", "-c", "echo hi"}},
		{"app with space", []string{"app", "with", "space"}},
		{`"/opt/My App/bin" %F`, []string{"/opt/My App/bin"}},
		{"printf 100%%", []string{"printf", "100%"}},
		{"", nil},
		{"   ", nil},
	}
	for _, tc := range cases {
		if got := SplitDesktopExec(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitDesktopExec(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDarwin(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Safari.app", "Contents", "MacOS"), 0o755)
	write(t, filepath.Join(root, "Safari.app", "Contents", "Info.plist"), "<plist/>")
	os.MkdirAll(filepath.Join(root, "Utilities", "Terminal.app"), 0o755)
	// A nested helper bundle inside a bundle must not become its own entry.
	os.MkdirAll(filepath.Join(root, "Safari.app", "Contents", "PlugIns", "Helper.app"), 0o755)
	write(t, filepath.Join(root, "not-an-app.txt"), "x")

	got := ScanDarwin(root)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	want := []string{"Safari", "Terminal"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("ScanDarwin names = %v, want %v", names, want)
	}
	if got[0].Path != filepath.Join(root, "Safari.app") {
		t.Errorf("Safari path = %q", got[0].Path)
	}
}

func TestScanWindows(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "Notepad.lnk"), "")
	write(t, filepath.Join(root, "Accessories", "Paint.lnk"), "")
	write(t, filepath.Join(root, "readme.txt"), "x")

	got := ScanWindows(root)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"Notepad", "Paint"}) {
		t.Errorf("ScanWindows names = %v", names)
	}
}

func TestScanLinux(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "firefox.desktop"), `
[Desktop Entry]
Type=Application
Name=Firefox
Exec=/usr/bin/firefox %u
Icon=firefox
`)
	write(t, filepath.Join(root, "hidden.desktop"), `
[Desktop Entry]
Name=Hidden
Exec=/usr/bin/hidden
Hidden=true
`)
	write(t, filepath.Join(root, "nodisplay.desktop"), `
[Desktop Entry]
Name=No Display
Exec=/usr/bin/nodisplay
NoDisplay=true
`)
	write(t, filepath.Join(root, "noexec.desktop"), `
[Desktop Entry]
Name=No Exec
`)
	write(t, filepath.Join(root, "other-section.desktop"), `
[Desktop Action new]
Name=New Window
Exec=/usr/bin/firefox --new-window
`)
	write(t, filepath.Join(root, "localized.desktop"), `
[Desktop Entry]
Name=Files
Name[zh]=文件
Exec=nautilus %U
`)

	got := ScanLinux(root)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"Files", "Firefox"}) {
		t.Errorf("ScanLinux names = %v", names)
	}
	for _, a := range got {
		if a.Name == "Firefox" {
			if !reflect.DeepEqual(a.Exec, []string{"/usr/bin/firefox"}) {
				t.Errorf("Firefox exec = %#v", a.Exec)
			}
		}
	}
}

func TestParseDesktopFileRejectsJunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "junk.desktop")
	write(t, path, "not a desktop file at all")
	if _, ok := ParseDesktopFile(path); ok {
		t.Error("a junk file parsed as an application")
	}
	if _, ok := ParseDesktopFile(filepath.Join(dir, "missing.desktop")); ok {
		t.Error("a missing file parsed as an application")
	}
}

func TestDedupeSortsAndCaps(t *testing.T) {
	in := []App{
		{Name: "zebra", Path: "/z"},
		{Name: "Alpha", Path: "/a"},
		{Name: "alpha", Path: "/a"}, // same path: one survives
		{Name: "beta", Path: "/b"},
	}
	got := dedupe(in)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"Alpha", "beta", "zebra"}) {
		t.Errorf("dedupe = %v", names)
	}
}
