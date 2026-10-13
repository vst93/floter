package settings

import (
	"runtime"
	"strings"

	"floter/internal/shortcuts"
)

// The app's own shortcut map: one accelerator per action, rebindable, with the
// platform defaults every earlier build shipped. It is separate from the
// global summon key (the `hotkey` field, which is the only shortcut the OS
// holds) and from the user's custom shortcuts: these are the keys the app
// answers while its window has the keyboard.
const shortcutMapKey = "shortcuts"

// The actions the map holds.
const (
	ShortcutToggleWindow         = "toggle_window"
	ShortcutNewCommand           = "new_command"
	ShortcutOpenExternalTerminal = "open_external_terminal"
	ShortcutCopySelection        = "copy_selection"
	ShortcutPaste                = "paste"
	ShortcutOpenSettings         = "open_settings"
	ShortcutSelectResult         = "select_result"
)

// DefaultSummonShortcut is the key the app registers when the user never
// changed it: the one action whose binding the system holds.
const DefaultSummonShortcut = "Ctrl+Space"

// ShortcutActions lists every action, in the order a settings page shows them.
var ShortcutActions = []string{
	ShortcutToggleWindow,
	ShortcutNewCommand,
	ShortcutOpenExternalTerminal,
	ShortcutCopySelection,
	ShortcutPaste,
	ShortcutOpenSettings,
	ShortcutSelectResult,
}

// defaultShortcuts is the shipped map: the app modifier is Command on macOS
// and Control elsewhere, which is what the `CmdOrCtrl` spelling means.
func defaultShortcuts() map[string]string {
	app := "Ctrl"
	if runtime.GOOS == "darwin" {
		app = "Cmd"
	}
	return map[string]string{
		ShortcutToggleWindow:         DefaultSummonShortcut,
		ShortcutNewCommand:           app + "+W",
		ShortcutOpenExternalTerminal: app + "+N",
		ShortcutCopySelection:        copySelectionDefault(),
		ShortcutPaste:                pasteDefault(),
		ShortcutOpenSettings:         app + "+,",
		ShortcutSelectResult:         app + "+1",
	}
}

// copySelectionDefault and pasteDefault keep the terminal's own conventions:
// the app modifier on macOS, and the shifted keys elsewhere so a terminal's
// Ctrl+C stays the interrupt.
func copySelectionDefault() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+C"
	}
	return "Ctrl+Shift+C"
}

func pasteDefault() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+V"
	}
	return "Ctrl+Shift+V"
}

// ShortcutsOf reads the map: every action is present, a stored binding
// overrides the shipped one, and a binding this build cannot parse falls back
// to the default rather than to nothing. `select_result` is a shortcut family
// rather than one literal key, so its stored value is reduced to the family
// head — the user's modifiers and the digit `1` — and a value with no modifier
// at all falls back to the default (a bare digit would otherwise launch a row
// on ordinary number entry, as the old build's `normalizeResultShortcut`
// refused).
func ShortcutsOf(s Settings) map[string]string {
	resolved := defaultShortcuts()
	raw, ok := s.Extra()[shortcutMapKey].(map[string]any)
	if !ok {
		return resolved
	}
	for action, value := range raw {
		if !isShortcutAction(action) {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		if action == ShortcutSelectResult {
			if normalized, ok := shortcuts.NormalizeResultShortcut(text); ok {
				resolved[action] = normalized
			}
			continue
		}
		if normalized, ok := shortcuts.Normalize(text); ok {
			resolved[action] = normalized
		}
	}
	return resolved
}

// Shortcut is one action's binding, for a settings page.
func Shortcut(s Settings, action string) string {
	return ShortcutsOf(s)[action]
}

// SetShortcut records one action's binding; an unparseable accelerator is
// refused so a typo cannot leave an action with no key at all. `select_result`
// is reduced to its family head on the way in, so the stored binding is the
// one every digit in the family is drawn from.
func (s *Settings) SetShortcut(action, accelerator string) bool {
	if !isShortcutAction(action) {
		return false
	}
	normalized, ok := normalizeShortcut(action, accelerator)
	if !ok {
		return false
	}
	raw := map[string]any{}
	for key, value := range s.shortcutMapRaw() {
		raw[key] = value
	}
	raw[action] = normalized
	s.SetExtra(shortcutMapKey, raw)
	return true
}

// normalizeShortcut is one action's binding as the map stores it: the
// accelerator itself, except for `select_result`, which is a shortcut family
// stored as its head (the user's modifiers and the digit `1`).
func normalizeShortcut(action, accelerator string) (string, bool) {
	if action == ShortcutSelectResult {
		return shortcuts.NormalizeResultShortcut(accelerator)
	}
	return shortcuts.Normalize(accelerator)
}

// ShortcutRecording is what pressing a chord on one action's recorder amounts
// to for the map.
type ShortcutRecording int

const (
	// ShortcutRecorded: the binding was new and was stored.
	ShortcutRecorded ShortcutRecording = iota
	// ShortcutUnchanged: the action already held that binding, so recording it
	// again changes nothing (the old build's `capture` returned without
	// writing rather than round-tripping the same value).
	ShortcutUnchanged
	// ShortcutRefused: another action already answers for the chord — one key
	// never runs two things — or the chord is not a binding this build can
	// parse. Nothing is stored and the page says so.
	ShortcutRefused
)

// ShortcutRecordingOf is the recorder's own rule, as `src/hooks/
// useShortcutCapture.ts:126-135` had it: the chord is reduced to the binding
// that would be stored (`select_result`'s family head included), a chord
// another action already holds is refused, and re-recording the action's own
// chord is no change at all. The map is the app's own keys, so the comparison
// is the accelerator's, not the platform's: the two spellings of one binding
// (`Cmd+,` and `Cmd+Comma`) are the same key here too.
func ShortcutRecordingOf(s Settings, action, accelerator string) ShortcutRecording {
	if !isShortcutAction(action) {
		return ShortcutRefused
	}
	normalized, ok := normalizeShortcut(action, accelerator)
	if !ok {
		return ShortcutRefused
	}
	if shortcuts.Equal(Shortcut(s, action), normalized) {
		return ShortcutUnchanged
	}
	for _, candidate := range ShortcutActions {
		if candidate == action {
			continue
		}
		if shortcuts.Equal(Shortcut(s, candidate), normalized) {
			return ShortcutRefused
		}
	}
	return ShortcutRecorded
}

// shortcutMapRaw is the stored map, as decoded.
func (s *Settings) shortcutMapRaw() map[string]any {
	raw, _ := s.extra[shortcutMapKey].(map[string]any)
	return raw
}

// isShortcutAction reports whether an action id is one this build knows.
func isShortcutAction(action string) bool {
	for _, candidate := range ShortcutActions {
		if candidate == action {
			return true
		}
	}
	return false
}

// SelectResultDigit is the accelerator that runs the n-th result: the digit
// follows the select_result binding's modifiers, so rebinding the family moves
// every number with it.
func SelectResultDigit(s Settings, digit int) string {
	binding := Shortcut(s, ShortcutSelectResult)
	if digit < 0 || digit > 9 {
		return ""
	}
	key := string(rune('0' + digit))
	if index := strings.LastIndex(binding, "+"); index >= 0 {
		return binding[:index+1] + key
	}
	return key
}
