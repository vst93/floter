package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/settingsui"
)

// settingsEscapeApp builds a shell whose settings page shows one installed
// integration and nothing else, so the removal bar is on screen without
// scrolling. The detected-tools list would otherwise fill the panel.
func settingsEscapeApp() *App {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	a.Settings.Actions.UninstallComponents = nil
	a.Settings.Actions.UninstallIntegration = func(string, string) {}
	a.Settings.Actions.Detected = nil
	a.Settings.Actions.ConnectDetected = nil
	a.Settings.Integrations = func() []settingsui.Integration {
		return []settingsui.Integration{{ID: "x", Name: "X Tools", Enabled: true}}
	}
	return a
}

// The settings page's removal bar owns Escape before the panel does. The
// shell's window-level handler runs before the surface view is built, so a
// literal Escape claim there would outrun the bar's own cancel — the
// reference's RemovalConfirmation installed a capture-phase listener with
// `stopImmediatePropagation` for exactly this. The first Escape answers the
// question and stays on the page; a second, with nothing left to answer,
// closes the panel.
func TestEscapeAnswersTheSettingsQuestionFirst(t *testing.T) {
	a := settingsEscapeApp()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 1600)
	tt.Frame()
	a.Open(SurfaceSettings)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("the integrations page: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Remove package"); err != nil {
		t.Fatalf("the uninstall control: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("Remove X Tools?") {
		t.Fatalf("the removal bar did not open: %v", tt.Texts())
	}

	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.Surf != SurfaceSettings {
		t.Errorf("Escape left the settings panel: surf = %v", a.Surf)
	}
	if tt.HasText("Remove X Tools?") {
		t.Errorf("Escape left the question up: %v", tt.Texts())
	}

	// With the question answered, the next Escape is the panel's way out.
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("the second Escape left surf = %v, want the launcher", a.Surf)
	}
}

// With nothing to answer, the settings panel's own Escape is its way out.
func TestEscapeClosesTheSettingsWhenNothingAsks(t *testing.T) {
	a := settingsEscapeApp()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 1600)
	tt.Frame()
	a.Open(SurfaceSettings)
	tt.Frame()

	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("Escape left surf = %v, want the launcher", a.Surf)
	}
}
