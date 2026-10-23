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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/inputtext"
	"floter/internal/launcher"
	"floter/internal/motion"
	"floter/internal/settings"
	"floter/internal/shortcuts"
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
	// Exit is called when a session's program ends, with the session it
	// happened to and the code it returned. The session is handed over rather
	// than an index: the plugin calls this on the terminal's own goroutine,
	// where reading the session list would race the main thread, and the main
	// thread turns the session back into its place (see IndexOf).
	Exit func(session *Session, code int)
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
	// ID is the session's stable identity, handed out once at creation and
	// never reused: the sessions list arms a close by identity, so a session
	// that changed position — because one ahead of it was deleted — still
	// answers to the same question (the reference's `killArmedId` keyed on
	// `session.sessionId`, `src/settings/SessionsPage.tsx:54`). Zero means a
	// session built by hand (tests), which never takes part in a list arm.
	ID uint64
	// Term is the running program's terminal, nil once it is closed.
	Term *terminal.Terminal
	// Title is the title the program set, empty until it sets one.
	Title string
	// Resident marks a program that has ended: the output is held on screen
	// until the user closes it, and the note at the bottom says how it went.
	Resident bool
	// ExitCode is what the program returned, meaningful while Resident.
	ExitCode int
	// CommandStarted marks a session that exists only to carry a command: the
	// launcher's structured execution plans and its own shell-syntax line. When
	// such a program ends the page leaves with it (R62), rather than holding a
	// finished frame. A bare shell — the system terminal row, the held-
	// modifier row, Enter on the empty page, an install row whose command is
	// typed into the user's own shell — is the user's terminal and never
	// counts.
	CommandStarted bool
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
	// nextSessionID is the source of Session.ID: bumped at every creation, so
	// the identity a list arm holds still names the same session after a
	// deletion shifted the slice (see Session.ID).
	nextSessionID uint64
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
	// The page keeps a bounded number of sessions. Every session is a shell
	// process, and a hand-off a held shortcut repeats (or a key the desktop
	// repeats) would otherwise fork one per repeat until the machine has
	// nothing left to draw a frame with — the report 「这边的某些快捷键会
	// 导致反复弹出终端，从而导致卡死」. The oldest session is the one that
	// goes, which is the one the user stopped looking at first.
	a.trimSessions()
	opts := a.options()
	if strings.TrimSpace(dir) != "" && !isKeywordDir(dir) {
		opts.Dir = dir
	}
	for _, apply := range mutate {
		if apply != nil {
			apply(&opts)
		}
	}
	// The paste boundary (internal/inputtext). A paste is a text seam: the
	// clipboard can carry a Unicode space separator a shell would read as part
	// of a word, and — with safe paste on — the trailing line break that ran
	// the command where it was copied. The hook reads the clipboard's text
	// representation, rewrites it, and takes the paste so the program sees the
	// bytes the boundary approves; a paste with no text representation (an
	// image, a file list) or one that needs no rewrite is left to the
	// framework's own path, which handles every format. The setting is read
	// per paste, so the drawer's switch applies to the session on screen
	// without restarting it.
	var term *terminal.Terminal
	var session *Session
	opts.OnPaste = func(d transfer.Data) bool {
		raw, err := d.Read(transfer.Text)
		if err != nil {
			// No text to rewrite: leave the framework to paste the clipboard's
			// own formats (an image, a file list).
			return false
		}
		rewritten, changed := pasteText(string(raw), a.Store.Snapshot().TerminalPasteSafe)
		if !changed || term == nil {
			return false
		}
		term.Paste(rewritten)
		return true
	}
	// The exit belongs to the session it happened to, not to whichever session
	// is on screen: the surface holds several, a background program can end
	// while the user looks at another, and the plugin's callback carries no
	// identity of its own.
	opts.OnExit = func(code int) {
		if session == nil || a.Actions.Exit == nil {
			return
		}
		a.Actions.Exit(session, code)
	}
	created, err := a.NewTerminal(opts)
	if err != nil {
		return nil, err
	}
	term = created
	a.nextSessionID++
	session = &Session{ID: a.nextSessionID, Term: term, Created: time.Now()}
	return session, nil
}

// maxSessions is how many sessions the page keeps alive at once.
const maxSessions = 16

// trimSessions closes the oldest sessions until one more can be added, leaving
// the one on screen alone: a session the user is looking at is never the one
// collected for room.
func (a *App) trimSessions() {
	for len(a.Sessions) >= maxSessions {
		victim := -1
		for i := range a.Sessions {
			if i != a.Active {
				victim = i
				break
			}
		}
		if victim < 0 {
			return
		}
		if session := a.Sessions[victim]; session.Term != nil {
			_ = session.Term.Close()
		}
		a.Sessions = append(a.Sessions[:victim], a.Sessions[victim+1:]...)
		if victim < a.Active {
			a.Active--
		}
	}
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
//
// The ended line is the old page's own (`main:src/settings/SessionsPage.tsx:173-177`):
// `terminal.sessionExited` alone for a clean exit, and only a non-zero code
// gets the ` · exit N` suffix. The resident note on the terminal surface keeps
// the separate, always-coded `terminal.processExited` wording.
func (a *App) NoteOf(index int) string {
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	if index < 0 || index >= len(a.Sessions) {
		return ""
	}
	session := a.Sessions[index]
	if a.Exited(index) {
		if session.ExitCode == 0 {
			return copy.SessionExited
		}
		return copy.SessionExited + " \u00b7 exit " + strconv.Itoa(session.ExitCode)
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

// RunCommand starts a session whose own program is argv: how an extension's
// command, and the launcher's structured line, are handed to the terminal.
// dir names the working directory policy the provider asked for ("home",
// "current", "inherit"), and env is the integration's configured environment
// ("KEY=value"). The session is command-started: when the program ends the
// page leaves with it (R62).
// RunShellWithCommand opens a new bare session (no command: the user's own
// shell) and types a command line into it. The install rows use it: the shell
// is the user's and outlives the install, and the command is typed rather than
// run, so the session is interactive when the install finishes and a finished
// frame is still worth holding.
//
// The command is typed once the session has produced its first output (a
// prompt); the deadline keeps a shell that never prints from holding the app.
//
// The line is the boundary's other string seam: a Unicode space separator
// typed with an input method would fuse two shell words into one
// (`go<U+3000>version` is a single token to zsh), so it becomes U+0020 here,
// at the place where text becomes PTY input, rather than at each caller.
func (a *App) RunShellWithCommand(command string) {
	a.runShellSession(command, false)
}

// RunCommandLine is the launcher's own shell-syntax line ("Run in terminal",
// which nothing but a shell can read: `ls | grep go`). The session exists to
// carry that line, so it is command-started and the page leaves with the shell
// (R62) — the reference's own distinction between `initialCommand` (the line is
// the point) and the install rows' bare spawn (the shell is the point).
func (a *App) RunCommandLine(command string) {
	a.runShellSession(command, true)
}

// runShellSession is the one body behind the two shell hand-offs; they differ
// only in whether the session exists to carry the line.
func (a *App) runShellSession(command string, carriesCommand bool) {
	command = shellCommandLine(command)
	if a.NewTerminal == nil || command == "" {
		return
	}
	session, err := a.startSession("")
	if err != nil {
		a.Err = err
		return
	}
	session.CommandStarted = carriesCommand
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
	session.CommandStarted = true
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
		session.Term.SetCopyOnSelect(opts.CopyOnSelect)
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

// options builds the plugin options from the settings. OnExit is left unset
// here: it names the session it belongs to, which only startSession knows (see
// its comment).
func (a *App) options() terminal.Options {
	s := a.Store.Snapshot()
	theme, darkTheme := themeFor(s.TerminalTheme)
	return terminal.Options{
		Font: terminal.Font{
			Family:     s.FontFamily,
			Size:       float32(s.FontSize),
			LineHeight: float32(s.TerminalLineHeight),
		},
		Cursor:    cursorStyle(s.CursorShape),
		NoBlink:   !s.CursorBlink,
		Theme:     theme,
		DarkTheme: darkTheme,
		// The selection that ends lands on the clipboard, as the reference's
		// `terminal_select_copy` did (`main:src/App.tsx:875` → the renderer's
		// `selectCopy`, which copied on `selectionchange`). It shipped with no
		// reader at all: the settings page recorded it and the terminal surface
		// never asked, so the switch did nothing. The plugin owns both halves —
		// `Options.CopyOnSelect` at creation and `SetCopyOnSelect` for a session
		// already running (Refresh).
		CopyOnSelect: copyOnSelect(s.TerminalSelectCopy),
		// The view never paints a background of its own (P7-dr). The transparency
		// is the terminal slider's, and it lives one layer up: this surface paints
		// the palette's colour - or, for the shipped `inherit`, nothing at all, so
		// the card's terminal-opacity tint shows - in `View` (see canvasFill).
		//
		// This is a constant, not a reading of the stored palette, because
		// `Transparent` is a creation-time option with no setter: a session that
		// started on a named palette kept painting its background after the user
		// switched back to `inherit` (an opaque one, which is the very thing the
		// slider is for), and a session that started on `inherit` never painted a
		// palette picked later. The reference met the same two cases by rebuilding
		// its renderer on every mount; here the layer the palette colours is the
		// app's own, so the setting reaches a running session on the next frame.
		Transparent: true,
		// The grid's room around the text, which the padding step owns
		// (compact 1, regular 3, relaxed 6 DIPs — the three values
		// `main:src/terminal/terminal-appearance.ts:85-87` shipped, read by
		// the reference's canvas renderer as both `paddingX` and
		// `paddingY`).
		//
		// The element's own `Padding(pad)` used to be the carrier and never
		// worked: an element's padding insets its *children*, and the
		// terminal's view is a leaf whose draw callback is handed the whole
		// border box, so the setting reached nothing and the grid sat at the
		// plugin's fixed 6 across and 4 down whatever the user picked. The
		// plugin takes the number itself (Options.Padding, mygo v0.6.0), so
		// this is where the setting now lands.
		//
		// PaddingColor stays the zero value (PaddingBackground), which with
		// Transparent paints nothing at all in the padding: the same empty
		// room the reserved inset already left, so the change moves where the
		// text starts and leaves the surface under it alone.
		//
		// A creation-time option with no setter, like Transparent above: a
		// session already running keeps the inset it started with, and the
		// next one takes the new step. That is not a regression but the first
		// time the step does anything at all.
		Padding:  terminal.Padding{X: a.Pad(), Y: a.Pad()},
		OnTitle:  a.Actions.Title,
		OpenLink: a.Actions.OpenLink,
		OnNotify: a.Actions.Notify,
		Keybinds: terminalKeybinds(s),
	}
}

// copyOnSelect maps the terminal's `select_copy` setting onto the plugin's
// policy. The plugin has two stops — copy the selection to the clipboard, or
// copy nothing — and the setting is the same two: a finished selection lands
// on the clipboard, or it waits for `copy_selection`.
func copyOnSelect(on bool) terminal.CopyOnSelect {
	if on {
		return terminal.CopyOnSelectClipboard
	}
	return terminal.CopyOnSelectNone
}

// terminalKeybinds binds the app's own `copy_selection` and `paste` shortcuts
// onto the terminal's own actions, which is where the reference answered them
// (`main:src/hooks/useAppKeyboard.ts:214-226`: `copy_selection` before `paste`,
// after the dismiss table and `open_external_terminal`).
//
// The port shipped both settings with no reader at all — the settings page
// recorded them and nothing on the terminal surface answered them — so a
// rebind was a lie. They are bindings of the plugin rather than a shortcut on
// the element around it because only the plugin can reach the selection: its
// `ActionCopy` copies what is selected, and `Performable` gives it the
// reference's condition exactly (a copy claims the key only when there is a
// selection, so a `Ctrl+C` binding still interrupts the foreground process —
// `main:src/hooks/useAppKeyboard.ts:219-222`, "Copy only claims the combination
// when there is something to copy"). Paste is not performable, as the
// reference claimed it unconditionally.
//
// These are creation-time options: a session already running keeps the keys it
// started with, and a rebind reaches the next one (see Refresh, which has no
// binding setter to call).
func terminalKeybinds(s settings.Settings) []terminal.Keybind {
	m := settings.ShortcutsOf(s)
	var binds []terminal.Keybind
	add := func(action string, a terminal.Action, performable bool) {
		mods, key, ok := shortcuts.Parse(m[action])
		if !ok {
			return
		}
		binds = append(binds, terminal.Keybind{Mods: mods, Key: key, Action: a, Performable: performable})
	}
	add(settings.ShortcutCopySelection, terminal.ActionCopy, true)
	add(settings.ShortcutPaste, terminal.ActionPaste, false)
	return binds
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
	stored := a.Store.Snapshot()
	copy := i18n.For(stored.Language).Terminal
	t := c.Theme()
	tokens := tokensOf(a, c)

	// The panel's own background is **one** layer, at the alpha the terminal
	// slider asks for less the share the card's face already carries
	// (glassmap.Spec.CanvasAlpha). Both layers used to paint the slider, so 46%
	// showed as `1-(1-0.46)^2` = 71% (「终端透明度滚轴说的和看到的不一致」); the
	// frame now carries its share and this one the rest, and the two composite
	// back to the slider once.
	//
	// The colour is the palette's own for a named palette, and the window's for
	// the shipped `inherit` — so an inherit session looks exactly as it did,
	// while a palette reaches a running session on the next frame (the plugin's
	// view is transparent; see canvasFill). The empty page takes the window's
	// colour too: there is no session whose palette it would be, and the page's
	// own ink has to stay readable on it.
	spec := glassmap.SpecForPreferencesOS(runtime.GOOS, stored.GlassStep, stored.TerminalOpacity, c.Preferences().HighContrast)
	session := a.Current()
	running := session != nil && session.Term != nil
	// The panel is the window's own face (terminal.css:31 draws it at
	// `--window-radius`), so its corner is the window's: 12 on all three
	// platforms (docs/raycast-design.md §2). P8-c 复核（§6.2 第 1 条）：半径
	// 保留，`panelFill` 只有一层面（调色板的底色 × `CanvasAlpha`），不叠
	// rim/sheen/lens，所以它身上没有需要压到 0.04 以下的光泽。
	ui.Box(c).Fill().Radius(panelCorner(tokens)).Background(panelFill(stored.TerminalTheme, running, spec, tokens)).Children(func() {
		if session == nil || session.Term == nil {
			// The empty state: what the page is, and the two lines that leave
			// it — each its own key and label, as the old build's
			// `.terminal-empty__line` drew them (the key in a badge, the label
			// beside it, neutral text rather than a filled control).
			ui.Column(c).Grow(1).FillWidth().Center().Gap(t.Space(2.5)).Children(func() {
				// The reference's empty state is exactly three things: the title and
				// the two lines that leave the page (`main:src/App.tsx:3852-3872`
				// draws `terminal-empty__title` over two `terminal-empty__line`s and
				// no second sentence), so no static tip is printed between them. A
				// failed start is the one addition: its reason is shown here instead
				// of the lines' own silence.
				ui.Text(c, copy.EmptyTitle).FontSize(t.FontSize + 1).TextColor(t.TextMuted)
				if a.Err != nil {
					ui.Text(c, a.Err.Error()).FontSize(t.FontSize).TextColor(t.TextMuted)
				}
				// The keys the lines print are the keys that work: Enter starts a
				// session, and the back line is the user's own new-command binding,
				// as the old build's `formatShortcut(shortcuts.new_command)` drew it.
				// Escape is not this page's: on the terminal surface it belongs to
				// the shell (`main:src/surface-policy.ts` `DISMISS_TABLE.terminal
				// .escape` is null), and the new-command chord is claimed by the
				// window handler (`appShortcuts` -> `returnToInput`), so the page
				// prints it rather than claiming a second copy.
				if c.Shortcut(0, ui.KeyEnter) {
					a.EnsureSession()
				}
				a.emptyLine(c, copy, keyBadge("enter"), copy.EmptyNew, func() { a.EnsureSession() })
				if a.Actions.NewCommand != nil {
					a.emptyLine(c, copy, newCommandBadge(a.Store.Snapshot()), copy.EmptyBack, a.Actions.NewCommand)
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
		// The focused canvas takes the accent edge (`.terminal-panel__mount:
		// focus-within`), the same lit frame the launcher's card takes.
		edge := tokensOf(a, c).InputStroke
		if a.Focus.Focused(c) {
			edge = tokensOf(a, c).InputStrokeActive
		}
		body := ui.Box(c).Grow(1).FillWidth()
		body.Children(func() {
			// The mount's fill is the terminal's background: the session's own
			// name for the surface it is drawn on, painted here rather than by
			// the plugin (the session is transparent, see options). Rebuilt from
			// the stored settings every frame, so a palette picked while a
			// session runs lands without restarting it.
			//
			// P8-c 重定基线（§6.2 第 2 条）：画布圆角 12 → `RadiusMD`(10)。12
			// 是「这张面**是**窗口」的档（§2：窗口、整张卡），而终端画布是面板
			// **里**的一块，和分组卡/浮层同档；面板自己仍是 `WindowRadius`(12)。
			ui.Box(c).Fill().Radius(mountCorner(tokensOf(a, c))).Border(1, edge).
				Children(func() {
					// No `Padding` here: the inset belongs to the grid and is
					// Options.Padding now (see options). An element's padding
					// insets its children, which a terminal's view has none
					// of, so the call that used to sit here was inert.
					terminal.View(c, session.Term).Fill().Bind(&a.Focus).Label(copy.Input)
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
// paint a pane behind the line. `badgeText` is the keycap's already-spelled
// text (`keyBadge("enter")`, `newCommandBadge(...)`).
func (a *App) emptyLine(c *ui.Context, copy i18n.Terminal, badgeText, label string, run func()) {
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
			ui.Text(c, badgeText).FontSize(tokens.Caption).TextColor(tokens.TextSecondary)
		})
		ui.Text(c, label).FontSize(t.FontSize).TextColor(color)
	})
	if line.Clicked() {
		run()
	}
}

// keyBadge is one of the empty state's fixed keys as the platform spells it:
// `↩` is the same glyph everywhere.
func keyBadge(key string) string {
	if key == "enter" {
		return "\u21a9"
	}
	return key
}

// newCommandBadge is the empty state's back key as the user's own binding
// spells it. The old build printed `formatShortcut(shortcuts.new_command)`
// there — a configurable accelerator, not a literal Escape — so a Mac reads
// ⌘W and elsewhere the app's own binding word for word.
func newCommandBadge(s settings.Settings) string {
	return shortcuts.Display(settings.Shortcut(s, settings.ShortcutNewCommand))
}

// barHints are the two tooltips the bar's labeled controls carry, each naming
// the chord that does the same thing from the keyboard. The reference read
// `formatShortcut(shortcuts.new_command)` / `…open_external_terminal` at the
// moment it drew its header (`main:src/App.tsx:3764-3790`), so a rebinding
// moves the word in the tooltip with the key; the pair is a pure function of
// the stored settings so a test can drive that without a hovered pointer.
func barHints(copy i18n.Terminal, s settings.Settings) (newCommand, openExternal string) {
	return copy.NewCommandHint(newCommandBadge(s)),
		copy.OpenExternalHint(shortcuts.Display(settings.Shortcut(s, settings.ShortcutOpenExternalTerminal)))
}

// tokensOf resolves the card language's own tokens for the stored appearance.
func tokensOf(a *App, c *ui.Context) theme.Tokens {
	return theme.ForPreferences(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
}

// Resident marks the session on screen ended: the output stays on screen and
// the note says how the program went, until the user closes it or starts
// another.
func (a *App) Resident(code int) {
	a.ResidentAt(a.Active, code)
}

// ResidentAt marks the session at an index ended. The index comes from a
// callback through IndexOf, so a background program's exit is recorded on its
// own session rather than on whichever one the user happens to be looking at.
func (a *App) ResidentAt(index, code int) {
	if index < 0 || index >= len(a.Sessions) {
		return
	}
	a.Sessions[index].Resident = true
	a.Sessions[index].ExitCode = code
}

// IndexOf is a session's place in the list, -1 when it is gone. A callback
// carries the session itself (see Actions.Exit); the main thread turns it back
// into an index here, where the list is only its own to read.
func (a *App) IndexOf(session *Session) int {
	for i, candidate := range a.Sessions {
		if candidate == session {
			return i
		}
	}
	return -1
}

// ShouldExitWithCommandSession is R62's rule, the reference's own predicate
// (`src/terminal/command-session.ts:47`): a session started only to carry a
// command takes the terminal page with it when the program ends, but only while
// the terminal is the surface the user is on. A bare shell never counts —
// exiting the user's own shell is the user closing their terminal, not the app
// closing a page.
func ShouldExitWithCommandSession(commandStarted, terminalOnScreen bool) bool {
	return commandStarted && terminalOnScreen
}

// CommandSessionEnded applies R62 to a session whose program has just ended and
// reports whether the caller should return to the launcher. terminalOnScreen is
// the shell's own fact (the terminal is the surface on the user's screen); the
// ended session must also be the one on screen, this build's refinement of the
// reference's single main session — an exit cannot yank the user out of another
// session they moved to. The session is dropped without a snapshot: a one-shot
// command's output is not the scrollback the next shell should inherit.
func (a *App) CommandSessionEnded(index int, terminalOnScreen bool) bool {
	if index < 0 || index >= len(a.Sessions) {
		return false
	}
	onScreen := terminalOnScreen && index == a.Active
	if !ShouldExitWithCommandSession(a.Sessions[index].CommandStarted, onScreen) {
		return false
	}
	if session := a.Sessions[index]; session.Term != nil {
		_ = session.Term.Close()
	}
	a.Sessions = append(a.Sessions[:index], a.Sessions[index+1:]...)
	switch {
	case len(a.Sessions) == 0:
		a.Active = -1
	case index < a.Active:
		a.Active--
	case index == a.Active && a.Active >= len(a.Sessions):
		a.Active = len(a.Sessions) - 1
	}
	return true
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
	tokens := theme.ForPreferences(a.Store.Snapshot(), t.Dark, c.Preferences())
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
	tokens := theme.ForPreferences(a.Store.Snapshot(), t.Dark, c.Preferences())
	// The reference's own notice: 12u from the panel's top-left corner, the
	// 7u/10u padding pair, the floater's fill, a 15u alert glyph and the body
	// step at 560 in the warning colour.
	//
	// P8-c 重定基线（§6.2 第 4 条 / §5）：环 `AccentEdge` → `ControlEdge`。accent
	// 只回答「哪个被选中/哪个是开的」，而这条是「某个动作没做成」——它的语义
	// 已经由那行 warning 墨和 15u 的警示字形说了，一圈 accent 环不在 §5 的
	// 任何合法处。几何（12u 起点、28u/40u 的 padding、`RadiusMD`）零改动。
	notice := ui.Row(c).Absolute().Top(t.Space(3)).Left(t.Space(3)).
		Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1.75), t.Space(2.5)).
		Radius(tokens.RadiusMD).Border(1, tokens.ControlEdge).
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
		// The pointer lights the resting lens rung (`.toolbar-button:hover`,
		// the reference's hover-only variant): the bar's icon button is
		// transparent at rest, so this is the only state it can be graded in.
		tokensOf(a, c).Lens(button)
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
	newCommandHint, openExternalHint := barHints(copy, a.Store.Snapshot())
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(t.Space(7)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).DragWindow()
	// The bar's whole content is hidden until the pointer is over it or
	// something on it has the keyboard, so neither the title nor the buttons
	// permanently occlude the first lines of output under them
	// (`.terminal-bar__identity` / `.terminal-panel__actions` reveal the same
	// way).
	revealed := bar.Hovered() || bar.FocusWithin()
	// `.terminal-bar:hover { box-shadow: var(--elev-bar) }` (`terminal.css:99-104`):
	// reaching for the bar re-casts a light shadow — `0 6px 16px` at
	// `0.18 * --elev-shadow-scale` — onto the content scrolling under it, the
	// reference's one cue that the bar itself is a draggable handle. Only the
	// pointer raises it; keyboard focus reveals the identity without lifting the
	// surface, exactly as `:hover` does.
	if bar.Hovered() {
		// P8-c 重定基线（§6.2 第 6 条）：投影读 `ShadowContact`，不再写字面
		// `rgba(0, 0, 0, 0.18)`。同一条 throw 在两套外观下是两档（深 0.40 /
		// 浅 0.10），字面值把它钉成了第三档，浅色下比合约重、深色下比合约轻。
		bar.Shadow(0, t.Space(1.5), t.Space(4), 0, tokensOf(a, c).ShadowContact)
	}
	bar.Children(func() {
		// The identity line names the program on screen and, once it has ended,
		// the code it returned — the reference's `identityTitle`, which is both
		// the bar's visible text and the container's `title=`. It is a different
		// word from the resident note's `ProcessExited`; both appear together for
		// a held session.
		identityTitle := a.sessionTitle()
		if identityTitle == "" {
			identityTitle = copy.Title
		}
		if session := a.Current(); session != nil && session.Resident {
			identityTitle += " \u00b7 " + copy.HeaderExited(session.ExitCode)
		}
		identity := ui.Row(c).Grow(1).Gap(t.Space(1.75)).AlignItems(ui.Center).Shrink(0).
			Label(identityTitle).Tooltip(identityTitle)
		if !revealed {
			identity.Opacity(0)
		}
		identity.Children(func() {
			a.statusDot(c)
			// `.terminal-bar__title` (`terminal.css:150-161`): the body step in
			// `--text-secondary`, 580, with the reference's `letter-spacing:
			// -0.01em`. The identity line used to read the muted step one size
			// down, which made the name of the session on screen the quietest
			// thing in the bar it names.
			ui.Text(c, identityTitle).FontSize(t.FontSize).FontWeight(580).
				LetterSpacing(-0.01 * t.FontSize).
				TextColor(tokensOf(a, c).TextSecondary).Ellipsis("\u2026").SingleLine()
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
				a.barButton(c, "command", copy.NewCommand, newCommandHint, a.Actions.NewCommand)
			}
			if a.Actions.OpenExternal != nil {
				a.barButton(c, "terminal", copy.OpenExternal, openExternalHint, a.Actions.OpenExternal)
			}
			if a.Actions.Close != nil {
				a.barButton(c, "close", copy.Close, copy.Close, a.Actions.Close)
			}
		})
	})
}

// statusDot is the bar's status light: a neutral dot breathing while the
// session runs, and a still grey dot once the program has ended.
func (a *App) statusDot(c *ui.Context) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	dot := ui.Box(c).Size(t.Space(1.75), t.Space(1.75)).Radius(t.Space(1.75)).Shrink(0)
	if a.exited() {
		// `.terminal-bar__dot--exited` (`terminal.css:163-168`): the tertiary
		// step, the halo dropped, at the reference's 0.7 opacity. It used to
		// read the *muted* step, one rung brighter than the reference's.
		dot.Background(tokens.TextTertiary).Opacity(0.7)
		return
	}
	// P8-e 重定基线（§5 判据）：旧值是 `t.Accent`，但「这个进程是活的」不是
	// 「哪个被选中 / 哪个是开的」，accent 无权回答它——所以色相转中性，
	// `tokens.TextTertiary`，与 exited 态同一档（两者只差明度与动效）。存活由
	// 「动」说：呼吸仍在，色相不再参与。旧值 → 新值：`t.Accent` →
	// `tokens.TextTertiary`（深色下峰值仍读得出，两侧实测见 P8-e 报告）。
	dot.Background(tokens.TextTertiary)
	// P8-c 重定基线（§6.2 第 5 条 / §1）：`--elev-halo` 的 2u `AccentHalo` 环
	// 去掉，只留这个 1.75u 的点。光环是旧 glass 语言的辉光——§1 把
	// 光泽一律压到 alpha ≤ 0.04，而 `AccentHalo` 是 0.2（上限的 5 倍），并且
	// 「哪个进程是活的」是一个**点**的存在与否，不需要一圈晕来强调。呼吸
	// 动画（透明度与缩放）是旧 build 自己的动效，不在墨与面的条文里，保留。
	// The breath is the old build's own animation: the light fades to half and
	// shrinks a little, twice over five seconds, so a running session reads
	// as alive without a spinner. Its keyframes are the reference — opacity 1
	// and scale 1 at 0% and 100%, 0.5 and 0.88 at 50% — so the value is only
	// read through motion: a desktop that asks for less motion gets the dot at
	// rest, exactly the 0% keyframe the old stylesheet left (`animation:
	// none`), and registers no loop.
	breath := motion.Value(c, func() float32 {
		return dot.Loop("terminal.dot", 2400*time.Millisecond, ui.Bounce(ui.EaseInOut))
	})
	dot.Opacity(1 - 0.5*breath)
	dot.Scale(1-0.12*breath, 1-0.12*breath)
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

// panelCorner and mountCorner are the two corner rungs of the terminal page,
// named so a pin can hold the pair apart (docs/raycast-design.md §2 / §6.2 第
// 1-2 条). The panel **is** the window, so it takes `WindowRadius` (12); the
// session's canvas is a block **inside** it, so it takes `RadiusMD` (10) — the
// same rung a grouped card or a floating pane wears. base.css drew both at
// `--window-radius`, which made a block inside the panel read as a second
// window: three faces deep, three windows stacked.
func panelCorner(tokens theme.Tokens) float32 { return tokens.WindowRadius }

func mountCorner(tokens theme.Tokens) float32 { return tokens.RadiusMD }

// panelFill is the colour the terminal surface paints behind its content: a
// named palette's own background, and the window's background for the shipped
// `inherit` (and for an id this build does not know, the same as the renderer's
// own fallback).
//
// It is the one place the palette reaches pixels (P7-dr): the plugin's view is
// transparent, and this layer is read from the stored settings on every frame,
// so the palette and the slider apply to a session that is already running. The
// old build met the same case by rebuilding its renderer on each mount rather
// than by reconfiguring the running one.
//
// The alpha is the terminal slider's *less* the share the card's face already
// carries (glassmap.Spec.CanvasAlpha), so the panel is one sheet: the two
// layers composite back to the slider once. It used to be the slider straight,
// on top of a card face that painted the slider too, so 46% read as 71%.
//
// `running` says whether a session is on screen: the palette's colour belongs
// to a session, and the empty page keeps the window's own colour so its ink
// stays readable on a light palette.
func panelFill(name string, running bool, spec glassmap.Spec, tokens theme.Tokens) ui.Color {
	colour := ui.RGB(18, 19, 22)
	if tokens.Theme != nil {
		colour = tokens.Theme.Background
	}
	if p, ok := palettes[name]; ok && running {
		colour = packed(p.bg)
	}
	return colour.Alpha(spec.CanvasAlpha())
}

// themeFor builds the plugin theme (or the light/dark pair) for a stored
// palette id: the shipped `inherit` follows the window, each override paints
// the fixed pair from TERMINAL_PALETTES.
//
// The palette's colours are opaque. How solid the background is, is a
// *different* value, and it belongs to the view (canvasFill): the plugin
// resolves the vt's default background through `rgb`, which has no alpha, so
// what the terminal's background is and how much of the desktop shows through
// it cannot be one colour.
//
// `inherit` used to pass no theme at all, which left *every* colour to the
// plugin's own defaults - including the cursor, which the plugin falls back to
// the foreground when a theme leaves it zero. The reference read
// `--terminal-cursor`, a teal token distinct from `--terminal-fg`, so an
// inherit session's cursor was drawn in the wrong colour. The plugin has no
// per-token "follow the window" switch; the equivalent is the light/dark pair
// it documents for a Ghostty `theme = light:…,dark:…`: hand it the two base
// themes with only the cursor token overridden, and it keeps following the
// window's appearance while painting that one colour.
//
// Each override is a single fixed palette (the plugin base is chosen by the
// palette's own light/dark, not the window's), so its pair is (theme, nil):
// the theme applies in both appearances, as before.
func themeFor(name string) (theme, dark *terminal.Theme) {
	spec, ok := palettes[name]
	if !ok {
		lightBase := *terminal.LightTheme()
		lightBase.Cursor = inheritCursor(false)
		darkBase := *terminal.DarkTheme()
		darkBase.Cursor = inheritCursor(true)
		return &lightBase, &darkBase
	}
	base := terminal.DarkTheme()
	if !spec.dark {
		base = terminal.LightTheme()
	}
	t := *base
	t.Background = packed(spec.bg)
	t.Foreground = packed(spec.fg)
	t.Cursor = packed(spec.cursor)
	t.Selection = spec.selection
	return &t, nil
}

// inheritCursor is the colour base.css's `--terminal-cursor` token gives the
// shipped `inherit` palette: a teal on the dark face (#8bd5ca) and a deeper one
// on the light face (#007c86). It is deliberately not the foreground - the
// plugin's zero-cursor fallback - which is what the token exists to override.
func inheritCursor(dark bool) ui.Color {
	if dark {
		return ui.RGB(0x8b, 0xd5, 0xca)
	}
	return ui.RGB(0x00, 0x7c, 0x86)
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
	// Follow app: the terminal's own default theme, with the shipped
	// `--terminal-cursor` token - the plugin's zero-cursor fallback (the
	// foreground) is not the token's colour. The selection falls back to the
	// foreground when a theme leaves it zero (the plugin's own rule).
	base := terminal.DarkTheme()
	if !dark {
		base = terminal.LightTheme()
	}
	cursor := inheritCursor(dark)
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
