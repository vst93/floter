package launcher

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-x · 分组头也参加折边（`docs/rounds/P13-x-spec.md`）。
//
// P13-v 把「折边之外的东西是退后，不是被切」落在启动器列表的行上，P13-w 把它
// 接给捕获输出页的行；两轮都漏了**分组头**。分组头在代码里是 `row`/`outputRow`
// 的**兄弟**元素（`launcher.go:2250` 那个 `return`、`output.go` 的 `row.Group` 臂），
// 行才经过 `Opacity`，标签从来没有——于是折边切到标签的时候它保持满墨。
//
// 实测（720 宽的帧）：启动器的 `Recently launched` 在露出 1/4 和露出 3/4 时
// contrast **逐像素相同**（浅色 145/145，深色 133/133）；捕获输出的 `Group B`
// 只剩 8/22u 时 134、整行可见 137。这不是「下面还有」，这是「界面坏了」。
//
// 这些 pin 量的是**同一段像素在不同折边下的对比度**，与 P13-w 的
// `contrastIn` 同一个读法：0 表示什么都没画，数字越大墨越重；它对外观无关，
// 因为量的是「离这一页自己的面有多远」，不是「有多黑」。

// headingRows builds a captured-output payload of n rows in two named groups.
func headingRows(n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		group := "Group A"
		if i >= n/2 {
			group = "Group B"
		}
		fmt.Fprintf(&b, `{"id":"r%d","title":"Row %d","group":%q,"action":{"type":"insert","text":"picked %d"}}`,
			i, i, group, i)
	}
	b.WriteString("]")
	return b.String()
}

// headingEvidence is one reading of a group heading: the ink of its own text box
// at a given fold, on one appearance.
type headingEvidence struct {
	ink   int
	fold  float64
	label string
}

// launcherHeadingAt drives the launcher's own list until its section title is
// cut by the top fold, and reports the ink of that label.
func launcherHeadingAt(t *testing.T, dark bool, height int) headingEvidence {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) {
		s.Theme = map[bool]string{true: "dark", false: "light"}[dark]
	}); err != nil {
		t.Fatal(err)
	}
	recentPage(a, 20)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, height)
	tt.SetDark(dark)
	tt.Frame()
	// Walk the keyboard until the heading is on the fold. The list reveals the
	// choice on every step, so a tall frame needs more steps than a short one;
	// the heading leaves the list entirely if we overshoot.
	for step := 0; step <= 20; step++ {
		box, seen := a.headingRects[0]
		fold := foldAlpha(a.listRect, box)
		rect, onScreen := tt.Find("Recently launched")
		if seen && onScreen && fold > 0 && fold < 1 {
			return headingEvidence{
				ink:   contrastIn(tt.Image(), rect, a.listRect),
				fold:  fold,
				label: fmt.Sprintf("height %d, step %d", height, step),
			}
		}
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	t.Fatalf("the launcher's section title never landed on the fold at height %d", height)
	return headingEvidence{}
}

// launcherHeadingWhole is the same label wholly inside the viewport: the
// reference reading the folded one is compared against.
func launcherHeadingWhole(t *testing.T, dark bool) headingEvidence {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) {
		s.Theme = map[bool]string{true: "dark", false: "light"}[dark]
	}); err != nil {
		t.Fatal(err)
	}
	recentPage(a, 20)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, frameHeight(maxResultSlots))
	tt.SetDark(dark)
	tt.Frame()
	rect, ok := tt.Find("Recently launched")
	if !ok {
		t.Fatalf("the section title is missing: %q", tt.Texts())
	}
	box, _ := a.headingRects[0]
	if fold := foldAlpha(a.listRect, box); fold != 1 {
		t.Fatalf("the premise failed: the label is only %.3f in view", fold)
	}
	return headingEvidence{ink: contrastIn(tt.Image(), rect, a.listRect), fold: 1, label: "whole"}
}

// outputHeadingAt is the captured-output page's half: `Group B` cut by the
// list's own clip.
func outputHeadingAt(t *testing.T, dark bool, height int) headingEvidence {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) {
		s.Theme = map[bool]string{true: "dark", false: "light"}[dark]
	}); err != nil {
		t.Fatal(err)
	}
	tt := runListOutputAt(t, a, headingRows(12), height)
	tt.SetDark(dark)
	tt.Frame()
	const second = 6 // the first row of the second group
	box, ok := a.outputHeadingRects[second]
	if !ok {
		t.Fatalf("the second group's heading has no box: %v", a.outputHeadingRects)
	}
	fold := foldAlpha(a.outputListRect, box)
	if !(fold > 0 && fold < 1) {
		t.Fatalf("the premise failed at height %d: the heading is %.3f in view", height, fold)
	}
	rect, onScreen := tt.Find("Group B")
	if !onScreen {
		t.Fatalf("the heading is not on screen at height %d: %q", height, tt.Texts())
	}
	return headingEvidence{
		ink:   contrastIn(tt.Image(), rect, a.outputListRect),
		fold:  fold,
		label: fmt.Sprintf("height %d", height),
	}
}

func outputHeadingWhole(t *testing.T, dark bool) headingEvidence {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) {
		s.Theme = map[bool]string{true: "dark", false: "light"}[dark]
	}); err != nil {
		t.Fatal(err)
	}
	tt := runListOutputAt(t, a, headingRows(12), frameHeight(maxResultSlots))
	tt.SetDark(dark)
	tt.Frame()
	rect, ok := tt.Find("Group B")
	if !ok {
		t.Fatalf("the heading is not on screen: %q", tt.Texts())
	}
	const second = 6
	box := a.outputHeadingRects[second]
	if fold := foldAlpha(a.outputListRect, box); fold != 1 {
		t.Fatalf("the premise failed: the heading is only %.3f in view", fold)
	}
	return headingEvidence{ink: contrastIn(tt.Image(), rect, a.outputListRect), fold: 1, label: "whole"}
}

// The launcher's own section title recedes as the fold cuts it, instead of
// keeping its full ink and being sliced in half.
//
// 阈值按比例给，不写「更淡就行」：M1（分组头不下发墨）实测让折边上的标签停在
// 143/145（浅色）与 131/133（深色）—— 一条「cut < whole」的判据会静默放过它，
// 因为被切掉的那几像素本来就少。折边 0.25 的读数是 36，所以「留下的墨不超过
// 整行可见的一半」既钉得死变异，又不依赖渲染器算出多少像素。
func TestTheLauncherHeadingAtTheFoldRecedes(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		whole := launcherHeadingWhole(t, dark)
		cut := launcherHeadingAt(t, dark, 300)
		t.Logf("whole=%+v cut=%+v", whole, cut)
		if cut.ink > whole.ink/2 {
			t.Errorf("the launcher's section title keeps its full ink at the fold: "+
				"%s contrast %d against %d whole in view (fold %.3f) — the fold "+
				"cuts the heading instead of letting it recede",
				cut.label, cut.ink, whole.ink, cut.fold)
		}
	})
}

// And it is a scale, not a flag: the less of the heading the viewport shows, the
// further it recedes. Same proportional gate as above (the mutant's two readings
// sit 2 apart).
func TestTheLessOfTheLauncherHeadingTheFurtherItRecedes(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		most := launcherHeadingAt(t, dark, 408)
		least := launcherHeadingAt(t, dark, 300)
		t.Logf("most=%+v least=%+v", most, least)
		if least.fold >= most.fold {
			t.Fatalf("the premise failed: the folds did not open (%.3f → %.3f)", least.fold, most.fold)
		}
		if least.ink > most.ink/2 {
			t.Errorf("a heading with %.3f showing keeps as much ink as one with "+
				"%.3f: %d vs %d — the fold is a flag, not a scale",
				least.fold, most.fold, least.ink, most.ink)
		}
	})
}

// The captured-output page's group heading is the same list and the same rule.
func TestTheOutputGroupHeadingAtTheFoldRecedes(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		whole := outputHeadingWhole(t, dark)
		cut := outputHeadingAt(t, dark, 404)
		t.Logf("whole=%+v cut=%+v", whole, cut)
		if cut.ink > whole.ink/2 {
			t.Errorf("the output group heading keeps its full ink at the fold: "+
				"%s contrast %d against %d whole in view (fold %.3f) — this page's "+
				"fold must call the same pure function the launcher's does",
				cut.label, cut.ink, whole.ink, cut.fold)
		}
	})
}

// The other half of the rule: a heading wholly inside the viewport keeps the ink
// it always had. The fold dims what it cuts, and nothing else.
func TestTheHeadingsInsideTheViewportKeepTheirInk(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		if got := launcherHeadingWhole(t, dark); got.ink < 100 {
			t.Errorf("the launcher's section title faded while wholly in view: contrast %d", got.ink)
		}
		if got := outputHeadingWhole(t, dark); got.ink < 100 {
			t.Errorf("the output group heading faded while wholly in view: contrast %d", got.ink)
		}
	})
}

// The heading boxes are measured in their **own** tables, never in the rows'.
// This is the machine reading of the rule the fold's own bookkeeping has to
// respect: `rowRects` feeds `viewportRange`, so a heading mixed into it would
// take a digit slot away from a real row (P13-w's "still to do" #1).
func TestTheHeadingBoxesStayOutOfTheNumberedFamily(t *testing.T) {
	a := testApp()
	recentPage(a, 20)
	tt := render(t, a)
	tt.Frame()

	if len(a.headingRects) == 0 {
		t.Fatalf("no heading box was recorded: %v", a.headingRects)
	}
	for i := range a.headingRects {
		if _, clash := a.rowRects[i]; clash {
			t.Errorf("the heading at index %d also has a row box: the numbered "+
				"family reads rowRects, so a heading in there takes a digit slot "+
				"(rowRects=%v)", i, a.rowRects)
		}
	}
	// The family itself is unchanged: nine recents in a ten-slot budget, 1..9.
	for i := 1; i <= 9; i++ {
		if a.numbers[i] != i%10 {
			t.Errorf("number for row %d = %d, want %d (numbers=%v)", i, a.numbers[i], i%10, a.numbers)
		}
	}
	if len(a.numbers) != 9 {
		t.Errorf("the numbered family has %d entries, want 9 (numbers=%v)", len(a.numbers), a.numbers)
	}
}

// P13-x visual evidence: the heading on the fold on both surfaces, both
// appearances. Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps.
func TestDumpP13XHeadingFoldPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		// The launcher page, walked until its section title is on the fold.
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		recentPage(a, 20)
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, 300)
		tt.SetDark(dark)
		tt.Frame()
		for step := 0; step <= 20; step++ {
			box, seen := a.headingRects[0]
			fold := foldAlpha(a.listRect, box)
			if _, onScreen := tt.Find("Recently launched"); seen && onScreen && fold > 0 && fold < 1 {
				break
			}
			tt.Key(0, ui.KeyDown)
			tt.Frame()
		}
		writeP13X(t, filepath.Join("/tmp", "floter-P13-x-headingfold-"+appearance+".png"), tt)

		// The captured-output page, at a height its second group's heading is
		// cut by.
		b := testApp()
		if err := b.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		ot := runListOutputAt(t, b, headingRows(12), 404)
		ot.SetDark(dark)
		ot.Frame()
		writeP13X(t, filepath.Join("/tmp", "floter-P13-x-outputheading-"+appearance+".png"), ot)
	}
}

func writeP13X(t *testing.T, path string, tt *ui.Tester) {
	t.Helper()
	f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", path)
}
