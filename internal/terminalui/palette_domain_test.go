package terminalui

import (
	"testing"

	"floter/internal/valuedomain"
)

// TestPalettesCoverTheStoredDomain keeps the colour table in step with the ids a
// settings file may hold.
//
// `palettes` carries each override's colours; the id list is
// `valuedomain.Palettes`. They are written apart on purpose — the colours
// belong to the renderer — but an id in one and not the other is silent today:
// `themeFor` treats an unknown palette as `inherit`, so a palette added to the
// picker without a colour spec would store fine and paint the window's own
// colours, which is not the palette the user chose.
//
// `inherit` is the one id with no entry: it means "follow the app", which is
// exactly what the missing-entry path does.
func TestPalettesCoverTheStoredDomain(t *testing.T) {
	for _, name := range valuedomain.Palettes {
		if name == "inherit" {
			continue
		}
		if _, ok := palettes[name]; !ok {
			t.Errorf("palette %q is a stored value with no colour spec", name)
		}
	}
	for name := range palettes {
		if !valuedomain.Has(valuedomain.Palettes, name) {
			t.Errorf("colour spec for %q, which is not a stored palette", name)
		}
	}
}

// TestInheritIsNotAColourSpec is the other half: `inherit` must not grow a spec,
// because the missing-entry path is what makes it follow the window's
// appearance in both light and dark.
func TestInheritIsNotAColourSpec(t *testing.T) {
	if _, ok := palettes["inherit"]; ok {
		t.Fatal("`inherit` has a colour spec; it follows the window instead")
	}
}
