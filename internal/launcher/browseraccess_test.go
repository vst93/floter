package launcher

import "testing"

func TestBrowserAccessFailureHasItsOwnEmptyState(t *testing.T) {
	a := testApp()
	a.Actions.SearchBrowser = func(_ string, done func(BrowserResults)) { done(BrowserResults{Found: true, ReadFailed: true}) }
	a.EnterBrowser()
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText(a.copy().BrowserReadFailed) || tt.HasText(a.copy().BrowserNoProfile) || tt.HasText(a.copy().BrowserEmpty) {
		t.Fatalf("access failure disguised as missing browser or no matches: %v", tt.Texts())
	}
}
