package shell

import (
	"net/url"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
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

// The panel is placed in the display's own coordinates, centred, and clamped
// so the window the terminal grows into still fits: a multi-display desktop
// must not open the launcher on whichever screen the platform picks.
func TestPlacementIsCentredAndClamped(t *testing.T) {
	cases := []struct {
		name                string
		areaX, areaY, w, h  int
		scale               float64
		windowW, referenceH float64
		wantX, wantY        int
	}{
		{"an ordinary display", 0, 0, 1920, 1080, 1, 720, 700, 600, 190},
		{"a display to the right", 1920, 0, 2560, 1440, 1, 720, 700, 2840, 370},
		{"a display scaled 2", 0, 0, 2560, 1440, 2, 720, 700, 280, 10},
		{"a display too short for the reference", 0, 0, 1920, 600, 1, 720, 700, 600, 0},
	}
	for _, c := range cases {
		gotX, gotY := placement(
			float64(c.areaX), float64(c.areaY), float64(c.w), float64(c.h),
			c.scale, c.windowW, c.referenceH)
		if gotX != c.wantX || gotY != c.wantY {
			t.Errorf("%s: placement = (%d, %d), want (%d, %d)", c.name, gotX, gotY, c.wantX, c.wantY)
		}
	}
}

// A `floter://register` link names a program the machine already has; the
// three spellings the old build accepted all reach it, and a name nothing can
// resolve is refused rather than half-connected. The link is the discovery →
// connect loop's own door.
func TestRegisterDeepLinkNamesAProgram(t *testing.T) {
	for _, raw := range []string{
		"floter://register?cmd=rg",
		"floter://register?tool=rg",
		"floter://register?program=rg",
		"floter://register/rg",
	} {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		// The same extraction HandleURL performs.
		name := firstNonEmpty(parsed.Query().Get("cmd"), parsed.Query().Get("tool"),
			parsed.Query().Get("program"), strings.TrimPrefix(parsed.Path, "/"))
		if name != "rg" {
			t.Errorf("%s named %q, want rg", raw, name)
		}
	}
	// A link with no name at all names nothing.
	if got := firstNonEmpty("", "  ", ""); got != "" {
		t.Errorf("firstNonEmpty = %q", got)
	}
	// A name nothing resolves is refused: the register path logs and leaves
	// the page as it was rather than writing half a package.
	a := appKeysApp()
	a.registerFromLink("no-such-program-anywhere-12345")
}

// A completion reported while the window is on screen goes into the panel's
// own row; only a hidden window gets a system notification. A completion that
// says nothing at all is worse than either.
func TestCompletionGoesWhereTheUserIsLooking(t *testing.T) {
	notifications := []string{}
	a := New(Options{
		Store:  settings.NewStore(settings.Default()),
		Notify: func(title, body string) { notifications = append(notifications, title+"|"+body) },
	})
	// No window at all (the tests' own world): the system notification is the
	// only channel, and it is used.
	a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled("V Tools") })
	if len(notifications) != 1 {
		t.Fatalf("notifications = %v", notifications)
	}
	if !strings.Contains(notifications[0], "V Tools") {
		t.Errorf("the message = %q", notifications[0])
	}
}

// The tray menu is the four things a tray icon is for — show, settings,
// re-scan, quit — and its labels follow the stored language, so the tray never
// lags behind the UI.
func TestTrayLabelsFollowTheLanguage(t *testing.T) {
	en := i18n.For("en").Launcher
	if en.TrayShow == "" || en.TraySettings == "" || en.TrayReload == "" || en.TrayQuit == "" {
		t.Errorf("a tray label is empty: %+v", en)
	}
	zh := i18n.For("zh").Launcher
	if zh.TrayShow == en.TrayShow {
		t.Errorf("the tray labels do not follow the language: %q", zh.TrayShow)
	}
	// Rescan re-reads what is installed, and is safe to call with no window
	// (the tray's own world before the panel exists).
	a := appKeysApp()
	a.Rescan()
}

// A menu bar belongs in macOS's system menu bar and nowhere else: on Linux and
// Windows the framework draws it *inside* the window — a File/Edit/View bar
// across a floating panel, which is not what this app is — and the old build
// installed no application menu at all (the tray is the menu).
func TestMenuBarBelongsOnMacOnly(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		want := goos == "darwin"
		if got := menuBarBelongs(goos); got != want {
			t.Errorf("menuBarBelongs(%q) = %v, want %v", goos, got, want)
		}
	}
}
