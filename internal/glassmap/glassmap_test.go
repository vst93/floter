package glassmap

import (
	"testing"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func lightTheme() *ui.Theme { return ui.LightTheme() }

func TestSpecForMapsEveryStop(t *testing.T) {
	// The four stored stops and the two axes the plugin can carry: the
	// material (the lens ladder) and the haze. The old build's blur and
	// saturation have no counterpart — see the package comment — so they
	// are deliberately absent here.
	cases := []struct {
		step        string
		wantEnabled bool
		wantStyle   glass.Style
		wantHaze    float32
	}{
		{"off", false, glass.Clear, 0},
		{"frosted", true, glass.Clear, 0.6},
		{"regular", true, glass.Regular, 0.5},
		{"liquid", true, glass.Regular, 0.2},
		// A legacy id migrates before the mapping.
		{"jelly", true, glass.Regular, 0.2},
		// An unknown id falls back to the shipped regular stop.
		{"ultra", true, glass.Regular, 0.5},
	}
	for _, tc := range cases {
		spec := SpecFor(tc.step, settings.DefaultMainOpacity)
		if spec.Enabled != tc.wantEnabled {
			t.Errorf("SpecFor(%q).Enabled = %v, want %v", tc.step, spec.Enabled, tc.wantEnabled)
		}
		if spec.Style != tc.wantStyle {
			t.Errorf("SpecFor(%q).Style = %v, want %v", tc.step, spec.Style, tc.wantStyle)
		}
		if spec.HazeAlpha != tc.wantHaze {
			t.Errorf("SpecFor(%q).HazeAlpha = %v, want %v", tc.step, spec.HazeAlpha, tc.wantHaze)
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

func TestOffCarriesNoMaterialAndNoHaze(t *testing.T) {
	spec := SpecFor("off", 47)
	if spec.Enabled {
		t.Error("the off stop must not enable a material")
	}
	if Material(spec, lightTheme()) != nil {
		t.Error("the off stop must not build a material")
	}
	if Haze(spec, lightTheme()) != (ui.Color{}) {
		t.Error("the off stop must not build a haze")
	}
}

func TestMaterialAndHazeUseTheTheme(t *testing.T) {
	theme := lightTheme()
	spec := SpecFor("regular", 80)
	m := Material(spec, theme)
	if m == nil {
		t.Fatal("regular built no material")
	}
	g, ok := m.(glass.Glass)
	if !ok {
		t.Fatalf("material is %T, want glass.Glass", m)
	}
	if want := theme.Background.Alpha(0.8); g.Tint != want {
		t.Errorf("tint = %v, want %v", g.Tint, want)
	}
	if got := Haze(spec, theme); got != theme.Background.Alpha(0.5) {
		t.Errorf("haze = %v", got)
	}
}
