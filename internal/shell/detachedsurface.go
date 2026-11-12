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

// detachedSurface is one pinned page: which surface it shows, and its window.
type detachedSurface struct {
	look themeFrame
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
var (
	detachedMu       sync.Mutex
	detachedSurfaces = map[*mygo.Window]*detachedSurface{}
)

// detachedSurfaceStateKey is the key a pinned page's geometry is remembered
// under: one per surface, so pinning the settings and pinning a terminal
// session open where the user last left each of them.
func detachedSurfaceStateKey(surf Surface) string {
	return settings.DetachedStateKeyPrefix + "surface\x00" + surf.Name()
}

// surfaceTitle is what a pinned page calls itself: the surface's own name in
// the user's language.
func (a *App) surfaceTitle(surf Surface) string {
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	switch surf {
	case SurfaceSettings:
		return copy.Title
	case SurfaceTerminal:
		return copy.PageSessions
	default:
		return i18n.For(a.Store.Snapshot().Language).Launcher.Label
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
	if existing := a.findDetachedSurface(surf); existing != nil {
		if existing.win != nil {
			existing.win.Show()
			existing.win.Focus()
			existing.win.Invalidate()
		}
		a.Open(SurfaceLauncher)
		return
	}
	state := &detachedSurface{surf: surf}
	if a.openDetachedSurface != nil {
		state.win = a.openDetachedSurface(surf)
	} else {
		state.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     detachWindowTitle(a.surfaceTitle(surf)),
			Width:     detachedSurfaceWidth,
			Height:    detachedSurfaceHeight,
			MinWidth:  detachedSurfaceMinWidth,
			MinHeight: detachedSurfaceMinHeight,
			// Remembered per surface, the same rule the other second windows
			// follow: a pinned page opens where the user last left it.
			StateKey: detachedSurfaceStateKey(surf),
			Content: ui.View(func(c *ui.Context) {
				a.detachedSurfaceView(c, state)
			}),
		})
	}
	if state.win != nil {
		state.win.OnClosed(func() { a.forgetDetachedSurface(state) })
	}
	detachedMu.Lock()
	detachedSurfaces[state.win] = state
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

// findDetachedSurface returns the pinned window already showing `surf`, or nil.
func (a *App) findDetachedSurface(surf Surface) *detachedSurface {
	detachedMu.Lock()
	defer detachedMu.Unlock()
	for _, open := range detachedSurfaces {
		if open.surf == surf {
			return open
		}
	}
	return nil
}

// forgetDetachedSurface drops a pinned page whose window is gone. A launcher
// page's second instance goes with it: it was that window's, and nothing else
// holds it.
func (a *App) forgetDetachedSurface(state *detachedSurface) {
	detachedMu.Lock()
	delete(detachedSurfaces, state.win)
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
	open := make([]*detachedSurface, 0, len(detachedSurfaces))
	for _, state := range detachedSurfaces {
		open = append(open, state)
	}
	detachedSurfaces = map[*mygo.Window]*detachedSurface{}
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
	surface := a.surfaceTokens(c, state.surf)
	tokens := surface.Tokens
	state.look.apply(c, tokens)
	t := c.Theme()
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	// A pinned page is opaque: it is a page the user put on screen, not a pane
	// reading the desktop through it.
	c.Root().Background(tokens.Opaque)

	ui.Column(c).Fill().Children(func() {
		a.detachedSurfaceBar(c, state, tokens, t, copy)
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
			switch state.surf {
			case SurfaceSettings:
				a.Settings.View(c)
			case SurfaceTerminal:
				a.Terminal.View(c)
			default:
				if state.launcher != nil {
					state.launcher.View(c)
				}
			}
		})
	})
}

// The pinned page's own bar: the page's name, the word that says it is pinned,
// and the way out.
//
// This is the 「特殊的标记」 the user asked for, and it is deliberately the one
// thing the page did not have before: a band above it that names what the user
// is looking at and says it is no longer the panel's. The mark itself is an
// accent dot beside the word — the same accent the app spends on "this is
// chosen" everywhere else (`docs/raycast-design.md` §5), never a new colour.
func (a *App) detachedSurfaceBar(c *ui.Context, state *detachedSurface, tokens theme.Tokens, t *ui.Theme, copy i18n.Launcher) {
	bar := ui.Row(c).FillWidth().Shrink(0).
		Padding(t.Space(detachedBarPadY), t.Space(detachedBarPadX)).
		Gap(t.Space(detachedBarGap)).AlignItems(ui.Center)
	bar.InsetShadow(0, -1, 0, 0, tokens.Hairline)
	bar.Children(func() {
		// The mark comes first, at the bar's leading edge, and the page's own
		// name follows it. The page prints its name again in its own header —
		// that header is the page's, and this bar is the window's — so the two
		// are not the same sentence: this one says *this page is pinned*, and
		// the page's says which page it is.
		ui.Box(c).Size(t.Space(1.5), t.Space(1.5)).Shrink(0).Radius(t.Space(0.75)).
			Background(t.Accent)
		ui.Text(c, copy.DetachedMark).FontSize(tokens.Caption).FontWeight(600).
			TextColor(tokens.TextSecondary).Shrink(0).SingleLine()
		ui.Box(c).Width(1).Height(t.Space(2.5)).Shrink(0).Margin(t.Space(1), 0).
			Background(tokens.Hairline)
		ui.Text(c, a.surfaceTitle(state.surf)).FontSize(tokens.Caption).FontWeight(500).
			LetterSpacing(tokens.Caption * 0.01).TextColor(t.TextMuted).Grow(1).
			SingleLine().Ellipsis("\u2026")
		// The way out: the same 28u transparent square the other second
		// windows' bars carry, so the three read as one family.
		close := ui.Row(c).Focusable().Shrink(0).
			Size(max(36, t.Space(9)), max(36, t.Space(9))).
			Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
			Label(copy.OutputWindowClose)
		plane := ui.Transparent
		switch {
		case close.Pressed():
			plane = tokens.ControlPress
			close.Scale(pressScaleKey, pressScaleKey)
		case close.Hovered():
			plane = tokens.ControlHover
		}
		close.Background(plane)
		if plane.A > 0 {
			tokens.LitFace(close)
		}
		close.Children(func() {
			ui.Text(c, "\u00d7").FontSize(tokens.Emphasis).TextColor(t.TextMuted)
		})
		if close.Clicked() {
			state.closing = true
			if state.win != nil {
				state.win.Close()
			}
		}
	})
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
	if existing := a.findDetachedSurface(SurfaceLauncher); existing != nil {
		if existing.win != nil {
			existing.win.Show()
			existing.win.Focus()
			existing.win.Invalidate()
		}
		a.Open(SurfaceLauncher)
		a.Launcher.LeavePluginMode()
		return
	}
	second := launcher.New(a.Store, a.detachedLauncherActions())
	a.seedDetachedLauncher(second)
	state := &detachedSurface{surf: SurfaceLauncher, launcher: second}
	if a.openDetachedSurface != nil {
		state.win = a.openDetachedSurface(SurfaceLauncher)
	} else {
		state.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     detachWindowTitle(a.detachedLauncherTitle(second)),
			Width:     detachedSurfaceWidth,
			Height:    detachedSurfaceHeight,
			MinWidth:  detachedSurfaceMinWidth,
			MinHeight: detachedSurfaceMinHeight,
			StateKey:  detachedSurfaceStateKey(SurfaceLauncher),
			Content: ui.View(func(c *ui.Context) {
				a.detachedSurfaceView(c, state)
			}),
		})
	}
	if state.win != nil {
		state.win.OnClosed(func() { a.forgetDetachedSurface(state) })
	}
	detachedMu.Lock()
	detachedSurfaces[state.win] = state
	detachedMu.Unlock()
	// The panel gives the page up and returns to its own search list.
	a.Launcher.LeavePluginMode()
}

// detachedLauncherTitle names a pinned plugin page: the plugin's own name.
func (a *App) detachedLauncherTitle(second *launcher.App) string {
	if title := second.PluginTitle(); strings.TrimSpace(title) != "" {
		return title
	}
	return i18n.For(a.Store.Snapshot().Language).Launcher.Label
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
