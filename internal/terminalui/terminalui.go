// Package terminalui draws the terminal surface: a window-wide Ghostty
// terminal for one session, or an empty state while none runs.
//
// It also owns the terminal's appearance: the settings file's font, cursor,
// line height, inset, palette and transparency, mapped onto the terminal
// plugin's options (see src/terminal/terminal-appearance.ts for the values
// they come from).
package terminalui

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// Actions are the shell's callbacks.
type Actions struct {
	// Close leaves the terminal surface.
	Close func()
	// NewCommand returns to the launcher with an empty field, and
	// OpenExternal hands the session to the system's own terminal. Both are
	// the old terminal header's controls; either may be nil in tests.
	NewCommand   func()
	OpenExternal func()
	// Title is called when the program sets the terminal's title.
	Title func(string)
	// Exit is called when the program ends; the shell returns to the
	// launcher.
	Exit func(int)
	// Pin copies the session's text into a window of its own.
	Pin func(title, text string)
	// Settings opens the panel where the terminal's appearance is set: the
	// bar's own gear, so the session has a way to the options that shape it.
	Settings func()
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
	// resident marks a session whose program has ended: the output is held on
	// screen until the user closes it, and the note at the bottom says so.
	// exitCode is what the program returned.
	resident bool
	exitCode int

	// Focus is the terminal view's identity, so it keeps the keyboard.
	Focus ui.Handle
	// Text is the session's text, for the pin control; nil reads it from
	// the terminal.
	Text func() string

	// SnapshotPath is where the session's screen is saved when it closes and
	// read back when the next one starts, so the last session's scrollback
	// comes back. Empty disables it.
	SnapshotPath string
}

// New builds the terminal surface.
func New(store *settings.Store, actions Actions, newTerminal func(terminal.Options) (*terminal.Terminal, error)) *App {
	return &App{Store: store, Actions: actions, NewTerminal: newTerminal}
}

// EnsureSession starts the session if none runs. The shell calls it as the
// surface opens.
func (a *App) EnsureSession() {
	if a.NewTerminal == nil {
		return
	}
	// A held session has ended: entering the page again starts a fresh one
	// rather than showing the same finished frame.
	if a.resident {
		if a.Term != nil {
			_ = a.Term.Close()
		}
		a.Term, a.resident, a.exitCode = nil, false, 0
	}
	if a.Term != nil {
		return
	}
	term, err := a.NewTerminal(a.options())
	if err != nil {
		a.Err = err
		return
	}
	a.Term, a.Err = term, nil
	a.restoreSnapshot(term)
}

// SaveSnapshot writes the session's screen to SnapshotPath: the escape
// sequences that reproduce what the terminal shows, which is what the next
// session is fed. A session with nothing on screen writes nothing, and a
// missing path is not an error.
func (a *App) SaveSnapshot() error {
	if a.SnapshotPath == "" || a.Term == nil {
		return nil
	}
	data := a.Term.Snapshot()
	if len(data) == 0 {
		return nil
	}
	return writeFileAtomically(a.SnapshotPath, data)
}

// restoreSnapshot feeds the saved screen into a fresh session, so the last
// session's scrollback comes back above the new shell's prompt. The file is
// left where it is: the next close overwrites it, and a crash therefore
// restores the state the user last saw.
func (a *App) restoreSnapshot(term *terminal.Terminal) {
	if a.SnapshotPath == "" || term == nil {
		return
	}
	data, err := os.ReadFile(a.SnapshotPath)
	if err != nil || len(data) == 0 {
		return
	}
	term.Feed(data)
}

// writeFileAtomically writes data to path: a temporary file in the same
// directory, flushed, then renamed over the target.
func writeFileAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".snapshot-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// RunCommand ends the current session and starts one that runs argv: how an
// extension's command is handed to the terminal. dir names the working
// directory policy the provider asked for ("home", "current", "inherit"),
// and env is the integration's configured environment ("KEY=value").
// RunShellWithCommand opens a new bare session (no command: the user's own
// shell) and types a command line into it. The install rows use it: the shell
// is the user's and outlives the install, and the command is typed rather than
// run, so the session is interactive when the install finishes.
//
// The command is typed once the session has produced its first output (a
// prompt); the deadline keeps a shell that never prints from holding the app.
func (a *App) RunShellWithCommand(command string) {
	if a.NewTerminal == nil || strings.TrimSpace(command) == "" {
		return
	}
	a.Close()
	opts := a.options()
	opts.Command = nil
	term, err := a.NewTerminal(opts)
	if err != nil {
		a.Err = err
		return
	}
	a.Term, a.Err = term, nil
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && a.Term.Text() == "" {
			time.Sleep(50 * time.Millisecond)
		}
		a.Term.Send([]byte(command + "\r"))
	}()
}

func (a *App) RunCommand(argv []string, dir string, env []string) error {
	if len(argv) == 0 {
		return errors.New("terminalui: no command to run")
	}
	if a.NewTerminal == nil {
		return errors.New("terminalui: no terminal available")
	}
	a.Close()

	opts := a.options()
	opts.Command = argv
	opts.Env = append([]string{}, env...)
	switch dir {
	case "", "home":
		// The plugin's default: the user's home directory.
	case "current", "inherit":
		// The app's own directory; a session's cwd is not tracked here.
		if wd, err := os.Getwd(); err == nil {
			opts.Dir = wd
		}
	default:
		if wd, err := os.Getwd(); err == nil {
			opts.Dir = wd
		}
	}

	term, err := a.NewTerminal(opts)
	if err != nil {
		a.Err = err
		return err
	}
	a.Term, a.Err = term, nil
	a.FocusTerminal()
	return nil
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

// sessionText is the session's text, from the injected reader or the
// terminal itself.
func (a *App) sessionText() string {
	if a.Text != nil {
		return a.Text()
	}
	if a.Term == nil {
		return ""
	}
	return a.Term.Text()
}

// Label is the session's name: the title the program set, or the localized
// default.
func (a *App) Label() string {
	if a.Title != "" {
		return a.Title
	}
	return i18n.For(a.Store.Snapshot().Language).Terminal.Title
}

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
		// Save before closing: the session the user closed is the one the
		// next start restores.
		if err := a.SaveSnapshot(); err != nil {
			log.Printf("floter: could not save the terminal snapshot: %v", err)
		}
		a.Term.Close()
		a.Term = nil
	}
}

// View builds the terminal surface: a title row over the session, or the
// empty state when none runs.
func (a *App) View(c *ui.Context) {
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	t := c.Theme()

	ui.Box(c).Fill().Children(func() {
		if a.Term == nil {
			// The empty state: what the page is, the control that opens a
			// blank session, and the one that returns to the search. A spawn
			// error replaces the hint, since that is what the user needs to
			// read.
			ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
				title := copy.EmptyTitle
				hint := copy.Hint
				if a.Err != nil {
					title, hint = copy.EmptyTitle, a.Err.Error()
				}
				ui.Text(c, title).FontSize(t.FontSize + 2).Bold()
				ui.Text(c, hint).FontSize(t.FontSize).TextColor(t.TextMuted)
				if a.Actions.Close == nil {
					return
				}
				ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
					if ui.Button(c, copy.EmptyNew).Clicked() {
						a.EnsureSession()
					}
					if ui.Button(c, copy.EmptyBack).Clicked() && a.Actions.Close != nil {
						a.Actions.Close()
					}
				})
			})
			// The bar floats over the blank page too: its close control is
			// the way out the user asked for, and its dot says the page holds
			// no live session.
			a.titleBar(c, copy)
			return
		}
		// The canvas fills the whole panel; the bar floats above it, so no
		// layout height is reserved and the first output line sits at the very
		// top edge (the old build's `.terminal-panel__body` over
		// `.terminal-bar`).
		pad := a.Pad()
		terminal.View(c, a.Term).Fill().Padding(pad).Bind(&a.Focus)
		a.titleBar(c, copy)
		a.residentNote(c, copy)
	})
}

// Resident marks the session ended: the output stays on screen and the note
// says how the program went, until the user closes it or starts another.
func (a *App) Resident(code int) {
	a.resident = true
	a.exitCode = code
}

// residentNote is the floating note a held session carries: what happened and
// that closing is the user's own decision, in the warning colour when the
// program returned non-zero.
func (a *App) residentNote(c *ui.Context, copy i18n.Terminal) {
	if !a.resident || a.Term == nil {
		return
	}
	t := c.Theme()
	color := t.TextMuted
	if a.exitCode != 0 {
		color = t.Warning
	}
	ui.Row(c).Absolute().Bottom(t.Space(3)).Left(t.Space(3)).Right(t.Space(3)).
		Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(2), t.Space(2.5)).
		Radius(theme.For(a.Store.Snapshot(), t.Dark).RadiusMD).
		Background(theme.For(a.Store.Snapshot(), t.Dark).Control).Children(func() {
		ui.Text(c, copy.ProcessExited(a.exitCode)).FontSize(t.FontSize - 1).
			FontWeight(580).TextColor(color)
		ui.Text(c, copy.ProcessExitedHint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
	})
}

// titleBar is the floating bar at the panel's top edge: the session's
// identity (a live dot and the title) at the start, revealed while the bar is
// hovered or something on it has the keyboard, and the controls at the end.
// It is the window's drag handle, so the panel moves by its own header.
func (a *App) titleBar(c *ui.Context, copy i18n.Terminal) {
	t := c.Theme()
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(t.Space(7)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).DragWindow()
	revealed := bar.Hovered()
	bar.Children(func() {
		// The identity is hidden until the bar is hovered, so the title never
		// sits on top of the first lines of output.
		identity := ui.Row(c).Grow(1).Gap(t.Space(1.75)).AlignItems(ui.Center).Shrink(0)
		if !revealed {
			identity.Opacity(0)
		}
		identity.Children(func() {
			a.statusDot(c)
			title := a.Title
			if title == "" {
				title = copy.Title
			}
			ui.Text(c, title).FontSize(t.FontSize - 1).FontWeight(580).
				TextColor(t.TextMuted).Ellipsis("\u2026").SingleLine()
		})
		if a.Actions.Settings != nil {
			gear := ui.Button(c, copy.SettingsLabel).Label(copy.SettingsLabel)
			if gear.Clicked() {
				a.Actions.Settings()
			}
		}
		if a.Actions.Pin != nil && a.Term != nil {
			if ui.Button(c, copy.Pin).Clicked() {
				a.Actions.Pin(a.Title, a.sessionText())
			}
		}
		if a.Actions.NewCommand != nil {
			if ui.Button(c, copy.NewCommand).Label(copy.NewCommandHint).Clicked() {
				a.Actions.NewCommand()
			}
		}
		if a.Actions.OpenExternal != nil {
			if ui.Button(c, copy.OpenExternal).Label(copy.OpenExternalHint).Clicked() {
				a.Actions.OpenExternal()
			}
		}
		if ui.Button(c, "\u2715").Label(copy.Close).Clicked() && a.Actions.Close != nil {
			a.Actions.Close()
		}
	})
}

// statusDot is the bar's status light: the accent, breathing while the
// session runs, and a still grey dot once the program has ended.
func (a *App) statusDot(c *ui.Context) {
	t := c.Theme()
	dot := ui.Box(c).Size(t.Space(1.75), t.Space(1.75)).Radius(t.Space(1.75)).Shrink(0)
	if a.resident || a.exited() {
		dot.Background(t.TextMuted).Opacity(0.7)
		return
	}
	dot.Background(t.Accent)
	// The breath is the old build's own animation: the light fades and
	// shrinks a little, twice over five seconds, so a running session reads
	// as alive without a spinner.
	dot.Opacity(0.75 + 0.25*dot.Loop("terminal.dot", 2400*time.Millisecond, ui.Bounce(ui.EaseInOut)))
}

// exited reports whether the session's program has ended (its screen is still
// on show until the surface closes).
func (a *App) exited() bool {
	if a.Term == nil {
		return a.resident
	}
	select {
	case <-a.Term.Done():
		return true
	default:
		return a.resident
	}
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
