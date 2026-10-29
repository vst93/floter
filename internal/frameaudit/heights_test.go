package frameaudit

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P11-bz · 这把尺子的尺子：用**手工高度**问它四个问题，不碰任何帧。
//
// 上一把尺子（`frameaudit_test.go`，P11-by）也是这么守的：规则本身要能被
// 手写的输入证伪，否则「全绿」只说明它没在看。四个问题分别是——阶梯外的
// 高度必须被报出、阶梯内（含容差边界）必须放过、缺了一档必须被报出、
// 只有叶子 run 的高度进这张表（容器的盒子是它画的框，不是一行字）。

func TestTheLadderOnlyKnowsItsRungs(t *testing.T) {
	rungs := []float32{15, 18, 22, 25}
	if got := Ladder(rungs, 0.5, []float32{15, 18.4, 22, 25}); len(got) != 0 {
		t.Fatalf("heights on the ladder were reported: %v", got)
	}
	got := Ladder(rungs, 0.5, []float32{15, 20})
	if len(got) != 1 || !strings.Contains(got[0], "20.0") {
		t.Fatalf("an off-ladder height was not reported: %v", got)
	}
}

func TestTheLadderToleranceIsHalfADIP(t *testing.T) {
	rungs := []float32{18}
	if got := Ladder(rungs, 0.5, []float32{18.5}); len(got) != 0 {
		t.Fatalf("18.5 is within half a DIP of the 18 rung: %v", got)
	}
	if got := Ladder(rungs, 0.5, []float32{18.6}); len(got) != 1 {
		t.Fatalf("18.6 is more than half a DIP off the 18 rung: %v", got)
	}
}

func TestAMissingRungIsReported(t *testing.T) {
	if got := Missing([]float32{15, 18, 22}, 0.5, []float32{15, 22}); len(got) != 1 || !strings.Contains(got[0], "18.0") {
		t.Fatalf("a rung the frame never painted was not reported: %v", got)
	}
	if got := Missing([]float32{15, 18}, 0.5, []float32{15, 18.2}); len(got) != 0 {
		t.Fatalf("18.2 is the 18 rung within tolerance: %v", got)
	}
}

// A label that wraps other runs is a container, and its box is the frame it drew
// (a card's, a list's), not a line of text. Only leaves make it onto the ladder.
func TestHeightsReadsLeavesOnly(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Label("panel").Padding(10).Children(func() {
			ui.Text(c, "one").FontSize(20)
			ui.Text(c, "two").FontSize(10)
		})
	}, 200, 200)
	tt.Frame()
	panel, ok := tt.Find("panel")
	if !ok {
		t.Fatalf("the labeled column was not registered: %v", tt.Texts())
	}
	heights := Heights(tt, nil)
	if len(heights) != 2 {
		t.Fatalf("heights = %v, want the two text runs and not the container", heights)
	}
	for _, h := range heights {
		if h == round1(panel.H) {
			t.Fatalf("the container's box (%.1f) made it onto the ladder: %v", panel.H, heights)
		}
	}
	if heights[0] >= heights[1] {
		t.Fatalf("heights are not ascending: %v", heights)
	}
	if got := Heights(tt, func(r Run) bool { return r.R.H >= MinHeight }); len(got) != 2 {
		t.Fatalf("both runs clear the fragment floor: %v", got)
	}
}

// One rung is one entry: a page of six rows of the same size is one height, not
// six, so the ladder stays readable when a page repeats a row twenty times.
func TestARepeatedHeightIsOneRung(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			ui.Text(c, "a").FontSize(12)
			ui.Text(c, "b").FontSize(12)
			ui.Text(c, "c").FontSize(12)
		})
	}, 200, 200)
	tt.Frame()
	if got := Heights(tt, nil); len(got) != 1 {
		t.Fatalf("three rows of one size are one rung: %v", got)
	}
}
