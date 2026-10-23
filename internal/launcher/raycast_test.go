package launcher

import (
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The P8-a pins: the Raycast contract's own numbers, checked against the tokens
// and against what the launcher actually paints (`docs/raycast-design.md`).
//
// This file is the *new* baseline. The nine tests the round before this one
// still carried were pinned to the old base.css language (an accent-tinted
// selection, a 30u text action slot, a raised footer pane, a 4u gutter); those
// were re-pinned in place, and these are the facts that had no pin at all.

// §4.1 / §2 · The card is the design's own: radius 12, the strong hairline, two
// casts (ambient and contact), and every gloss rung capped at 0.04.
func TestTheCardWearsTheContractsRadiusEdgeAndCast(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			tk := theme.ForPreferences(settings.Settings{Theme: "auto"}, appearance.dark, ui.Preferences{})
			if tk.WindowRadius != float32(theme.RadiusLG) {
				t.Errorf("the window radius is %v, want the contract's 12 (RadiusLG %v)",
					tk.WindowRadius, float32(theme.RadiusLG))
			}
			if theme.RadiusLG != 12 || theme.RadiusMD != 10 || theme.RadiusSM != 8 ||
				theme.RadiusXS != 6 || theme.KeycapRadius != 5 {
				t.Errorf("the ladder is %v/%v/%v/%v + keycap %v, want 12/10/8/6 + 5",
					theme.RadiusLG, theme.RadiusMD, theme.RadiusSM, theme.RadiusXS, theme.KeycapRadius)
			}
			// The card's edge: one strong hairline, and it is the same value the
			// input stroke carries (a card's border and a section rule are one
			// hairline at two widths, §1).
			if tk.InputStroke != tk.Hairline {
				t.Errorf("the card's edge %v is not the strong hairline %v", tk.InputStroke, tk.Hairline)
			}
			if tk.InputStroke.A == 0 || tk.InputStroke.A >= 64 {
				t.Errorf("the card's edge alpha is %d, want a hairline (0 < a < 0.25)", tk.InputStroke.A)
			}
			// The two casts (contract §4.1: `0 18px 48px` + `0 1px 2px`): a wide,
			// diffuse ambient one and a tight contact one, both present and both
			// a real cast (not a hairline). The ambient is the denser of the two
			// because it is the one that is spread out; the contact is the tight
			// dark line under the card's own edge.
			if tk.ShadowAmbient.A == 0 || tk.ShadowContact.A == 0 {
				t.Errorf("a cast is missing: ambient %v, contact %v", tk.ShadowAmbient, tk.ShadowContact)
			}
			if tk.ShadowContact.A < 16 {
				t.Errorf("the contact cast is alpha %d, too faint to be a cast", tk.ShadowContact.A)
			}
			if tk.ShadowAmbient == tk.ShadowContact {
				t.Errorf("the two casts are one value %v; the contract names two", tk.ShadowAmbient)
			}
			// Low gloss: the whole lens family — the control's rim, sheen and
			// shade, and the card's own sheen — stays at or under 0.04. That cap
			// is the line between Raycast and the liquid-glass language it
			// replaced (§1).
			const glossCap uint8 = 10 // 0.04 of 255, rounded down
			for _, rung := range []struct {
				name string
				c    ui.Color
			}{
				{"LensRim", tk.LensRim}, {"LensSheen", tk.LensSheen}, {"LensEdge", tk.LensEdge},
				{"SheenTop", tk.SheenTop}, {"SheenBottom", tk.SheenBottom},
			} {
				if rung.c.A > glossCap+1 {
					t.Errorf("%s is alpha %d, above the low-gloss cap %d (0.04)", rung.name, rung.c.A, glossCap)
				}
			}
		})
	}
}

// §4.3 · The row's three states are neutral and strictly ordered, and the
// accent never appears in the list.
func TestTheRowLadderIsNeutralAndOrdered(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
		hint struct{ hover, sel, press uint8 }
	}{
		// The contract's own numbers (§1): dark 0.045/0.085/0.11, light
		// 0.04/0.06/0.08.
		{"dark", true, struct{ hover, sel, press uint8 }{11, 22, 28}},
		{"light", false, struct{ hover, sel, press uint8 }{10, 15, 20}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tk := theme.ForPreferences(settings.Settings{Theme: "auto"}, tc.dark, ui.Preferences{})
			rungs := []struct {
				name string
				c    ui.Color
				want uint8
			}{
				{"RowHover", tk.RowHover, tc.hint.hover},
				{"RowSelected", tk.RowSelected, tc.hint.sel},
				{"RowPressed", tk.RowPressed, tc.hint.press},
			}
			for _, rung := range rungs {
				if rung.c.A != rung.want {
					t.Errorf("%s alpha = %d, want the contract's %d", rung.name, rung.c.A, rung.want)
				}
				// Neutral: the fill's own channels are equal, so the plane cannot
				// carry a hue.
				if rung.c.R != rung.c.G || rung.c.G != rung.c.B {
					t.Errorf("%s = %v is not neutral (its channels differ)", rung.name, rung.c)
				}
			}
			if !(tk.RowHover.A < tk.RowSelected.A && tk.RowSelected.A < tk.RowPressed.A) {
				t.Errorf("the row ladder is not strictly ordered: hover %d, selected %d, pressed %d",
					tk.RowHover.A, tk.RowSelected.A, tk.RowPressed.A)
			}
		})
	}
}

// §4.3 + §2 · The row block is inset 8u on both sides and draws an 8px corner,
// and the text column it lands on is still the field's 54u one.
func TestTheRowBlockIsTheInsetRoundedPill(t *testing.T) {
	forEachAppearance(t, rowBlockIsTheInsetRoundedPill)
}

func rowBlockIsTheInsetRoundedPill(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	tt.Frame()
	img := tt.Image()
	row, ok := tt.Find("Restart the computer")
	if !ok {
		t.Fatalf("the first row was not drawn: %q", tt.Texts())
	}
	gutter := int(ui.LightTheme().Space(2)) // 8u
	y := int(row.Y + row.H/2)
	panel := img.RGBAAt(1, y)
	left, right := -1, -1
	for x := 0; x < InputWindowWidth; x++ {
		if img.RGBAAt(x, y) != panel {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	if left != gutter {
		t.Errorf("the row block starts at x=%d, want the 8u gutter (x=%d)", left, gutter)
	}
	if want := InputWindowWidth - gutter - 1; right != want {
		t.Errorf("the row block ends at x=%d, want the mirrored 8u gutter (x=%d)", right, want)
	}
	// The pill's own top-left corner is an arc: at the block's first row the
	// gutter column still shows the panel, and only a few rows down does the
	// fill reach it. A square corner would paint the gutter at y=0 of the block.
	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	if tokens.RadiusSM != 8 {
		t.Errorf("the row's corner token is %v, want the contract's 8 (RadiusSM)", tokens.RadiusSM)
	}
	blockTop := int(row.Y - row.H/2)
	if blockTop < 0 {
		blockTop = 0
	}
	if c := img.RGBAAt(left, blockTop); c == over(tokens.RowSelected, panel) {
		t.Errorf("the row block is square at its top-left corner (y=%d): %v is the fill at the very edge",
			blockTop, c)
	}
	// The text column did not move: the title still starts on the field's 54u
	// from the card's content edge (8u block + 8u padding + 28u icon + 10u gap).
	if want := float32(54) * float32(ui.LightTheme().Spacing/4); row.X < want-1.5 || row.X > want+1.5 {
		t.Errorf("the title starts at x=%.1f, want the reference's 54u (%.1f)", row.X, want)
	}
}

// §4.5 · A keycap is a 20u-tall plate with a 5px corner, the keycap face and
// its hairline; a chord is one plate per key with a 2u seam and no `+`.
func TestTheKeycapIsTheContractsPlate(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			tk := theme.ForPreferences(settings.Settings{Theme: "auto"}, tc.dark, ui.Preferences{})
			if tk.KeycapRadius != 5 {
				t.Errorf("the keycap radius = %v, want 5", tk.KeycapRadius)
			}
			// The plate is a face plus a hairline, and the hairline is the
			// control edge's own step (§1).
			if tk.KeycapFace.A == 0 {
				t.Error("the keycap face is transparent; a keycap with no plate is not a keycap")
			}
			if tk.KeycapEdge != tk.ControlEdge {
				t.Errorf("the keycap edge %v is not the control hairline %v", tk.KeycapEdge, tk.ControlEdge)
			}
			// And the plate is the size the contract names: 20u. One `Space(n)` is
			// 4u, so a keycap is `Space(5)` — which resolves to 18px at the
			// shipped 0.9 interface step, because a keycap follows `--u` while a
			// *corner* does not (§2/§3).
			if keycapUnits != 5 {
				t.Errorf("a keycap is Space(%v), want Space(5) = 20u", keycapUnits)
			}
			unit := tk.Theme.Spacing / 4
			if got, want := tk.Theme.Space(keycapUnits), 20*unit; got != want {
				t.Errorf("a keycap resolves to %.1fpx, want 20u (%.1fpx) at this step", got, want)
			}
			if keycapGapUnits*4 != 2 {
				t.Errorf("the seam is %v units, want the contract's 2u", keycapGapUnits*4)
			}
		})
	}

	// The chord splits on the platform's spelling: glyphs on macOS, `+`
	// elsewhere; a single key is one plate either way.
	for _, badge := range []string{"Ctrl+1", "Ctrl+Shift+1"} {
		labels := keycapLabels(badge)
		if len(labels) < 2 {
			t.Errorf("%q did not split into keys: %v", badge, labels)
		}
		for _, label := range labels {
			if strings.Contains(label, "+") {
				t.Errorf("the key %q still carries a `+`; the contract draws no separator", label)
			}
		}
	}
	if got := keycapLabels("Tab"); len(got) != 1 || got[0] != "Tab" {
		t.Errorf("a single-key badge split into %v, want one plate", got)
	}
}

// §4.6 · The footer is a block of the card: one weak hairline on top, no pane of
// its own, and the actions legend at its right edge as the same keycap chord.
func TestTheFooterIsABlockWithTheWeakRule(t *testing.T) {
	forEachAppearance(t, footerIsABlockWithTheWeakRule)
}

func footerIsABlockWithTheWeakRule(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	tt.Type("term")
	tt.Frame()
	// The modifier hands the footer the choice, so its row block paints the
	// chosen pill: the block's painted left edge is then the gutter itself
	// (an unselected row paints no fill, and its first ink is the icon plate's
	// own 8u padding inside the block).
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	tt.Advance(300 * 1e6) // the rows' 120ms enter
	tt.Frame()
	img := tt.Image()
	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	row, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatalf("the footer's row was not drawn: %q", tt.Texts())
	}
	gutter := int(ui.LightTheme().Space(2))
	panel := img.RGBAAt(1, 300)
	// The band's top edge: the last full-width painted row above the footer.
	top := -1
	for y := 100; y < img.Bounds().Max.Y; y++ {
		if img.RGBAAt(1, y) != panel && img.RGBAAt(gutter+2, y) != panel &&
			img.RGBAAt(InputWindowWidth-1, y) != panel {
			top = y
		}
	}
	if top < 0 {
		t.Fatalf("the footer's rule was not found: %q", tt.Texts())
	}
	// One pixel: the weak hairline, full width, and specifically *not* the
	// strong rung (the card's own edge) — §4.6 says the footer takes the
	// quieter of the two.
	rule := img.RGBAAt(InputWindowWidth/2, top)
	if !closeRGB(rule, over(tokens.HairlineSoft, img.RGBAAt(1, top-2))) {
		t.Errorf("the footer's rule is %v, want the weak hairline %v over the panel", rule, tokens.HairlineSoft)
	}
	if strong := over(tokens.Hairline, img.RGBAAt(1, top-2)); closeRGB(rule, strong) && tokens.Hairline != tokens.HairlineSoft {
		t.Errorf("the footer's rule is the strong hairline %v; the contract gives the footer the weak one", strong)
	}
	// The legend: `⌘K`'s two plates at the footer's right edge, each the same
	// keycap the rows draw.
	labels := keycapLabels(footerActionsBadge())
	if len(labels) < 2 {
		t.Fatalf("the footer's legend %q is not a chord: %v", footerActionsBadge(), labels)
	}
	for _, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the footer's legend is missing the key %q: %q", label, tt.Texts())
		}
		if gap := plateAround(tt, r).H; gap < 17 || gap > 24 {
			t.Errorf("the legend's key %q is %.1f tall, want the keycap's 20u", label, gap)
		}
	}
	// And the footer's own block stops on the list's gutter, so the two are one
	// column (§4.3/§4.6).
	left := -1
	for x := 0; x < InputWindowWidth; x++ {
		if img.RGBAAt(x, int(row.Y+row.H/2)) != img.RGBAAt(1, int(row.Y+row.H/2)) {
			left = x
			break
		}
	}
	if left != gutter {
		t.Errorf("the footer's block starts at x=%d, want the list's 8u gutter (x=%d)", left, gutter)
	}
}

// §1/§4.3 · The exported frame carries no accent below the field: the launcher's
// list, its selected row and its footer are neutral planes, and the accent is
// spent in exactly three places that are not this surface (§5).
func TestTheLauncherListCarriesNoAccent(t *testing.T) {
	forEachAppearance(t, launcherListCarriesNoAccent)
}

func launcherListCarriesNoAccent(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	tt.Frame()
	tt.Advance(300 * 1e6)
	tt.Frame()
	img := tt.Image()
	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	accent := tokens.Theme.Accent
	// The field band is the card's first band and legitimately holds the field's
	// own controls; below it is the list and the footer, where the accent has no
	// business being.
	fieldBottom := int(ui.LightTheme().Space(fieldRowUnits))
	tinted := 0
	var sample color.RGBA
	for y := fieldBottom + 1; y < img.Bounds().Max.Y-1; y++ {
		for x := 0; x < InputWindowWidth; x++ {
			c := img.RGBAAt(x, y)
			// The accent is a saturated *red*: R well over both G and B, while G
			// and B stay together (the accent is `#E5484D`, a 5-level spread
			// between its green and blue). That pair of facts is exactly what
			// separates it from the warm system-icon amber (`#a24d15`, whose
			// green is 56 levels over its blue): the power rows' plate is a
			// legitimate surface colour and must not be counted as accent spend.
			if int(c.R)-int(c.G) > 24 && int(c.R)-int(c.B) > 24 && absInt(int(c.G)-int(c.B)) <= 8 {
				tinted++
				sample = c
			}
		}
	}
	if tinted > 0 {
		t.Errorf("%d pixels below the field carry the accent (e.g. %v); the launcher's list is neutral (accent %v)",
			tinted, sample, accent)
	}
}

// §4.2 · The field block's floor is a line that is actually *in* the frame: one
// strict row of `HairlineSoft` composited over the fill it closes, on the block's
// last pixel (y = fieldRow-1), full width, and not the strong rung.
//
// The pin exists because the line was promised by the contract and drawn by
// nobody: the box sat in front of the field row in the same absolute stack, so
// the row's own fill (y = 0..fieldRow-1) painted over it and the exported frame
// went straight from field fill to card with no rule between. Both halves are
// checked against the *pixels*, so a comment cannot restore the claim.
func TestTheFieldBlockClosesOnOneWeakHairline(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = tc.name }); err != nil {
				t.Fatal(err)
			}
			tt := render(t, a)
			// The tester is the framework's own light theme unless the dark
			// appearance is asked for (selface_test.go does the same).
			if tc.dark {
				tt.SetDark(true)
			}
			tt.Frame()
			img := tt.Image()
			tokens := theme.ForPreferences(a.settings(), tc.dark, ui.Preferences{})
			fieldRow := int(ui.LightTheme().Space(fieldRowUnits))
			// The fill the line closes, read from inside the block (four rows
			// above its last one, clear of the rule and of the field's ink).
			fill := img.RGBAAt(2, fieldRow-4)
			want := over(tokens.HairlineSoft, fill)
			strong := over(tokens.Hairline, fill)
			// 1) Exactly one row of the frame is the weak hairline over the
			//    fill. Not "at least one": a second row would be a 2px rule.
			matches := make([]int, 0, 2)
			for y := 0; y < img.Bounds().Max.Y; y++ {
				if closeRGB(img.RGBAAt(2, y), want) {
					matches = append(matches, y)
				}
			}
			if len(matches) != 1 {
				t.Fatalf("%d rows are the weak hairline over the field fill %v (want exactly 1, %v); rows %v",
					len(matches), fill, want, matches)
			}
			// 2) That row is the block's own last pixel: `Top(fieldRow-1)`,
			//    a 1px box, so the block spans y=0..fieldRow-1 and the rule is
			//    the closing edge — 「这块的下沿」. One row further down would be
			//    the card's first row, i.e. a line drawn *outside* the block.
			if got, wantY := matches[0], fieldRow-1; got != wantY {
				t.Errorf("the field block's floor is at y=%d, want y=%d (the block's last row, fieldRow %d)",
					got, wantY, fieldRow)
			}
			line := matches[0]
			// 3a) Full width: the rule is the block's, so it runs the card's whole
			//     width. Three samples far apart catch a rule inset to the text
			//     column or to the field's own padding.
			for _, x := range []int{2, 360, InputWindowWidth - 3} {
				got := img.RGBAAt(x, line)
				if !closeRGB(got, want) {
					t.Errorf("the floor is not full width: x=%d,y=%d is %v, want the weak hairline %v",
						x, line, got, want)
				}
			}
			// 3b) …and it is the *weak* rung, not the strong one. The two rungs
			//     are one value apart only if the token pair collapsed, so that
			//     case is argued rather than skipped: if the composites are
			//     indistinguishable, the row must still differ from its two
			//     neighbours (a line at all), and the report records the loss of
			//     the strength distinction.
			if closeRGB(want, strong) {
				if tokens.HairlineSoft != tokens.Hairline {
					t.Logf("%s: the two hairlines composite to one value %v over %v; "+
						"falling back to 「the row differs from its neighbours」", tc.name, want, fill)
				}
				for _, y := range []int{line - 1, line + 1} {
					if closeRGB(img.RGBAAt(2, y), img.RGBAAt(2, line)) {
						t.Errorf("%s: the floor at y=%d is the same as its neighbour y=%d; "+
							"the weak and strong rungs are indistinguishable here and the row "+
							"is not a line at all", tc.name, line, y)
					}
				}
			} else if closeRGB(img.RGBAAt(2, line), strong) {
				t.Errorf("the field block's floor is the strong hairline %v; §4.2 gives it the weak one %v",
					strong, want)
			}
		})
	}
}

// The guard the task book asks for: the views paint from tokens, never from a
// literal colour. `internal/launcher` and `internal/settingsui` are the two
// surfaces this slice touched, and neither may spell `#RRGGBB` or `rgba(` in a
// non-test file — `internal/theme` is the one place a value is written down.
func TestTheViewsPaintFromTokensNotLiterals(t *testing.T) {
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)
	rgba := regexp.MustCompile(`rgba\s*\(`)
	for _, dir := range []string{"../launcher", "../settingsui"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("could not read %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read %s: %v", path, err)
			}
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
	}
	_ = runtime.GOOS
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
