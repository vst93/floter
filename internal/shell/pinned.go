package shell

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// pinned is the state of one pinned-output window: its title and the text it
// shows, which a later pin replaces. win is the window it is drawn in, so the
// bar's own close key can reach it (`pinnedBody`).
type pinned struct {
	title string
	text  string
	// family is the family the text is drawn in, taken from the settings the pin
	// was made under: a note holds terminal output or a clipboard clip, and its
	// sibling window (`detach.go`) draws the same text in the terminal's own
	// family (P13-gw).
	family string
	win    *mygo.Window
}

// pinnedStateKey is the key a note's geometry is remembered under: one per
// note, keyed by its title, the way a command-less detached run is keyed by its
// own trimmed title (`detachContentKey`). R91's rule is why: *a window belongs
// to its content* — and a note's content is what it is called. One shared key
// made every note open on the last note's rectangle and fight over it.
func pinnedStateKey(title string) string {
	return settings.DetachedStateKeyPrefix + "text\x00" + strings.TrimSpace(title)
}

// PinText opens a window holding a copy of some text — a terminal session's
// output, a clipboard clip — where it stays put and can be selected, as the
// old build's detached windows did.
//
// The window is an ordinary one: the user moves, resizes and closes it like
// any other, and closing it only forgets the pin.
//
// R90 · this is the reference's *text* arm of the one plugin-window surface
// (`DetachedPluginApp.tsx:308-350`, a request of `kind: "text"`): the snapshot
// wears the same bar — the title and a Close key, and no Rerun (there is no
// command to re-run) — over the same body. The port drew it as a barless
// window, so a pinned clip had no chrome and no way to close it but the OS.
func (a *App) PinText(title, text string) {
	if a.openPinned != nil {
		a.openPinned(title, text)
		return
	}
	state := a.newPinned(title, text)
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:    state.title,
		Width:    720,
		Height:   480,
		StateKey: pinnedStateKey(state.title),
		Content: ui.View(func(c *ui.Context) {
			a.pinnedView(c, state)
		}),
	})
	state.win = win
	win.OnClosed(func() {
		a.pinMu.Lock()
		delete(a.pins, win)
		a.pinMu.Unlock()
	})
	a.pinMu.Lock()
	if a.pins == nil {
		a.pins = map[*mygo.Window]*pinned{}
	}
	a.pins[win] = state
	a.pinMu.Unlock()
}

// newPinned is the note that will be drawn in a window: its title (a blank one
// falls back to the app's name), its text, and the family that text is drawn in
// — the settings' own, normalized the way the terminal normalizes it, so a
// blank setting means the shipped monospace on both surfaces (P13-gw).
func (a *App) newPinned(title, text string) *pinned {
	state := &pinned{title: strings.TrimSpace(title), text: text,
		family: settings.NormalizeFontFamily(a.Store.Snapshot().FontFamily)}
	if state.title == "" {
		state.title = "floter"
	}
	return state
}

// pinnedView draws one pinned window's whole frame: the theme it resolves for
// its own Context, the opaque root, and the bar + body (`pinnedBody`). It is
// the counterpart of `detachedView` — both windows are the same plugin-window
// surface, so both resolve their palette the same way — and it is what a test
// renders, since the window itself needs a display.
func (a *App) pinnedView(c *ui.Context, state *pinned) {
	surface := launcher.Resolve(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
	tokens := surface.Tokens
	c.SetTheme(tokens.Theme)
	t := c.Theme()
	c.Root().Background(tokens.Opaque)
	a.pinnedBody(c, state, tokens, t)
}

// pinnedBody draws the pinned window's chrome and its text: the reference's
// bar (`.plugin-window__bar`) carrying only the title and the Close key — a
// text snapshot has no command, so no Rerun (`DetachedPluginApp.tsx:293-306`)
// — over the body's text block (`PluginTextView`, `.launcher-plugin-text` at
// `launcher.css:2615-2637`): the body register at a 1.45 line, selectable,
// which is what the pin exists for.
func (a *App) pinnedBody(c *ui.Context, state *pinned, tokens theme.Tokens, t *ui.Theme) {
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	ui.Column(c).Fill().Children(func() {
		bar := ui.Row(c).FillWidth().Shrink(0).
			Padding(t.Space(detachedBarPadY), t.Space(detachedBarPadX)).
			Gap(t.Space(detachedBarGap)).AlignItems(ui.Center)
		bar.InsetShadow(0, -1, 0, 0, tokens.Hairline)
		bar.Children(func() {
			ui.Text(c, state.title).FontSize(tokens.Caption).FontWeight(500).
				LetterSpacing(tokens.Caption * 0.01).TextColor(t.TextMuted).Grow(1).
				SingleLine().Ellipsis("\u2026")
			close := ui.Row(c).Focusable().Shrink(0).
				Padding(t.Space(detachedBarButtonY), t.Space(detachedBarButtonX)).
				Radius(tokens.RadiusSM).Label(copy.OutputWindowClose)
			if close.Pressed() {
				// P13-al · same key, same third rung (see `detach.go`'s `barButton`):
				// the pressed note's ✕ sinks to `ControlPress` and steps back to 0.92,
				// keeping its E1 pair. Without this arm the frame under a held pointer
				// was the hover frame.
				close = tokens.LitFace(close.Background(tokens.ControlPress))
				close = close.Scale(pressScaleKey, pressScaleKey)
			} else if close.Hovered() {
				// P13-y · the pinned note's ✕ is the same key the detached output
				// window draws (`detach.go` `barButton`), on the same
				// `.plugin-window__bar`: E1's pair, not a flat wash. Two windows, one
				// bar, one touch.
				close = tokens.LitFace(close.Background(tokens.ControlHover))
			}
			close.Children(func() {
				if glyph, ok := launcher.Glyph("close"); ok {
					ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(tokens.TextTertiary)
				}
			})
			if close.Clicked() && state.win != nil {
				state.win.Close()
			}
		})
		body := ui.Scroll(c.Key("pinned.output")).Fill().Padding(t.Space(detachedBodyInset))
		body.Children(func() {
			ui.Text(c, state.text).Font(state.family).FontSize(tokens.Body).LineHeight(1.45).
				TextColor(tokens.TextSecondary).Wrap().Selectable()
		})
		// P13-fz（`docs/liquid-glass.md` §6.145）· 与捕获输出窗口同一道下沿：便条是
		// 同一个 `.plugin-window` 表面（`pinnedView` 也把根节点刷成 `tokens.Opaque`），
		// 所以同一块自由滚动的面用同一条边、同一个高度、同一条自门控算术。
		ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(t.Space(detachedEdgeInset)).PassThrough().
			Material(glass.ScrollEdge{Bottom: true, Background: tokens.Opaque})
	})
}
