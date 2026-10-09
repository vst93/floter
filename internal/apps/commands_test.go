package apps

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestScanCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's executables are shell scripts")
	}
	first := t.TempDir()
	second := t.TempDir()
	for name := range map[string]bool{"aaa": true, "tmux": true, "ripgrep": true} {
		writeMode(t, filepath.Join(first, name), "#!/bin/sh\n", 0o755)
	}
	// A later directory does not shadow the first, a non-runnable file is
	// not a command, and dot/artifact files are skipped.
	writeMode(t, filepath.Join(second, "aaa"), "#!/bin/sh\n", 0o755)
	writeMode(t, filepath.Join(second, "zzz"), "text", 0o644)
	writeMode(t, filepath.Join(second, ".hidden"), "#!/bin/sh\n", 0o755)
	writeMode(t, filepath.Join(second, "libfoo.dylib"), "binary", 0o755)

	got := ScanCommands([]string{first, second, filepath.Join(first, "missing")})
	names := make([]string, 0, len(got))
	for _, app := range got {
		names = append(names, app.Name)
	}
	want := []string{"aaa", "ripgrep", "tmux"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("commands = %v, want %v", names, want)
	}
	if got[0].Path != filepath.Join(first, "aaa") {
		t.Errorf("the first directory did not win: %q", got[0].Path)
	}
	if len(ScanCommands(nil)) != 0 {
		t.Error("no directories yielded commands")
	}
}

func TestCommandDirs(t *testing.T) {
	t.Setenv("PATH", "/one:"+"/two::/three")
	got := CommandDirs()
	if !reflect.DeepEqual(got, []string{"/one", "/two", "/three"}) {
		t.Errorf("dirs = %v", got)
	}
}

func writeMode(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
