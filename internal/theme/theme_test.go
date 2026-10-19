package theme

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		theme      string
		systemDark bool
		want       Appearance
	}{
		{"auto", false, Light},
		{"auto", true, Dark},
		{"light", false, Light},
		{"light", true, Light}, // explicit wins over the desktop
		{"dark", false, Dark},
		{"dark", true, Dark},
		{"", false, Dark}, // App.tsx paints anything else dark
		{"blue", true, Dark},
	}
	for _, tc := range cases {
		if got := Resolve(tc.theme, tc.systemDark); got != tc.want {
			t.Errorf("Resolve(%q, %v) = %q, want %q", tc.theme, tc.systemDark, got, tc.want)
		}
	}
}

func TestForScalesByTheInterfaceStep(t *testing.T) {
	base := settings.Normalize(settings.Settings{
		Theme:           "light",
		GlassStep:       settings.DefaultGlassStep,
		Language:        settings.DefaultLanguage,
		MainOpacity:     settings.DefaultMainOpacity,
		TerminalOpacity: settings.DefaultTerminalOpacity,
		UIScale:         "default",
	})
	one := For(base, false)

	if one.Appearance != Light {
		t.Errorf("appearance = %q, want light", one.Appearance)
	}
	if one.Scale != 1 {
		t.Errorf("scale = %v, want 1", one.Scale)
	}
	if one.Theme.Dark {
		t.Errorf("theme is dark, want light")
	}

	small := For(settings.Settings{Theme: "auto", UIScale: "small"}, false)
	if small.Scale != 0.9 {
		t.Errorf("small scale = %v, want 0.9", small.Scale)
	}
	if got, want := small.Theme.FontSize, round1(one.Theme.FontSize*0.9); got != want {
		t.Errorf("small font = %v, want %v", got, want)
	}
	if got, want := small.RadiusMD, float32(RadiusMD); got != want {
		t.Errorf("small radius md = %v, want the ladder's literal %v", got, want)
	}

	large := For(settings.Settings{Theme: "dark", UIScale: "large"}, false)
	if large.Appearance != Dark {
		t.Errorf("large appearance = %q, want dark", large.Appearance)
	}
	if got, want := large.RadiusLG, float32(RadiusLG); got != want {
		t.Errorf("large radius lg = %v, want the ladder's literal %v", got, want)
	}
	// An unknown step lands on the shipped default's factor, never zero.
	if got := For(settings.Settings{Theme: "auto", UIScale: "huge"}, false).Scale; got != 0.9 {
		t.Errorf("unknown step scale = %v, want the shipped 0.9", got)
	}
}

// base.css writes the radius ladder in literal px and keeps it out of
// `--ui-scale`: the window and the card keep 14/12/9/6 at every interface step,
// unlike the `--u` metrics around them (base.css:93-96). The port used to
// multiply the ladder by the step, so a `small` chip was 8.1px and a `large` one
// 9.9px instead of the reference's 9. The framework's own control radius reads
// the same `--radius-sm`, so it is literal too.
func TestTheRadiusLadderIsLiteralAtEveryInterfaceStep(t *testing.T) {
	want := struct{ xs, sm, md, lg float32 }{RadiusXS, RadiusSM, RadiusMD, RadiusLG}
	for _, step := range []string{"small", "default", "large", "huge"} {
		tokens := For(settings.Settings{Theme: "dark", UIScale: step}, true)
		if tokens.RadiusXS != want.xs || tokens.RadiusSM != want.sm ||
			tokens.RadiusMD != want.md || tokens.RadiusLG != want.lg {
			t.Errorf("[%s] radius ladder = %v/%v/%v/%v, want the literal %v/%v/%v/%v",
				step, tokens.RadiusXS, tokens.RadiusSM, tokens.RadiusMD, tokens.RadiusLG,
				want.xs, want.sm, want.md, want.lg)
		}
		if tokens.Theme.Radius != RadiusSM {
			t.Errorf("[%s] the framework control radius = %v, want the ladder's literal %v",
				step, tokens.Theme.Radius, RadiusSM)
		}
	}
}

func TestForDoesNotMutateTheSharedTheme(t *testing.T) {
	// Hold the value the framework handed out, then check For scaled only
	// its own copy of it.
	shared := ui.LightTheme()
	_ = For(settings.Settings{Theme: "light", UIScale: "large"}, false)
	if *shared != *ui.LightTheme() {
		t.Errorf("For mutated the shared light theme: %+v", *shared)
	}
}

// The strokes are the two appearances' edge colors, per appearance, as
// base.css wrote them: a lit hairline in the dark, a dark one in the light.
func TestStrokes(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.Appearance != Dark {
		t.Fatalf("appearance = %v", dark.Appearance)
	}
	if dark.InputStroke.A >= 255 {
		t.Errorf("dark stroke = %v, want translucent", dark.InputStroke)
	}
	light := For(settings.Settings{Theme: "light"}, false)
	if light.Appearance != Light {
		t.Fatalf("appearance = %v", light.Appearance)
	}
	if light.InputStroke.R != 0 {
		t.Errorf("light stroke = %v, want the dark-on-light one", light.InputStroke)
	}
	// The focused step takes the accent in both.
	if dark.InputStrokeActive == dark.InputStroke {
		t.Error("the active stroke is the idle one")
	}
}

// The settings content pane carries its own fill, and it rides the
// transparency slider as a single sheet: the pane is painted at the alpha that
// composites to the frame tint plus a small recess (glassmap.Spec.ContentAlpha),
// so the visible bottom is the frame the slider moves, not a second full tint
// stacked on it. The reference pinned `--surface-sunken` to a fixed 0.62 + 0.18
// band instead, which stayed ~0.64-0.80 no matter where the slider was and hid
// the frame tint it sat on - the user read that as "the bottom is opaque and the
// slider does nothing".
func TestSunkenFollowsTheOpacity(t *testing.T) {
	dim := settings.Settings{Theme: "dark", MainOpacity: 10}
	bright := settings.Settings{Theme: "dark", MainOpacity: 100}
	lowT := For(dim, false)
	highT := For(bright, false)
	if lowT.Sunken.A >= highT.Sunken.A {
		t.Errorf("the sunken fill did not rise with the slider: %v vs %v", lowT.Sunken.A, highT.Sunken.A)
	}
	// The pane is a recess over the frame, not a second sheet: at the low stop
	// its own alpha is small, because the frame underneath already carries the
	// tint the slider owns.
	if a := float64(lowT.Sunken.A) / 255; a > 0.55 {
		t.Errorf("the low stop alpha = %v, want a thin recess", a)
	}
	// At the top it reaches the near-solid face the frame itself owns.
	if a := float64(highT.Sunken.A) / 255; a < 0.97 {
		t.Errorf("the high stop alpha = %v, want the frame tint's ~0.98", a)
	}
	// A light appearance sunken fill is the light pane's own colour.
	lightT := For(settings.Settings{Theme: "light", MainOpacity: 100}, false)
	if a, b := float64(lightT.Sunken.A)/255, float64(highT.Sunken.A)/255; a != b {
		t.Errorf("the two appearances' alphas differ: %v vs %v", a, b)
	}
}

// The ladder scales once, by the step: every register at the Large step is its
// own base times 1.1, not "body plus an offset" - the offset that grew with
// every step above the shipped one.
//
// The ladder is one rung above the reference's (11/12/13/14/19), because the
// **shipped** step is `small` = 0.9: one rung up is what makes the shipped step
// draw base.css's own 10/11/12/13/17. The old ladder was the base numbers, so
// the shipped step drew the whole design at 90% (a 9pt caption, a 10.8pt
// emphasis) and every surface read cramped - 「整体 UI/UX 很 小气」.
func TestLadderScalesWithTheStep(t *testing.T) {
	large := For(settings.Settings{Theme: "dark", UIScale: "large"}, false)
	if large.Display < 20.8 || large.Display > 21.0 {
		t.Errorf("display at Large = %v, want 19 x 1.1", large.Display)
	}
	if large.Title < 15.3 || large.Title > 15.5 {
		t.Errorf("title at Large = %v, want 14 x 1.1", large.Title)
	}
	if large.Body < 13.1 || large.Body > 13.3 {
		t.Errorf("body at Large = %v, want 12 x 1.1", large.Body)
	}
	// The shipped step: 0.9. Its registers land on base.css's own physical
	// sizes — caption 10, body 11, emphasis 12, title 13, display 17 — within
	// half a point, which is the whole point of the rung.
	small := For(settings.Settings{Theme: "dark"}, false)
	for _, reg := range []struct {
		name   string
		got    float32
		want   float64
		expect float32
	}{
		{"caption", small.Caption, 10, 9.9},
		{"body", small.Body, 11, 10.8},
		{"emphasis", small.Emphasis, 12, 11.7},
		{"title", small.Title, 13, 12.6},
		{"display", small.Display, 17, 17.1},
	} {
		if d := float64(reg.got) - reg.want; d > 0.5 || d < -0.5 {
			t.Errorf("the shipped step's %s = %v, want the reference's %v (one rung up x 0.9)",
				reg.name, reg.got, reg.want)
		}
		// And its own number, spelled out: a test that only compares against
		// the reference's sizes is satisfied by any rung that lands near it,
		// and the regression this guards is the ladder sliding back down.
		if reg.got != reg.expect {
			t.Errorf("the shipped step's %s = %v, want %v (11/12/13/14/19 x 0.9)",
				reg.name, reg.got, reg.expect)
		}
	}
	// The framework's own body size is the ladder's body register, so a widget
	// the views do not size by hand is the same size as the text beside it.
	if small.Theme.FontSize != small.Body {
		t.Errorf("the theme's font size = %v, want the body register %v", small.Theme.FontSize, small.Body)
	}
}

// The palette is base.css's, not the framework's stock one: the pastel
// accent, the translucent control ladder and the near-black glass. These are
// the tokens the whole app is drawn against, so a regression here repaints
// every surface at once.
func TestPaletteComesFromBaseCSS(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.Theme.Accent != ui.Hex("#8fb7ff") {
		t.Errorf("dark accent = %v, want the pastel #8fb7ff", dark.Theme.Accent)
	}
	if dark.Theme.Background != ui.Hex("#121316") {
		t.Errorf("dark background = %v, want the glass tint #121316", dark.Theme.Background)
	}
	if dark.Theme.Text != ui.Hex("#f2f3f5") {
		t.Errorf("dark text = %v, want --text-primary", dark.Theme.Text)
	}
	// The control ladder is translucent white, not an opaque grey: the
	// framework's stock SurfaceHover would paint a solid block over the glass.
	if dark.Theme.SurfaceHover.A >= 255 {
		t.Errorf("dark hover = %v, want a translucent lift", dark.Theme.SurfaceHover)
	}
	if dark.Theme.AccentText != ui.Hex("#15171b") {
		t.Errorf("dark accent text = %v, want --accent-contrast", dark.Theme.AccentText)
	}

	light := For(settings.Settings{Theme: "light"}, false)
	if light.Theme.Accent != ui.Hex("#064aa6") {
		t.Errorf("light accent = %v, want the darker #064aa6", light.Theme.Accent)
	}
	if light.Theme.Background != ui.Hex("#fafafc") {
		t.Errorf("light background = %v, want #fafafc", light.Theme.Background)
	}
	if light.Theme.Text != ui.Hex("#1d1d1f") {
		t.Errorf("light text = %v, want --text-primary", light.Theme.Text)
	}
	// The halo is `--accent-halo` (base.css:693,958): the accent at a fixed
	// low alpha, the live status dot's ring, not a second accent fill.
	if want := ui.RGBA(143, 183, 255, 0.18); dark.AccentHalo != want {
		t.Errorf("dark accent halo = %v, want %v", dark.AccentHalo, want)
	}
	if want := ui.RGBA(6, 74, 166, 0.24); light.AccentHalo != want {
		t.Errorf("light accent halo = %v, want %v", light.AccentHalo, want)
	}
}

// The surface ladder is ordered and visible: four rungs a viewer can tell
// apart, in the order the design means them — the card's edge over the row's
// hairline, the hairline over a control plane, the plane over nothing.
//
// The rungs were all under 0.08 alpha, which over a translucent dark frame is
// one or two levels of grey: the ladder existed in the token file and nowhere
// on screen, so a card was a card only because of its shadow (「整体 UI/UX 很
// 小气」). These assertions are the numbers, rung by rung, plus the order: a
// revert to the flat ladder fails both the exact values and the ordering, and
// an over-correction into a border-on-every-box design fails the ceiling
// against the contrast preference's own tokens.
func TestTheSurfaceLadderHasAStepToSee(t *testing.T) {
	alpha := func(c ui.Color) float32 { return float32(c.A) / 255 }
	cases := []struct {
		name         string
		systemDark   bool
		stroke       ui.Color
		controlEdge  ui.Color
		hairline     ui.Color
		control      ui.Color
		controlHover ui.Color
		controlPress ui.Color
		track        ui.Color
	}{
		{
			name: "light", systemDark: false,
			stroke:       ui.RGBA(0, 0, 0, 0.12),
			controlEdge:  ui.RGBA(0, 0, 0, 0.1),
			hairline:     ui.RGBA(0, 0, 0, 0.1),
			control:      ui.RGBA(0, 0, 0, 0.05),
			controlHover: ui.RGBA(0, 0, 0, 0.075),
			controlPress: ui.RGBA(0, 0, 0, 0.032),
			track:        ui.RGBA(0, 0, 0, 0.06),
		},
		{
			name: "dark", systemDark: true,
			stroke:       ui.RGBA(255, 255, 255, 0.11),
			controlEdge:  ui.RGBA(255, 255, 255, 0.1),
			hairline:     ui.RGBA(255, 255, 255, 0.09),
			control:      ui.RGBA(255, 255, 255, 0.075),
			controlHover: ui.RGBA(255, 255, 255, 0.105),
			controlPress: ui.RGBA(255, 255, 255, 0.05),
			track:        ui.RGBA(255, 255, 255, 0.09),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tk := ForPreferences(settings.Settings{Theme: "auto", GlassStep: "regular"}, tc.systemDark, ui.Preferences{})
			rungs := []struct {
				name string
				got  ui.Color
				want ui.Color
			}{
				{"InputStroke", tk.InputStroke, tc.stroke},
				{"ControlEdge", tk.ControlEdge, tc.controlEdge},
				{"Hairline", tk.Hairline, tc.hairline},
				{"Control", tk.Control, tc.control},
				{"ControlHover", tk.ControlHover, tc.controlHover},
				{"ControlPress", tk.ControlPress, tc.controlPress},
				{"Track", tk.Track, tc.track},
			}
			for _, rung := range rungs {
				if rung.got != rung.want {
					t.Errorf("%s = %v, want %v", rung.name, rung.got, rung.want)
				}
			}
			// The order is the design: card edge, then the row's rule, then a
			// control plane; hover lifts a pane and a press sinks it below rest.
			if !(alpha(tc.stroke) >= alpha(tc.controlEdge) && alpha(tc.controlEdge) >= alpha(tc.hairline) &&
				alpha(tc.hairline) > alpha(tc.control)) {
				t.Errorf("the ladder is out of order: stroke %.3f, edge %.3f, hairline %.3f, control %.3f",
					alpha(tc.stroke), alpha(tc.controlEdge), alpha(tc.hairline), alpha(tc.control))
			}
			if alpha(tc.controlHover) <= alpha(tc.control) || alpha(tc.controlPress) >= alpha(tc.control) {
				t.Errorf("the pointer ladder is flat: hover %.3f, rest %.3f, press %.3f",
					alpha(tc.controlHover), alpha(tc.control), alpha(tc.controlPress))
			}
			// A control plane is a plane, not a hairline: it has to be a step a
			// 1x display resolves over the frame it sits on.
			if alpha(tc.control) < 0.045 {
				t.Errorf("a control plane at %.3f is below the step a 1x display resolves", alpha(tc.control))
			}
			// And the ladder stays light-handed: the desktop that asked for more
			// contrast must still get a visibly stronger edge, or this rung has
			// eaten the preference.
			high := ForPreferences(settings.Settings{Theme: "auto", GlassStep: "regular"}, tc.systemDark, ui.Preferences{HighContrast: true})
			if alpha(high.InputStroke) <= 2*alpha(tc.stroke) {
				t.Errorf("the contrast stroke %.3f is not clearly above this rung's %.3f", alpha(high.InputStroke), alpha(tc.stroke))
			}
		})
	}
}

// The sidebar's chosen page is the opaque raised rung: the framework paints
// the accent behind a chosen list row, so a translucent lift would read as a
// blue pill, which is exactly what the reference's quiet pill is not.
func TestRaisedQuietIsOpaqueAndNeutral(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    Tokens
	}{
		{"dark", For(settings.Settings{Theme: "dark"}, true)},
		{"light", For(settings.Settings{Theme: "light"}, false)},
	} {
		if tc.t.RaisedQuiet.A != 255 {
			t.Errorf("%s raised-quiet alpha = %d, want opaque", tc.name, tc.t.RaisedQuiet.A)
		}
		// A neutral lift over a near-neutral pane keeps R, G and B within a
		// few steps of each other; an accent-tinted pill would not.
		r, g, b := int(tc.t.RaisedQuiet.R), int(tc.t.RaisedQuiet.G), int(tc.t.RaisedQuiet.B)
		if spread := max(r, g, b) - min(r, g, b); spread > 12 {
			t.Errorf("%s raised-quiet = %v, want a neutral lift", tc.name, tc.t.RaisedQuiet)
		}
	}
}

// The control lens is three inset layers plus the pane edge: a control that
// carries it reads as a lens rather than as a flat tint with a border.
func TestLensTokensAreTranslucent(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.LensRim.A == 0 || dark.LensRim.A >= 255 {
		t.Errorf("lens rim = %v, want a translucent highlight", dark.LensRim)
	}
	if dark.LensEdge.R != 0 {
		t.Errorf("lens edge = %v, want a dark bottom shade", dark.LensEdge)
	}
}

// The glass stop scales the control lens (base.css's `--glass-lens-scale`:
// 0.5 / 1 / 1.6). It is the one rung of the old build's effect table the
// scene can paint on every platform, so it is what makes `frosted`, `regular`
// and `liquid` read apart even where the window has no material to blur the
// desktop with.
func TestLensScalesWithTheGlassStop(t *testing.T) {
	base := settings.Settings{Theme: "dark"}
	frosted := For(withStep(base, "frosted"), true)
	regular := For(withStep(base, "regular"), true)
	liquid := For(withStep(base, "liquid"), true)
	if !(frosted.LensRim.A < regular.LensRim.A && regular.LensRim.A < liquid.LensRim.A) {
		t.Errorf("lens rim did not rise with the stop: %d / %d / %d", frosted.LensRim.A, regular.LensRim.A, liquid.LensRim.A)
	}
	if !(frosted.LensSheen.A < regular.LensSheen.A && regular.LensSheen.A < liquid.LensSheen.A) {
		t.Errorf("lens sheen did not rise with the stop: %d / %d / %d", frosted.LensSheen.A, regular.LensSheen.A, liquid.LensSheen.A)
	}
	// The regular stop is the reference's own lens strength: the palette's
	// base value unchanged.
	palette := paletteFor(Dark)
	if regular.LensRim.A != palette.lensRim.A {
		t.Errorf("regular lens rim = %d, want the palette's %d", regular.LensRim.A, palette.lensRim.A)
	}
	// The off stop drops the lens entirely.
	if off := For(withStep(base, "off"), true); off.LensRim.A != 0 || off.LensSheen.A != 0 || off.LensEdge.A != 0 {
		t.Errorf("off kept a lens: %v / %v / %v", off.LensRim, off.LensSheen, off.LensEdge)
	}
}

// withStep is s with its glass stop set, for the lens tests.
func withStep(s settings.Settings, step string) settings.Settings {
	s.GlassStep = step
	return s
}

// base.css's `--sheen-top` / `--sheen-bottom` are fixed tokens per appearance
// (:628-629 dark 0.046/0.01, :906-907 light 0.66/0.22) and do **not** vary
// with `[data-glass]`: the only token a stop moves is `--glass-lens-scale`,
// and that grades the **control** lens (TestLensScalesWithTheGlassStop), not
// the frame's lit face. So the frame's sheen is the same on `frosted`,
// `regular` and `liquid` — an earlier rewrite graded it by the stop to make the
// stops visible where the window has no material, which is exactly the
// deviation F3b removes — and the off stop drops it with the material.
func TestTheFrameSheenIsFixedAcrossStops(t *testing.T) {
	for _, appearance := range []string{"dark", "light"} {
		base := settings.Settings{Theme: appearance, MainOpacity: settings.DefaultMainOpacity}
		frosted := For(withStep(base, "frosted"), true)
		regular := For(withStep(base, "regular"), true)
		liquid := For(withStep(base, "liquid"), true)

		// The three effect stops paint the same lit face: the sheen is not a
		// rung.
		if frosted.SheenTop != regular.SheenTop || regular.SheenTop != liquid.SheenTop {
			t.Errorf("%s: the stop graded the sheen's top: %v / %v / %v", appearance, frosted.SheenTop, regular.SheenTop, liquid.SheenTop)
		}
		if frosted.SheenBottom != regular.SheenBottom || regular.SheenBottom != liquid.SheenBottom {
			t.Errorf("%s: the stop graded the sheen's bottom: %v / %v / %v", appearance, frosted.SheenBottom, regular.SheenBottom, liquid.SheenBottom)
		}
		// And it is the reference's own fixed token, capped at the frame's
		// alpha so `FaceFill`'s compensation can keep the frame's alpha the
		// slider's (see `baseAlpha`).
		spec := glassmap.SpecForOS(runtime.GOOS, "regular", settings.DefaultMainOpacity)
		palette := paletteFor(Appearance(appearance))
		wantTop := capAlpha(palette.sheenTop, spec.TintAlpha)
		wantBottom := capAlpha(palette.sheenBottom, spec.TintAlpha)
		if regular.SheenTop != wantTop || regular.SheenBottom != wantBottom {
			t.Errorf("%s: sheen = %v / %v, want the palette's capped %v / %v", appearance, regular.SheenTop, regular.SheenBottom, wantTop, wantBottom)
		}
		// The off stop has no glass, so it paints no sheen at all.
		if off := For(withStep(base, "off"), true); off.SheenTop.A != 0 || off.SheenBottom.A != 0 {
			t.Errorf("%s: the off stop kept a sheen: %v / %v", appearance, off.SheenTop, off.SheenBottom)
		}
	}
}

// The sheen is a fixed token, not the frame's tint scaled: where the tint is
// thicker than the dark appearance's own 0.046 the cap is inert and the pixel is
// the reference's fixed alpha, while the light appearance's 0.66 is always
// capped to the thin frame. Either way the frame's alpha stays the slider's —
// the `FaceFill` compensation is the other half of that promise.
//
// The reading is taken at the shipped default rather than at the slider's
// bottom: a macOS window's material is compensated out of the tint
// (glassmap.MaterialVeil), so at the bottom end the fill can be 0 there and the
// cap would bite on every platform's token, which says nothing about the token.
func TestTheFrameSheenIsTheFixedTokenNotTheTint(t *testing.T) {
	at := func(appearance string, opacity uint8) Tokens {
		return For(settings.Settings{Theme: appearance, MainOpacity: opacity}, true)
	}
	// The dark token is under the tint at the shipped default on every
	// platform, so the cap is inert and the pixel is the reference's own fixed
	// alpha.
	if got, want := at("dark", settings.DefaultMainOpacity).SheenTop, paletteFor(Dark).sheenTop; got != want {
		t.Errorf("the dark sheen = %v, want the fixed token %v", got, want)
	}
	// The light token is heavier than a thin frame, so the cap bites — but the
	// value is still the fixed token capped, never the tint scaled.
	light := paletteFor(Light)
	spec := glassmap.SpecForOS(runtime.GOOS, "liquid", settings.MinWindowOpacity)
	want := capAlpha(light.sheenTop, spec.TintAlpha)
	got := at("light", settings.MinWindowOpacity).SheenTop
	if got != want {
		t.Errorf("the light sheen at the slider's bottom = %v, want the capped fixed token %v", got, want)
	}
	if want.A >= light.sheenTop.A {
		t.Errorf("the light cap did not bite: %d vs the token's %d", want.A, light.sheenTop.A)
	}
}

// A desktop that asked for more contrast gets base.css's
// `@media (prefers-contrast: more)` tokens: the card stroke, the grouped
// row's inner edge and the field placeholder are swapped per appearance. The
// rest of the palette is untouched, which is what makes the preference a
// change of *contrast* rather than a second look.
func TestHighContrastSwapsTheStrokeAndPlaceholderTokens(t *testing.T) {
	cases := []struct {
		name        string
		systemDark  bool
		stroke      ui.Color
		soft        ui.Color
		placeholder ui.Color
	}{
		{
			name: "light", systemDark: false,
			stroke:      ui.RGBA(0, 0, 0, 0.5),
			soft:        ui.RGBA(0, 0, 0, 0.36),
			placeholder: ui.RGBA(28, 28, 32, 0.68),
		},
		{
			name: "dark", systemDark: true,
			stroke:      ui.RGBA(255, 255, 255, 0.42),
			soft:        ui.RGBA(255, 255, 255, 0.34),
			placeholder: ui.RGBA(242, 243, 245, 0.58),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto", GlassStep: "regular"}
			plain := ForPreferences(s, tc.systemDark, ui.Preferences{})
			high := ForPreferences(s, tc.systemDark, ui.Preferences{HighContrast: true})

			if high.InputStroke != tc.stroke {
				t.Errorf("InputStroke = %v, want %v", high.InputStroke, tc.stroke)
			}
			if high.ControlEdge != tc.soft {
				t.Errorf("ControlEdge = %v, want %v", high.ControlEdge, tc.soft)
			}
			if high.Theme.TextMuted != tc.placeholder {
				t.Errorf("theme TextMuted = %v, want the placeholder contrast %v", high.Theme.TextMuted, tc.placeholder)
			}
			// The framework theme the widgets read is the same palette: its
			// Border is the card's stroke, so a button or a field the views
			// do not paint by hand gets the stronger edge too.
			if high.Theme.Border != tc.stroke {
				t.Errorf("theme Border = %v, want the contrast stroke %v", high.Theme.Border, tc.stroke)
			}
			// The preference has to change something, or it is an inert knob.
			if high.InputStroke == plain.InputStroke && high.ControlEdge == plain.ControlEdge && high.Theme.TextMuted == plain.Theme.TextMuted {
				t.Error("more contrast changed no token")
			}

			// Nothing else moved: blank the three contrast tokens, the
			// pointer and the three fields the material floor moves, and the
			// two resolutions have to be the same value.
			blank := func(tk Tokens) Tokens {
				tk.Theme = nil
				tk.InputStroke, tk.ControlEdge = ui.Color{}, ui.Color{}
				// The three the material floor and the lens damping move.
				tk.Sunken, tk.RaisedQuiet, tk.Float = ui.Color{}, ui.Color{}, ui.Color{}
				tk.LensRim, tk.LensSheen, tk.LensEdge = ui.Color{}, ui.Color{}, ui.Color{}
				tk.SheenTop, tk.SheenBottom = ui.Color{}, ui.Color{}
				return tk
			}
			if got, want := blank(high), blank(plain); got != want {
				t.Errorf("contrast moved tokens besides the three:\n got %+v\nwant %+v", got, want)
			}
			// The material floor rides through: the content pane and a
			// floating pane are painted at the raised frame's own alpha.
			if high.Sunken.A <= plain.Sunken.A {
				t.Errorf("the content pane did not follow the raised floor: %d vs %d", high.Sunken.A, plain.Sunken.A)
			}
			if high.Float.A <= plain.Float.A {
				t.Errorf("a floating pane did not follow the raised floor: %d vs %d", high.Float.A, plain.Float.A)
			}
		})
	}
}

// IconSurface is the warm amber plane (base.css's `--system-icon-surface`),
// not the warning ink at a low alpha: quoting the label's colour here painted
// the launcher feedback row pink-red. The two tokens must be measurably
// different in both appearances.
func TestIconSurfaceIsTheWarmPlaneNotTheWarningInk(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
		wantHover  ui.Color
	}{
		{"dark", true, ui.RGBA(255, 176, 132, 0.115), ui.RGBA(255, 176, 132, 0.2)},
		{"light", false, ui.RGBA(214, 118, 44, 0.13), ui.RGBA(214, 118, 44, 0.21)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto"}
			tokens := ForPreferences(s, tc.systemDark, ui.Preferences{})
			if tokens.IconSurface != tc.want {
				t.Errorf("IconSurface = %v, want %v", tokens.IconSurface, tc.want)
			}
			if tokens.IconSurfaceHover != tc.wantHover {
				t.Errorf("IconSurfaceHover = %v, want %v", tokens.IconSurfaceHover, tc.wantHover)
			}
			// The plane must not equal the warning ink: the launcher feedback
			// row quotes this token as its fill, and the warning colour is
			// reserved for the label.
			if tokens.IconSurface == tokens.Theme.Warning {
				t.Error("IconSurface equals Warning: the feedback row would read pink-red instead of warm amber")
			}
		})
	}
}

// RaisedWarm is base.css's `--glass-raised-warm`, an alias of
// `--system-icon-surface` (`base.css:611`): the warm raised pane a broken
// integration row paints. It is the same amber plane as IconSurface, not the
// accent and not the warning ink, and unlike the frame's tint it does not
// follow the transparency slider.
func TestRaisedWarmIsTheWarmPlane(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
	}{
		{"dark", true, ui.RGBA(255, 176, 132, 0.115)},
		{"light", false, ui.RGBA(214, 118, 44, 0.13)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := ForPreferences(settings.Settings{Theme: "auto"}, tc.systemDark, ui.Preferences{})
			if tokens.RaisedWarm != tc.want {
				t.Errorf("RaisedWarm = %v, want the reference's --glass-raised-warm %v", tokens.RaisedWarm, tc.want)
			}
			if tokens.RaisedWarm != tokens.IconSurface {
				t.Errorf("RaisedWarm %v is not the --system-icon-surface alias %v", tokens.RaisedWarm, tokens.IconSurface)
			}
			if tokens.RaisedWarm == tokens.Theme.Warning {
				t.Error("RaisedWarm equals the warning ink; the broken row's fill would read pink-red")
			}
			if tokens.RaisedWarm == tokens.Theme.Accent {
				t.Error("RaisedWarm equals the accent; a broken row would read as an action")
			}
		})
	}
}

// AccentTintQuiet is the launcher selection's own rung of the accent-tint
// family (base.css's `--accent-tint-quiet`): the fill a result row wears when
// the emphasis is its 700-weight title rather than the fill. Unlike the
// single-number `--accent-tint`, it is a *per-appearance* token — dark 0.085,
// light 0.065 — because the same pastel blue over near-black gains contrast
// while the same alpha over white barely moves the pixel. The port had quoted
// the dark number as `Accent.Alpha(0.085)` for both appearances, so the light
// launcher painted its selected row a rung too heavy.
func TestAccentTintQuietStepsDownInTheLightAppearance(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
	}{
		{"dark", true, ui.RGBA(143, 183, 255, 0.085)},
		{"light", false, ui.RGBA(6, 74, 166, 0.065)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := ForPreferences(settings.Settings{Theme: "auto"}, tc.systemDark, ui.Preferences{})
			if tokens.AccentTintQuiet != tc.want {
				t.Errorf("AccentTintQuiet = %v, want the reference's --accent-tint-quiet %v", tokens.AccentTintQuiet, tc.want)
			}
			// It is the accent's own hue at a quieter alpha, not a second
			// colour: the two must share R/G/B and differ only in alpha.
			if tokens.AccentTintQuiet.R != tokens.Theme.Accent.R ||
				tokens.AccentTintQuiet.G != tokens.Theme.Accent.G ||
				tokens.AccentTintQuiet.B != tokens.Theme.Accent.B {
				t.Errorf("AccentTintQuiet %v is a different hue from the accent %v", tokens.AccentTintQuiet, tokens.Theme.Accent)
			}
		})
	}
}

// SystemIconText is the ink a warm system-icon plane carries (base.css's
// `--system-icon-text`): the amber ink the launcher's power rows draw their
// glyph in. It is a token of its own, not the warning colour, and it differs
// by appearance — a pale warm peach on dark, a burnt orange on light.
func TestSystemIconTextIsItsOwnAmberInkNotTheWarningColour(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
	}{
		{"dark", true, ui.RGBA(255, 209, 178, 0.82)},
		{"light", false, ui.Hex("#a24d15")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := ForPreferences(settings.Settings{Theme: "auto"}, tc.systemDark, ui.Preferences{})
			if tokens.SystemIconText != tc.want {
				t.Errorf("SystemIconText = %v, want the reference's --system-icon-text %v", tokens.SystemIconText, tc.want)
			}
			if tokens.SystemIconText == tokens.Theme.Warning {
				t.Error("SystemIconText equals the warning ink: the power glyph would wear the label's red-grey")
			}
			// Warm, not neutral: the plate it sits on is amber, so its ink
			// leads red over blue on both appearances.
			if tokens.SystemIconText.R <= tokens.SystemIconText.B {
				t.Errorf("SystemIconText %v does not read warm (R=%d, B=%d)", tokens.SystemIconText, tokens.SystemIconText.R, tokens.SystemIconText.B)
			}
		})
	}
}

// The platform reaches the tokens: a macOS window's backdrop material already
// lays a veil of its own over the desktop (glassmap.MaterialVeil), so the tint
// the app paints is thinner there than on a platform with no material — while
// the alpha the user *sees* is the slider's on both. That is the whole point of
// the compensation: one slider position, one solidity, whichever material the
// window server puts behind it.
func TestThePlatformsMaterialReachesTheTokens(t *testing.T) {
	s := settings.Settings{Theme: "dark", GlassStep: "regular", MainOpacity: settings.DefaultMainOpacity}
	darwin := ForPlatform(s, true, ui.Preferences{}, "darwin")
	linux := ForPlatform(s, true, ui.Preferences{}, "linux")
	windows := ForPlatform(s, true, ui.Preferences{}, "windows")

	// The fill is thinner under a material...
	if darwin.Sunken.A >= linux.Sunken.A {
		t.Errorf("the darwin content pane = %d, want thinner than the material-free %d", darwin.Sunken.A, linux.Sunken.A)
	}
	if darwin.Float.A >= linux.Float.A {
		t.Errorf("the darwin floating pane = %d, want thinner than the material-free %d", darwin.Float.A, linux.Float.A)
	}
	// ...and the sheen is capped to that thinner fill, or `FaceFill` could not
	// composite back to the slider's own alpha.
	if darwin.SheenTop.A > linux.SheenTop.A {
		t.Errorf("the darwin sheen = %d, want at most the material-free %d", darwin.SheenTop.A, linux.SheenTop.A)
	}
	// Windows has one backdrop for every stop and this build does not
	// compensate for it yet (glassmap.MaterialVeil), so it paints the tint a
	// material-free platform does.
	if windows.Sunken != linux.Sunken {
		t.Errorf("windows content pane = %v, want the uncompensated %v", windows.Sunken, linux.Sunken)
	}
	// The visible alpha is the slider's on all three.
	for _, goos := range []string{"darwin", "windows", "linux"} {
		spec := glassmap.SpecForOS(goos, s.GlassStep, s.MainOpacity)
		if diff := spec.Composite() - float32(s.MainOpacity)/100; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("%s composite = %v, want the slider's %v", goos, spec.Composite(), float32(s.MainOpacity)/100)
		}
	}
}

// The window's own corner is the platform's, not the design's.
//
// base.css carries two `--window-radius` values: 14px for the default sheet and
// 8px inside `.platform-windows` (`base.css:66,1492`), because Windows' DWM
// rounds every window at its own ~8px preset (`DWMWCP_ROUND`) and the card's
// corner has to follow it — a 14px card corner inside an 8px window corner
// leaves the system's frame color showing as a crescent in each corner, which
// is one of the pieces the user read as 「窗口周边外圈还有一圈额外的底色」.
//
// The port had one radius for all three platforms (RadiusLG), so Windows' card
// was the reference's macOS corner. The three answers are pinned by name here
// so no host's suite can only see its own.
func TestTheWindowsWindowCornerFollowsDWM(t *testing.T) {
	s := settings.Settings{}
	for _, tc := range []struct {
		goos string
		want float32
	}{
		{"windows", 8},
		{"darwin", RadiusLG},
		{"linux", RadiusLG},
	} {
		got := ForPlatform(s, true, ui.Preferences{}, tc.goos).WindowRadius
		if got != tc.want {
			t.Errorf("%s window radius = %v, want %v", tc.goos, got, tc.want)
		}
		// The rung it replaces is untouched: `--radius-lg` stays 14px in the
		// ladder (a card holding controls), whatever the window's corner is.
		if lg := ForPlatform(s, true, ui.Preferences{}, tc.goos).RadiusLG; lg != RadiusLG {
			t.Errorf("%s RadiusLG = %v, want the ladder's %v", tc.goos, lg, float32(RadiusLG))
		}
	}
}
