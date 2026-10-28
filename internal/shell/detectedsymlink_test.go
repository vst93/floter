package shell

import (
	"os"
	"path/filepath"
	"testing"
)

// A tool the user connected through a symlink must not be offered again under
// the resolved binary's path.
//
// The Detected list is built by walking every candidate and skipping the ones
// the integrations already own. That set used to hold the recorded path's
// spellings while the lookup used the candidate's — so a tool connected through
// `~/.local/bin/rg` (a symlink) came back as "detected" through
// `~/.cargo/bin/rg`, and could be connected a second time. Both sides now go
// through `pathid.Keys`.
func TestAConnectedToolIsNotDetectedAgainThroughItsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "rg")
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "rg-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlinks unavailable in the test directory: %v", err)
	}
	other := filepath.Join(dir, "fd")
	if err := os.WriteFile(other, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Recorded resolved, found as the link.
	if !connected(connectedKeys([]string{real}), link) {
		t.Fatalf("a candidate at %q read as unconnected while %q is connected", link, real)
	}
	// Recorded as the link, found resolved.
	if !connected(connectedKeys([]string{link}), real) {
		t.Fatalf("a candidate at %q read as unconnected while %q is connected", real, link)
	}
	// A different program stays detected.
	if connected(connectedKeys([]string{real}), other) {
		t.Fatalf("a candidate at %q read as connected to %q", other, real)
	}
	// An empty recorded path owns nothing — "" is the absence of a program.
	if connected(connectedKeys([]string{""}), link) || connected(connectedKeys([]string{real}), "") {
		t.Fatalf("an empty path matched a real program")
	}
}
