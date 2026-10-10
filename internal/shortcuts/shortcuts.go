// Package shortcuts translates between the accelerator spellings floter
// stores and the ones the framework registers.
//
// The old build wrote its own names — "Cmd+Comma", "Alt+Space",
// "CmdOrCtrl+Shift+Space" — and the framework takes a similar but not
// identical vocabulary ("Cmd+,", "Space", …). Normalize is the bridge, and
// FromKey turns a key event into the same spelling, for the settings page's
// recorder.
package shortcuts

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Modifier names, in the order an accelerator writes them.
const (
	modCmd   = "Cmd"
	modCtrl  = "Ctrl"
	modAlt   = "Alt"
	modShift = "Shift"
)

// keyNames maps the names the old build wrote to the spelling the framework
// accepts. A name that is already a single character passes through.
var keyNames = map[string]string{
	"comma":              ",",
	"period":             ".",
	"dot":                ".",
	"slash":              "/",
	"semicolon":          ";",
	"quote":              "'",
	"apostrophe":         "'",
	"bracketleft":        "[",
	"bracketright":       "]",
	"backslash":          "\\",
	"backquote":          "`",
	"grave":              "`",
	"minus":              "-",
	"equal":              "=",
	"equals":             "=",
	"plus":               "Plus",
	"space":              "Space",
	"spacebar":           "Space",
	"enter":              "Enter",
	"return":             "Enter",
	"escape":             "Esc",
	"esc":                "Esc",
	"backspace":          "Backspace",
	"delete":             "Delete",
	"del":                "Delete",
	"tab":                "Tab",
	"up":                 "Up",
	"down":               "Down",
	"left":               "Left",
	"right":              "Right",
	"home":               "Home",
	"end":                "End",
	"pageup":             "PageUp",
	"pagedown":           "PageDown",
	"insert":             "Insert",
	"ins":                "Insert",
	"printscreen":        "PrintScreen",
	"capslock":           "CapsLock",
	"numlock":            "NumLock",
	"scrolllock":         "ScrollLock",
	"volumeup":           "VolumeUp",
	"volumedown":         "VolumeDown",
	"volumemute":         "VolumeMute",
	"mediaplaypause":     "MediaPlayPause",
	"medianexttrack":     "MediaNextTrack",
	"mediaprevioustrack": "MediaPreviousTrack",
	"mediastop":          "MediaStop",
}

// modifierNames maps the modifiers the old build wrote onto the framework's.
var modifierNames = map[string]string{
	"cmd":              modCmd,
	"command":          modCmd,
	"super":            modCmd,
	"meta":             modCmd,
	"win":              modCmd,
	"cmdorctrl":        "CmdOrCtrl",
	"commandorcontrol": "CmdOrCtrl",
	"ctrl":             modCtrl,
	"control":          modCtrl,
	"alt":              modAlt,
	"option":           modAlt,
	"altgr":            modAlt,
	"shift":            modShift,
}

// Normalize rewrites an accelerator into the framework's spelling, and
// reports whether it is one the framework can register.
func Normalize(accelerator string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(accelerator), "+")
	if len(parts) == 0 {
		return "", false
	}
	var modifiers []string
	key := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if name, ok := modifierNames[strings.ToLower(part)]; ok {
			if !containsFold(modifiers, name) {
				modifiers = append(modifiers, name)
			}
			continue
		}
		if key != "" {
			return "", false // two keys
		}
		normalized, ok := normalizeKey(part)
		if !ok {
			return "", false
		}
		key = normalized
	}
	if key == "" {
		return "", false
	}
	return join(modifiers, key), true
}

// normalizeKey rewrites one key name.
func normalizeKey(name string) (string, bool) {
	lower := strings.ToLower(name)
	if mapped, ok := keyNames[lower]; ok {
		return mapped, true
	}
	if strings.HasPrefix(lower, "num") && len(lower) == 4 {
		return "Num" + strings.ToUpper(lower[3:]), true
	}
	if len(lower) >= 2 && lower[0] == 'f' {
		if number, ok := digits(lower[1:]); ok && number >= 1 && number <= 24 {
			return "F" + lower[1:], true
		}
	}
	if len([]rune(name)) == 1 {
		// A letter is written upper case, as every example does; digits and
		// signs keep themselves.
		if r := []rune(name)[0]; r >= 'a' && r <= 'z' {
			return strings.ToUpper(name), true
		}
		return name, true
	}
	// A modifier on its own, or a name the framework does not know.
	return "", false
}

// digits parses a plain decimal string, reporting false for anything else.
func digits(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	value := 0
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, false
		}
		value = value*10 + int(r-'0')
	}
	return value, true
}

// join writes the modifiers in the framework's order, then the key.
func join(modifiers []string, key string) string {
	order := []string{"CmdOrCtrl", modCmd, modCtrl, modAlt, modShift}
	ordered := make([]string, 0, len(modifiers)+1)
	for _, name := range order {
		for _, modifier := range modifiers {
			if modifier == name {
				ordered = append(ordered, name)
			}
		}
	}
	ordered = append(ordered, key)
	return strings.Join(ordered, "+")
}

// FromKey builds an accelerator from a key event, for the recorder. It
// requires a modifier other than Shift: a bare key would take over ordinary
// typing.
func FromKey(mods ui.Modifiers, key ui.Key) (string, bool) {
	if mods&(ui.Super|ui.Ctrl|ui.Alt) == 0 {
		return "", false
	}
	name, ok := KeyName(key)
	if !ok {
		return "", false
	}
	var modifiers []string
	if mods&ui.Super != 0 {
		modifiers = append(modifiers, modCmd)
	}
	if mods&ui.Ctrl != 0 {
		modifiers = append(modifiers, modCtrl)
	}
	if mods&ui.Alt != 0 {
		modifiers = append(modifiers, modAlt)
	}
	if mods&ui.Shift != 0 {
		modifiers = append(modifiers, modShift)
	}
	return join(modifiers, name), true
}

// KeyName is the accelerator spelling of a key, and whether it has one.
func KeyName(key ui.Key) (string, bool) {
	switch {
	case key >= ui.KeyA && key <= ui.KeyZ:
		return string(rune('A' + int(key-ui.KeyA))), true
	case key >= ui.Key0 && key <= ui.Key9:
		return string(rune('0' + int(key-ui.Key0))), true
	case key >= ui.KeyF1 && key <= ui.KeyF12:
		return "F" + itoa(int(key-ui.KeyF1)+1), true
	}
	switch key {
	case ui.KeySpace:
		return "Space", true
	case ui.KeyEnter:
		return "Enter", true
	case ui.KeyEscape:
		return "Esc", true
	case ui.KeyBackspace:
		return "Backspace", true
	case ui.KeyDelete:
		return "Delete", true
	case ui.KeyInsert:
		return "Insert", true
	case ui.KeyTab:
		return "Tab", true
	case ui.KeyUp:
		return "Up", true
	case ui.KeyDown:
		return "Down", true
	case ui.KeyLeft:
		return "Left", true
	case ui.KeyRight:
		return "Right", true
	case ui.KeyHome:
		return "Home", true
	case ui.KeyEnd:
		return "End", true
	case ui.KeyPageUp:
		return "PageUp", true
	case ui.KeyPageDown:
		return "PageDown", true
	case ui.KeyMinus:
		return "-", true
	case ui.KeyEqual:
		return "=", true
	case ui.KeyComma:
		return ",", true
	case ui.KeyPeriod:
		return ".", true
	case ui.KeySlash:
		return "/", true
	case ui.KeySemicolon:
		return ";", true
	case ui.KeyQuote:
		return "'", true
	case ui.KeyBracketLeft:
		return "[", true
	case ui.KeyBracketRight:
		return "]", true
	case ui.KeyBackslash:
		return "\\", true
	case ui.KeyBackquote:
		return "`", true
	}
	return "", false
}

// NormalizeOr normalizes an accelerator, returning it unchanged when the
// framework would not take it: a settings file with a value from another
// build must not lose it just because this one cannot parse it.
func NormalizeOr(accelerator string) string {
	if normalized, ok := Normalize(accelerator); ok {
		return normalized
	}
	return accelerator
}

// Display renders an accelerator the way a list shows it: the old build's
// spelling, with Command written as the platform's symbol.
func Display(accelerator string) string {
	normalized, ok := Normalize(accelerator)
	if !ok {
		return accelerator
	}
	// The spelling is read back from the normalized text, so the display
	// never has to map an enum back to a name: the parts are the modifiers
	// and the key.
	parts := strings.Split(normalized, "+")
	key := parts[len(parts)-1]
	modifiers := parts[:len(parts)-1]
	// macOS shows the glyphs a Mac user reads (⌘⇧⌥⌃); elsewhere the modifier
	// words, and "Super" rather than "Win" so a binding recorded as Super+K
	// reads back as the key that was pressed, as the old build's
	// `formatShortcut` did.
	if runtime.GOOS == "darwin" {
		var out strings.Builder
		for _, name := range modifiers {
			switch name {
			case modCtrl:
				out.WriteString("\u2303")
			case modAlt:
				out.WriteString("\u2325")
			case modShift:
				out.WriteString("\u21e7")
			case modCmd:
				out.WriteString("\u2318")
			case "CmdOrCtrl":
				out.WriteString("\u2318")
			}
		}
		return out.String() + displayKey(key)
	}
	var out strings.Builder
	for _, name := range modifiers {
		switch name {
		case modCtrl, "CmdOrCtrl":
			out.WriteString("Ctrl + ")
		case modAlt:
			out.WriteString("Alt + ")
		case modShift:
			out.WriteString("Shift + ")
		case modCmd:
			out.WriteString("Super + ")
		}
	}
	return out.String() + displayKey(key)
}

// displayKey is a key's own spelling for display: a single letter is upper
// case, and the rest keep the name the parser gave them.
func displayKey(key string) string {
	if len([]rune(key)) == 1 {
		return strings.ToUpper(key)
	}
	return key
}

func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// Equal reports whether two accelerators are the same *binding on this
// platform*. The two spellings the app has stored ("Cmd+Comma" and "Cmd+,")
// resolve to one, and `CmdOrCtrl` is resolved to the modifier the platform
// actually binds (Cmd on macOS, Ctrl elsewhere) — so `CmdOrCtrl+Shift+P` and
// `Cmd+Shift+P` are one key on macOS and two different keys on Linux, which is
// exactly what the system will do with them. An accelerator neither side can
// parse compares as trimmed, case-folded text.
func Equal(left, right string) bool {
	normalizedLeft, okLeft := Normalize(left)
	normalizedRight, okRight := Normalize(right)
	if okLeft && okRight {
		return resolvePlatformModifier(normalizedLeft) == resolvePlatformModifier(normalizedRight)
	}
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

// resolvePlatformModifier replaces the portable `CmdOrCtrl` with the modifier
// this platform binds, leaving everything else alone.
func resolvePlatformModifier(accelerator string) string {
	if !strings.Contains(accelerator, "CmdOrCtrl") {
		return accelerator
	}
	resolved := modCtrl
	if runtime.GOOS == "darwin" {
		resolved = modCmd
	}
	return strings.ReplaceAll(accelerator, "CmdOrCtrl", resolved)
}

// Duplicate returns the first key in keys that is the same binding as key, or
// an empty string. ignoreIndex skips the row being edited.
func Duplicate(key string, keys []string, ignoreIndex int) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	for index, existing := range keys {
		if index == ignoreIndex {
			continue
		}
		if Equal(existing, key) {
			return existing
		}
	}
	return ""
}

// Badge formats a binding as the compact key badge a row prints: the glyphs
// on macOS (⌘⇧⌥⌃) and the words elsewhere (Ctrl, Alt, Shift, Super), with the
// key appended — no spaces, because a badge is a key, not a sentence. The
// `enter` spelling renders as ↩ on every platform, as the old build's result
// badge did.
func Badge(accelerator, key string) string {
	mods, _, ok := Parse(accelerator)
	if !ok {
		return key
	}
	var out strings.Builder
	if runtime.GOOS == "darwin" {
		if mods&ui.Ctrl != 0 {
			out.WriteString("\u2303")
		}
		if mods&ui.Alt != 0 {
			out.WriteString("\u2325")
		}
		if mods&ui.Shift != 0 {
			out.WriteString("\u21e7")
		}
		if mods&ui.Super != 0 {
			out.WriteString("\u2318")
		}
	} else {
		if mods&ui.Ctrl != 0 {
			out.WriteString("Ctrl+")
		}
		if mods&ui.Alt != 0 {
			out.WriteString("Alt+")
		}
		if mods&ui.Shift != 0 {
			out.WriteString("Shift+")
		}
		if mods&ui.Super != 0 {
			out.WriteString("Super+")
		}
	}
	if key == "enter" || key == "Enter" {
		key = "\u21a9"
	}
	out.WriteString(key)
	return out.String()
}
