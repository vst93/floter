//go:build windows

package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// A Windows shortcut keeps its icon inside the shell's own database, so the
// only way to a PNG is to ask the shell: PowerShell's
// `[System.Drawing.Icon]::ExtractAssociatedIcon` is that ask. The result is
// cached as a PNG under the user's cache directory, keyed by the shortcut's
// path and its modification time, so a redraw costs one file read and a
// changed shortcut is re-read.

// iconExtractScript is the PowerShell program that extracts one icon. The
// source path and the target file are appended as arguments, so a path with a
// quote in it cannot break the script.
func iconExtractScript(source, target string) string {
	return "Add-Type -AssemblyName System.Drawing; " +
		"$icon = [System.Drawing.Icon]::ExtractAssociatedIcon(" + psQuote(source) + "); " +
		"if (-not $icon) { exit 1 }; " +
		"$icon.ToBitmap().Save(" + psQuote(target) + ", [System.Drawing.Imaging.ImageFormat]::Png)"
}

// psQuote quotes a value for a PowerShell single-quoted string, where a quote
// is doubled.
func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// extractIcon extracts a shortcut's icon into the cache.
func extractIcon(app App) []byte {
	if app.Path == "" {
		return nil
	}
	target := windowsIconCachePath(app)
	if data, err := os.ReadFile(target); err == nil && isPNG(data) {
		return data
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil
	}
	script := iconExtractScript(app.Path, target)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Run(); err != nil {
		return nil
	}
	data, err := os.ReadFile(target)
	if err != nil || !isPNG(data) {
		return nil
	}
	return data
}

// runPowerShell runs one script, for the tests and the shortcut helper.
func runPowerShell(script string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return cmd.Run()
}

// windowsIconCachePath is where a shortcut's icon is cached: the user's cache
// directory, the app's own folder, a name built from the shortcut's path and
// its modification time (so a changed shortcut does not keep the old icon).
func windowsIconCachePath(app App) string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	stamp := ""
	if info, err := os.Stat(app.Path); err == nil {
		stamp = info.ModTime().UTC().Format("20060102150405")
	}
	sum := sha256.Sum256([]byte(app.Path + "|" + stamp))
	return filepath.Join(base, "floter", "icons", hex.EncodeToString(sum[:8])+".png")
}
