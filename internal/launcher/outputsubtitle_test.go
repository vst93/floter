package launcher

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-cj · 选中行把副标题提到和标题同墨——同一张卡的另一张列表只提一档
// （`docs/rounds/P13-cj-spec.md`、`docs/liquid-glass.md` §6.53）。
//
// 同一张卡里有两张列表，选中行是同一件东西（P12-b / P13-cg 已经把面、灯、游标
// 都收到同一条规则上）。这一轮量的是**副标题的墨**：
//
//	              标题 墨距      副标题 墨距
//	启动器 · 选中   211           131（`TextSecondary` + 600）
//	捕获输出 · 选中 214 → 214     **214 → 131**（改前 `Text` + 400，与标题同墨）
//
// 改前捕获输出页的选中行**两行同墨**（差 0 级），整档 rank 只剩字重一条腿；
// 同一张卡的另一张列表在同一状态下副标题留在标题下一档（差 80 级）。所以这条
// pin 判三件事：① 选中行的副标题必须离开标题的墨（差 ≥ 60 级）；② 它落的档位
// 与启动器选中行**逐级相同**（跨列表）；③ 静息行的两处副标题也相同
// （都是 `TextMuted`）——选中改的只是「提一档」，不是「换一套颜色」。
//
// P13-dx（§6.93）：上面那三个 131 是**库存尺**的读数，而 app 画的是出货尺。
// 拉到出货尺上，②③ 两臂一直是红的（选中 119 / 112 对 125 / 118，差 **6 级**）——
// 差的不是墨（两支都是 `TextSecondary`），是**字号**：这一页把副标题画在
// `t.FontSize - 1`（9.8），9.8 上这个 token 覆盖不到自己的值，于是「选中的副标题」
// 从来没画出过它自己的颜色。产品改成 `tokens.Body`（10.8 = 阶梯上的一档）之后
// 两支逐级相同，所以 ②③ 的 ±4 容差带一起删掉（§6.92：同族 pin 不许带容差带），
// 并且新增 ⑥：副标题得**够得到自己的墨**——两条列表一起漂时 ② 是绿的，⑥ 是红的。
//
// 量法：`docs/liquid-glass.md` §6.51 判据 2 的同一课——深色外观下最暗的是面，
// 不是字，所以每个字带读的是**离面色最远的像素**，不是最暗像素。
func TestTheChosenRowsSubtitleSitsOneRungBelowItsTitle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			dumpChosenRows(t, appearance, dark)
			outChosen, outPlain := outputRowBands(t, appearance, dark)
			listSelected, listPlain := launcherRowBands(t, appearance, dark)

			t.Logf("%s · chosen   title far %d / subtitle far %d (gap %d)",
				appearance, outChosen.titleFar, outChosen.subFar, outChosen.titleFar-outChosen.subFar)
			t.Logf("%s · selected title far %d / subtitle far %d (gap %d)",
				appearance, listSelected.titleFar, listSelected.subFar, listSelected.titleFar-listSelected.subFar)
			t.Logf("%s · plain    out subtitle far %d, list subtitle far %d",
				appearance, outPlain.subFar, listPlain.subFar)

			// ① 选中行的副标题不许和标题同墨。
			if gap := outChosen.titleFar - outChosen.subFar; gap < 60 {
				t.Errorf("the chosen row's subtitle is %d levels from its title's ink (want ≥ 60): "+
					"the two lines are one ink again, and only the weight is left to tell them apart "+
					"(liquid-glass.md §4: 层级靠面差 + 字重，不靠画框)", gap)
			}
			// ② 两张列表的选中行给副标题的是同一个档位（P13-dx：精确相等，不带容差带）。
			if outChosen.subFar != listSelected.subFar {
				t.Errorf("the chosen row's subtitle ink is %d levels from the selected result row's (%d vs %d): "+
					"the same card's two lists lift a subtitle by different amounts (liquid-glass.md §6.53/§6.93)",
					outChosen.subFar-listSelected.subFar, outChosen.subFar, listSelected.subFar)
			}
			// ③ 静息行两处仍是同一个档位。
			if outPlain.subFar != listPlain.subFar {
				t.Errorf("the plain rows' subtitles read %d vs %d levels from their own faces: "+
					"the resting subtitle rung drifted between the two lists", outPlain.subFar, listPlain.subFar)
			}
			// ④ 选中行与静息行**一样高**：这一页的行高是内容撑的（没有
			// `MinHeight` 地板，与启动器结果列表的 40u 地板不同），所以给
			// 副标题加字重会换一个字体面、把这一行压短 1px（实测 54 → 53），
			// 游标一移，整张列表就跟着抖 1px。**提墨不提重**是这一页的读法。
			if outChosen.h != outPlain.h {
				t.Errorf("the chosen row is %dpx tall and the plain row %dpx: the selection changed the row's box, "+
					"so moving the cursor down the list shifts every row below it by a pixel "+
					"(the launcher's rows carry a MinHeight floor; this page's do not)",
					outChosen.h, outPlain.h)
			}
			// ⑤ 选中行与静息行的**墨距之比**不判：两张行的面不同（选中行戴
			// `RowSelected`），同一句话在两块面上离面的距离不同，比值随面走，
			// 不是一条规则。规则是 ① 的「差」，不是比值。
			//
			// ⑥ P13-dx · 副标题得**够得到自己的墨**：选中档读到的墨距必须等于
			// `TextSecondary` 在这个面上应有的距离，静息档同理（`TextMuted`）。
			// ② 钉的是「两条列表相等」，两条一起漂它仍绿；⑥ 钉的是**字号**——
			// 字号小到覆盖不足时，同一句话读到的墨比它自己的颜色浅，而这一页
			// 此前正是这样（9.8 上浅 119 / 深 112，而 token 自己在 125 / 118）。
			tk := theme.ForPreferences(settings.Normalize(settings.Default()), dark, ui.Preferences{})
			if want := subInkFloor(outChosen.ground, tk.TextSecondary); outChosen.subFar != want {
				t.Errorf("the chosen row's subtitle reads %d levels from its face, but its own ink sits at %d: "+
					"the line is drawn at a size that cannot cover the colour it names "+
					"(liquid-glass.md §6.93: 小字是阶梯上的档，不是「比正文小一档」)",
					outChosen.subFar, want)
			}
			if want := subInkFloor(outPlain.ground, tk.Theme.TextMuted); outPlain.subFar != want {
				t.Errorf("the plain row's subtitle reads %d levels from its face, but its own ink sits at %d",
					outPlain.subFar, want)
			}
		})
	}
}

// subInkFloor is the ink distance a subtitle token sits at over a row's own
// face: the token composited over the face, read exactly the way `textBand`
// reads a glyph. A line drawn at a size whose strokes never reach full
// coverage reads *less* than this - which is what §6.93 was about.
func subInkFloor(ground int, ink ui.Color) int {
	face := color.RGBA{uint8(ground), uint8(ground), uint8(ground), 255}
	return inkAway(inkOver(ink, face).R, uint8(ground))
}

// dumpChosenRows writes the capture page's chosen row and the result list's
// selected row, one above the other, for a human eye (opt-in:
// `FLOTER_P13CJ_DUMP=1`, with an optional `FLOTER_P13CJ_TAG` for the
// before/after pair).
func dumpChosenRows(t *testing.T, appearance string, dark bool) {
	t.Helper()
	if os.Getenv("FLOTER_P13CJ_DUMP") == "" {
		return
	}
	tag := os.Getenv("FLOTER_P13CJ_TAG")
	if tag != "" {
		tag = "-" + tag
	}
	// The capture page's chosen row.
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	tt := listOutputShipped(t, a, rowsWithSubtitle(false), dark)
	row := a.outputRowRects[0]
	crop := image.Rect(0, int(row.Y), 520, int(row.Y+row.H))
	writeCrop(t, tt.Image(), crop, fmt.Sprintf("/tmp/floter-P13-cj-outrow%s-%s.png", tag, appearance))
	// The result list's selected row, same two lines.
	b := testApp()
	if err := b.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	found := make([]apps.App, 0, 6)
	for i := 0; i < 6; i++ {
		found = append(found, apps.App{
			Name:    fmt.Sprintf("Title %d", i),
			Path:    fmt.Sprintf("/Applications/Title %d.app", i),
			Comment: fmt.Sprintf("Subtt %d", i),
		})
	}
	b.SetApps(found)
	b.Query = "title"
	tt2 := renderShipped(t, b, dark)
	row2 := b.rowRects[0]
	crop2 := image.Rect(0, int(row2.Y), 520, int(row2.Y+row2.H))
	writeCrop(t, tt2.Image(), crop2, fmt.Sprintf("/tmp/floter-P13-cj-listrow%s-%s.png", tag, appearance))
}

// writeCrop copies one rectangle of the frame into a PNG.
func writeCrop(t *testing.T, img *image.RGBA, crop image.Rectangle, path string) {
	t.Helper()
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	for y := crop.Min.Y; y < crop.Max.Y; y++ {
		for x := crop.Min.X; x < crop.Max.X; x++ {
			out.Set(x-crop.Min.X, y-crop.Min.Y, img.At(x, y))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}

// rowBands is one row's two text bands, each read as the pixel farthest from
// the row's own face (the colour half of rank; the weight half is measured by
// the ink-mass pins in rowweight_test.go).
type rowBands struct {
	ground   int
	h        int
	titleFar int
	subFar   int
}

// textBand reads x in the text column and y in [from, to), against ground.
//
// P13-dx：列起点从字面量 54 改成 `54 × designUnit()`（出货尺上 48）——§6.87 的
// 判据：参考写 54u 的列就乘 `designUnit`，不要写死像素（库存尺的 54 与出货尺的
// 48.6 是两个帧上的同一个列）。
func textBand(img *image.RGBA, from, to, ground int) (far int) {
	x0 := int(54 * designUnit())
	for y := from; y < to; y++ {
		for x := x0; x < 500; x++ {
			if d := inkAway(img.RGBAAt(x, y).R, uint8(ground)); d > far {
				far = d
			}
		}
	}
	return far
}

// outputRowBands measures the chosen row (0) and the plain row (1) of the
// capture page's list, each with a subtitle.
//
// P13-dx：取景从 `runListOutput`（库存尺 720×488）换成 `listOutputShipped`
// （出货尺 720×439 + 这一面的调色板）。旧取景下两支副标题读 131 / 131（差 0），
// 新取景下改前读 119 / 125（差 6）——这条 pin 之前量的是 app 不画的那一帧。
func outputRowBands(t *testing.T, appearance string, dark bool) (chosen, plain rowBands) {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	tt := listOutputShipped(t, a, rowsWithSubtitle(false), dark)
	img := tt.Image()
	read := func(i int) rowBands {
		row := a.outputRowRects[i]
		ground := int(img.RGBAAt(int(row.X)+4, int(row.Y+row.H/2)).R)
		mid := int(row.Y + row.H/2)
		return rowBands{
			ground:   ground,
			h:        int(row.H),
			titleFar: textBand(img, int(row.Y)+8, mid, ground),
			subFar:   textBand(img, mid+2, int(row.Y+row.H)-2, ground),
		}
	}
	return read(0), read(1)
}

// launcherRowBands measures the selected row (0) and the plain row (1) of the
// result list. The subtitle of an application row is the `.desktop` comment,
// so the fixture gives every application one.
func launcherRowBands(t *testing.T, appearance string, dark bool) (selected, plain rowBands) {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	found := make([]apps.App, 0, 6)
	for i := 0; i < 6; i++ {
		found = append(found, apps.App{
			Name:    fmt.Sprintf("Title %d", i),
			Path:    fmt.Sprintf("/Applications/Title %d.app", i),
			Comment: fmt.Sprintf("Subtt %d", i),
		})
	}
	a.SetApps(found)
	a.Query = "title"
	tt := renderShipped(t, a, dark)
	img := tt.Image()
	read := func(i int) rowBands {
		row := a.rowRects[i]
		mid := int(row.Y + row.H/2)
		ground := int(img.RGBAAt(int(row.X)+4, mid).R)
		return rowBands{
			ground:   ground,
			h:        int(row.H),
			titleFar: textBand(img, int(row.Y)+6, mid, ground),
			subFar:   textBand(img, mid+2, int(row.Y+row.H)-2, ground),
		}
	}
	return read(0), read(1)
}
