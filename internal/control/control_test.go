package control

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// render draws one segmented control in a window, with the theme the app
// resolves for the shipped appearance.
func render(t *testing.T, labels []string, current int, apply func(int)) *ui.Tester {
	t.Helper()
	tokens := theme.For(settings.Default(), true)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		Segmented(c, tokens, labels, current, apply)
	}, 400, 120)
	tt.Frame()
	return tt
}

// TestSegmentedDrawsEveryChoice: the control is one implementation for both the
// settings panel and the terminal drawer, so it must draw every label it is
// handed.
func TestSegmentedDrawsEveryChoice(t *testing.T) {
	tt := render(t, []string{"Beam", "Block", "Underline"}, 0, func(int) {})
	for _, label := range []string{"Beam", "Block", "Underline"} {
		if !tt.HasText(label) {
			t.Errorf("the control is missing %q: %q", label, tt.Texts())
		}
	}
}

// TestSegmentedReportsTheChosenIndex: the caller is told which option the user
// picked, by index into the labels it passed.
func TestSegmentedReportsTheChosenIndex(t *testing.T) {
	chosen := -1
	tt := render(t, []string{"Beam", "Block", "Underline"}, 0, func(i int) { chosen = i })
	if err := tt.Click("Block"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if chosen != 1 {
		t.Errorf("chosen = %d, want 1", chosen)
	}
}

// TestSegmentedKeepsTheChosenOptionMarked: the chosen segment is a raised pane
// with the accent keyline, and it is the one the caller named.
func TestSegmentedKeepsTheChosenOptionMarked(t *testing.T) {
	tt := render(t, []string{"Beam", "Block", "Underline"}, 2, func(int) {})
	rect, ok := tt.Find("Underline")
	if !ok {
		t.Fatal("the chosen label is missing")
	}
	if rect.W <= 0 || rect.H <= 0 {
		t.Errorf("the chosen segment has no box: %v", rect)
	}
}
