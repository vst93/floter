package settingsui

import (
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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

// renderIn draws the app in one appearance, with the resolved tokens installed
// as the context theme first: a pixel read then compares against the same
// numbers the view painted with.
//
// `dark` is set on the tester *before* the frame that is read, because that is
// the flag the view's own resolution reads (`a.tokens` → `c.Theme().Dark`,
// `rows.go:31`). It is the same path a window takes when the OS switches to the
// dark appearance with the store left on "system" — not a test-only theme swap.
func renderIn(t *testing.T, a *App, w, h int, dark bool) (*ui.Tester, theme.Tokens) {
	t.Helper()
	skipWithoutTheReferenceFont(t)
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		a.View(c)
	}, w, h)
	tt.SetDark(dark)
	// The rows carry a 120ms colour transition (`sidebarRow`, `rows.go`), so the
	// frame immediately after an appearance switch is still the *old* palette
	// easing out. Reading it would measure a mid-transition blend — in dark the
	// chosen row's fill came back as the light `RowSelected` (a black wash) for
	// exactly that reason. Advance past the transition, then read.
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	return tt, tokens
}

// renderThemed is the light appearance of renderIn, for the pins that are
// about a rule rather than about an appearance.
func renderThemed(t *testing.T, a *App, w, h int) (*ui.Tester, theme.Tokens) {
	t.Helper()
	return renderIn(t, a, w, h, false)
}

// subPage is the plane a sub-surface pin draws its one widget on. The light
// half has always been white — the pins' own stable reading plane; the dark
// half is the theme's dark base fill (`internal/theme/theme.go:489-494`), so a
// dark pin reads the widget against the plane it actually sits on instead of
// white. Both are opaque, which is what makes a 1px edge readable.
func subPage(dark bool) ui.Color {
	if dark {
		return ui.RGB(17, 18, 20)
	}
	return ui.RGB(255, 255, 255)
}

// renderOn is renderIn for a *sub-surface*: one widget drawn on a bare root
// rather than the whole page. `draw` installs the tokens itself (it is the only
// place that sees the context) and paints the widget; the frame read afterwards
// is already past the appearance switch, same as renderIn.
func renderOn(t *testing.T, w, h int, dark bool, a *App, draw func(c *ui.Context)) (*ui.Tester, theme.Tokens) {
	t.Helper()
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(subPage(dark))
		draw(c)
	}, w, h)
	tt.SetDark(dark)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	// P9-b's lesson, applied here: without this the dark half would silently
	// measure a light frame (the pins' expected values are computed from the
	// same tokens, so a wrong appearance stays green). `TestTheDarkAppearanceIs
	// ActuallyDark` is the page-level version of the same canary.
	if tokens.Theme.Dark != dark {
		t.Fatalf("renderOn resolved Dark=%v, want %v: the sub-surface pin would be reading the other appearance",
			tokens.Theme.Dark, dark)
	}
	return tt, tokens
}

// appearanceNames is the pair every appearance-aware pin runs, in the order the
// subtests are reported.
var appearanceNames = []struct {
	name string
	dark bool
}{{"light", false}, {"dark", true}}

// The canary for the eleven dark subtests below: if `SetDark` stopped reaching
// the view's token resolution — or if a pin read a frame drawn before it — the
// dark half would silently measure the light one and every threshold would
// still pass. This pin states that the two appearances are two different
// pictures, and that the dark one is dark.
//
// P9-b's lesson is the reason it exists: a guard that cannot fire looks exactly
// like a guard that passes.
func TestTheDarkAppearanceIsActuallyDark(t *testing.T) {
	pane := func(dark bool) ui.Color {
		a := New(newStore(t), Actions{})
		tt, tokens := renderIn(t, a, 720, 580, dark)
		if tokens.Theme.Dark != dark {
			t.Fatalf("[%v] the view resolved Dark=%v, want %v: renderIn did not switch the appearance",
				dark, tokens.Theme.Dark, dark)
		}
		return imagePixel(tt.Image(), 2, 2)
	}
	light, dark := pane(false), pane(true)
	if mean(dark) > 40 {
		t.Errorf("the dark pane is %v (mean %d): the dark subtests would be reading a lit surface", dark, mean(dark))
	}
	if mean(light) < 200 {
		t.Errorf("the light pane is %v (mean %d): the light baseline is not lit", light, mean(light))
	}
	if d := channelDelta(light, dark); d < 100 {
		t.Errorf("the two appearances' panes differ by only %d levels (%v vs %v): one of them is not being drawn",
			d, light, dark)
	}
}

// mean is a colour's average channel, used only to say "this surface is dark".
func mean(c ui.Color) int {
	return (int(c.R) + int(c.G) + int(c.B)) / 3
}

// §6.1 第 1 条 · The sidebar's chosen page is the **neutral** row-selection
// rung, not the old opaque raised pill, and it carries no lens.
//
// The pin reads the chosen row's own fill and compares it to
// `RowSelected` over the sidebar's pane — and then proves the fill is
// *neutral*, which is the whole point: the accent is spent on the leading
// glyph (see the pin below), never on the row's plane.
func TestTheSidebarSelectionIsTheNeutralRowSelected(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			tt, tokens := renderIn(t, a, 720, 580, dark)
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
		})
	}
}

// §5 第 1 条 · The leading glyph of the chosen page **is** the accent. This is
// the counter-pin to the test above: the row's fill lost the accent, the mark
// kept it, and a change that stripped both would fail here.
func TestTheSidebarChosenGlyphKeepsTheAccent(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			tt, tokens := renderIn(t, a, 720, 580, dark)
			img := tt.Image()
			chosen := mustFind(t, tt, "General")
			rest := mustFind(t, tt, "Sessions")
			// The mark sits in the row's leading 28u slot, left of the label.
			warmest := func(r ui.Rect) (ui.Color, int) {
				best, ink := -999, ui.Color{}
				for y := int(r.Y); y < int(r.Y+r.H); y++ {
					for x := int(r.X) + 6; x < int(r.X)+26; x++ {
						p := imagePixel(img, x, y)
						if d := int(p.B) - int(p.R); d > best {
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
		})
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
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			tt, tokens := renderIn(t, a, 720, 580, dark)
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
			// The guard below says "not the strong hairline"; it can only say that if
			// the two tiers are further apart than its own tolerance (they are: 8
			// levels over this pane).
			discriminating(t, "the header's rule", want, strong, 2)
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
		})
	}
}

// §6.1 第 3 条 · The grouped card is `Control` + `HairlineSoft` at radius 10 —
// no lens and no control-edge keyline.
//
// P9-b 重定基线：这条 pin 原来只画浅色外观，而且它的「不是 ControlEdge」守卫是死的——
// 写成 `!panelClose(weak, controlEdge, 2) && panelClose(edge, controlEdge, 3)`，可浅色下
// 两档发丝叠在卡片面上只差 **1/255**（229 vs 228），第一个条件恒假，守卫永远不开火
// （P9-a 的变异实验就是这么发现它的：把卡片边换成 ControlEdge，pin 依然绿）。
// 现在两种外观都画，守卫换成没有魔数容差的判别式：卡片边必须**离弱发丝比离控件边更近**。
// 深色下两档差 4/255（32 vs 36），浅色下差 1/255——判别式两种外观都能开火，
// 而任何 `panelClose(..., tol≥1)` 的写法在浅色下都必然失明。
func TestTheGroupedCardWearsTheContractsFaceEdgeAndCorner(t *testing.T) {
	a := New(newStore(t), Actions{})
	if tokens := theme.ForPreferences(a.Store.Snapshot(), false, ui.Preferences{}); tokens.RadiusMD != 10 {
		t.Errorf("the panel-block radius is %v, want the contract's 10 (RadiusMD)", tokens.RadiusMD)
	}
	for _, appearance := range []string{"light", "dark"} {
		t.Run(appearance, func(t *testing.T) {
			store := newStore(t)
			if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			// The page the card is painted on: white under the light card,
			// black under the dark one. Both are opaque, so a token composited
			// over them is readable as the card's own face.
			page0 := ui.RGB(255, 255, 255)
			if appearance == "dark" {
				page0 = ui.RGB(0, 0, 0)
			}
			var tokens theme.Tokens
			tt := ui.NewTester(func(c *ui.Context) {
				tokens = a.tokens(c)
				c.SetTheme(tokens.Theme)
				c.Root().Background(page0)
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
			// And it is *not* the ControlEdge tier. A magnitude check cannot say
			// this — the two tiers are one level apart over this face — so the
			// pin asks which of the two the edge is nearer to. The canary above
			// keeps the question answerable: if a future token change collapses
			// the two tiers onto one pixel, this pin has lost its subject and
			// says so instead of passing.
			controlEdge := panelOver(tokens.ControlEdge, cardFace)
			if channelDelta(weak, controlEdge) == 0 {
				t.Fatalf("HairlineSoft and ControlEdge both composite to %v over this card's face: this pin can no longer tell the two tiers apart", weak)
			}
			if channelDelta(edge, controlEdge) <= channelDelta(edge, weak) {
				t.Errorf("the card's edge = %v sits nearer the ControlEdge tier (%v) than the weak hairline (%v); §6.1 第 3 条 gives it the weak hairline",
					edge, controlEdge, weak)
			}
			// No lens: the card's top inner row is its face, not a lit rim.
			if inner := imagePixel(img, 200, edgeY+2); channelDelta(inner, face) > 4 {
				t.Errorf("the card still wears a lens: its top inner row %v is not its face %v", inner, face)
			}
		})
	}
}

// §6.1 第 7 条 · The permission pills are neutral — both tiers — and no pill
// carries an accent inset.
func TestThePermissionPillsAreNeutralAndAccentFree(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
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
			tt, _ := renderIn(t, a, 720, 1100, dark)
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
		})
	}
}

// §6.1 第 9 条 + §5 第 2 条 · The primary CTA is an accent *fill* with the
// contract's `RadiusSM` corner — an accent button, not an accent stadium.
func TestThePrimaryCTAIsAnAccentButtonAtRadiusSM(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			copy := i18n.For("en").Settings
			a := New(newStore(t), Actions{InstallUpdate: func() {}})
			a.About.Version = "0.3.14"
			a.Update = UpdateState{Checked: true, Available: true, Version: "9.9.9"}
			a.selectPage(PageAbout)
			tt, tokens := renderIn(t, a, 900, 700, dark)
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
		})
	}
}

// §6.1 第 6 条 · The overlay button's corner is `RadiusSM` at *every* interface
// size: a radius is literal px and does not follow `--ui-scale` (§2). The two
// steps below differ by 22% in their `Space()` unit, so a `Space(3.5)` corner
// cannot pass both.
func TestTheOverlayButtonCornerIsLiteralAtEveryStep(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			page := subPage(dark)
			cornerInset := func(step string) (float32, theme.Tokens) {
				t.Helper()
				store := stepStore(t, step)
				a := New(store, Actions{})
				var ref ui.Handle
				var box ui.Rect
				tt, tokens := renderOn(t, 400, 200, dark, a, func(c *ui.Context) {
					ui.Column(c).Padding(40).Children(func() {
						ui.Row(c).Children(func() {
							a.sheetActionButton(c, "Clear history", false, func() {}).Bind(&ref)
						})
					})
					box = c.Resolve(ref).Bounds()
				})
				img := tt.Image()
				left := -1
				for x := int(box.X) - 2; x < int(box.X+box.W)+2; x++ {
					// The page is not always white (P9-d): "not the page" is what marks
					// the button's first painted column, in either appearance.
					if p := imagePixel(img, x, int(box.Y)+1); channelDelta(p, page) > 3 {
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
		})
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
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			a.Integrations = func() []Integration {
				return []Integration{{ID: "dev.floter.bad", Name: "Bad Tool", Enabled: true, Health: "Degraded"}}
			}
			tt, tokens := renderIn(t, a, 720, 900, dark)
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
			// The chip's warm amber face, measured warm-to-red (R−B): it is not
			// the accent, so it keeps its own polarity after P11-al.
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
		})
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
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			var box ui.Rect
			var ref ui.Handle
			tt, _ := renderOn(t, 600, 140, dark, a, func(c *ui.Context) {
				ui.Column(c).Padding(20).Children(func() {
					// The segmented control returns nothing, so its box is read off a
					// wrapping row: the row sizes to the track (the track is its only
					// child and does not grow).
					ui.Row(c).Bind(&ref).Children(func() {
						a.segmented(c, []string{"System", "Light", "Dark"}, 1, func(int) {})
					})
				})
				box = c.Resolve(ref).Bounds()
			})

			// The chosen segment is the middle third of the track: its keyline is the
			// accent at 0.55, so the frame shows a warm run along its top edge. The
			// accent is the same red in both appearances, so the thresholds hold.
			third := box.W / 3
			mid := int(box.X + box.W/2)
			// The keyline is a 1px inset on the chosen pane, i.e. a row or two inside
			// the track's own top edge (the track is padded).
			warmest, at := -999, ui.Rect{}
			for y := int(box.Y); y < int(box.Y)+5; y++ {
				for x := mid - int(third)/2; x < mid+int(third)/2; x++ {
					p := imagePixel(tt.Image(), x, y)
					if d := int(p.B) - int(p.R); d > warmest {
						warmest, at = d, ui.Rect{X: float32(x), Y: float32(y)}
					}
				}
			}
			// P11-al 重定基线：旧阈值 40（品牌红 `--accent-edge` 0.55）。参考的
			// 蓝 keyline 是 `--accent-edge: 0.30`（深）/0.36（浅），压在深色轨道上
			// 只剩三十几级；实测深 36 / 浅 60。旧 40 → 新 24。
			if warmest < 24 {
				t.Fatalf("the chosen segment's own edge is %d levels accent-leaning (bluest at %v): the accent keyline is gone",
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
		})
	}
}

// ⑦ §5 第 3 条 · A switch in its ON state is the accent. The other half of the
// widened §5: the ON fill is a *state*, not decoration, so it keeps the accent
// while a switch that is off wears the neutral track.
func TestTheSwitchOnStateCarriesTheAccent(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			measure := func(on bool) (warmest int, tokens theme.Tokens) {
				t.Helper()
				var box ui.Rect
				var ref ui.Handle
				tt, tokens := renderOn(t, 300, 120, dark, a, func(c *ui.Context) {
					ui.Column(c).Padding(20).Children(func() {
						value := on
						a.switchControl(c, &value).Bind(&ref)
					})
					box = c.Resolve(ref).Bounds()
				})
				warmest = -999
				for y := int(box.Y); y < int(box.Y+box.H); y++ {
					for x := int(box.X); x < int(box.X+box.W); x++ {
						p := imagePixel(tt.Image(), x, y)
						if d := int(p.B) - int(p.R); d > warmest {
							warmest = d
						}
					}
				}
				return warmest, tokens
			}
			onWarm, tokens := measure(true)
			offWarm, _ := measure(false)
			// The accent's own R−B (the red these tokens ship) is ~150; a grey track's
			// is a handful of levels. Both appearances ship the same red, so the
			// thresholds hold on the dark track as well.
			if warm := int(tokens.Theme.Accent.B) - int(tokens.Theme.Accent.R); warm < 100 {
				t.Fatalf("the accent is not the red this pin measures against (%d)", warm)
			}
			if onWarm < 100 {
				t.Errorf("the switch's ON state is %d levels warm (R−B): §5 第 3 条 keeps the accent here", onWarm)
			}
			if offWarm > 12 {
				t.Errorf("the switch's OFF state is %d levels warm (R−B): the accent belongs to the ON state alone", offWarm)
			}
		})
	}
}

// ⑧ §6.3 第 2 条 · The integration row a register link named wears the neutral
// `RowSelected` over the card — **no** accent bar and no accent keyline — and
// the accent is spent on its leading package mark instead.
//
// Mutation M3 (put `RaisedQuiet` + `AccentEdge` + the accent bar back) turns
// this red.
func TestTheRegisterRowIsTheNeutralSelectedRung(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
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
			tt, tokens := renderIn(t, a, 720, 1400, dark)
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
					if d := int(p.B) - int(p.R); d > warmest {
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
					if d := int(p.B) - int(p.R); d > warmest {
						warmest, markAt = d, ui.Rect{X: float32(x), Y: float32(y)}
					}
				}
			}
			if warmest < 40 {
				t.Errorf("the register row's leading package mark carries no accent (%d levels warm at %v)",
					warmest, markAt)
			}
		})
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
		field := panelOver(tokens.Field, page)
		discriminating(t, "the field's face", field, panelOver(tokens.Control, page), 3)
		if !panelClose(tokens.Field, tokens.Control, 3) && panelClose(face, field, 3) {
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

// The P8-d pins: §5's one-line judgment ("accent can only answer which one is
// chosen / which one is on") read against the surfaces that had been spending
// it on something else. Each is written **正面 + 反面** in pairs: the accent is
// *not* there, and the neutral rung that replaced it still reads as a rung (a
// pin that only proves "not red" would pass on a blank frame).
//
// ⑩ §5 判据 · `statusInk` 的 `healthy` 是中性且比 quiet 档更亮；
// `error` 仍是 `Warning` 色相。正面 + 反面成对。
func TestStatusInkIsNeutralWhenHealthyAndWarningWhenNot(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Settings{Theme: appearance}, dark, ui.Preferences{})
		// The classifier needs the framework theme for the two semantic inks —
		// the same resolved one the view installs, not the framework's stock
		// palette (whose `Warning` is a different amber).
		var th *ui.Theme
		probe := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			th = c.Theme()
		}, 10, 10)
		probe.SetDark(dark)
		probe.Frame()

		for _, quiet := range []ui.Color{tokens.TextSecondary, tokens.TextTertiary} {
			for _, word := range []string{"ok", "Ok", " HEALTHY "} {
				got := statusInk(word, tokens, th, quiet)
				// 反面：不是 accent（accent 自身是 ~150 级暖）。
				if d := int(got.B) - int(got.R); d > 14 || d < -14 {
					t.Errorf("[%s] statusInk(%q) = %v carries a hue (R−B %d): the accent is back",
						appearance, word, got, d)
				}
				if channelDelta(got, tokens.Theme.Accent) < 60 {
					t.Errorf("[%s] statusInk(%q) = %v is the accent %v", appearance, word, got, tokens.Theme.Accent)
				}
				// 正面：它是 strong 档，而且**比 caller 的 quiet 档更亮**
				// （浅色下 TextStrong 比 TextTertiary 更暗，所以比的是「离
				// 面的距离」——两档的对比度，而不是通道大小）。
				if channelDelta(got, tokens.TextStrong) > 12 {
					t.Errorf("[%s] statusInk(%q) = %v, want the strong text rung %v",
						appearance, word, got, tokens.TextStrong)
				}
				if contrast(got) <= contrast(quiet) {
					t.Errorf("[%s] statusInk(%q) = %v (contrast %d) is not above its quiet fallback %v (contrast %d): "+
						"the healthy word must still outrank the row it sits in",
						appearance, word, got, contrast(got), quiet, contrast(quiet))
				}
			}
		}
		// The error side keeps the semantic colour: a warning *is* a warning.
		for _, word := range []string{"error", "broken", "BROKEN"} {
			if got := statusInk(word, tokens, th, tokens.TextTertiary); got != tokens.Theme.Warning {
				t.Errorf("[%s] statusInk(%q) = %v, want the warning ink %v",
					appearance, word, got, tokens.Theme.Warning)
			}
		}
		// And the fallback is still the caller's own quiet rung, untouched.
		for _, word := range []string{"unknown", "", "pending"} {
			if got := statusInk(word, tokens, th, tokens.TextTertiary); got != tokens.TextTertiary {
				t.Errorf("[%s] statusInk(%q) = %v, want the caller's quiet rung %v",
					appearance, word, got, tokens.TextTertiary)
			}
		}
	}
}

// contrast is an ink's distance from mid-grey: a coarse stand-in for "how much
// this ink stands off the plane", used to order two neutral rungs without
// assuming which way round the appearance goes.
func contrast(c ui.Color) int {
	mean := (int(c.R) + int(c.G) + int(c.B)) / 3
	if mean > 127 {
		return mean - 127
	}
	return 127 - mean
}

// ⑪ §5 判据 · 集成页「运行中」的状态字是中性，且比 `TextTertiary` 亮。
//
// The chip's plane is read as the row's own fill and the word's ink as the
// pixel of its box farthest from that fill — the glyph's own colour, not an
// antialiased edge.
func TestTheRunningStateMarkIsTheNeutralStrongRung(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			copy := i18n.For("en").Settings
			a := New(newStore(t), Actions{})
			a.Integrations = func() []Integration {
				return []Integration{
					{ID: "dev.floter.on", Name: "Plain Tool", Enabled: true, RuntimeAvailable: true},
					{ID: "dev.floter.off", Name: "Off Tool", Enabled: false, RuntimeAvailable: true},
				}
			}
			tt, tokens := renderIn(t, a, 720, 1200, dark)
			if err := tt.Click("Integrations"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			running := mustFind(t, tt, copy.IntegrationsRunning)
			off := mustFind(t, tt, copy.IntegrationsOff)
			runningInk, offInk := inkColor(tt, running), inkColor(tt, off)
			// 反面：不是 accent。
			if d := int(runningInk.B) - int(runningInk.R); d > 14 || d < -14 {
				t.Errorf("the running word %v carries a hue (R−B %d): the accent is back", runningInk, d)
			}
			if channelDelta(runningInk, tokens.Theme.Accent) < 60 {
				t.Errorf("the running word %v is the accent %v", runningInk, tokens.Theme.Accent)
			}
			// 正面：strong 档，且**比 tertiary 那档更亮**（这里比的是同一页面上
			// 两档中性墨的对比度：运行中 vs 关）。
			if channelDelta(runningInk, tokens.TextStrong) > 16 {
				t.Errorf("the running word %v, want the strong rung %v", runningInk, tokens.TextStrong)
			}
			if contrast(runningInk) <= contrast(tokens.TextTertiary) {
				t.Errorf("the running word %v (contrast %d) is not above TextTertiary %v (contrast %d)",
					runningInk, contrast(runningInk), tokens.TextTertiary, contrast(tokens.TextTertiary))
			}
			// The "off" word is the other neutral rung, and the two must stay apart —
			// §5's new sentence says the neutral ladder is *encouraged* to separate by
			// lightness. A change that flattened both to one ink fails here.
			if channelDelta(runningInk, offInk) < 20 {
				t.Errorf("running %v and off %v read at one level: the neutral ladder collapsed",
					runningInk, offInk)
			}
		})
	}
}

// ⑫ §5 判据 · 会话 meta 的「运行中」段是中性（不是 accent）。
func TestTheSessionRunningSegmentIsNeutral(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{SelectSession: func(int) {}})
			a.Sessions = func() []Session {
				created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
				return []Session{{
					ID: 1, Title: "zsh", Running: true, Index: 0,
					Note: "Running · attached", Size: "80×24", Created: created,
				}}
			}
			tt, tokens := renderIn(t, a, 720, 760, dark)
			if err := tt.Click("Sessions"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			state := inkColor(tt, mustFind(t, tt, "Running · attached"))
			size := inkColor(tt, mustFind(t, tt, "80×24"))
			// 反面：不是 accent。
			if d := int(state.B) - int(state.R); d > 14 || d < -14 {
				t.Errorf("the running state word %v carries a hue (R−B %d): the accent is back", state, d)
			}
			if channelDelta(state, tokens.Theme.Accent) < 60 {
				t.Errorf("the running state word %v is the accent %v", state, tokens.Theme.Accent)
			}
			// 正面：strong 档，并且仍然与它右边的 size 段分得开（明度分层）。
			if channelDelta(state, tokens.TextStrong) > 16 {
				t.Errorf("the running state word %v, want the strong rung %v", state, tokens.TextStrong)
			}
			if channelDelta(state, size) < 20 {
				t.Errorf("the running state word %v and its size neighbour %v read at one level", state, size)
			}
		})
	}
}

// ⑬ §5 判据 · reset 行的图标与文字静息中性；hover 后**仍然**中性（只变亮，
// 不变色相）。The action is drawn on its own so both frames are readable
// without the surrounding page.
func TestTheResetActionIsNeutralAtRestAndOnHover(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			page := subPage(dark)
			a := New(newStore(t), Actions{})
			var ref ui.Handle
			var box ui.Rect
			tt, tokens := renderOn(t, 400, 120, dark, a, func(c *ui.Context) {
				ui.Row(c).Padding(20).Children(func() {
					a.actionWithGlyph(c, "plus", "Add one", func() {}).Bind(&ref)
				})
				box = c.Resolve(ref).Bounds()
			})

			warmest := func() (ui.Color, int) {
				img := tt.Image()
				var worst ui.Color
				best := -999
				for y := int(box.Y) - 2; y < int(box.Y+box.H)+2; y++ {
					for x := int(box.X) - 2; x < int(box.X+box.W)+2; x++ {
						p := imagePixel(img, x, y)
						if d := int(p.B) - int(p.R); d > best {
							best, worst = d, p
						}
					}
				}
				return worst, best
			}

			restBox := box
			_, restWarm := warmest()
			restInk := inkColor(tt, mustFind(t, tt, "Add one"))
			// 反面：静息时图标与文字都不带色相。
			if restWarm > 20 {
				t.Fatalf("the resting action is %d levels warm: the accent is on its icon or its word", restWarm)
			}
			if channelDelta(restInk, tokens.Theme.Accent) < 60 {
				t.Fatalf("the resting action's ink %v is the accent %v", restInk, tokens.Theme.Accent)
			}
			// 正面：静息是 secondary 档。
			if channelDelta(restInk, tokens.TextSecondary) > 16 {
				t.Errorf("the resting action's ink = %v, want the secondary rung %v", restInk, tokens.TextSecondary)
			}

			tt.Move(restBox.X+restBox.W/2, restBox.Y+restBox.H/2)
			tt.Frame()
			_, hoverWarm := warmest()
			hoverInk := inkColor(tt, mustFind(t, tt, "Add one"))
			// 反面：hover 也没把色相抬起来。
			if hoverWarm > 20 {
				t.Errorf("the hovered action is %d levels warm: hover raised the accent", hoverWarm)
			}
			if channelDelta(hoverInk, tokens.Theme.Accent) < 60 {
				t.Errorf("the hovered action's ink %v is the accent %v", hoverInk, tokens.Theme.Accent)
			}
			// 正面：hover 让墨更重一档（明度，不是色相）。
			if contrast(hoverInk) <= contrast(restInk) {
				t.Errorf("hover did not lift the action's ink: %v (contrast %d) → %v (contrast %d)",
					restInk, contrast(restInk), hoverInk, contrast(hoverInk))
			}
			// Faded by the reference's own `opacity: .78`, the primary rung composites
			// to a specific grey — read it rather than the raw token. The plane is the
			// appearance's own page (P9-d), not always white.
			if want := panelOver(tokens.Theme.Text.Alpha(0.78), page); channelDelta(hoverInk, want) > 16 {
				t.Errorf("the hovered action's ink = %v, want the primary rung at .78 (%v)", hoverInk, want)
			}
		})
	}
}

// ⑭ §5 禁止侧「行 hover/按下」· `cardRow` 的 hover 帧里**没有**任何 accent
// 像素（扫整行，不只是前导条那一列）。反面 + 正面成对：行面确实抬起来了。
func TestTheCardRowHoverCarriesNoAccentPixel(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			var labelRef ui.Handle
			var labelBox ui.Rect
			tt, _ := renderOn(t, 480, 120, dark, a, func(c *ui.Context) {
				a.cardRows = 0
				a.cardRow(c, ui.Center, nil, rowFace{}, func() {}, func() {
					ui.Text(c, "V Tools").Bind(&labelRef)
				})
				labelBox = c.Resolve(labelRef).Bounds()
			})

			restPlane := sample(tt, labelBox.X+labelBox.W+20, labelBox.Y+labelBox.H/2)
			tt.Move(labelBox.X+labelBox.W/2, labelBox.Y+labelBox.H/2)
			tt.Frame()
			hoverPlane := sample(tt, labelBox.X+labelBox.W+20, labelBox.Y+labelBox.H/2)

			// The whole row, from 60u left of the label (past the leading rule slot) to
			// well right of it, is scanned: a bar painted anywhere on hover is caught.
			img := tt.Image()
			warmest, at := -999, ui.Rect{}
			for y := int(labelBox.Y) - 6; y < int(labelBox.Y+labelBox.H)+6; y++ {
				for x := int(labelBox.X) - 60; x < int(labelBox.X+labelBox.W)+40; x++ {
					if x < 0 || y < 0 {
						continue
					}
					p := imagePixel(img, x, y)
					if d := int(p.B) - int(p.R); d > warmest {
						warmest, at = d, ui.Rect{X: float32(x), Y: float32(y)}
					}
				}
			}
			if warmest > 20 {
				t.Errorf("the hovered row carries an accent pixel at %v (%d levels warm): the hover bar is back",
					at, warmest)
			}
			// 正面：hover 确实抬了面——扫到的平面比静息更亮（深色）或更暗（浅色），
			// 也就是「只抬面」这件事真的发生了，而不是整行没反应。
			if channelDelta(hoverPlane, restPlane) < 3 {
				t.Errorf("hover moved nothing: plane %v → %v; the row must still lift its face", restPlane, hoverPlane)
			}
		})
	}
}

// ⑮ §5 判据 · 导入成功提示是中性（成功是结果语义，不是选中）。
func TestTheImportSummaryIsNeutral(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			a.TransferReport = &ImportReport{Path: "/tmp/backup.json", Succeeded: 4, Skipped: 1}
			tt, tokens := renderOn(t, 600, 300, dark, a, func(c *ui.Context) {
				a.transferReport(c, i18n.For("en").Settings)
			})
			summary := mustFind(t, tt, "Imported 4, unchanged 1")
			ink := inkColor(tt, summary)
			// 反面：不是 accent。
			if d := int(ink.B) - int(ink.R); d > 14 || d < -14 {
				t.Errorf("the import summary %v carries a hue (R−B %d): the accent is back", ink, d)
			}
			if channelDelta(ink, tokens.Theme.Accent) < 60 {
				t.Errorf("the import summary %v is the accent %v", ink, tokens.Theme.Accent)
			}
			// 正面：strong 档（与同一块里失败行的 `t.Warning` 判然不同）。
			if channelDelta(ink, tokens.TextStrong) > 16 {
				t.Errorf("the import summary = %v, want the strong rung %v", ink, tokens.TextStrong)
			}

			// The failure line keeps the semantic colour — the other half of the pair.
			a.TransferReport = &ImportReport{
				Path:   "/tmp/backup.json",
				Failed: []TransferFailure{{ID: "a", Reason: "no package"}},
			}
			tt.Frame()
			if !tt.HasText("Failed: a: no package") {
				t.Fatalf("the failure line is missing: %q", tt.Texts())
			}
			failure := mustFind(t, tt, "Failed: a: no package")
			// The dark appearance's warning is a straight-alpha colour (0.82), so the
			// pixel the renderer leaves is that ink over the plane, not the raw token.
			// P9-d re-baseline: light compared to the raw token (opaque there); dark
			// compares to the token composited over its own page.
			wantFailure := panelOver(tokens.Theme.Warning, subPage(dark))
			if got := inkColor(tt, failure); channelDelta(got, wantFailure) > 24 {
				t.Errorf("the failure detail = %v, want the warning ink %v over the page (%v)",
					got, tokens.Theme.Warning, wantFailure)
			}
		})
	}
}

// ⑯ §5 禁止侧 · 卸载弹层的危险按钮 hover 边是 `ControlEdge`（不是 accent），
// **但**墨仍是 `t.Warning`（危险语义没丢）；勾选卡 hover 边同理。
//
// The pill and the card are drawn on their own over a white page so the 1px
// edge is a stable pixel. A control's border composites over its *own* plane
// (`Control` rests, `ControlHover` under the pointer), so the expected edge is
// the border token over that plane, not over the page.
func TestTheDangerPillHoverKeepsTheNeutralEdgeAndTheWarningInk(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			page := subPage(dark)
			a := New(newStore(t), Actions{})
			var pillRef, cardRef ui.Handle
			var pillBox, cardBox ui.Rect
			off := false
			part := uninstallPart{Label: "Host configuration", Hint: "Remove it.", Value: &off}
			tt, tokens := renderOn(t, 520, 200, dark, a, func(c *ui.Context) {
				ui.Column(c).Padding(20).Gap(20).Children(func() {
					ui.Row(c).Children(func() {
						a.uninstallActionButton(c, "Uninstall", true, false, checkIcon).Bind(&pillRef)
					})
					a.uninstallPartRow(c, part).Bind(&cardRef)
				})
				pillBox = c.Resolve(pillRef).Bounds()
				cardBox = c.Resolve(cardRef).Bounds()
			})

			// topEdge is the 1px border at the box's mid-span, clear of the rounding.
			topEdge := func(box ui.Rect) ui.Color {
				return imagePixel(tt.Image(), int(box.X+box.W/2), int(box.Y))
			}
			// nearest reports the closest pixel of a box's *interior* (2px in from the
			// border) to a target colour, so a border never counts as ink.
			nearest := func(box ui.Rect, target ui.Color) (ui.Color, int) {
				img := tt.Image()
				var best ui.Color
				near := 1 << 30
				for y := int(box.Y) + 2; y < int(box.Y+box.H)-2; y++ {
					for x := int(box.X) + 2; x < int(box.X+box.W)-2; x++ {
						p := imagePixel(img, x, y)
						if d := channelDelta(p, target); d < near {
							near, best = d, p
						}
					}
				}
				return best, near
			}
			// The accent is ~99 levels from the warning ink on every channel, so a
			// "closer to the warning than to the accent" read is unambiguous.
			accentFree := func(name string, box ui.Rect) {
				if _, near := nearest(box, tokens.Theme.Accent); near < 30 {
					t.Errorf("%s carries a pixel within %d levels of the accent %v: the accent is back",
						name, near, tokens.Theme.Accent)
				}
			}

			// Rest: the edge is `ControlEdge` over the resting `Control` plane.
			restFace := panelOver(tokens.Control, page)
			if got, want := topEdge(pillBox), panelOver(tokens.ControlEdge, restFace); !panelClose(got, want, 6) {
				t.Errorf("the danger pill's resting edge = %v, want ControlEdge over Control (%v)", got, want)
			}
			accentFree("the resting danger pill", pillBox)

			tt.Move(pillBox.X+pillBox.W/2, pillBox.Y+pillBox.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)

			// 反面：hover 边不是 accent（AccentEdge over the same plane is a pink
			// ring, ~{240 150 152}).
			hoverEdge := topEdge(pillBox)
			hoverFace := panelOver(tokens.ControlHover, page)
			if warm := int(hoverEdge.B) - int(hoverEdge.R); warm > 20 {
				t.Errorf("the danger pill's hover edge %v is %d levels warm: the accent ring is back", hoverEdge, warm)
			}
			if channelDelta(hoverEdge, panelOver(tokens.AccentEdge, hoverFace)) < 12 {
				t.Errorf("the danger pill's hover edge %v is the old AccentEdge", hoverEdge)
			}
			// 正面：它仍是 `ControlEdge` 那一档——hover 只抬面，不抬边。
			if want := panelOver(tokens.ControlEdge, hoverFace); !panelClose(hoverEdge, want, 6) {
				t.Errorf("the danger pill's hover edge = %v, want the neutral ControlEdge over the hover plane (%v)",
					hoverEdge, want)
			}
			// 墨仍是 warning：危险语义留在墨上，不靠 hover 变色相。深色下 warning 是
			// 0.82 的直通 alpha（`theme.go:752`），落在药丸自己的 hover 面上，所以
			// 期望值是该墨合成到 hoverFace 之后的颜色（P9-d 重定基线：浅色 warning 不透明，
			// 合成后与原始 token 相同，判据不变）。
			wantWarning := panelOver(tokens.Theme.Warning, hoverFace)
			if _, near := nearest(pillBox, wantWarning); near > 24 {
				t.Errorf("the hovered danger pill has no warning ink (nearest pixel is %d levels off %v over the hover face): "+
					"hover replaced the danger semantic", near, wantWarning)
			}
			accentFree("the hovered danger pill", pillBox)

			// The check card, the same treatment: its border is `ControlEdge` at rest
			// and under the pointer.
			tt.Move(0, 0)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			if got, want := topEdge(cardBox), panelOver(tokens.ControlEdge, restFace); !panelClose(got, want, 6) {
				t.Errorf("the uninstall card's resting edge = %v, want ControlEdge over Control (%v)", got, want)
			}
			tt.Move(cardBox.X+cardBox.W/2, cardBox.Y+cardBox.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			cardEdge := topEdge(cardBox)
			if warm := int(cardEdge.B) - int(cardEdge.R); warm > 20 {
				t.Errorf("the hovered uninstall card's edge %v is %d levels warm: the accent edge is back", cardEdge, warm)
			}
			if channelDelta(cardEdge, panelOver(tokens.AccentEdge, hoverFace)) < 12 {
				t.Errorf("the hovered uninstall card's edge %v is the old AccentEdge", cardEdge)
			}
			if want := panelOver(tokens.ControlEdge, hoverFace); !panelClose(cardEdge, want, 6) {
				t.Errorf("the hovered uninstall card's edge = %v, want the neutral ControlEdge (%v)", cardEdge, want)
			}
			accentFree("the hovered uninstall card", cardBox)
		})
	}
}

// ⑰ §5 判据 · 会话可恢复红点改中性。
//
// The dot is 5u in the resume button's own top-right corner; its centre is read
// from the same three tokens the view uses, so the probe cannot drift from the
// geometry it is measuring.
func TestTheSessionResumeDotIsNeutral(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{SelectSession: func(int) {}})
			a.Sessions = func() []Session {
				created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
				return []Session{{
					ID: 1, Title: "zsh", Running: true, Index: 0,
					Note: "Running", Size: "80×24", Created: created,
				}}
			}
			tt, tokens := renderIn(t, a, 720, 760, dark)
			if err := tt.Click("Sessions"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			btn := mustFind(t, tt, i18n.For("en").Settings.SessionsResume)
			img := tt.Image()
			// `.session-manager__resume-dot`: Top/Right 0.75u, 1.25u square.
			cx := btn.X + btn.W - tokens.Theme.Space(0.75) - tokens.Theme.Space(1.25)/2
			cy := btn.Y + tokens.Theme.Space(0.75) + tokens.Theme.Space(1.25)/2
			got := imagePixel(img, int(cx), int(cy))
			// 反面：不是 accent。
			if d := int(got.B) - int(got.R); d > 14 || d < -14 {
				t.Errorf("the resume dot %v carries a hue (R−B %d): the accent is back", got, d)
			}
			if channelDelta(got, tokens.Theme.Accent) < 60 {
				t.Errorf("the resume dot %v is the accent %v", got, tokens.Theme.Accent)
			}
			// 正面：它是 secondary 那一档中性墨（颜色值本身就读得出来）。
			if channelDelta(got, tokens.TextSecondary) > 12 {
				t.Errorf("the resume dot = %v, want the secondary neutral rung %v", got, tokens.TextSecondary)
			}
		})
	}
}

// ⑱ 守卫（反证）· 权限 ● 与 `packageChip` 的前导图标**仍然**是 accent ——
// 这两条防「一刀切把 accent 全删了」。
//
// §5 注脚第 1 条与第 4 条已把这两处定审为合法：前者是「已强制」的 ON/OFF
// 画法（实心/空心即 ON/OFF），后者是「选中项的前导图标」与设置侧栏同构。
func TestThePermissionDotAndTheRegisterChipKeepTheAccent(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			a := New(newStore(t), Actions{})
			var tokens theme.Tokens
			tt := ui.NewTester(func(c *ui.Context) {
				tokens = a.tokens(c)
				c.SetTheme(tokens.Theme)
				a.card(c, func() {
					a.permissionTier(c, "Floter enforces", "hint", true, "en", []string{"clipboard-write"})
					a.permissionTier(c, "Declared", "hint2", false, "en", []string{"filesystem-read"})
				})
			}, 660, 420)
			tt.Frame()

			enforced := inkColor(tt, mustFind(t, tt, "\u25cf"))
			declared := inkColor(tt, mustFind(t, tt, "\u25cb"))
			// 正面：实心点就是 accent。
			if channelDelta(enforced, tokens.Theme.Accent) > 24 {
				t.Errorf("the enforced permission dot %v is not the accent %v (§5 注脚第 1 条)", enforced, tokens.Theme.Accent)
			}
			if d := int(enforced.B) - int(enforced.R); d < 100 {
				t.Errorf("the enforced permission dot %v is only %d levels warm; the accent is ~152", enforced, d)
			}
			// 反面（同一对的另一半）：空心点不是 accent —— 「实心/空心即 ON/OFF」
			// 这个读法要求两者判然分开，否则 ● 的 accent 也就没有意义了。
			if d := int(declared.B) - int(declared.R); d > 14 || d < -14 {
				t.Errorf("the declared permission ring %v carries a hue (R−B %d): the ON/OFF split is gone", declared, d)
			}

			// The register row's leading package chip, the other guarded accent.
			reg := New(newStore(t), Actions{})
			reg.Integrations = func() []Integration {
				return []Integration{{ID: "dev.floter.reg", Name: "Register Me", Enabled: true,
					Highlighted: true, RuntimeAvailable: true}}
			}
			rtt, rtokens := renderIn(t, reg, 720, 1000, dark)
			if err := rtt.Click("Integrations"); err != nil {
				t.Fatal(err)
			}
			rtt.Frame()
			title := mustFind(t, rtt, "Register Me")
			img := rtt.Image()
			warmest, at := -999, ui.Rect{}
			for y := int(title.Y) - 8; y < int(title.Y+title.H)+8; y++ {
				for x := int(title.X) - 50; x < int(title.X)-8; x++ {
					p := imagePixel(img, x, y)
					if d := int(p.B) - int(p.R); d > warmest {
						warmest, at = d, ui.Rect{X: float32(x), Y: float32(y)}
					}
				}
			}
			if warmest < 40 {
				t.Errorf("the register row's leading package mark carries no accent (%d levels warm at %v); "+
					"§5 注脚第 4 条 keeps it", warmest, at)
			}
			if channelDelta(rtokens.Theme.Accent, rtokens.RowSelected) == 0 {
				t.Fatal("the accent and the row rung coincide: the probe cannot tell them apart")
			}
		})
	}
}
