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

// A dangling symlink is still a thing the user dropped: the reference asks
// `symlink_metadata` whether the path is there (which does not follow the
// link) and `is_dir` whether it is a folder (which does). The two answers must
// therefore disagree on every broken link: kept, and not a directory.
func TestNormalizeKeepsADanglingSymlinkButNotAsADirectory(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken")
	if err := os.Symlink(filepath.Join(dir, "gone.txt"), broken); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	realDir := filepath.Join(dir, "realdir")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dirLink := filepath.Join(dir, "dirlink")
	if err := os.Symlink(realDir, dirLink); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	files := Normalize([]string{broken, dirLink}, t.TempDir(), t.TempDir())
	if len(files) != 2 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].Path != broken || files[0].IsDirectory {
		t.Errorf("a dangling symlink = %+v, want kept and not a directory", files[0])
	}
	if files[1].Path != dirLink || !files[1].IsDirectory {
		t.Errorf("a symlink to a directory = %+v, want kept and a directory", files[1])
	}
}

// The trim is the emptiness test, not a repair: the path the reference resolves
// is the string the desktop reported. A path carrying surrounding whitespace is
// therefore a path that is not there, not one quietly turned into the file
// that would have been there without it.
func TestNormalizeDoesNotTrimThePathItResolves(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	write(t, file)

	if got := Normalize([]string{" " + file + " "}, t.TempDir(), t.TempDir()); len(got) != 0 {
		t.Fatalf("a padded path = %+v, want dropped", got)
	}
	// The same path without the padding is still the file, so the test above
	// is about the trim and not about the fixture.
	if got := Normalize([]string{file}, t.TempDir(), t.TempDir()); len(got) != 1 || got[0].Path != file {
		t.Fatalf("an unpadded path = %+v", got)
	}
}

// `~alice` is a directory literally called that, not another user's home:
// resolving it would walk out of the process's own filesystem. The reference
// joins it to home, so a file named `~alice` under home is kept and one under
// cwd is not.
func TestNormalizeTreatsALiteralTildeNameAsAFileUnderHome(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	write(t, filepath.Join(home, "~alice"))
	write(t, filepath.Join(cwd, "~bob"))

	got := Normalize([]string{"~alice", "~bob"}, home, cwd)
	if len(got) != 1 || got[0].Path != filepath.Join(home, "~alice") {
		t.Fatalf("literal tilde names = %+v, want only home/~alice", got)
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
