package shell

import (
	"strconv"
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
	// `detachedSurfaces` is the process's own table: a test that pins a page
	// leaves it behind for the next one, which would then take the "raise the
	// window already showing this page" branch and never build its own.
	detachedMu.Lock()
	detachedSurfaces = nil
	detachedMu.Unlock()
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
	// The mark is on the window's own title bar, which a pinned window draws in
	// place of the system one (`newDetachedSurfaceWindow` hides it). That bar is
	// the window's chrome, not a band of the page: it sits inside the card's
	// face, above the page, and the page below it is untouched. What must not
	// happen is the page *itself* being pushed around by it — so the reading is
	// that the settings page's own header is still the first thing under the
	// bar, at the panel's own place.
	mark := i18n.For(store.Snapshot().Language).Launcher.DetachedMark
	if !tt.HasText(mark) {
		t.Errorf("the pinned window's title bar does not say it is pinned: %q", tt.Texts())
	}
	if got := strings.Count(strings.Join(tt.Texts(), "\x00"), mark); got != 1 {
		t.Errorf("the pinned page says %q %d times, want once (on the title bar)", mark, got)
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

// A pinned page has no way back to the search list.
//
// The user's rule: 「独立出来的窗口不应该能回退到搜索页，不能通过删除、esc 回退，
// 只能整个退出」. Three routes would have walked it back, and all three are shut:
// Escape's mode ladder, an empty field's Backspace (which walks the same
// ladder), and the page's own close action (which returns to the launcher).
// The window's close button is the only exit.
func TestAPinnedPageCannotWalkBackToTheSearchList(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher
	f.app.Launcher.EnterClipboard()
	f.app.Launcher.SetQuery("note")
	f.app.detachSurface()

	detachedMu.Lock()
	var second *detachedSurface
	for _, state := range detachedSurfaces {
		second = state
	}
	detachedMu.Unlock()
	if second == nil || second.launcher == nil {
		t.Fatal("no pinned launcher")
	}
	page := second.launcher

	// Escape: the mode ladder refuses, and so does the query clear behind it.
	tt := ui.NewTester(func(c *ui.Context) { page.View(c) }, 720, 560)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !page.PluginModeActive() {
		t.Error("Escape left the pinned page's mode")
	}
	if page.Query != "note" {
		t.Errorf("Escape changed the pinned page's field to %q", page.Query)
	}

	// Backspace on an empty field: the same ladder, so the same refusal.
	page.SetQuery("")
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if !page.PluginModeActive() {
		t.Error("Backspace on an empty field left the pinned page's mode")
	}

	// The page's close action does nothing while the pinned page is drawing.
	f.app.detachedDrawing = true
	f.app.surfaceExit()
	f.app.detachedDrawing = false
	if f.app.Surf != SurfaceLauncher {
		t.Errorf("the pinned page's close action moved the panel to %v", f.app.Surf.Name())
	}
}

// Pinning a page keeps the operation going: the pinned window opens on the same
// rows under the same choice.
//
// 「独立切换时要保留当前的操作结果，最好就是把插件本体独立出来，不影响接着操作」 —
// so `AdoptPage` carries the content across, not just the mode.
func TestAPinnedPageOpensWhereThePanelWas(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher
	f.app.Launcher.EnterClipboard()
	f.app.Launcher.SetQuery("note")
	f.app.Launcher.Selected = 1
	before := f.app.Launcher.Query

	f.app.detachSurface()

	detachedMu.Lock()
	var second *detachedSurface
	for _, state := range detachedSurfaces {
		second = state
	}
	detachedMu.Unlock()
	if second == nil || second.launcher == nil {
		t.Fatal("no pinned launcher")
	}
	if got := second.launcher.Query; got != before {
		t.Errorf("the pinned page's field holds %q, want the needle it was pinned with (%q)", got, before)
	}
	if got := second.launcher.Selected; got != 1 {
		t.Errorf("the pinned page starts on row %d, want the row the panel was on (1)", got)
	}
}

// Two plugin pages pin as two windows.
//
// The report was 「现在好像只能固定一个插件了」. Every plugin page — the
// clipboard, the browser, the calculator, the dropped files, an extension
// command — is the one `SurfaceLauncher` surface, and the first version keyed a
// pinned window on its *surface*, so the second plugin page looked like the
// first one and raised its window instead of opening its own. The key is the
// page's own identity now (see `launcher.PageKey`).
func TestTwoPluginPagesPinAsTwoWindows(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher

	f.app.Launcher.EnterClipboard()
	f.app.detachSurface()
	if len(f.pinned) != 1 {
		t.Fatalf("pinned surfaces after the first page = %v, want one", f.pinned)
	}

	f.app.Launcher.EnterCalculator()
	f.app.detachSurface()
	if len(f.pinned) != 2 {
		t.Fatalf("pinned surfaces after the second page = %v, want two — the second plugin "+
			"page raised the first one's window instead of opening its own", f.pinned)
	}

	detachedMu.Lock()
	keys := make([]string, 0, len(detachedSurfaces))
	for _, state := range detachedSurfaces {
		keys = append(keys, state.key)
	}
	detachedMu.Unlock()
	if len(keys) != 2 || keys[0] == keys[1] {
		t.Errorf("the two pinned pages are keyed %v, want two different keys", keys)
	}
}

// Pinning the same page twice is a raise, not a second window: two windows for
// one page are two windows the user cannot tell apart.
func TestPinningTheSamePageTwiceRaisesIt(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher
	f.app.Launcher.EnterClipboard()
	f.app.detachSurface()
	f.app.Launcher.EnterClipboard()
	f.app.detachSurface()
	if len(f.pinned) != 1 {
		t.Errorf("pinning the clipboard page twice opened %d windows, want one", len(f.pinned))
	}
}

// The pin ceiling is spoken, not silent.
//
// The user asked for a limit and a message with it — 「超过上限的时候给到提示」 —
// because a key that opens nothing reads as a broken key rather than as a
// reached limit.
func TestThePinCeilingRefusesAndSaysSo(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher

	// Fill the table without going through the windows: this test is about the
	// ceiling, not about which page each one shows.
	detachedMu.Lock()
	detachedSurfaces = nil
	for i := 0; i < MaxDetachedSurfaces; i++ {
		detachedSurfaces = append(detachedSurfaces,
			&detachedSurface{key: "page-" + strconv.Itoa(i)})
	}
	detachedMu.Unlock()
	if got := f.app.countDetachedSurfaces(); got != MaxDetachedSurfaces {
		t.Fatalf("the table holds %d pages, want %d", got, MaxDetachedSurfaces)
	}

	f.app.Launcher.EnterClipboard()
	f.app.detachSurface()

	if len(f.pinned) != 0 {
		t.Errorf("the ceiling opened %d more windows, want none", len(f.pinned))
	}
	copy := i18n.For(f.app.Store.Snapshot().Language).Launcher
	if !f.app.Launcher.FeedbackVisible() {
		t.Error("the refusal is silent: nothing says the ceiling was reached")
	}
	want := copy.DetachLimit(MaxDetachedSurfaces)
	got, warning := f.app.Launcher.FeedbackText()
	if got != want {
		t.Errorf("the panel says %q, want %q", got, want)
	}
	if !warning {
		t.Error("the ceiling's sentence is drawn as a note; a refusal is a warning")
	}
}

// The pinned page's title bar is as tall as the window's controls, never taller.
//
// The report was 「现在 Mac 下有点过高了，导致系统自带的红绿灯栏和标题并没有完全上下
// 居中」. The window server places the traffic lights at the system height; a row
// drawn taller leaves them above its centre. The first version took
// `max(bar.Height, Space(9))`, and `Space(9)` is 32.4px at the shipped step
// against macOS's 28 — so the row was 4.4px too tall on exactly the platform the
// user reported.
func TestTheTitleBarIsAsTallAsTheWindowsControls(t *testing.T) {
	for _, tc := range []struct {
		name string
		bar  ui.TitleBar
	}{
		{"macOS inset traffic lights", ui.TitleBar{Height: 28, Left: 78}},
		{"windows", ui.TitleBar{Height: 32, Right: 138}},
		{"linux", ui.TitleBar{Height: 32, Right: 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got float32
			tt := ui.NewTester(func(c *ui.Context) {
				got = detachedTitleBarHeight(c.TitleBar(), c.Theme())
			}, 720, 560)
			tt.SetTitleBar(tc.bar)
			tt.Frame()
			if got != tc.bar.Height {
				t.Errorf("the title bar is %v tall, want the window's own %v — the controls "+
					"are placed at the system height and a taller row leaves them off centre",
					got, tc.bar.Height)
			}
		})
	}

	// A frame with no title bar still gets a usable row rather than a zero-height
	// one: the fallback is the platform's minimum, not a design number.
	var fallback float32
	tt := ui.NewTester(func(c *ui.Context) {
		fallback = detachedTitleBarHeight(ui.TitleBar{}, c.Theme())
	}, 720, 560)
	tt.Frame()
	if fallback <= 0 {
		t.Errorf("a frame with no title bar draws a %v-tall row", fallback)
	}
}

// A pinned page moves the page; it does not edit it.
//
// 「切换独立之后要保持之前的交互结果，也就是除了形式变化，其他东西应该都不动」 — the
// caret included. The first version set `pendingCaret`, which put the caret at
// the end of the field on the pinned page's first frame: a visible change to a
// page the user only asked to move.
func TestAPinnedPageMovesThePageWithoutEditingIt(t *testing.T) {
	f := newDetachFixture(t)
	f.app.Surf = SurfaceLauncher
	f.app.Launcher.EnterClipboard()
	f.app.Launcher.SetQuery("note")
	f.app.Launcher.History = []string{"older", "newest"}
	f.app.Launcher.Selected = 2

	f.app.detachSurface()

	detachedMu.Lock()
	var second *detachedSurface
	for _, state := range detachedSurfaces {
		second = state
	}
	detachedMu.Unlock()
	if second == nil || second.launcher == nil {
		t.Fatal("no pinned launcher")
	}
	page := second.launcher
	if page.Query != "note" {
		t.Errorf("the pinned page's field holds %q, want the needle", page.Query)
	}
	if page.Selected != 2 {
		t.Errorf("the pinned page starts on row %d, want the row the panel was on (2)", page.Selected)
	}
	// The caret stays where it was: no pending re-placement on the next frame.
	tt := ui.NewTester(func(c *ui.Context) { page.View(c) }, 720, 560)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if page.Query != "note" {
		t.Errorf("the first frame changed the pinned page's field to %q", page.Query)
	}
}
