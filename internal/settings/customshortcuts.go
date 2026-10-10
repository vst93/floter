package settings

import (
	"strings"

	"floter/internal/shortcuts"
)

// customShortcutsKey is the settings key the user-defined global shortcuts
// live under. The block is carried through (this package does not model it as
// a field) but typed here so the settings page and the shell agree on the
// shape and on the normalization.
const customShortcutsKey = "custom_shortcuts"

// CustomShortcut binds one system-wide key to one launcher action.
type CustomShortcut struct {
	// Key is the accelerator, in either spelling the app has stored.
	Key string
	// Action is what the key runs: `plugin:<id>` opens a built-in plugin's
	// mode, `action:<id>` runs one of the app's own actions, and anything
	// else is a shell command line run silently.
	Action string
}

// The action vocabulary the picker offers. A free-form command line is
// anything outside it.
const (
	// CustomActionPluginPrefix marks a built-in plugin's launcher mode.
	CustomActionPluginPrefix = "plugin:"
	// CustomActionPrefix marks one of the app's own actions.
	CustomActionPrefix = "action:"

	// The plugin ids the picker offers.
	CustomPluginClipboard  = "clipboard"
	CustomPluginBrowser    = "browser"
	CustomPluginCalculator = "calculator"

	// The app actions the picker offers.
	CustomActionToggleWindow         = "toggle_window"
	CustomActionNewCommand           = "new_command"
	CustomActionOpenSettings         = "open_settings"
	CustomActionOpenExternalTerminal = "open_external_terminal"
)

// CustomShortcutsOf reads the list from the settings file, normalized.
func CustomShortcutsOf(s Settings) []CustomShortcut {
	raw, ok := s.Extra()[customShortcutsKey].([]any)
	if !ok {
		return nil
	}
	entries := make([]CustomShortcut, 0, len(raw))
	for _, value := range raw {
		record, ok := value.(map[string]any)
		if !ok {
			continue
		}
		key, _ := record["key"].(string)
		action, _ := record["action"].(string)
		entries = append(entries, CustomShortcut{Key: key, Action: action})
	}
	return NormalizeCustomShortcuts(entries)
}

// SetCustomShortcuts writes the list back, normalized.
func (s *Settings) SetCustomShortcuts(entries []CustomShortcut) {
	normalized := NormalizeCustomShortcuts(entries)
	raw := make([]any, 0, len(normalized))
	for _, entry := range normalized {
		raw = append(raw, map[string]any{"key": entry.Key, "action": entry.Action})
	}
	s.SetExtra(customShortcutsKey, raw)
}

// NormalizeCustomShortcuts drops entries that are not a whole binding and
// collapses duplicate keys: an entry with no key or no action is a draft, not
// a binding, and of two entries claiming one key the first wins.
func NormalizeCustomShortcuts(entries []CustomShortcut) []CustomShortcut {
	out := make([]CustomShortcut, 0, len(entries))
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entry.Key)
		action := strings.TrimSpace(entry.Action)
		if key == "" || action == "" {
			continue
		}
		if shortcuts.Duplicate(key, keys, -1) != "" {
			continue
		}
		keys = append(keys, key)
		out = append(out, CustomShortcut{Key: key, Action: action})
	}
	return out
}

// ClassifyCustomShortcut splits an action string into what runs it: the
// plugin id, the app action id, or a shell command line.
func ClassifyCustomShortcut(action string) (kind, value string) {
	trimmed := strings.TrimSpace(action)
	if rest, ok := strings.CutPrefix(trimmed, CustomActionPluginPrefix); ok {
		return "plugin", strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutPrefix(trimmed, CustomActionPrefix); ok {
		return "action", strings.TrimSpace(rest)
	}
	return "command", trimmed
}
