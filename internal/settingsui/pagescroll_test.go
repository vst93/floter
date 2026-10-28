package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// settingsScrolledBody is a settings surface whose General page has been
// scrolled by one wheel notch: the precondition every pin below shares, with
// its own check so a frame that never scrolled cannot pass them vacuously.
func settingsScrolledBody(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a := New(newStore(t), Actions{})
	tt, _ := renderIn(t, a, 720, 420, false)
	tt.Wheel(400, 250, 0, 240)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if a.Body.Y == 0 {
		t.Fatalf("the General page did not scroll (Y = %v, max %v)", a.Body.Y, a.Body.MaxY)
	}
	return a, tt
}

// TestANewPageStartsAtTheTop: the reference keys the page's scroller with the
// page (`main:src/App.tsx:2930`: `<main className="settings-content" ...
// key={settingsPage}>`, and `.settings-content` is the scroller at
// `main:src/styles/settings.css:233-244`), so a page change remounts the
// scroll container at 0. This build has one scroller for all five pages and
// used to keep its offset: scrolling General to the bottom and then opening
// Shortcuts landed on a page whose own heading was already above the viewport.
func TestANewPageStartsAtTheTop(t *testing.T) {
	a, tt := settingsScrolledBody(t)

	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatalf("the sidebar's Shortcuts row is not clickable: %v", err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if a.Page != PageShortcuts {
		t.Fatalf("the click landed on page %v, want Shortcuts", a.Page)
	}
	if a.Body.Y != 0 {
		t.Errorf("the new page kept the old offset: Y = %v, want 0", a.Body.Y)
	}

	// The resting position of the page's first row is a fact of the page, not
	// a number this test may write down: a fresh surface that never scrolled
	// draws it at the same y (`Find` reads the laid-out rectangle).
	row, ok := tt.Find("Show / hide floter")
	if !ok {
		t.Fatalf("the Shortcuts page has no first row: %q", tt.Texts())
	}
	fresh := New(newStore(t), Actions{})
	ft, _ := renderIn(t, fresh, 720, 420, false)
	fresh.selectPage(PageShortcuts)
	ft.Advance(200 * time.Millisecond)
	ft.Frame()
	want, ok := ft.Find("Show / hide floter")
	if !ok {
		t.Fatalf("the fresh Shortcuts page has no first row: %q", ft.Texts())
	}
	if row.Y != want.Y {
		t.Errorf("the row sits at y=%v after the page change, a fresh page draws it at y=%v",
			row.Y, want.Y)
	}
}

// TestClickingTheCurrentPageKeepsItsScroll: a page that is already on screen
// has the same key in the reference, so the scroller is not remounted and the
// reading position stays. Re-clicking the sidebar row of the page one is
// reading must not jump to the top.
func TestClickingTheCurrentPageKeepsItsScroll(t *testing.T) {
	a, _ := settingsScrolledBody(t)
	scrolled := a.Body.Y

	a.selectPage(a.Page)

	if a.Body.Y != scrolled {
		t.Errorf("reopening the current page moved the scroll: Y = %v, was %v", a.Body.Y, scrolled)
	}
}
