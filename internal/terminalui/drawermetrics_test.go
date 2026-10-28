package terminalui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// drawerMetricsFrame draws just the drawer's strip, over a black root, with the
// reference's own metrics resolved from the frame it is drawn in (the tester's
// preferences round `Space`, so a theme built beside it would not agree).
type drawerMetricsFrame struct {
	frame  *ui.Tester
	tokens theme.Tokens
}

func drawDrawerMetrics(t *testing.T, a *App) drawerMetricsFrame {
	t.Helper()
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.drawer(c, i18n.For("en").Terminal)
	}, 900, 400)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return drawerMetricsFrame{frame: tt, tokens: tok}
}

func (f drawerMetricsFrame) mustFind(t *testing.T, text string) ui.Rect {
	t.Helper()
	box, ok := f.frame.Find(text)
	if !ok {
		t.Fatalf("the drawer does not draw %q: %v", text, f.frame.Texts())
	}
	return box
}

// inkRun is the extent of the pixels brighter than a threshold within a window
// of the frame: the knob's own white lens over the strip's grey track, or the
// accent ink of a value.
type inkRun struct {
	min, max int
}

func (r inkRun) empty() bool     { return r.max < 0 }
func (r inkRun) center() float32 { return float32(r.min+r.max) / 2 }

// brightRun scans a window of the last frame for the pixels a threshold
// catches.
func brightRun(tt *ui.Tester, x0, x1, y0, y1, threshold int) inkRun {
	img := tt.Image()
	out := inkRun{min: 1 << 30, max: -1}
	for y := max(0, y0); y < min(img.Rect.Dy(), y1); y++ {
		for x := max(0, x0); x < min(img.Rect.Dx(), x1); x++ {
			i := y*img.Stride + x*4
			if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) > threshold {
				if x < out.min {
					out.min = x
				}
				if x > out.max {
					out.max = x
				}
			}
		}
	}
	return out
}

// liveKnobCenter renders the real drawer with the font-size setting at one end
// of its range and returns the centre of the range's knob ink — the slider the
// production code draws, moved by the setting it edits, between the label and
// its readout.
func liveKnobCenter(t *testing.T, a *App, size int) float32 {
	t.Helper()
	if err := a.Store.Update(func(s *settings.Settings) { s.FontSize = size }); err != nil {
		t.Fatal(err)
	}
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.drawer(c, i18n.For("en").Terminal)
	}, 900, 400)
	tt.Frame()
	en := i18n.For("en").Settings
	label, ok := tt.Find(en.FontSize)
	if !ok {
		t.Fatalf("the drawer does not draw the range's label %q: %v", en.FontSize, tt.Texts())
	}
	readout, ok := tt.Find(fmt.Sprintf("%d px", size))
	if !ok {
		t.Fatalf("the drawer does not draw the readout %q: %v", fmt.Sprintf("%d px", size), tt.Texts())
	}
	// The knob is the one bright lens in the range: hunt between the label and
	// the readout for the whitest pixels.
	run := brightRun(tt, int(label.X+label.W)+2, int(readout.X), int(label.Y)-8, int(label.Y+label.H)+8, 600)
	if run.empty() {
		t.Fatal("the drawer's range drew no knob")
	}
	return run.center()
}

// probeKnobCenter lays the same range at one width step on a bare root and
// returns its knob's centre; the pair of probes is the ruler a live reading is
// held against, at the reference's width and at the old port's.
func probeKnobCenter(t *testing.T, tok theme.Tokens, step float32, size float64) float32 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Row(c).FillWidth().Padding(tok.Theme.Space(2), tok.Theme.Space(2)).Children(func() {
			ui.Slider(c, &size, 8, 48).Width(tok.Theme.Space(step))
		})
	}, 400, 60)
	tt.Frame()
	run := brightRun(tt, 0, 400, 0, 60, 600)
	if run.empty() {
		t.Fatal("the probe slider drew no knob")
	}
	return run.center()
}

// The drawer's range slider is the reference's `.terminal-setting-range`
// (`terminal.css:677-692`): a fixed `width: 110px` track — 27.5 units, not the
// 22 (88px) the old port wrote. The live drawer is read: the font-size setting
// is moved to both ends of its range and the distance the range's own knob
// travels is held against the same slider laid out at the reference's width and
// at the old port's. The knob travels the track minus its own two-radius inset,
// so the two widths are what the reading distinguishes.
func TestTheDrawerSliderIsTheReferenceWidth(t *testing.T) {
	if drawerRangeWidth != 27.5 {
		t.Fatalf("drawerRangeWidth = %v, want the reference's 110px = 27.5u", drawerRangeWidth)
	}
	a := drawerApp(t)
	tok := drawDrawerMetrics(t, a).tokens

	live := liveKnobCenter(t, a, 8) - liveKnobCenter(t, a, 48)
	reference := probeKnobCenter(t, tok, drawerRangeWidth, 8) - probeKnobCenter(t, tok, drawerRangeWidth, 48)
	old := probeKnobCenter(t, tok, 22, 8) - probeKnobCenter(t, tok, 22, 48)
	if reference == old {
		t.Fatalf("the probe cannot tell the two widths apart (both travel %v)", reference)
	}
	if absFloat32(live-reference) > absFloat32(live-old) {
		t.Errorf("the live range's knob travels %v, closer to the old port's 22u width (%v) than the reference's 110px (%v)",
			live, old, reference)
	}
}

// textWidthAt lays one line at a register and reads its natural width — the
// only handle a frame gives a test on a font size (the framework's text boxes
// are the ruler internal/settingsui's register tests use too).
func textWidthAt(t *testing.T, tok theme.Tokens, text string, mutate func(ui.Element) ui.Element) float32 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Row(c).Children(func() {
			mutate(ui.Text(c, text).SingleLine().Shrink(0))
		})
	}, 400, 60)
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the register probe did not draw %q", text)
	}
	return box.W
}

// The range's live readout is the reference's `.terminal-setting-control__value`
// (`settings.css:854-865`): `--text-body` in monospace with `tabular-nums` at
// 600 — not the muted emphasis step one size down the old port wrote. The live
// strip is read: the ink's own box against the same string laid out at the body
// and emphasis registers, and its colour against the frame's neutral ink.
//
// P8-c 重定基线（§6.2 第 7 条 / §5）：旧测试的第三条断言是「读数的墨是 accent」
// ——`Accent` → `TextStrong`。这个读数是「滑轴现在的值是什么」，它不回答「哪个
// 被选中/哪个是开的」，所以 accent 在这里越界（§5 的判据）。register 与权重
// （body 步、mono、`tnum`、600）零改动，颜色从 accent 换成中性墨，所以第三条
// 断言翻成：墨**跟着 TextStrong 走**且**不是** accent（R−B / R−G 落在灰墨的
// 那几级上，远离 accent 的 ~152/~157）。
func TestTheDrawerReadoutIsTheNeutralBodyRegister(t *testing.T) {
	a := drawerApp(t)
	f := drawDrawerMetrics(t, a)
	tok := f.tokens

	const readout = "14 px"
	rect := f.mustFind(t, readout)

	// The mono body register, with the reference's features and weight.
	want := textWidthAt(t, tok, readout, func(e ui.Element) ui.Element {
		return e.Font(terminalSettingValueFont).FontSize(tok.Body).FontWeight(600).FontFeatures("tnum")
	})
	emphasis := textWidthAt(t, tok, readout, func(e ui.Element) ui.Element {
		return e.FontSize(tok.Emphasis).TextColor(tok.Theme.TextMuted)
	})
	if absFloat32(rect.W-want) > absFloat32(rect.W-emphasis) {
		t.Errorf("the readout is not the body step (box %v, body probe %v, emphasis probe %v)", rect.W, want, emphasis)
	}

	// Its ink is the neutral strong step, not the accent: an antialiased
	// accent-only ink over the black root is `c * {245, 84, 78}`, so its
	// red-minus-blue gap is ~150; a grey ink's gap is a few units at most,
	// however bright it is.
	//
	// P8-c 重定基线：旧断言要求 R−B ≥40 且 R−G ≥15（accent）；合约 §5 把
	// 文字用 accent 禁掉，所以同一条「墨是哪个寄存器」的断言改成两极：读数墨
	// 贴着 `TextStrong` 的合成值，同时落在灰墨的色相容差内。
	img := f.frame.Image()
	near, bestHue := ui.Color{}, 0
	best := -1
	for y := int(rect.Y); y < int(rect.Y+rect.H)+2; y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			i := y*img.Stride + x*4
			p := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			lum := int(p.R) + int(p.G) + int(p.B)
			if lum > best {
				best, near = lum, p
				bestHue = absInt(int(p.B) - int(p.R))
				if x := absInt(int(p.R) - int(p.G)); x > bestHue {
					bestHue = x
				}
			}
		}
	}
	if best < 0 {
		t.Fatal("the readout drew no ink in its own box")
	}
	// Grey: no channel stands off the others like the accent's red does.
	if bestHue > 12 {
		t.Errorf("the readout's brightest pixel %v is %d levels off grey: it carries a hue (the accent is back)",
			near, bestHue)
	}
	// And it is the *strong* neutral: a step above the muted secondary ink
	// the label beside it wears, i.e. readability came from contrast, not hue.
	strongOverBlack := flatColor(tok, tok.TextStrong, 1)
	if gray := int(near.R) + int(near.G) + int(near.B); gray < (int(strongOverBlack.R)+int(strongOverBlack.G)+int(strongOverBlack.B))*3/4 {
		t.Errorf("the readout's ink %v is dimmer than the neutral strong step %v at three-quarters coverage",
			near, strongOverBlack)
	}
}

// The strip is a wrapping grid whose cells have the reference's floor,
// `minmax(230px, 1fr)` (`terminal.css:654-658`): 57.5 units. Rendering the real
// `drawerGrid` at both the reference's 57.5u floor and the old port's 60u
// (240px) one shows the shelf moves with the floor: a wider floor fits fewer
// cells per line, so the grid folds onto more lines and its last inked row sits
// lower at the same window width.
//
// P11-as 重定基线：这条 pin 以前数的是「标签落在多少条不同的 y 上」。那需要一个
// **每个单元格等高**的网格——而 P11-as 让两行宽控件（光标形状的三选一、调色板的
// 选择器+预览）改成上下两行，格子高度就不齐了：标签按各自格子的中线落，y 自然
// 分叉，于是 57.5u 与 60u 两个 floor 量出同一个断点（1400），信号全丢。改成量
// **网格自己有多高**（最后一排落墨的 y），语义不变、判据更难糊弄：多折一行就一
// 定更低，与字体、与格子里的行数无关。
func TestTheDrawerGridCellsKeepTheReferenceFloor(t *testing.T) {
	if drawerCellFloor != 57.5 {
		t.Fatalf("drawerCellFloor = %v, want the reference's 230px = 57.5u", drawerCellFloor)
	}
	// inkedBottom is the strip's own height: the last row of the grid that has
	// any ink on it.
	inkedBottom := func(floorStep float32, w int) int {
		a := drawerApp(t)
		tt := ui.NewTester(func(c *ui.Context) {
			tok := tokensOf(a, c)
			c.SetTheme(tok.Theme)
			c.Root().Background(ui.RGBA(0, 0, 0, 1))
			a.drawerGrid(c, tok.Theme.Space(floorStep))
		}, w, 900)
		tt.Frame()
		img := tt.Image()
		if img == nil {
			t.Fatalf("floor %.1f at %d: no frame", floorStep, w)
		}
		bottom := 0
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r+g+b > 3*0x2000 {
					bottom = y - img.Rect.Min.Y + 1
					break
				}
			}
		}
		return bottom
	}
	// A 900px strip fits three 230px cells and not three 240px ones, so the
	// reference's floor folds one line higher (measured: 170 against 199).
	reference, old := inkedBottom(drawerCellFloor, 900), inkedBottom(60, 900)
	if reference >= old {
		t.Fatalf("the grid at the reference's floor is %d tall and at the old 60u floor %d: the floor did not move the shelf", reference, old)
	}
}
