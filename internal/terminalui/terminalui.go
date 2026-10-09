// Package terminalui draws the terminal surface: a window-wide Ghostty
// terminal for one session, or an empty state while none runs.
//
// It also owns the terminal's appearance: the settings file's font, cursor,
// line height, inset, palette and transparency, mapped onto the terminal
// plugin's options (see src/terminal/terminal-appearance.ts for the values
// they come from).
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

	// NewTerminal creates the session, with the options built from the
	// settings. Nil means no session can start (tests).
	NewTerminal func(terminal.Options) (*terminal.Terminal, error)

	// Term is the running session, nil until the session starts.
	Term *terminal.Terminal
	// Title is the title the program set.
	Title string
	// Err is why the session could not start.
	Err error

	// Focus is the terminal view's identity, so it keeps the keyboard.
	Focus ui.Handle
}

// New builds the terminal surface.
func New(store *settings.Store, actions Actions, newTerminal func(terminal.Options) (*terminal.Terminal, error)) *App {
	return &App{Store: store, Actions: actions, NewTerminal: newTerminal}
}

// EnsureSession starts the session if none runs. The shell calls it as the
// surface opens.
func (a *App) EnsureSession() {
	if a.Term != nil || a.NewTerminal == nil {
		return
	}
	term, err := a.NewTerminal(a.options())
	if err != nil {
		a.Err = err
		return
	}
	a.Term, a.Err = term, nil
}

// Refresh applies the appearance settings to a running session, for a
// change made while the terminal is open.
func (a *App) Refresh() {
	if a.Term == nil {
		return
	}
	opts := a.options()
	a.Term.SetFont(opts.Font)
	a.Term.SetTheme(opts.Theme, opts.DarkTheme)
}

// FocusTerminal asks for the keyboard focus on the terminal.
func (a *App) FocusTerminal() { a.Focus.Focus() }

// options builds the plugin options from the settings.
func (a *App) options() terminal.Options {
	s := a.Store.Snapshot()
	theme := palette(s.TerminalTheme, s.TerminalOpacity)
	return terminal.Options{
		Font: terminal.Font{
			Family:     s.FontFamily,
			Size:       float32(s.FontSize),
			LineHeight: float32(s.TerminalLineHeight),
		},
		Cursor:  cursorStyle(s.CursorShape),
		NoBlink: !s.CursorBlink,
		Theme:   theme,
		// The terminal's own background carries the transparency: the
		// panel behind (the card's glass) shows through it.
		Transparent: false,
		OnTitle:     a.Actions.Title,
		OnExit:      a.Actions.Exit,
	}
}

// Pad is the inset around the terminal, from the padding step: the three
// values terminal-appearance.ts shipped (1, 3 and 6 DIPs).
func (a *App) Pad() float32 {
	switch a.Store.Snapshot().TerminalPadding {
	case "compact":
		return 1
	case "relaxed":
		return 6
	default:
		return 3
	}
}

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
		pad := a.Pad()
		terminal.View(c, a.Term).Fill().Grow(1).Padding(pad).Bind(&a.Focus)
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

// cursorStyle maps the stored cursor shape onto the plugin's.
func cursorStyle(shape string) terminal.CursorStyle {
	switch shape {
	case "block":
		return terminal.CursorBlock
	case "underline":
		return terminal.CursorUnderline
	default: // beam
		return terminal.CursorBar
	}
}

// palette builds the terminal colors for a stored palette id: the shipped
// `inherit` follows the window (no theme), and each override paints the
// fixed pair from TERMINAL_PALETTES, with the transparency applied to the
// background.
func palette(name string, opacity uint8) *terminal.Theme {
	spec, ok := palettes[name]
	if !ok {
		return nil
	}
	base := terminal.DarkTheme()
	if !spec.dark {
		base = terminal.LightTheme()
	}
	t := *base
	alpha := float32(opacity) / 100
	t.Background = packed(spec.bg).Alpha(alpha)
	t.Foreground = packed(spec.fg)
	t.Cursor = packed(spec.cursor)
	t.Selection = spec.selection
	return &t
}

// paletteSpec is one palette override from terminal-appearance.ts.
type paletteSpec struct {
	bg, fg, cursor uint32
	selection      ui.Color
	dark           bool
}

var palettes = map[string]paletteSpec{
	"contrast": {bg: 0x000000, fg: 0xffffff, cursor: 0x00ff9c, selection: ui.RGBA(255, 255, 255, 0.30), dark: true},
	"paper":    {bg: 0xf4efe4, fg: 0x2a2a28, cursor: 0x8a5a00, selection: ui.RGBA(0, 0, 0, 0.14)},
	"ink":      {bg: 0x0b0d10, fg: 0xe6e9ef, cursor: 0x7dd3fc, selection: ui.RGBA(255, 255, 255, 0.24), dark: true},
	"fog":      {bg: 0x1b1f24, fg: 0xc9d1d9, cursor: 0x9ece6a, selection: ui.RGBA(255, 255, 255, 0.22), dark: true},
	"forest":   {bg: 0x0f1a14, fg: 0xd7e4d0, cursor: 0x8bd450, selection: ui.RGBA(255, 255, 255, 0.22), dark: true},
	"dusk":     {bg: 0x1a1526, fg: 0xe2d9f3, cursor: 0xc792ea, selection: ui.RGBA(255, 255, 255, 0.24), dark: true},
	"mist":     {bg: 0xdfe7ef, fg: 0x2b3440, cursor: 0x2f6f9f, selection: ui.RGBA(0, 0, 0, 0.16)},
	"amber":    {bg: 0x1c140a, fg: 0xf0e2c8, cursor: 0xffb454, selection: ui.RGBA(255, 255, 255, 0.24), dark: true},
}

// packed turns a 0xRRGGBB literal into a color.
func packed(v uint32) ui.Color {
	return ui.RGB(uint8(v>>16), uint8(v>>8), uint8(v))
}
