package shortcuts

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Parse turns a normalized accelerator back into the modifiers and key it
// binds, so a view that has to compare a binding with a key event can do it
// without a second table. It reports false for an accelerator this build
// cannot parse.
func Parse(accelerator string) (ui.Modifiers, ui.Key, bool) {
	normalized, ok := Normalize(accelerator)
	if !ok {
		return 0, 0, false
	}
	var mods ui.Modifiers
	key := ui.Key(0)
	found := false
	for _, part := range strings.Split(normalized, "+") {
		switch part {
		case "CmdOrCtrl":
			// The portable spelling is the modifier the platform binds.
			if runtime.GOOS == "darwin" {
				mods |= ui.Super
			} else {
				mods |= ui.Ctrl
			}
		case modCmd:
			mods |= ui.Super
		case modCtrl:
			mods |= ui.Ctrl
		case modAlt:
			mods |= ui.Alt
		case modShift:
			mods |= ui.Shift
		default:
			parsed, ok := parseKey(part)
			if !ok || found {
				return 0, 0, false
			}
			key, found = parsed, true
		}
	}
	if !found {
		return 0, 0, false
	}
	return mods, key, true
}

// parseKey is the reverse of KeyName: the accelerator spelling of a key back
// to the key.
func parseKey(name string) (ui.Key, bool) {
	if len([]rune(name)) == 1 {
		r := []rune(name)[0]
		switch {
		case r >= 'A' && r <= 'Z':
			return ui.KeyA + ui.Key(r-'A'), true
		case r >= '0' && r <= '9':
			return ui.Key0 + ui.Key(r-'0'), true
		}
	}
	if len(name) >= 2 && name[0] == 'F' {
		if number, ok := digits(name[1:]); ok && number >= 1 && number <= 12 {
			return ui.KeyF1 + ui.Key(number-1), true
		}
	}
	switch name {
	case "Space":
		return ui.KeySpace, true
	case "Enter":
		return ui.KeyEnter, true
	case "Esc":
		return ui.KeyEscape, true
	case "Backspace":
		return ui.KeyBackspace, true
	case "Delete":
		return ui.KeyDelete, true
	case "Insert":
		return ui.KeyInsert, true
	case "Tab":
		return ui.KeyTab, true
	case "Up":
		return ui.KeyUp, true
	case "Down":
		return ui.KeyDown, true
	case "Left":
		return ui.KeyLeft, true
	case "Right":
		return ui.KeyRight, true
	case "Home":
		return ui.KeyHome, true
	case "End":
		return ui.KeyEnd, true
	case "PageUp":
		return ui.KeyPageUp, true
	case "PageDown":
		return ui.KeyPageDown, true
	case "-":
		return ui.KeyMinus, true
	case "=":
		return ui.KeyEqual, true
	case ",":
		return ui.KeyComma, true
	case ".":
		return ui.KeyPeriod, true
	case "/":
		return ui.KeySlash, true
	case ";":
		return ui.KeySemicolon, true
	case "'":
		return ui.KeyQuote, true
	case "[":
		return ui.KeyBracketLeft, true
	case "]":
		return ui.KeyBracketRight, true
	case "\\":
		return ui.KeyBackslash, true
	case "`":
		return ui.KeyBackquote, true
	}
	return 0, false
}

// Match reports whether a key event is the one an accelerator binds.
func Match(accelerator string, mods ui.Modifiers, key ui.Key) bool {
	wanted, wantedKey, ok := Parse(accelerator)
	if !ok || wantedKey != key {
		return false
	}
	// Shift is part of a letter's spelling (Cmd+Shift+P) but a bare letter is
	// typed with Shift too, so a binding without Shift accepts either case.
	if wanted&ui.Shift == 0 {
		mods &^= ui.Shift
	}
	return mods == wanted
}
