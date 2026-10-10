package shell

import (
	"log"
	"runtime"
	"strings"

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
func (a *App) openPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if a.openPathCommand != nil {
		if err := a.openPathCommand(path); err != nil {
			log.Printf("floter: could not open %s: %v", path, err)
			a.onMain(func() { a.Launcher.WarnFeedback(a.Launcher.Copy().InvokeFailed) })
		}
		return
	}
	if err := spawn.Program(systemOpener(), path); err != nil {
		log.Printf("floter: could not open %s: %v", path, err)
		a.onMain(func() { a.Launcher.WarnFeedback(a.Launcher.Copy().InvokeFailed) })
	}
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
		a.onMain(func() { a.Launcher.WarnFeedback(a.Launcher.Copy().InvokeFailed) })
	}
}
