package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// A group heading is one element on both surfaces: the reference's
// `.launcher-section-title` is printed above the launcher's own blocks (the
// "Recently launched" block) and above a plugin list's group, with the same
// type register. This pins the output view's heading to the launcher list's
// section title — same font step, same line box — and to its own text box
// rather than the full width of the row it labels.
func TestOutputGroupHeadingSharesTheLauncherSectionRegister(t *testing.T) {
	// The launcher list's own block title, drawn by the row's heading branch.
	a := testApp()
	a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
	a.SetRecent([]string{"/Applications/Editor.app"}, true)
	// A taller window so the recents block and its heading are not clipped.
	lt := ui.NewTester(func(c *ui.Context) { a.View(c) },
		InputWindowWidth, int(WindowHeight("small"))+500)
	lt.Frame()
	want, ok := lt.Find("Recently launched")
	if !ok {
		t.Fatalf("the launcher's section title is missing: %q", lt.Texts())
	}

	// The output view's group heading, on the same theme and scale.
	b := testApp()
	stdout := `[` +
		`{"id":"a","title":"Alpha","group":"First"},` +
		`{"id":"b","title":"Beta"}` +
		`]`
	ot := runListOutput(t, b, stdout)
	got, ok := ot.Find("First")
	if !ok {
		t.Fatalf("the group heading is missing: %q", ot.Texts())
	}
	if got.H != want.H {
		t.Errorf("the output group heading is %v tall, the launcher's section title is %v",
			got.H, want.H)
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
