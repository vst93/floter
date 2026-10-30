package terminalui

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-d · `docs/liquid-glass.md` §6.3 终端页：E1 的受光边与 E2 的两层影落到这一页。
//
// 这一支的材质不变量只有一条（§1 末尾）：**受光边严格亮于面**；elevation 的判据
// 三条（§2）：一个表面只有一个 E2、**E1 不许有外投影**、**E2 必须两层影**。
//
// 三条 pin，每条都在深/浅两套外观上跑，全部量**活视图**的像素（不读注释）：
//   - 抽屉的缝：E1 的 `LensRim` —— 亮于抽屉自己的面，且比它上一版的弱发丝
//     `HairlineSoft` 与 E2 的 `EdgeHighlight` 都更接近；
//   - 悬停的图标按钮：**面而不是环** —— 左右两侧的列与它自己的面同色（`Lens` 的
//     四边 `ControlEdge` 环会让它们变一个色号），且它没有外投影；
//   - 空态的键帽：**一对线** —— 顶边亮于面、**底边那条内线暗于面**（上一版只有顶边）。
//
// 冠线怎么量（与 P12-c 同一个手法）：**不猜渲染器的合成算术**，把两条候选线各自
// 源叠加在同一个面上当参照，再要求被量到的那条离它「应该」那条更近；两条候选各画
// 一遍并断言它们本身分得开，否则这个比较永远不会红。pin 里没有一处写死渲染器合成
// 出来的像素值。

// litAppearanceNames is the two appearances, in the shape the sibling surfaces'
// pins use.
var litAppearanceNames = []struct {
	name string
	dark bool
}{
	{"light", false},
	{"dark", true},
}

// litOver composites a straight-alpha colour over an opaque one — src-over, the
// arithmetic the renderer uses for an inset line drawn on a painted face.
func litOver(over, base ui.Color) ui.Color {
	a := float32(over.A) / 255
	mix := func(o, b uint8) uint8 { return uint8(float32(o)*a + float32(b)*(1-a) + 0.5) }
	return ui.Color{R: mix(over.R, base.R), G: mix(over.G, base.G), B: mix(over.B, base.B), A: 255}
}

// litLighter reports whether a is strictly lighter than b on the sum of the
// channels: the lit edge is a neutral white line, so a real crown lifts all of
// them. A rim painted in one channel only would not be light.
func litLighter(a, b ui.Color) bool {
	return int(a.R)+int(a.G)+int(a.B) > int(b.R)+int(b.G)+int(b.B)
}

// litTokens resolves one appearance's tokens on a plain store.
func litTokens(t *testing.T, dark bool) theme.Tokens {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	return theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
}

// litPage renders the live terminal page with a session and (optionally) the
// drawer open, in one appearance, settled past the rows' colour transition. The
// pointer is parked clear of the bar, so no hover state leaks into a reading.
func litPage(t *testing.T, dark, drawer bool, w, h int) (*ui.Tester, theme.Tokens) {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{Settings: func() {}, Close: func() {}, NewCommand: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)
	a.Drawer = drawer
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = tokensOf(a, c)
		c.SetTheme(tokens.Theme)
		a.View(c)
	}, w, h)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Move(450, 96)
	tt.Frame()
	return tt, tokens
}

// TestTheDockedDrawerWearsTheE1LitEdge pins §6.3 row 3-3: the seam between the
// canvas and the docked drawer is **E1's** lit edge — brighter than the drawer's
// own face (§1 的不变量), and nearer the component `LensRim` than either the
// weak hairline it replaced or the floating `EdgeHighlight`.
//
// The judgement is §2's: the drawer is a block that lives *in* the panel — a
// sibling of the canvas that takes layout height and squeezes the canvas, never a
// layer over the output — so it is E1, and E1's edge is the component rim. E2's
// `EdgeHighlight` belongs to the panes that press on other content and can be
// dismissed (the settings page's two floats, P12-c); if both storeys wore one
// line the elevation ladder would have nothing to say about this page.
//
// The seam is read from the strip's first row, where P11-av keeps it (the
// drawer's *top* edge); the line did not move, only its ink did.
func TestTheDockedDrawerWearsTheE1LitEdge(t *testing.T) {
	for _, appearance := range litAppearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tt, tokens := litPage(t, dark, true, 900, 600)
			strip, ok := tt.Find(i18n.For("en").Settings.GroupTerminal)
			if !ok {
				t.Fatalf("the drawer's strip is not on the frame: %q", tt.Texts())
			}
			img := tt.Image()
			x := img.Rect.Dx() / 2
			seam := imagePixel(img, x, int(strip.Y))
			face := imagePixel(img, x, int(strip.Y)+1)
			if seam == face {
				t.Fatalf("[%s] the drawer's seam reads as its own face (%v): there is no line to grade",
					appearance.name, seam)
			}
			if !litLighter(seam, face) {
				t.Errorf("[%s] the drawer's seam %v is not brighter than its own face %v: the E1 lit edge is "+
					"missing (liquid-glass.md §1 的不变量：受光边严格亮于面)", appearance.name, seam, face)
			}
			rim := litOver(tokens.LensRim, face)
			hair := litOver(tokens.HairlineSoft, face)
			e2 := litOver(tokens.EdgeHighlight, face)
			if delta(rim, hair) < 3 {
				t.Fatalf("[%s] LensRim and HairlineSoft composite to %v and %v over the drawer's face (%d level(s)): "+
					"this pin cannot tell the two tiers apart", appearance.name, rim, hair, delta(rim, hair))
			}
			if delta(rim, e2) < 3 {
				t.Fatalf("[%s] LensRim and EdgeHighlight composite to %v and %v over the drawer's face (%d level(s)): "+
					"this pin cannot tell the two storeys apart", appearance.name, rim, e2, delta(rim, e2))
			}
			if delta(seam, rim) >= delta(seam, hair) {
				t.Errorf("[%s] the drawer's seam %v is nearer the old weak hairline (%v) than the E1 lit edge (%v): "+
					"the block is back to a partition line (liquid-glass.md §6.3 第 3-3 行)",
					appearance.name, seam, hair, rim)
			}
			if delta(seam, rim) >= delta(seam, e2) {
				t.Errorf("[%s] the drawer's seam %v is nearer the E2 `EdgeHighlight` (%v) than the E1 `LensRim` (%v): "+
					"a block docked in the panel wears the component rim, not the floating one",
					appearance.name, seam, e2, rim)
			}
			// The line is still on the drawer's own top edge (P11-av): the canvas
			// row above it is untouched.
			if above := imagePixel(img, x, int(strip.Y)-1); above == seam {
				t.Errorf("[%s] the row above the drawer (y=%d) carries the seam colour %v: the line was painted on "+
					"the canvas instead of on the drawer", appearance.name, int(strip.Y)-1, above)
			}
		})
	}
}

// TestTheHoveredBarButtonIsAFaceNotARing pins §6.3 row 3-7: the bar's icon
// button, while the pointer rests on it, is **E1's face** — one lit line above,
// one shade below — and not the four-layer control `Lens`.
//
// `Lens` is the rung this generation retires: its fourth layer is a
// `ControlEdge` ring on **all four** sides, i.e. the 「画框分层」 §2 replaces with
// the face step. That ring shows up on the button's own first and last columns at
// mid height, where `LitFace` paints nothing; so that is what the pin reads. (The
// old comment next to the call still claimed the lens was 「at ≤0.04 alpha」 —
// P12-a had already re-based `LensRim` to 0.09.)
//
// The reading is on a *hovered* button: it is transparent at rest, so this is the
// only state it can be graded in, and the bar's own backdrop is what those
// columns sit on.
func TestTheHoveredBarButtonIsAFaceNotARing(t *testing.T) {
	for _, appearance := range litAppearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tt, _ := litPage(t, dark, false, 900, 600)
			copy := i18n.For("en").Terminal
			gear, ok := tt.Find(copy.SettingsOpen)
			if !ok {
				t.Fatalf("[%s] the bar's gear is not on the frame: %q", appearance.name, tt.Texts())
			}
			tt.Move(gear.X+gear.W/2, gear.Y+gear.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			img := tt.Image()
			cy := int(gear.Y + gear.H/2)
			face := imagePixel(img, int(gear.X)+4, cy)
			// The hover wash must be there at all, read against the bar's own
			// backdrop well clear of the button: a reading taken before the 90ms
			// colour transition would have graded the backdrop, not the button.
			backdrop := imagePixel(img, int(gear.X)-60, cy)
			if delta(face, backdrop) < 2 {
				t.Fatalf("[%s] the button painted no hover face at all (face %v, backdrop %v): the reading would "+
					"grade the bar's backdrop, not the button", appearance.name, face, backdrop)
			}
			for _, side := range []struct {
				what string
				got  ui.Color
			}{{
				"left", imagePixel(img, int(gear.X), cy)},
				{"right", imagePixel(img, int(gear.X+gear.W)-1, cy)},
			} {
				if delta(side.got, face) > 1 {
					t.Errorf("[%s] the button's %s column at mid height is %v but its face is %v: a ring is drawn "+
						"around the whole box — the hovered icon button is an E1 face, not the four-layer `Lens` "+
						"(liquid-glass.md §6.3 第 3-7 行)", appearance.name, side.what, side.got, face)
				}
			}
			// …and E1 casts nothing (§2 judgement 2): the bar's backdrop just
			// outside the button's trailing edge is the backdrop it was before.
			outside := imagePixel(img, int(gear.X+gear.W)+2, cy)
			if delta(outside, backdrop) > 2 {
				t.Errorf("[%s] the button casts a shadow beside itself (%v against the bar's backdrop %v): an E1 "+
					"block lies on the surface, it does not float over it", appearance.name, outside, backdrop)
			}
		})
	}
}

// TestTheEmptyKeycapWearsTheE1Pair pins §6.3 row 3-8, the row this round added:
// the empty state's keycap is **E1 complete** — one `LensRim` line along its top
// *and* the `LensEdge` counter-side along its bottom.
//
// The previous generation gave it only the top line. On the light appearance a
// 0.09 white line over a near-white face lifts the pixel by two or three levels
// (白压白, §3 末尾), so the keycap read as the row it sits on; what makes a block
// stand up is the **counter-side** (顶亮一分、底暗一分). It is the same
// `theme.Tokens.LitFace` the launcher's keycap wears (P12-b), not a second recipe.
//
// The badge's box is **measured** from the frame rather than derived from the
// theme's numbers: the glyph's text box is what `Find` hands back, and the badge
// around it is padding plus a 1px border whose exact pixel height depends on the
// appearance. The column is scanned against the ground it sits on, so the crown
// is read from the rows just inside the top border and the counter-side from the
// last inner row **above** the bottom border.
func TestTheEmptyKeycapWearsTheE1Pair(t *testing.T) {
	for _, appearance := range litAppearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			name := "light"
			if dark {
				name = "dark"
			}
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{Close: func() {}, NewCommand: func() {}}, nil)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokensOf(a, c).Theme)
				a.View(c)
			}, 860, 600)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			// The empty state's first line keys the session in a badge carrying
			// the enter glyph (`keyBadge("enter")`).
			glyph, ok := tt.Find("\u21a9")
			if !ok {
				t.Fatalf("[%s] the empty state's keycap is not on the frame: %q", appearance.name, tt.Texts())
			}
			img := tt.Image()
			x := int(glyph.X + glyph.W/2)
			// The badge's painted box on this column: the contiguous run of
			// non-ground rows that contains the glyph. Scanning a run rather than
			// the first and last differing row keeps a second line of the empty
			// state (or the row below) out of the reading.
			ground := imagePixel(img, x, int(glyph.Y)-8)
			lo, hi := int(glyph.Y)-8, int(glyph.Y+glyph.H)+10
			top, bottom := -1, -1
			for y := int(glyph.Y); y >= lo; y-- {
				if imagePixel(img, x, y) == ground {
					break
				}
				top = y
			}
			for y := int(glyph.Y); y <= hi; y++ {
				if imagePixel(img, x, y) == ground {
					break
				}
				bottom = y
			}
			if top < 0 || bottom-top < 8 {
				t.Fatalf("[%s] the keycap's painted box was not found on the column x=%d (ground %v, top %d, "+
					"bottom %d)", appearance.name, x, ground, top, bottom)
			}
			// The face: three rows in, past the 1px border and clear of the glyph
			// (which starts well below on this 20u chip).
			face := imagePixel(img, x, top+3)
			// The crown: the brightest of the rows inside the border.
			crown := imagePixel(img, x, top+1)
			for dy := 2; dy <= 2; dy++ {
				if q := imagePixel(img, x, top+dy); litLighter(q, crown) {
					crown = q
				}
			}
			// The counter-side: the darkest of the last two rows *above* the
			// bottom border — the border itself is the box's own `ControlEdge`,
			// not the E1 shade.
			shade := imagePixel(img, x, bottom-1)
			for _, y := range []int{bottom - 2, bottom - 1} {
				if q := imagePixel(img, x, y); int(q.R)+int(q.G)+int(q.B) < int(shade.R)+int(shade.G)+int(shade.B) {
					shade = q
				}
			}
			if !litLighter(crown, face) {
				t.Errorf("[%s] the keycap's crown %v is not brighter than its face %v: the lit line is missing "+
					"(E1 = face + one 1px LensRim, liquid-glass.md §2)", appearance.name, crown, face)
			}
			if int(shade.R)+int(shade.G)+int(shade.B) >= int(face.R)+int(face.G)+int(face.B) {
				t.Errorf("[%s] the keycap has no counter-side: its last inner row %v is not darker than its face "+
					"%v (顶亮一分、底暗一分，控件才站起来, §3) — it wore only half of the E1 pair",
					appearance.name, shade, face)
			}
			// The pair is the contract's own tokens, not two arbitrary lines: the
			// shade's drop is graded against a `LensEdge` reference over the same
			// face beside the badge.
			ref := litOver(tokensOfLite(t, dark).LensEdge, face)
			if delta(shade, ref) > 3 {
				t.Errorf("[%s] the keycap's counter-side = %v, want LensEdge over its face (%v)",
					appearance.name, shade, ref)
			}
		})
	}
}

// tokensOfLite resolves one appearance's tokens without a context, for the
// reference composites the pins compare against.
func tokensOfLite(t *testing.T, dark bool) theme.Tokens {
	t.Helper()
	return litTokens(t, dark)
}

// imagePixel reads one pixel of a frame as a ui.Color.
func imagePixel(img *image.RGBA, x, y int) ui.Color {
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return ui.Color{}
	}
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// TestDumpP13DTerminalFrames is the round's visual evidence: the terminal page
// with the drawer open, the bar with the pointer on it, and the empty state — in
// both appearances. Opt-in (`FLOTER_PNG=1`) like the other dumps: a test run
// should not litter /tmp. `FLOTER_PNG_TAG` names the side of the change
// (`before` / `after`) so the two sets sit side by side.
func TestDumpP13DTerminalFrames(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	dir := os.Getenv("FLOTER_PNG_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	for _, appearance := range litAppearanceNames {
		dark, name := appearance.dark, appearance.name

		// ① The drawer: the E1 seam between the canvas and the strip.
		ttDrawer, _ := litPage(t, dark, true, 900, 600)
		writePNG(t, filepath.Join(dir, "floter-P13-d-drawer-"+name+"-"+tag+".png"), ttDrawer.Image())

		// ② The bar with the pointer on the gear: the E2 two-layer throw, and the
		// hovered button's E1 face.
		ttBar, _ := litPage(t, dark, false, 900, 240)
		gears := i18n.For("en").Terminal
		if gear, ok := ttBar.Find(gears.SettingsOpen); ok {
			ttBar.Move(gear.X+gear.W/2, gear.Y+gear.H/2)
		} else {
			ttBar.Move(450, 14)
		}
		ttBar.Advance(300 * time.Millisecond)
		ttBar.Frame()
		writePNG(t, filepath.Join(dir, "floter-P13-d-bar-"+name+"-"+tag+".png"), ttBar.Image())

		// ③ The empty state: the keycap pair.
		ttEmpty := emptyPage(t, dark, 860, 480)
		writePNG(t, filepath.Join(dir, "floter-P13-d-empty-"+name+"-"+tag+".png"), ttEmpty.Image())
	}
}

// emptyPage renders the terminal's empty state in one appearance.
func emptyPage(t *testing.T, dark bool, w, h int) *ui.Tester {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{Close: func() {}, NewCommand: func() {}}, nil)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokensOf(a, c).Theme)
		a.View(c)
	}, w, h)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}
