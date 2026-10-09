// Package terminalui draws the terminal surface: a window-wide Ghostty
// terminal for one session, or an empty state while none runs.
//
// The session is created through a Start function the shell supplies, so the
// surface itself never touches the pseudo-terminal: tests render the chrome
// and the empty state without a library, and the shell owns the real
// terminal's lifetime.
package terminalui

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// Actions are the shell's callbacks.
type Actions struct {
	// Close leaves the terminal surface.
	Close func()
	// Title is called when the program sets the terminal's title.
	Title func(string)
	// Exit is called when the program ends; the shell returns to the
	// launcher.
	Exit func(int)
}

// App is the terminal surface's state.
type App struct {
	Store   *settings.Store
	Actions Actions

	// Start creates the session. Nil means no session can start (tests, or a
	// build without the terminal plugin).
	Start func() (*terminal.Terminal, error)

	// Term is the running session, nil until Start succeeds.
	Term *terminal.Terminal
	// Title is the title the program set.
	Title string
	// Err is why the session could not start.
	Err error

	// Focus is the terminal view's identity, so it keeps the keyboard.
	Focus ui.Handle
}

// New builds the terminal surface.
func New(store *settings.Store, actions Actions, start func() (*terminal.Terminal, error)) *App {
	return &App{Store: store, Actions: actions, Start: start}
}

// EnsureSession starts the session if none runs. The shell calls it as the
// surface opens.
func (a *App) EnsureSession() {
	if a.Term != nil || a.Start == nil {
		return
	}
	term, err := a.Start()
	if err != nil {
		a.Err = err
		return
	}
	a.Term, a.Err = term, nil
}

// FocusTerminal asks for the keyboard focus on the terminal.
func (a *App) FocusTerminal() { a.Focus.Focus() }

// Close ends the session, if any.
func (a *App) Close() {
	if a.Term != nil {
		a.Term.Close()
		a.Term = nil
	}
}

// View builds the terminal surface: a title row over the session, or the
// empty state when none runs.
func (a *App) View(c *ui.Context) {
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	t := c.Theme()

	ui.Column(c).Fill().Gap(t.Space(1)).Children(func() {
		a.titleRow(c, copy)
		if a.Term == nil {
			ui.Column(c).Fill().Grow(1).Center().Children(func() {
				hint := copy.Hint
				if a.Err != nil {
					hint = a.Err.Error()
				}
				ui.Text(c, hint).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
			return
		}
		terminal.View(c, a.Term).Fill().Grow(1).Bind(&a.Focus).AutoFocus()
	})
}

// titleRow is the terminal's own header: the session title and a close
// button, since the window is frameless.
func (a *App) titleRow(c *ui.Context, copy i18n.Terminal) {
	t := c.Theme()
	title := a.Title
	if title == "" {
		title = copy.Title
	}
	row := ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center)
	row.Children(func() {
		ui.Text(c, title).FontSize(t.FontSize).Bold().Grow(1)
		if ui.Button(c, "✕").Label(copy.Close).Clicked() && a.Actions.Close != nil {
			a.Actions.Close()
		}
	})
}
