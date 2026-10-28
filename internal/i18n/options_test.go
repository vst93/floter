package i18n

import (
	"testing"

	"floter/internal/valuedomain"
)

// TestPickerOptionsMatchTheStoredDomains is the pin `options.go` states: the
// picker's rows are `internal/valuedomain`'s ids in that order, and every row
// carries a label of its own.
//
// The failure this catches is the one that made P11-w worth doing. The ids used
// to be written out here — fourteen `[]Option` literals across the two Copy
// values — and the settings loader had its own copies. Adding an option meant
// editing both, and forgetting one was silent: a row the loader throws away, or
// a stored id the picker never offers. A missing label is the same shape one
// level down: `optionsFor` leaves it empty rather than showing the id, and an
// empty segment is what this test refuses.
func TestPickerOptionsMatchTheStoredDomains(t *testing.T) {
	domains := []struct {
		name    string
		ids     []string
		options func(Settings) []Option
	}{
		{"app icons", valuedomain.AppIcons, func(s Settings) []Option { return s.AppIcons }},
		{"themes", valuedomain.Themes, func(s Settings) []Option { return s.Themes }},
		{"glass steps", valuedomain.GlassSteps, func(s Settings) []Option { return s.GlassSteps }},
		{"scales", valuedomain.Scales, func(s Settings) []Option { return s.Scales }},
		{"cursor shapes", valuedomain.CursorShapes, func(s Settings) []Option { return s.CursorShapes }},
		{"paddings", valuedomain.Paddings, func(s Settings) []Option { return s.Paddings }},
		{"palettes", valuedomain.Palettes, func(s Settings) []Option { return s.Palettes }},
	}

	copies := map[string]Settings{
		"en": For("en").Settings,
		"zh": For("zh").Settings,
	}

	for language, copy := range copies {
		for _, domain := range domains {
			options := domain.options(copy)
			if len(options) != len(domain.ids) {
				t.Errorf("%s/%s: %d rows for %d ids", language, domain.name, len(options), len(domain.ids))
				continue
			}
			for i, id := range domain.ids {
				if options[i].ID != id {
					t.Errorf("%s/%s: row %d is %q, want %q", language, domain.name, i, options[i].ID, id)
				}
				if options[i].Label == "" {
					t.Errorf("%s/%s: id %q has no label", language, domain.name, id)
				}
				if options[i].Label == id {
					t.Errorf("%s/%s: id %q is its own label, so the map has no entry for it", language, domain.name, id)
				}
			}
		}
	}
}

// TestPickerOrderIsTheShippedOrder pins the one thing about a value domain that
// a user can see: the order of the rows. Membership is a set, so the order is
// pure presentation — but it is presentation that a refactor can silently
// change, and no existing pin looked at it. `internal/settingsui/view_test.go`
// reads its expected first/last segment out of the same table it is checking
// (`copy.Themes[0].Label`), so it holds whatever order the table happens to
// have.
//
// The expected rows below are what this build shows, read off the picker before
// the refactor. A deliberate reorder edits this list; an accidental one fails.
func TestPickerOrderIsTheShippedOrder(t *testing.T) {
	for _, test := range []struct {
		name string
		got  []Option
		want []string
	}{
		{"app icons", For("en").Settings.AppIcons, []string{"Dark", "Light"}},
		{"themes", For("en").Settings.Themes, []string{"Auto", "Light", "Dark"}},
		{"glass steps", For("en").Settings.GlassSteps, []string{"Off", "Frosted", "Liquid", "Liquid Max"}},
		{"scales", For("en").Settings.Scales, []string{"Tiny", "Small", "Standard", "Large"}},
		{"cursor shapes", For("en").Settings.CursorShapes, []string{"Beam", "Block", "Underline"}},
		{"paddings", For("en").Settings.Paddings, []string{"Compact", "Regular", "Relaxed"}},
		{"palettes", For("en").Settings.Palettes, []string{
			"Follow app", "High contrast", "Paper", "Ink", "Fog", "Forest", "Dusk", "Mist", "Amber",
		}},
	} {
		if len(test.got) != len(test.want) {
			t.Errorf("%s: %d rows, want %d", test.name, len(test.got), len(test.want))
			continue
		}
		for i := range test.want {
			if test.got[i].Label != test.want[i] {
				t.Errorf("%s: row %d is %q, want %q", test.name, i, test.got[i].Label, test.want[i])
			}
		}
	}
}

// TestLanguageLabelsAreEndonyms pins the one picker table that is *not* built
// from `valuedomain`: a language's label is its own endonym, identical in both
// copies, so `language.go` builds it from one table (P11-v). If it ever moved
// into the per-language label maps this test would still pass; what it refuses
// is the opposite move — an endonym that starts differing per copy.
func TestLanguageLabelsAreEndonyms(t *testing.T) {
	en := For("en").Settings.Languages
	zh := For("zh").Settings.Languages
	if len(en) != len(zh) {
		t.Fatalf("language tables differ: %d vs %d rows", len(en), len(zh))
	}
	for i := range en {
		if en[i] != zh[i] {
			t.Errorf("row %d differs between copies: %+v vs %+v", i, en[i], zh[i])
		}
		if en[i].Label == "" || en[i].Label == en[i].ID {
			t.Errorf("row %d has no endonym: %+v", i, en[i])
		}
	}
}
