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
		step      string
		wantGlass bool
		wantStyle glass.Style
		wantHaze  float32
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
		if spec.Glass != tc.wantGlass {
			t.Errorf("SpecFor(%q).Glass = %v, want %v", tc.step, spec.Glass, tc.wantGlass)
		}
		// Every stop paints the frame's own tint: even off is translucent, so
		// turning the glass off never turns the window opaque.
		if !spec.Translucent {
			t.Errorf("SpecFor(%q).Translucent = false, want the frame tint on every stop", tc.step)
		}
		if spec.Style != tc.wantStyle {
			t.Errorf("SpecFor(%q).Style = %v, want %v", tc.step, spec.Style, tc.wantStyle)
		}
		if spec.HazeAlpha != tc.wantHaze {
			t.Errorf("SpecFor(%q).HazeAlpha = %v, want %v", tc.step, spec.HazeAlpha, tc.wantHaze)
		}
	}
}

// The stop ladder carries the old build's whole effect table, not only the
// haze: blur, saturation and the control lens. The haze alone left `frosted`
// and `regular` 0.03 of alpha apart and the user read the three stops as one
// (「玻璃效果…看起来好像没什么区别」); the lens is the rung the scene can
// actually paint, so every stop must carry its own.
func TestSpecForCarriesTheEffectTable(t *testing.T) {
	type row struct {
		step           string
		blur, saturate float32
		lens           float32
	}
	want := []row{
		{"frosted", 10, 130, 0.5},
		{"regular", 22, 170, 1},
		{"liquid", 28, 200, 1.6},
	}
	for _, w := range want {
		spec := SpecFor(w.step, settings.DefaultMainOpacity)
		if spec.Blur != w.blur {
			t.Errorf("SpecFor(%q).Blur = %v, want %v", w.step, spec.Blur, w.blur)
		}
		if spec.Saturate != w.saturate {
			t.Errorf("SpecFor(%q).Saturate = %v, want %v", w.step, spec.Saturate, w.saturate)
		}
		if spec.LensScale != w.lens {
			t.Errorf("SpecFor(%q).LensScale = %v, want %v", w.step, spec.LensScale, w.lens)
		}
	}
	// Every rung rises from the thinnest stop to the heaviest, so a stop
	// switch is a visible change and not a re-ordering.
	frosted := SpecFor("frosted", settings.DefaultMainOpacity)
	regular := SpecFor("regular", settings.DefaultMainOpacity)
	liquid := SpecFor("liquid", settings.DefaultMainOpacity)
	if !(frosted.Blur < regular.Blur && regular.Blur < liquid.Blur) {
		t.Errorf("blur ladder = %v/%v/%v, want it rising", frosted.Blur, regular.Blur, liquid.Blur)
	}
	if !(frosted.Saturate < regular.Saturate && regular.Saturate < liquid.Saturate) {
		t.Errorf("saturate ladder = %v/%v/%v, want it rising", frosted.Saturate, regular.Saturate, liquid.Saturate)
	}
	if !(frosted.LensScale < regular.LensScale && regular.LensScale < liquid.LensScale) {
		t.Errorf("lens ladder = %v/%v/%v, want it rising", frosted.LensScale, regular.LensScale, liquid.LensScale)
	}
	// The off stop carries no effect at all.
	if off := SpecFor("off", settings.DefaultMainOpacity); off.Blur != 0 || off.Saturate != 0 || off.LensScale != 0 {
		t.Errorf("off carries an effect: blur %v saturate %v lens %v", off.Blur, off.Saturate, off.LensScale)
	}
}

func TestSpecForSpendsMainOpacityOnTheTintAlpha(t *testing.T) {
	// The tint alpha is the frame's own alpha on **every** stop — the old
	// build's `--glass-frame-alpha = min(--glass-solid-top, opacity)` — so the
	// stop never moves the frame's transparency. The previous rewrite raised it
	// by the stop's dimming layer, which made a glass-on frame denser than the
	// `off` one and the slider's low end inert (「开启时又没有透明了」).
	frame := func(opacity float64) float32 { return min(SolidTop, float32(opacity)) }
	cases := []struct {
		opacity uint8
		want    float32
	}{
		{0, frame(float64(settings.DefaultMainOpacity) / 100)}, // unset -> default
		{1, frame(float64(settings.MinWindowOpacity) / 100)},   // clamped up
		{10, frame(0.10)},
		{47, frame(0.47)},
		{100, frame(1.0)},
		{255, frame(1.0)}, // clamped down
	}
	for _, tc := range cases {
		for _, step := range []string{"off", "frosted", "regular", "liquid"} {
			spec := SpecFor(step, tc.opacity)
			if diff := spec.TintAlpha - tc.want; diff > 1e-6 || diff < -1e-6 {
				t.Errorf("SpecFor(%s, %d).TintAlpha = %v, want %v", step, tc.opacity, spec.TintAlpha, tc.want)
			}
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
	// The stop never raises the fill: `off`, `frosted`, `regular` and `liquid`
	// all sit on the slider's own alpha, so turning the glass on cannot make
	// the window denser (「开玻璃时透明底失效」).
	for _, step := range []string{"frosted", "regular", "liquid"} {
		if got, off := SpecFor(step, 47).TintAlpha, SpecFor("off", 47).TintAlpha; got != off {
			t.Errorf("%s tint = %v, want the off frame's own %v", step, got, off)
		}
	}
	// The haze is still carried for the material choice, even though it no
	// longer moves the fill: a thinner stop asks for a stronger veil.
	frosted := SpecFor("frosted", 47)
	liquid := SpecFor("liquid", 47)
	if frosted.HazeAlpha <= liquid.HazeAlpha {
		t.Errorf("frosted haze = %v, liquid haze = %v; a thinner stop must ask for more", frosted.HazeAlpha, liquid.HazeAlpha)
	}
}

// The content pane rides the frame's tint as a single sheet: the pane is only
// the recess it needs to composite to `frame + contentStep`, not a second full
// tint. The reference's fixed band pinned the visible pane to 0.62-0.80 and
// the user read it as "the bottom is opaque and the transparency slider does
// nothing"; a pane painted at the frame's own alpha (the previous rewrite)
// stacked to `1-(1-t)^2` and halved the slider's amplitude instead.
func TestContentAndFloatRideTheSlider(t *testing.T) {
	thin := SpecFor("regular", settings.MinWindowOpacity)
	thick := SpecFor("regular", settings.MaxWindowOpacity)

	if thin.ContentAlpha() >= thick.ContentAlpha() {
		t.Errorf("the content pane did not rise with the slider: %v vs %v", thin.ContentAlpha(), thick.ContentAlpha())
	}
	// The pane's own alpha is the *over-frame* fill, so the visible recess is
	// one sheet the slider moves, not a second full tint.
	visible := func(s Spec) float32 { return 1 - (1-s.TintAlpha)*(1-s.ContentAlpha()) }
	if got, want := visible(thin), thin.TintAlpha+contentStep; diff(got, want) {
		t.Errorf("the low stop's visible pane = %v, want the frame + step %v", got, want)
	}
	if got, want := visible(thick), float32(1); diff(got, want) {
		t.Errorf("the high stop's visible pane = %v, want opaque %v", got, want)
	}
	// The bottom end is close to the frame's own alpha, not the near-opaque
	// double sheet the old code drew.
	if a := visible(thin); a >= 0.7 {
		t.Errorf("the low stop's visible pane = %v, want a translucent bottom", a)
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
	// The off stop rides the slider too: turning the glass off drops the
	// effect but keeps the frame's own transparency, so the panes are
	// translucent - never the opaque stand-in the old `off` branch drew.
	if off := SpecFor("off", 47); off.ContentAlpha() >= 1 || off.FloatAlpha() >= 1 {
		t.Errorf("off panes = %v/%v, want the frame's own translucency", off.ContentAlpha(), off.FloatAlpha())
	}
	if off := SpecFor("off", 47); off.Glass {
		t.Error("the off stop must not carry the glass material")
	}
}

// diff reports whether two alphas differ beyond the float tolerance the
// formulas accumulate.
func diff(a, b float32) bool { return a-b > 1e-5 || b-a > 1e-5 }

func TestOffKeepsTheFrameTintButNoGlass(t *testing.T) {
	spec := SpecFor("off", 47)
	if spec.Glass {
		t.Error("the off stop must not carry the glass material")
	}
	// No effect rungs at all.
	if spec.HazeAlpha != 0 || spec.Blur != 0 || spec.Saturate != 0 || spec.LensScale != 0 {
		t.Errorf("off carries an effect: haze %v blur %v saturate %v lens %v",
			spec.HazeAlpha, spec.Blur, spec.Saturate, spec.LensScale)
	}
	// But the frame's own alpha still paints: the window stays translucent.
	if !spec.Translucent {
		t.Error("the off stop must keep the frame's transparency")
	}
	if want := min(SolidTop, float32(47)/100); spec.TintAlpha != want {
		t.Errorf("off tint = %v, want the frame's own alpha %v", spec.TintAlpha, want)
	}
	fill, translucent := Frame(spec, lightTheme())
	if !translucent {
		t.Fatal("the off stop built no fill")
	}
	if fill.A >= 255 {
		t.Errorf("off fill = %v, want translucent", fill)
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

// A desktop that asked for more contrast gets the material dialled back, not
// only the borders — base.css's `@media (prefers-contrast: more)` block. Three
// tokens move and everything else stays: the frame's floor comes up to 0.86 on
// *every* stop (including `off`, so a thin frame cannot stay see-through), the
// haze is dropped, and the lens is damped to 0.6×. The blur is deliberately
// left alone here — the old build left it to the window's material.
func TestContrastRaisesTheFloorDropsHazeAndDampsTheLens(t *testing.T) {
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		base := SpecFor(step, settings.MinWindowOpacity)
		got := SpecForPreferences(step, settings.MinWindowOpacity, true)
		if !got.Translucent {
			t.Errorf("%s: contrast turned the frame opaque", step)
		}
		if got.TintAlpha < ContrastFrameFloor {
			t.Errorf("%s: contrast frame alpha = %v, want >= %v", step, got.TintAlpha, ContrastFrameFloor)
		}
		if got.TintAlpha > SolidTop {
			t.Errorf("%s: contrast frame alpha = %v, want <= the solid top %v", step, got.TintAlpha, SolidTop)
		}
		if got.HazeAlpha != 0 {
			t.Errorf("%s: contrast haze = %v, want 0", step, got.HazeAlpha)
		}
		if want := base.LensScale * ContrastLensScale; got.LensScale != want {
			t.Errorf("%s: contrast lens = %v, want %v", step, got.LensScale, want)
		}
		if got.Blur != base.Blur || got.Saturate != base.Saturate || got.Style != base.Style {
			t.Errorf("%s: contrast moved the material: %+v vs %+v", step, got, base)
		}
		if got.Glass != base.Glass || got.Glass && got.HazeAlpha != 0 {
			t.Errorf("%s: contrast changed the stop's material choice", step)
		}
	}
}

// The floor is a *floor*, not a replacement: a frame the slider has already
// pushed past it keeps its own (capped) alpha, so the transparency slider's
// top end is not pulled *down* by the preference the way a plain assignment
// would.
func TestContrastFloorNeverLowersATransparentFrame(t *testing.T) {
	opaque := SpecForPreferences("regular", settings.MaxWindowOpacity, true)
	if want := min(SolidTop, float32(settings.MaxWindowOpacity)/100); opaque.TintAlpha != want {
		t.Errorf("alpha at %d%% = %v, want %v", settings.MaxWindowOpacity, opaque.TintAlpha, want)
	}
	// And the default preference is the plain spec, byte for byte: every
	// existing glass test has to keep passing untouched.
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		if got, want := SpecForPreferences(step, settings.DefaultMainOpacity, false), SpecFor(step, settings.DefaultMainOpacity); got != want {
			t.Errorf("%s: no preference = %+v, want %+v", step, got, want)
		}
	}
}
