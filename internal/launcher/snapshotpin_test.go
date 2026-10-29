package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-co · **一帧只拷一份设置**（启动器，对照臂）。
//
// The launcher grew this memo first (`App.settings`), and it is the surface the
// other two now share the type with (`settings.Memo`). The arm below pins what
// the shared type has to keep true here: a steady frame copies nothing, and a
// frame after a change copies once — the launcher's 19 readers all go through
// `a.settings()`, so there is no second copy anywhere in the frame.
func TestALauncherFrameCopiesTheSettingsAtMostOnce(t *testing.T) {
	a := testApp()
	a.Query = "application 4"
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()

	// The first frame has already taken the value; a steady frame takes none.
	before := a.Store.Snapshots()
	tt.Frame()
	if got := a.Store.Snapshots() - before; got != 0 {
		t.Errorf("a steady launcher frame took %d settings snapshots, want 0", got)
	}

	// A change is a new version: exactly one copy, however many rows draw.
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("the update failed: %v", err)
	}
	before = a.Store.Snapshots()
	tt.Frame()
	if got := a.Store.Snapshots() - before; got != 1 {
		t.Errorf("a launcher frame after a change took %d settings snapshots, want 1", got)
	}
}
