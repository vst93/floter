package apps

import (
	"os"
	"path/filepath"
	"testing"
)

// A localized name is UTF-16 in the files Xcode writes for Chinese and Japanese
// names, and a name may hold a character outside the BMP (an emoji, an
// extension-B ideograph). Decoding one code unit at a time turned those into two
// replacement characters, so the row was titled `�� Games`.

// utf16File writes text as UTF-16 with a byte-order mark.
func utf16File(t *testing.T, dir, name, text string, bigEndian bool) string {
	t.Helper()
	path := filepath.Join(dir, name)
	units := utf16Units(text)
	data := []byte{0xff, 0xfe}
	if bigEndian {
		data = []byte{0xfe, 0xff}
	}
	for _, unit := range units {
		if bigEndian {
			data = append(data, byte(unit>>8), byte(unit))
		} else {
			data = append(data, byte(unit), byte(unit>>8))
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// utf16Units spells a string in UTF-16 code units, surrogate pairs included.
func utf16Units(text string) []uint16 {
	var units []uint16
	for _, r := range text {
		if r < 0x10000 {
			units = append(units, uint16(r))
			continue
		}
		r -= 0x10000
		units = append(units, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
	}
	return units
}

func TestALocalizedNameOutsideTheBMPReadsAsOneCharacter(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name      string
		bigEndian bool
	}{
		{"le.lproj", false},
		{"be.lproj", true},
	} {
		path := utf16File(t, dir, tc.name,
			`"CFBundleDisplayName" = "🎮 Games";`, tc.bigEndian)
		if got := readStringsFile(path); got != "🎮 Games" {
			t.Errorf("%s: name = %q, want %q", tc.name, got, "🎮 Games")
		}
	}
}
