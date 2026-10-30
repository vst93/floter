package launcher

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P12-b · `docs/liquid-glass.md` §6.1 启动器：E1 的受光边落到这一张卡的四个面上。
//
// 这一支的材质不变量只有一条（§1 末尾）：**受光边严格亮于面**。E1 是「面 + 一条
// 1px 顶边」，判据两条（§2）：一个表面只有一个 E2；**E1 不许有外投影**。
//
// 四条 pin 都量像素，不读注释：
//   - 选中块（结果行 / 捕获输出页的选中行）：顶边亮于块面，块外没有投影；
//   - 键帽：板内顶边亮于板面；
//   - 反馈带：静息与警告两档都亮，同一条带不是两个材质。
//
// 每个断言都在两套外观上跑：受光边在浅色底上是**近白压白**（§3 末尾的说明），
// 所以两档的色阶差不是同一个数量级 —— pin 因此只钉「严格更亮」这条不变量，
// 而把「面差本身要看得见」单独钉在 E1 的面阶上（`litedgeFaceFloor`）。

// litedgeFaceFloor is the face step an E1 block must show against the surface
// it sits on. It is the rung §1 抬到 0.10/0.07 的理由：「面与底的差在 1x 屏上
// 是 8~11 级」。受光边在浅色上只有 2 级，所以**站起来**这件事由面差负责，
// 受光边是它上面那一条光 —— 两者都在，E1 才成立。
const litedgeFaceFloor = 8

// litedgeProbe is one rendered frame plus the tokens it was drawn from.
type litedgeProbe struct {
	tt     *ui.Tester
	tokens theme.Tokens
}

func litedgeRender(t *testing.T, dark bool) litedgeProbe {
	t.Helper()
	a := testApp()
	recentPage(a, 3)
	name := "light"
	if dark {
		name = "dark"
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a)
	if dark {
		tt.SetDark(true)
	}
	tt.Frame()
	// The rows enter over 120ms; a frame read at opacity 0 is the panel's fill.
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	return litedgeProbe{tt: tt, tokens: theme.ForPreferences(a.settings(), dark, ui.Preferences{})}
}

// bandOf returns the pixel column a block is read down: inside the block, clear
// of every glyph and plate. The rows' titles stop well before x=600 on this
// width, and the block's own fill runs to the card's right gutter.
const litedgeColumn = 600

// TestTheChosenRowStandsOnOneLitEdge pins §6.1 rows 1-2 and 1-5: a chosen row
// is E1 — its face (`RowSelected`) plus one 1px `LensRim` top edge — and it
// carries no cast shadow of its own (§2 judgement 2: an E1 block cannot cast
// onto the ground it lies on).
func TestTheChosenRowStandsOnOneLitEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			p := litedgeRender(t, dark)
			img := p.tt.Image()
			row, ok := p.tt.Find("Recent 00") // the empty page's selected first row
			if !ok {
				t.Fatalf("the first result was not drawn: %q", p.tt.Texts())
			}
			// The block's painted box: the row's own box, which starts on the
			// panel's 8u gutter (`docs/raycast-design.md` §4.3).
			gutter := int(ui.LightTheme().Space(2))
			cy := int(row.Y + row.H/2)
			panel := img.RGBAAt(1, cy)
			body := img.RGBAAt(litedgeColumn, cy)
			top := img.RGBAAt(litedgeColumn, int(row.Y))

			// ① The face rung: the block is visibly a面 over the ground. This
			// is what §1's raise to 0.10/0.07 buys, and it is the whole of
			// "standing up" on the light appearance.
			if step := channelGap(body, panel); step < litedgeFaceFloor {
				t.Errorf("the chosen row's face is %v against the panel %v — a %d-level step, under the E1 floor %d: "+
					"the block reads as the ground it sits on",
					body, panel, step, litedgeFaceFloor)
			}
			// ② …plus one lit line on top, strictly brighter than the face it
			// closes. `LensRim` 0.09 over a near-white face is a whisper on the
			// light appearance by construction (白压白, §3 末尾), so the pin is
			// the invariant, not a magnitude.
			if !brighter(top, body) {
				t.Errorf("the chosen row's top edge %v is not brighter than its own face %v: the lit edge is missing "+
					"(E1 = face + one 1px LensRim, liquid-glass.md §2)", top, body)
			}
			// ③ E1 casts nothing. Two pixels outside the block's left gutter,
			// at the block's own height, must be the panel — the same colour it
			// is a band above, where nothing is drawn. A `Shadow(` on the row
			// would darken this column.
			if side := img.RGBAAt(gutter-2, cy); side != panel {
				t.Errorf("the chosen row casts a shadow beside itself (%v at its own height, %v further up): "+
					"an E1 block lies on the ground, it does not float over it (liquid-glass.md §2)", side, panel)
			}
			// …and the same column one row above the block is still the panel:
			// a cast would reach up as well as down.
			if above := img.RGBAAt(gutter-2, int(row.Y)-2); above != panel {
				t.Errorf("the chosen row's cast reaches above its own top edge (%v, panel %v)", above, panel)
			}
			// The rim is the *lit* one, not the card's edge: `EdgeHighlight` is
			// the E2 rung (§2), so an E1 row must not wear it.
			if p.tokens.LensRim == p.tokens.EdgeHighlight {
				t.Fatalf("the component rim and the floating rim are one value %v: E1 and E2 would be the same edge",
					p.tokens.LensRim)
			}
		})
	}
}

// TestTheKeycapIsTheSmallestLitFace pins §6.1 row 1-4: the 20x20 keycap is the
// smallest face on this card and the one that most wants to be lit — it is a
// key, and a key has a crown.
func TestTheKeycapIsTheSmallestLitFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			p := litedgeRender(t, dark)
			img := p.tt.Image()
			labels := keycapLabels("Ctrl+1")
			key := labels[0]
			r, ok := p.tt.Find(key)
			if !ok {
				t.Fatalf("the row's first key %q was not drawn: %q", key, p.tt.Texts())
			}
			plate := plateAround(p.tt, r)
			face := faceOf(img, plate)
			if crown, ok := crownOf(img, plate, p.tokens.LensRim); !ok {
				t.Errorf("the keycap has no lit edge: its face is %v and none of the plate's top four rows is a neutral "+
					"line brighter than it — the smallest face on the card lost its crown "+
					"(liquid-glass.md §6.1 第 1-4 行)", face)
			} else if step := channelGap(crown, face); step < 2 {
				t.Errorf("the keycap's crown %v lifts its face %v by only %d levels: the lit edge is not there "+
					"(liquid-glass.md §1: the lit edge must read as a line)", crown, face, step)
			}
		})
	}
}

// TestTheFeedbackBandIsOneMaterialInBothStates pins §6.1 row 1-7. The band had
// the lit edge only in its **warning** variant ("The non-warning row carries
// none of them"), so one band was two materials: a warm pane that stands when
// something failed, a flat colour block when something worked.
func TestTheFeedbackBandIsOneMaterialInBothStates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		warning bool
	}{
		{"note", false}, {"warning", true},
	} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			t.Run(tc.name+"/"+appearance, func(t *testing.T) {
				a := testApp()
				recentPage(a, 3)
				if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
					t.Fatal(err)
				}
				msg := "installed the thing"
				if tc.warning {
					msg = "could not install the thing"
					a.WarnFeedback(msg)
				} else {
					a.Feedback(msg)
				}
				tt := render(t, a)
				if dark {
					tt.SetDark(true)
				}
				tt.Frame()
				// The band enters over 160ms (`feedbackRow`'s Enter transition).
				tt.Advance(300 * time.Millisecond)
				tt.Frame()
				img := tt.Image()
				tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})
				r, ok := tt.Find(msg)
				if !ok {
					t.Fatalf("the band's sentence was not drawn: %q", tt.Texts())
				}
				// The band's own rectangle, found in the pixels rather than
				// derived from the sentence's text box: a column through the
				// band's left inset, clear of the glyph and of the corner arc.
				panel := img.RGBAAt(1, 300)
				x := int(r.X) - 6
				top, bottom := -1, -1
				for y := int(r.Y) - 24; y < int(r.Y+r.H)+24 && y < img.Bounds().Max.Y; y++ {
					if y < 0 {
						continue
					}
					if img.RGBAAt(x, y) != panel {
						if top < 0 {
							top = y
						}
						bottom = y
					} else if top >= 0 {
						break
					}
				}
				if top < 0 {
					t.Fatalf("the %s band was not found in the frame: %q", tc.name, tt.Texts())
				}
				band := ui.Rect{X: float32(x), Y: float32(top), W: 1, H: float32(bottom - top + 1)}
				face := faceOf(img, band)
				crown, ok := crownOf(img, band, tokens.LensRim)
				if !ok {
					t.Errorf("the %s band is a flat colour block: its face is %v and none of its top four rows is a "+
						"neutral line brighter than it — the band is not an E1 pane (liquid-glass.md §6.1 第 1-7 行)",
						tc.name, face)
					return
				}
				if step := channelGap(crown, face); step < 2 {
					t.Errorf("the %s band's crown %v lifts its face %v by only %d levels (liquid-glass.md §1: "+
						"the lit edge must read as a line)", tc.name, crown, face, step)
				}
			})
		}
	}
}

// TestTheOutputSelectedRowWearsTheSameLitFace pins §6.1 row 1-5: the captured
// output page's chosen row is the *same* material as the launcher list's — one
// rule, one function (`theme.Tokens.LitFace`). Two lists inside one card cannot
// have one chosen block that stands and one that lies flat.
func TestTheOutputSelectedRowWearsTheSameLitFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			a := testApp()
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
				t.Fatal(err)
			}
			tt := runListOutput(t, a, outputRowsWithActions(4))
			img := tt.Image()
			row, ok := tt.Find("Row 0") // the list protocol's first row is chosen
			if !ok {
				t.Fatalf("the chosen output row was not drawn: %q", tt.Texts())
			}
			tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})
			// The chosen block's own rectangle, found in the pixels: a column
			// through the block, right of the title's ink and left of the row's
			// trailing slot, scanned for the contiguous run that is the block.
			block := blockAround(img, litedgeColumn, int(row.Y), int(row.Y+row.H))
			face := faceOf(img, block)
			crown, ok := crownOf(img, block, tokens.LensRim)
			if !ok {
				t.Errorf("the chosen output row is a flat block (%v, face %v): none of its top four rows is LensRim %v "+
					"over that face — the two lists in this card are two materials "+
					"(liquid-glass.md §6.1 第 1-5 行)", block, face, tokens.LensRim)
				return
			}
			if step := channelGap(crown, face); step < 2 {
				t.Errorf("the chosen output row's crown %v lifts its face %v by only %d levels", crown, face, step)
			}
		})
	}
}

// blockAround is the block a text run sits in: the contiguous run of rows around
// the text's own band that is not the panel. It is read from the pixels rather
// than derived from the text box, so a block that grows or shrinks (a subtitle,
// a taller row) is still found.
func blockAround(img *image.RGBA, x, y0, y1 int) ui.Rect {
	panel := img.RGBAAt(1, y0-20)
	top, bottom := y0, y1
	for y := y0 - 1; y >= 0 && y > y0-30; y-- {
		if img.RGBAAt(x, y) == panel {
			break
		}
		top = y
	}
	for y := y1 + 1; y < img.Bounds().Max.Y && y < y1+30; y++ {
		if img.RGBAAt(x, y) == panel {
			break
		}
		bottom = y
	}
	return ui.Rect{X: float32(x), Y: float32(top), W: 1, H: float32(bottom - top + 1)}
}

// faceOf is a block's own face colour: the most common pixel inside it, clear of
// its 1px edges and of its corner arcs. A block's face is the one colour that
// covers its interior, so the mode is the reading that cannot be fooled by a
// single glyph pixel.
func faceOf(img *image.RGBA, rect ui.Rect) color.RGBA {
	x0, x1 := int(rect.X)+3, int(rect.X+rect.W)-3
	y0, y1 := int(rect.Y)+3, int(rect.Y+rect.H)-3
	if x1 <= x0 || y1 <= y0 {
		x0, x1 = int(rect.X), int(rect.X+rect.W)-1
		y0, y1 = int(rect.Y), int(rect.Y+rect.H)-1
	}
	seen := make(map[color.RGBA]int)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			seen[img.RGBAAt(x, y)]++
		}
	}
	var face color.RGBA
	best := 0
	for c, n := range seen {
		if n > best {
			face, best = c, n
		}
	}
	return face
}

// crownOf finds a block's lit edge: a row in its top four that **is** `LensRim`
// composited over the block's own face. It is an exact reading, not a
// "something brighter is there": the lit line has one value per appearance, and
// a block wearing it is wearing *that* line. The top four because the lit line
// sits inside the block's own 1px border, and the renderer's antialiasing shares
// those rows.
//
// A warning band's border is the warning keyline — a strongly coloured row that
// is neither the face nor the lit line, so it cannot be mistaken for the crown.
func crownOf(img *image.RGBA, rect ui.Rect, rim ui.Color) (color.RGBA, bool) {
	face := faceOf(img, rect)
	want := over(rim, face)
	rows := 4
	if h := int(rect.H); h < rows {
		rows = h
	}
	for dy := 0; dy < rows; dy++ {
		y := int(rect.Y) + dy
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if c := img.RGBAAt(x, y); closeRGB(c, want) {
				return c, true
			}
		}
	}
	return color.RGBA{}, false
}

// brighter reports whether a is a strictly lighter pixel than b on every channel
// that carries the step. The lit edge is a neutral white line, so a real crown
// lifts all three channels; a rim painted in one channel only would not be light.
func brighter(a, b interface {
	RGBA() (uint32, uint32, uint32, uint32)
}) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return ar > br && ag > bg && ab > bb
}
