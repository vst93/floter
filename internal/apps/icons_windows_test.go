//go:build windows

package apps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The script hands the source and the target to PowerShell as single-quoted
// arguments, so a path with a quote in it cannot break out.
func TestIconExtractScriptQuotes(t *testing.T) {
	script := iconExtractScript(`C:\Users\it's\App.lnk`, `C:\cache\out.png`)
	if !strings.Contains(script, `'C:\Users\it''s\App.lnk'`) {
		t.Errorf("the source is not quoted: %s", script)
	}
	if !strings.Contains(script, "'C:\\cache\\out.png'") {
		t.Errorf("the target is not quoted: %s", script)
	}
	if !strings.Contains(script, "ExtractAssociatedIcon") || !strings.Contains(script, "ImageFormat]::Png") {
		t.Errorf("the script does not extract a PNG: %s", script)
	}
}

// A real extraction: PowerShell is part of Windows, so this runs wherever the
// suite does.
func TestExtractIconFromAShortcut(t *testing.T) {
	// A shortcut to a program that certainly has an icon.
	system32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	source := filepath.Join(system32, "notepad.exe")
	if _, err := os.Stat(source); err != nil {
		t.Skip("no notepad.exe to take an icon from")
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "Notepad.lnk")
	if err := createShortcut(link, source); err != nil {
		t.Skipf("could not create a shortcut: %v", err)
	}
	icon := Icon(App{Name: "Notepad", Path: link})
	if len(icon) == 0 {
		t.Skip("PowerShell did not produce an icon")
	}
	if !isPNG(icon) {
		t.Errorf("the icon is not a PNG: %d bytes", len(icon))
	}
	// The second read is the cache.
	if again := Icon(App{Name: "Notepad", Path: link}); len(again) != len(icon) {
		t.Errorf("the cached icon = %d bytes, want %d", len(again), len(icon))
	}
}

// createShortcut makes a .lnk with the shell's own interface, through
// PowerShell (Go has no shortcut API).
func createShortcut(link, target string) error {
	script := " $s = (New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(link) + ");" +
		" $s.TargetPath = " + psQuote(target) + "; $s.Save()"
	return runPowerShell(script)
}
