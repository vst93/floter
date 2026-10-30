package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The settings surface's own lens landings, from the reference's
// `glass-reaxis` census (`src/tests/glass-reaxis.test.ts:47-90`):
// `.extensions-action-button` and `.settings-section__card` (the raised lens)
// and `.settings-switch` (the recessed groove). P7-ej measured that the
// controls contributed ≤0.4 RGB between stops — effectively invisible — so the
// user's 「特别是各种控件按钮」 was still unanswered. These controls now wrap
// the framework's flat pane in `theme.Tokens.Lens`/`LensInset`, and this is the
// pixel acceptance: on a constant background, the control's own pixels must
// ride `--glass-lens-scale` (0.5/1/1.6) with the stop.
func renderLensControl(t *testing.T, step string, w, h int, draw func(c *ui.Context, a *App)) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.GlassStep = step }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(28, 29, 33)).Padding(24).Children(func() {
			draw(c, a)
		})
	}, w, h)
	tt.Frame()
	return tt
}

// frameRGBSum is the sum of every pixel's three channels: with a constant
// background it is the controls' own light budget.
func frameRGBSum(tt *ui.Tester) float64 {
	img := tt.Image()
	var sum float64
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			i := y*img.Stride + x*4
			sum += float64(img.Pix[i]) + float64(img.Pix[i+1]) + float64(img.Pix[i+2])
		}
	}
	return sum
}

// rowMeanRGB is the mean colour of one scanline over x0..x1.
func rowMeanRGB(tt *ui.Tester, y, x0, x1 int) float64 {
	img := tt.Image()
	if y < 0 || y >= img.Rect.Dy() {
		return 0
	}
	var sum float64
	var n int
	for x := x0; x < x1 && x < img.Rect.Dx(); x++ {
		i := y*img.Stride + x*4
		sum += float64(img.Pix[i]) + float64(img.Pix[i+1]) + float64(img.Pix[i+2])
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// The raised lens: a control that still draws a lens (`.extensions-action-button`)
// gets strictly brighter with the stop (a lit rim and a broad sheen, under a dark
// shade that is the smaller term), so the whole frame's light budget rises.
//
// P8-b 重定基线（S：结构变了）：`.settings-section__card` 之前也在这张清单上，
// 因为旧卡带 `--elev-0` 透镜；合约 §6.1 第 3 条把分组卡的镜片去掉，所以这条
// 子测试**不再要求卡随玻璃档变亮**，而是反过来说「卡的面不再吃透镜」：四个
// 玻璃档上帧的光量必须一格不变。这是一个更强的 pin——一个不小心把透镜加回来
// 的改动会直接把它撞红。卡的静态面（`Control` + `HairlineSoft` + 半径 10）由
// `raycast_test.go` 的 `TestTheGroupedCardWearsTheContractsFaceEdgeAndCorner`
// 钉住。
func TestTheSettingsCardControlsCarryTheStopLens(t *testing.T) {
	// A control that still carries the lens: the button rises with the stop.
	sums := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		sums[step] = frameRGBSum(renderLensControl(t, step, 400, 120, func(c *ui.Context, a *App) {
			a.actionButton(c, "Check")
		}))
		t.Logf("%-26s %-8s rgb sum %.1f", ".extensions-action-button", step, sums[step])
	}
	if !(sums["off"] < sums["frosted"] && sums["frosted"] < sums["regular"] && sums["regular"] < sums["liquid"]) {
		t.Errorf("the control's lens did not rise with the stop: off %.0f, frosted %.0f, regular %.0f, liquid %.0f",
			sums["off"], sums["frosted"], sums["regular"], sums["liquid"])
	}
	if d := sums["liquid"] - sums["regular"]; d < 300 {
		t.Errorf("regular→liquid moved %.0f of colour, want a visible spread", d)
	}

	// P13-i 重定基线（V）：旧断言是「卡的涨幅 ≤ 按钮的涨幅 + 50」——它默认卡自己
	// 的贡献恒定，而 P12-c 给分组卡戴上的 E1 一对线（2-3 行）自己也吃
	// `--glass-lens-scale`，这个前提已经倒了（P13-i 实测：卡 27612，独立胶囊
	// 25626）。新值：**把卡自己的涨幅量出来**——同一张卡、同一行、不带按钮，
	// 再与同宽、同面、同一帧的参照 pane（`Control` + `LitFace`）比。四层透镜
	// 回来会让卡自己的涨幅翻倍（透镜是 +25k 量级），2% 的容差盖不住它。
	cards := make(map[string]float64)
	bare := make(map[string]float64)
	refs := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		cards[step] = frameRGBSum(renderLensControl(t, step, 400, 120, func(c *ui.Context, a *App) {
			a.card(c, func() {
				a.row(c, "Row", "hint", func() { a.actionButton(c, "Go") })
			})
		}))
		bare[step] = frameRGBSum(renderLensControl(t, step, 400, 120, func(c *ui.Context, a *App) {
			a.card(c, func() {
				a.row(c, "Row", "hint", func() {})
			})
		}))
		refs[step] = frameRGBSum(renderLensControl(t, step, 400, 120, func(c *ui.Context, a *App) {
			tokens := a.tokens(c)
			e := ui.Row(c).FillWidth().Height(c.Theme().Space(12)).Radius(tokens.RadiusMD).
				Background(tokens.Control)
			tokens.LitFace(e)
		}))
		t.Logf("%-26s %-8s rgb sum %.1f (bare %.1f, ref %.1f)",
			".settings-section__card", step, cards[step], bare[step], refs[step])
	}
	if got, want := bare["liquid"]-bare["off"], refs["liquid"]-refs["off"]; got > want+500 || got < want-500 {
		t.Errorf("the card's own light budget rose %.0f with the stop; one E1 pair of the same width rises %.0f "+
			"— the card is wearing something more than `LitFace`", got, want)
	}
	if got := (cards["liquid"] - cards["off"]) - (bare["liquid"] - bare["off"]); got > sums["liquid"]-sums["off"]+50 {
		t.Errorf("the button inside the card added %.0f of light with the stop, more than it does standing alone "+
			"(%.0f): the card is grading it a second time", got, sums["liquid"]-sums["off"])
	}
}

// The custom-tool form's mode pickers (the reference's
// `.extension-custom-mode`): this build uses the framework's `Segmented` inside
// the recessed lens, so the control must ride the stop like the switch does.
func TestTheCustomModeControlCarriesTheStopLens(t *testing.T) {
	draw := func(c *ui.Context, a *App) {
		index := 0
		a.segmentedControl(c, &index, "Program", "Script")
	}
	sums := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		sums[step] = frameRGBSum(renderLensControl(t, step, 400, 120, draw))
		t.Logf("%-8s custom-mode rgb sum %.1f", step, sums[step])
	}
	if !(sums["off"] < sums["frosted"] && sums["frosted"] < sums["regular"] && sums["regular"] < sums["liquid"]) {
		t.Errorf("the control's lens did not rise with the stop: off %.0f, frosted %.0f, regular %.0f, liquid %.0f",
			sums["off"], sums["frosted"], sums["regular"], sums["liquid"])
	}
	if d := sums["liquid"] - sums["regular"]; d < 300 {
		t.Errorf("regular→liquid moved %.0f of colour, want a visible spread", d)
	}
}

// The recessed lens: the switch's track is a groove whose *spread* — lit
// bottom edge minus dark top edge — grows with the stop. The reference pins
// this direction too (`the recessed track lens is the raised lens read
// backwards`, the same test file), so a flat sum is not the metric here: the
// two edges move in opposite directions and nearly cancel.
func TestTheSettingsSwitchCarriesTheRecessedStopLens(t *testing.T) {
	// The switch is laid out at the padding's top-left; it is 20 DIPs tall, so
	// its top inner edge and bottom inner edge are the scanlines below the
	// padding. The rows are read across the switch's width, clear of its pill
	// corners.
	const x0, x1 = 30, 80
	draw := func(c *ui.Context, a *App) {
		on := true
		a.switchControl(c, &on)
	}
	spread := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		tt := renderLensControl(t, step, 200, 80, draw)
		top, bottom := rowMeanRGB(tt, 24, x0, x1), rowMeanRGB(tt, 43, x0, x1)
		spread[step] = bottom - top
		t.Logf("%-8s top %.1f bottom %.1f (spread %.1f)", step, top, bottom, spread[step])
		if bottom <= top {
			t.Errorf("%s: the switch's bottom edge (%.1f) is not lit over its top edge (%.1f)", step, bottom, top)
		}
	}
	if !(spread["off"] < spread["frosted"] && spread["frosted"] < spread["regular"] && spread["regular"] < spread["liquid"]) {
		t.Errorf("the switch's recessed lens did not rise with the stop: off %.1f, frosted %.1f, regular %.1f, liquid %.1f",
			spread["off"], spread["frosted"], spread["regular"], spread["liquid"])
	}
}
