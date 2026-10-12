// Package terminalui draws the terminal surface: a window-wide Ghostty
// terminal for one session, or an empty state while none runs.
//
// It also owns the terminal's appearance: the settings file's font, cursor,
// line height, inset, palette and transparency, mapped onto the terminal
// plugin's options — and the appearance drawer docked at the panel's bottom
// edge (drawer.go), which edits those same settings without leaving the
// session.
package terminalui

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/inputtext"
	"floter/internal/launcher"
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
	// Settings flips the terminal's own appearance drawer: the bar's gear, so
	// the session has a way to the options that shape it *in place*. Nil
	// draws no gear at all.
	Settings func()
	// OpenLink opens what a Command+click on the terminal points at — a
	// hyperlink, a URL printed as text, or a path — and reports whether it
	// did. Nil leaves hyperlinks and URLs to the system, and paths alone.
	OpenLink func(link string) bool
	// Notify is a program's own request for a desktop notification (OSC 9 and
	// OSC 777): the title and body it asked for.
	Notify func(title, body string)
}

// Session is one terminal session: its own program, its own screen, what the
// program called it, and what became of it.
//
// The surface holds a list of them, as the old build's sessions page did: a
// session that is not on screen is not gone — its program keeps running — and
// the user can come back to it.
type Session struct {
	// Term is the running program's terminal, nil once it is closed.
	Term *terminal.Terminal
	// Title is the title the program set, empty until it sets one.
	Title string
	// Resident marks a program that has ended: the output is held on screen
	// until the user closes it, and the note at the bottom says how it went.
	Resident bool
	// ExitCode is what the program returned, meaningful while Resident.
	ExitCode int
	// Created is when the session started: the sessions list's order.
	Created time.Time
}

// App is the terminal surface's state.
type App struct {
	Store   *settings.Store
	Actions Actions

	// NewTerminal creates a session, with the options built from the
	// settings. Nil means no session can start (tests).
	NewTerminal func(terminal.Options) (*terminal.Terminal, error)

	// Sessions are the terminal's own sessions, oldest first, and Active is
	// the one on screen: -1 when there are none.
	Sessions []*Session
	Active   int
	// Err is why the last session could not start.
	Err error

	// Drawer is whether the terminal's own appearance drawer is docked at
	// the panel's bottom edge. The bar's gear flips it, and the drawer's own
	// close control closes it: the session is never left to change how it
	// looks (the old build's `.terminal-settings-drawer`).
	Drawer bool

	// Focus is the terminal view's identity, so it keeps the keyboard.
	Focus ui.Handle
	// Text is the session's text, for the pin control; nil reads it from
	// the terminal.
	Text func() string

	// feedback is the panel's floating notice — the old build's
	// `.terminal-feedback`, raised when an action the panel owns cannot run
	// (the header's "open in system terminal" with no emulator to open). A
	// newer message restarts its own clock; see feedbackNote.
	feedback      string
	feedbackShown string
	feedbackUntil time.Time

	// SnapshotPath is where the session's screen is saved when it closes and
	// read back when the next one starts, so the last session's scrollback
	// comes back. Empty disables it.
	SnapshotPath string
}

// New builds the terminal surface.
func New(store *settings.Store, actions Actions, newTerminal func(terminal.Options) (*terminal.Terminal, error)) *App {
	return &App{Store: store, Actions: actions, NewTerminal: newTerminal, Active: -1}
}

// Current is the session on screen, nil when the surface holds none.
func (a *App) Current() *Session {
	if a.Active < 0 || a.Active >= len(a.Sessions) {
		return nil
	}
	return a.Sessions[a.Active]
}

// term is the current session's terminal, nil when there is none.
func (a *App) term() *terminal.Terminal {
	if session := a.Current(); session != nil {
		return session.Term
	}
	return nil
}

// sessionTitle is the current session's title.
func (a *App) sessionTitle() string {
	if session := a.Current(); session != nil {
		return session.Title
	}
	return ""
}

// startSession creates one session and returns it, reporting why it could not.
// The optional mutators adjust the options first: a directory to start in, and
// the command to run instead of the user's shell.
func (a *App) startSession(dir string, mutate ...func(*terminal.Options)) (*Session, error) {
	if a.NewTerminal == nil {
		return nil, errors.New("terminalui: no terminal available")
	}
	opts := a.options()
	if strings.TrimSpace(dir) != "" && !isKeywordDir(dir) {
		opts.Dir = dir
	}
	for _, apply := range mutate {
		if apply != nil {
			apply(&opts)
		}
	}
	// The paste boundary (internal/inputtext). A paste is a string seam: the
	// clipboard can carry a Unicode space separator a shell would read as part
	// of a word, and — with safe paste on — the trailing line break that ran
	// the command where it was copied. The hook rewrites the text and takes the
	// paste, so the program sees the bytes the boundary approves; a paste that
	// needs no rewrite is left to the framework's own path. The setting is read
	// per paste, so the drawer's switch applies to the session on screen
	// without restarting it.
	var term *terminal.Terminal
	opts.OnPaste = func(text string) bool {
		rewritten, changed := pasteText(text, a.Store.Snapshot().TerminalPasteSafe)
		if !changed || term == nil {
			return false
		}
		term.Paste(rewritten)
		return true
	}
	created, err := a.NewTerminal(opts)
	if err != nil {
		return nil, err
	}
	term = created
	return &Session{Term: term, Created: time.Now()}, nil
}

// pasteText is the terminal's paste rule: with safe paste on, one trailing
// line break goes — a copied command almost always carries the newline that
// ran it in the terminal it came from, and pasting it here would execute
// before the user had read the line — and every Unicode space separator
// becomes U+0020 either way. It reports whether the text changed, so an
// untouched paste is the framework's own (and the hot path does not send the
// same bytes twice).
func pasteText(text string, safe bool) (string, bool) {
	rewritten := text
	if safe {
		rewritten = inputtext.StripPasteNewline(rewritten)
	}
	rewritten = inputtext.NormalizeSpaces(rewritten)
	return rewritten, rewritten != text
}

// shellCommandLine is the line a bare session is handed: the boundary's
// normalization — a Unicode space separator typed with an input method would
// fuse two shell words into one — and the trim the reference's run entry did
// before typing. Empty after that is not a command.
func shellCommandLine(command string) string {
	return strings.TrimSpace(inputtext.NormalizeSpaces(command))
}

// isKeywordDir reports a manifest's own spelling for "where the session
// starts". The registry resolves those words to real directories before an
// entry gets here (`extensions.WorkingDirectory`), so this only guards a
// direct caller that still passes a descriptor's word. The two the plugin
// resolves itself, plus the app's own directory.
func isKeywordDir(dir string) bool {
	switch dir {
	case "", "home", "current", "inherit":
		return true
	}
	return false
}

// EnsureSession starts a session if the surface has none to show. The shell
// calls it as the surface opens.
func (a *App) EnsureSession() {
	if a.NewTerminal == nil {
		return
	}
	// A held session has ended: entering the page again starts a fresh one
	// rather than showing the same finished frame.
	if session := a.Current(); session != nil && session.Resident {
		if session.Term != nil {
			_ = session.Term.Close()
		}
		a.Sessions = append(a.Sessions[:a.Active], a.Sessions[a.Active+1:]...)
		if a.Active >= len(a.Sessions) {
			a.Active = len(a.Sessions) - 1
		}
	}
	if a.Current() != nil {
		return
	}
	session, err := a.startSession("")
	if err != nil {
		a.Err = err
		return
	}
	a.Sessions = append(a.Sessions, session)
	a.Active = len(a.Sessions) - 1
	a.Err = nil
	a.restoreSnapshot(session.Term)
}

// NewSession starts another session and shows it: the old build's "new
// session" left the running one alone, and the sessions list was the way back
// to it.
func (a *App) NewSession() {
	if a.NewTerminal == nil {
		return
	}
	session, err := a.startSession("")
	if err != nil {
		a.Err = err
		return
	}
	a.Sessions = append(a.Sessions, session)
	a.Active = len(a.Sessions) - 1
	a.Err = nil
	a.FocusTerminal()
}

// SetTitle records the title the program set on the session on screen. The
// callback runs on the terminal's own goroutine, so the shell hands it to the
// main thread first.
func (a *App) SetTitle(title string) {
	if session := a.Current(); session != nil {
		session.Title = title
	}
}

// LabelOf is a session's name, by index: the title its program set, or the
// localized default.
func (a *App) LabelOf(index int) string {
	if index < 0 || index >= len(a.Sessions) {
		return i18n.For(a.Store.Snapshot().Language).Terminal.Title
	}
	if title := a.Sessions[index].Title; title != "" {
		return title
	}
	return i18n.For(a.Store.Snapshot().Language).Terminal.Title
}

// NoteOf is a session's own state line, by index: the program is running, or
// how it went — the wording the sessions page prints under the session's name.
func (a *App) NoteOf(index int) string {
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	if index < 0 || index >= len(a.Sessions) {
		return ""
	}
	session := a.Sessions[index]
	if a.Exited(index) {
		return copy.ProcessExited(session.ExitCode)
	}
	if index == a.Active {
		return copy.SessionAttached
	}
	return copy.SessionDetached
}

// DirOf is the working directory a session's shell reported (OSC 7), by
// index, or "" while it has not said — the sessions page's own first line.
func (a *App) DirOf(index int) string {
	if index < 0 || index >= len(a.Sessions) || a.Sessions[index].Term == nil {
		return ""
	}
	return a.Sessions[index].Term.Dir()
}

// SizeOf is a session's screen size in cells, by index, written "80×24";
// empty when the session has no terminal to ask.
func (a *App) SizeOf(index int) string {
	if index < 0 || index >= len(a.Sessions) || a.Sessions[index].Term == nil {
		return ""
	}
	cols, rows := a.Sessions[index].Term.Size()
	return strconv.Itoa(cols) + "\u00d7" + strconv.Itoa(rows)
}

// Exited reports whether a session's program has ended, by index.
func (a *App) Exited(index int) bool {
	if index < 0 || index >= len(a.Sessions) {
		return false
	}
	session := a.Sessions[index]
	if session.Resident {
		return true
	}
	if session.Term == nil {
		return true
	}
	select {
	case <-session.Term.Done():
		return true
	default:
		return false
	}
}

// SelectSession shows the session at an index.
func (a *App) SelectSession(index int) {
	if index < 0 || index >= len(a.Sessions) {
		return
	}
	a.Active = index
	a.FocusTerminal()
}

// CloseSession ends the session at an index and forgets it. The one on screen
// is replaced by its neighbour, or by nothing when it was the last: the
// surface then shows its empty state.
func (a *App) CloseSession(index int) {
	if index < 0 || index >= len(a.Sessions) {
		return
	}
	session := a.Sessions[index]
	if session.Term != nil {
		if index == a.Active {
			// The screen the user is looking at is the one worth saving.
			if err := a.SaveSnapshot(); err != nil {
				log.Printf("floter: could not save the terminal snapshot: %v", err)
			}
		}
		_ = session.Term.Close()
	}
	a.Sessions = append(a.Sessions[:index], a.Sessions[index+1:]...)
	switch {
	case len(a.Sessions) == 0:
		a.Active = -1
	case index < a.Active:
		a.Active--
	case index == a.Active:
		if a.Active >= len(a.Sessions) {
			a.Active = len(a.Sessions) - 1
		}
	}
	a.FocusTerminal()
}

// SaveSnapshot writes the session's screen to SnapshotPath: the escape
// sequences that reproduce what the terminal shows, which is what the next
// session is fed. A session with nothing on screen writes nothing, and a
// missing path is not an error.
func (a *App) SaveSnapshot() error {
	term := a.term()
	if a.SnapshotPath == "" || term == nil {
		return nil
	}
	data := term.Snapshot()
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
//
// The line is the boundary's other string seam: a Unicode space separator
// typed with an input method would fuse two shell words into one
// (`go<U+3000>version` is a single token to zsh), so it becomes U+0020 here,
// at the place where text becomes PTY input, rather than at each caller.
func (a *App) RunShellWithCommand(command string) {
	command = shellCommandLine(command)
	if a.NewTerminal == nil || command == "" {
		return
	}
	session, err := a.startSession("")
	if err != nil {
		a.Err = err
		return
	}
	a.Sessions = append(a.Sessions, session)
	a.Active = len(a.Sessions) - 1
	a.Err = nil
	term := session.Term
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && term.Text() == "" {
			time.Sleep(50 * time.Millisecond)
		}
		term.Send([]byte(command + "\r"))
	}()
}

// RunShellIn opens a bare session (the user's own shell) in a directory: a
// dropped file's "cd" action and the action bar's own path action. An empty
// directory keeps the session's own default.
func (a *App) RunShellIn(dir string) {
	if a.NewTerminal == nil {
		return
	}
	session, err := a.startSession(dir, nil)
	if err != nil {
		a.Err = err
		return
	}
	a.Sessions = append(a.Sessions, session)
	a.Active = len(a.Sessions) - 1
	a.Err = nil
	a.FocusTerminal()
}

func (a *App) RunCommand(argv []string, dir string, env []string) error {
	if len(argv) == 0 {
		return errors.New("terminalui: no command to run")
	}
	if a.NewTerminal == nil {
		return errors.New("terminalui: no terminal available")
	}
	session, err := a.startSession(dir, func(opts *terminal.Options) {
		opts.Command = argv
		opts.Env = append([]string{}, env...)
	})
	if err != nil {
		a.Err = err
		return err
	}
	a.Sessions = append(a.Sessions, session)
	a.Active = len(a.Sessions) - 1
	a.Err = nil
	a.FocusTerminal()
	return nil
}

// Refresh applies the appearance settings to every running session, for a
// change made while the terminal is open.
func (a *App) Refresh() {
	opts := a.options()
	for _, session := range a.Sessions {
		if session.Term == nil {
			continue
		}
		session.Term.SetFont(opts.Font)
		session.Term.SetTheme(opts.Theme, opts.DarkTheme)
	}
}

// FocusTerminal asks for the keyboard focus on the terminal.
func (a *App) FocusTerminal() { a.Focus.Focus() }

// terminalFeedbackDuration is how long the panel's floating notice stays up.
// The old build's `useTimedFeedback` gave the terminal line 4500ms, the same
// window its launcher line carried.
const terminalFeedbackDuration = 4500 * time.Millisecond

// ShowFeedback puts a message in the panel's floating notice, replacing any
// that is showing; the clock starts on the next frame that draws it.
func (a *App) ShowFeedback(message string) { a.feedback = message }

// FeedbackVisible reports whether the panel's notice is showing.
func (a *App) FeedbackVisible() bool { return a.feedback != "" }

// sessionText is the session's text, from the injected reader or the
// terminal itself.
func (a *App) sessionText() string {
	if a.Text != nil {
		return a.Text()
	}
	term := a.term()
	if term == nil {
		return ""
	}
	return term.Text()
}

// Label is the current session's name: the title the program set, or the
// localized default.
func (a *App) Label() string {
	if title := a.sessionTitle(); title != "" {
		return title
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
		// The terminal's own background carries the transparency: the panel
		// behind (the card's glass) shows through it. A named palette paints
		// its own colour at the terminal slider's alpha; the `inherit` theme
		// has no palette of its own, so the view leaves its background undrawn
		// and the card's terminal-opacity tint is what shows - the reference's
		// background-opacity semantics for the default theme. Without this the
		// plugin's default (opaque) background hid the glass entirely, so the
		// terminal slider did nothing until a named palette was picked.
		Transparent: s.TerminalTheme == "inherit",
		OnTitle:     a.Actions.Title,
		OnExit:      a.Actions.Exit,
		OpenLink:    a.Actions.OpenLink,
		OnNotify:    a.Actions.Notify,
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

// Close ends the session on screen, if any. The others keep running: a
// session the user is not looking at is not gone.
func (a *App) Close() {
	a.CloseSession(a.Active)
}

// CloseAll ends every session: the app is quitting, and a program left running
// behind a closed panel would be one nobody can reach.
func (a *App) CloseAll() {
	for len(a.Sessions) > 0 {
		a.CloseSession(0)
	}
}

// View builds the terminal surface: a title row over the session, or the
// empty state when none runs.
func (a *App) View(c *ui.Context) {
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	t := c.Theme()

	ui.Box(c).Fill().Children(func() {
		session := a.Current()
		if session == nil || session.Term == nil {
			// The empty state: what the page is, and the two lines that leave
			// it — each its own key and label, as the old build's
			// `.terminal-empty__line` drew them (the key in a badge, the label
			// beside it, neutral text rather than a filled control).
			ui.Column(c).Grow(1).FillWidth().Center().Gap(t.Space(2.5)).Children(func() {
				title := copy.EmptyTitle
				hint := copy.Hint
				if a.Err != nil {
					title, hint = copy.EmptyTitle, a.Err.Error()
				}
				ui.Text(c, title).FontSize(t.FontSize + 1).TextColor(t.TextMuted)
				ui.Text(c, hint).FontSize(t.FontSize).TextColor(t.TextMuted)
				// The keys the lines print are the keys that work: Enter
				// starts a session, Escape leaves the surface.
				if c.Shortcut(0, ui.KeyEnter) {
					a.EnsureSession()
				}
				if c.Shortcut(0, ui.KeyEscape) && a.Actions.Close != nil {
					a.Actions.Close()
				}
				a.emptyLine(c, copy, "enter", copy.EmptyNew, func() { a.EnsureSession() })
				if a.Actions.Close != nil {
					a.emptyLine(c, copy, "esc", copy.EmptyBack, a.Actions.Close)
				}
			})
			// The bar floats over the blank page too: its close control is
			// the way out the user asked for, and its dot says the page holds
			// no live session.
			a.titleBar(c, copy)
			a.feedbackNote(c)
			if a.Drawer {
				a.drawer(c, copy)
			}
			return
		}
		// The canvas fills the whole panel; the bar floats above it, so no
		// layout height is reserved and the first output line sits at the very
		// top edge (the old build's `.terminal-panel__body` over
		// `.terminal-bar`). The drawer is a sibling of the canvas, so opening
		// it shrinks the canvas (and resizes the PTY) rather than covering the
		// session's output.
		pad := a.Pad()
		// The focused canvas takes the accent edge (`.terminal-panel__mount:
		// focus-within`), the same lit frame the launcher's card takes.
		edge := tokensOf(a, c).InputStroke
		if a.Focus.Focused(c) {
			edge = tokensOf(a, c).InputStrokeActive
		}
		body := ui.Box(c).Grow(1).FillWidth()
		body.Children(func() {
			ui.Box(c).Fill().Radius(tokensOf(a, c).RadiusLG).Border(1, edge).Children(func() {
				terminal.View(c, session.Term).Fill().Padding(pad).Bind(&a.Focus)
			})
			// The resident note rides the canvas, so the docked drawer cannot
			// sit under it.
			a.residentNote(c, copy)
		})
		a.titleBar(c, copy)
		a.feedbackNote(c)
		if a.Drawer {
			a.drawer(c, copy)
		}
	})
}

// emptyLine is one of the empty state's two lines: its key in a keycap and
// its label beside it, the whole line a hit target. Neutral text, never a
// filled control — the line is furniture, not a primary button. The pointer
// lifts the *text* one step, as `.terminal-empty__line:hover` did; it does not
// paint a pane behind the line.
func (a *App) emptyLine(c *ui.Context, copy i18n.Terminal, key, label string, run func()) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	line := ui.Row(c).Focusable().Padding(t.Space(0.5), t.Space(1)).
		Radius(tokens.RadiusSM).Gap(t.Space(2)).AlignItems(ui.Center)
	color := tokens.TextTertiary
	if line.Hovered() {
		color = tokens.TextSecondary
	}
	line.Children(func() {
		// The keycap is the reference's 20u chip: the control edge and fill,
		// the keycap bevel and the xs radius — the same bevel the settings
		// recorder's keys wear, not a plain badge.
		badge := ui.Row(c).Height(t.Space(5)).Padding(0, t.Space(1.5)).Radius(tokens.RadiusXS).
			Border(1, tokens.ControlEdge).Background(tokens.Control).
			AlignItems(ui.Center).Justify(ui.Center)
		badge.InsetShadow(0, 1, 0, 0, tokens.LensRim)
		badge.Children(func() {
			ui.Text(c, keyBadge(key)).FontSize(tokens.Caption).TextColor(tokens.TextSecondary)
		})
		ui.Text(c, label).FontSize(t.FontSize).TextColor(color)
	})
	if line.Clicked() {
		run()
	}
}

// keyBadge is one of the empty state's keys as the platform spells it: `↩` and
// `esc` are the same word everywhere.
func keyBadge(key string) string {
	switch key {
	case "enter":
		return "\u21a9"
	case "esc":
		return "esc"
	}
	return key
}

// tokensOf resolves the card language's own tokens for the stored appearance.
func tokensOf(a *App, c *ui.Context) theme.Tokens {
	return theme.For(a.Store.Snapshot(), c.Theme().Dark)
}

// Resident marks the session on screen ended: the output stays on screen and
// the note says how the program went, until the user closes it or starts
// another.
func (a *App) Resident(code int) {
	if session := a.Current(); session != nil {
		session.Resident = true
		session.ExitCode = code
	}
}

// residentNote is the floating note a held session carries: what happened and
// that closing is the user's own decision, in the warning colour when the
// program returned non-zero.
func (a *App) residentNote(c *ui.Context, copy i18n.Terminal) {
	session := a.Current()
	if session == nil || !session.Resident || session.Term == nil {
		return
	}
	t := c.Theme()
	tokens := theme.For(a.Store.Snapshot(), t.Dark)
	color := tokens.TextSecondary
	if session.ExitCode != 0 {
		color = t.Warning
	}
	// The reference's own note: a floating card 12u from the panel's edges,
	// a 7u/10u padding pair, the control hairline and the floater's own fill
	// and shadow (`--glass-float` / `--elev-3`), with the title at the body
	// step in 560 — the warning colour when the program returned non-zero.
	// The note enters as the old build's `feedback-enter` did: fading in and
	// settling down from two units above.
	note := ui.Row(c).Absolute().Bottom(t.Space(3)).Left(t.Space(3)).Right(t.Space(3)).
		Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1.75), t.Space(2.5)).
		Radius(tokens.RadiusMD).Border(1, tokens.ControlEdge).
		Background(tokens.Float).
		Transition(ui.ElementTransition{
			Enter:    &ui.Motion{Y: -2},
			Duration: 160 * time.Millisecond,
		})
	// The floater's throw and its lit top rim, as `--elev-3` composed them.
	note.Shadow(0, t.Space(2.5), t.Space(6.5), 0, ui.RGBA(0, 0, 0, 0.3))
	note.Shadow(0, t.Space(0.25), t.Space(0.5), 0, ui.RGBA(0, 0, 0, 0.2))
	note.InsetShadow(0, 1, 0, 0, tokens.LensRim)
	note.Children(func() {
		ui.Text(c, copy.ProcessExited(session.ExitCode)).FontSize(t.FontSize).
			FontWeight(560).TextColor(color)
		ui.Text(c, copy.ProcessExitedHint).FontSize(t.FontSize).TextColor(t.TextMuted)
	})
}

// feedbackNote is the panel's floating notice: a short sentence an action the
// panel owns could not complete, at the panel's top-left, in the warning
// colour. It is the old build's `.terminal-feedback` — the accent keyline, the
// floater's own fill and shadow, the same `feedback-enter` settle — and its
// clock is the old build's `useTimedFeedback`: the line clears itself after
// 4500ms, and a newer message restarts the window rather than stacking.
func (a *App) feedbackNote(c *ui.Context) {
	if a.feedback == "" {
		return
	}
	now := c.Now()
	if a.feedback != a.feedbackShown {
		a.feedbackShown = a.feedback
		a.feedbackUntil = now.Add(terminalFeedbackDuration)
	}
	if now.After(a.feedbackUntil) {
		a.feedback, a.feedbackShown, a.feedbackUntil = "", "", time.Time{}
		return
	}
	// The notice is a timed line: frames must keep coming while it shows, or
	// a still panel would leave the last sentence up forever.
	c.AnimationFrame()
	t := c.Theme()
	tokens := theme.For(a.Store.Snapshot(), t.Dark)
	// The reference's own notice: 12u from the panel's top-left corner, the
	// 7u/10u padding pair, the accent keyline and the floater's fill, a 15u
	// alert glyph and the body step at 560 in the warning colour.
	notice := ui.Row(c).Absolute().Top(t.Space(3)).Left(t.Space(3)).
		Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1.75), t.Space(2.5)).
		Radius(tokens.RadiusMD).Border(1, tokens.AccentEdge).
		Background(tokens.Float).
		Transition(ui.ElementTransition{
			Enter:    &ui.Motion{Y: -2},
			Duration: 160 * time.Millisecond,
		})
	notice.Shadow(0, t.Space(2.5), t.Space(6.5), 0, ui.RGBA(0, 0, 0, 0.3))
	notice.Shadow(0, t.Space(0.25), t.Space(0.5), 0, ui.RGBA(0, 0, 0, 0.2))
	notice.InsetShadow(0, 1, 0, 0, tokens.LensRim)
	notice.Children(func() {
		if glyph, ok := launcher.Glyph("alert"); ok {
			ui.Icon(c, glyph).Size(t.Space(3.75), t.Space(3.75)).TextColor(t.Warning)
		}
		ui.Text(c, a.feedback).FontSize(t.FontSize).FontWeight(560).TextColor(t.Warning)
	})
}

// titleBar is the floating bar at the panel's top edge: the session's
// identity (a live dot and the title) at the start, revealed while the bar is
// hovered or something on it has the keyboard, and the controls at the end.
// It is the window's drag handle, so the panel moves by its own header.
// barButton is one of the terminal bar's own controls: a 28u transparent
// square with a 16u mark, as the reference's `.toolbar-button` — not a filled
// pill, which in a floating strip would be the loudest thing on the panel.
// label is the control's accessible name, hint the sentence its tooltip
// carries once the pointer rests (the old header's own `title=`).
func (a *App) barButton(c *ui.Context, name, label, hint string, run func()) {
	t := c.Theme()
	button := ui.Row(c).Focusable().Size(t.Space(7), t.Space(7)).Shrink(0).
		Radius(tokensOf(a, c).RadiusSM).AlignItems(ui.Center).Justify(ui.Center).Label(label).
		Tooltip(hint)
	if button.Hovered() {
		button.Background(t.SurfaceHover)
	}
	button.Children(func() {
		if glyph, ok := launcher.Glyph(name); ok {
			ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
		}
	})
	if button.Clicked() {
		run()
	}
}

func (a *App) titleBar(c *ui.Context, copy i18n.Terminal) {
	t := c.Theme()
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(t.Space(7)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).DragWindow()
	// The bar's whole content is hidden until the pointer is over it or
	// something on it has the keyboard, so neither the title nor the buttons
	// permanently occlude the first lines of output under them
	// (`.terminal-bar__identity` / `.terminal-panel__actions` reveal the same
	// way).
	revealed := bar.Hovered() || bar.FocusWithin()
	bar.Children(func() {
		identity := ui.Row(c).Grow(1).Gap(t.Space(1.75)).AlignItems(ui.Center).Shrink(0)
		if !revealed {
			identity.Opacity(0)
		}
		identity.Children(func() {
			a.statusDot(c)
			title := a.sessionTitle()
			if title == "" {
				title = copy.Title
			}
			ui.Text(c, title).FontSize(t.FontSize - 1).FontWeight(580).
				TextColor(t.TextMuted).Ellipsis("\u2026").SingleLine()
		})
		// The controls, at the bar's trailing edge: 28u transparent squares
		// with a 4u gap, hidden and non-interactive until the bar is revealed
		// (a hidden button must not catch a click meant for the terminal
		// underneath).
		actions := ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Shrink(0)
		if !revealed {
			actions.Opacity(0).PassThrough()
		}
		actions.Children(func() {
			if a.Actions.Settings != nil {
				// The gear flips to an X while the drawer is up: the control
				// that opens a module is the one that closes it, in the same
				// spot (the reference's `toolbar-button--settings-open`).
				name, label, hint := "settings", copy.SettingsOpen, copy.SettingsOpenHint
				if a.Drawer {
					name, label, hint = "close", copy.SettingsClose, copy.SettingsClose
				}
				a.barButton(c, name, label, hint, a.Actions.Settings)
			}
			if a.Actions.Pin != nil && a.term() != nil {
				a.barButton(c, "text", copy.Pin, copy.Pin, func() { a.Actions.Pin(a.sessionTitle(), a.sessionText()) })
			}
			if a.Actions.NewCommand != nil {
				a.barButton(c, "command", copy.NewCommand, copy.NewCommandHint, a.Actions.NewCommand)
			}
			if a.Actions.OpenExternal != nil {
				a.barButton(c, "terminal", copy.OpenExternal, copy.OpenExternalHint, a.Actions.OpenExternal)
			}
			if a.Actions.Close != nil {
				a.barButton(c, "close", copy.Close, copy.Close, a.Actions.Close)
			}
		})
	})
}

// statusDot is the bar's status light: the accent, breathing while the
// session runs, and a still grey dot once the program has ended.
func (a *App) statusDot(c *ui.Context) {
	t := c.Theme()
	dot := ui.Box(c).Size(t.Space(1.75), t.Space(1.75)).Radius(t.Space(1.75)).Shrink(0)
	if a.exited() {
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
	session := a.Current()
	if session == nil {
		return false
	}
	if session.Term == nil {
		return session.Resident
	}
	select {
	case <-session.Term.Done():
		return true
	default:
		return session.Resident
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

// PaletteColors is the four colors a palette paints — its background, text,
// cursor and selection — as the settings page's preview shows them (R43: the
// user asked a palette to show what it does). The background is opaque: the
// preview is about the colors, not the transparency the live terminal also
// applies.
type PaletteColors struct {
	Background, Foreground, Cursor, Selection ui.Color
}

// PaletteColorsFor returns the colors a stored palette id paints. The shipped
// `inherit` follows the window's appearance, so it takes dark; an override
// paints its own fixed pair. ok is false for an id this build does not know.
func PaletteColorsFor(name string, dark bool) (PaletteColors, bool) {
	if spec, ok := palettes[name]; ok {
		return PaletteColors{
			Background: packed(spec.bg),
			Foreground: packed(spec.fg),
			Cursor:     packed(spec.cursor),
			Selection:  spec.selection,
		}, true
	}
	if name != "inherit" {
		return PaletteColors{}, false
	}
	// Follow app: the terminal's own default theme, whose cursor and
	// selection fall back to the foreground when a theme leaves them zero
	// (the plugin's own rule).
	base := terminal.DarkTheme()
	if !dark {
		base = terminal.LightTheme()
	}
	cursor := base.Cursor
	if cursor.A == 0 {
		cursor = base.Foreground
	}
	selection := base.Selection
	if selection.A == 0 {
		selection = base.Foreground.Alpha(0.33)
	}
	return PaletteColors{
		Background: base.Background,
		Foreground: base.Foreground,
		Cursor:     cursor,
		Selection:  selection,
	}, true
}

// PaletteMono is the preview's type: a terminal face, as the reference's
// `.terminal-palette-preview` had it.
const PaletteMono = "SF Mono, Menlo, Monaco, Consolas, monospace"

// PalettePreview paints the miniature terminal the palette picker shows: three
// lines of static text in the palette's own background and text, the middle
// line behind its selection tint and a block cursor at the end — the
// reference's `.terminal-palette-preview`, which costs no canvas and no PTY.
//
// A width of zero fills the parent, which is how the settings page draws it;
// a fixed width is the terminal drawer's own column, where the preview sits
// beside the picker rather than under it.
func PalettePreview(c *ui.Context, tokens theme.Tokens, name, label string, width float32) {
	t := c.Theme()
	colors, ok := PaletteColorsFor(name, t.Dark)
	if !ok {
		return
	}
	box := ui.Column(c).Padding(t.Space(3), t.Space(4)).
		Gap(1).Border(1, tokens.ControlEdge).Radius(tokens.RadiusSM).
		Background(colors.Background).Clip().Label(label).Shrink(0)
	if width > 0 {
		box.Width(width)
	} else {
		box.FillWidth().MarginX(t.Space(3.5))
	}
	box.Children(func() {
		// The prompt line: the prompt a shade quieter than its command, as a
		// real prompt reads.
		promptLine := ui.Row(c).Shrink(0).Gap(t.Space(1)).AlignItems(ui.Center)
		promptLine.Children(func() {
			ui.Text(c, "$").Font(PaletteMono).FontSize(t.FontSize - 1).
				TextColor(colors.Foreground).Opacity(0.72)
			ui.Text(c, "floter --version").Font(PaletteMono).FontSize(t.FontSize - 1).
				TextColor(colors.Foreground)
		})
		// The output line wears the palette's selection tint behind it.
		selected := ui.Row(c).Shrink(0).Background(colors.Selection)
		selected.Children(func() {
			ui.Text(c, "0.3.5-preview").Font(PaletteMono).FontSize(t.FontSize - 1).
				TextColor(colors.Foreground)
		})
		// The last prompt, with the palette's block cursor.
		cursorLine := ui.Row(c).Shrink(0).Gap(t.Space(1)).AlignItems(ui.Center)
		cursorLine.Children(func() {
			ui.Text(c, "$").Font(PaletteMono).FontSize(t.FontSize - 1).
				TextColor(colors.Foreground).Opacity(0.72)
			ui.Box(c).Width(t.Space(1.25)).Height(t.Space(2.5)).Shrink(0).
				Background(colors.Cursor)
		})
	})
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
