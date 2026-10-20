package shell

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"floter/internal/i18n"
	"floter/internal/spawn"
)

// The launcher's hand-offs to the system: opening a path, opening the terminal
// surface in a directory, and opening the user's own terminal emulator. Each is
// one of the launcher's own actions — a dropped file's rows, the action bar's
// path action, the external-terminal shortcut — and each used to be an action
// the shell never answered.

// openPath hands a path to the system's own opener: the file manager for a
// directory, the default application for a file. This is what a dropped file's
// "open" row and the action bar's own path action do.
//
// The path is resolved the way the old `open_path` resolved it — `~` against
// the home directory, a relative path joined to it — and it has to be there: a
// typo answers the query instead of silently opening nothing, so the launcher
// can stay up to be corrected (`src-tauri/src/commands/actions.rs:73-98`).
func (a *App) openPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	fail := func(target string, err error) {
		log.Printf("floter: could not open %s: %v", target, err)
		a.onMain(func() { a.Launcher.WarnFeedback(a.Launcher.Copy().OpenPathFailed) })
	}
	home := a.homeDir
	if home == "" {
		found, err := os.UserHomeDir()
		if err != nil {
			fail(path, err)
			return
		}
		home = found
	}
	resolved := resolvePath(path, home)
	if _, err := os.Stat(resolved); err != nil {
		fail(resolved, err)
		return
	}
	if a.openPathCommand != nil {
		if err := a.openPathCommand(resolved); err != nil {
			fail(resolved, err)
		}
		return
	}
	// The platform's own opener, from the one shared table (`spawn.Open`):
	// ShellExecute by name on Windows, `open` on macOS, and `xdg-open` with the
	// reference's own fallbacks on Linux. The port had a second, hand-written
	// opener here (`systemOpener`), so a path and a page could be opened by two
	// different programs on the same machine.
	if err := spawn.Open(resolved); err != nil {
		fail(resolved, err)
	}
}

// resolvePath turns what the user typed into a path the opener can take: a
// leading `~` becomes the home directory, and anything relative is joined to
// home. It is the old `resolve_path` (`src-tauri/src/commands/actions.rs:107-135`),
// kept apart from `openPath` so the rule can be exercised without touching the
// filesystem.
func resolvePath(path, home string) string {
	rest, tilde := strings.CutPrefix(path, "~")
	if !tilde {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(home, path)
	}
	// `~alice` is not another user's home: nothing portable can resolve it, and
	// a directory literally called `~alice` is the likelier thing a launcher is
	// being asked about — so it is joined to home rather than treated as a user.
	if rest != "" && !os.IsPathSeparator(rest[0]) {
		return filepath.Join(home, path)
	}
	// The separators come off before joining: joining a path that starts with
	// one discards everything to its left and yields the filesystem root. Once
	// they are gone, `~` and `~/` are both simply the home directory.
	for rest != "" && os.IsPathSeparator(rest[0]) {
		rest = rest[1:]
	}
	if rest == "" {
		return home
	}
	return filepath.Join(home, rest)
}

// spawnGap is how long a process hand-off is left alone after one ran: a
// shortcut held down (or a global hotkey the desktop repeats while the key is
// down) fired the hand-off once per repeat, and every hand-off is a *process* —
// a shell in the panel, another emulator window, or a detached command line.
// The report 「这边的某些快捷键会导致反复弹出终端，从而导致卡死」 is that
// storm: the repeat rate of a held key outruns anything the user can see, and
// the machine ends up with dozens of shells and a window that cannot keep up.
// One hand-off per press is the intent of the key, so the second one inside the
// gap is a repeat, not a request.
const spawnGap = 750 * time.Millisecond

// The hand-off kinds the gate keeps apart. Two different hand-offs inside one
// gap are two presses — opening the user's emulator and then running a command
// is a person, not a repeating key — so each kind carries its own clock, and
// only a *repeat of the same hand-off* is dropped.
const (
	spawnSession  = "session"
	spawnExternal = "external"
	spawnCommand  = "command"
)

// allowSpawn reports whether a hand-off of this kind may run now, and records
// it when it may. Every route that starts a process — a session in the panel,
// the user's own emulator, a detached command line — asks here first, so the
// rule is one rule and not a guard per callback (the bug this closes was three
// routes sharing a storm and only one of them gated). The clock is the app's
// own, so the rule is a unit test rather than a stopwatch.
func (a *App) allowSpawn(kind string) bool {
	now := a.now()
	if a.spawnAt == nil {
		a.spawnAt = make(map[string]time.Time, 3)
	}
	if last := a.spawnAt[kind]; !last.IsZero() && now.Sub(last) < spawnGap {
		return false
	}
	a.spawnAt[kind] = now
	return true
}

// openTerminalSession is the one gate every in-panel session goes through: run
// starts the session, and the terminal surface comes forward either way, so a
// repeat of the press that started it shows that session rather than forking a
// second shell beside it. A session is a shell process, and a held key repeats
// at a rate nothing on screen can follow — the report 「反复弹出终端，从而导致
// 卡死」.
func (a *App) openTerminalSession(run func()) {
	if a.Terminal == nil {
		return
	}
	if a.allowSpawn(spawnSession) {
		run()
	}
	a.Open(SurfaceTerminal)
}

// openInTerminal opens the terminal surface in a directory: the dropped file's
// "cd" action, and the action bar's own path action. The session is a bare one
// (the user's shell), so the directory is somewhere to start rather than a
// command to watch.
func (a *App) openInTerminal(dir string) {
	a.openTerminalSession(func() { a.Terminal.RunShellIn(dir) })
}

// openTerminalWindow opens the user's own terminal emulator, with a login
// shell, for the external-terminal shortcut. The Go build's terminal runs in
// the panel, so this is a *new* window rather than a hand-off of the session
// the panel holds.
func (a *App) openTerminalWindow() {
	a.spawnExternalTerminal(func(err error) {
		log.Printf("floter: could not open the terminal: %v", err)
		a.onMain(func() { a.Launcher.WarnFeedback(a.Launcher.Copy().OpenTermFailed) })
	})
}

// spawnExternalTerminal is the one gate every external-terminal hand-off goes
// through: the launcher's row and shortcut, the terminal page's own hint, and a
// custom shortcut's action all ask here. Each hand-off is a *process* and a
// window, so a key the desktop repeats (or two bindings pointing at the same
// action) must not start one per repeat — the report 「某些快捷键会导致反复弹出
// 终端，从而导致卡死」 was exactly that storm. The failed callback says where a
// machine with no emulator to open reports it: the surface the press came from.
func (a *App) spawnExternalTerminal(failed func(error)) {
	if !a.allowSpawn(spawnExternal) {
		return
	}
	if err := a.OpenExternalTerminal(); err != nil {
		failed(err)
	}
}

// openTerminalLink opens what a Command+click in the terminal points at: a
// hyperlink or a URL printed as text goes to the browser the user chose, and a
// path goes to the file manager — the same two openers the rest of the app
// uses, so a link in the output behaves like a row in the launcher.
//
// A path printed with a `:line:col` suffix is opened without it when the whole
// string is not itself a file, which is how a compiler error reads. A relative
// path is left alone: the session's own directory is not tracked here, and
// guessing the app's would open the wrong file.
func (a *App) openTerminalLink(link string) bool {
	link = strings.TrimSpace(link)
	if link == "" {
		return false
	}
	if schemeOf(link) != "" {
		a.openURL("", link)
		return true
	}
	path := link
	if _, err := os.Stat(path); err != nil {
		if trimmed, ok := stripLineSuffix(path); ok {
			path = trimmed
		}
	}
	absolute := filepath.IsAbs(path) || strings.HasPrefix(path, "~")
	if !absolute {
		return false
	}
	a.openPath(path)
	return true
}

// schemeOf is a link's own scheme, empty when it has none: the thing that
// decides whether a click opens a page or a file.
func schemeOf(link string) string {
	index := strings.Index(link, "://")
	if index <= 0 {
		// A scheme without slashes: mailto:, file:, tel:.
		if colon := strings.Index(link, ":"); colon > 0 {
			scheme := link[:colon]
			if !strings.ContainsAny(scheme, `/\`) && scheme == strings.ToLower(scheme) {
				return scheme
			}
		}
		return ""
	}
	scheme := link[:index]
	if strings.ContainsAny(scheme, `/\ `) {
		return ""
	}
	return strings.ToLower(scheme)
}

// stripLineSuffix removes a `:line` or `:line:col` suffix from a path, for the
// paths a compiler prints.
func stripLineSuffix(path string) (string, bool) {
	base := path
	stripped := false
	// At most two: a line and a column. A suffix that is not digits ends the
	// walk with whatever was already stripped, so `C:/a/b.go:12` keeps its
	// drive letter while losing the line.
	for i := 0; i < 2; i++ {
		colon := strings.LastIndex(base, ":")
		if colon <= 0 || colon == len(base)-1 {
			break
		}
		digits := base[colon+1:]
		allDigits := true
		for _, r := range digits {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if !allDigits {
			break
		}
		base, stripped = base[:colon], true
	}
	if !stripped {
		return "", false
	}
	return base, true
}

// notifyFromProgram raises the notification a program asked for (OSC 9, OSC
// 777) while it ran in the panel's terminal: the program's own words, through
// the same channel the app's own completions use.
func (a *App) notifyFromProgram(title, body string) {
	if strings.TrimSpace(title) == "" && strings.TrimSpace(body) == "" {
		return
	}
	if title == "" {
		title = i18n.For(a.Store.Snapshot().Language).Notifications.Title
	}
	a.Notify(title, body)
}
