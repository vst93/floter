package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// contextSpace reads a spacing step from the theme a frame actually draws in,
// not a theme built beside the tester: the tester's own preferences round the
// step (Space(2.5) is the reference's 10u there, 9 beside it).
func contextSpace(t *testing.T, steps ...float64) []float32 {
	t.Helper()
	out := make([]float32, len(steps))
	ui.NewTester(func(c *ui.Context) {
		for i, step := range steps {
			out[i] = c.Theme().Space(float32(step))
		}
	}, 100, 100).Frame()
	return out
}

// The General page's material group is the reference's one section spaced
// differently from the rest: `.settings-section--material` is both
// `gap: calc(var(--u) * 10)` and `padding: calc(var(--u) * 10) 0`
// (`settings.css:767-772`), against the 8u gap every other `.settings-section`
// keeps (`:411-419`) and its two cards 10u apart (`.settings-cards`,
// `:491-495`). `sectionMaterial` reproduces it; `section` stays at the shared
// 8u with no padding. This builds both with boxes for their cards and reads
// every gutter and the group's own padding off the resolved boxes.
func TestTheMaterialSectionKeepsItsOwnSpacing(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	spacing := contextSpace(t, 2.5, 2)
	ten, eight := spacing[0], spacing[1]

	type built struct {
		group                ui.Rect
		card1, card2         ui.Rect
		title, hintA, hintB  ui.Rect
		haveTitle, haveHintA bool
		haveHintB            bool
	}

	build := func(material bool, title string, hints []string) built {
		t.Helper()
		var groupRef, c1, c2 ui.Handle
		var out built
		tt := ui.NewTester(func(c *ui.Context) {
			draw := func() {
				ui.Box(c).FillWidth().Height(60).Bind(&c1)
				ui.Box(c).FillWidth().Height(60).Bind(&c2)
			}
			if material {
				a.sectionMaterial(c, title, hints, draw).Bind(&groupRef)
			} else {
				a.section(c, title, "", draw).Bind(&groupRef)
			}
			out.group = c.Resolve(groupRef).Bounds()
			out.card1 = c.Resolve(c1).Bounds()
			out.card2 = c.Resolve(c2).Bounds()
		}, 640, 400)
		tt.Frame()
		out.title, out.haveTitle = tt.Find(title)
		if len(hints) > 0 {
			out.hintA, out.haveHintA = tt.Find(hints[0])
		}
		if len(hints) > 1 {
			out.hintB, out.haveHintB = tt.Find(hints[1])
		}
		return out
	}

	const title = "Material"
	hintA, hintB := "The glass effect grades the whole surface.", "The transparency sliders set how solid the frame is."
	got := build(true, title, []string{hintA, hintB})

	if !got.haveTitle || !got.haveHintA || !got.haveHintB {
		t.Fatalf("the material section did not draw its title and both footnotes: %+v", got)
	}

	// `.settings-section--material { padding: calc(var(--u) * 10) 0 }`: the
	// group's own box opens 10u above the heading and closes 10u below the last
	// footnote. The padding sits inside the labelled group box, so it does not
	// move the 22u page gap between this section and its neighbour.
	if gap := got.title.Y - got.group.Y; absFloat(gap-ten) > 1 {
		t.Errorf("material top padding = %v, want the reference's 10u (%v)", gap, ten)
	}
	if gap := (got.group.Y + got.group.H) - (got.hintB.Y + got.hintB.H); absFloat(gap-ten) > 1 {
		t.Errorf("material bottom padding = %v, want the reference's 10u (%v)", gap, ten)
	}

	// The section's grid gap is 10u: heading → first card, card → card,
	// last card → first footnote, and footnote → footnote are all the same
	// gutter.
	for _, row := range []struct {
		name string
		gap  float32
	}{
		{"heading to first card", got.card1.Y - (got.title.Y + got.title.H)},
		{"first card to second card", got.card2.Y - (got.card1.Y + got.card1.H)},
		{"second card to first footnote", got.hintA.Y - (got.card2.Y + got.card2.H)},
		{"first footnote to second footnote", got.hintB.Y - (got.hintA.Y + got.hintA.H)},
	} {
		if absFloat(row.gap-ten) > 1 {
			t.Errorf("material %s = %v, want the reference's 10u section gap (%v)", row.name, row.gap, ten)
		}
	}

	// A plain section keeps the shared 8u gap and no padding, so the material
	// group's looser spacing cannot leak into every other section.
	plain := build(false, "Appearance", nil)
	if !plain.haveTitle {
		t.Fatalf("the plain section did not draw its title: %+v", plain)
	}
	if gap := plain.title.Y - plain.group.Y; absFloat(gap) > 1 {
		t.Errorf("plain section top padding = %v, want none", gap)
	}
	if gap := plain.card1.Y - (plain.title.Y + plain.title.H); absFloat(gap-eight) > 1 {
		t.Errorf("plain heading to first card = %v, want the shared 8u section gap (%v)", gap, eight)
	}
}

// On the live General page the material group is the last section and its two
// cards and two footnotes are drawn in the reference's order with the looser
// 10u gutter before the footnotes. The page-gap test
// (`TestTheGeneralPageKeepsTheReferenceSectionGap`) only sees group boxes; this
// one reads the live group's own internals.
func TestTheLiveMaterialSectionUsesItsOwnSpacing(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	ten := contextSpace(t, 2.5)[0]

	copy := i18n.For("en").Settings
	tt := render(t, a, 720, 2800)

	group, ok := tt.Find(sectionLabel(copy.Material))
	if !ok {
		t.Fatalf("the material section has no group box: %q", tt.Texts())
	}
	title := mustFind(t, tt, copy.Material)
	glass := mustFind(t, tt, copy.Glass)
	hintA := mustFind(t, tt, copy.GlassHint)
	hintB := mustFind(t, tt, copy.TransparencyHint)

	if gap := title.Y - group.Y; absFloat(gap-ten) > 1 {
		t.Errorf("live material top padding = %v, want the reference's 10u (%v)", gap, ten)
	}
	if gap := (group.Y + group.H) - (hintB.Y + hintB.H); absFloat(gap-ten) > 1 {
		t.Errorf("live material bottom padding = %v, want the reference's 10u (%v)", gap, ten)
	}
	if hintA.Y <= glass.Y+glass.H {
		t.Errorf("the glass footnote (Y=%v) is not after the glass row (Y=%v)", hintA.Y, glass.Y+glass.H)
	}
	if gap := hintB.Y - (hintA.Y + hintA.H); absFloat(gap-ten) > 1 {
		t.Errorf("live footnote gutter = %v, want the reference's 10u (%v)", gap, ten)
	}
}
