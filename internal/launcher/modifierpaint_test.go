package launcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// Read the released frame before advancing time or asking for another frame.
// An extra Frame hides a stale viewport: the last row remains faded until an
// unrelated redraw, even though the result and its layout have already returned.
func TestModifierReleaseRestoresRowInkImmediately(t *testing.T) {
	for _, appearance := range []string{"light", "dark"} {
		for _, n := range []int{1, 3, 10} {
			t.Run(fmt.Sprintf("%s/%d", appearance, n), func(t *testing.T) {
				a := testApp()
				recentPage(a, n)
				if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
					t.Fatal(err)
				}
				height := 420
				a.Actions.ResizeTo = func(h int) { height = h }
				tt := ui.NewTester(themedView(a), 720, height)
				tt.SetDark(appearance == "dark")
				tt.Frame()
				tt.SetSize(720, height)
				tt.Advance(time.Second)
				label := fmt.Sprintf("Recent %02d", n-1)
				rect, ok := tt.Find(label)
				viewport := ui.Rect{W: 720, H: float32(height)}
				before := contrastIn(tt.Image(), rect, viewport)
				if !ok || before < 80 || rect.Y+rect.H > viewport.H {
					t.Fatalf("last row must be visible before holding Super: rect=%+v contrast=%d", rect, before)
				}
				for cycle := range 4 {
					for _, hold := range []time.Duration{0, 8 * time.Millisecond, 80 * time.Millisecond, 600 * time.Millisecond} {
						tt.HoldModifiers(ui.Cmd)
						if hold > 0 {
							tt.Advance(hold)
						}
						tt.HoldModifiers(0)
						got, ok := tt.Find(label)
						if !ok || got != rect {
							t.Fatalf("cycle=%d hold=%v: released row moved: %+v -> %+v", cycle, hold, rect, got)
						}
						if ink := contrastIn(tt.Image(), rect, viewport); ink != before {
							t.Fatalf("cycle=%d hold=%v: released row ink=%d, want %d immediately without an extra frame", cycle, hold, ink, before)
						}
					}
				}
			})
		}
	}
}
