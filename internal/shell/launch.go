package shell

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	if err := spawn.Program(systemOpener(), resolved); err != nil {
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

// systemOpener is the platform's own opener: the one command every desktop
// answers with a path.
func systemOpener() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "explorer"
	default:
		return "xdg-open"
	}
}

// openInTerminal opens the terminal surface in a directory: the dropped file's
// "cd" action, and the action bar's own path action. The session is a bare one
// (the user's shell), so the directory is somewhere to start rather than a
// command to watch.
func (a *App) openInTerminal(dir string) {
	if a.Terminal == nil {
		return
	}
	a.Terminal.RunShellIn(dir)
	a.Open(SurfaceTerminal)
}

// openTerminalWindow opens the user's own terminal emulator, with a login
// shell, for the external-terminal shortcut. The Go build's terminal runs in
// the panel, so this is a *new* window rather than a hand-off of the session
// the panel holds.
func (a *App) openTerminalWindow() {
	if err := a.OpenExternalTerminal(); err != nil {
		log.Printf("floter: could not open the terminal: %v", err)
		a.onMain(func() { a.Launcher.WarnFeedback(a.Launcher.Copy().OpenTermFailed) })
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
