package shell

import (
	"runtime"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
	apptheme "floter/internal/theme"
)

// The detached page: the surface the app is on, pinned into a window of its
// own.
//
// The user asked for this on every page but the search one — 「除了最基础的搜索
// 主页之外，其他的插件页和功能（包括终端页）应该要有“分离”功能，或者叫“盯住”
// 的功能」 — with two requirements: the page stops vanishing when the panel does
// (「页面将不再自动消失，而是固定在位置上进行展示，相当于和主窗口进行分割」),
// and the pinned window says so on its own face (「这个分离出来的窗口样式可以做
// 一些特殊的标记或特效，让用户感受到这是被分离或者固定住的」).
//
// It is built on the *same* state, not on a second copy of the app: the one
// App keeps holding `Settings`, `Terminal` and `Launcher`, and the two windows
// draw different surfaces of it. That is why a terminal session keeps running
// in the pinned window without any of the PTY plumbing knowing about windows at
// all — the user's 「终端也底层逻辑不改」 — and why a settings draft or a
// launcher query does not have to be mirrored between two apps.
//
// What it is *not* is a copy of `detach.go`. That window owns one captured run
// and re-runs its command; this one owns a **surface** and draws whatever that
// surface is showing right now. The two share the shape every second window in
// this app has: an ordinary window (so no panel hide-on-blur), geometry
// remembered per content, and a `tokens.Opaque` root because the pinned window
// is a solid page rather than a pane over the desktop.

// detachedSurface is one pinned page: which page it shows, and its window.
type detachedSurface struct {
	look themeFrame
	// key is the page's own identity — what makes two pinned pages different.
	//
	// It is *not* the surface. The settings and a terminal session are each one
	// surface, but every plugin page is `SurfaceLauncher`, so keying on the
	// surface made the second plugin page look like the first one and raise it
	// instead of opening its own window (the report 「现在好像只能固定一个插件
	// 了」). The key is the plugin's own id for those, and the surface's name for
	// the two pages that are one-of-a-kind.
	key string
	// surf is the surface the window draws. It is fixed when the window opens:
	// a pinned page does not follow the panel to another page, which is the
	// whole point of pinning it.
	surf Surface
	win  *mygo.Window
	// launcher is the second launcher instance a pinned *plugin page* draws.
	// The panel's own launcher is showing the search page by then, and one
	// launcher App cannot draw two different pages at once — its mode is a
	// single value. Nil for the settings and the terminal, whose Apps are not
	// shared with the panel's page.
	launcher *launcher.App
	// closing marks the window being closed by us, so `OnClosed` does not try
	// to forget it twice.
	closing bool
}

// detachedSurfaces is every pinned page, guarded because a window's close
// callback arrives from the platform's own thread.
//
// A slice, not a map keyed on the window: the window is the *result* of pinning,
// not the page's identity, and two pages pinned before either window exists (a
// headless frame, a test) would collide on a nil key. The page's own key is what
// tells two entries apart (see `detachedSurface.key`).
var (
	detachedMu       sync.Mutex
	detachedSurfaces []*detachedSurface
)

// detachedWindowTitle is what the window manager calls a pinned page:
// `已固定 · Settings · floter`.
//
// The mark rides the window's own title bar rather than a band inside the page,
// which is the whole of the user's correction — 「我希望的是分离之后和原来的浮窗
// 几乎保持一致，只有少量的标识来区分…整体的样式、布局以及交互应该保持不变」. A
// band above the page was a second piece of chrome the panel never had: it moved
// the page down, it competed with the page's own header, and it made the pinned
// window a different thing to look at rather than the same thing in a different
// place. The title bar is chrome the window already has, so the mark costs the
// page nothing.
func (a *App) detachedWindowTitle(surf Surface, alt *launcher.App) string {
	mark := i18n.For(a.Store.Snapshot().Language).Launcher.DetachedMark
	name := strings.TrimSpace(a.surfaceTitle(surf))
	if alt != nil {
		// A pinned plugin page is named by the plugin that owns it; the panel's
		// own launcher has already gone back to its search list and no longer
		// knows which page it was showing.
		if title := strings.TrimSpace(alt.PluginTitle()); title != "" {
			name = title
		}
	}
	return mark + detachedTitleSeparator + name + detachWindowTitleSuffix
}

// detachedTitleSeparator is the middle dot the app's other window titles use
// (`detachWindowTitleSuffix`), so a pinned page reads as one more title in the
// same family rather than a new shape.
const detachedTitleSeparator = " \u00b7 "

// detachedSurfaceStateKey is the key a pinned page's geometry is remembered
// under: one per *page*, so pinning the settings, a terminal session and two
// different plugin pages each open where the user last left them.
func detachedSurfaceStateKey(key string) string {
	return settings.DetachedStateKeyPrefix + "surface\x00" + key
}

// surfaceTitle is what a pinned page calls itself: the surface's own name in
// the user's language.
func (a *App) surfaceTitle(surf Surface) string {
	copy := i18n.For(a.Store.Snapshot().Language)
	switch surf {
	case SurfaceSettings:
		return copy.Settings.Title
	case SurfaceTerminal:
		return copy.Terminal.Title
	default:
		// A pinned plugin page is named by the plugin, and falls back to the
		// app's own name when the launcher that drew it is already gone.
		if title := strings.TrimSpace(a.Launcher.PluginTitle()); title != "" {
			return title
		}
		return copy.Launcher.Label
	}
}

// canDetachSurface reports whether the page the app is on has something to pin.
//
// The search page does not: it is the app's home, it is what the panel returns
// to, and pinning it would leave the panel with nothing. Every other page —
// the settings, a terminal session, and the launcher's own plugin modes — is a
// page the user can be *in* and want to keep.
func (a *App) canDetachSurface() bool {
	if a.Surf == SurfaceSettings || a.Surf == SurfaceTerminal {
		return true
	}
	// The launcher surface: only its plugin modes and an open configuration
	// sheet are pages. The ordinary search list is not.
	return a.Launcher != nil && (a.Launcher.PluginModeActive() || a.Launcher.ConfigOpen())
}

// newDetachedSurfaceWindow builds a pinned page's window.
//
// It is an ordinary window — not a panel — which is what makes the page outlive
// the panel: the panel's `HideOnBlur` is bound to `a.Win` alone
// (`shell.go`, `a.Win.OnBlur`), so a second window never answers it. The glass
// is the panel's own: the same material, the same transparency, the same
// platform inset, because the pinned page wears the *same card* the panel draws
// (see `viewWindow`) and a card that reads no desktop through it would not be
// the same card.
func (a *App) newDetachedSurfaceWindow(surf Surface, key, title string, state *detachedSurface) *mygo.Window {
	s := a.settings()
	return mygo.NewWindow(mygo.WindowOptions{
		Title:       title,
		Width:       detachedSurfaceWidth,
		Height:      detachedSurfaceHeight,
		MinWidth:    detachedSurfaceMinWidth,
		MinHeight:   detachedSurfaceMinHeight,
		Transparent: true,
		// The page draws its own title bar (`detachedTitleBar`), so the system
		// one is hidden and the window's controls ride over the page instead:
		// the traffic lights inset on macOS, the window buttons on Linux and
		// Windows. That is what lets the bar be the card's own material rather
		// than the OS's grey strip — 「需要优化下现在独立后的系统菜单栏，可以有创意
		// 点，只要能和现在风格融合好」.
		TitleBarStyle: titleBarStyleFor(runtime.GOOS),
		// The panel turns its shadow off everywhere but macOS, where the window
		// server casts one that follows the card's rounded, transparent content
		// (`shell.go`, `Start`). A pinned page is the same card, so it keeps the
		// same rule.
		DisableShadow: runtime.GOOS != "darwin",
		Vibrancy:      windowVibrancyFor(runtime.GOOS, activeMaterialStep(s, surf)),
		// Remembered per surface, the same rule the other second windows
		// follow: a pinned page opens where the user last left it.
		StateKey: detachedSurfaceStateKey(key),
		Content: ui.View(func(c *ui.Context) {
			a.detachedSurfaceView(c, state)
		}),
	})
}

// detachSurface pins the page the app is on into a window of its own.
//
// The panel goes back to the search page: the two windows then hold two
// different pages of the one app, which is the split the user described
// (「相当于和主窗口进行分割」). A page that is already pinned is raised rather
// than pinned twice.
func (a *App) detachSurface() {
	if !a.canDetachSurface() {
		return
	}
	surf := a.Surf
	if surf == SurfaceLauncher {
		a.detachLauncherPage()
		return
	}
	key := a.detachedPageKey(surf)
	if existing := a.findDetachedSurface(key); existing != nil {
		a.raiseDetachedSurface(existing)
		a.Open(SurfaceLauncher)
		return
	}
	if !a.allowAnotherDetachedSurface() {
		return
	}
	state := &detachedSurface{key: key, surf: surf}
	if a.openDetachedSurface != nil {
		state.win = a.openDetachedSurface(surf)
	} else {
		state.win = a.newDetachedSurfaceWindow(surf, key, a.detachedWindowTitle(surf, nil), state)
	}
	if state.win != nil {
		state.win.OnClosed(func() { a.forgetDetachedSurface(state) })
	}
	detachedMu.Lock()
	detachedSurfaces = append(detachedSurfaces, state)
	detachedMu.Unlock()
	// The panel gives the page up: it returns to the search list, which is
	// what makes the two windows independent.
	a.Open(SurfaceLauncher)
}

// detachedSurfaceWidth and friends are the pinned page's own opening size: a
// page is a page, so it opens at the settings panel's own width and a
// comfortable reading height, both remembered afterwards.
const (
	detachedSurfaceWidth     = 720
	detachedSurfaceHeight    = 560
	detachedSurfaceMinWidth  = 420
	detachedSurfaceMinHeight = 320
)

// findDetachedSurface returns the pinned window already showing the page `key`
// names, or nil.
func (a *App) findDetachedSurface(key string) *detachedSurface {
	detachedMu.Lock()
	defer detachedMu.Unlock()
	for _, open := range detachedSurfaces {
		if open.key == key {
			return open
		}
	}
	return nil
}

// countDetachedSurfaces is how many pages are pinned right now.
func (a *App) countDetachedSurfaces() int {
	detachedMu.Lock()
	defer detachedMu.Unlock()
	return len(detachedSurfaces)
}

// detachedPageKey is the identity of the page the app is on: the surface's own
// name for the settings and the terminal, and the plugin's id for a plugin page
// (which is a `SurfaceLauncher` like every other plugin page and like the search
// list itself). The empty string means "nothing to pin".
func (a *App) detachedPageKey(surf Surface) string {
	if surf != SurfaceLauncher {
		return surf.Name()
	}
	if a.Launcher == nil {
		return ""
	}
	return a.Launcher.PageKey()
}

// forgetDetachedSurface drops a pinned page whose window is gone. A launcher
// page's second instance goes with it: it was that window's, and nothing else
// holds it.
func (a *App) forgetDetachedSurface(state *detachedSurface) {
	detachedMu.Lock()
	for i, open := range detachedSurfaces {
		if open == state {
			detachedSurfaces = append(detachedSurfaces[:i], detachedSurfaces[i+1:]...)
			break
		}
	}
	detachedMu.Unlock()
	if state.launcher != nil {
		state.launcher = nil
	}
}

// closeDetachedSurfaces closes every pinned page. Quitting ends them with the
// rest of the app; a panel that hid does not call this, which is what keeps a
// pinned page on screen across a dismissal.
func (a *App) closeDetachedSurfaces() {
	detachedMu.Lock()
	open := detachedSurfaces
	detachedSurfaces = nil
	detachedMu.Unlock()
	for _, state := range open {
		state.closing = true
		if state.win != nil {
			state.win.Close()
		}
	}
}

// surfaceTokens resolves the palette a page draws with. The terminal has its
// own appearance (its opacity slider and palette), and every other page shares
// the launcher's; the pinned window must resolve exactly what the panel does,
// or the same page would change colour by being pinned.
func (a *App) surfaceTokens(c *ui.Context, surf Surface) launcher.Surface {
	s := a.settings()
	if surf == SurfaceTerminal {
		var out launcher.Surface
		out.Tokens = apptheme.ForTerminalPreferences(s, c.Theme().Dark, c.Preferences())
		out.Glass = glassmap.TerminalSpecForPreferencesOS(runtime.GOOS, s.GlassStep, s.TerminalOpacity, c.Preferences().HighContrast)
		return out
	}
	return launcher.Resolve(s, c.Theme().Dark, c.Preferences())
}

// detachedSurfaceView draws one pinned page: the mark that says it is pinned,
// and the surface's own view under it.
//
// The page draws itself. Each surface's `View` already paints its own sheet —
// the settings' `Sunken` pane, the terminal's canvas, the launcher's panel — so
// this window is a plain column over the near-solid root, exactly as the two
// other second windows are. Reusing the panel's glass card here would mean the
// pinned page wore a second, differently-sized frame around a sheet that
// already has one.
func (a *App) detachedSurfaceView(c *ui.Context, state *detachedSurface) {
	// The pinned page *is* the panel, drawn in its own window: the same card,
	// the same glass, the same corner, the same page under them (the user's
	// 「整体样式、布局以及交互应该保持不变」). `viewWindow` is the one function
	// that draws a window, so the two cannot drift; the only difference it is
	// told about is `detached`, which is the accent stroke that marks the
	// window as pinned and the title bar it draws in place of the system one.
	//
	// The flag is set for the length of the build because input is read while
	// the tree is built: a page's Escape handler runs *during* this call, and it
	// has to know that the window it is answering in is a pinned one (see
	// `surfaceExit`). The two Apps are shared with the panel, so there is no
	// per-window Actions set to carry the answer instead.
	a.detachedDrawing = true
	defer func() { a.detachedDrawing = false }()
	a.viewWindow(c, state.surf, true, state.launcher)
}

// detachLauncherPage pins the launcher's plugin page (or its open
// configuration sheet) into a window of its own.
//
// It needs a *second* `launcher.App`: the panel's launcher goes back to the
// search page, and one launcher's mode is a single value, so it cannot draw the
// plugin page and the search list at once. The second instance is seeded from
// the same catalogs and put into the same mode, and its `ResizeTo` is dropped —
// that action resizes the panel's window, and a pinned page must not.
func (a *App) detachLauncherPage() {
	key := a.detachedPageKey(SurfaceLauncher)
	if existing := a.findDetachedSurface(key); existing != nil {
		a.raiseDetachedSurface(existing)
		a.Open(SurfaceLauncher)
		a.Launcher.LeavePluginMode()
		return
	}
	if !a.allowAnotherDetachedSurface() {
		return
	}
	second := launcher.New(a.Store, a.detachedLauncherActions())
	a.seedDetachedLauncher(second)
	// The pinned page keeps the mode it was showing and refuses to leave it.
	second.PinPage()
	state := &detachedSurface{key: key, surf: SurfaceLauncher, launcher: second}
	if a.openDetachedSurface != nil {
		state.win = a.openDetachedSurface(SurfaceLauncher)
	} else {
		state.win = a.newDetachedSurfaceWindow(SurfaceLauncher, key, a.detachedWindowTitle(SurfaceLauncher, second), state)
	}
	if state.win != nil {
		state.win.OnClosed(func() { a.forgetDetachedSurface(state) })
	}
	detachedMu.Lock()
	detachedSurfaces = append(detachedSurfaces, state)
	detachedMu.Unlock()
	// The panel gives the page up and returns to its own search list.
	a.Launcher.LeavePluginMode()
}

// detachedLauncherActions is the panel launcher's own action set with the one
// entry a pinned page must not inherit: `ResizeTo`.
//
// That action resizes the *panel's* window to the height the list asks for
// (`shell.launcherHeight` → `walkLauncherHeight`). A pinned page runs in a
// window the user sizes, so a second launcher calling it would fight the user's
// own drag — and, worse, resize the window that is no longer showing the page.
// Every other action is the panel's own, shared as-is: the pinned page is the
// same app, and a command run from it must behave exactly as it does from the
// panel.
func (a *App) detachedLauncherActions() launcher.Actions {
	actions := a.Launcher.Actions
	actions.ResizeTo = nil
	// The pin's own two doors would open *another* second window from inside a
	// second window. A pinned page is already pinned, so both are dropped: the
	// page keeps its content, and the user's way out is the window's own bar.
	actions.DetachOutput = nil
	actions.PinText = nil
	return actions
}

// seedDetachedLauncher gives a pinned page the catalogs the panel's launcher
// holds, so its rows are the same rows.
//
// It copies *content*, never view state: the commands, the applications, the
// recent list, the aliases and the clipboard/calculator sources. Where the
// panel's field is, what its list has selected and how far it is scrolled stay
// with the panel — the pinned page starts at its own first row (see
// `AdoptPage`).
func (a *App) seedDetachedLauncher(second *launcher.App) {
	panel := a.Launcher
	second.SetCommands(panel.Commands)
	second.SetApps(panel.Apps)
	second.SetRecent(panel.Recent, true)
	second.Aliases = panel.Aliases
	second.Clipboard = panel.Clipboard
	second.Calculator = panel.Calculator
	second.CopyMode = panel.CopyMode
	second.AdoptPage(panel)
}

// surfaceExit is the settings page's way out — Escape, its close key, ⌘W — and
// it is the one place that knows which window is asking.
//
// In the panel it returns to the launcher, which is what it has always done. In
// a pinned page it does nothing at all: a pinned page has no way back to the
// search list (「独立出来的窗口不应该能回退到搜索页，不能通过删除、esc 回退，只能
// 整个退出」), and Escape there is not an exit — the window's own close is. The
// page's *inner* ladders still work, because they run before this one: an open
// confirmation is cancelled, a recording is abandoned, an integration's detail
// closes. Only the last rung, the one that would leave the page, is withheld.
func (a *App) surfaceExit() {
	if a.detachedDrawing {
		return
	}
	a.Open(SurfaceLauncher)
}

// The pinned page's title bar: the one band the panel's window does not have,
// because the panel has no title bar at all.
//
// The user asked for this specifically — 「需要优化下现在独立后的系统菜单栏，可以
// 有创意点，只要能和现在风格融合好」 — so it is not a system-looking strip: it
// takes the card's own material (the window is created with a hidden title bar,
// so the row sits on the same glass the card does), its corner is the card's,
// and its contents are the app's own vocabulary rather than the OS's.
//
// What it says, left to right:
//
//	● 已固定   Settings                        (drag anywhere)      ×
//
// The accent disc is the same mark the accent stroke carries: this window is a
// pinned one. The name is the page's own. The trailing cross closes the window,
// which on a pinned page is the *only* way out (see `surfaceExit`). Everything
// between them drags the window, which is what a title bar is for.
func (a *App) detachedTitleBar(c *ui.Context, surf Surface, tokens theme.Tokens) {
	t := c.Theme()
	bar := c.TitleBar()
	// The height is the *system's*, never ours.
	//
	// `c.TitleBar().Height` is the room the window controls need in this
	// window on this platform: macOS's inset traffic lights, Windows' and
	// Linux's own buttons. A taller row of our own would push the controls off
	// centre — the report 「现在 Mac 下有点过高了，导致系统自带的红绿灯栏和标题并没有
	// 完全上下居中」 — because the controls are placed by the window server at the
	// system height while the row would be drawn at ours. The `max` with a
	// spacing step was exactly that mistake: `Space(9)` is 32.4px at the shipped
	// step, against macOS's 28.
	//
	// A platform that reports no title bar (a bare `TitleBarHidden` with no
	// controls to clear, or a headless frame) still gets a usable row: the
	// fallback is the platform's own minimum, not a design number.
	height := detachedTitleBarHeight(bar, t)
	row := ui.Row(c).FillWidth().Shrink(0).Height(height).
		// The controls own the leading corner on macOS (the traffic lights) and
		// the trailing one elsewhere; `c.TitleBar` reports which, so the row
		// keeps clear of them instead of guessing a platform.
		Padding(0, bar.Right+t.Space(3), 0, bar.Left+t.Space(3)).
		Gap(t.Space(2)).AlignItems(ui.Center).
		// The bar drags the window, as the system title bar would.
		DragWindow()
	// The bar's own floor: one weak hairline, the same rule a block draws
	// between its rows (`docs/raycast-design.md` §6.1). Without it the bar and
	// the page read as one undivided surface, and the page's own header (which
	// starts with the page's title) looks like a second line of the bar.
	row.InsetShadow(0, -1, 0, 0, tokens.HairlineSoft)
	row.Children(func() {
		// The mark: an accent disc, then the word. It is the same sentence the
		// card's accent stroke says, in words.
		ui.Box(c).Size(t.Space(1.5), t.Space(1.5)).Shrink(0).Radius(t.Space(0.75)).
			Background(tokens.AccentEdgeStrong)
		ui.Text(c, i18n.For(a.Store.Snapshot().Language).Launcher.DetachedMark).
			FontSize(tokens.Caption).FontWeight(600).
			TextColor(tokens.TextSecondary).Shrink(0).SingleLine()
		// A hairline between the mark and the page's name: the two are different
		// facts (this window is pinned / this is what it shows), and the same
		// separator the app's other window titles use.
		ui.Box(c).Width(1).Height(t.Space(2.5)).Shrink(0).
			Background(tokens.HairlineSoft)
		ui.Text(c, a.surfaceTitle(surf)).FontSize(tokens.Caption).FontWeight(500).
			LetterSpacing(tokens.Caption * 0.01).TextColor(t.TextMuted).
			SingleLine().Ellipsis("\u2026").Grow(1)
	})
}

// titleBarStyleFor is the hidden title bar a pinned page draws under: the
// traffic lights inset on macOS, the plain hidden bar elsewhere (where the
// window buttons fill the bar's trailing end). One table, so the bar's own
// padding and the window's chrome cannot disagree.
func titleBarStyleFor(goos string) mygo.TitleBarStyle {
	if goos == "darwin" {
		return mygo.TitleBarHiddenInset
	}
	return mygo.TitleBarHidden
}

// MaxDetachedSurfaces is how many pages may be pinned at once.
//
// Ten, the same budget the launcher's own list uses (`launcher.rowBudget`): a
// window is a bigger thing than a row, and ten of them is already more than a
// desktop holds before the user starts losing track of which is which. It is a
// ceiling, not a target — most sessions will have one or two.
const MaxDetachedSurfaces = 10

// allowAnotherDetachedSurface reports whether one more page may be pinned, and
// says so in the panel when it may not.
//
// The refusal is spoken rather than silent: the user pressed a key and nothing
// opened, which without a sentence reads as a broken key rather than as a
// reached limit. It rides the panel's own docked feedback row (`Feedback`), the
// same place every other "that did not happen" answer goes.
func (a *App) allowAnotherDetachedSurface() bool {
	if a.countDetachedSurfaces() < MaxDetachedSurfaces {
		return true
	}
	if a.Launcher != nil {
		a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Launcher.DetachLimit(MaxDetachedSurfaces))
	}
	return false
}

// raiseDetachedSurface brings a pinned page's window forward. Pinning a page
// that is already pinned is a raise, not a second window: the same page twice
// would be two windows the user cannot tell apart.
func (a *App) raiseDetachedSurface(state *detachedSurface) {
	if state.win == nil {
		return
	}
	state.win.Show()
	state.win.Focus()
	state.win.Invalidate()
}

// detachedTitleBarHeight is the pinned page's title bar height: the system's own
// when the window reports one, and the platform's minimum when it does not.
//
// The rule is "never taller than the controls", because the window server
// places the controls at the system height and a taller row would leave them
// sitting above its centre. macOS's inset traffic lights want 28 DIP; Windows
// and Linux want their own bar height (32 on Windows 11). A frame with no title
// bar at all (a headless one, or a platform whose controls are hidden) gets the
// minimum the platform would use, so the row is still a row.
func detachedTitleBarHeight(bar ui.TitleBar, t *ui.Theme) float32 {
	if bar.Height > 0 {
		return bar.Height
	}
	// No controls to clear: the smallest bar that still reads as one — a
	// caption line box plus one breathing step on each side.
	return max(t.Space(7), t.FontSize+t.Space(2))
}
