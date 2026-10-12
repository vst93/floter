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
	// The tint alpha is the reference's `--glass-tint-alpha`: the frame's own
	// alpha (`min(--glass-solid-top, opacity)`) raised by the stop's dimming
	// layer, so the slider is the only knob that moves the frame's
	// transparency truth. The near-solid top is 0.98, not 1: a fully opaque
	// frame drops the material's sheen and rim.
	regular := func(opacity float64) float32 {
		return float32(1 - (1-0.5*(1-opacity))*(1-min(SolidTop, opacity)))
	}
	cases := []struct {
		opacity uint8
		want    float32
	}{
		{0, regular(float64(settings.DefaultMainOpacity) / 100)}, // unset -> default
		{1, regular(float64(settings.MinWindowOpacity) / 100)},   // clamped up
		{10, regular(0.10)},
		{47, regular(0.47)},
		{100, regular(1.0)},
		{255, regular(1.0)}, // clamped down
	}
	for _, tc := range cases {
		spec := SpecFor("regular", tc.opacity)
		if diff := spec.TintAlpha - tc.want; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("SpecFor(regular, %d).TintAlpha = %v, want %v", tc.opacity, spec.TintAlpha, tc.want)
		}
	}
	// The maxed slider is the near-solid top, not paint.
	if spec := SpecFor("regular", 100); spec.TintAlpha != SolidTop {
		t.Errorf("SpecFor(regular, 100).TintAlpha = %v, want the solid top %v", spec.TintAlpha, SolidTop)
	}
	// The shipped default is translucent, not opaque: the whole point of the
	// fill is that the desktop shows through it.
	if spec := SpecFor("regular", settings.DefaultMainOpacity); spec.TintAlpha >= 0.99 {
		t.Errorf("the default tint alpha = %v, want a translucent frame", spec.TintAlpha)
	}
	// A thinner stop carries a stronger dim, so it ends up *more* opaque than
	// the heavier one at the same slider — the readability floor.
	frosted := SpecFor("frosted", 47)
	liquid := SpecFor("liquid", 47)
	if frosted.TintAlpha <= liquid.TintAlpha {
		t.Errorf("frosted = %v, liquid = %v; a thinner stop must dim more", frosted.TintAlpha, liquid.TintAlpha)
	}
}

// The content pane and a floating pane ride the frame's tint, so the app's
// own background is what the transparency slider moves. The reference pinned
// the content pane to a fixed band (`--glass-content-alpha`), which the slider
// barely moved and which hid the frame tint it sat on — the report is
// 「底部还是非透明的，无论怎么调节应用透明度都不起作用」.
func TestContentAndFloatRideTheSlider(t *testing.T) {
	thin := SpecFor("regular", settings.MinWindowOpacity)
	thick := SpecFor("regular", settings.MaxWindowOpacity)

	if thin.ContentAlpha() >= thick.ContentAlpha() {
		t.Errorf("the content pane did not rise with the slider: %v vs %v", thin.ContentAlpha(), thick.ContentAlpha())
	}
	// It rides the frame tint exactly, so the low stop is as thin as the
	// frame (not the fixed 0.638 band that hid it).
	if thin.ContentAlpha() != thin.TintAlpha {
		t.Errorf("the content pane at the low stop = %v, want the frame tint %v", thin.ContentAlpha(), thin.TintAlpha)
	}
	// A floating pane is always denser than the frame it floats over, and
	// still follows the slider.
	if thin.FloatAlpha() <= thin.TintAlpha || thick.FloatAlpha() < thick.TintAlpha {
		t.Errorf("a float must be denser than its frame: %v/%v over %v/%v",
			thin.FloatAlpha(), thick.FloatAlpha(), thin.TintAlpha, thick.TintAlpha)
	}
	if thin.FloatAlpha() >= thick.FloatAlpha() {
		t.Errorf("the float did not rise with the slider: %v vs %v", thin.FloatAlpha(), thick.FloatAlpha())
	}
	// The off stop has no transparency at all: both panes are opaque.
	if off := SpecFor("off", 47); off.ContentAlpha() != 1 || off.FloatAlpha() != 1 {
		t.Errorf("off panes = %v/%v, want opaque", off.ContentAlpha(), off.FloatAlpha())
	}
}

func TestOffCarriesNoFillAndNoHaze(t *testing.T) {
	spec := SpecFor("off", 47)
	if spec.Enabled {
		t.Error("the off stop must not enable a material")
	}
	if _, enabled := Frame(spec, lightTheme()); enabled {
		t.Error("the off stop must not build a fill")
	}
}

func TestFrameUsesTheThemeBackground(t *testing.T) {
	theme := lightTheme()
	spec := SpecFor("regular", 80)
	fill, enabled := Frame(spec, theme)
	if !enabled {
		t.Fatal("regular built no fill")
	}
	if want := theme.Background.Alpha(spec.TintAlpha); fill != want {
		t.Errorf("fill = %v, want %v", fill, want)
	}
	if fill.A >= 255 {
		t.Errorf("fill = %v, want translucent", fill)
	}
}
