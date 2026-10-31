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
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"floter/internal/deadline"
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

	// tokenMemo is this frame's palette, and tokenStore is the store it was
	// read from (P11-bt). The surface asks for tokens from many places per
	// frame — `tokensOf` alone has thirteen call sites, and two more draw
	// straight from `theme.ForPreferences` — and a build of a `ui.Theme` plus
	// its palette per call was measurable work on a surface that redraws on
	// every byte the program writes. The memo is keyed by the store's version,
	// the appearance and the desktop's preferences, so a hit is the value the
	// miss would have built; see `theme.Memo`.
	tokenMemo  theme.Memo
	tokenStore *settings.Store

	// settingsMemo is this surface's copy of the settings value (P11-co).
	// The frame reads it from the title bar, the empty page, the bar's inks and
	// the drawer's grid — five snapshots per frame before this memo existed —
	// and `Store.Snapshot` deep-copies the whole value, the carried-through keys
	// included. Readers that run off the render thread (the paste callback, on
	// the terminal's own goroutine) keep reading the store directly: a Memo is
	// one reader's and is not safe for concurrent use.
	settingsMemo settings.Memo

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
	// DrawerScroll is how far the drawer's strip is scrolled when the panel
	// is shorter than the strip's natural height (the terminal's minimum
	// window is 360, the grid asks for 365 at the narrow widths). The strip
	// itself is the scroll container: it scrolls only when it cannot fit, so
	// a full-height window reports no range and the offset stays 0.
	DrawerScroll ui.ScrollState

	// debouncing marks a slider preview that has been shown but not written
	// yet, with debounceAt the moment the window closes. The drawer's font size
	// and line height ride the reference's 180ms window (see debounce.go): a
	// drag emits a tick per frame, and the store writes the file on every
	// accepted Update, so an un-debounced drag wrote it once per frame.
	debouncing bool
	debounceAt time.Time
	// frameAt is the clock of the frame being built, so a preview schedules
	// its window against the frame's own time rather than a wall clock the
	// tester cannot move (see debounce.go).
	frameAt time.Time

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

	// dotBreath is when the bar's status dot was first drawn: the start of its
	// breath. It lives on the app rather than in the element's state because
	// the breath is now read while the frame paints (Painter.Draw), not while
	// the view is built, so it has to survive builds the way the drawing does.
	// Zero until a frame draws the dot; see statusDot.
	dotBreath time.Time
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
	copy := i18n.For(a.settings().Language).Terminal
	if index < 0 || index >= len(a.Sessions) {
		return copy.Title
	}
	if title := a.Sessions[index].Title; title != "" {
		return title
	}
	return copy.Title
}

// NoteOf is a session's own state line, by index: the program is running, or
// how it went — the wording the sessions page prints under the session's name.
//
// The ended line is the old page's own (`main:src/settings/SessionsPage.tsx:173-177`):
// `terminal.sessionExited` alone for a clean exit, and only a non-zero code
// gets the ` · exit N` suffix. The resident note on the terminal surface keeps
// the separate, always-coded `terminal.processExited` wording.
func (a *App) NoteOf(index int) string {
	copy := i18n.For(a.settings().Language).Terminal
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

// A session's screen is **not** saved and replayed into the next one. It used
// to be, and that was the report 「打开终端时出现异常字符 / 莫名被输入的内容」.
//
// `Terminal.Snapshot()` is the plugin's VT export: the scrollback and screen
// with their styles, and then the modes, the scrolling region, the tab stops,
// the working directory and the keyboard's flags
// (`mygo@v0.7.0/plugins/terminal/internal/vt/terminal.go:565-590`). Its own
// documentation names its reader: "as a terminal attaching again to a session
// a server keeps" — a terminal with **no program**. This surface fed it into a
// terminal that had just started a shell, so the new program inherited a dead
// program's state. Two of those are content, not chrome, and both were
// reproduced headlessly against libghostty-vt:
//
//   - a replayed `\e(0` (G0 = DEC graphics, what `vim` and `ls --color` leave
//     behind) renders the new shell's plain ASCII as box drawing: `qqq-wxyz`
//     came out `───-┬│≤≥`.
//   - a replayed `\e[?2004h` (bracketed paste) wraps a paste in `\e[200~`
//     `\e[201~` although the new shell never asked, so the shell echoes
//     `^[[200~hello^[[201~` into the line.
//
// There is no sound way to keep the feature with the public API: `Snapshot()`
// is all-or-nothing and the formatter flags that would drop the program state
// live in the plugin's internal package. Filtering the sequences back out is
// the very thing this project forbids — it misses the scrolling region, the
// working directory and the keyboard flags, and misses them silently. The
// reference has no such feature either (`git grep -n snapshot main --
// src/terminal src-tauri/src/terminal` finds only scrollback offsets), so the
// surface keeps what the reference keeps: a fresh session, nothing replayed.
// Bringing the last session's scrollback back needs a **text-only** export
// from the plugin (a mygo feature); it is a decision for the user, not a
// workaround to invent here.

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
	// The frame's clock, for the debounce window a preview schedules (see
	// debounce.go). Reading it here rather than at each preview keeps one answer
	// per frame, and it is the frame's own time — the tester moves it with
	// Advance, so a pin does not have to wait 180ms.
	a.frameAt = c.Now()
	// A window that ran out writes the value now; while it is still running this
	// arms the one frame that will (P11-bq: a line that changes once asks for one
	// wake, not one build per refresh).
	a.expireSettingsWrites(c)
	stored := a.settings()
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
					a.emptyLine(c, copy, newCommandBadge(a.settings()), copy.EmptyBack, a.Actions.NewCommand)
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
		// P11-z 重定基线（用户实测 F3）：画布这里曾有一条 `Border(1, edge)`，
		// 聚焦时还是 accent 红。它在面板（12）的框里再套一圈框（10），两层圆角
		// 框叠在一起就是用户读到的「终端页面外圈那圈类玻璃边」。面板是这一页
		// **唯一**的框，画布只留一层填充；不加 rim/sheen/高光，也不做
		// floating/overlay 观感（元素 inline、贴进布局）。
		// P11-at：画布至少留出浮条那一条带（`titleBar` 的 `Space(7)`）。抽屉
		// 按网格的自然高度要位置，窗口缩到最小（`MinTerminalHeight` = 360）
		// 时那个高度比面板还高，于是画布被压到 0、抽屉从 y=0 开始画——正好
		// 压在浮着的标题条下面：标题 `Terminal` 与 `Font family` 落在同一块
		// 像素（实测 720×360 重叠 81×8u），抽屉自己的关闭钮与浮条的 X 叠成
		// 一个（28×18u）。带宽是画布的地板，不是抽屉的预算：抽屉放不下就在
		// 自己那条带里滚动（`drawer` 的 `DrawerScroll`）。
		body := ui.Box(c).Grow(1).MinHeight(barHeight(t)).FillWidth()
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
			ui.Box(c).Fill().Radius(mountCorner(tokensOf(a, c))).
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
		// P13-d（`docs/liquid-glass.md` §6.3 第 3-8 行）：它原先只戴了受光边的**一半**
		// （只有顶边那条 `LensRim`），没有对侧——于是在 1x 屏上与行底分不开（浅色档
		// 更明显：白压白，§3 末尾）。与 P12-b 给启动器键帽的是同一个
		// `theme.Tokens.LitFace`：顶亮一分、底暗一分，**底侧那条**才是它站起来的原因。
		// 外圈描边一个字节没动——受光边是**内线**，不是画框。
		badge := ui.Row(c).Height(t.Space(5)).Padding(0, t.Space(1.5)).Radius(tokens.RadiusXS).
			Border(1, tokens.ControlEdge).Background(tokens.Control).
			AlignItems(ui.Center).Justify(ui.Center)
		badge = tokens.LitFace(badge)
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
//
// It answers the frame's one palette rather than building a fresh one per call
// (P11-bt): every reader on this surface passes the same appearance and the same
// desktop preferences, and nothing else reaches `ForPreferences`, so the memo's
// key is exact — a change to the settings bumps the store's version and the next
// read builds again, which is what a preview slider on the drawer needs.
func tokensOf(a *App, c *ui.Context) theme.Tokens {
	if a.tokenStore != a.Store {
		// Two stores both begin at version zero, so a swap is noticed by the
		// pointer, exactly as the settings surface's memo does it (P11-bs).
		a.tokenStore = a.Store
		a.tokenMemo.Reset()
	}
	// The loader is the surface's own settings memo, not `Store.Snapshot`: a
	// palette build reads the whole value, and routing it through the memo is
	// what keeps a frame to one copy even when it also builds a palette.
	return a.tokenMemo.Tokens(a.Store.Version(), c.Theme().Dark, c.Preferences(), a.settings)
}

// settings is this frame's settings value, copied once per change rather than
// once per reader (see `App.settingsMemo` and `settings.Memo`).
func (a *App) settings() settings.Settings {
	return a.settingsMemo.Get(a.Store)
}

// barInks is the ink the floating terminal bar draws with, and the face it
// draws it on. The bar floats over the *canvas*, not over the window: the panel
// under it is the palette's own background, so a bar that reads the window's
// rungs is a window-grey mark on a near-black face whenever the two disagree
// (light window, `ink`/`dusk`/`forest` palette: under 2:1 for the identity
// line).
//
// P11-ae 重定基线（用户实测 F2「可读性与对比度达标」的延伸）：bar 的墨与面
// 只回答「它浮在哪块面上」这一个问题——`inherit`、未知 id 与空页（没有会话，
// 也就没有它的调色板）拿窗口自己的四档、不画面，逐像素与参考的浮条一致；
// 命名调色板则让 bar 穿画布自己的颜色（面 = 调色板底色，墨 = 调色板前景），
// 于是它属于它浮着的那块面，就像终端自己的标题行。
type barInks struct {
	// Ink is the identity line (the reference's body step in the secondary
	// register), Muted the bar's glyphs, Tertiary the status dot, and Wash the
	// pane behind a button the pointer rests on.
	Ink, Muted, Tertiary, Wash ui.Color
	// Press is the same pane one rung *down*: `docs/liquid-glass.md` §5 asks a
	// pressed control to sink back toward the face it sits on, and the bar's
	// controls sit on the canvas, so this rung is thinner than Wash, never a
	// different material (P13-am).
	Press ui.Color
	// Face is the bar's own fill while it is revealed; a zero alpha means the
	// bar owns no face and the canvas under it is what the user sees, which is
	// the window's own case.
	Face ui.Color
}

// barFaceContrast is the floor the bar's ink has to clear against its own
// face: WCAG 2.1 AA for body text, the same number `internal/theme` grades its
// quiet steps with.
const barFaceContrast = 4.5

// windowBarInks is the bar's four rungs as the window spells them: the shape
// the bar has always had, and the shape it keeps wherever no palette owns the
// face under it.
// The bar's two lifting rungs over a named palette: the wash the pointer rests on
// and the thinner one it leaves under the finger (P13-am). One number each, so the
// ladder is one statement and the press is always a fixed step back from hover.
const (
	barWashAlpha  = 0.1
	barPressAlpha = 0.035
)

func windowBarInks(f *ui.Theme, tokens theme.Tokens) barInks {
	return barInks{
		Ink:      tokens.TextSecondary,
		Muted:    f.TextMuted,
		Tertiary: tokens.TextTertiary,
		// Over the window's own canvas the bar's two faces *are* the control
		// ladder's two lifting rungs (`--glass-control-hover` /
		// `--glass-control-press`): nothing has to be derived, and borrowing the
		// pair keeps the terminal's keys on the same ladder as every other
		// pressable in the app.
		Wash:  f.SurfaceHover,
		Press: f.SurfacePressed,
	}
}

// canvasInks resolves this frame's bar inks from the face the bar floats on.
func (a *App) canvasInks(c *ui.Context) barInks {
	tokens := tokensOf(a, c)
	session := a.Current()
	return barInksFor(tokens, windowBarInks(c.Theme(), tokens), a.settings().TerminalTheme,
		session != nil && session.Term != nil)
}

// barInksFor is the one answer to "what does the bar draw with": `running`
// says whether a session is on screen (a palette with no session under it is
// not a face the bar can belong to), and `name` is the stored palette id.
func barInksFor(tokens theme.Tokens, window barInks, name string, running bool) barInks {
	spec, ok := palettes[name]
	if !ok || !running {
		return window
	}
	// The face beneath the canvas: the window's own colour, the same proxy
	// `panelFill` reads for `inherit` (the desktop behind a translucent window
	// is not knowable here, and the panel is what the canvas is composited
	// against inside the app).
	under := ui.RGB(18, 19, 22)
	if tokens.Theme != nil {
		under = tokens.Theme.Background
	}
	face := packed(spec.bg)
	ink := packed(spec.fg)
	scrim := face.Alpha(barFaceAlpha(ink, face, under))
	onBar := scrim.Over(under)
	// The quieter rungs are the same ink as quiet as it can be: each is the
	// least opacity that still clears its floor, so the bar's whole ladder
	// scales with the palette instead of mixing two materials.
	return barInks{
		Ink:      ink,
		Muted:    quietRung(ink, onBar, 3.0),
		Tertiary: quietRung(ink, onBar, 2.5),
		Wash:     ink.Alpha(barWashAlpha),
		// The press rung is the same wash stepped back toward the canvas. It
		// cannot be the window's `ControlPress`: a window-grey plate over a
		// `contrast` canvas reads *lighter* than the black it sits on, i.e. the
		// key would lift when pushed. Over a named palette the whole ladder is
		// the palette's own ink, so the press is `barPressAlpha` of that ink.
		// The two numbers are the control ladder's own ratio, not a new one:
		// `controlPress / controlHover` is 0.032/0.105 = 0.305 on the light
		// appearance and 0.05/0.14 = 0.357 on the dark, so a bar wash of 0.10
		// steps back to 0.035 — the top of that band, because the bar's wash is
		// already thinner than the settings ladder's and 0.035 of a palette ink
		// is the least that still paints ~8 levels on the darkest canvas
		// (`contrast`, `forest`) rather than vanishing.
		Press: ink.Alpha(barPressAlpha),
		Face:  scrim,
	}
}

// quietRung is the least opacity of `ink` over `face` that still clears
// `floor`: the quietest a secondary rung can be and still be read.
func quietRung(ink, face ui.Color, floor float64) ui.Color {
	const steps = 32
	for i := 1; i <= steps; i++ {
		rung := ink.Alpha(float32(i) / steps)
		if inkContrast(rung.Over(face), face) >= floor {
			return rung
		}
	}
	return ink
}

// barFaceAlpha is the least alpha at which the bar's own face makes its ink
// readable on it: the bar is as light as it can be while its text still clears
// `barFaceContrast` over the face beneath. A palette's foreground and
// background are designed to be read together, so an opaque face always
// clears; the search only keeps the bar from being denser than it has to be.
func barFaceAlpha(ink, face, under ui.Color) float32 {
	const steps = 32
	for i := 0; i <= steps; i++ {
		alpha := float32(i) / steps
		if inkContrast(ink, face.Alpha(alpha).Over(under)) >= barFaceContrast {
			return alpha
		}
	}
	return 1
}

// inkContrast is WCAG 2.1's contrast ratio between an opaque ink and an opaque
// face.
func inkContrast(ink, face ui.Color) float64 {
	lighter, darker := relativeLuminance(ink), relativeLuminance(face)
	if lighter < darker {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

// relativeLuminance is WCAG 2.1's relative luminance of one sRGB colour.
func relativeLuminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
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
	tokens := tokensOf(a, c)
	color := tokens.TextSecondary
	if session.ExitCode != 0 {
		color = t.Warning
	}
	// The reference's own note is a floating card 12u from the panel's edges, a
	// 7u/10u padding pair, the floater's own fill and shadow (`--glass-float` /
	// `--elev-3`), with the title at the body step in 560 — the warning colour
	// when the program returned non-zero.
	//
	// P11-bc 重定基线（用户实测 F3「不要 floating/overlay 观感，元素要 inline、
	// 要贴进布局」）：它以前就是那张浮卡——四边各让 12u、`RadiusMD` 圆角、
	// `Float` 面、`ElevThrow` 投影、`LensRim` 受光边，再加一段从上方 2u 落下的
	// 160ms 入场。用户要的「轻盈」在纯白画布上读成一块压在输出上的玻璃片，
	// 而这条便条是**常驻家具**（会话退出后一直在，直到用户自己关掉），不是
	// 一条转瞬即逝的消息。现在它是画布底边的一条**贴边带**：整宽、贴底、无
	// 圆角、无投影、无高光、无入场动画，面与线用抽屉自己那套说法
	// （`Control` 面 + 上边一条 `HairlineSoft` 弱发丝，见 `drawer`）。锚点、
	// 文字、三档墨、字号、出现条件、它在 `body` 里的父子关系、画布高度与 PTY
	// 行数一个没动——`Absolute` 的便条不占布局高度，所以终端不被 resize。
	//
	// 同时补上参考的 `pointer-events: none`（`main:src/styles/terminal.css:303`）：
	// 这条带现在横跨画布整宽，不做 `PassThrough` 就会把最后一行输出的点击、
	// drag-select 与滚轮全吃掉（与 P11-af 浮条吃掉前两行是同一个后果）。
	note := ui.Row(c).Absolute().Bottom(0).Left(0).Right(0).PassThrough().
		Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1.75), t.Space(2.5)).
		Background(tokens.Control).
		BorderWidth(1, 0, 0, 0).BorderColor(tokens.HairlineSoft)
	note.Children(func() {
		// P11-bd：`PassThrough` **不沿子树传播**——框架的命中表逐个元素记，而一句
		// `Text` 自己就是可选中面 = 命中目标。只标外层时这条带的**字**仍然吃掉
		// 指针（实测点 (322,582) 拿不到画布的焦点，点画布 (400,300) 能拿到），
		// 参考的 `pointer-events: none` 在 CSS 里是继承的。所以两层都标，与
		// P11-af 给 `titleBar`/identity/`statusDot` 各标一次同一个手法。
		ui.Text(c, copy.ProcessExited(session.ExitCode)).FontSize(t.FontSize).
			FontWeight(560).TextColor(color).PassThrough()
		ui.Text(c, copy.ProcessExitedHint).FontSize(t.FontSize).TextColor(t.TextMuted).PassThrough()
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
	// The note is a timed line: it changes once, when its 4.5s window runs out,
	// so the panel arms one wake at that moment instead of rebuilding the view
	// on every refresh for a sentence that does not change (P11-bq — the frames
	// `Context.AnimationFrame` asked for were builds, `ui/runtime.go:744-748`).
	deadline.Wake(c, a.feedbackUntil.Sub(now))
	t := c.Theme()
	tokens := tokensOf(a, c)
	// The reference's own notice: 12u from the panel's top-left corner, the
	// 7u/10u padding pair, the floater's fill, a 15u alert glyph and the body
	// step at 560 in the warning colour. The glyph, the ink, the two padding
	// steps and the 4.5s clock are untouched below.
	//
	// P11-bg 重定基线（用户 F3「不要 floating/overlay 观感，元素要 inline、要
	// 贴进布局」）：这条提示以前是那张浮卡——12u 见方的圆角面、`ControlEdge`
	// 描边、`Float` 填充、`ElevThrow` 投影、`LensRim` 受光边，再加一段从上方
	// 2u 落下的 160ms 入场。P11-bc 把「进程已退出」那条常驻便条改成了画布底边
	// 的带，这一条当时故意留着；现在它是**同一页的第二种说法里的最后一处浮面**。
	//
	// 现在它挂在标题条下沿、整宽贴边（`barHeight`），面与线用带子自己那套说法
	// （`Control` 面 + **底边**一条 `HairlineSoft` 弱发丝，底边那条带是上边一条：
	// 两条带各自朝画布内侧收口），无圆角、无投影、无受光边、无入场动画。标题条
	// 本就浮在画布顶上（`titleBar` 的 `Top(0)`，画布不预留高度），所以这条带读作
	// chrome 的第二行而不是压在输出上的一块玻璃片。`Absolute` 不占布局高度，
	// **画布高度与 PTY 行数一个字节没动**。
	//
	// P8-c 重定基线（§6.2 第 4 条 / §5）：环 `AccentEdge` → `ControlEdge`。accent
	// 只回答「哪个被选中/哪个是开的」，而这条是「某个动作没做成」——它的语义
	// 已经由那行 warning 墨和 15u 的警示字形说了，一圈 accent 环不在 §5 的
	// 任何合法处。几何（12u 起点、28u/40u 的 padding、`RadiusMD`）零改动。
	// P11-bd：参考给这条提示 `pointer-events: none`（`main:src/styles/terminal.css:414`，
	// 与常驻便条 `:303` 是同一句），本仓从来没有 `PassThrough()`。它浮在画布的
	// 左上角、盖住前两行输出的左半边 4.5 秒——那段时间里点进去、拖选区、滚轮
	// 全落在这块矩形上，画布拿不到键盘（实测：点 (400,400) 画布拿到焦点，点
	// 提示自己的字 (89,31) 与它的内边距 (20,30) 都拿不到）。与 P11-af 的浮条
	// 吃掉前两行、P11-bc 的常驻便条吃掉最后两行是同一条判据：**一条没有动作
	// 的浮层不该是命中目标**。几何、面、墨、字号、4.5s 的时钟一个字节没动。
	notice := ui.Row(c).Absolute().Top(barHeight(t)).Left(0).Right(0).PassThrough().
		Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1.75), t.Space(2.5)).
		Background(tokens.Control).
		BorderWidth(0, 0, 1, 0).BorderColor(tokens.HairlineSoft)
	notice.Children(func() {
		// `PassThrough` 不沿子树传播（框架的命中表逐个元素记），而一句 `Text`
		// 自己就是可选中面 = 命中目标：只把外层标成透明时，点提示的**字**仍然
		// 被它吃掉（实测 (89,31) 拿到画布的焦点前被拦下）。参考的
		// `pointer-events: none` 在 CSS 里是继承的，所以这里把两层都标上，与
		// P11-af 给 `titleBar`/identity/`statusDot` 各标一次是同一个手法。
		if glyph, ok := launcher.Glyph("alert"); ok {
			ui.Icon(c, glyph).Size(t.Space(3.75), t.Space(3.75)).TextColor(t.Warning).PassThrough()
		}
		ui.Text(c, a.feedback).FontSize(t.FontSize).FontWeight(560).TextColor(t.Warning).PassThrough()
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
//
// P11-bd：`live` 是「这条浮条现在是揭示着的」。未揭示时浮条的内容是 `Opacity(0)`
// 的——看得见的东西没有，可是**命中表不看透明度**：框架只把 `flagInvisible` /
// `flagInert` 的元素从命中里拿掉（`ui/runtime.go:1196`），于是这些隐形按钮照样
// 接点击（实测：静态下点右上角 `(834,14)` 把终端**关了**），也照样在指针停 0.6s
// 后弹出自己的提示（实测静态下悬停齿轮弹出 “Appearance settings for this
// terminal”，而屏上一个字都没画）。参考把这一半写成
// `.terminal-bar:not(:hover):not(:focus-within) .terminal-panel__actions {
// pointer-events: none }`（`main:src/styles/terminal.css:501-505`，注释原话是
// “While hidden the buttons must not catch clicks meant for the terminal
// underneath”）。
//
// 关键：`PassThrough` **不沿子树传播**（与 CSS 的 `pointer-events: none` 不同），
// 所以按钮、它的字形、它的提示要各标一次——只标 `actions` 那一行是本轮修掉的
// 那个 bug 的形状。
func (a *App) barButton(c *ui.Context, name, label, hint string, live bool, run func()) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	button := ui.Row(c).Focusable().Size(t.Space(7), t.Space(7)).Shrink(0).
		Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).Label(label)
	if live {
		// P13-ax：提示不是框架那枚白片子，是 E2 浮（`theme.Tokens.Tip`）。
		tokens.Tip(c, button, hint)
	} else {
		// Hidden: no click, no tip. Keyboard focus still reaches it
		// (`Focusable` is untouched), which is the reference's
		// `:focus-within` reveal — a Tab onto the bar brings it back.
		button.PassThrough()
	}
	// The bar's inks come from the canvas it floats on (see `canvasInks`): over
	// a named palette the glyph is the palette's own ink and the hover pane is
	// a tint of it, so a dark palette in a light window no longer paints a
	// window-grey mark on a near-black face.
	inks := a.canvasInks(c)
	switch {
	case button.Pressed():
		// P13-am · the third rung. This bar had rest and hover only, so the frame
		// the pointer was held down on was pixel-identical to the hover frame
		// (measured: 221/221 light, 48/48 dark, and on a named palette
		// 33,36,38 in both states) — a key that does not move when pressed reads
		// as a key that did not take the click. `docs/liquid-glass.md` §5 asks for
		// the ladder to step *back down*; the rung comes from the palette the bar
		// belongs to (`barInks.Press`), because the window's `ControlPress` would
		// paint a window-grey plate on a palette canvas. `LitFace` is drawn again
		// inside the arm: the crown and counter-side line are per state, and a
		// pressed key keeps its material rather than losing it exactly when it is
		// most obviously a solid object.
		//
		// 为什么这一页不跟族里那一步 `Scale(0.92)`（P13-q / P13-al）：实测它会把
		// 「按下去再松开」丢掉一整个点击。浮条上的键在抽屉滑入的那两帧里会被
		// 裁到 **2 DIP** 宽（`drawer_test.go` 的两条 pin 点的就是那一帧，实测
		// 盒子 `{858 314 2 28}`）；`Scale` 绕盒子中心缩，2 DIP 的键左边缘因此
		// 往里让 0.08 DIP，正好把落在中心线上的那个点挤出命中框，释放时
		// `contains` 为假 —— 抽屉关不掉，两条 B pin 变红。0.08 DIP 的命中差是
		// **框架**的账（盒宽收缩时命中框跟着动），不是一次视觉轮该在这里改的
		// 语义：本页只收面，几何留字给框架。已记进 `liquid-glass.md` §6.7b
		// 第 7-10 行「需拍板」。
		button = tokensOf(a, c).LitFace(button.Background(inks.Press))
	case button.Hovered():
		button.Background(inks.Wash)
		// The pointer lights the resting lens rung (`.toolbar-button:hover`,
		// the reference's hover-only variant): the bar's icon button is
		// transparent at rest, so this is the only state it can be graded in.
		// P13-d（`docs/liquid-glass.md` §6.3 第 3-7 行）：`Lens` 那四层里的第四层是
		// 四边 `ControlEdge` 的**环** —— 正是这一支要退掉的「画框分层」，而它顶上那条
		// `LensRim` 也早不是 0.04 了（P12-a 重定到 0.09；上一版注释写的「≤0.04」是旧
		// 上限，已经不成立）。悬停的 28u 图标按钮是**长在浮条上的一个小面** = E1，
		// 戴 `LitFace`：顶受光边 + 底对侧暗线，**没有环**、也没有那片宽 sheen。
		tokensOf(a, c).LitFace(button)
	}
	button.Children(func() {
		if glyph, ok := launcher.Glyph(name); ok {
			icon := ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(inks.Muted)
			if !live {
				icon.PassThrough()
			}
		}
	})
	if button.Clicked() {
		run()
	}
}

// barHeight is the title bar's own band: the bar draws into it, the canvas is
// floored at it, and the panel's notices hang from its lower edge. Three
// readers, one number — a second spelling of it is how the failure note ends up
// half a step off the bar it is docked to (P11-bg).
func barHeight(t *ui.Theme) float32 { return t.Space(7) }

func (a *App) titleBar(c *ui.Context, copy i18n.Terminal) {
	t := c.Theme()
	newCommandHint, openExternalHint := barHints(copy, a.settings())
	// P11-ae：这一条（以及它里面的点与按钮）的墨来自它浮着的那块面，不是窗口。
	inks := a.canvasInks(c)
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(barHeight(t)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).PassThrough()
	// The bar's whole content is hidden until the pointer is over it or
	// something on it has the keyboard, so neither the title nor the buttons
	// permanently occlude the first lines of output under them
	// (`.terminal-bar__identity` / `.terminal-panel__actions` reveal the same
	// way).
	//
	// P11-af：揭示读**指针位置**，不读命中。这一条是 `PassThrough`：整条带不是
	// 命中目标，指针落进它下面的画布——否则浮在画布上的 28u 会把第一/第二行
	// 输出的点击与 drag-select 全吃掉（终端的 `CopyOnSelect` 在这两行上失效），
	// 这是 F3「不要 overlay」在输入侧的后果。`PointerPosition` 在
	// `PassThrough` 元素上仍算包含关系（ui/input.go:1265-1274），所以悬停仍能
	// 把标题与按钮揭示出来。窗口拖拽手柄收窄成会话标题本身（见下面的标题），不再是一整条隐形带。
	_, _, hovered := bar.PointerPosition()
	revealed := hovered || bar.FocusWithin()
	// A palette that owns the canvas owns the bar too: while the bar is
	// revealed (the only state its content is drawn in) it carries the palette's
	// own face, so its ink sits on the material it was measured against. At rest
	// it paints nothing, so the first lines of output stay uncovered; the top
	// corners take the panel's radius because the bar's own rectangle is the
	// panel's top edge.
	if revealed && inks.Face.A != 0 {
		bar.Radius(panelCorner(tokensOf(a, c)), panelCorner(tokensOf(a, c)), 0, 0).
			Background(inks.Face)
	}
	// `.terminal-bar:hover { box-shadow: var(--elev-bar) }` (`terminal.css:99-104`):
	// reaching for the bar re-casts a light shadow — `0 6px 16px` at
	// `0.18 * --elev-shadow-scale` — onto the content scrolling under it, the
	// reference's one cue that the bar itself is a draggable handle. Only the
	// pointer raises it; keyboard focus reveals the identity without lifting the
	// surface, exactly as `:hover` does.
	if hovered {
		// P8-c 重定基线（§6.2 第 6 条）：投影读 token，不再写字面
		// `rgba(0, 0, 0, 0.18)`。同一条 throw 在两套外观下是两档（深 0.40 /
		// 浅 0.10），字面值把它钉成了第三档，浅色下比合约重、深色下比合约轻。
		//
		// P13-d（`docs/liquid-glass.md` §6.3 第 3-4 行）：上一版把**几何与墨配错了**
		// ——宽影（`0 6px 16px`，参考的 `--elev-bar`）配的是 §3 的**紧贴边**墨
		// `ShadowContact`，于是这条带压出的是一团宽而重的阴影，读起来是「底下加深
		// 一块」而不是「浮起来一张纸」（§2 判据 3：**E2 必须两层影**——只有环境影则
		// 浮层「贴」在底上，只有接触影则它「悬」在空里）。现在宽影读
		// `ShadowAmbient`，再补一条 `0 1px 2px` 的 `ShadowContact` 做接触影。
		// 旧值 → 新值：`0 6px 16px ShadowContact` → `0 6px 16px ShadowAmbient` +
		// `0 1px 2px ShadowContact`。几何、触发条件与字一个字节没动。
		tokens := tokensOf(a, c)
		bar.Shadow(0, t.Space(1.5), t.Space(4), 0, tokens.ShadowAmbient)
		bar.Shadow(0, t.Space(0.25), t.Space(0.5), 0, tokens.ShadowContact)
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
			Label(identityTitle).PassThrough()
		if !revealed {
			identity.Opacity(0)
		}
		identity.Children(func() {
			a.statusDot(c, inks, revealed)
			// `.terminal-bar__title` (`terminal.css:150-161`): the body step in
			// `--text-secondary`, 580, with the reference's `letter-spacing:
			// -0.01em`. The identity line used to read the muted step one size
			// down, which made the name of the session on screen the quietest
			// thing in the bar it names.
			// P11-ae：墨来自它浮着的那块面（`canvasInks`）——选了命名调色板时
			// 就是调色板自己的前景，`inherit` 与空页仍是窗口的 `TextSecondary`。
			//
			// P11-af：**会话标题就是这一页的 title bar**。浮条从整条 `DragWindow`
			// 收窄到这里，于是拖窗口仍然有手柄（字宽的那一块），而标题右边到
			// 按钮之间的那条带交给画布：那一段的点击与选区都不再被浮条吃掉。
			// 提示（截断时的全名）也跟到字上：整行是 `PassThrough`，挂在行上的
			// tooltip 永远不会被悬停到。
			// P13-ax：全名那条提示同样改戴 E2 浮（`theme.Tokens.Tip`）：它一
			// 直是全仓唯一一枚白片子，而这页上它是读全名的**唯一**位置。
			tokensOf(a, c).Tip(c, ui.Text(c, identityTitle).FontSize(t.FontSize).FontWeight(580).
				LetterSpacing(-0.01*t.FontSize).
				TextColor(inks.Ink).Ellipsis("\u2026").SingleLine().DragWindow(), identityTitle)
		})
		// The controls, at the bar's trailing edge: 28u transparent squares
		// with a 4u gap, hidden and non-interactive until the bar is revealed
		// (a hidden button must not catch a click meant for the terminal
		// underneath).
		//
		// P11-bd：这一行的 `PassThrough` 只负责**行自己的盒子**（4u 的缝、行尾的
		// 余量）；按钮与它们的字形各自标一次（见 `barButton` 的 `live`）——
		// `PassThrough` 不沿子树传播，只标这一行正是本轮修掉的 bug 的形状。
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
				a.barButton(c, name, label, hint, revealed, a.Actions.Settings)
			}
			if a.Actions.Pin != nil && a.term() != nil {
				a.barButton(c, "text", copy.Pin, copy.Pin, revealed, func() { a.Actions.Pin(a.sessionTitle(), a.sessionText()) })
			}
			if a.Actions.NewCommand != nil {
				a.barButton(c, "command", copy.NewCommand, newCommandHint, revealed, a.Actions.NewCommand)
			}
			if a.Actions.OpenExternal != nil {
				a.barButton(c, "terminal", copy.OpenExternal, openExternalHint, revealed, a.Actions.OpenExternal)
			}
			if a.Actions.Close != nil {
				a.barButton(c, "close", copy.Close, copy.Close, revealed, a.Actions.Close)
			}
		})
	})
}

// statusDot is the bar's status light. It answers one question — is a program
// running — and answers it with **form**: a solid dot that breathes while a
// session runs, and a hollow ring when nothing does (a program that ended, and
// the blank page that never had one). Both wear the bar's own tertiary rung
// (`canvasInks`), so the light belongs to the canvas the bar floats on rather
// than to the window behind it.
func (a *App) statusDot(c *ui.Context, inks barInks, drawn bool) {
	t := c.Theme()
	// P11-af: the dot is 7u of the band's first 7u — a plain Box would still be a
	// hit target there, so the status light follows the bar and the identity row
	// into `PassThrough` (see titleBar): only the title's own words take the
	// pointer.
	dot := ui.Box(c).Size(t.Space(1.75), t.Space(1.75)).Radius(t.Space(1.75)).Shrink(0).PassThrough()
	if !a.live() {
		// P13-ao：**暗**那一档从「更淡的实心盘」改成**空心环**。旧实现是
		// `Background(Tertiary).Opacity(0.7)`，靠 0.30 的 alpha 差与「活着」
		// 分开；而活着那一档的呼吸把 alpha 从 1.00 走到 0.50（下面 `1-0.5*breath`），
		// 每个 2.4s 周期里约 40% 的时间比这一档还暗——两个状态的范围重叠，
		// 静帧里分不出谁是谁。形不重叠：实心 vs 空心。（续见 `live`。）
		//
		// 环是 `Painter.Stroke` 画在盒子**内缘**的 1 DIP（`t.Space(0.25)`），
		// 所以灯还是 1.75u、还是 `Shrink(0)`，布局一个像素没动；墨用满
		// `Tertiary` 而不是 0.7，因为 1 DIP 的环只有圆盘一半面积，再乘 0.7
		// 就淡成灰影（实测质量比见 `dotform_test.go`）。
		dot.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Stroke(r, inks.Tertiary, r.W/2, t.Space(0.375))
		})
		return
	}
	// P8-e 重定基线（§5 判据）：旧值是 `t.Accent`，但「这个进程是活的」不是
	// 「哪个被选中 / 哪个是开的」，accent 无权回答它——所以色相转中性，
	// `tokens.TextTertiary`，与 exited 态同一档（两者只差明度与动效）。存活由
	// 「动」说：呼吸仍在，色相不再参与。旧值 → 新值：`t.Accent` →
	// `tokens.TextTertiary`（深色下峰值仍读得出，两侧实测见 P8-e 报告）。
	ink := inks.Tertiary
	// P8-e 之后这盏灯两档同墨（`Tertiary`），分开它们的是**形与动**：实心盘 +
	// 呼吸 = 有程序在跑，空心环 + 静止 = 没有（P13-ao）。
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
	//
	// P11-bm：**画不出来的点不读循环**。`drawn` 是浮条揭示的那一个状态
	// （`terminalui.go` 的 `revealed`）：未揭示时上层的身份行是
	// `identity.Opacity(0)`，点连同它的名字一起是零不透明度——可它仍然每帧
	// 调一次 `Element.Loop`，而 `Loop`（`ui/widgets.go:69-84`）要的是
	// `Context.AnimationFrame`，`runtime.go:744-748` 把它翻成「下一帧**重建**
	// 整棵视图」。于是终端页一开着、指针又不在浮条上，整个终端视图就按
	// 显示器刷新率无限重建（120 Hz 面板 = 每秒 120 次），为一个谁也看不见
	// 的动画。停在这里不读循环，揭示时的像素与动效逐字节不变。
	if !drawn {
		return
	}
	// P11-bn：**呼吸在画的那一帧里读，不再在构建的那一帧里读**。
	//
	// 之前这里读 `dot.Loop`，而 `Loop` 要的是 `Context.AnimationFrame`，
	// `runtime.go:744-748` 把它翻成「下一帧**重建**整棵视图」——揭示态于是按
	// 显示器刷新率重建整个终端视图（120 Hz = 每秒 120 次）为一个 1.75u 的圆点。
	// `Painter.AnimationFrameRate`（`ui/paint.go:126-145`）要的帧是**只重画**：
	// 「paints the elements of this one again without building the view」，
	// 而且按请求的帧率来。所以相位改在 `Draw` 里按 `p.Now()` 读，起点落在
	// App 的 `dotBreath` 上（元素状态随重建而丢，绘制不该丢）。
	//
	// 周期、缓动、两个 keyframe 与墨一个字节没动：呼吸仍是 2.4s 一回、
	// 半程到 50%（opacity 0.5 / scale 0.88）。减少动效仍走 `internal/motion`：
	// 它返回 0（0% keyframe）且**不读相位**，于是这一帧既不记起点，也不注册
	// `AnimationFrameRate`——静止的桌面拿不到任何帧。
	dot.Draw(func(p *ui.Painter, r ui.Rect) {
		breath := motion.Value(c, func() float32 {
			if a.dotBreath.IsZero() {
				a.dotBreath = p.Now()
			}
			phase := p.Now().Sub(a.dotBreath) % dotBreathPeriod
			return ui.Bounce(ui.EaseInOut)(float32(phase) / float32(dotBreathPeriod))
		})
		if !c.Preferences().ReduceMotion {
			p.AnimationFrameRate(dotBreathFPS)
		}
		size := min(r.W, r.H) * (1 - 0.12*breath)
		p.Fill(ui.Rect{
			X: r.X + (r.W-size)/2,
			Y: r.Y + (r.H-size)/2,
			W: size,
			H: size,
		}, ink.Alpha(1-0.5*breath), size/2)
	})
}

// The breath's own numbers: the old build's 2.4 s loop, drawn at the rate a
// slow fade needs rather than at the display's.
const (
	dotBreathPeriod = 2400 * time.Millisecond
	dotBreathFPS    = 30
)

// live reports whether a program is running on screen: there is a session and
// it has not ended. It is what the status dot's filled/dark pair answers, so the
// blank page — no session at all — lands on the dark side with an ended program
// rather than breathing a light for a process that does not exist (P13-ao).
func (a *App) live() bool {
	session := a.Current()
	if session == nil {
		return false
	}
	return !a.exited()
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
