package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-cn · 设置页最后两个「两档的可点面」：按住不放的那一帧与悬停帧**逐像素相同**。
//
// 这一族在 P13-q（28u 小格）/ P13-al（浮条与 toast）/ P13-bv（保存失败横幅的 ✕）/
// P13-bw（捕获输出页页头）/ P13-bx（终端空态的两条出路）已经修过五批，本轮把设置页
// 剩下的两处收口：侧栏的导航行（`sidebarRow`）与集成页折起的「查看输出」行
// （`runOutputToggle`）。
//
// 量三件事（每套外观各一遍，全部读像素，不读注释）：
//
//	① 悬停真的画了面（悬停帧 ≠ 静息帧）——不然「两档」这个前提本身就不成立；
//	② 按下真的画了**另一个**面（按下帧 ≠ 悬停帧）——旧实现这一条逐像素相同，
//	   这是本轮的根因；
//	③ 第三档是**往静息的回退**，不是第三种颜色：按下与静息的距离必须小于悬停与
//	   静息的距离（`docs/liquid-glass.md` §5 的按下档）。
//
// 侧栏另加一条回归钉：按下只改**自己那一块**——被选中的那一页的行，在
// 「悬停在 About 上」与「按住 About」两帧里逐字节相同。
//
// 出图：`FLOTER_P13CN_DUMP=1 go test ./internal/settingsui/ -run TestDumpP13CN`，
// 落到 `/tmp/floter-P13-cn-<surface>-<light|dark>-<rest|hover|press>.png`。

// pressRung2SidebarX is where the sidebar's own row face is read: inside the
// row's fill, right of any page name (the column's inner edge is ~141 and the
// longest name still ends well left of it), and clear of the leading glyph.
const pressRung2SidebarX = 132

// pressRung2Frames parks the pointer at one state and returns the settled frame.
func pressRung2Frames(t *testing.T, tt *ui.Tester, x, y int, state string) {
	t.Helper()
	switch state {
	case "rest":
		// On the surface, on no control: the row's own resting face.
		tt.Move(4, 4)
	case "hover":
		tt.Move(float32(x), float32(y))
	case "press":
		tt.Move(float32(x), float32(y))
		tt.Frame()
		tt.Press(float32(x), float32(y))
	default:
		t.Fatalf("unknown state %q", state)
	}
	// The rows carry a 120ms colour transition: settle past it, twice, so the
	// frame read is never a mid-blend (`liteRender` does the same).
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
}

// TestTheSidebarNavRowTakesAPressRung is ①②③ over the settings sidebar.
func TestTheSidebarNavRowTakesAPressRung(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			face := map[string]ui.Color{}
			for _, state := range []string{"rest", "hover", "press"} {
				tt, _ := liteRender(t, appearance.dark, PageGeneral)
				// The pointer works on a page the user is *not* on, so the row it
				// presses carries the hover/press ladder rather than the chosen fill.
				box := mustFind(t, tt, "About")
				y := int(box.Y + box.H/2)
				pressRung2Frames(t, tt, pressRung2SidebarX, y, state)
				face[state] = imagePixel(tt.Image(), pressRung2SidebarX, y)
				if state == "press" && os.Getenv("FLOTER_P13CN_DUMP") != "" {
					pressRung2Dump(t, tt, "sidebar", appearance.name, state)
				}
			}
			// ① the pointer lights a face at all.
			if d := channelDelta(face["rest"], face["hover"]); d < 6 {
				t.Errorf("hover face %v is %d levels off the resting face %v; the pointer paints nothing",
					face["hover"], d, face["rest"])
			}
			// ② the press paints its own face — this is the round's root cause.
			if d := channelDelta(face["hover"], face["press"]); d < 6 {
				t.Errorf("press face %v is %d levels off the hover face %v: holding the pointer down is pixel-identical to hovering (P13-cn)",
					face["press"], d, face["hover"])
			}
			// ③ the press rung is the row ladder's **heaviest wash**: a row has no face at
			// rest, so its press deepens the tint past the hover rung instead of backing
			// off toward rest (the launcher's result rows read the same way:
			// `RowHover` 0.045 → `RowPressed` 0.11). The control ladder below goes the
			// other way — see the folded-output pin.
			toRest, toHover := channelDelta(face["rest"], face["press"]), channelDelta(face["rest"], face["hover"])
			if toRest <= toHover {
				t.Errorf("press is %d levels off rest and hover is %d: a row's press rung is the heavier wash, so it must sit further from rest than hover",
					toRest, toHover)
			}
		})
	}
}

// TestThePressedSidebarRowLeavesItsNeighbourAlone is the regression pin: a press
// is one row's own face, never a relayout of the column.
func TestThePressedSidebarRowLeavesItsNeighbourAlone(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			var chosen ui.Color
			for _, state := range []string{"hover", "press"} {
				tt, _ := liteRender(t, appearance.dark, PageGeneral)
				box := mustFind(t, tt, "About")
				y := int(box.Y + box.H/2)
				pressRung2Frames(t, tt, pressRung2SidebarX, y, state)
				// The chosen page is the row above; its face must not move.
				chosenRow := mustFind(t, tt, "General")
				got := imagePixel(tt.Image(), pressRung2SidebarX, int(chosenRow.Y+chosenRow.H/2))
				if state == "hover" {
					chosen = got
					continue
				}
				if got != chosen {
					t.Errorf("the chosen page's face moved under a press on another row: %v → %v", chosen, got)
				}
			}
		})
	}
}

// pressRung2IntegrationFrame builds the integrations page with one recorded
// background run, its output block folded (so the affordance is on screen).
func pressRung2IntegrationFrame(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	a := outputBlockApp(false)
	a.selectPage(PageIntegrations)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 720, 1400)
	tt.SetDark(dark)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	return tt
}

// TestTheFoldedOutputToggleTakesAPressRung is ①②③ over the folded "view
// output" line: it is a full-width text control whose face is transparent at
// rest, `ControlHover` under the pointer — and was *nothing* under a press.
func TestTheFoldedOutputToggleTakesAPressRung(t *testing.T) {
	en := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			face := map[string]ui.Color{}
			for _, state := range []string{"rest", "hover", "press"} {
				tt := pressRung2IntegrationFrame(t, appearance.dark)
				box := mustFind(t, tt, en.RunViewOutput)
				// The control is the row's own full-width line (the label's box *is* the
				// control), so the face is read just inside its right end, clear of the
				// chevron and the word.
				x, y := int(box.X+box.W)-20, int(box.Y+box.H/2)
				pressRung2Frames(t, tt, x, y, state)
				face[state] = imagePixel(tt.Image(), x, y)
				if os.Getenv("FLOTER_P13CN_DUMP") != "" {
					pressRung2Dump(t, tt, "outputtoggle", appearance.name, state)
				}
			}
			if d := channelDelta(face["rest"], face["hover"]); d < 6 {
				t.Errorf("hover face %v is %d levels off the resting face %v; the pointer paints nothing",
					face["hover"], d, face["rest"])
			}
			if d := channelDelta(face["hover"], face["press"]); d < 6 {
				t.Errorf("press face %v is %d levels off the hover face %v: holding the pointer down is pixel-identical to hovering (P13-cn)",
					face["press"], d, face["hover"])
			}
			// ③ this is a **control**, not a row: its resting face is the card itself, so
			// the press rung sinks *below* the hover lift (the press face under the hover
			// face) and therefore lands between rest and hover. Opposite direction to the
			// sidebar row above, and deliberately so: a row's press deepens a wash, a
			// control's press gives the lift back.
			toRest, toHover := channelDelta(face["rest"], face["press"]), channelDelta(face["rest"], face["hover"])
			if toRest >= toHover {
				t.Errorf("press is %d levels off rest and hover is %d: a control's press rung must sit between rest and hover",
					toRest, toHover)
			}
		})
	}
}

// pressRung2Dump writes one frame to /tmp for the report's before/after strip.
func pressRung2Dump(t *testing.T, tt *ui.Tester, surface, appearance, state string) {
	t.Helper()
	path := "/tmp/floter-P13-cn-" + surface + "-" + appearance + "-" + state + ".png"
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
