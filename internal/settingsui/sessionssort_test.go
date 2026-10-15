package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/launcher"
)

// The reference's Sessions heading carries the sort control with lucide's
// ArrowUpDown at 13px before the order word (`SessionsPage.tsx:123-125`,
// `<ArrowUpDown size={13} strokeWidth={1.8}/>`). The old port drew the word
// alone, so the control said which way the list *did* run but never which way
// it *could* — the two arrows are what names the axis.
func TestTheSessionsSortControlLeadsWithTheReferenceMark(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	a.Sessions = func() []Session {
		return []Session{{Title: "zsh — floter", Running: true, Active: true, Index: 0, ID: 1}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click(en.PageSessions); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	label, ok := tt.Find(en.SessionsSortNewest)
	if !ok {
		t.Fatalf("the sort control is missing: %q", tt.Texts())
	}
	// The mark lives in the control's own leading slot, in the 5u gutter left
	// of the word (`rows.go`, `actionWithGlyph`). A text-only action leaves
	// that band as plane; the reference's mark leaves accent ink in it.
	midY := label.Y + label.H/2
	mark := 256
	for x := int(label.X) - 19; x < int(label.X)-2; x++ {
		p := sample(tt, float32(x), midY)
		for _, ch := range []byte{p.R, p.G, p.B} {
			if int(ch) < mark {
				mark = int(ch)
			}
		}
	}
	if mark >= 200 {
		t.Errorf("no accent mark left of the sort word (min channel %d); the control is text-only", mark)
	}
	// The mark has to be one the app's closed vocabulary actually speaks, or
	// it would silently fall back to the alert circle (`mustGlyph`).
	if _, ok := launcher.Glyph("arrow-up-down"); !ok {
		t.Errorf("the arrow-up-down mark is not in the glyph set")
	}

	// The control still flips the order, and the word follows it.
	if err := tt.Click(en.SessionsSortNewest); err != nil {
		t.Fatalf("sort: %v", err)
	}
	tt.Frame()
	if !tt.HasText(en.SessionsSortOldest) {
		t.Errorf("the control did not flip to the other order: %q", tt.Texts())
	}
}
