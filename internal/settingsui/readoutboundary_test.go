package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-fa · 读数行 SpaceBetween 两段的窄窗边界（`docs/liquid-glass.md` §6.122）。
//
// 量出来的结论：**可达宽度域内永不重叠、永不换行、永不塌成不可见**。
// 设置窗不可缩放、`targetSize` 恒问 720 契约宽、`holdContractWidth` 把更宽的拉回
// 720 —— 用户拖不出窄窗，屏宽 <720 时窗口被夹到屏宽（现实下限 ≈614）。实测
// 720 / 614 / 420 × en/zh 六臂，名字与数值都是单行、互不重叠、W>0；360px 处 en
// 名字右沿（284.4）到数值左沿（294.2）仍有 9.8px 空隙；350px 起 en 名字才首次
// 换行（H 17→34，仍不撞），330px 起 flex 把两段都塌成 0×0。
//
// 所以这轮**不加** `SingleLine()`、不让名字让位 —— 那会把「能换行的安全退化」换成
// 「截断」，而可达域里根本没有需要救的场面。边界被钉在这里，将来谁加宽字体或收窄
// 卡片，先撞到的是这两条 pin，而不是用户。

// readoutPair renders the real General page at the given width and resolves the
// readout row's two segments. The value is derived from the store, exactly as
// the production row does.
func readoutPair(t *testing.T, lang string, width int) (name, value ui.Rect) {
	t.Helper()
	copy := i18n.For(lang).Settings
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.selectPage(PageGeneral)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, width, 2600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	snap := store.Snapshot()
	spec := glassmap.SpecForPreferencesOS(runtime.GOOS, snap.GlassStep, snap.MainOpacity, false)
	percent := uint8(math.Round(float64(spec.Composite()) * 100))

	name = mustFind(t, tt, copy.EffectiveOpacity)
	value = mustFind(t, tt, copy.EffectiveOpacityValue(percent))
	return name, value
}

// TestTheReadoutPairNeverCollidesAtReachableWidths pins §6.122 的测量表：三个
// 宽度（契约宽 720 / 窄屏代理 614 / 更窄的 420）× 中英两套，名字与数值必须
// 各自单行、互不重叠、且都非塌。任何一条在未来变红，都是「读数行在可达域里
// 撞了」—— 那就该先量，而不是先加 `SingleLine()`。
func TestTheReadoutPairNeverCollidesAtReachableWidths(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, width := range []int{720, 614, 420} {
			name, value := readoutPair(t, lang, width)
			t.Logf("%s w=%d name=(%.1f,%.1f %.0fx%.0f) value=(%.1f,%.1f %.0fx%.0f)",
				lang, width, name.X, name.Y, name.W, name.H, value.X, value.Y, value.W, value.H)
			if name.W <= 0 || value.W <= 0 {
				t.Errorf("%s w=%d: a segment collapsed to invisible (name %.0fx%.0f, value %.0fx%.0f) "+
					"— the flex floor swallowed the row (§6.122)", lang, width, name.W, name.H, value.W, value.H)
			}
			if name.H > 17.5 || value.H > 17.5 {
				t.Errorf("%s w=%d: a segment wrapped (name H=%.1f, value H=%.1f): the reachable row "+
					"must stay one line (§6.122)", lang, width, name.H, value.H)
			}
			if name.X+name.W > value.X {
				t.Errorf("%s w=%d: the name's right edge %.1f overlaps the value's left edge %.1f "+
					"(§6.122 — 可达域内永不重叠)", lang, width, name.X+name.W, value.X)
			}
		}
	}
}

// TestTheReadoutPairsSlackShrinksBeforeItWraps pins 临界余量：360px 处（第一档
// 真正开始逼近的宽度）en 名字右沿到数值左沿还有 ≥9px 空隙。这条是「先量后改」的
// 预备 —— 将来加宽字体或收窄卡片，这条先红，提醒先量，而不是悄悄加截断。
func TestTheReadoutPairsSlackShrinksBeforeItWraps(t *testing.T) {
	name, value := readoutPair(t, "en", 360)
	slack := value.X - (name.X + name.W)
	if slack < 9 {
		t.Errorf("at 360px the en name ends %.1fpx short of the value: the first wrap "+
			"boundary was 9.8px and is closing (§6.122)", slack)
	}
}

// TestDumpP13FABoundaryPNGs is this round's visual evidence: the readout row's
// card at the 720 contract and at 350 — the first width where the en name wraps
// (still without overlap). Opt-in (`FLOTER_P13FA_DUMP=1`).
func TestDumpP13FABoundaryPNGs(t *testing.T) {
	if os.Getenv("FLOTER_P13FA_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13FA_DUMP=1 writes floter-P13-fa-<width>-<appearance>.png")
	}
	dir := os.Getenv("FLOTER_P13FA_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	tag := os.Getenv("FLOTER_P13FA_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{720, 350} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			copy := i18n.For("en").Settings
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			a.selectPage(PageGeneral)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				a.View(c)
			}, width, 2600)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			name := mustFind(t, tt, copy.EffectiveOpacity)
			// Crop the window's full width (the card is at its left margin), and
			// the readout row plus a little air above and below. Cropping from
			// x=0 keeps the 350 frame inside the 350-wide window instead of
			// spilling past its right edge.
			box := ui.Rect{X: 0, Y: name.Y - 90, W: float32(width), H: name.H + 140}
			img := tt.Image()
			r := image.Rect(int(box.X), int(box.Y), int(box.X+box.W), int(box.Y+box.H))
			crop := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
			for y := r.Min.Y; y < r.Max.Y && y < img.Bounds().Dy(); y++ {
				for x := r.Min.X; x < r.Max.X && x < img.Bounds().Dx(); x++ {
					c := imagePixel(img, x, y)
					i := crop.PixOffset(x-r.Min.X, y-r.Min.Y)
					crop.Pix[i], crop.Pix[i+1], crop.Pix[i+2], crop.Pix[i+3] = c.R, c.G, c.B, c.A
				}
			}
			name2 := filepath.Join(out, fmt.Sprintf("floter-P13-fa-%d-%s.png", width, appearance))
			f, err := os.Create(name2)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, crop); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("wrote %s", name2)
		}
	}
}
