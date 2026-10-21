package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// The reference's shared text action is `.settings-reset` itself
// (`SettingsRows.tsx:201-216`; `settings.css:682-730`): a 24u floor, 2u of side
// padding, a 5u gap, a `--radius-xs` corner and a transparent fill. The old
// port padded the row by 4u/6u and rounded it to `--radius-sm` (`rows.go`,
// `actionWithGlyph`), so every text action — "Restore defaults" and both
// deep-link copies — stood 6u wider and 4u taller than the one it copied.
// Rendering the action on its own reads its box without the surrounding page.
func TestTheTextActionsWearTheReferenceResetMetrics(t *testing.T) {
	en := i18n.For("en").Settings
	store := newStore(t)
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})

	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c).Width(120)
			a.action(c, en.ShortcutsReset, func() {}).Bind(&ref)
		})
		box = c.Resolve(ref).Bounds()
	}, 400, 200)
	tt.Frame()
	text, ok := tt.Find(en.ShortcutsReset)
	if !ok {
		t.Fatalf("the action label is missing: %q", tt.Texts())
	}

	// The old port's 4u/6u padding gave a 6u leading inset and a 30u box; the
	// reference's box is 24u with a 2u inset. Both mutations (padding and
	// min-height) are caught by bracketting the box from both sides.
	if want := tokens.Theme.Space(0.5); absFloat(text.X-box.X-want) > 1 {
		t.Errorf("leading inset = %v, want the reference's 2u (%v)", text.X-box.X, want)
	}
	if right := box.X + box.W - (text.X + text.W); absFloat(right-tokens.Theme.Space(0.5)) > 1 {
		t.Errorf("trailing inset = %v, want the reference's 2u (%v)", right, tokens.Theme.Space(0.5))
	}
	if box.H < 23.5 {
		t.Errorf("action height = %v, want at least the reference's 24u floor", box.H)
	}
	if box.H > 25 {
		t.Errorf("action height = %v, want the reference's 24u, not the old 30u box", box.H)
	}
}

// `.settings-reset:not(:disabled):hover { opacity: .78 }` and `:active { opacity:
// .6 }` (`settings.css:713-721`): the interaction moves the *ink*, it never
// paints a band behind it. The old port swapped a `SurfaceHover` background in,
// so the words' edge was the only thing that moved — the plane under them lit
// up. This drives the live Shortcuts group's "Restore defaults" so the action
// is the one the app really draws.
//
// P8-d 重定基线（**V**，旧值 → 新值）：静息 `t.Accent`（≈`{229,72,77}`）→
// `tokens.TextSecondary`，hover `t.Accent` + 0.78 透明度（"变淡"）→
// `t.Text` + 0.78 透明度（墨更重一档）。旧断言问的是「hover 后最小通道变大
// （变淡）吗」——它只对红色墨成立；新断言问的是合约真正说的两件事：**面一格
// 未动**，以及**墨的对比度抬高了**（离面的距离变大，在深浅两套外观里都成
// 立），并额外钉住「抬的是明度不是色相」（两个状态都不得带彩度）。
func TestTheTextActionHoverLiftsTheInkOnly(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 800)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	label, ok := tt.Find(en.ShortcutsReset)
	if !ok {
		t.Fatalf("the reset action is missing: %q", tt.Texts())
	}
	midY := label.Y + label.H/2
	// 1u left of the glyphs is inside the action's own 2u padding, where there
	// is plane and no ink.
	plane := func() ui.Color { return sample(tt, label.X-1, midY) }
	// ink is the pixel of the label band farthest from the plane it sits on:
	// the glyph's own colour. Antialiased glyph edges sit between the two, so
	// "farthest" is the ink and "nearest" would be the plane.
	ink := func() ui.Color {
		bg := plane()
		var best ui.Color
		far := -1
		for x := int(label.X); x < int(label.X+label.W); x++ {
			for y := int(label.Y); y < int(label.Y+label.H); y++ {
				p := sample(tt, float32(x), float32(y))
				if d := colorDistance(p, bg); d > far {
					far, best = d, p
				}
			}
		}
		return best
	}

	restPlane, restInk := plane(), ink()
	// Neutral at rest: the action's words are grey, not a hue. This is the
	// half that fails if the accent comes back (it is ~150 levels from grey).
	if d := int(restInk.R) - int(restInk.B); d > 14 || d < -14 {
		t.Fatalf("the resting label carries a hue (%v, R−B %d): the accent is back", restInk, d)
	}
	if colorDistance(restInk, restPlane) < 60*60*3 {
		t.Fatalf("the resting label %v does not stand off its plane %v: the probe cannot read it",
			restInk, restPlane)
	}
	tt.Move(label.X+label.W/2, midY)
	tt.Frame()
	hoverPlane, hoverInk := plane(), ink()

	if delta := channelDelta(restPlane, hoverPlane); delta > 2 {
		t.Errorf("hover painted the plane: %v → %v (delta %d); the reference only moves the ink",
			restPlane, hoverPlane, delta)
	}
	// The ink gains contrast under the pointer (on light it darkens, on dark it
	// brightens) — the reference's `opacity: .78` read against a heavier rung.
	if colorDistance(hoverInk, hoverPlane) <= colorDistance(restInk, restPlane) {
		t.Errorf("hover did not lift the ink: %v (Δ %d) → %v (Δ %d) against the plane %v",
			restInk, colorDistance(restInk, restPlane),
			hoverInk, colorDistance(hoverInk, hoverPlane), hoverPlane)
	}
	// …and it is still a grey: hover moves the *lightness*, never the hue.
	if d := int(hoverInk.R) - int(hoverInk.B); d > 14 || d < -14 {
		t.Errorf("the hovered label carries a hue (%v, R−B %d); hover must not raise the accent",
			hoverInk, d)
	}
}

// The reference's `.settings-section__heading` is a flex row, flex-end aligned,
// space-between, 12u gap, 2u side padding (`settings.css:421-429`). The action
// sits at the row's right end, bottom-aligned with the title block — not
// centred mid-height. This drives the live Shortcuts group's heading and
// checks the action's box against the reference's geometry.
func TestSectionHeadingActionSitsAtTheRightEndBottomAligned(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 800)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	action, ok := tt.Find(en.ShortcutsReset)
	if !ok {
		t.Fatalf("the reset action is missing: %q", tt.Texts())
	}
	title, ok := tt.Find(en.ShortcutsToggle)
	if !ok {
		t.Fatalf("the section title is missing: %q", tt.Texts())
	}
	// The action's right edge sits at least 60% across the content column —
	// the reference's space-between pushes it to the row's right end, not
	// beside the title.
	if action.X+action.W < 60 {
		t.Errorf("the action is not at the right end: right edge at %v", action.X+action.W)
	}
	// The action is bottom-aligned with the heading block (flex-end), not
	// centred on it: its bottom edge is within 2u of the title block's bottom.
	// The group's explanation is a `sectionHint` line after the cards, so the
	// heading block is the title alone.
	blockBottom := title.Y + title.H
	if diff := action.Y + action.H - blockBottom; diff < -2 || diff > 2 {
		t.Errorf("the action is not bottom-aligned: bottom delta %v", diff)
	}
}
