package launcher

import (
	"testing"
)

// A group heading is one element on both surfaces: the reference's
// `.launcher-section-title` is printed above the launcher's own blocks (the
// "Dropped files" block) and above a plugin list's group, with the same
// type register. This pins the output view's heading to the launcher list's
// section title — same font step, same line box — and to its own text box
// rather than the full width of the row it labels.
func TestOutputGroupHeadingSharesTheLauncherSectionRegister(t *testing.T) {
	// The launcher list's own block title, drawn by the row's heading branch.
	a := testApp()
	droppedHeadingPage(a, 1)
	// P13-dw：两半都跑出货尺。此前这一半是裸 `ui.NewTester` 加一个「别裁掉标题」的
	// 高于库存帧高的窗口（22 的行盒），另一半已经是出货尺（17）—— 这条 pin 因此
	// 量的是混排帧里一个在生产上不存在的差。
	lt := renderShipped(t, a, false)
	want, ok := lt.Find("Dropped files")
	if !ok {
		t.Fatalf("the launcher's section title is missing: %q", lt.Texts())
	}

	// The output view's group heading, on the same theme and scale.
	b := testApp()
	stdout := `[` +
		`{"id":"a","title":"Alpha","group":"First"},` +
		`{"id":"b","title":"Beta"}` +
		`]`
	ot := listOutputShipped(t, b, stdout, false)
	got, ok := ot.Find("First")
	if !ok {
		t.Fatalf("the group heading is missing: %q", ot.Texts())
	}
	if got.H != want.H {
		t.Errorf("the output group heading is %v tall, the launcher's section title is %v",
			got.H, want.H)
	}
	// Both surfaces agreeing is one thing; landing on the register the reference
	// names is another. The section title prints at the subtitle register — the
	// line box the app measures for its body size — and this pins it there, so a
	// round that moves both headings together still has to say so.
	if _, _, _, subtitle := designMetrics(); tenths(float64(got.H)) != tenths(subtitle) {
		t.Errorf("the group heading's line box is %v tall; the subtitle register is %v",
			got.H, subtitle)
	}
	// The heading is the text's own box, not the row's full width: the old
	// heading applied its padding straight to the text, so the row stretched
	// it across the whole list.
	row, ok := ot.Find("Alpha")
	if !ok {
		t.Fatalf("the first row is missing: %q", ot.Texts())
	}
	if got.W >= row.W {
		t.Errorf("the group heading spans %v of the %v-wide row; it should be its own text box",
			got.W, row.W)
	}
}
