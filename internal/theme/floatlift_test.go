package theme

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

// P13-gb · 同一张浮层，两套外观 × 三个平台规格：真 `Tokens`，不是手算的 alpha。
//
// `Tokens.Float` 是浮层自己那一笔填充，`Tokens.Sunken` 是它压着的那张面（设置体 /
// 结果列表那一层）。浮层坐在面上，眼睛量的是「浮层比面亮多少」，所以这条 pin 钉的是
// **可见抬升** = (1−C)·F，C = 框的可见 alpha = `glassmap.Spec.Composite()`。
//
// 旧算式把 0.10 加在 `TintAlpha`（被 veil 补偿过的填充）上，于是 darwin 的浮层填充
// 比 linux 薄（107 vs 145），可见抬升 0.223 vs 0.302 —— 材质越厚差得越多（`liquid`
// 0.210）。新算式把它钉在 `Composite()` 上：填充相等，抬升相等。
func TestTheFloatingPaneCarriesTheSameLiftOnEveryPlatform(t *testing.T) {
	cases := []struct {
		step       string
		opacity    uint8
		linuxAlpha int
		darwinComp float32 // darwin 框的可见 alpha，字面量
		darwinA    int     // darwin 浮层填充的字节，字面量
		linuxLift  float32 // 可见抬升 = (1−C)·F，字面量
		darwinLift float32
		whyLabel   string
	}{
		{"regular", 10, 51, 0.22, 82, 0.1800, 0.2508,
			"标准档的薄框：材质把框抬到 0.22，浮层跟着抬后的框走；旧值 darwin 26"},
		{"regular", 47, 145, 0.47, 145, 0.3014, 0.3014,
			"出厂的默认档：旧值 darwin 107（抬升 0.223）、linux 145（0.302）"},
		{"liquid", 47, 145, 0.47, 145, 0.3014, 0.3014,
			"厚档的默认档：旧值 darwin 76（抬升 0.210）"},
		{"liquid", 90, 255, 0.90, 255, 0.1000, 0.1000,
			"顶上一档：两边都被 min(1, …) 封在 1.00，抬升只剩框之上的余量"},
	}
	for _, dark := range []bool{true, false} {
		for _, tc := range cases {
			s := settings.Default()
			s.MainOpacity = tc.opacity
			s.GlassStep = tc.step
			darwin := ForPlatform(s, dark, ui.Preferences{}, "darwin")
			linux := ForPlatform(s, dark, ui.Preferences{}, "linux")
			windows := ForPlatform(s, dark, ui.Preferences{}, "windows")
			if int(linux.Float.A) != tc.linuxAlpha {
				t.Errorf("dark=%v %s/%d%%: the material-free float = %d, want %d — %s", dark, tc.step, tc.opacity, linux.Float.A, tc.linuxAlpha, tc.whyLabel)
			}
			if windows.Float.A != linux.Float.A {
				t.Errorf("dark=%v %s/%d%%: windows paints %d, want the material-free %d", dark, tc.step, tc.opacity, windows.Float.A, linux.Float.A)
			}
			if int(darwin.Float.A) != tc.darwinA {
				t.Errorf("dark=%v %s/%d%%: the darwin float = %d, want %d — %s", dark, tc.step, tc.opacity, darwin.Float.A, tc.darwinA, tc.whyLabel)
			}
			// 可见抬升：(1 − C)·F，C 由 spec 给（材质已补偿），F 取真 token 的 alpha。
			specD := glassmap.SpecForPreferencesOS("darwin", tc.step, tc.opacity, false)
			if diff32(specD.Composite(), tc.darwinComp) {
				t.Errorf("dark=%v %s/%d%%: the darwin frame reads %v, want %v", dark, tc.step, tc.opacity, specD.Composite(), tc.darwinComp)
			}
			if lift := (1 - specD.Composite()) * (float32(darwin.Float.A) / 255); diff32(lift, tc.darwinLift) {
				t.Errorf("dark=%v %s/%d%%: the darwin lift = %v, want %v", dark, tc.step, tc.opacity, lift, tc.darwinLift)
			}
			specL := glassmap.SpecForPreferencesOS("linux", tc.step, tc.opacity, false)
			if lift := (1 - specL.Composite()) * (float32(linux.Float.A) / 255); diff32(lift, tc.linuxLift) {
				t.Errorf("dark=%v %s/%d%%: the material-free lift = %v, want %v", dark, tc.step, tc.opacity, lift, tc.linuxLift)
			}
			// 同 C 就必须同 F：`regular` / `liquid` 的 47% 上两平台是同一档。
			if !diff32(specD.Composite(), specL.Composite()) && darwin.Float.A != linux.Float.A {
				t.Errorf("dark=%v %s/%d%%: the same frame (%v) carries %d on darwin and %d on linux",
					dark, tc.step, tc.opacity, specD.Composite(), darwin.Float.A, linux.Float.A)
			}
		}
	}
}

func diff32(a, b float32) bool { return a-b > 1e-4 || b-a > 1e-4 }

// The P13-gb picture: the float pane on its own face, once per platform spec, with the
// old formula's fill painted beside it. The window server's veil is not here (the app
// does not paint it, and neither does the headless tester) — what the frames compare is
// the app's own two fills, which is exactly the quantity this round changed.
func TestDumpP13gbFloatLift(t *testing.T) {
	if os.Getenv("FLOTER_P13GB_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GB_DUMP=1 to write /tmp/floter-P13-gb-floatlift-*.png")
	}
	const (
		cellW = 168
		cellH = 120
	)
	type column struct {
		name  string
		alpha float32
	}
	for _, dark := range []bool{true, false} {
		dark := dark
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		s := settings.Default()
		s.MainOpacity = 47
		s.GlassStep = "liquid"
		linux := ForPlatform(s, dark, ui.Preferences{}, "linux")
		darwin := ForPlatform(s, dark, ui.Preferences{}, "darwin")
		old := glassmap.SpecForPreferencesOS("darwin", "liquid", 47, false)
		columns := []column{
			{"linux 47%", float32(linux.Float.A) / 255},
			{"darwin 47% (P13-gb)", float32(darwin.Float.A) / 255},
			{"darwin 47% (old fill)", old.TintAlpha + 0.1},
		}
		pane := ui.RGB(24, 25, 29)
		if !dark {
			pane = ui.RGB(252, 252, 254)
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(ui.DarkTheme())
			ui.Row(c).FillWidth().FillHeight().Background(ui.RGB(128, 128, 128)).Padding(12).Gap(12).Children(func() {
				for _, col := range columns {
					col := col
					ui.Column(c).Width(cellW).Height(cellH).Radius(12).
						Background(linux.Sunken).Padding(10).Children(func() {
						ui.Box(c).FillWidth().FillHeight().Radius(8).Background(pane.Alpha(col.alpha))
					})
				}
			})
		}, 3*cellW+4*12, cellH+24)
		tt.SetDark(dark)
		tt.Frame()
		// 三格里的浮层各取一块 20x10 的均值：旧填充那一列本该更淡。
		img := tt.Image()
		for i, col := range columns {
			x0 := 12 + i*(cellW+12) + 10
			var sum [3]int
			n := 0
			for y := 30; y < 90; y++ {
				for x := x0 + 4; x < x0+24; x++ {
					o := y*img.Stride + x*4
					sum[0] += int(img.Pix[o])
					sum[1] += int(img.Pix[o+1])
					sum[2] += int(img.Pix[o+2])
					n++
				}
			}
			t.Logf("%s %-22s alpha=%.3f mean=(%d,%d,%d)", appearance, col.name, col.alpha, sum[0]/n, sum[1]/n, sum[2]/n)
		}
		path := "/tmp/floter-P13-gb-floatlift-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
