package settingsui

import (
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

// P13-ez · 透明度组那句「答案」比它回答的数字小一档（`docs/liquid-glass.md` §6.121）。
//
// 透明度组里唯一一行读数是这一组的**结论**（`a.effectiveOpacity`）：窗口实际显示多少。
// 它回答的那个数字——滑块自己的实时读数——穿正文档（盒高 17）；这句答案此前穿 caption
// 档（盒高 15），与它上面那条**轴词**（`Transparent` / `Solid`，卡里最不要紧的两个词）
// 同级。同一张卡里两个同类数字（「这一组说的是什么数」）不该穿两个档，尤其不该让答案
// 比问题小：§6.120 立的规矩是「一页之内同一角色只有一个字号档，层级由**墨**说」。
//
// P13-ez 重定基线：读数两段的字号 `tokens.Caption`（盒高 15）→ `tokens.Body`（盒高 17）；
// 墨、字重、内边距、`ContentTransition` 一个字节不动，卡只长 2px。
//
// 两条 pin 量的是**真页面**上的盒（860×2400 的常规页，深浅两套），不是合成卡：
// 字号档与主题无关，所以两套外观的盒高必须逐值相同。

// readoutProbe is one frame of the real General page with the five lines the
// transparency group draws.
type readoutProbe struct {
	tt                                 *ui.Tester
	face                               int
	title, value, axis, label, readout ui.Rect
}

// readoutPage renders the real settings page (常规页) tall enough that the whole
// transparency card is on screen, and resolves the five lines by their text.
func readoutPage(t *testing.T, dark bool) readoutProbe {
	t.Helper()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	en := i18n.For("en").Settings
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.selectPage(PageGeneral)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 860, 2400)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	snap := store.Snapshot()
	spec := glassmap.SpecForPreferencesOS(runtime.GOOS, snap.GlassStep, snap.MainOpacity, false)
	percent := uint8(math.Round(float64(spec.Composite()) * 100))

	out := readoutProbe{tt: tt}
	out.title = mustFind(t, tt, en.MainOpacity)
	out.value = mustFind(t, tt, en.Percent(uint8(snap.MainOpacity)))
	out.axis = mustFind(t, tt, en.TransparencyScaleLow)
	out.label = mustFind(t, tt, en.EffectiveOpacity)
	out.readout = mustFind(t, tt, en.EffectiveOpacityValue(percent))
	// The card's own face, read on the card's left margin (between its edge and
	// the row's inset) — never the ground behind it.
	out.face = pageLuma(tt, out.label.X-6, out.label.Y+out.label.H/2)
	return out
}

// TestTheReadoutLineWearsTheSameStepAsTheValueItAnswers pins §6.121 的两条：
// 读数的名字与数值必须同档（此前一个名字一个数都还是同档，真正的病在下一条），
// 且这一档必须等于同卡滑块实时读数的那一档 —— 并把旧的 caption 盒高 15 钉成下限。
func TestTheReadoutLineWearsTheSameStepAsTheValueItAnswers(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceOf(dark), func(t *testing.T) {
			p := readoutPage(t, dark)
			if d := math.Abs(float64(p.label.H - p.readout.H)); d > 0.5 {
				t.Errorf("the readout's name is %.1f tall and its number %.1f: one line, one step (§6.121)",
					p.label.H, p.readout.H)
			}
			// The whole point of the round: the answer is the same step as the
			// value it answers, not one step below it.
			if d := math.Abs(float64(p.readout.H - p.value.H)); d > 0.5 {
				t.Errorf("the readout is %.1f tall but the slider value it answers is %.1f: "+
					"the answer wears a smaller step than the ask (P13-ez 重定基线：旧 15 → 新 %.1f)",
					p.readout.H, p.value.H, p.value.H)
			}
			// 15 was the caption box before this round; 16.5 is the regression
			// floor, so a future `tokens.Caption` on either segment goes red even
			// if both go back together.
			if p.readout.H < 16.5 {
				t.Errorf("the readout is %.1f tall: the caption step's 15 is back (§6.121)", p.readout.H)
			}
			// The axis words stay where they were: they are a scale label, not a
			// value, and this round must not have pulled them up with it.
			if d := math.Abs(float64(p.axis.H - 15)); d > 0.5 {
				t.Errorf("the axis word is %.1f tall, it was 15: the round moved the scale label too (§6.121)", p.axis.H)
			}
		})
	}
}

// TestTheReadoutLineKeepsItsOwnInk pins §6.121 的另一半：**只抬字号，不抬墨**。
// 层级仍由墨说 —— 名字比它的数安静，两者都比行名安静。这条防「顺手把墨也统一了」：
// 一旦三处同墨，层级就只剩字号在说，而字号刚被拉平。
func TestTheReadoutLineKeepsItsOwnInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceOf(dark), func(t *testing.T) {
			p := readoutPage(t, dark)
			name := rowWeight(p.tt, p.face, p.label)
			number := rowWeight(p.tt, p.face, p.readout)
			ask := rowWeight(p.tt, p.face, p.value)
			title := rowWeight(p.tt, p.face, p.title)
			t.Logf("[%s] face %d · name %d · number %d · slider value %d · row title %d",
				appearanceOf(dark), p.face, name, number, ask, title)
			// The answer stays quieter than the ask: both are numbers at the same
			// step now, so this is the one comparison that reads apples to apples
			// — and it is what catches a promotion to the row name's own ink.
			if number >= ask {
				t.Errorf("the readout's number reads %d and the slider value it answers reads %d: "+
					"the answer must stay quieter than the ask (§6.121 — 层级由墨说，字号已被拉平)", number, ask)
			}
			if name >= number {
				t.Errorf("the readout's name reads %d and its number %d: the name must stay quieter "+
					"than the value it names (tertiary vs secondary, §6.120 的 120-3)", name, number)
			}
			if number >= title {
				t.Errorf("the readout's number reads %d and the row's own name %d: the group's answer "+
					"must stay quieter than the setting's name (§6.121)", number, title)
			}
		})
	}
}

// TestDumpP13EZReadoutPNGs is this round's visual evidence: the same real page
// drawn by the build that is checked out. Opt-in (`FLOTER_P13EZ_DUMP=1`); the
// "before" frames come from a worktree at the previous HEAD running this same
// file.
func TestDumpP13EZReadoutPNGs(t *testing.T) {
	if os.Getenv("FLOTER_P13EZ_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13EZ_DUMP=1 writes floter-P13-ez-readout-<appearance>.png")
	}
	dir := os.Getenv("FLOTER_P13EZ_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	tag := os.Getenv("FLOTER_P13EZ_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		p := readoutPage(t, dark)
		// The transparency card, with a little air on every side.
		box := ui.Rect{X: p.label.X - 24, Y: p.title.Y - 24, W: 560, H: p.label.Y + p.label.H + 32 - (p.title.Y - 24)}
		img := p.tt.Image()
		r := image.Rect(int(box.X), int(box.Y), int(box.X+box.W), int(box.Y+box.H))
		crop := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c := imagePixel(img, x, y)
				i := crop.PixOffset(x-r.Min.X, y-r.Min.Y)
				crop.Pix[i], crop.Pix[i+1], crop.Pix[i+2], crop.Pix[i+3] = c.R, c.G, c.B, c.A
			}
		}
		name := filepath.Join(out, "floter-P13-ez-readout-"+appearanceOf(dark)+".png")
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, crop); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
