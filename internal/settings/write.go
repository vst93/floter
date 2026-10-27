package settings

import (
	"os"

	"floter/internal/statefile"
)

// writeFileAtomic writes data to path by way of a temporary file in the same
// directory: create, write, sync, close, chmod, then rename over path. A
// reader therefore sees either the old file or the new one, never a truncated
// one, and a crash between the writes cannot leave a half-written JSON file
// behind. It is the Go counterpart of the old build's
// `write_settings_file` (`NamedTempFile` + `persist`).
//
// P11-k: the body is `statefile.Write`, the one implementation every state
// file in this app now goes through. The function stays as the settings
// package's own name for it, because the two-file contract above (backup, then
// primary, both 0600) is `Save`'s and this is the half that has to be atomic.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	return statefile.Write(path, data, mode)
}
