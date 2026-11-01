package terminalui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-bx · 终端空态的两条「出路」行：指针按下去的那一帧此前与悬停帧**逐像素相同**。
//
// `emptyLine` 是这一页唯一能点的东西（`↩ Start a session` / `⌘N New command`），
// 它的指针反馈此前只有一档：`if line.Hovered() { color = TextStrong }`，按住不放
// 与悬停同帧——一条能被按下去的行，按下去却不回答，读起来是「按坏了」。
//
// 三条判据（每套外观 × 两条行）：
//
//	① 悬停**不**画面：行内那条空隙里的像素，静息与悬停逐值相同（`.terminal-empty__line:hover`
//	   抬的是墨，不是面——这一页的既有说法，本轮不改）；
//	② 按下画了**另一个**面：同一点在按下帧上离开悬停帧 ≥ 6 级（旧实现在这一条上逐像素
//	   相同，这是本轮的根因）；面借设置页那张卡行的第三档 `t.SurfacePressed`
//	   （`settingsui.cardRow`，`rows.go:1405-1416`；`theme.go:487` 是同一个 token）；
//	③ 按下**保留**悬停的抬墨（指针还压在上面，字不该退回静息），且**没有搬盒子**：
//	   键帽与标签的盒子在悬停/按下两帧之间逐值相同，行外一点不动（`Scale` 会让这条红，
//	   行是**行**不是**键**，所以这一族不缩放——P13-am 的第 ④ 条判据）。
//
// 出图：`/tmp/floter-P13-bx-emptylines-<appearance>-<rest|hover|press>.png`（opt-in，
// `FLOTER_PNG=1`）。

// emptyLineProbe is a point inside the row but on neither the keycap nor a glyph:
// the row's 2u gap between the two (the label's own box starts 8px right of the
// keycap's right edge, so 4px left of the label is 4px from both).
func emptyLineProbe(box ui.Rect) (int, int) {
	return int(box.X) - 4, int(box.Y + box.H/2)
}

// emptyLineLuma is the luma of one pixel, the same arithmetic `crownLuma` uses.
func emptyLineLuma(img *image.RGBA, x, y int) int {
	i := y*img.Stride + x*4
	return (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
}

// TestTheEmptyStateWaysOutSinkWhenPressed is the round's pin.
func TestTheEmptyStateWaysOutSinkWhenPressed(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			term := i18n.For("en").Terminal
			for _, label := range []string{term.EmptyNew, term.EmptyBack} {
				// A fresh frame per line: the pointer stays held at the end of one
				// line's press, and a second line measured on that frame would be
				// measured with a button down.
				tt, _, boxes := emptyRegisterFrame(t, dark)
				page := pageLevel(tt.Image(), dark)
				box := boxes[label]
				x, y := emptyLineProbe(box)

				restImg := tt.Image()
				restProbe := emptyLineLuma(restImg, x, y)
				restInk := contrast(inkLevel(restImg, box, dark), page)

				// ① hover: the ink lifts, the face does not appear.
				tt.Move(box.X+box.W/2, box.Y+box.H/2)
				tt.Frame()
				tt.Frame()
				hoverImg := tt.Image()
				hoverProbe := emptyLineLuma(hoverImg, x, y)
				hoverInk := contrast(inkLevel(hoverImg, box, dark), page)
				hoverLabel, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%s: %q left the frame under the pointer", name, label)
				}

				// ② press: a face this row did not have.
				tt.Press(box.X+box.W/2, box.Y+box.H/2)
				tt.Frame()
				tt.Frame()
				pressImg := tt.Image()
				pressProbe := emptyLineLuma(pressImg, x, y)
				pressInk := contrast(inkLevel(pressImg, box, dark), page)
				pressLabel, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%s: %q left the frame while held", name, label)
				}

				t.Logf("%s/%q: probe rest %d hover %d press %d (page %d) · ink rest %d hover %d press %d",
					name, label, restProbe, hoverProbe, pressProbe, page, restInk, hoverInk, pressInk)

				if hoverProbe != restProbe {
					t.Errorf("%s/%q: hovering moved the row's own gap from %d to %d — this page's hover is "+
						"an ink rung, not a face (`.terminal-empty__line:hover`)", name, label, restProbe, hoverProbe)
				}
				if delta := pressProbe - hoverProbe; delta > -6 && delta < 6 {
					t.Errorf("%s/%q: the held frame's gap reads %d and the hovered frame's %d — the press "+
						"painted no face of its own (want |press − hover| ≥ 6); a row that answers nothing "+
						"when held reads as a row that did not take the click", name, label, pressProbe, hoverProbe)
				}
				// ③ the ink the pointer lifted stays lifted, and nothing moves.
				if pressInk < restInk+10 {
					t.Errorf("%s/%q: the label carries %d levels of contrast at rest and %d while held — the "+
						"press dropped the hover rung instead of keeping it (want press ≥ rest + 10)",
						name, label, restInk, pressInk)
				}
				if pressInk < hoverInk-2 {
					t.Errorf("%s/%q: the label carries %d levels under the pointer and %d while held — the "+
						"press dimmed the ink it had just lifted", name, label, hoverInk, pressInk)
				}
				if hoverLabel != pressLabel {
					t.Errorf("%s/%q: the label's box moved %v → %v between the hovered and the held frame — "+
						"a row is a row, not a key: the press rung changes the face, never the geometry",
						name, label, hoverLabel, pressLabel)
				}
				// A point above the row (outside its box, 8px above the label's top) must
				// not change: the face is the row's, not the column's.
				if above := emptyLineLuma(pressImg, x, int(box.Y)-8); above != emptyLineLuma(hoverImg, x, int(box.Y)-8) {
					t.Errorf("%s/%q: the press painted outside the row (y=%d reads %d, hovered %d)",
						name, label, int(box.Y)-8, above, emptyLineLuma(hoverImg, x, int(box.Y)-8))
				}
			}
		})
	}
}

// TestDumpEmptyLinePressPNGs writes the round's evidence: the empty page at rest,
// under the pointer, and held — both appearances. Opt-in, like the other dumps.
func TestDumpEmptyLinePressPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, term, boxes := emptyRegisterFrame(t, dark)
		box := boxes[term.EmptyNew]
		shots := []struct {
			step string
			act  func()
		}{
			{"rest", func() {}},
			{"hover", func() { tt.Move(box.X+box.W/2, box.Y+box.H/2) }},
			{"press", func() { tt.Press(box.X+box.W/2, box.Y+box.H/2) }},
		}
		for _, shot := range shots {
			shot.act()
			tt.Frame()
			tt.Frame()
			path := filepath.Join("/tmp", "floter-P13-bx-emptylines-"+appearance+"-"+shot.step+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("wrote %s", path)
		}
	}
}
