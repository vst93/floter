package launcher

import (
	"testing"

	"floter/internal/apps"
)

// The list scrolls with the wheel: a wheel event moves the offset, and the
// rows move with it.
func TestScrollingTheResultList(t *testing.T) {
	a := testApp()
	items := make([]apps.App, 30)
	for i := range items {
		items[i] = apps.App{Name: "App" + string(rune('A'+i%26)) + string(rune('0'+i/26)), Path: "/app/i"}
	}
	a.SetApps(items)
	a.Query = ""
	tt := render(t, a)
	tt.Frame()
	if len(a.Results()) == 0 {
		t.Fatal("no results")
	}
	// Scroll down: the first visible row changes.
	before := tt.Texts()
	tt.Scroll(340, 200, 0, 300)
	tt.Frame()
	after := tt.Texts()
	if len(before) == 0 || len(after) == 0 {
		t.Fatalf("texts %v %v", before, after)
	}
	same := len(before) == len(after)
	if same {
		for i := range before {
			if before[i] != after[i] {
				same = false
				break
			}
		}
	}
	if same && len(a.Results()) > 10 {
		t.Errorf("scrolling did not move the list: %v", after)
	}
}
