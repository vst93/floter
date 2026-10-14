package settings

import (
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path by way of a temporary file in the same
// directory: create, write, sync, close, chmod, then rename over path. A
// reader therefore sees either the old file or the new one, never a truncated
// one, and a crash between the writes cannot leave a half-written JSON file
// behind. It is the Go counterpart of the old build's
// `write_settings_file` (`NamedTempFile` + `persist`).
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)

	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
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
