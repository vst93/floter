package apps

import (
	"os"
	"path/filepath"
	"testing"
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
	if _, err := os.Create(filepath.Join(other, "x")); err != nil {
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
	one := pathsSignature([]string{dir})
	// Order does not matter.
	two := pathsSignature([]string{dir})
	if one != two {
		t.Errorf("the same sources hashed differently: %d vs %d", one, two)
	}
	// A new entry changes the signature.
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if pathsSignature([]string{dir}) == one {
		t.Error("a new entry did not change the signature")
	}
	// An empty list hashes to something, and different lists differ.
	if pathsSignature(nil) == pathsSignature([]string{dir}) {
		t.Error("an empty list hashed like a non-empty one")
	}
}
