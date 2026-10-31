package shell

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/launcher"
)

// P13-bi · 插件窗口里「同一个运行的结局」此前没有画出来
// （`docs/rounds/P13-bi-spec.md`、`docs/liquid-glass.md` §6.28）。
//
// 先量的数（HEAD `2930aca6`，720×460 静息，窗口自己的地面 `tokens.Opaque`
// 浅 246 / 深 20）：三个结局臂的 `Texts()` 都是 `["jv" "Close" "hello"]` ——
// 差不是「小」，是 **0**。`launcher.OutputView.Status` 一直带着结局那句话，
// 但只有启动器自己的条子画过它（§6.26 / P13-bg），插件窗口的正文只读
// `Text` / `Rows` / `failed` 三样。
//
// 改后这一行是**三档**（对窗口地面，浅 / 深）：
//
//	bar 标题 `Caption` + `TextMuted`         131 / 130（§6.24 不动）
//	结局 `TextSecondary` + 580                 145 / 133   ← 这一次运行的结论
//	结局（失败）`t.Warning` + 580                139 / 115   ← 色相是语义（§6.19）
//	事实 `TextTertiary` + 500                  109 /  89   ← 时长与截断，最轻的一件
//	正文 `Body` + `TextSecondary`              139 / 134

// verdictArms are the three outcomes of one run, each with text to show — the
// two arms that reach the band at all.
func verdictArms() []struct {
	name string
	run  extensions.CapturedRun
} {
	return []struct {
		name string
		run  extensions.CapturedRun
	}{
		{"finished", extensions.CapturedRun{Stdout: "hello\n", Success: true, Duration: 400 * time.Millisecond}},
		{"failed", extensions.CapturedRun{Stdout: "hello\n", ExitCode: 3, Duration: 400 * time.Millisecond}},
		{"timedout", extensions.CapturedRun{Stdout: "hello\n", TimedOut: true, Duration: 300 * time.Second}},
	}
}

// verdictFrame is one arm's frame: the window's ground, the band's two inks,
// and the two boxes the measurement was taken from.
type verdictFrame struct {
	tt       *ui.Tester
	view     launcher.OutputView
	ground   ui.Color
	verdict  ui.Color
	facts    ui.Color
	hasFacts bool
}

// verdictProbe renders one arm and reads the band between the window's bar and
// its body.
func verdictProbe(t *testing.T, dark bool, run extensions.CapturedRun) verdictFrame {
	t.Helper()
	a := barApp(t, dark)
	view := a.Launcher.OutputFor(run, nil, nil)
	detached := &detachedOutput{title: "jv", ran: true, view: view}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	ground := shellPixelAt(img, 5, 5)
	title, ok := tt.Find("jv")
	if !ok {
		t.Fatalf("dark=%v: the bar title is missing: %q", dark, tt.Texts())
	}
	body, ok := tt.Find("hello")
	if !ok {
		t.Fatalf("dark=%v: the body text is missing: %q", dark, tt.Texts())
	}
	top := int(title.Y + title.H)
	verdict, facts, hasFacts := verdictBandInks(img, ground, top, int(body.Y))
	return verdictFrame{tt: tt, view: view, ground: ground, verdict: verdict, facts: facts, hasFacts: hasFacts}
}

// verdictBandInks reads the band's two inks the way the eye reads them: the
// darkest ink to the left of the band's widest blank gap (the verdict), and the
// darkest ink to its right (the facts). The gap is the reference's own `·`
// column, which no glyph reaches. Nothing here knows which ink the renderer
// chose — the two halves are read as "whatever is drawn there", so a rewrite
// that draws them in one register cannot pass by agreeing with the test.
func verdictBandInks(img *image.RGBA, ground ui.Color, top, bottom int) (verdict, facts ui.Color, hasFacts bool) {
	luma := func(c ui.Color) int { return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000 }
	g := luma(ground)
	inkAt := func(x, y int) (ui.Color, bool) {
		if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
			return ui.Color{}, false
		}
		i := y*img.Stride + x*4
		c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
		if c.A == 0 {
			return c, false
		}
		d := luma(c) - g
		if d < 0 {
			d = -d
		}
		return c, d > 8
	}
	// The band's own top is the first row after the bar that carries ink but does
	// not span the window: the bar's `InsetShadow` hairline is a full-width row,
	// and reading it as part of the band would fill every column and leave no
	// separator to split on.
	width := img.Rect.Dx()
	bandTop := top
	for y := top; y < bottom; y++ {
		n := 0
		for x := 0; x < width; x++ {
			if _, ok := inkAt(x, y); ok {
				n++
			}
		}
		if n > 0 && n < width*9/10 {
			bandTop = y
			break
		}
	}
	hasInk := make([]bool, width)
	for x := 0; x < width; x++ {
		for y := bandTop; y < bottom; y++ {
			if _, ok := inkAt(x, y); ok {
				hasInk[x] = true
				break
			}
		}
	}
	first, last := -1, -1
	for x, v := range hasInk {
		if v {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first < 0 {
		return ui.Color{}, ui.Color{}, false
	}
	// The widest blank run between the first and last ink column is the
	// separator.
	split, best := first, 0
	run, start := 0, 0
	for x := first; x <= last; x++ {
		if hasInk[x] {
			if run > best {
				best, split = run, start
			}
			run = 0
			continue
		}
		if run == 0 {
			start = x
		}
		run++
	}
	if run > best {
		best, split = run, start
	}
	left := func(x int) bool { return x >= first && x < split }
	right := func(x int) bool { return x > split+best-1 && x <= last }
	deepest := func(keep func(int) bool) (ui.Color, int) {
		var ink ui.Color
		best := -1
		for x := first; x <= last; x++ {
			if !keep(x) {
				continue
			}
			for y := bandTop; y < bottom; y++ {
				c, ok := inkAt(x, y)
				if !ok {
					continue
				}
				if d := shellColorDistance(c, ground); d > best {
					best, ink = d, c
				}
			}
		}
		return ink, best
	}
	verdict, _ = deepest(left)
	rightInk, rightD := deepest(right)
	if rightD <= 0 {
		return verdict, ui.Color{}, false
	}
	return verdict, rightInk, true
}

// TestTheDetachedWindowSaysTheRunsVerdict is the round's own pin: every one of
// the three outcomes is on screen, and the three are **different** — the defect
// was that they were byte-identical (all three said nothing).
func TestTheDetachedWindowSaysTheRunsVerdict(t *testing.T) {
	seen := map[string]bool{}
	for _, arm := range verdictArms() {
		a := barApp(t, false)
		view := a.Launcher.OutputFor(arm.run, nil, nil)
		detached := &detachedOutput{title: "jv", ran: true, view: view}
		tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
		tt.Frame()
		if view.Verdict == "" {
			t.Fatalf("%s: the view carries no verdict at all (launcher.OutputView.Verdict)", arm.name)
		}
		if !tt.HasText(view.Verdict) {
			t.Errorf("%s: the window does not say the run's verdict %q: %q",
				arm.name, view.Verdict, tt.Texts())
		}
		if seen[view.Verdict] {
			t.Errorf("%s: two outcomes share the same sentence %q", arm.name, view.Verdict)
		}
		seen[view.Verdict] = true
	}
}

// TestTheVerdictOutranksItsFacts pins the ladder inside the band: the outcome is
// the page's conclusion, the duration is a fact about it. Measured light 145 vs
// 109, dark 133 vs 89 — a gap of 36 / 44 levels. A rewrite that draws both at
// one register collapses the band into one sentence again.
func TestTheVerdictOutranksItsFacts(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, arm := range verdictArms() {
			r := verdictProbe(t, dark, arm.run)
			if !r.hasFacts {
				t.Fatalf("dark=%v %s: the band has no facts half to compare (verdict %v)", dark, arm.name, r.verdict)
			}
			verdict := bodyWeight(r.ground, r.verdict)
			facts := bodyWeight(r.ground, r.facts)
			if gap := verdict - facts; gap < 20 {
				t.Errorf("dark=%v %s: the verdict does not outrank its facts (gap %d, want ≥ 20; verdict %v, facts %v)",
					dark, arm.name, gap, r.verdict, r.facts)
			}
		}
	}
}

// TestTheFailedRunKeepsTheWarningHue pins the semantic half: a run that failed
// is not "more important text", it is a different kind of sentence, so its
// verdict wears the warning hue rather than the neutral ink.
func TestTheFailedRunKeepsTheWarningHue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, arm := range verdictArms() {
			r := verdictProbe(t, dark, arm.run)
			warmth := int(r.verdict.R) - int(r.verdict.B)
			failed := arm.name != "finished"
			if failed && warmth < 25 {
				t.Errorf("dark=%v %s: the verdict lost the warning hue (r−b = %d, want ≥ 25; ink %v)",
					dark, arm.name, warmth, r.verdict)
			}
			if !failed && (warmth > 8 || warmth < -8) {
				t.Errorf("dark=%v %s: a successful run's verdict is not the neutral ink (r−b = %d; ink %v)",
					dark, arm.name, warmth, r.verdict)
			}
		}
	}
}

// TestTheStateLinesHaveNoVerdictBand pins the guard: the four state lines
// (`running` / `failed` / empty / idle) are already this page's conclusion
// (§6.24), so a run whose body is one of those sentences must not grow a second
// verdict band above them — one page, one statement.
//
// The empty arm is the synthetic one: `OutputFor` always fills a text body (the
// reference's own "The command wrote nothing"), so the state-line arm is only
// reachable from a view built by hand — which is exactly what the body's switch
// falls through to, and what this pins.
func TestTheStateLinesHaveNoVerdictBand(t *testing.T) {
	cases := []struct {
		name    string
		running bool
		failed  string
		ran     bool
		run     extensions.CapturedRun
		view    *launcher.OutputView
	}{
		{name: "running", running: true, ran: true, run: extensions.CapturedRun{Success: true}},
		{name: "refused", failed: "Could not start /opt/bin/node: no such file", ran: true, run: extensions.CapturedRun{Success: true}},
		{name: "empty", ran: true, view: &launcher.OutputView{Title: "jv", Verdict: "Finished"}},
		{name: "idle", run: extensions.CapturedRun{Success: true}},
	}
	for _, dark := range []bool{false, true} {
		for _, tc := range cases {
			a := barApp(t, dark)
			view := a.Launcher.OutputFor(tc.run, nil, nil)
			if tc.view != nil {
				view = *tc.view
			}
			if view.Verdict == "" {
				t.Fatalf("%s: the fixture lost its verdict, so this pin proves nothing", tc.name)
			}
			detached := &detachedOutput{
				title: "jv", running: tc.running, failed: tc.failed, ran: tc.ran, view: view,
			}
			tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
			tt.SetDark(dark)
			tt.Frame()
			if tt.HasText(view.Verdict) {
				t.Errorf("dark=%v %s: a state-line body grew a verdict band (%q): %q",
					dark, tc.name, view.Verdict, tt.Texts())
			}
		}
	}
}

// TestTheVerdictBandIsOneFunction pins the structural half: the band has a
// home of its own, the split lives in the launcher, and the window draws the
// two halves rather than the joined sentence.
func TestTheVerdictBandIsOneFunction(t *testing.T) {
	data, err := os.ReadFile("detach.go")
	if err != nil {
		t.Fatalf("read detach.go: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		"func (a *App) detachedVerdict(",
		"a.detachedVerdict(c, detached, tokens, t)",
		"spans := []ui.Span{{Text: view.Verdict, Weight: 580, Color: verdict}}",
		"ui.Span{Text: view.Facts, Weight: 500, Color: tokens.TextTertiary}",
	} {
		if !contains(src, want) {
			t.Errorf("the band's %q is missing from detach.go", want)
		}
	}
	// The window must not fall back to the joined sentence: `view.Status` is the
	// launcher strip's single line (§6.26) and drawing it here would put both
	// halves back on one ink.
	if contains(src, "view.Status") {
		t.Errorf("detach.go draws the joined `view.Status`, which re-flattens the band")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestDumpTheVerdictBand writes both appearances × the three outcomes so a
// report can show them side by side. Opt-in.
func TestDumpTheVerdictBand(t *testing.T) {
	if os.Getenv("FLOTER_P13BI_DUMP") == "" {
		t.Skip("set FLOTER_P13BI_DUMP=1 to dump the plugin window's verdict band")
	}
	tag := os.Getenv("FLOTER_P13BI_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, arm := range verdictArms() {
			a := barApp(t, dark)
			view := a.Launcher.OutputFor(arm.run, nil, nil)
			detached := &detachedOutput{title: "jv", ran: true, view: view}
			tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
			tt.SetDark(dark)
			tt.Frame()
			name := fmt.Sprintf("/tmp/floter-P13-bi-band-%s-%s-%s.png", appearance, arm.name, tag)
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
