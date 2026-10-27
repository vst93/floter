//go:build darwin

package apps

import (
	"os/exec"
	"strings"
)

// sipsPath is the converter the reference runs (`/usr/bin/sips`,
// `main:src-tauri/src/commands/apps/macos.rs:520`). It is on every macOS
// install, and it is the only thing that reads an `.icns` container's
// pre-10.7 bitmap entries.
const sipsPath = "/usr/bin/sips"

// extractIcon is the platform's own extraction: a bundle whose `.icns` holds
// only the bitmap entry types this build's reader skips is converted by sips,
// which is what the reference does for **every** icon. This build keeps its own
// reader as the fast path (a PNG entry needs no process at all) and asks sips
// only for the file that reader refused.
func extractIcon(app App) []byte {
	if !strings.HasSuffix(strings.ToLower(app.IconPath), ".icns") {
		return nil
	}
	return convertICNS(app.IconPath, func(args []string) error {
		return exec.Command(sipsPath, args...).Run()
	})
}

// deferredIcon reports whether an application's icon needs a platform
// extraction rather than a file this build can read.
//
// It stays false: the launch above happens only for a bundle whose icon this
// build's reader refused, and the answer is cached per path, so a scan of five
// hundred applications pays it for at most the handful of pre-10.7 bundles. A
// Windows shortcut is the other case, where *every* row needs a launch — that
// is what the background extraction exists for.
func deferredIcon(App) bool { return false }
