package launcher

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// P13-bg · 捕获输出页条子上的那一行：三种结局长得一模一样
// （`docs/rounds/P13-bg-spec.md`、`docs/liquid-glass.md` §6.26）。
//
// 改前实测（720 帧、真启动器地面；地面浅 253 / 深 23）：
//
//	Finished    TextMuted  浅 116（对地面 137） / 深 164（141）
//	Exit code 3 TextMuted  浅 116（137）        / 深 164（141）
//	Timed out   TextMuted  浅 116（137）        / 深 164（141）
//
// 三个臂一个字节不差：`OutputView.Status` 只是一串字，条子上没有任何东西知道这次
// 运行是成功还是失败。而且结局与它自己的键位图例共用同一档墨、同一个字重。
//
// 改后（同一帧）：结局 `TextSecondary` + 580（失败换 `t.Warning`，色相是语义）、
// 中点 `TextTertiary` 0.7、图例 `TextTertiary` + 500：
//
//	成功  结局 145 / 133    图例 109 / 89   （差 36 / 44）
//	失败  结局 139 / 115    图例 109 / 89   （差 30 / 26，且色相 r−b ≥ 90）

// statusProbe draws the capture-output page for one run and reads its strip.
//
// The frame's pixels are read **once** into `img`: `Tester.Image` copies the
// whole framebuffer on every call, and a per-pixel read through it turned this
// probe into minutes of memcpy.
type statusProbe struct {
	img     *image.RGBA
	ground  ui.Color
	lineBox ui.Rect
	verdict ui.Color
	legend  ui.Color
}

// statusArms are the three endings the strip can report.
func statusArms() []struct {
	name string
	run  extensions.CapturedRun
} {
	return []struct {
		name string
		run  extensions.CapturedRun
	}{
		{"finished", extensions.CapturedRun{Command: []string{"tool"}, Success: true}},
		{"failed", extensions.CapturedRun{Command: []string{"tool"}, ExitCode: 3}},
		{"timedout", extensions.CapturedRun{Command: []string{"tool"}, TimedOut: true, ExitCode: -1}},
	}
}

// statusLine is the strip's one line, as `Tester.Find` labels it.
func statusLine(a *App) string {
	return a.output.Status + "  \u00b7  " + a.outputHint(a.copy(), a.output)
}

func probeStatus(t *testing.T, dark bool, arm struct {
	name string
	run  extensions.CapturedRun
}) statusProbe {
	t.Helper()
	a := testApp()
	a.Actions.PinText = func(string, string) {}
	a.Actions.DetachOutput = func(OutputView) {}
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	run := arm.run
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		run.Stdout = "line 00\nline 01\nline 02"
		done(run, nil)
	}
	tt := render(t, a)
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.SetDark(dark)
	tt.Frame()
	box, ok := tt.Find(statusLine(a))
	if !ok {
		t.Fatalf("%s dark=%v: the strip's line is missing: %q", arm.name, dark, tt.Texts())
	}
	img := tt.Image()
	ground := statusPixel(img, 5, 5)
	verdict, legend := statusSplit(img, box, ground)
	return statusProbe{img: img, ground: ground, lineBox: box, verdict: verdict, legend: legend}
}

func statusPixel(img *image.RGBA, x, y int) ui.Color {
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

func statusLuma(c ui.Color) int { return (int(c.R) + int(c.G) + int(c.B)) / 3 }

// statusSplit reads the line's two halves: the darkest ink to the left of the
// widest blank gap inside the line's own ink extent (the verdict), and the
// darkest ink to the right of it (the legend). The gap is the reference's own
// `·` column, which no glyph reaches; nothing here knows which ink the renderer
// chose, so the two halves are read as "whatever is drawn there".
func statusSplit(img *image.RGBA, box ui.Rect, ground ui.Color) (ui.Color, ui.Color) {
	g := statusLuma(ground)
	ink := func(c ui.Color) bool {
		if c.A == 0 {
			return false
		}
		d := g - statusLuma(c)
		if d < 0 {
			d = -d
		}
		return d > 6
	}
	cols := int(box.W)
	hasInk := make([]bool, cols)
	for i := 0; i < cols; i++ {
		x := int(box.X) + i
		for y := int(box.Y); y < int(box.Y+box.H); y++ {
			if ink(statusPixel(img, x, y)) {
				hasInk[i] = true
				break
			}
		}
	}
	first, last := -1, -1
	for i, v := range hasInk {
		if v {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return ui.Color{}, ui.Color{}
	}
	gapAt, gapLen, run := 0, 0, -1
	for i := first; i <= last; i++ {
		if !hasInk[i] {
			if run < 0 {
				run = i
			}
			if i-run+1 > gapLen {
				gapAt, gapLen = run, i-run+1
			}
		} else {
			run = -1
		}
	}
	darkest := func(x0, x1 int) ui.Color {
		var out ui.Color
		best := -1
		for x := x0; x < x1; x++ {
			for y := int(box.Y); y < int(box.Y+box.H); y++ {
				c := statusPixel(img, x, y)
				if !ink(c) {
					continue
				}
				d := g - statusLuma(c)
				if d < 0 {
					d = -d
				}
				if d > best {
					best, out = d, c
				}
			}
		}
		return out
	}
	return darkest(int(box.X), int(box.X)+gapAt), darkest(int(box.X)+gapAt+gapLen, int(box.X)+cols)
}

// statusWeight is how heavy an ink reads on its ground.
func statusWeight(ground, ink ui.Color) int {
	d := statusLuma(ground) - statusLuma(ink)
	if d < 0 {
		d = -d
	}
	return d
}

// TestTheVerdictIsNotTheSameLineWhicheverWayTheRunWent pins the first half:
// a failed run and a successful one must not draw the strip identically. The
// fault keeps a warm hue (a semantic colour, §6.19) and the success stays
// neutral, so the two arms differ in hue even where their weights are close.
func TestTheVerdictIsNotTheSameLineWhicheverWayTheRunWent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		arms := statusArms()
		finished := probeStatus(t, dark, arms[0])
		failed := probeStatus(t, dark, arms[1])
		timedOut := probeStatus(t, dark, arms[2])
		for _, fault := range []statusProbe{failed, timedOut} {
			if d := int(fault.verdict.R) - int(fault.verdict.B); d < 40 {
				t.Errorf("dark=%v: the fault verdict is not in a warm hue (r−b = %d, want ≥ 40; ink %v)",
					dark, d, fault.verdict)
			}
			if d := int(finished.verdict.R) - int(finished.verdict.B); d < -12 || d > 12 {
				t.Errorf("dark=%v: the success verdict is not a neutral ink (r−b = %d, want |·| ≤ 12; ink %v)",
					dark, d, finished.verdict)
			}
			// The whole line differs, not just the hue: the two arms must not
			// be pixel-identical, which is what they were before this round.
			if fault.verdict == finished.verdict {
				t.Errorf("dark=%v: the failed run draws the same verdict ink as the successful one (%v)",
					dark, fault.verdict)
			}
		}
		// Both fault arms agree with each other (a timeout is a fault too).
		if d := int(failed.verdict.R) - int(timedOut.verdict.R); d < -6 || d > 6 {
			t.Errorf("dark=%v: the two fault arms disagree (%v vs %v)", dark, failed.verdict, timedOut.verdict)
		}
	}
}

// TestTheOutputStatusLineSeparatesItsVerdictFromItsLegend pins the second half:
// the verdict is the page's conclusion and the legend is its instruction, so the
// verdict is the heavier of the two and clears it by a rung, in both appearances
// and on both the success and the fault arm.
func TestTheOutputStatusLineSeparatesItsVerdictFromItsLegend(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, arm := range statusArms() {
			p := probeStatus(t, dark, arm)
			gap := statusWeight(p.ground, p.verdict) - statusWeight(p.ground, p.legend)
			if gap < 20 {
				t.Errorf("dark=%v %s: the verdict does not outrank its own legend (gap %d, want ≥ 20; verdict %v, legend %v)",
					dark, arm.name, gap, p.verdict, p.legend)
			}
			if gap > 120 {
				t.Errorf("dark=%v %s: the legend fell off the ladder (gap %d, want ≤ 120)", dark, arm.name, gap)
			}
			// Same register: the two halves are one line, so one line box.
			if p.lineBox.H < 15 {
				t.Errorf("dark=%v %s: the strip's line collapsed (height %.1f)", dark, arm.name, p.lineBox.H)
			}
		}
	}
}

// TestTheStripDrawsItsVerdictFromTheRunOutcome is the structural pin: the view
// carries the fact, the strip reads it, and the line is drawn as more than one
// span. A single `ui.Text` cannot give its two halves two inks, so its absence
// is the shape this round put in.
func TestTheStripDrawsItsVerdictFromTheRunOutcome(t *testing.T) {
	src, err := os.ReadFile("output.go")
	if err != nil {
		t.Fatalf("cannot read output.go: %v", err)
	}
	text := string(src)
	for _, want := range []string{
		"Failed:     run.TimedOut || !run.Success,",
		"func (a *App) outputStatusSpans(",
		"ui.RichText(c, a.outputStatusSpans(c, copy, view)...)",
		"verdict = t.Warning",
		"{Text: a.outputHint(copy, view), Weight: 500, Color: tokens.TextTertiary}",
	} {
		if !containsLine(text, want) {
			t.Errorf("output.go no longer carries %q", want)
		}
	}
	// And the old one-ink line is gone: `ui.Text(...)` on `view.Status` was
	// the shape that made the three endings identical.
	if containsLine(text, "ui.Text(c, view.Status+") {
		t.Error("output.go still draws the strip's line as one `ui.Text`")
	}
}

func containsLine(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && (indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestDumpP13BGStatusStripPNGs writes the round's visual evidence: the three
// endings, both appearances, as whole frames.
//
// Opt-in (`FLOTER_P13BG_DUMP=1`) for the same reason as the other dumps.
func TestDumpP13BGStatusStripPNGs(t *testing.T) {
	if os.Getenv("FLOTER_P13BG_DUMP") == "" {
		t.Skip("set FLOTER_P13BG_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		backdrop := color.RGBA{R: 0xDF, G: 0xDF, B: 0xE4, A: 0xFF}
		if dark {
			appearance = "dark"
			backdrop = color.RGBA{R: 0x14, G: 0x14, B: 0x16, A: 0xFF}
		}
		for _, arm := range statusArms() {
			p := probeStatus(t, dark, arm)
			out := composited(p.img, backdrop)
			path := fmt.Sprintf("/tmp/floter-P13-bg-outputstatus-%s-%s-%s.png", appearance, arm.name, tag)
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("cannot create %s: %v", path, err)
			}
			if err := png.Encode(f, out); err != nil {
				f.Close()
				t.Fatalf("cannot write %s: %v", path, err)
			}
			f.Close()
			t.Logf("wrote %s", path)
		}
	}
}

// composited lays a headless frame over the backdrop the window would sit on.
func composited(img *image.RGBA, backdrop color.RGBA) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := y*img.Stride + x*4
			a := float32(img.Pix[i+3]) / 255
			for k := 0; k < 3; k++ {
				base := []uint8{backdrop.R, backdrop.G, backdrop.B}[k]
				out.Pix[i+k] = uint8(float32(img.Pix[i+k])*a + float32(base)*(1-a))
			}
			out.Pix[i+3] = 0xFF
		}
	}
	return out
}
