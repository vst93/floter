package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// The reference's group heading is `.settings-section__heading`
// (`settings.css:421-438`): a row inset 2u from the page's edge, its two ends
// 12u apart, bottom-aligned (`align-items: flex-end`), the action resting on
// the title block's bottom edge. The old port drew the title flush with the
// card and centred the action on the block. The action here is a short box, so
// the title block is the taller child and the two alignments differ — with a
// single-line title and the real 24u action they would coincide.
func TestTheSectionHeadingIsTheReferenceRow(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	store := newStore(t)
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})
	title := "Appearance"

	var rowRef, actionRef ui.Handle
	var rowBox, actionBox ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.sectionHeading(c, title, func() {
			// A short control, so the 24px title block is the taller child and
			// the row's flex-end alignment is observable.
			ui.Box(c).Width(60).Height(10).Bind(&actionRef)
		}).Bind(&rowRef)
		rowBox = c.Resolve(rowRef).Bounds()
		actionBox = c.Resolve(actionRef).Bounds()
	}, 640, 200)
	tt.Frame()

	titleBox, ok := tt.Find(title)
	if !ok {
		t.Fatalf("the group title is missing: %q", tt.Texts())
	}

	// `.settings-section__heading { padding: 0 calc(var(--u) * 2) }`.
	if got := titleBox.X - rowBox.X; absFloat(got-tokens.Theme.Space(0.5)) > 1 {
		t.Errorf("heading inset = %v, want the reference's 2u (%v)", got, tokens.Theme.Space(0.5))
	}
	// `align-items: flex-end`: the action rests on the block's bottom edge; a
	// centred action would float half the slack above it.
	if slack := rowBox.H - actionBox.H; slack < 8 {
		t.Fatalf("the action is not shorter than the block: block %v, action %v", rowBox.H, actionBox.H)
	}
	if got := actionBox.Y + actionBox.H; absFloat(got-(rowBox.Y+rowBox.H)) > 1 {
		t.Errorf("action bottom = %v, want the heading's bottom %v (flex-end)", got, rowBox.Y+rowBox.H)
	}
	// `justify-content: space-between`: the action is the row's right end.
	if actionBox.X < titleBox.X+titleBox.W {
		t.Errorf("action X = %v, want it right of the title's end %v", actionBox.X, titleBox.X+titleBox.W)
	}
}

// The group's one-line explanation is a `.settings-section__hint` line at the
// *end* of the section, after the cards it explains (`settings.css:1198-1206`;
// `ShortcutsPage.tsx:424`, `GeneralPage.tsx:597-598`). The old port drew it
// inside the heading, a second line under the title, which is why the heading
// carried two text lines instead of the reference's one. Building a section
// with a box for its card reads where the hint lands.
func TestTheGroupHintRidesAfterItsCards(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})
	title, hint := "Appearance", "Auto follows your system appearance."

	var cardRef ui.Handle
	var cardBox ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.section(c, title, hint, func() {
			ui.Box(c).FillWidth().Height(60).Bind(&cardRef)
		})
		cardBox = c.Resolve(cardRef).Bounds()
	}, 640, 200)
	tt.Frame()

	hintBox, ok := tt.Find(hint)
	if !ok {
		t.Fatalf("the group hint is missing: %q", tt.Texts())
	}
	if hintBox.Y < cardBox.Y+cardBox.H {
		t.Errorf("the group hint (Y=%v) is not after the card's bottom (%v)", hintBox.Y, cardBox.Y+cardBox.H)
	}
	// `.settings-section__hint { padding: 0 calc(var(--u) * 2) }`: the same 2u
	// inset the heading's title keeps.
	if got := hintBox.X - cardBox.X; absFloat(got-tokens.Theme.Space(0.5)) > 1 {
		t.Errorf("hint inset = %v, want the reference's 2u (%v)", got, tokens.Theme.Space(0.5))
	}
}

// On the live Shortcuts page the hint is not a second line of the heading: it
// sits well below the title, after the card of shortcut rows. The old port put
// it one 8u gap under the title.
func TestTheLiveShortcutsHintIsNotInTheHeading(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	title, ok := tt.Find(en.ShortcutsToggle)
	if !ok {
		t.Fatalf("the Shortcuts group title is missing: %q", tt.Texts())
	}
	hint, ok := tt.Find(en.ShortcutsHint)
	if !ok {
		t.Fatalf("the Shortcuts explanation is missing: %q", tt.Texts())
	}
	if gap := hint.Y - (title.Y + title.H); gap < 20 {
		t.Errorf("live title-to-hint gap = %v, want the hint after the cards, not under the title", gap)
	}
}

// The reference's one heading that carries its own explanation is the
// deep-link group (`DeepLinkRow.tsx:61-64`): `heading-main` is a 1px stack of
// the label and `settings-section__hint--inline` (`settings.css:1209-1211`),
// so the hint rides *above* the card, in the title's own column. The old port
// drew it as a group hint after the card, like every other section.
func TestTheDeepLinkHintRidesInsideItsHeading(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{CopyText: func(string) {}})
	tt := render(t, a, 720, 1000)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	title, ok := tt.Find(en.DeepLinkTitle)
	if !ok {
		t.Fatalf("the deep-link title is missing: %q", tt.Texts())
	}
	hint, ok := tt.Find(en.DeepLinkHint)
	if !ok {
		t.Fatalf("the deep-link hint is missing: %q", tt.Texts())
	}
	value, ok := tt.Find(deepLinkConnectExample)
	if !ok {
		t.Fatalf("the deep-link card is missing: %q", tt.Texts())
	}
	// `--inline` stays in `heading-main`, so the hint is *before* the card
	// rather than the `sectionHint` line every other group keeps after it.
	if hint.Y >= value.Y {
		t.Errorf("the deep-link hint (Y=%v) is not above the card (Y=%v)", hint.Y, value.Y)
	}
	// The heading-main stack shares the title's own column.
	if absFloat(hint.X-title.X) > 1 {
		t.Errorf("the inline hint X = %v, want the title's column %v", hint.X, title.X)
	}
	// `heading-main { gap: 1px }`: the hint rides one pixel under the label.
	if got := hint.Y - (title.Y + title.H); absFloat(got-1) > 1 {
		t.Errorf("title-to-inline-hint gap = %v, want heading-main's 1px", got)
	}
}
