package shell

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// detachFixture builds a shell whose pinned-page window is recorded rather than
// opened, and reports every surface that was pinned.
type detachFixture struct {
	app    *App
	pinned []Surface
}

func newDetachFixture(t *testing.T) *detachFixture {
	t.Helper()
	f := &detachFixture{}
	f.app = New(Options{
		Store: settings.NewStore(settings.Default()),
		OpenDetachedSurface: func(surf Surface) *mygo.Window {
			f.pinned = append(f.pinned, surf)
			return nil
		},
	})
	return f
}

// A pinned page is a page the user can keep: the settings, a terminal session,
// and the launcher's own plugin pages. The search list is not — it is the app's
// home, and pinning it would leave the panel with nothing.
func TestOnlyPagesCanBePinnedNotTheSearchList(t *testing.T) {
	for _, tc := range []struct {
		name  string
		surf  Surface
		setup func(a *App)
		want  bool
	}{
		{"search list", SurfaceLauncher, nil, false},
		{"typed query", SurfaceLauncher, func(a *App) { a.Launcher.SetQuery("term") }, false},
		{"clipboard page", SurfaceLauncher, func(a *App) { a.Launcher.EnterClipboard() }, true},
		{"settings", SurfaceSettings, nil, true},
		{"terminal", SurfaceTerminal, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDetachFixture(t)
			f.app.Surf = tc.surf
			if tc.setup != nil {
				tc.setup(f.app)
			}
			if got := f.app.canDetachSurface(); got != tc.want {
				t.Errorf("canDetachSurface() on %s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// Pinning a page splits the app in two: the page goes to its own window and the
// panel returns to the search list. That split is the whole feature — the user
// asked for 「页面将不再自动消失，而是固定在位置上进行展示，相当于和主窗口进行
// 分割」.
func TestPinningTheSettingsSendsThePanelBackToTheSearchList(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceSettings
	f.app.detachSurface()

	if len(f.pinned) != 1 || f.pinned[0] != SurfaceSettings {
		t.Fatalf("pinned surfaces = %v, want the settings once", f.pinned)
	}
	if f.app.Surf != SurfaceLauncher {
		t.Errorf("the panel is on %v, want the search list — the page moved to its own window",
			f.app.Surf.Name())
	}
}

// A terminal session is pinned without the terminal's own plumbing knowing
// about windows: the same `Terminal` App is drawn by the pinned window, so the
// PTY, the scrollback and the session list are untouched (the user's 「终端也
// 底层逻辑不改」).
func TestPinningTheTerminalKeepsTheSameSession(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceTerminal
	before := f.app.Terminal
	f.app.detachSurface()

	if len(f.pinned) != 1 || f.pinned[0] != SurfaceTerminal {
		t.Fatalf("pinned surfaces = %v, want the terminal once", f.pinned)
	}
	if f.app.Terminal != before {
		t.Error("pinning the terminal replaced the terminal App; the session would have been lost")
	}
}

// Pinning a plugin page needs a second launcher: the panel's own goes back to
// the search list, and one launcher's mode is a single value. The second
// instance adopts the page — its mode and its field text — and starts at its
// own first row.
func TestPinningAPluginPageAdoptsItIntoASecondLauncher(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher
	f.app.Launcher.EnterClipboard()
	f.app.Launcher.SetQuery("note")
	panel := f.app.Launcher

	f.app.detachSurface()

	if len(f.pinned) != 1 || f.pinned[0] != SurfaceLauncher {
		t.Fatalf("pinned surfaces = %v, want the launcher page once", f.pinned)
	}
	// The panel gave the page up…
	if panel.PluginModeActive() {
		t.Error("the panel is still in the plugin mode it just pinned")
	}
	// …and the second instance has it.
	detachedMu.Lock()
	var second *detachedSurface
	for _, state := range detachedSurfaces {
		second = state
	}
	detachedMu.Unlock()
	if second == nil || second.launcher == nil {
		t.Fatal("no second launcher was created for the pinned page")
	}
	if !second.launcher.PluginModeActive() {
		t.Error("the pinned launcher is not in the plugin mode it was pinned from")
	}
	if got := second.launcher.Query; got != "note" {
		t.Errorf("the pinned page's field holds %q, want the needle it was pinned with", got)
	}
}

// The pin's own chord is the user's to change, and the page it pins is the page
// the app is on. This is the keyboard half of the feature: the reference had no
// binding for it (`main:src/App.tsx:976-988` opened the window from a button),
// so the key is this build's own and must be rebindable like every other.
func TestTheDetachChordIsRebindable(t *testing.T) {
	store := settings.NewStore(settings.Default())
	if got := settings.Shortcut(store.Snapshot(), settings.ShortcutDetachSurface); got != settings.DefaultDetachShortcut() {
		t.Errorf("the shipped detach chord is %q, want %q", got, settings.DefaultDetachShortcut())
	}
	if err := store.Update(func(s *settings.Settings) {
		if !s.SetShortcut(settings.ShortcutDetachSurface, "Cmd+Shift+P") {
			t.Fatal("the detach chord refused a rebind")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if got := settings.Shortcut(store.Snapshot(), settings.ShortcutDetachSurface); got != "Cmd+Shift+P" {
		t.Errorf("after rebinding the detach chord is %q", got)
	}
}

// A pinned page is the panel's own card in a window of its own, and the mark
// that says so costs the page nothing.
//
// The user's correction was 「我希望的是分离之后和原来的浮窗几乎保持一致，只有少量
// 的标识来区分是主窗口还是独立窗口，整体的样式、布局以及交互应该保持不变」. The
// first version failed exactly that: it put a bar above the page, which moved
// the page down and competed with the page's own header. The mark now rides the
// window's title (`已固定 · Settings · floter`) and the card's own stroke, which
// is the accent on a pinned window and the input stroke on the panel's — one
// line of difference, and no layout of the page touched at all.
func TestAPinnedPageKeepsThePanelsShapeAndOnlyItsStrokeChanges(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(Options{Store: store})
	state := &detachedSurface{surf: SurfaceSettings}
	tt := ui.NewTester(func(c *ui.Context) {
		a.detachedSurfaceView(c, state)
	}, 720, 560)
	tt.Frame()
	tt.Frame()

	// The page is the settings page, whole: its own header, its own rows. No
	// band of ours stands between the window's top edge and the page.
	if !tt.HasText("General") || !tt.HasText("Appearance") {
		t.Fatalf("the pinned page is not the settings page: %q", tt.Texts())
	}
	mark := i18n.For(store.Snapshot().Language).Launcher.DetachedMark
	if tt.HasText(mark) {
		t.Errorf("the pinned page draws a band of its own (%q); the mark belongs to the "+
			"window's title and the card's stroke, not to the page's layout", mark)
	}

	// The window's title carries the mark, and it is the only place the user
	// reads it in the page's own text.
	title := a.detachedWindowTitle(SurfaceSettings, nil)
	if !strings.HasPrefix(title, mark) {
		t.Errorf("the pinned window's title is %q, want it to start with %q", title, mark)
	}
	if !strings.Contains(title, i18n.For(store.Snapshot().Language).Settings.Title) {
		t.Errorf("the pinned window's title is %q, want it to name the page", title)
	}
}
