package apps

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDesktopFileWithBOMIsStillAnApp pins the one file-framing byte that made
// the two `.desktop` readers disagree: `internal/inventory` stripped a leading
// UTF-8 BOM, this one did not, and because U+FEFF is not whitespace
// (`strings.TrimSpace` leaves it alone) the first line read as
// "\ufeff[Desktop Entry]" — not the section header — so `inEntry` stayed false
// and the whole application vanished from the launcher.
//
// A Windows editor writes that BOM, so the same file that shows up in the
// tools list showed up in no application list at all.
func TestDesktopFileWithBOMIsStillAnApp(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"bom only", "\ufeff[Desktop Entry]\nType=Application\nName=Firefox\nExec=firefox %u\n"},
		{"bom and crlf", "\ufeff[Desktop Entry]\r\nType=Application\r\nName=Firefox\r\nExec=firefox %u\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "firefox.desktop")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			app, ok := ParseDesktopFile(path)
			if !ok {
				t.Fatalf("ParseDesktopFile rejected a BOM'd entry")
			}
			if app.Name != "Firefox" {
				t.Fatalf("Name = %q, want Firefox", app.Name)
			}
			if len(app.Exec) == 0 || app.Exec[0] != "firefox" {
				t.Fatalf("Exec = %q, want it to start with firefox", app.Exec)
			}
		})
	}
}
