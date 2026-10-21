package settingsui

import (
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The P8-b pins: the Raycast contract's own numbers, checked against what the
// settings surface actually paints (`docs/raycast-design.md` §4/§6.1).
//
// This file is the *new* baseline for the settings page. The pins the round
// before it carried were written against the old base.css language (an opaque
// raised sidebar pill, a lensed grouped card, a gradient separator, a
// `--glass-field` slot, a `999px` CTA). Those were re-pinned in place with the
// reason in their own comments; these are the facts that had no pin at all, or
// that the contract states as a *prohibition* and nothing was reading.

// renderThemed draws the app in the appearance the store asks for, with the
// resolved tokens installed as the context theme first: a pixel read then
// compares against the same numbers the view painted with.
func renderThemed(t *testing.T, a *App, w, h int) (*ui.Tester, theme.Tokens) {
	t.Helper()
	skipWithoutTheReferenceFont(t)
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt, tokens
}

// §6.1 第 1 条 · The sidebar's chosen page is the **neutral** row-selection
// rung, not the old opaque raised pill, and it carries no lens.
//
// The pin reads the chosen row's own fill and compares it to
// `RowSelected` over the sidebar's pane — and then proves the fill is
// *neutral*, which is the whole point: the accent is spent on the leading
// glyph (see the pin below), never on the row's plane.
func TestTheSidebarSelectionIsTheNeutralRowSelected(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt, tokens := renderThemed(t, a, 720, 580)
	img := tt.Image()
	chosen := mustFind(t, tt, "General")
	// The pane the row sits on, read just outside the row's left edge.
	pane := imagePixel(img, 2, int(chosen.Y+chosen.H/2))
	fill := imagePixel(img, int(chosen.X)+4, int(chosen.Y+chosen.H/2))
	want := panelOver(tokens.RowSelected, pane)
	if !panelClose(fill, want, 4) {
		t.Errorf("the chosen page's fill = %v, want RowSelected over the pane (%v)", fill, want)
	}
	// The old face was `RaisedQuiet`, an opaque lift — a level or two above
	// the rung the contract names. It must not be what is painted.
	if old := panelOver(tokens.RaisedQuiet.Alpha(0.99), pane); panelClose(fill, old, 3) {
		t.Errorf("the chosen page's fill = %v is the old raised-quiet pane (%v)", fill, old)
	}
	// Neutral: one level in all three channels, so the plane carries no hue.
	if d := channelDelta(fill, ui.Color{R: fill.B, G: fill.B, B: fill.B, A: fill.A}); d > 4 {
		t.Errorf("the chosen page's fill %v is not neutral (%d levels of hue)", fill, d)
	}
	// No lens: the row's top inner edge is the same colour as its mid-line.
	// A `--elev-0` stack painted a lit rim one pixel under the top edge.
	top := imagePixel(img, int(chosen.X)+4, int(chosen.Y)+2)
	mid := imagePixel(img, int(chosen.X)+4, int(chosen.Y+chosen.H/2))
	if channelDelta(top, mid) > 4 {
		t.Errorf("the chosen page still wears a lens: its top rim %v is not its fill %v", top, mid)
	}
}

// §5 第 1 条 · The leading glyph of the chosen page **is** the accent. This is
// the counter-pin to the test above: the row's fill lost the accent, the mark
// kept it, and a change that stripped both would fail here.
func TestTheSidebarChosenGlyphKeepsTheAccent(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt, tokens := renderThemed(t, a, 720, 580)
	img := tt.Image()
	chosen := mustFind(t, tt, "General")
	rest := mustFind(t, tt, "Sessions")
	// The mark sits in the row's leading 28u slot, left of the label.
	warmest := func(r ui.Rect) (ui.Color, int) {
		best, ink := -999, ui.Color{}
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X) + 6; x < int(r.X)+26; x++ {
				p := imagePixel(img, x, y)
				if d := int(p.R) - int(p.B); d > best {
					best, ink = d, p
				}
			}
		}
		return ink, best
	}
	ink, warmth := warmest(chosen)
	if warmth < 60 {
		t.Errorf("the chosen page's mark is not the accent: %v is %d levels from grey", ink, warmth)
	}
	if d := channelDelta(ink, tokens.Theme.Accent); d > 24 {
		t.Errorf("the chosen page's mark = %v, want the accent %v", ink, tokens.Theme.Accent)
	}
	// And a page that is not chosen draws its mark in the plain ink rung.
	if _, restWarmth := warmest(rest); restWarmth >= 40 {
		t.Errorf("an unchosen page's mark carries a hue (warmth %d); the accent is the chosen page's alone",
			restWarmth)
	}
}

// §6.1 第 4 条 (launcher §4.2/§4.6's own rule) · The two separators — the
// header's and the sidebar's — are one solid *weak* hairline: no gradient, and
// the same colour at both ends and in the middle.
//
// The old rule was base.css's `--hairline-fade`: three gradient segments
// (lead 0.12 / solid 0.76 / lead 0.12) in the *strong* hairline. The pin is
// written to catch exactly that: the three samples must agree, and each must be
// `HairlineSoft` over the pane, not `Hairline`.
func TestTheSettingsRulesAreOneSolidWeakHairline(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt, tokens := renderThemed(t, a, 720, 580)
	img := tt.Image()
	pane := imagePixel(img, 2, 30)
	// The header's rule: the first full-width row under the header band that
	// is not the pane. The band is `launcher.FieldRowUnits` tall, the same one
	// the launcher's field row charges.
	band := int(tokens.Theme.Space(launcher.FieldRowUnits))
	rule, ruleY := -1, -1
	for y := band - 6; y < band+6; y++ {
		if y < 0 || y >= img.Rect.Dy() {
			continue
		}
		if channelDelta(imagePixel(img, 400, y), pane) > 4 {
			rule, ruleY = y, y
			break
		}
	}
	if rule < 0 {
		t.Fatalf("the header's rule was not drawn under the header band (y≈%d)", band)
	}
	want := panelOver(tokens.HairlineSoft, pane)
	strong := panelOver(tokens.Hairline, pane)
	for _, x := range []int{3, 100, 360, 700} {
		got := imagePixel(img, x, ruleY)
		if !panelClose(got, want, 4) {
			t.Errorf("the header's rule at x=%d is %v, want the weak hairline over the pane (%v)", x, got, want)
		}
		if !panelClose(want, strong, 2) && panelClose(got, strong, 3) {
			t.Errorf("the header's rule at x=%d is the *strong* hairline %v; §6.1 gives it the weak one", x, got)
		}
	}
	// One pixel tall: the row above and below are the pane, not the rule.
	for _, y := range []int{ruleY - 1, ruleY + 1} {
		if y < 0 || y >= img.Rect.Dy() {
			continue
		}
		if panelClose(imagePixel(img, 400, y), want, 3) {
			t.Errorf("the header's rule spans more than one row (y=%d is also the rule)", y)
		}
	}

	// The sidebar's own column separator: one vertical line, the same value at
	// the top, the middle and the bottom.
	x := -1
	for candidate := 100; candidate < 220; candidate++ {
		if channelDelta(imagePixel(img, candidate, 300), pane) > 4 {
			x = candidate
			break
		}
	}
	if x < 0 {
		t.Fatalf("the sidebar's column separator was not drawn between x=100 and x=220")
	}
	for _, y := range []int{60, 300, img.Rect.Dy() - 20} {
		got := imagePixel(img, x, y)
		if !panelClose(got, want, 5) {
			t.Errorf("the sidebar's separator at y=%d is %v, want the weak hairline (%v)", y, got, want)
		}
	}
	// The fade's own signature — a *differently* coloured first and last few
	// pixels — is what the two previous blocks would have caught; this states
	// it once more from the "no gradient anywhere in the band" side: the rule's
	// run of like-coloured pixels spans the panel's whole width.
	run := 0
	for cx := 0; cx < img.Rect.Dx(); cx++ {
		if panelClose(imagePixel(img, cx, ruleY), want, 5) {
			run++
		}
	}
	if run < img.Rect.Dx()-8 {
		t.Errorf("the header's rule covers %d of %d columns; a full-width solid hairline covers all of them",
			run, img.Rect.Dx())
	}
}

// §6.1 第 3 条 · The grouped card is `Control` + `HairlineSoft` at radius 10 —
// no lens and no control-edge keyline.
func TestTheGroupedCardWearsTheContractsFaceEdgeAndCorner(t *testing.T) {
	a := New(newStore(t), Actions{})
	if tokens := theme.ForPreferences(a.Store.Snapshot(), false, ui.Preferences{}); tokens.RadiusMD != 10 {
		t.Errorf("the panel-block radius is %v, want the contract's 10 (RadiusMD)", tokens.RadiusMD)
	}
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(ui.RGB(255, 255, 255))
		ui.Column(c).Padding(20).Children(func() {
			a.card(c, func() {
				a.row(c, "Row", "hint", func() {})
			})
		})
	}, 400, 200)
	tt.Frame()
	img := tt.Image()
	page := imagePixel(img, 2, 2)
	// The card's own box, found by its extent: the outermost painted columns
	// and rows (the 20u page padding is the page colour on all four sides).
	box := paintedBox(img, page)
	// The card's face: Control over the page, read inside the card clear of
	// its rows' ink.
	face := imagePixel(img, int(box.X+box.W)-6, int(box.Y+box.H/2))
	if want := panelOver(tokens.Control, page); !panelClose(face, want, 5) {
		t.Errorf("the card's face = %v, want Control over the page (%v)", face, want)
	}
	// Its edge is the *weak* hairline, and it is one pixel: the card's first
	// painted row over the page.
	edgeY := -1
	for y := 0; y < img.Rect.Dy(); y++ {
		if channelDelta(imagePixel(img, 200, y), page) > 4 {
			edgeY = y
			break
		}
	}
	if edgeY < 0 {
		t.Fatalf("the card's edge was not drawn")
	}
	cardFace := panelOver(tokens.Control, page)
	edge := imagePixel(img, 200, edgeY)
	// The border is composited over the card's own face (a border is inside
	// the box), which is what makes the edge readable as this card's outline
	// rather than as a line on the page.
	weak := panelOver(tokens.HairlineSoft, cardFace)
	if !panelClose(edge, weak, 4) {
		t.Errorf("the card's edge = %v, want HairlineSoft over the card's face (%v)", edge, weak)
	}
	if controlEdge := panelOver(tokens.ControlEdge, cardFace); !panelClose(weak, controlEdge, 2) &&
		panelClose(edge, controlEdge, 3) {
		t.Errorf("the card's edge = %v is the old ControlEdge keyline; §6.1 第 3 条 gives it the weak hairline", edge)
	}
	// No lens: the card's top inner row is its face, not a lit rim.
	if inner := imagePixel(img, 200, edgeY+2); channelDelta(inner, face) > 4 {
		t.Errorf("the card still wears a lens: its top inner row %v is not its face %v", inner, face)
	}
}

// §6.1 第 7 条 · The permission pills are neutral — both tiers — and no pill
// carries an accent inset.
func TestThePermissionPillsAreNeutralAndAccentFree(t *testing.T) {
	copy := i18n.For("en").Settings
	_ = copy
	record := &ApprovalRecord{
		At: 1700000000, ApprovedDigest: "sha256-aaaa", CurrentDigest: "sha256-bbbb",
		Enforced: []string{"clipboard-write"}, Disclosure: []string{"filesystem-read"},
	}
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.p", Name: "P", Enabled: true, Approval: record}}
	}
	tt := render(t, a, 720, 1100)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	img := tt.Image()
	enforced := mustFind(t, tt, "Write the clipboard")
	declared := mustFind(t, tt, "Read files")
	for _, tc := range []struct {
		name string
		rect ui.Rect
	}{{"enforced", enforced}, {"declared", declared}} {
		// The pill's own leading padding band, clear of its label's glyphs.
		pill := imagePixel(img, int(tc.rect.X)-5, int(tc.rect.Y+tc.rect.H/2))
		// R−G and G−B both under the threshold: the face is a grey, so it
		// cannot be the accent's wash.
		if rMinusG, gMinusB := channelDelta(pill, ui.Color{R: pill.G, G: pill.G, B: pill.B, A: pill.A}), channelDelta(pill, ui.Color{R: pill.R, G: pill.R, B: pill.R, A: pill.A}); rMinusG > 6 || gMinusB > 6 {
			t.Errorf("[%s] the pill's face %v is not neutral (R−G %d, G−B %d)", tc.name, pill, rMinusG, gMinusB)
		}
	}
}

// §6.1 第 9 条 + §5 第 2 条 · The primary CTA is an accent *fill* with the
// contract's `RadiusSM` corner — an accent button, not an accent stadium.
func TestThePrimaryCTAIsAnAccentButtonAtRadiusSM(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{InstallUpdate: func() {}})
	a.About.Version = "0.3.14"
	a.Update = UpdateState{Checked: true, Available: true, Version: "9.9.9"}
	a.selectPage(PageAbout)
	tt, tokens := renderThemed(t, a, 900, 700)
	img := tt.Image()
	button := mustFind(t, tt, copy.UpdateDownload)
	// The fill is the accent.
	fill := imagePixel(img, int(button.X)+6, int(button.Y+button.H/2))
	if d := channelDelta(fill, tokens.Theme.Accent); d > 20 {
		t.Errorf("the primary CTA's fill = %v, want the accent %v", fill, tokens.Theme.Accent)
	}
	// The corner is the ladder's 8, not a stadium: at the button's own top row
	// the accent starts `radius × (1 - 1/√2)` ≈ 2.3 px in for a radius-8
	// corner, and effectively at the box's edge for a `999px` one. Reading the
	// inset at the top row is the pixel form of "which radius is this".
	left := -1
	for x := int(button.X) - 2; x < int(button.X+button.W)+2; x++ {
		if channelDelta(imagePixel(img, x, int(button.Y)+1), tokens.Theme.Accent) < 60 {
			left = x
			break
		}
	}
	if left < 0 {
		t.Fatalf("no accent ink found in the button's top row")
	}
	inset := float32(left) - button.X
	// A radius-8 corner at a 30px-tall box reaches ~2.9px at the first row; a
	// 999px stadium reaches ~15px (half the height). The pin brackets the
	// first, so the pill cannot pass.
	// A radius-8 arc reaches ~5-6px in at a 29px box's first painted row; a
	// `999px` stadium reaches half the height (~15px). The bracket sits between
	// them with room for the arc's own boxing, so the stadium cannot pass.
	if inset > 8 {
		t.Errorf("the CTA's top row starts %v px in: that is a stadium's curve, not RadiusSM(%v)",
			inset, tokens.RadiusSM)
	}
	if inset < 1 {
		t.Errorf("the CTA has no corner at all (inset %v at its first row)", inset)
	}
	// And the token the view asks for is the ladder's, not a `Space()` multiple.
	if tokens.RadiusSM != float32(theme.RadiusSM) {
		t.Errorf("the CTA's corner token = %v, want the literal RadiusSM %v", tokens.RadiusSM, theme.RadiusSM)
	}
}

// §6.1 第 6 条 · The overlay button's corner is `RadiusSM` at *every* interface
// size: a radius is literal px and does not follow `--ui-scale` (§2). The two
// steps below differ by 22% in their `Space()` unit, so a `Space(3.5)` corner
// cannot pass both.
func TestTheOverlayButtonCornerIsLiteralAtEveryStep(t *testing.T) {
	cornerInset := func(step string) (float32, theme.Tokens) {
		t.Helper()
		store := stepStore(t, step)
		a := New(store, Actions{})
		var tokens theme.Tokens
		var ref ui.Handle
		var box ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			tokens = a.tokens(c)
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(255, 255, 255))
			ui.Column(c).Padding(40).Children(func() {
				ui.Row(c).Children(func() {
					a.sheetActionButton(c, "Clear history", false, func() {}).Bind(&ref)
				})
			})
			box = c.Resolve(ref).Bounds()
		}, 400, 200)
		tt.Frame()
		img := tt.Image()
		left := -1
		for x := int(box.X) - 2; x < int(box.X+box.W)+2; x++ {
			if p := imagePixel(img, x, int(box.Y)+1); p.R < 252 || p.G < 252 || p.B < 252 {
				left = x
				break
			}
		}
		if left < 0 {
			t.Fatalf("[%s] the button's top row draws no edge", step)
		}
		return float32(left) - box.X, tokens
	}
	smallInset, smallTokens := cornerInset("small")
	largeInset, largeTokens := cornerInset("large")
	if smallTokens.RadiusSM != 8 || largeTokens.RadiusSM != 8 {
		t.Errorf("the corner token moved with the step: small %v, large %v, want 8 at both",
			smallTokens.RadiusSM, largeTokens.RadiusSM)
	}
	// The same literal corner on a taller box: the inset a radius-8 arc makes
	// on its first painted row grows with the box only because the row is
	// measured from a taller box's own top, and the two steps' boxes differ.
	// What must hold is that neither corner is the `Space(3.5)` stadium the old
	// code asked for: that would be 12.6 and 15.4, i.e. more than half the box.
	for _, tc := range []struct {
		step  string
		inset float32
	}{{"small", smallInset}, {"large", largeInset}} {
		if tc.inset > 6 {
			t.Errorf("[%s] the overlay button's first row starts %v px in: a stadium, not RadiusSM(8)",
				tc.step, tc.inset)
		}
		if tc.inset < 1 {
			t.Errorf("[%s] the overlay button has no corner (inset %v)", tc.step, tc.inset)
		}
	}
}

// §6.1 第 8 条 · A status chip's ring is `ControlEdge`, not the accent or the
// warning ink, and its weight is capped at 600.
//
// Two halves, because a 1px inset ring over a warm face is not a stable pixel
// to read on its own: the face resolution is pinned at the token level (the
// ring the view *asks for*), and the frame is pinned as a *prohibition* — no
// pixel of the chip or its immediate border may be warmer than the warm face
// it sits on, which is exactly what a `t.Warning` keyline produced.
func TestTheStatusChipRingIsTheControlEdge(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.bad", Name: "Bad Tool", Enabled: true, Health: "Degraded"}}
	}
	tt, tokens := renderThemed(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	var cardTheme *ui.Theme
	probe := ui.NewTester(func(c *ui.Context) {
		cardTheme = a.cardTheme(c)
	}, 10, 10)
	probe.Frame()
	for _, tc := range []struct{ name, raw string }{
		{"degraded", extensions.HealthDegraded}, {"unhealthy", extensions.HealthUnhealthy},
		{"healthy", extensions.HealthHealthy},
	} {
		_, keyline, _ := healthStatusFace(tokens, cardTheme, copy, tc.name, tc.raw)
		if keyline != tokens.ControlEdge {
			t.Errorf("[%s] the chip's keyline = %v, want the neutral ControlEdge %v (§6.1 第 8 条)",
				tc.name, keyline, tokens.ControlEdge)
		}
		if keyline == tokens.AccentEdge || keyline == tokens.Theme.Warning {
			t.Errorf("[%s] the chip's keyline is the old coloured ring (%v)", tc.name, keyline)
		}
	}

	img := tt.Image()
	word := mustFind(t, tt, "Degraded")
	// The chip's warm face: the most common colour in the box just left of the
	// verdict word (the chip's own 8u padding, clear of glyphs).
	face := imagePixel(img, int(word.X)-4, int(word.Y+word.H/2))
	if warmth := int(face.R) - int(face.B); warmth < 8 {
		t.Fatalf("the verdict chip's face is not warm (%v, %d levels); the probe is not reading the chip",
			face, warmth)
	}
	// No pixel of the chip's own padding band — its rounding, its ring and its
	// face, but not the warning-ink glyphs inside — may be warmer than that
	// face: the old ring was a `t.Warning` keyline, which is warmer still.
	worst, worstC := 0, ui.Color{}
	for y := int(word.Y) - 6; y < int(word.Y+word.H)+6; y++ {
		for x := int(word.X) - 14; x < int(word.X)-1; x++ {
			p := imagePixel(img, x, y)
			if w := int(p.R) - int(p.B); w > worst {
				worst, worstC = w, p
			}
		}
	}
	if faceWarm := int(face.R) - int(face.B); worst > faceWarm+8 {
		t.Errorf("a pixel of the verdict chip is warmer than its own face (%v at %d levels, face %d): "+
			"the old warning keyline is back", worstC, worst, faceWarm)
	}
}

// The guard the task book asks for: the settings views paint from tokens, never
// from a literal colour. `internal/theme` is the one place a value is written
// down; a `#RRGGBB` or an `rgba(` in a settings view is a token that was not
// hoisted.
func TestTheSettingsViewsPaintFromTokensNotLiterals(t *testing.T) {
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)
	rgba := regexp.MustCompile(`rgba\s*\(`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not read the package: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := name
		data, err := os.ReadFile(filepath.Join(".", path))
		if err != nil {
			t.Fatalf("could not read %s: %v", path, err)
		}
		checked++
		for i, line := range strings.Split(string(data), "\n") {
			// A comment may quote the reference's own CSS — that is
			// documentation of where a token came from, not a painted value.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if loc := hex.FindString(line); loc != "" {
				t.Errorf("%s:%d paints a hex literal (%s); read a token from internal/theme instead",
					path, i+1, loc)
			}
			if loc := rgba.FindString(line); loc != "" {
				t.Errorf("%s:%d paints an rgba() literal; read a token from internal/theme instead",
					path, i+1)
			}
		}
	}
	if checked == 0 {
		t.Fatal("the guard read no source files")
	}
}

// paintedBox is the bounding box of every pixel in the frame that differs from
// page (the filler a view was drawn on): the shape a card's fill and edge make.
func paintedBox(img *image.RGBA, page ui.Color) ui.Rect {
	left, top := img.Rect.Dx(), img.Rect.Dy()
	right, bottom := -1, -1
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			if channelDelta(imagePixel(img, x, y), page) <= 4 {
				continue
			}
			if x < left {
				left = x
			}
			if x > right {
				right = x
			}
			if y < top {
				top = y
			}
			if y > bottom {
				bottom = y
			}
		}
	}
	if right < 0 {
		return ui.Rect{}
	}
	return ui.Rect{X: float32(left), Y: float32(top), W: float32(right - left + 1), H: float32(bottom - top + 1)}
}

// The P8-c pins for the settings surface: §6.3's four rows plus the two accent
// states §5 (as P8-c rewrote it) keeps legal. They are here rather than in the
// per-feature files because what they assert is the *contract*, not a widget.

// ⑥ §5 第 3 条 · The chosen segment of a segmented control carries the accent
// keyline. This is a **positive** pin: §5 was widened in P8-c from three
// places to four, and the third one — "a control in its chosen/on state" —
// had no test reading it before, so a later round could have stripped the
// keyline as "the old base.css language" with nothing to stop it.
func TestTheChosenSegmentKeepsItsAccentKeyline(t *testing.T) {
	a := New(newStore(t), Actions{})
	var tokens theme.Tokens
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(ui.RGB(255, 255, 255))
		ui.Column(c).Padding(20).Children(func() {
			// The segmented control returns nothing, so its box is read off a
			// wrapping row: the row sizes to the track (the track is its only
			// child and does not grow).
			ui.Row(c).Bind(&ref).Children(func() {
				a.segmented(c, []string{"System", "Light", "Dark"}, 1, func(int) {})
			})
		})
		box = c.Resolve(ref).Bounds()
	}, 600, 140)
	tt.Frame()

	// The chosen segment is the middle third of the track: its keyline is the
	// accent at 0.55, so the frame shows a warm run along its top edge.
	third := box.W / 3
	mid := int(box.X + box.W/2)
	// The keyline is a 1px inset on the chosen pane, i.e. a row or two inside
	// the track's own top edge (the track is padded).
	warmest, at := -999, ui.Rect{}
	for y := int(box.Y); y < int(box.Y)+5; y++ {
		for x := mid - int(third)/2; x < mid+int(third)/2; x++ {
			p := imagePixel(tt.Image(), x, y)
			if d := int(p.R) - int(p.B); d > warmest {
				warmest, at = d, ui.Rect{X: float32(x), Y: float32(y)}
			}
		}
	}
	if warmest < 40 {
		t.Fatalf("the chosen segment's own edge is %d levels warm (warmest at %v): the accent keyline is gone",
			warmest, at)
	}
	// And the *unchosen* segments' edges are not: the keyline is the chosen
	// state's, not the whole track's.
	for _, third := range []int{0, 2} {
		x0 := int(box.X) + third*int(box.W)/3 + 8
		x1 := int(box.X) + (third+1)*int(box.W)/3 - 8
		worst := -999
		for y := int(box.Y); y < int(box.Y)+5; y++ {
			for x := x0; x < x1; x++ {
				if d := int(imagePixel(tt.Image(), x, y).R) - int(imagePixel(tt.Image(), x, y).B); d > worst {
					worst = d
				}
			}
		}
		if worst > 20 {
			t.Errorf("an unchosen segment's edge is %d levels warm: the accent keyline spread to the whole track", worst)
		}
	}
}

// ⑦ §5 第 3 条 · A switch in its ON state is the accent. The other half of the
// widened §5: the ON fill is a *state*, not decoration, so it keeps the accent
// while a switch that is off wears the neutral track.
func TestTheSwitchOnStateCarriesTheAccent(t *testing.T) {
	a := New(newStore(t), Actions{})
	measure := func(on bool) (warmest int, tokens theme.Tokens) {
		t.Helper()
		var box ui.Rect
		var ref ui.Handle
		tt := ui.NewTester(func(c *ui.Context) {
			tokens = a.tokens(c)
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(255, 255, 255))
			ui.Column(c).Padding(20).Children(func() {
				value := on
				a.switchControl(c, &value).Bind(&ref)
			})
			box = c.Resolve(ref).Bounds()
		}, 300, 120)
		tt.Frame()
		warmest = -999
		for y := int(box.Y); y < int(box.Y+box.H); y++ {
			for x := int(box.X); x < int(box.X+box.W); x++ {
				p := imagePixel(tt.Image(), x, y)
				if d := int(p.R) - int(p.B); d > warmest {
					warmest = d
				}
			}
		}
		return warmest, tokens
	}
	onWarm, tokens := measure(true)
	offWarm, _ := measure(false)
	// The accent's own R−B (the red these tokens ship) is ~150; a grey track's
	// is a handful of levels.
	if warm := int(tokens.Theme.Accent.R) - int(tokens.Theme.Accent.B); warm < 100 {
		t.Fatalf("the accent is not the red this pin measures against (%d)", warm)
	}
	if onWarm < 100 {
		t.Errorf("the switch's ON state is %d levels warm (R−B): §5 第 3 条 keeps the accent here", onWarm)
	}
	if offWarm > 12 {
		t.Errorf("the switch's OFF state is %d levels warm (R−B): the accent belongs to the ON state alone", offWarm)
	}
}

// ⑧ §6.3 第 2 条 · The integration row a register link named wears the neutral
// `RowSelected` over the card — **no** accent bar and no accent keyline — and
// the accent is spent on its leading package mark instead.
//
// Mutation M3 (put `RaisedQuiet` + `AccentEdge` + the accent bar back) turns
// this red.
func TestTheRegisterRowIsTheNeutralSelectedRung(t *testing.T) {
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.reg", Name: "Register Me", Enabled: true,
		Highlighted: true, RuntimeAvailable: true,
	}, Actions{})
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "plain", Name: "Plain Tool", Enabled: true, RuntimeAvailable: true},
			{ID: "dev.floter.reg", Name: "Register Me", Enabled: true, Highlighted: true, RuntimeAvailable: true},
		}
	}
	tt, tokens := renderThemed(t, a, 720, 1400)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	img := tt.Image()

	// The register row's title and the plain row's, so the two planes can be
	// compared at the same offset inside each row.
	marked := mustFind(t, tt, "Register Me")
	plain := mustFind(t, tt, "Plain Tool")
	// The row's plane, read left of the title (past the 30u chip and its gap).
	at := func(r ui.Rect, dx int) ui.Color {
		return imagePixel(img, int(r.X)-dx, int(r.Y+r.H/2))
	}
	card := panelSurface(tt, plain)

	// The plane is `RowSelected` over the card: no lift to an opaque pane.
	got := at(marked, 6)
	want := panelOver(tokens.RowSelected, card)
	if !panelClose(got, want, 6) {
		t.Errorf("the register row's plane = %v, want RowSelected over the card (%v)", got, want)
	}
	// And it is *not* the old raised-quiet lift, which is an opaque pane and
	// therefore visibly brighter than any translucent wash over this card.
	if old := panelOver(tokens.RaisedQuiet.Alpha(0.99), card); panelClose(got, old, 3) {
		t.Errorf("the register row's plane = %v is the old RaisedQuiet pane (%v)", got, old)
	}
	// No accent anywhere on the row's plane: the warmest plane pixel (clear of
	// the title's own glyphs and the chip's accent mark) is neutral.
	warmest, at2 := -999, ui.Rect{}
	for y := int(marked.Y) - 4; y < int(marked.Y+marked.H)+4; y++ {
		for x := int(marked.X) - 8; x < int(marked.X+marked.W)+8; x++ {
			p := imagePixel(img, x, y)
			if d := int(p.R) - int(p.B); d > warmest {
				warmest, at2 = d, ui.Rect{X: float32(x), Y: float32(y)}
			}
		}
	}
	// The chip's package glyph sits left of the title and *does* carry the
	// accent (§5 第 1 条的同一处理), so the plane check stays on the row's
	// own field, right of the chip's column.
	planeWarmest := -999
	for y := int(marked.Y) - 4; y < int(marked.Y+marked.H)+4; y++ {
		for x := int(marked.X) - 2; x < int(marked.X+marked.W)+8; x++ {
			if d := int(imagePixel(img, x, y).R) - int(imagePixel(img, x, y).B); d > planeWarmest {
				planeWarmest = d
			}
		}
	}
	if planeWarmest > 30 {
		t.Errorf("a pixel of the register row's field is %d levels warm at %v: the accent plane/bar is back",
			planeWarmest, at2)
	}
	// The leading mark **is** the accent: the counter-pin, so a change that
	// stripped both would fail.
	warmest, markAt := -999, ui.Rect{}
	for y := int(marked.Y) - 8; y < int(marked.Y+marked.H)+8; y++ {
		for x := int(marked.X) - 50; x < int(marked.X)-8; x++ {
			p := imagePixel(img, x, y)
			if d := int(p.R) - int(p.B); d > warmest {
				warmest, markAt = d, ui.Rect{X: float32(x), Y: float32(y)}
			}
		}
	}
	if warmest < 40 {
		t.Errorf("the register row's leading package mark carries no accent (%d levels warm at %v)",
			warmest, markAt)
	}
}

// ⑨ §0(b) · The settings fields are the `Control` rung and **not** the `Field`
// rung: one field material per app, and the 0.85 light rim is gone.
//
// The pin reads the view's own `fieldFace` on both appearances: the face it
// paints is `Control` over the page, and the deprecated `Field` + `FieldSlot`
// stack — the one whose light rim is 21× §1's gloss cap — is not what appears.
func TestTheSettingsFieldsWearTheControlRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
		value := ""
		var ref ui.Handle
		var box ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(0, 0, 0))
			ui.Column(c).Padding(24).Children(func() {
				a.textInput(c, &value).Bind(&ref).Width(220)
			})
			box = c.Resolve(ref).Bounds()
		}, 400, 160)
		tt.Frame()
		img := tt.Image()

		page := ui.RGBA(0, 0, 0, 1)
		face := imagePixel(img, int(box.X+box.W/2), int(box.Y+box.H/2))
		if want := panelOver(tokens.Control, page); !panelClose(face, want, 3) {
			t.Errorf("[%s] the field's face = %v, want Control over the page (%v)", appearance, face, want)
		}
		if field := panelOver(tokens.Field, page); !panelClose(tokens.Field, tokens.Control, 3) &&
			panelClose(face, field, 3) {
			t.Errorf("[%s] the field's face = %v is the deprecated --glass-field rung (%v)", appearance, face, field)
		}
		// The 0.85 white rim the old slot painted on light glass: no row inside
		// the field's top edge may be that bright. `Control` + one hairline has
		// nothing above the face there.
		if !dark {
			for y := int(box.Y); y < int(box.Y)+3; y++ {
				for x := int(box.X) + 6; x < int(box.X+box.W)-6; x++ {
					if p := imagePixel(img, x, y); p.R > 200 && p.G > 200 && p.B > 200 {
						t.Fatalf("[light] the field's inner rim at (%d,%d) is %v: the 0.85 FieldSlot rim is back", x, y, p)
					}
				}
			}
		}
	}
}
