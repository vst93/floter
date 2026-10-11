package shell

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
)

// pinned is the state of one pinned-output window: its title and the text it
// shows, which a later pin replaces.
type pinned struct {
	title string
	text  string
}

// PinText opens a window holding a copy of some text — a terminal session's
// output, a clipboard clip — where it stays put and can be selected, as the
// old build's detached windows did.
//
// The window is an ordinary one: the user moves, resizes and closes it like
// any other, and closing it only forgets the pin.
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
			// Its own Context, so the floter theme has to be set here too;
			// the window is opaque, not the panel's glass.
			surface := launcher.Resolve(a.Store.Snapshot(), c.Theme().Dark)
			tokens := surface.Tokens
			c.SetTheme(tokens.Theme)
			t := c.Theme()
			c.Root().Background(tokens.Opaque)
			ui.Scroll(c).Fill().Padding(t.Space(3)).Children(func() {
				ui.Text(c, state.text).FontSize(t.FontSize).TextColor(t.Text).Selectable()
			})
		}),
	})
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
