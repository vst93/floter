package glassmap

import (
	"testing"

	"github.com/egoist/mygo/plugins/glass"

	"floter/internal/settings"
)

func TestSpecForMapsEveryStop(t *testing.T) {
	// The four stored stops. The plugin exposes only Regular and Clear, so
	// liquid collapses onto Regular; the mapping is pinned here so a plugin
	// intensity parameter (P2) has to update this table on purpose.
	cases := []struct {
		step        string
		wantEnabled bool
		wantStyle   glass.Style
	}{
		{"off", false, glass.Clear},
		{"frosted", true, glass.Clear},
		{"regular", true, glass.Regular},
		{"liquid", true, glass.Regular},
		// A legacy id migrates before the mapping.
		{"jelly", true, glass.Regular},
		// An unknown id falls back to the shipped regular stop.
		{"ultra", true, glass.Regular},
	}
	for _, tc := range cases {
		spec := SpecFor(tc.step, settings.DefaultMainOpacity)
		if spec.Enabled != tc.wantEnabled {
			t.Errorf("SpecFor(%q).Enabled = %v, want %v", tc.step, spec.Enabled, tc.wantEnabled)
		}
		if spec.Style != tc.wantStyle {
			t.Errorf("SpecFor(%q).Style = %v, want %v", tc.step, spec.Style, tc.wantStyle)
		}
	}
}

func TestSpecForSpendsMainOpacityOnTheTintAlpha(t *testing.T) {
	cases := []struct {
		opacity uint8
		want    float32
	}{
		{0, float32(settings.DefaultMainOpacity) / 100}, // unset -> default
		{1, float32(settings.MinWindowOpacity) / 100},   // clamped up
		{10, 0.10},
		{47, 0.47},
		{100, 1.0},
		{255, 1.0}, // clamped down
	}
	for _, tc := range cases {
		spec := SpecFor("regular", tc.opacity)
		if diff := spec.TintAlpha - tc.want; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("SpecFor(regular, %d).TintAlpha = %v, want %v", tc.opacity, spec.TintAlpha, tc.want)
		}
	}
}

func TestOffCarriesNoMaterial(t *testing.T) {
	if spec := SpecFor("off", 47); spec.Enabled {
		t.Error("the off stop must not enable a material")
	}
}
