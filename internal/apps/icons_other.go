//go:build !windows && !darwin

package apps

// extractIcon is the platform's own extraction, for an application whose icon
// is not a file this build can read. Linux names a file in the desktop entry
// (`Icon=`), and this build resolves it to a PNG, so there is nothing to
// extract. macOS is the other case — an `.icns` that holds only the pre-10.7
// bitmap entries — and it has its own file (`icons_darwin.go`).
func extractIcon(App) []byte { return nil }

// deferredIcon reports whether an application's icon needs a platform
// extraction rather than a file this build can read. Linux names a file, so
// nothing here is deferred.
func deferredIcon(App) bool { return false }
