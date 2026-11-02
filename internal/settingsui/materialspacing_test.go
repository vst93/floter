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
//
// P13-br 重定基线：`gap` 那一半的旧值 → 新值。参考实现给这一个 section 写了两个
// 例外（`gap: 10u` 与 `padding: 10u 0`），量下来前者只值 1.8px（9px 对别组的
// 7.2px），后者值 9px——留大的、删小的。于是这一组的**组级**gutter（组头→卡、
// 卡→脚注、脚注→脚注）回到共享的 8u，而**卡与卡之间**仍是 `.settings-cards`
// 的 10u（那是另一个角色，全页同值）。padding 原封不动，它才是这一组读起来更松的
// 原因（见 `docs/liquid-glass.md` §6.36）。
func TestTheMaterialSectionKeepsItsOwnSpacing(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	spacing := contextSpace(t, 2.5, 2, 4)
	ten, eight, four := spacing[0], spacing[1], spacing[2]

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

	// The group's own gutter — last card → first footnote, footnote → footnote —
	// is the shared 8u every section keeps (P13-br 重定基线：旧值 10u → 新值 8u).
	// The card-to-card gutter is not that role: it is `.settings-cards { gap: 10u }`,
	// which every section's cards column keeps, and it stays 10u.
	//
	// P13-ek 重定基线：**heading → first card** 旧值 8u → 新值 16u（`Space(4)`）。
	// 那一格是这一对**自己**的空气（§4.3），不是整列的：8u（7.2px）比同一组里两张卡
	// 之间的 10u **还紧**，标题读成「又一张卡」。抬一整档之后它是 14.4px，与浮层里
	// 同一格（§6.105）逐值相同。其余三条 gutter 一个字节没动。
	// `docs/liquid-glass.md` §6.106。
	for _, row := range []struct {
		name string
		gap  float32
		want float32
	}{
		{"heading to first card", got.card1.Y - (got.title.Y + got.title.H), four},
		{"first card to second card", got.card2.Y - (got.card1.Y + got.card1.H), ten},
		{"second card to first footnote", got.hintA.Y - (got.card2.Y + got.card2.H), eight},
		{"first footnote to second footnote", got.hintB.Y - (got.hintA.Y + got.hintA.H), eight},
	} {
		if absFloat(row.gap-row.want) > 1 {
			t.Errorf("material %s = %v, want %v", row.name, row.gap, row.want)
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
	if gap := plain.card1.Y - (plain.title.Y + plain.title.H); absFloat(gap-four) > 1 {
		t.Errorf("plain heading to first card = %v, want the heading's own 16u (%v) — P13-ek "+
			"重定基线：旧值 8u → 新值 Space(4)", gap, four)
	}
}

// On the live General page the material group is the last section and its two
// cards and two footnotes are drawn in the reference's order with the shared
// group gutter before the footnotes (P13-br 重定基线：旧值 10u → 新值 8u; the
// 10u this section still keeps is its padding and its cards column). The
// page-gap test (`TestTheGeneralPageKeepsTheReferenceSectionGap`) only sees
// group boxes; this one reads the live group's own internals.
func TestTheLiveMaterialSectionUsesItsOwnSpacing(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	ten := contextSpace(t, 2.5)[0]
	eight := contextSpace(t, 2)[0]

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
	if gap := hintB.Y - (hintA.Y + hintA.H); absFloat(gap-eight) > 1 {
		t.Errorf("live footnote gutter = %v, want the shared 8u group gutter (%v)", gap, eight)
	}
}
