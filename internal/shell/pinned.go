package shell

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/theme"
)

// pinned is the state of one pinned-output window: its title and the text it
// shows, which a later pin replaces. win is the window it is drawn in, so the
// bar's own close key can reach it (`pinnedBody`).
type pinned struct {
	title string
	text  string
	win   *mygo.Window
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
	state := &pinned{title: title, text: text}
	if title == "" {
		state.title = "floter"
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:    state.title,
		Width:    720,
		Height:   480,
		StateKey: "pinned",
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
			ui.Text(c, state.text).FontSize(tokens.Body).LineHeight(1.45).
				TextColor(tokens.TextSecondary).Wrap().Selectable()
		})
	})
}
