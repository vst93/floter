package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-hk · the plugin sheet's editor state belongs to one open.
//
// The settings panel is one long-lived App (the shell keeps it for the whole
// run) and the launcher draws every plugin's sheet through it. The sheet's
// scroll offset and its half-typed fields live on that App so a redraw does
// not lose them — but nothing retired them when the overlay closed, so the
// next open inherited the abandoned state. The scroll offset is the reading
// the shell can see without reaching into the panel's private draft maps.

// pluginConfigSheetTester opens the clipboard plugin's sheet in the launcher.
func pluginConfigSheetTester(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 1400)
	tt.Frame()
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if err := tt.Click(i18n.For("en").Launcher.ConfigurePlugin); err != nil {
		t.Fatalf("the gear: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if !a.Launcher.ConfigOpen() {
		t.Fatal("the sheet did not open")
	}
	return tt
}

// reopenPluginConfigSheet opens the sheet again on the same frame, after the
// gear closed it.
func reopenPluginConfigSheet(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(i18n.For("en").Launcher.ConfigurePlugin); err != nil {
		t.Fatalf("the gear after closing: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if !a.Launcher.ConfigOpen() {
		t.Fatal("the sheet did not reopen")
	}
}

// Closing the sheet retires the state it was holding, so the next open starts
// at the top over the stored values.
func TestClosingThePluginSheetRetiresItsEditorState(t *testing.T) {
	a := pluginConfigApp(t)
	tt := pluginConfigSheetTester(t, a)
	// Scroll the sheet, then close it with its own gear.
	a.Settings.PluginScroll.Y = 42
	if err := tt.Click(i18n.For("en").Launcher.ConfigurePluginClose); err != nil {
		t.Fatalf("the closing gear: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if a.Launcher.ConfigOpen() {
		t.Fatal("the gear left the sheet open")
	}
	if a.Settings.PluginScroll.Y != 0 {
		t.Errorf("the closed sheet kept its scroll offset: Y = %v", a.Settings.PluginScroll.Y)
	}
	reopenPluginConfigSheet(t, a, tt)
	if a.Settings.PluginScroll.Y != 0 {
		t.Errorf("the reopened sheet started at Y = %v, want the top", a.Settings.PluginScroll.Y)
	}
}

// The same on the Escape path: the key closes the sheet and the state with it.
func TestEscapeClosingThePluginSheetRetiresItsEditorState(t *testing.T) {
	a := pluginConfigApp(t)
	tt := pluginConfigSheetTester(t, a)
	a.Settings.PluginScroll.Y = 42
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.Launcher.ConfigOpen() {
		t.Fatal("Escape left the sheet open")
	}
	if a.Settings.PluginScroll.Y != 0 {
		t.Errorf("the escaped sheet kept its scroll offset: Y = %v", a.Settings.PluginScroll.Y)
	}
}
