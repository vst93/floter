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
// the system refused.
func (a *App) setCustomShortcuts(entries []settings.CustomShortcut) []settingsui.CustomShortcutRejection {
	if err := a.Store.Update(func(s *settings.Settings) { s.SetCustomShortcuts(entries) }); err != nil {
		log.Printf("floter: could not save the custom shortcuts: %v", err)
	}
	rejections := a.ApplyCustomShortcuts()
	out := make([]settingsui.CustomShortcutRejection, 0, len(rejections))
	for _, rejection := range rejections {
		out = append(out, settingsui.CustomShortcutRejection{Key: rejection.Key, Reason: rejection.Reason})
	}
	return out
}

// customShortcutSignature is the stored list as one comparable string, so the
// settings listener only re-registers when the list really changed.
func customShortcutSignature(s settings.Settings) string {
	var builder strings.Builder
	for _, entry := range settings.CustomShortcutsOf(s) {
		builder.WriteString(entry.Key)
		builder.WriteByte(0)
		builder.WriteString(entry.Action)
		builder.WriteByte(1)
	}
	return builder.String()
}

// ApplyCustomShortcuts re-registers the user's own shortcuts: the keys that
// are no longer bound are released first, so a change made in the settings
// file or the settings page takes effect at once. A key the system refuses
// (another application holds it, or it shadows the summon key) is reported and
// skipped, never silently swallowed.
func (a *App) ApplyCustomShortcuts() []CustomShortcutRejection {
	for _, key := range a.customKeys {
		a.unregisterShortcut(key)
	}
	a.customKeys = nil

	snapshot := a.Store.Snapshot()
	a.customSignature = customShortcutSignature(snapshot)
	entries := settings.CustomShortcutsOf(snapshot)
	registered := make([]string, 0, len(entries))
	rejections := make([]CustomShortcutRejection, 0)
	for _, entry := range entries {
		accelerator, ok := shortcuts.Normalize(entry.Key)
		if !ok {
			rejections = append(rejections, CustomShortcutRejection{Key: entry.Key, Reason: "the accelerator is not one the system accepts"})
			continue
		}
		if accelerator == a.summonKey {
			rejections = append(rejections, CustomShortcutRejection{Key: entry.Key, Reason: "the key already summons floter"})
			continue
		}
		action := entry.Action
		if err := a.registerShortcut(accelerator, func() { a.runCustomShortcut(action) }); err != nil {
			rejections = append(rejections, CustomShortcutRejection{Key: entry.Key, Reason: err.Error()})
			continue
		}
		registered = append(registered, accelerator)
	}
	a.customKeys = registered
	for _, rejection := range rejections {
		log.Printf("floter: custom shortcut %s: %s", rejection.Key, rejection.Reason)
	}
	return rejections
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
		if err := a.runSilentCommand(value); err != nil {
			log.Printf("floter: custom shortcut command %q: %v", value, err)
		}
	}
}

// openPluginMode shows the panel with a built-in plugin's mode open: the
// clipboard history or the browser search.
func (a *App) openPluginMode(id string) {
	a.Open(SurfaceLauncher)
	switch id {
	case settings.CustomPluginClipboard:
		a.Launcher.EnterClipboard()
	case settings.CustomPluginBrowser:
		a.Launcher.EnterBrowser()
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
		if err := a.OpenExternalTerminal(); err != nil {
			log.Printf("floter: could not open a terminal: %v", err)
		}
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

// OpenExternalTerminal opens the user's terminal emulator with a login shell,
// so work can continue in a real terminal window. The Go build runs its
// terminal in-process, so this is a *new* shell rather than a hand-off of the
// running session.
func (a *App) OpenExternalTerminal() error {
	if a.openExternalTerminal != nil {
		return a.openExternalTerminal()
	}
	return openTerminalEmulator()
}

// openTerminalEmulator starts the first terminal emulator this machine has.
func openTerminalEmulator() error {
	return spawn.Terminal()
}
