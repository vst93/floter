package launcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestRapidModifierKeepsWindowFieldAndVisibleRowsStill(t *testing.T) {
	for _, query := range []string{"", "Recent", "command-without-matches"} {
		for _, count := range []int{0, 1, 3, 10} {
			t.Run(fmt.Sprintf("%s/%d", query, count), func(t *testing.T) {
				a := testApp()
				recentPage(a, count)
				a.Query = query
				h := 420
				a.Actions.ResizeTo = func(next int) { h = next }
				tt := ui.NewTester(themedView(a), 720, h)
				tt.Frame()
				tt.SetSize(720, h)
				tt.Frame()
				tt.Advance(time.Second)
				base := h
				field, _ := tt.Find(a.copy().Label)
				row, hasRow := tt.Find("Recent 00")
				if hasRow && count > 1 {
					a.Selected = resultIndex(a, "Recent 01")
					tt.Frame()
				}
				selection := a.Selected
				for i := 0; i < 16; i++ {
					tt.HoldModifiers(ui.Cmd)
					tt.Frame()
					if h != base {
						t.Fatalf("press requested native resize %d -> %d", base, h)
					}
					got, _ := tt.Find(a.copy().Label)
					if got != field {
						t.Fatalf("field moved: %+v -> %+v", field, got)
					}
					if hasRow && count > 1 {
						got, ok := tt.Find("Recent 00")
						if !ok || got.Y != row.Y {
							t.Fatalf("visible row moved: %+v -> %+v", row, got)
						}
					}
					bar, ok := tt.Find(a.copy().HeldTerminalHint)
					if query == "" && (!ok || bar.Y+bar.H > float32(base)) {
						t.Fatalf("footer outside window: %+v h=%d", bar, base)
					}
					tt.HoldModifiers(0)
					tt.Frame()
					if h != base || a.Selected != selection {
						t.Fatalf("release height=%d/%d selection=%d/%d", h, base, a.Selected, selection)
					}
					tt.Advance(8 * time.Millisecond)
				}
			})
		}
	}
}

func TestModifierReleasePreservesScrolledPage(t *testing.T) {
	a := testApp()
	recentPage(a, 10)
	tt := ui.NewTester(themedView(a), 720, 260)
	tt.Frame()
	tt.Advance(time.Second)
	a.List.ScrollTo(3, ui.Start)
	tt.Frame()
	before, ok := tt.Find("Recent 03")
	if !ok {
		t.Fatal("scrolled fixture missing")
	}
	for i := 0; i < 10; i++ {
		tt.HoldModifiers(ui.Cmd)
		tt.Frame()
		tt.HoldModifiers(0)
		tt.Frame()
		after, ok := tt.Find("Recent 03")
		if !ok || before.Y != after.Y {
			t.Fatalf("release scrolled page: %+v -> %+v", before, after)
		}
	}
}

func TestRecentAppsHaveNoSectionTitle(t *testing.T) {
	a := testApp()
	recentPage(a, 3)
	for _, item := range a.Results() {
		if item.heading {
			t.Fatalf("recent page has heading %q", item.Title)
		}
	}
	if len(a.Results()) != 3 {
		t.Fatalf("recent apps lost: %v", a.Results())
	}
}
