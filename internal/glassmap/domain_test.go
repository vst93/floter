package glassmap

import (
	"testing"

	"floter/internal/valuedomain"
)

// TestGlassStopsCoverTheStoredDomain keeps the effect table in step with the
// ids a settings file may hold.
//
// `stops` carries a stop's render parameters, not its id list, so the two are
// written apart on purpose — but they answer one question ("which stops exist")
// and today nothing checks that they agree. A fifth stop added to
// `valuedomain.GlassSteps` with no entry here would render through the map's
// zero value: no dim, no blur, no lens, which reads as `off` while the picker
// says otherwise.
//
// `off` is the one id with no stop: it is the position that removes the
// material, so it is handled before the table is ever consulted
// (shell.vibrancyForStop returns VibrancyNone for it).
func TestGlassStopsCoverTheStoredDomain(t *testing.T) {
	for _, step := range valuedomain.GlassSteps {
		if step == "off" {
			continue
		}
		if _, ok := stops[step]; !ok {
			t.Errorf("stop %q is a stored value with no effect entry", step)
		}
	}
	for step := range stops {
		if !valuedomain.Has(valuedomain.GlassSteps, step) {
			t.Errorf("effect entry for %q, which is not a stored stop", step)
		}
	}
}

// TestOffIsNotAnEffectStop is the other half of the rule above: `off` must not
// grow an entry, because a stop table entry would make it paint a material.
func TestOffIsNotAnEffectStop(t *testing.T) {
	if _, ok := stops["off"]; ok {
		t.Fatal("`off` has an effect entry; the stop that removes the material must not have one")
	}
}
