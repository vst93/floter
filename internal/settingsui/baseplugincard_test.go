package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The Installed tab's built-in plugin cards were the last family on the
// Integrations page never compared against the reference field by field.
// `main:src/ExtensionsPanel.tsx:2388-2448` draws one `.extensions-base-plugin`
// per plugin — *not* a settings row — and `main:src/styles/extensions.css`
// names every piece:
//
//	.extensions-base-plugin        min-height 66px, `align-items: flex-start`,
//	                               `gap: 16px`, `padding: 12px 14px`, the
//	                               `--ext-border` hairline on the
//	                               `--glass-control` plane, `--radius-md`
//	.extensions-base-plugin__main  `min-width: 0`, a flex column, `gap: 3px`
//	.extensions-base-plugin__name  `--text-title` (13px), line-height 1.5,
//	                               `--text-strong` — and **no weight** of its
//	                               own
//	.extensions-base-plugin__description  `--text-emphasis` (12px), 1.5,
//	                               `--text-secondary`
//	.extensions-base-plugin__actions  `flex: 0 0 auto`, a flex row, `gap: 10px`
//	.extensions-base-plugin__configure  the action pill at **24px** and the
//	                               emphasis step
//	.extensions-base-plugin__privacy  `--text-body` + `.settings-privacy-hint`'s
//	                               `--text-warning`, `padding: 2px 2px 0`
//
// The port had folded each plugin into one settings row — the name at the
// emphasis step, the hint one register too quiet and single-line, the privacy
// note weight-500 with no padding, and the three plugins separated by the
// card's ruled rows instead of standing alone.

// basePluginApp renders with the built-in plugins and both of their controls
// wired, which is the only state the section draws in.
func basePluginApp(t *testing.T) *App {
	t.Helper()
	a := New(newStore(t), Actions{
		OpenPluginConfig: func(string) {},
		SetPluginEnabled: func(string, bool) {},
	})
	a.Integrations = func() []Integration { return nil }
	return a
}

// cardBox is a card's painted vertical extent around one of its labels: the
// text rows above and below a card are separated by the page's own plane, so a
// card of its own has a box around its label while a row of one shared card
// does not (the neighbouring rows' planes are the same fill). It walks out
// from the label line at a clear column until the page's plane comes back, so
// a test reads geometry without depending on the widget tree.
func cardBox(tt *ui.Tester, label string) (top, bottom int) {
	r, ok := tt.Find(label)
	if !ok {
		return 0, 0
	}
	img := tt.Image()
	y := int(r.Y + r.H/2)
	fill := imagePixel(img, 2, y)
	x := int(r.X + 200) // a card column, clear of the label and its controls
	if x >= img.Rect.Dx() {
		x = img.Rect.Dx() - 4
	}
	top, bottom = -1, -1
	for yy := y; yy >= 0; yy-- {
		if channelDelta(imagePixel(img, x, yy), fill) > 4 {
			top = yy
		} else if top >= 0 {
			break
		}
	}
	for yy := y; yy < img.Rect.Dy(); yy++ {
		if channelDelta(imagePixel(img, x, yy), fill) > 4 {
			bottom = yy
		} else if bottom >= 0 {
			break
		}
	}
	return top, bottom
}

// The three built-in plugins stand as their own cards, not as rows of one
// shared card: each is a 66px box with the reference's own plane, its own
// top-aligned copy column and its own trailing control group. The card height
// is the reference's `min-height: 66px` at the interface step the panel draws
// in (its literal-px unit), which is what keeps the short calculator card as
// tall as the clipboard card that carries three text lines.
func TestTheBasePluginCardsAreTheirOwnBoxes(t *testing.T) {
	en := i18n.For("en").Settings
	a := basePluginApp(t)
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// `min-height: 66px` is a `border-box` floor (base.css's `*` reset), so the
	// *painted* box — borders included — is the reference's literal 66 at every
	// interface step (extension panel CSS is written in literal px).
	const wantFloor = 66

	calcTop, calcBottom := cardBox(tt, en.BaseCalculator)
	calc := float32(calcBottom - calcTop + 1)
	if calc < wantFloor-1 || calc > wantFloor+1 {
		t.Errorf("the calculator card's box is %.0f tall, want the reference's 66px floor", calc)
	}
	clipTop, clipBottom := cardBox(tt, en.BaseClipboard)
	clip := float32(clipBottom - clipTop + 1)
	if calcTop < 0 || clipTop < 0 {
		t.Fatalf("a card's box was not found: calculator %d..%d, clipboard %d..%d",
			calcTop, calcBottom, clipTop, clipBottom)
	}
	if clip <= calc {
		t.Errorf("the clipboard card (%.0f) is not taller than the calculator's (%.0f): its extra text lines do not grow it",
			clip, calc)
	}
	// The two cards are separate boxes: there is page plane between them, not a
	// separator drawn on one shared card's plane.
	if clipBottom >= calcTop {
		t.Errorf("the clipboard card ends at %d and the calculator starts at %d: the two are not separate boxes",
			clipBottom, calcTop)
	}
}

// The card's text column is the reference's own three registers: the name at
// the title step, the description one step down in the secondary ink (and
// wrapping, not a single ellipsized line), and the privacy note at the body
// step in the warning ink — the one line that discloses rather than describes.
func TestTheBasePluginCardTextRunsTheReferenceLadder(t *testing.T) {
	en := i18n.For("en").Settings
	a := basePluginApp(t)
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	name := mustFind(t, tt, en.BaseClipboard)
	hint := mustFind(t, tt, en.BaseClipboardHint)
	privacy := mustFind(t, tt, en.BaseClipboardPrivacy)
	if name.H <= hint.H {
		t.Errorf("the card's name box (%.0f) is not taller than its description (%.0f): the name is the title step",
			name.H, hint.H)
	}
	if privacy.Y <= hint.Y {
		t.Errorf("the privacy note (Y=%.0f) is not under the description (Y=%.0f)", privacy.Y, hint.Y)
	}
	// The note is where the reference put it: the two labels it describes sit
	// on the same column, and the note keeps the reference's own 2px top pad —
	// so the gap between the two text lines is wider than the name/description
	// gap above them.
	tokens := theme.For(settings.Default(), false)
	noteInk := inkColor(tt, privacy)
	bg := sample(tt, privacy.X+1, privacy.Y+privacy.H/2)
	warm := overSurface(tokens.Theme.Warning, bg)
	cool := overSurface(tokens.TextSecondary, bg)
	if colorDistance(noteInk, warm) > colorDistance(noteInk, cool) {
		t.Errorf("the privacy note's ink %v is nearer the card's secondary ink (%v) than the warning ink (%v)",
			noteInk, cool, warm)
	}
}

// The controls column is the reference's own: the Configure pill at 24px and
// the emphasis step, sitting beside the plugin name's line — never the
// settings row's 30u pill and never a ruled row under the copy.
func TestTheBasePluginConfigureIsTheTwoFourPill(t *testing.T) {
	en := i18n.For("en").Settings
	a := basePluginApp(t)
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	button := mustFind(t, tt, en.PluginConfigure+" Calculator")
	// `.extensions-base-plugin__configure { height: 24px }`: the panel's action
	// pill one step down, at *its own* unit — the extension panel's CSS is
	// written in literal px (`extensionMetrics`), so the pill is 24 whatever the
	// interface step.
	if got := button.H; got < 22 || got > 26 {
		t.Errorf("the Configure pill is %.0f tall, want the reference's literal 24px", got)
	}
	name := mustFind(t, tt, en.BaseCalculator)
	if button.Y > name.Y+name.H+2 {
		t.Errorf("the Configure pill (Y=%.0f) hangs below the plugin name's line (Y=%.0f..%.0f)",
			button.Y, name.Y, name.Y+name.H)
	}
}
