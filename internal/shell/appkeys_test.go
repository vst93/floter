package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// parseShortcut reads one action's stored accelerator into the tester's
// key form.
func parseShortcut(t *testing.T, action string) (ui.Modifiers, ui.Key, bool) {
	t.Helper()
	m, k, ok := shortcuts.Parse(settings.Shortcut(settings.Normalize(settings.Default()), action))
	return ui.Modifiers(m), ui.Key(k), ok
}

func appKeysApp() *App {
	return New(Options{Store: settings.NewStore(settings.Default())})
}

func appKeysRender(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	return tt
}

// The old build's dismiss table, surface by surface. The literal ⌘W is
// the same key as the new-command binding by default, so the launcher's
// and the settings' rows answer it; the terminal's row does not — Ctrl+W
// is the running shell's delete-word.
func TestDismissTable(t *testing.T) {
	// Launcher: ⌘W hides.
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("the launcher's ⌘W changed the surface")
	}
	// Terminal: the default new-command binding IS ⌘W, and its rule wins —
	// the press returns to the launcher, as the old build's table did (the
	// shell's own delete-word yields to the configured chord).
	a = appKeysApp()
	a.Open(SurfaceTerminal)
	tt = appKeysRender(t, a)
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("the terminal's ⌘W left surf=%v, want launcher", a.Surf)
	}
	// Settings: ⌘W closes settings, back to the launcher.
	a = appKeysApp()
	a.Open(SurfaceSettings)
	tt = appKeysRender(t, a)
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("settings ⌘W left surf=%v, want launcher", a.Surf)
	}
}

// The new-command binding on the terminal returns to the launcher with an
// empty field, the way the old build's returnToInputMode did.
func TestNewCommandReturnsToTheLauncher(t *testing.T) {
	a := appKeysApp()
	a.Open(SurfaceTerminal)
	tt := appKeysRender(t, a)
	mods, key, ok := parseShortcut(t, settings.ShortcutNewCommand)
	if !ok {
		t.Fatal("the new command shortcut did not parse")
	}
	tt.Key(mods, key)
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("surf = %v, want launcher", a.Surf)
	}
	if a.Launcher.Query != "" {
		t.Errorf("query = %q, want empty", a.Launcher.Query)
	}
}

// The terminal's own new-command row does the same: the shortcut answers
// wherever the user is looking.
func TestTerminalNewCommandRowReturns(t *testing.T) {
	a := appKeysApp()
	a.Open(SurfaceTerminal)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	if err := tt.Click("New command"); err != nil {
		t.Fatalf("row: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if a.Surf != SurfaceLauncher {
		t.Errorf("surf = %v, want launcher", a.Surf)
	}
}
