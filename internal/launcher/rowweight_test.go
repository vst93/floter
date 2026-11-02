package launcher

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"floter/internal/settings"
)

// P13-ci · 捕获输出页的行，标题和副标题是同一个字重（`docs/rounds/P13-ci-spec.md`、
// `docs/liquid-glass.md` §6.52）。
//
// 同一张卡里有两张列表：启动器的结果列表给标题 580、副标题 510（`launcher.go` 的
// `rowBody`），捕获输出页这一张一直是 400 / 400。于是一份只有 `kind:"status"` 便条
// 的列表里，标题与副标题**同墨同重**（实测浅色 113 / 113）——两行读起来是两句等价
// 的话，标题没有说它是标题。
//
// 字重是**墨级差看不见的那一半**（P13-ch §6.51）：同一 token 下 400 与 580 的最暗
// 像素同值，分得开的是墨量。所以这条 pin 量的是字带上的**墨量和**，不是最暗像素。
func TestANoteRowsTitleOutweighsItsSubtitle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			for _, note := range []bool{true, false} {
				label := "door"
				if note {
					label = "note"
				}
				titleMass, subtitleMass := outputLineInkMasses(t, appearance, dark, note)
				ratio := float64(titleMass) / float64(subtitleMass)
				t.Logf("%s %s: title ink %d, subtitle ink %d (ratio %.2f)",
					appearance, label, titleMass, subtitleMass, ratio)
				// 便条行两行同字号同墨，整档 rank 全靠字重。**1.41 是库存尺的读法**
				// （标题 14 对副标题 10.8），报告里那个帧生产上不存在；出货尺实测
				// **1.28**（P13-ec 取景收口，旧值 1.41 → 新值 1.28）。门槛 1.25 仍高于
				// 「两行同重」的 1.00，所以 580 → 400 的变异照旧变红。
				// 门行还多一层墨级差：实测 2.37 / 2.40（改前 2.06 / 2.02）。
				want := 1.25
				if !note {
					want = 2.0
				}
				if ratio < want {
					t.Errorf("the %s row's title carries %d ink over its subtitle's %d (ratio %.2f, want %.2f) — "+
						"the two lines are drifting back to one weight (liquid-glass.md §4: 层级靠面差 + 字重，不靠画框)",
						label, titleMass, subtitleMass, ratio, want)
				}
			}
			dumpOutputRowWeights(t, appearance, dark)
		})
	}
}

// inkAway is a pixel's distance from the ground, so one profile reads both
// appearances (light: darker is ink; dark: lighter is ink).
func inkAway(v, ground uint8) int {
	if int(v) > int(ground) {
		return int(v) - int(ground)
	}
	return int(ground) - int(v)
}

// rowsWithSubtitle is a two-row list: `note` picks the protocol's
// `kind:"status"` rows (which Enter cannot run) over ordinary doors.
func rowsWithSubtitle(note bool) string {
	kind := `"action":{"type":"insert","text":"x"}`
	if note {
		kind = `"kind":"status"`
	}
	return `[{"id":"a","title":"Title A","subtitle":"Sub A",` + kind + `},` +
		`{"id":"b","title":"Title B","subtitle":"Sub B",` + kind + `}]`
}

// outputLineInkMasses renders a two-row list (with subtitles) and sums the ink
// the row's own two text bands carry. The split is the row's midpoint: a 54u
// row puts the title's band above it and the subtitle's below.
func outputLineInkMasses(t *testing.T, appearance string, dark, note bool) (int, int) {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	tt := runListOutput(t, a, rowsWithSubtitle(note))
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	img := tt.Image()
	row := a.outputRowRects[1]
	if row.H == 0 {
		t.Fatalf("the second row has no box (%v)", a.outputRowRects)
	}
	ground := int(img.RGBAAt(litedgeColumn, int(row.Y+row.H/2)).R)
	mass := func(from, to int) int {
		sum := 0
		for y := from; y < to; y++ {
			for x := 54; x < 500; x++ {
				sum += inkAway(img.RGBAAt(x, y).R, uint8(ground))
			}
		}
		return sum
	}
	mid := int(row.Y + row.H/2)
	return mass(int(row.Y), mid), mass(mid, int(row.Y+row.H))
}

// dumpOutputRowWeights writes the note row's two lines for a human eye
// (opt-in: `FLOTER_P13CI_DUMP=1`).
func dumpOutputRowWeights(t *testing.T, appearance string, dark bool) {
	t.Helper()
	if os.Getenv("FLOTER_P13CI_DUMP") == "" {
		return
	}
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	tt := runListOutput(t, a, rowsWithSubtitle(true))
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	row := a.outputRowRects[1]
	img := tt.Image()
	crop := image.Rect(0, int(row.Y)-4, 420, int(row.Y+row.H)+4)
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	for y := crop.Min.Y; y < crop.Max.Y; y++ {
		for x := crop.Min.X; x < crop.Max.X; x++ {
			out.Set(x-crop.Min.X, y-crop.Min.Y, img.At(x, y))
		}
	}
	path := fmt.Sprintf("/tmp/floter-P13-ci-noterow-%s.png", appearance)
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
