package valuedomain

import "testing"

// TestEveryScaleStepHasAFactor keeps the id list and the factor table in step.
//
// They are two shapes of one answer: `Scales` says which ids a settings file may
// hold, `ScaleFactors` says how big each one is. A step in the list without a
// factor scales the whole interface to zero (`UIScaleFactor` would return the
// map's zero value), and a factor without a step is a value no file can select.
func TestEveryScaleStepHasAFactor(t *testing.T) {
	for _, step := range Scales {
		factor, ok := ScaleFactors[step]
		if !ok {
			t.Errorf("step %q has no factor", step)
			continue
		}
		if factor <= 0 {
			t.Errorf("step %q has factor %v, want > 0", step, factor)
		}
	}
	for step := range ScaleFactors {
		if !Has(Scales, step) {
			t.Errorf("factor for %q, which is not a stored step", step)
		}
	}
}

// TestHasIsAnExactMatch pins the one rule every loader shares: these ids are
// stored strings, so case and whitespace are not forgiven. `NormalizeGlassStep`
// trims and lowercases on purpose; nothing else does.
func TestHasIsAnExactMatch(t *testing.T) {
	for _, test := range []struct {
		ids  []string
		id   string
		want bool
	}{
		{Themes, "dark", true},
		{Themes, "Dark", false},
		{Themes, " dark", false},
		{Themes, "", false},
		{Palettes, "inherit", true},
		{Palettes, "inherit ", false},
	} {
		if got := Has(test.ids, test.id); got != test.want {
			t.Errorf("Has(%v, %q) = %v, want %v", test.ids, test.id, got, test.want)
		}
	}
}

// TestNormalizeAppIcon pins the app-icon rule: the icon is an `Extra` key, so
// nothing in the settings loader normalizes it, and both the tray and the
// settings picker ask here instead.
func TestNormalizeAppIcon(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"dark", "dark"},
		{"light", "light"},
		{"", DefaultAppIcon},
		{"Light", DefaultAppIcon},
		{"bogus", DefaultAppIcon},
	} {
		if got := NormalizeAppIcon(test.in); got != test.want {
			t.Errorf("NormalizeAppIcon(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// TestDomainsHaveNoDuplicateIDs is the cheap invariant the lists themselves
// must hold: a duplicate would give one id two rows in the picker and make
// `Has` a coin flip between two positions.
func TestDomainsHaveNoDuplicateIDs(t *testing.T) {
	domains := map[string][]string{
		"app icons":     AppIcons,
		"themes":        Themes,
		"glass steps":   GlassSteps,
		"scales":        Scales,
		"cursor shapes": CursorShapes,
		"paddings":      Paddings,
		"palettes":      Palettes,
		"bold modes":    BoldModes,
	}
	for name, ids := range domains {
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				t.Errorf("%s: %q listed twice", name, id)
			}
			seen[id] = true
			if id == "" {
				t.Errorf("%s: empty id", name)
			}
		}
	}
}
