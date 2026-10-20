package shell

import (
	"errors"
	"log"
	"strings"

	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/shortcuts"
	"floter/internal/spawn"
)

// The user-defined global shortcuts: one system-wide key bound to one
// launcher action, or to a shell command line that runs with no window at all.
//
// A plugin action opens the plugin *normally* (the panel is shown and the mode
// entered); a plugin is a visible thing, never a silent one. A command line is
// the opposite: it runs detached, with no window, no terminal and no UI, which
// is what makes it useful for a quick script.

// setCustomShortcuts persists a new list and registers it, reporting the keys
// that were refused: a key that shadows one of the app's own shortcuts is a
// `conflict`, one the system will not take is `occupied`. It is the only writer
// of the list, and it writes back only the keys that registered — a refused key
// is not left on disk to be retried on the next launch, as the old host's
// `set_custom_shortcuts` had it (`src-tauri/src/commands/config.rs:1666-1718`).
func (a *App) setCustomShortcuts(entries []settings.CustomShortcut) []settingsui.CustomShortcutRejection {
	registered, rejections := a.applyCustomShortcuts(entries)
	// Record the signature of what is about to be written *before* the write, so
	// the store's own listener sees the list it just registered and does not
	// claim the keys a second time.
	a.customSignature = customShortcutSignatureOf(registered)
	if err := a.Store.Update(func(s *settings.Settings) { s.SetCustomShortcuts(registered) }); err != nil {
		log.Printf("floter: could not save the custom shortcuts: %v", err)
	}
	out := make([]settingsui.CustomShortcutRejection, 0, len(rejections))
	for _, rejection := range rejections {
		out = append(out, settingsui.CustomShortcutRejection{Key: rejection.Key, Reason: rejection.Reason})
	}
	return out
}

// customShortcutSignature is the stored list as one comparable string, so the
// settings listener only re-registers when the list really changed.
func customShortcutSignature(s settings.Settings) string {
	return customShortcutSignatureOf(settings.CustomShortcutsOf(s))
}

// customShortcutSignatureOf is the same signature for a list that has not been
// written yet.
func customShortcutSignatureOf(entries []settings.CustomShortcut) string {
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(entry.Key)
		builder.WriteByte(0)
		builder.WriteString(entry.Action)
		builder.WriteByte(1)
	}
	return builder.String()
}

// ApplyCustomShortcuts re-registers the user's own shortcuts from the settings
// file, reporting the keys that could not be claimed. The keys that are no
// longer bound are released first, so a change made in the settings file or the
// settings page takes effect at once.
func (a *App) ApplyCustomShortcuts() []CustomShortcutRejection {
	snapshot := a.Store.Snapshot()
	a.customSignature = customShortcutSignature(snapshot)
	_, rejections := a.applyCustomShortcuts(settings.CustomShortcutsOf(snapshot))
	return rejections
}

// applyCustomShortcuts is the one registration path — startup, a settings-file
// change, the resume after a recorder and the settings page all go through it —
// so the conflict and occupancy rules are the same everywhere. It releases the
// current keys and claims the whole list, returning the entries that registered
// and the ones refused. A key that shadows one of the app's own shortcuts (or
// the summon key) is a `conflict`; one the system will not take is `occupied`.
// Every entry that reaches it has already been normalized into a whole binding,
// so an unparseable key is dropped before it can be reported.
func (a *App) applyCustomShortcuts(entries []settings.CustomShortcut) ([]settings.CustomShortcut, []CustomShortcutRejection) {
	for _, key := range a.customKeys {
		a.unregisterShortcut(key)
	}
	a.customKeys = nil

	snapshot := a.Store.Snapshot()
	normalized := settings.NormalizeCustomShortcuts(entries)
	registered := make([]settings.CustomShortcut, 0, len(normalized))
	claimed := make([]string, 0, len(normalized))
	rejections := make([]CustomShortcutRejection, 0)
	for _, entry := range normalized {
		accelerator := entry.Key
		if shortcuts.Equal(accelerator, a.summonKey) || settings.CustomShortcutConflict(snapshot, accelerator) {
			rejections = append(rejections, CustomShortcutRejection{Key: entry.Key, Reason: settingsui.CustomShortcutReasonConflict})
			continue
		}
		if shortcuts.Duplicate(accelerator, claimed, -1) != "" {
			rejections = append(rejections, CustomShortcutRejection{Key: entry.Key, Reason: settingsui.CustomShortcutReasonConflict})
			continue
		}
		action := entry.Action
		if err := a.registerShortcut(accelerator, func() { a.runCustomShortcut(action) }); err != nil {
			rejections = append(rejections, CustomShortcutRejection{Key: entry.Key, Reason: settingsui.CustomShortcutReasonOccupied})
			continue
		}
		registered = append(registered, entry)
		claimed = append(claimed, accelerator)
	}
	a.customKeys = claimed
	for _, rejection := range rejections {
		log.Printf("floter: custom shortcut %s: %s", rejection.Key, rejection.Reason)
	}
	return registered, rejections
}

// CustomShortcutRejection is a key the system would not take.
type CustomShortcutRejection struct {
	Key    string
	Reason string
}

// runCustomShortcut runs one custom shortcut's action.
func (a *App) runCustomShortcut(action string) {
	kind, value := settings.ClassifyCustomShortcut(action)
	switch kind {
	case "plugin":
		a.openPluginMode(value)
	case "action":
		a.runAppAction(value)
	default:
		// A command line is a process too, and a held shortcut repeats: the
		// same gate the terminal hand-offs use keeps one press to one process
		// (the report 「某些快捷键会导致反复弹出终端」 covers a shortcut bound
		// to a command that opens a window of its own).
		if !a.allowSpawn(spawnCommand) {
			return
		}
		if err := a.runSilentCommand(value); err != nil {
			log.Printf("floter: custom shortcut command %q: %v", value, err)
		}
	}
}

// openPluginMode shows the panel with a built-in plugin's mode open: the
// clipboard history, the browser search or the calculator history. The three
// ids are the same vocabulary the settings page's base-plugin rows and the
// configuration overlay use; an id nothing names is a bug, not a mode.
func (a *App) openPluginMode(id string) {
	a.Open(SurfaceLauncher)
	switch id {
	case settings.CustomPluginClipboard:
		a.Launcher.EnterClipboard()
	case settings.CustomPluginBrowser:
		a.Launcher.EnterBrowser()
	case settings.CustomPluginCalculator:
		a.Launcher.EnterCalculator()
	default:
		log.Printf("floter: no plugin mode named %q", id)
	}
}

// runAppAction runs one of the app's own actions.
func (a *App) runAppAction(id string) {
	switch id {
	case settings.CustomActionToggleWindow:
		a.Toggle()
	case settings.CustomActionNewCommand:
		a.Launcher.ResetQuery()
		a.Open(SurfaceLauncher)
	case settings.CustomActionOpenSettings:
		a.Open(SurfaceSettings)
	case settings.CustomActionOpenExternalTerminal:
		// A custom shortcut is the same hand-off as the built-in one, so it goes
		// through the same single-flight gate: a held key repeats, and each
		// hand-off is a process.
		a.openTerminalWindow()
	default:
		log.Printf("floter: no action named %q", id)
	}
}

// runSilentCommand runs a shell command line with no window, no terminal and
// no UI: the user's shell runs it detached, so it outlives the app if it
// wants to.
func (a *App) runSilentCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("no command to run")
	}
	if a.silentCommand != nil {
		return a.silentCommand(command)
	}
	return spawn.Command(command)
}

// OpenExternalTerminal opens the user's terminal emulator at the directory the
// active session reported, so work can continue in a real terminal window where
// it was. The Go build runs its terminal in-process, so this is a *new* shell
// rather than a hand-off of the running session — the directory is what carries
// the session over.
func (a *App) OpenExternalTerminal() error {
	if a.openExternalTerminal != nil {
		return a.openExternalTerminal()
	}
	return terminalAt(a.externalTerminalDir())
}

// terminalAt is the one call that starts an emulator, named so a test can stand
// in for the process.
var terminalAt = spawn.TerminalAt

// externalTerminalDir is where a hand-off opens: the active session's own
// directory (the `OSC 7` report `terminalui` keeps), and "" — a login shell
// wherever the emulator opens — while no session has reported one yet. The
// reference reads the same field off its session
// (`main:src-tauri/src/terminal/session.rs:643-663`: `open_terminal_session(dir,
// …)`), and hands off from the launcher as well as from the terminal page, so
// the surface on screen does not decide this.
func (a *App) externalTerminalDir() string {
	if a.Terminal == nil {
		return ""
	}
	return a.Terminal.DirOf(a.Terminal.Active)
}
