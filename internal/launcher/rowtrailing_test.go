package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// TestDumpKeycapSlotPNGs writes the P9-r visual evidence: the command row's Tab
// keycap slot (and the footer's own chord beside it) in both appearances, so the
// round's report can say what the pin measured. Opt-in (`FLOTER_PNG=1`); a plain
// run must not litter /tmp.
func TestDumpKeycapSlotPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := commandApp()
		tt := renderIn(t, a, dark)
		path := filepath.Join("/tmp", "floter-P9-r-keycapslot-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		img := tt.Image()
		t.Logf("%s (%dx%d) dark=%v", path, img.Rect.Dx(), img.Rect.Dy(), dark)
	}
}

// The row's trailing slot: Raycast prints a *key*, not a sentence
// (`docs/raycast-design.md` §4.5).
//
// P8-a 重定基线：旧 base.css 值 → 合约 §4.5。
//
//   - 旧：`.launcher-result__action` 是「一整串 `Ctrl+1` 文本 + 30u 最小宽度
//   - body 步 620 字重」，选中的行把它抬到 `--text-primary`。
//   - 新：合约把行尾的键改成**键帽**——一个键一块 20u 高的板、圆角
//     `KeycapRadius`（5）、底 `KeycapFace`、边 1px `KeycapEdge`、字
//     `TextTertiary` 在 `Caption` 步 500；组合键 = 每键一块、间隔 2u、**不画
//     `+`**。
//
// 所以这条测试量的事实也换了：旧的那条量的是「一串文本的槽宽与墨色」，新的这条
// 量的是**两块板**——各自的盒高（20u）、各自的圆角 token（5）、它们之间的缝，
// 以及「没有 `Ctrl+1` 这一整串、也没有 `+`」。
func TestTheRowTrailingBadgeIsTheReferenceActionSlot(t *testing.T) {
	forEachAppearance(t, rowTrailingBadgePin)
}

// rowTrailingBadgePin is the badge pin on one appearance. The keycap's plate
// height, its seam, its face and its right alignment are geometry, so they must
// hold in both — P9-b…P9-f brought the other pixel pins to both appearances;
// this one was still light-only (P9-r).
func rowTrailingBadgePin(t *testing.T, dark bool) {
	a := testApp()
	recentPage(a, 2) // P11-bk: the empty page's rows are the recents
	tt := renderIn(t, a, dark)

	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	plate := float32(tokens.Theme.Space(keycapUnits)) // 20u: a keycap's own box

	// The badge is a *chord*, so it is drawn as one plate per key: `Ctrl` and
	// `1` on every platform that spells the modifier in words (`⌘` and `1` on
	// macOS), never the joined string.
	selected := a.resultNumber(1)
	labels := keycapLabels(selected)
	if len(labels) < 2 {
		t.Fatalf("the badge %q did not split into keys: %v", selected, labels)
	}
	if tt.HasText(selected) {
		t.Errorf("the badge is still drawn as one run of text (%q): %q", selected, tt.Texts())
	}
	texts := make([]ui.Rect, len(labels))
	plates := make([]ui.Rect, len(labels))
	for i, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the key %q of the badge %q was not drawn: %q", label, selected, tt.Texts())
		}
		texts[i] = r
		plates[i] = plateAround(tt, r)
		// A keycap's box: 20u tall (the *legend's* height, not the label's — a
		// word grows the plate along its padding, never its height). The painted
		// extent is read off the pixels, so the border's own 1px and the
		// renderer's antialiasing are allowed for: 3px on the high side, 1px on
		// the low.
		if d := plates[i].H - plate; d > 3 || d < -1 {
			t.Errorf("the key %q's plate is %.1f tall, want the keycap's 20u (%.1f)",
				label, plates[i].H, plate)
		}
		if plates[i].W < plate-1 {
			t.Errorf("the key %q's plate is %.1f wide, want at least the keycap's 20u (%.1f)",
				label, plates[i].W, plate)
		}
	}
	// The keys are one seam apart and in order, so the chord reads left to
	// right as the binding does. The seam is 2u; a plate's own border and the
	// glyph's antialiasing eat into the gap as measured, so the two plates may
	// touch but never overlap.
	for i := 1; i < len(plates); i++ {
		if plates[i].X+1 < plates[i-1].X+plates[i-1].W {
			t.Errorf("the keys overlap: %q's plate ends at %.1f, %q's starts at %.1f",
				labels[i-1], plates[i-1].X+plates[i-1].W, labels[i], plates[i].X)
		}
	}

	// The plates are the keycap face: a point inside the last key's plate but
	// clear of its glyph is painted, while the same row away from every plate
	// is the row's own fill.
	img := tt.Image()
	last := plates[len(plates)-1]
	cy := int(last.Y + last.H/2)
	face := img.RGBAAt(int(last.X+1), cy)
	rowFill := img.RGBAAt(3, cy) // the panel gutter: the row's block stops short of it
	if face == rowFill {
		t.Errorf("the keycap's plate is not painted: %v inside the key, %v in the panel's gutter", face, rowFill)
	}

	// The chord is right-aligned at the row's inner edge: the last key's own
	// text box ends before the row block's right gutter, which is the block's
	// 8u inset from the card.
	gutter := tokens.Theme.Space(2)
	if right := texts[len(texts)-1].X + texts[len(texts)-1].W; right > float32(InputWindowWidth)-gutter+1 {
		t.Errorf("the badge's right edge is %.1f, past the row block's inner edge (%.1f)",
			right, float32(InputWindowWidth)-gutter)
	}
}

// A command row's `Tab` affordance and the action bar's own hint are the *same*
// kind of object (`docs/raycast-design.md` §4.5).
//
// P8-a 重定基线：旧 base.css 值 → 合约 §4.5。旧断言有两个 register：行尾槽是
// 30u 的文本盒、页脚 hint 是更矮更软的 caption 文本，于是「行槽必须比页脚 hint
// 高」才成立。合约把两者统一成一枚键帽，所以旧的那条「行槽比页脚高」不再是事实
// ——它量的是两套语汇。新的这条换成：两者**同一枚键帽**（同高、同圆角 token），
// 且页脚的动作图例也是同一枚（`⌘K` 不再是一块拼好的长板）。
func TestTheCommandRowTabHintUsesTheRowSlot(t *testing.T) {
	forEachAppearance(t, commandRowTabHintPin)
}

// commandRowTabHintPin is the same-keycap-shape pin on one appearance: the row's
// Tab plate, the footer's chord and its action legend are all one keycap, and
// that identity is geometry, not ink — so it must hold in both (P9-r).
func commandRowTabHintPin(t *testing.T, dark bool) {
	a := commandApp()
	tt := renderIn(t, a, dark)

	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	if tokens.KeycapRadius != theme.KeycapRadius {
		t.Fatalf("the keycap radius token = %v, want the contract's %v",
			tokens.KeycapRadius, float32(theme.KeycapRadius))
	}

	if !tt.HasText("Tab") {
		t.Fatalf("the command row's Tab affordance is missing: %q", tt.Texts())
	}
	tabText, ok := tt.Find("Tab")
	if !ok {
		t.Fatalf("the Tab hint was not drawn: %q", tt.Texts())
	}
	tabPlate := plateAround(tt, tabText)

	// The footer's own chord: keys on their own plates, the same keycap the row
	// draws.
	labels := keycapLabels(a.enterBadge())
	if len(labels) < 2 {
		t.Fatalf("the action bar's badge %q did not split into keys: %v", a.enterBadge(), labels)
	}
	var barPlate ui.Rect
	for _, label := range labels {
		found, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the action bar's own hint was not drawn (%q missing): %q", label, tt.Texts())
		}
		barPlate = plateAround(tt, found)
	}

	// Both are keycaps, so their plates are the same height. The slack is the
	// renderer's own rounding of a 1px border and the glyph's antialiasing (the
	// same 3px the row's plate allows above); the old pair — a 30u text slot
	// against a caption run — was tens of pixels apart in *meaning*, not just
	// in size.
	if gap := tabPlate.H - barPlate.H; gap > 3 || gap < -3 {
		t.Errorf("the row's Tab plate is %.1f tall and the footer's is %.1f; both are keycaps and must match",
			tabPlate.H, barPlate.H)
	}

	// And there is no second keycap shape: the footer's action legend is a chord
	// of plates, not one plate spelling the joined string.
	if tt.HasText(footerActionsBadge()) {
		t.Errorf("the footer's legend is still one drawn run (%q): %q", footerActionsBadge(), tt.Texts())
	}
	kText, ok := tt.Find("K")
	if !ok {
		t.Fatalf("the footer's action legend has no second plate: %q", tt.Texts())
	}
	if gap := plateAround(tt, kText).H - tabPlate.H; gap > 3 || gap < -3 {
		t.Errorf("the footer legend's plate is %.1f tall, the row's Tab plate %.1f; one keycap shape only",
			plateAround(tt, kText).H, tabPlate.H)
	}
}

// plateAround grows a label's text box out to the plate it is drawn in: the run
// of pixels that are not the fill they sit on. A keycap is the one object in a
// row whose plate is a filled box around its legend, and the row's own fill (or
// the panel's, for the pinned footer) is what surrounds it.
//
// The surrounding fill is the modal colour along the label's own centre line: a
// row of glyphs has a handful of ink pixels and hundreds of fill pixels, so the
// mode is the plane every glyph and plate sits on. A single fixed sample would
// read a glyph on a wide row and the panel outside it on a narrow one.
func plateAround(tt *ui.Tester, text ui.Rect) ui.Rect {
	img := tt.Image()
	pack := func(c interface{ RGBA() (r, g, b, a uint32) }) uint32 {
		r, g, b, a := c.RGBA()
		return r>>8<<24 | g>>8<<16 | b>>8<<8 | a>>8
	}
	cy := int(text.Y + text.H/2)
	counts := make(map[uint32]int, 32)
	var fill uint32
	for x := 0; x < img.Bounds().Max.X; x++ {
		key := pack(img.RGBAAt(x, cy))
		counts[key]++
		if counts[key] > counts[fill] {
			fill = key
		}
	}
	same := func(x, y int) bool { return pack(img.RGBAAt(x, y)) == fill }
	cx := int(text.X + text.W/2)
	left, right := cx, int(text.X+text.W-1)
	for left > 0 && !same(left-1, cy) {
		left--
	}
	for right < img.Bounds().Max.X-1 && !same(right+1, cy) {
		right++
	}
	top, bottom := cy, cy
	for top > 0 && !same(cx, top-1) {
		top--
	}
	for bottom < img.Bounds().Max.Y-1 && !same(cx, bottom+1) {
		bottom++
	}
	return ui.Rect{
		X: float32(left), Y: float32(top),
		W: float32(right - left + 1), H: float32(bottom - top + 1),
	}
}

// strongestInk is the pixel farthest (in summed channels) from the light panel
// surface inside a rect — the glyph's own colour, since text is darker than the
// plane it is drawn on.
func strongestInk(tt *ui.Tester, r ui.Rect) [3]int {
	img := tt.Image()
	var best [3]int
	bestSum := 1 << 30
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			sum := int(c.R) + int(c.G) + int(c.B)
			if sum < bestSum {
				bestSum = sum
				best = [3]int{int(c.R), int(c.G), int(c.B)}
			}
		}
	}
	return best
}

func channelDiff(a, b [3]int) int {
	d := 0
	for i := 0; i < 3; i++ {
		v := a[i] - b[i]
		if v < 0 {
			v = -v
		}
		if v > d {
			d = v
		}
	}
	return d
}
