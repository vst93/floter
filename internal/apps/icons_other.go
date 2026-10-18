//go:build !windows

package apps

// extractIcon is the platform's own extraction, for an application whose icon
// is not a file this build can read. macOS and Linux name a file (see
// Info.plist and the desktop entry), so there is nothing to extract.
func extractIcon(App) []byte { return nil }

// deferredIcon reports whether an application's icon needs a platform
// extraction rather than a file this build can read. macOS and Linux name a
// file, so nothing here is deferred.
func deferredIcon(App) bool { return false }
