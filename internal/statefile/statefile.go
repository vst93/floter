// Package statefile is the one way this app writes one of its own state files.
//
// The app keeps a handful of them — settings and its recovery copy, the usage
// table behind the launcher's frequent rows, the clipboard index and the
// images beside it, the remembered window state. They are small JSON (or PNG)
// documents a reader loads on the next start, and every one of them is
// rewritten in place. Writing one of those with a plain `os.WriteFile` is a
// truncate followed by a write: a crash, a power loss or a full disk between
// the two leaves a *half* document where the old one was, and the next start
// reads a parse error instead of the user's data.
//
// So the write is a temporary file in the same directory, flushed to the
// platform, closed, given its final mode, and renamed over the target: a
// reader sees the old bytes or the new bytes, never a mixture. The directory
// is synced afterwards where the platform has such a thing, so the *name* is
// durable too.
//
// It is a package rather than a helper beside its first caller because the
// same fifteen lines had been written seven times, and the copies had drifted:
// one of them never flushed the file before renaming it (so the rename could
// land before the bytes did), and two of them never renamed at all. One
// implementation is the only way "this file is either old or new" can be true
// of every file at once.
//
// The two seams (`syncFile`, `renameFile`) exist so the ordering — flush
// before rename, target untouched when the rename is refused — is pinned by a
// test rather than by reading the function.
package statefile

import (
	"os"
	"path/filepath"
)

// tempPattern is the temporary file's name. It is deliberately not the target
// name: a reader that lists the directory must not mistake a write in flight
// for a state file, and a failed write must be recognizable as garbage.
const tempPattern = ".floter-state-*.tmp"

// syncFile flushes a temporary file to the platform. It is a variable so a
// test can refuse the flush and prove the write stops there.
var syncFile = func(f *os.File) error { return f.Sync() }

// renameFile moves the finished temporary file over the target. It is a
// variable for the same reason: a refused rename must leave the target's old
// bytes exactly as they were.
var renameFile = os.Rename

// Write writes data to path by way of a temporary file in the same directory:
// create, write, flush, close, chmod, then rename over path. A reader
// therefore sees either the old file or the new one, never a truncated one,
// and a crash between the writes cannot leave a half-written document behind.
//
// mode is the finished file's permission bits (the callers' own contract:
// state files are 0600, the window-state file is 0644). A missing directory is
// created first, so a first run does not have to know the layout.
func Write(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return err
	}
	name := temporary.Name()
	// Every exit after this point removes the temporary file, so a refused
	// write leaves the directory as it found it. After a successful rename
	// there is nothing left to remove and the call is a no-op.
	defer os.Remove(name)

	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := syncFile(temporary); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if err := renameFile(name, path); err != nil {
		return err
	}
	// The rename is the durable step on Unix; syncing the directory makes the
	// new name survive a power loss. Windows has no directory fsync, so a
	// failure there is ignored rather than reported: the data file itself is
	// already synced.
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}
