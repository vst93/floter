package apps

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachedScanRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Safari.app", "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := []string{dir}

	// No scan yet: nothing is cached.
	if found, upToDate := CachedScan(roots); upToDate || found != nil {
		t.Fatalf("an empty cache answered: %+v upToDate=%v", found, upToDate)
	}

	// A scan is stored, and the next read is up to date. The stored entries
	// are fixtures: what this pins is the cache, not the scanners.
	scan := []App{{Name: "Safari", Path: filepath.Join(dir, "Safari.app")}}
	StoreScan(roots, scan)
	found, upToDate := CachedScan(roots)
	if !upToDate || len(found) != 1 || found[0].Name != "Safari" {
		t.Fatalf("cached = %+v upToDate=%v", found, upToDate)
	}

	// A source change invalidates the cache.
	if err := os.MkdirAll(filepath.Join(dir, "Terminal.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, upToDate = CachedScan(roots)
	if upToDate {
		t.Error("the cache stayed valid after a source change")
	}

	// An unrelated directory does not invalidate it.
	other := t.TempDir()
	// Closed before the temp dir is cleaned up: Windows refuses to remove a
	// file another handle still holds, and the failure lands on the cleanup
	// rather than on anything the cache did.
	f, err := os.Create(filepath.Join(other, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	StoreScan(roots, scan)
	_, upToDate = CachedScan(roots)
	if !upToDate {
		t.Error("an unrelated change invalidated the cache")
	}
}

func TestPathsSignature(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	one := SourceSignature([]string{dir})
	// The same sources hash the same way, and the order they are listed in
	// does not matter: the paths are sorted before they are hashed.
	if two := SourceSignature([]string{dir}); one != two {
		t.Errorf("the same sources hashed differently: %d vs %d", one, two)
	}
	other := t.TempDir()
	if SourceSignature([]string{dir, other}) != SourceSignature([]string{other, dir}) {
		t.Error("the sources' order changed the signature")
	}

	// A new entry changes the signature. What covers it is the entry's own
	// path — the cache hashes the roots *and* their first two levels of names,
	// not the root's modification time, which is a guarantee Windows does not
	// give: NTFS writes a directory's timestamps when its handle closes, so a
	// child created moments ago may leave the parent's mtime untouched. This
	// test used to hash a bare root path and wait on that mtime; it read as a
	// cache bug on windows-latest while the cache was answering correctly.
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if SourceSignature([]string{dir}) == one {
		t.Error("a new entry did not change the signature")
	}

	// A change in place — same name, new mtime — is covered too: each path's
	// modification time is mixed into the hash. The file's own time is set
	// explicitly rather than waited on, so this holds on every filesystem.
	tool := filepath.Join(dir, "a", "tool")
	if err := os.WriteFile(tool, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := SourceSignature([]string{dir})
	if err := os.Chtimes(tool, time.Unix(1000000, 0), time.Unix(1000000, 0)); err != nil {
		t.Fatal(err)
	}
	if SourceSignature([]string{dir}) == before {
		t.Error("a touched file did not change the signature")
	}

	// An empty list hashes to something, and different lists differ.
	if pathsSignature(nil) == pathsSignature([]string{dir}) {
		t.Error("an empty list hashed like a non-empty one")
	}
}
