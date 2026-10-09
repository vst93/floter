package drops

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeResolvesAndFilters(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	write(t, filepath.Join(home, "Documents", "report.pdf"))
	write(t, filepath.Join(cwd, "local.txt"))
	if err := os.MkdirAll(filepath.Join(home, "Projects"), 0o755); err != nil {
		t.Fatal(err)
	}

	files := Normalize([]string{
		"~/Documents/report.pdf",
		"local.txt",
		"~/Projects",
		"/does/not/exist",
		"   ",
	}, home, cwd)
	if len(files) != 3 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].Name != "report.pdf" || files[0].IsDirectory ||
		files[0].Directory != filepath.Join(home, "Documents") {
		t.Errorf("first = %+v", files[0])
	}
	if files[1].Path != filepath.Join(cwd, "local.txt") {
		t.Errorf("relative path = %+v", files[1])
	}
	if !files[2].IsDirectory || files[2].Path != filepath.Join(home, "Projects") {
		t.Errorf("directory = %+v", files[2])
	}
	if files[2].Directory != home {
		t.Errorf("directory's parent = %q", files[2].Directory)
	}

	// No home to expand means the path is dropped, not guessed at.
	if got := Normalize([]string{"~/x"}, "", cwd); len(got) != 0 {
		t.Errorf("no home = %+v", got)
	}
	if got := Normalize(nil, home, cwd); len(got) != 0 {
		t.Errorf("no paths = %+v", got)
	}
}

func TestDirectoryForCD(t *testing.T) {
	file := File{Path: "/tmp/dir/file.txt", Directory: "/tmp/dir"}
	if got := DirectoryForCD(file); got != "/tmp/dir" {
		t.Errorf("a file's cd = %q", got)
	}
	folder := File{Path: "/tmp/dir", Directory: "/tmp", IsDirectory: true}
	if got := DirectoryForCD(folder); got != "/tmp/dir" {
		t.Errorf("a folder's cd = %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, unix, windows string }{
		{"/tmp/plain.txt", "/tmp/plain.txt", "/tmp/plain.txt"},
		{"/tmp/with space.txt", "'/tmp/with space.txt'", `"/tmp/with space.txt"`},
		{"/tmp/it's.txt", `'/tmp/it'\''s.txt'`, `/tmp/it's.txt`},
		{"/tmp/a$b", "'/tmp/a$b'", "/tmp/a$b"},
		{"/tmp/a&b", "'/tmp/a&b'", `"/tmp/a&b"`},
	}
	for _, tc := range cases {
		if got := ShellQuote(tc.in, false); got != tc.unix {
			t.Errorf("ShellQuote(%q, unix) = %q, want %q", tc.in, got, tc.unix)
		}
		if got := ShellQuote(tc.in, true); got != tc.windows {
			t.Errorf("ShellQuote(%q, windows) = %q, want %q", tc.in, got, tc.windows)
		}
	}
	if got := CDCommand("/tmp/with space"); got != "cd '/tmp/with space'" && runtime.GOOS != "windows" {
		t.Errorf("CDCommand = %q", got)
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
