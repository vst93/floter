package settingsui

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-bh（`docs/liquid-glass.md` §6.27）：全仓最后两处 `.Italic()`。
//
// 中文没有斜体字形——苹方 / 思源 / 微软雅黑都只有正体，`font-style: italic` 落到
// 引擎的**伪合成斜切**（pango `pangoStyleItalic`、DirectWrite `fontStyleItalic`、
// CoreText `syntheticItalic`）。11px 的 CJK 笔画被重采样一次并横向拉伸约 15°，
// 结果是「糊」——用户偏好账本 P-1（硬禁令：界面排版禁用斜体，中文尤其）与
// `chinese-typography.md` §三「中文禁斜体（强制倾斜=笔画变形）」在全仓的最后两处
// 违反都在 `runOutput` 凹槽里：
//
//	settingsui.go:6899  空输出占位  `（无输出）`              TextMuted + Italic
//	settingsui.go:6926  截断尾      `……输出已截断（上限 64KB）` TextTertiary + Italic
//
// 参考 CSS 用 `font-style: italic` 说「这一行不是程序的输出，是一句旁白」
// （`extensions.css:1259-1267` 与 `:1268-1280`）。在中文里这个载体不存在，所以
// 语义换一个能读的载体：文案自带的**全角括号**与**省略号**（都不像程序输出），
// 加上两档墨分家（占位 muted > 尾行 tertiary）。

// TestNoSurfaceInTheAppSlantsItsChinese is the census arm: no production file in
// `cmd/` or `internal/` calls `.Italic()` at all. It is not "no italic in UI" as
// a taste rule — it is narrower and harder: every remaining call site was over
// CJK copy, and CJK has no italic face, so the call can only ask the engine to
// shear the glyphs.
//
// A future non-CJK use would still have to come through here: a lone italic
// over Latin text is the reference's own `.extension-row__output-empty`
// reading, and that reading is now served by the copy's brackets and the ink
// step instead. If a round ever wants a real italic face back, it says so in
// `liquid-glass.md` §6.27 and updates this table.
func TestNoSurfaceInTheAppSlantsItsChinese(t *testing.T) {
	sites := italicCensus(t)
	if len(sites) != 0 {
		t.Errorf("production code still asks for an italic face at %d site(s):\n  %s\n"+
			"CJK has no italic face, so this is the engine's synthetic shear over a 11px "+
			"stroke: use the copy's own brackets / ellipsis and an ink step instead "+
			"(liquid-glass.md §6.27)", len(sites), strings.Join(sites, "\n  "))
	}
}

// italicCensus lists every production `.Italic()` call as `file:line`, in a
// stable order so a failure names the site it means.
func italicCensus(t *testing.T) []string {
	t.Helper()
	var sites []string
	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path) // #nosec G304 -- the repo's own sources
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(body), "\n") {
				// A comment naming the call is documentation, not a call: this
				// round's own note explains the removal and would otherwise
				// count itself.
				if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(line, ".Italic()") {
					sites = append(sites, filepath.ToSlash(path)+":"+strconv.Itoa(i+1))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return sites
}

// glyphMask is a text box's ink as a bitmap normalised to its own bounding
// box: "how the glyphs are shaped", with the position they were drawn at
// divided out.
type glyphMask struct {
	w, h int
	pix  []bool
}

func glyphMaskOf(tt *ui.Tester, box ui.Rect, plane int) glyphMask {
	img := tt.Image()
	minX, minY, maxX, maxY := 1<<30, 1<<30, 0, 0
	ink := func(x, y int) bool {
		i := y*img.Stride + x*4
		d := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) - plane
		if d < 0 {
			d = -d
		}
		// Both directions: an ink is *darker* than its plane in the light
		// appearance and *lighter* in the dark one.
		return d >= 8
	}
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			if !ink(x, y) {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	m := glyphMask{w: maxX - minX + 1, h: maxY - minY + 1}
	m.pix = make([]bool, m.w*m.h)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if ink(x, y) {
				m.pix[(y-minY)*m.w+(x-minX)] = true
			}
		}
	}
	return m
}

// diff returns how many cells differ between two masks of the same size, or -1
// when the shapes are not comparable (a synthetic shear can widen a box by a
// pixel, and then "different" is already answered).
func (m glyphMask) diff(other glyphMask) int {
	if m.w != other.w || m.h != other.h {
		return -1
	}
	n := 0
	for i := range m.pix {
		if m.pix[i] != other.pix[i] {
			n++
		}
	}
	return n
}

// TestTheCJKPlaceholderIsNotSheared is the pixel arm: the two lines the round
// touches are drawn upright.
//
// It draws them through the page's own builder (`runOutputAside`, the one call
// site both asides now share) and compares their normalised glyph shape against
// the same string drawn upright. The italic frame is the control: it proves the
// comparison can tell the two apart at all (measured: the sheared frame drifts
// −1..+1 px row by row against the upright one, and the placeholder's mask
// differs in 77 of 407 glyph cells). Without a control, a pin that merely says
// "the mask has ink" would pass on a blank frame.
func TestTheCJKPlaceholderIsNotSheared(t *testing.T) {
	zh := i18n.For("zh").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Settings{Language: "zh"}, dark, ui.Preferences{})
		face := runOutputEdgeFace(tokens, tokens.Theme)
		plane := luma(face.R, face.G, face.B)
		for _, arm := range []struct {
			name string
			line string
			tail bool
		}{
			{"empty", zh.RunNoOutput, false},
			{"truncated", zh.RunOutputTruncatedTail, true},
		} {
			ink := runOutputAsideInk(tokens, arm.tail)
			drawn := asideMask(t, tokens, face, arm.line, arm.tail, plane)
			upright, sheared := referenceMasks(t, tokens, face, arm.line, ink, plane)
			if d := upright.diff(sheared); d == 0 {
				t.Fatalf("[%s] %q: the italic frame is pixel-identical to the upright one; the "+
					"control is vacuous and this pin cannot mean anything", appearance, arm.line)
			}
			if d := drawn.diff(upright); d != 0 {
				t.Errorf("[%s] %q: the page draws a shape %d glyph cells away from upright "+
					"(the sheared frame is %d away): the line is still italic",
					appearance, arm.line, d, upright.diff(sheared))
			}
		}
	}
}

// asideMask draws one aside through the page's own builder on the block's own
// face, and returns its normalised glyph mask.
func asideMask(t *testing.T, tokens theme.Tokens, face ui.Color, line string, tail bool, plane int) glyphMask {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(face)
		ui.Column(c).Padding(24).Children(func() {
			(&App{}).runOutputAside(c, tokens, line, tail)
		})
	}, 460, 120)
	tt.Frame()
	box, ok := tt.Find(line)
	if !ok {
		t.Fatalf("the aside builder drew no %q: %q", line, tt.Texts())
	}
	return glyphMaskOf(tt, box, plane)
}

// referenceMasks renders one string upright and sheared on face and returns
// both normalised glyph masks. It is the control for the pin above.
func referenceMasks(t *testing.T, tokens theme.Tokens, face ui.Color, line string, ink ui.Color, plane int) (glyphMask, glyphMask) {
	t.Helper()
	var upright, sheared ui.Rect
	var upRef, shearRef ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(face)
		ui.Column(c).Padding(24).Gap(12).Children(func() {
			ui.Text(c, line).FontSize(tokens.Caption).TextColor(ink).Bind(&upRef)
			ui.Text(c, line).FontSize(tokens.Caption).TextColor(ink).Italic().Bind(&shearRef)
		})
		upright = c.Resolve(upRef).Bounds()
		sheared = c.Resolve(shearRef).Bounds()
	}, 460, 140)
	tt.Frame()
	return glyphMaskOf(tt, upright, plane), glyphMaskOf(tt, sheared, plane)
}

// TestTheEmptyCaptureStaysAheadOfTheTruncationAside pins what carries "this is
// not the program's output" once the shear is gone.
//
// The reference's carrier was `font-style: italic` — "a quiet aside, not the
// capture" (`extensions.css:1259-1267`, `:1268-1280`). Chinese has no italic
// face, so the port was asking the engine to shear an 11px stroke. Two carriers
// survive, and both are already in the copy and in the block:
//
//  1. **The type register.** The capture itself is `runOutputFont` (the
//     monospace stack, `extensions.css:1210-1221`) because it is a program's
//     bytes; the placeholder and the truncation aside are set in the *system*
//     face, so they never read as bytes.
//  2. **The copy.** The placeholder is wrapped in full-width parentheses and the
//     aside opens with an ellipsis — neither shape appears in a program's own
//     output, and both are the reference's own words (`（无输出）` /
//     `……输出已截断（上限 64KB）`).
//
// The ink ladder is the reference's own and is *not* raised here: `--text-muted`
// for the placeholder, `--text-tertiary` for the aside. Measured off the block's
// own face (the sunken recess composited opaque, `runOutputEdgeFace`): light
// **96 / 89**, dark **91 / 75**. Light's 7 levels is thin, and it is recorded
// as such rather than fixed by inventing a fourth ink — raising the placeholder
// would make the block's *empty* voice louder than its own head.
func TestTheEmptyCaptureStaysAheadOfTheTruncationAside(t *testing.T) {
	zh := i18n.For("zh").Settings
	// The type register, in the source: the capture carries runOutputFont and
	// the two asides do not. Read as text because the font choice is a claim
	// about which call site draws what.
	source := readPackageSource(t, "settingsui.go")
	// P13-kf S: unkeyed RichText -> per-result keyed RichText + reader binding;
	// keep the exact font chain pin, including the new identity and binding.
	if !strings.Contains(source, "ui.RichText(c.Key(a.outputKeys[integration.ID]), view.TextSpans(tokens.Appearance, tokens.TextStrong, tokens.Caption)...).\n\t\t\t\t\t\t\tBind(&reader.panel).\n\t\t\t\t\t\t\tFont(runOutputFont)") {
		t.Error("the captured text no longer wears runOutputFont: the monospace register is " +
			"what separates a program's bytes from the two asides")
	}
	// The builder is the one place the two asides are drawn, so it is the one
	// place the register can be checked: a caller cannot smuggle the monospace
	// stack in, and the builder must not set it.
	if !strings.Contains(source, "func (a *App) runOutputAside(") {
		t.Error("the two asides no longer share runOutputAside: the pin below reads that one call site")
	}
	for _, line := range strings.Split(source, "\n") {
		if !strings.Contains(line, "ui.Text(c, line)") {
			continue
		}
		if strings.Contains(line, "runOutputFont") {
			t.Errorf("runOutputAside draws its line in the capture's monospace register: it is "+
				"an aside, not the program's output (%s)", strings.TrimSpace(line))
		}
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Settings{Language: "zh"}, dark, ui.Preferences{})
		face := runOutputEdgeFace(tokens, tokens.Theme)
		plane := luma(face.R, face.G, face.B)
		measure := func(line string, tail bool) int {
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(face)
				(&App{}).runOutputAside(c, tokens, line, tail)
			}, 300, 60)
			tt.SetDark(dark)
			tt.Frame()
			img := tt.Image()
			far := 0
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					i := y*img.Stride + x*4
					l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
					d := l - plane
					if d < 0 {
						d = -d
					}
					if d > far {
						far = d
					}
				}
			}
			return far
		}
		empty := measure(zh.RunNoOutput, false)
		tail := measure(zh.RunOutputTruncatedTail, true)
		if empty < tail {
			t.Errorf("[%s] the empty placeholder stands %d levels off the block's face, the "+
				"truncation aside %d: the block's own voice must not be quieter than its aside",
				appearance, empty, tail)
		}
		if tail < 60 {
			t.Errorf("[%s] the truncation aside stands only %d levels off the block's face: "+
				"an 11px caption aside needs more than that", appearance, tail)
		}
	}
}

// readPackageSource is the package's own file, read as text.
func readPackageSource(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name) // #nosec G304 -- the package's own source
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// The P13-bh visual evidence: the block in its empty and truncated arms, both
// appearances, so the report can point at a file instead of describing one.
//
// Opt-in like the other dumps: a test run should not litter /tmp.
func TestDumpP13bhRunOutputAsides(t *testing.T) {
	if os.Getenv("FLOTER_P13BH_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13BH_DUMP=1 to write /tmp/floter-P13-bh-*.png")
	}
	tag := os.Getenv("FLOTER_P13BH_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, arm := range []struct {
			name   string
			result RunResult
		}{
			{"empty", RunResult{Success: true}},
			{"truncated", RunResult{Stdout: "one\ntwo\nthree\n", Success: true, Truncated: true}},
		} {
			store := settings.NewStore(settings.Settings{Language: "zh", Theme: appearance})
			a := New(store, Actions{})
			a.Integrations = func() []Integration {
				return []Integration{{
					ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
					Enabled: true, Running: true, RuntimeAvailable: true,
					Permissions: []string{"environment"},
					Enforced:    map[string]bool{"environment": true},
					OutputMode:  "background",
				}}
			}
			a.SetRunResult("dev.floter.tool", arm.result)
			a.setRunOutputOpen("dev.floter.tool", true)
			a.selectPage(PageIntegrations)
			tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				a.View(c)
			}, 720, 900)
			tt.SetDark(dark)
			tt.Frame()
			tt.Frame()
			writePNG(t, filepath.Join("/tmp", "floter-P13-bh-asides-"+appearance+"-"+arm.name+"-"+tag+".png"), tt)
		}
	}
}

// frameDiffPNG is kept beside the dump: two frames of the same surface, and
// the count and bounding box of the pixels that changed. The report quotes it.
func frameDiffPNG(t *testing.T, before, after *image.RGBA) (int, ui.Rect) {
	t.Helper()
	n := 0
	minX, minY, maxX, maxY := 1<<30, 1<<30, 0, 0
	for y := 0; y < before.Bounds().Dy(); y++ {
		for x := 0; x < before.Bounds().Dx(); x++ {
			i := y*before.Stride + x*4
			if before.Pix[i] == after.Pix[i] && before.Pix[i+1] == after.Pix[i+1] &&
				before.Pix[i+2] == after.Pix[i+2] {
				continue
			}
			n++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if n == 0 {
		return 0, ui.Rect{}
	}
	return n, ui.Rect{X: float32(minX), Y: float32(minY),
		W: float32(maxX - minX + 1), H: float32(maxY - minY + 1)}
}
