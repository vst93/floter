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

// TestAppearanceAxesSplit: the schema is the one place that decides which
// surface paints which terminal-appearance row (R42/R55). The settings card is
// a strict subset of the drawer in the schema's own order; the five axes the
// drawer keeps to itself are the padding, the cursor shape, the blink, the
// copy-on-select switch and the safe-paste switch.
func TestAppearanceAxesSplit(t *testing.T) {
	all := AppearanceAxes()
	seen := make(map[AppearanceAxis]bool, len(all))
	for _, axis := range all {
		if axis == "" {
			t.Error("the schema names an empty axis")
		}
		if seen[axis] {
			t.Errorf("the schema names %q twice", axis)
		}
		seen[axis] = true
	}

	card := SettingsCardAxes()
	// The card order is the schema's: the card list is a filter, not a second
	// ordering.
	filtered := make([]AppearanceAxis, 0, len(card))
	for _, axis := range all {
		if OnSettingsCard(axis) {
			filtered = append(filtered, axis)
		}
	}
	if len(card) != len(filtered) {
		t.Fatalf("the card axes %v are not the schema filtered by OnSettingsCard %v", card, filtered)
	}
	for i := range card {
		if card[i] != filtered[i] {
			t.Errorf("card axis %d = %q, want %q", i, card[i], filtered[i])
		}
	}
	if len(card) >= len(all) {
		t.Errorf("the card keeps %d of %d axes; the drawer must keep more", len(card), len(all))
	}

	// The drawer-only set is exactly the five the reference hides on the card.
	drawerOnly := map[AppearanceAxis]bool{}
	for _, axis := range all {
		if !OnSettingsCard(axis) {
			drawerOnly[axis] = true
		}
	}
	want := map[AppearanceAxis]bool{
		AxisPadding: true, AxisCursorShape: true, AxisCursorBlink: true,
		AxisSelectCopy: true, AxisPasteSafe: true,
	}
	if len(drawerOnly) != len(want) {
		t.Fatalf("the drawer-only axes are %v, want %v", drawerOnly, want)
	}
	for axis := range want {
		if !drawerOnly[axis] {
			t.Errorf("%q is no longer drawer-only", axis)
		}
	}
}
