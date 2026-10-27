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

// NormalizeBinding rewrites an accelerator into the framework's spelling and
// reports whether it is a *whole* binding a user could have recorded, in the
// shape the old host's `normalize_shortcut` required before it would keep a
// stored shortcut (`src-tauri/src/commands/config.rs:1408`): no empty part
// (`"Ctrl++A"`), no modifier named twice (`"Ctrl+Ctrl+A"`), exactly one key,
// and at least one modifier (the old `normalize_custom_shortcuts` filter
// `key.contains('+')`, so a bare `"F5"` is not a shortcut).
//
// It is `Normalize`'s stricter sibling. `Normalize` is deliberately lenient —
// it is the bridge to the framework for values the app itself wrote and lets a
// stray empty part through — while a *stored* binding has to be one the
// recorder could have produced, or a hand-edited file could make a bare key
// global. The two share the same modifier/key tables and the same canonical
// `join`, so a value that passes both normalizes to one spelling.
func NormalizeBinding(accelerator string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(accelerator), "+")
	var modifiers []string
	key := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", false // an empty part (`"Ctrl++A"`, `"+"`)
		}
		if name, ok := modifierNames[strings.ToLower(part)]; ok {
			if containsFold(modifiers, name) {
				return "", false // the same modifier twice
			}
			modifiers = append(modifiers, name)
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
	if key == "" || len(modifiers) == 0 {
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

// modifierList spells the modifiers a binding holds, for `join` to order.
func modifierList(mods ui.Modifiers) []string {
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
	return modifiers
}

// FromKey builds an accelerator from a key event, for the recorder. It
// requires at least one modifier, with one exception: a bare function key is
// allowed, because the old build's `ShortcutRecorder` let a modifier-less
// `F\d{1,2}` through (`if (bare && !/^F\d{1,2}$/.test(next)) return;`). Every
// other bare key would take over ordinary typing. Shift alone is accepted, as
// the old build's `shortcutFromEvent` did — its `parts` array included
// `Shift` and the result passed `normalize_shortcut`.
func FromKey(mods ui.Modifiers, key ui.Key) (string, bool) {
	if mods&(ui.Super|ui.Ctrl|ui.Alt|ui.Shift) == 0 && !isFunctionKey(key) {
		return "", false
	}
	name, ok := KeyName(key)
	if !ok {
		return "", false
	}
	return join(modifierList(mods), name), true
}

// isFunctionKey reports whether a key is one of the function keys, the one
// family a recorder accepts without a modifier. The old build's regex allowed
// F1-F99; this build's key enum stops at F12, so F1-F12 is the whole family.
func isFunctionKey(key ui.Key) bool {
	return key >= ui.KeyF1 && key <= ui.KeyF12
}

// NormalizeResultShortcut normalizes a `select_result` binding to the family
// head: the modifiers the user chose and the digit `1`. The result family is a
// *family*, not one literal key — the recorder stores its modifiers and the
// first digit, and the rest of the digits reuse them (SelectResultDigit) — so a
// binding with no modifier at all is refused: a bare digit would launch a row
// on ordinary number entry. This is the old build's `normalizeResultShortcut`.
func NormalizeResultShortcut(accelerator string) (string, bool) {
	normalized, ok := Normalize(accelerator)
	if !ok {
		return "", false
	}
	mods, _, ok := Parse(normalized)
	if !ok || mods == 0 {
		return "", false
	}
	return join(modifierList(mods), "1"), true
}

// SameResultFamily reports whether `other` names a key in the `select_result`
// family whose head is `result`: the same modifiers, and a single digit as the
// key. `select_result` is a family — the app selects the n-th row, so the
// modifiers with any digit are keys it already answers for. A custom global
// binding that reuses them is the same collision the old host's
// `shortcut_conflicts` caught (`src-tauri/src/commands/config.rs:1456`), which
// a plain accelerator comparison misses: the family head is stored as one
// literal key (the digit `1`), not the whole family.
func SameResultFamily(result, other string) bool {
	resultMods, _, ok := Parse(result)
	if !ok || resultMods == 0 {
		return false
	}
	otherMods, otherKey, ok := Parse(other)
	if !ok {
		return false
	}
	return resultMods == otherMods && otherKey >= ui.Key0 && otherKey <= ui.Key9
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
	return displayFor(runtime.GOOS, accelerator)
}

// displayFor is Display with the platform named, so the whole table can be
// pinned from any host.
func displayFor(goos, accelerator string) string {
	normalized, ok := Normalize(accelerator)
	if !ok {
		return accelerator
	}
	// The spelling is read back from the normalized text, so the display
	// never has to map an enum back to a name: the parts are the modifiers
	// and the key. The *order* is not read back — `join` writes it meta-first
	// for the framework, while a chord is drawn in the reference's order (see
	// chordModifiers).
	parts := strings.Split(normalized, "+")
	return chordPrefix(goos, parts[:len(parts)-1]) + displayKey(parts[len(parts)-1])
}

// chordOrder is the order a chord's modifiers are drawn in, and it is the
// reference's: `formatShortcut` parses a binding into four flags and then
// writes them in one fixed order — Control, Alt, Shift, Meta
// (`main:src/shortcuts.ts:283-300`). That is Apple's own order (⌃⌥⇧⌘), and the
// old build used it on every platform. The written order of the stored string
// is deliberately irrelevant: `Cmd+Shift+P` and `Shift+Cmd+P` are one binding,
// so they must draw as one chord.
var chordOrder = []string{modCtrl, modAlt, modShift, modCmd}

// chordModifiers resolves an accelerator's modifier tokens into chordOrder,
// dropping duplicates and anything this vocabulary does not name.
//
// `CmdOrCtrl` is the portable spelling of "the modifier this platform binds":
// Command on macOS, which is the meta slot, and Control elsewhere, which is the
// ctrl slot — the same resolution `Parse` makes, but against the *named*
// platform rather than the host, so a table can be pinned from anywhere.
func chordModifiers(goos string, modifiers []string) []string {
	held := make(map[string]bool, len(chordOrder))
	for _, name := range modifiers {
		switch name {
		case "CmdOrCtrl":
			if goos == "darwin" {
				held[modCmd] = true
			} else {
				held[modCtrl] = true
			}
		default:
			held[name] = true
		}
	}
	ordered := make([]string, 0, len(chordOrder))
	for _, name := range chordOrder {
		if held[name] {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

// chordPrefix draws a chord's modifiers the way this platform spells them, with
// the key still to come: the glyphs a Mac user reads (⌃⌥⇧⌘) with no separator,
// and elsewhere the modifier words each followed by `+` (`Ctrl+Shift+`), the
// meta key named the way the platform's own keyboard names it — `Super` on
// Linux, `Win` on Windows — so a binding recorded as Super+K reads back as the
// key that was pressed, as the old build's `formatShortcut` did.
//
// This is the only producer of a chord's word order: `Display` and `Badge` both
// go through it, so the settings page and a launcher keycap cannot disagree.
func chordPrefix(goos string, modifiers []string) string {
	var out strings.Builder
	for _, name := range chordModifiers(goos, modifiers) {
		if goos == "darwin" {
			switch name {
			case modCtrl:
				out.WriteString("\u2303")
			case modAlt:
				out.WriteString("\u2325")
			case modShift:
				out.WriteString("\u21e7")
			case modCmd:
				out.WriteString("\u2318")
			}
			continue
		}
		switch name {
		case modCtrl:
			out.WriteString("Ctrl+")
		case modAlt:
			out.WriteString("Alt+")
		case modShift:
			out.WriteString("Shift+")
		case modCmd:
			out.WriteString(metaKeyWord(goos) + "+")
		}
	}
	return out.String()
}

// metaKeyWord is the meta modifier's own name on this platform: the old build
// pushed `IS_LINUX ? "Super" : "Win"` (`main:src/shortcuts.ts:299`).
func metaKeyWord(goos string) string {
	if goos == "linux" {
		return "Super"
	}
	return "Win"
}

// displayKey is a key's own spelling for display: the symbols the old build's
// `KEY_SYMBOLS` table drew (⌫ for Backspace, the four arrows), a single letter
// upper case, and the rest as the name the parser gave them.
func displayKey(key string) string {
	if symbol, ok := keySymbols[key]; ok {
		return symbol
	}
	if len([]rune(key)) == 1 {
		return strings.ToUpper(key)
	}
	return key
}

// keySymbols maps a normalized key token onto the glyph the old build drew for
// it (`main:src/shortcuts.ts:82-100`). Punctuation is already its own symbol in
// this vocabulary, so only the delete key and the arrows need a spelling.
var keySymbols = map[string]string{
	"Backspace": "\u232b",
	"Up":        "\u2191",
	"Down":      "\u2193",
	"Left":      "\u2190",
	"Right":     "\u2192",
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
// on macOS (⌃⌥⇧⌘) and the words elsewhere (Ctrl, Alt, Shift, and the meta key
// named the platform's way), with the key appended — no spaces, because a
// badge is a key, not a sentence. The `enter` spelling renders as ↩ on every
// platform, as the old build's result badge did.
func Badge(accelerator, key string) string {
	return badgeFor(runtime.GOOS, accelerator, key)
}

// badgeFor is Badge with the platform named, so the whole table can be pinned
// from any host. It answers the same question `displayFor` does — what is this
// binding's prefix — so it asks `chordPrefix` rather than re-deriving the order
// from `Parse`'s flags: two derivations is how the two faces came to disagree.
// Going through `Normalize` also keeps the modifiers of a key `Parse` has no
// enum for (`Ctrl+PrintScreen`), which the old build's `formatShortcut` drew.
func badgeFor(goos, accelerator, key string) string {
	normalized, ok := Normalize(accelerator)
	if !ok {
		return key
	}
	parts := strings.Split(normalized, "+")
	if key == "enter" || key == "Enter" {
		key = "\u21a9"
	}
	return chordPrefix(goos, parts[:len(parts)-1]) + displayKey(key)
}
