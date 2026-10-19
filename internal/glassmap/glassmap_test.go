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

// The slider is the frame's alpha on **every** stop: the old build's
// `--glass-frame-alpha = min(--glass-solid-top, opacity)`. The previous rewrite
// raised it by the stop's dimming layer, which made a glass-on frame denser
// than the `off` one and the slider's low end inert (「开启时又没有透明了」).
//
// The fill the app *paints* is not the slider any more: it is the slider less
// the veil the window's own material already contributes (Spec.TintAlpha), so
// the same slider position shows the same solidity on every stop — the user's
// complaint that 47% read three different ways on three materials. Composite is
// the answer, TintAlpha the paint.
func TestSpecForSpendsMainOpacityOnTheComposite(t *testing.T) {
	want := func(opacity float64) float32 { return min(SolidTop, float32(opacity)) }
	cases := []struct {
		opacity uint8
		want    float32
	}{
		{0, want(float64(settings.DefaultMainOpacity) / 100)}, // unset -> default
		{1, want(float64(settings.MinWindowOpacity) / 100)},   // clamped up
		{10, want(0.10)},
		{47, want(0.47)},
		{100, want(1.0)},
		{255, want(1.0)}, // clamped down
	}
	for _, tc := range cases {
		for _, step := range []string{"frosted", "regular", "liquid"} {
			spec := SpecFor(step, tc.opacity)
			if diff := spec.Composite() - tc.want; diff > 1e-6 || diff < -1e-6 {
				t.Errorf("SpecFor(%s, %d).Composite() = %v, want the slider %v", step, tc.opacity, spec.Composite(), tc.want)
			}
			// And the painted fill is no denser than the slider: the material
			// carries its own weight, the tint never adds to it.
			if spec.TintAlpha > tc.want+1e-6 {
				t.Errorf("SpecFor(%s, %d).TintAlpha = %v, want at most the slider %v", step, tc.opacity, spec.TintAlpha, tc.want)
			}
		}
	}
	// The maxed slider is the near-solid top, not paint.
	if spec := SpecFor("regular", 100); spec.Composite() != SolidTop || spec.TintAlpha != SolidTop {
		t.Errorf("SpecFor(regular, 100) = fill %v composite %v, want the solid top %v on both", spec.TintAlpha, spec.Composite(), SolidTop)
	}
	// The shipped default is translucent, not opaque: the whole point of the
	// fill is that the desktop shows through it.
	if spec := SpecFor("regular", settings.DefaultMainOpacity); spec.Composite() >= 0.99 {
		t.Errorf("the default composite = %v, want a translucent frame", spec.Composite())
	}
	// The haze is still carried for the material choice, even though it no
	// longer moves the fill: a thinner stop asks for a stronger veil.
	frosted := SpecFor("frosted", 47)
	liquid := SpecFor("liquid", 47)
	if frosted.HazeAlpha <= liquid.HazeAlpha {
		t.Errorf("frosted haze = %v, liquid haze = %v; a thinner stop must ask for more", frosted.HazeAlpha, liquid.HazeAlpha)
	}
}

// The `off` stop has no material, so its tint *is* what the user sees — which
// is why it carries a readability floor: with the blur gone, the slider's
// bottom end would put the text on the wallpaper. The floor is a floor, not a
// replacement: a slider already past it keeps its own value, and the top is
// still the solid top.
func TestTheOffStopKeepsAReadabilityFloor(t *testing.T) {
	if got := SpecFor("off", settings.MinWindowOpacity).TintAlpha; got != OffFrameFloor {
		t.Errorf("off at the slider's bottom = %v, want the floor %v", got, OffFrameFloor)
	}
	if got := SpecFor("off", 60).TintAlpha; got != 0.6 {
		t.Errorf("off at 60%% = %v, want the slider's own 0.6", got)
	}
	if got := SpecFor("off", 100).TintAlpha; got != SolidTop {
		t.Errorf("off at 100%% = %v, want the solid top %v", got, SolidTop)
	}
	// The floor is the `off` stop's alone: a glass stop is readable because its
	// backdrop is blurred, so it follows the slider all the way down.
	for _, step := range []string{"frosted", "regular", "liquid"} {
		if got := SpecFor(step, settings.MinWindowOpacity).Composite(); diff(got, 0.10) {
			t.Errorf("%s at the slider's bottom = %v, want the slider's own 0.10", step, got)
		}
	}
	// The floor never moves the composite off the slider's own band.
	if got := SpecFor("off", settings.MinWindowOpacity).Composite(); got < OffFrameFloor {
		t.Errorf("off composite = %v, want at least the floor %v", got, OffFrameFloor)
	}
}

// MaterialVeil is the table the compensation reads: macOS has a material per
// stop (shell.vibrancyForStop) and the three weights differ a lot, which is
// exactly why the same slider read three ways. Windows and Linux carry 0 — one
// backdrop for every stop, or none at all — so their tint needs no
// compensation, and `off` carries none anywhere (it is the stop that removes
// the material).
func TestMaterialVeilIsThePlatformsOwnWeight(t *testing.T) {
	cases := []struct {
		goos, step string
		want       float32
	}{
		{"darwin", "off", 0},
		{"darwin", "frosted", 0.12},
		{"darwin", "regular", 0.22},
		{"darwin", "liquid", 0.34},
		// The ladder is ordered: a denser material is a heavier veil, or the
		// compensation would invert the stops' solidity.
		{"windows", "off", 0},
		{"windows", "frosted", 0},
		{"windows", "regular", 0},
		{"windows", "liquid", 0},
		{"linux", "off", 0},
		{"linux", "frosted", 0},
		{"linux", "regular", 0},
		{"linux", "liquid", 0},
	}
	for _, tc := range cases {
		if got := MaterialVeil(tc.goos, tc.step); got != tc.want {
			t.Errorf("MaterialVeil(%q, %q) = %v, want %v", tc.goos, tc.step, got, tc.want)
		}
	}
	if !(MaterialVeil("darwin", "frosted") < MaterialVeil("darwin", "regular") &&
		MaterialVeil("darwin", "regular") < MaterialVeil("darwin", "liquid")) {
		t.Error("the macOS veil ladder is not ordered frosted < regular < liquid")
	}
}

// The compensation is the over-frame algebra read backwards: the fill plus the
// material composites back to the slider, so the same slider position is the
// same solidity on every stop and every platform. A veil heavier than the
// slider's ask leaves no fill at all — the material alone is already denser
// than the ask, and a negative alpha is not a colour.
func TestTheCompensationHoldsOnEveryPlatform(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		for _, step := range []string{"frosted", "regular", "liquid"} {
			veil := MaterialVeil(goos, step)
			for _, opacity := range []uint8{settings.MinWindowOpacity, 25, 47, 70, settings.MaxWindowOpacity} {
				spec := SpecForOS(goos, step, opacity)
				slider := min(SolidTop, float32(opacity)/100)
				want := max(veil, slider) // the material's own weight is a floor
				if got := spec.Composite(); diff(got, want) {
					t.Errorf("%s/%s at %d%%: composite = %v, want %v", goos, step, opacity, got, want)
				}
				if spec.TintAlpha < 0 || spec.TintAlpha > SolidTop {
					t.Errorf("%s/%s at %d%%: fill %v is outside [0, %v]", goos, step, opacity, spec.TintAlpha, SolidTop)
				}
				if spec.Material != veil {
					t.Errorf("%s/%s: Material = %v, want the veil %v", goos, step, spec.Material, veil)
				}
			}
		}
	}
	// A veil past the slider's ask leaves the fill at zero rather than a
	// negative alpha, and the composite is then the material's own weight.
	thin := SpecForOS("darwin", "liquid", settings.MinWindowOpacity)
	if thin.TintAlpha != 0 {
		t.Errorf("the liquid stop at the slider's bottom painted %v, want no fill under a 0.34 material", thin.TintAlpha)
	}
	if got, want := thin.Composite(), MaterialVeil("darwin", "liquid"); diff(got, want) {
		t.Errorf("composite under a heavier material = %v, want the material's own %v", got, want)
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

// The terminal has two backgrounds by design — the app's frame (the title bar
// and the margin sit on it) and the canvas (the colour the user picked) — and
// both used to paint the terminal slider, so 46% showed as
// `1-(1-0.46)^2` = 71% (「终端透明度滚轴说的和看到的不一致」). The split keeps
// one visible number: the frame carries TerminalFrameShare of the alpha and the
// canvas the rest, and the two composite back to the slider exactly.
func TestTheTerminalSplitsItsAlphaBetweenFrameAndCanvas(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		for _, step := range []string{"off", "frosted", "regular", "liquid"} {
			for _, opacity := range []uint8{settings.DefaultTerminalOpacity, 70, settings.MaxWindowOpacity} {
				spec := SpecForOS(goos, step, opacity)
				frame, canvas := spec.TerminalFrameAlpha(), spec.CanvasAlpha()
				if frame <= 0 || frame >= spec.TintAlpha {
					t.Errorf("%s/%s at %d%%: frame = %v, want a share of the frame's %v", goos, step, opacity, frame, spec.TintAlpha)
				}
				if canvas <= 0 || canvas >= 1 {
					t.Errorf("%s/%s at %d%%: canvas = %v, want a fill between 0 and 1", goos, step, opacity, canvas)
				}
				// One sheet: the two layers composite to the frame's alpha, not
				// to `1-(1-t)^2`.
				got := 1 - (1-frame)*(1-canvas)
				if diff(got, spec.TintAlpha) {
					t.Errorf("%s/%s at %d%%: frame %v over canvas %v composites to %v, want the frame's %v",
						goos, step, opacity, frame, canvas, got, spec.TintAlpha)
				}
			}
		}
	}
	// The canvas is the larger share: it carries the colour the user picked, so
	// the palette must dominate the panel it paints.
	spec := SpecFor("regular", settings.DefaultTerminalOpacity)
	if spec.CanvasAlpha() <= spec.TerminalFrameAlpha() {
		t.Errorf("canvas %v is not the larger share of frame %v", spec.CanvasAlpha(), spec.TerminalFrameAlpha())
	}
	// The shipped default no longer doubles: 46% shows as 46%, not 71%.
	if got := spec.Composite(); diff(got, 0.46) {
		t.Errorf("the terminal's visible alpha = %v, want the slider's 0.46", got)
	}
	// A material denser than the slider's ask leaves no fill at all — the
	// window's material is already the whole of the panel, and a negative alpha
	// is not a colour — so both layers are 0 rather than one of them negative.
	thin := SpecForOS("darwin", "liquid", settings.MinWindowOpacity)
	if thin.TerminalFrameAlpha() != 0 || thin.CanvasAlpha() != 0 {
		t.Errorf("the terminal at the slider's bottom under a 0.34 material = %v/%v, want 0/0",
			thin.TerminalFrameAlpha(), thin.CanvasAlpha())
	}
	// An opaque spec (no transparency at all) paints both layers solid.
	opaque := Spec{Translucent: false}
	if opaque.TerminalFrameAlpha() != 1 || opaque.CanvasAlpha() != 1 {
		t.Errorf("an opaque spec = %v/%v, want 1/1", opaque.TerminalFrameAlpha(), opaque.CanvasAlpha())
	}
}

// A window with no system backdrop cannot show the desktop at all, so the
// transparency slider has nothing to move there: the settings page disables it
// with the reason (shell.windowShowsMaterial paints the card opaque in that
// state). macOS and Linux always composite their own windows.
func TestTransparencyIsUnavailableWithoutAMaterial(t *testing.T) {
	cases := []struct {
		goos     string
		vibrancy bool
		want     bool
	}{
		{"windows", false, false}, // Windows before 11 22H2
		{"windows", true, true},
		{"darwin", false, true},
		{"darwin", true, true},
		{"linux", false, true},
		{"linux", true, true},
	}
	for _, tc := range cases {
		if got := TransparencyAvailable(tc.goos, tc.vibrancy); got != tc.want {
			t.Errorf("TransparencyAvailable(%q, %v) = %v, want %v", tc.goos, tc.vibrancy, got, tc.want)
		}
	}
}
