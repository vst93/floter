package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The reference's deep-link copy is a success notification too: its
// `DeepLinkRow` calls `onCopied(t("settings.deepLinkCopied"))`, which lands on
// the app's own toast stack (`DeepLinkRow.tsx:31-36`). The old build's Go port
// kept only the button's 1.6s label, so a copy made no sound anywhere else.
func TestDeepLinkCopyRaisesTheSuccessToast(t *testing.T) {
	en := i18n.For("en").Settings
	var copied, notified []string
	a := New(newStore(t), Actions{
		CopyText:      func(text string) { copied = append(copied, text) },
		NotifySuccess: func(text string) { notified = append(notified, text) },
	})
	tt := render(t, a, 720, 900)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(en.DeepLinkCopy); err != nil {
		t.Fatalf("the deep-link copy control is missing: %v", err)
	}
	tt.Frame()

	if len(copied) != 1 || copied[0] != deepLinkConnectExample {
		t.Fatalf("copied = %v, want the connect example", copied)
	}
	if len(notified) != 1 || notified[0] != en.DeepLinkCopied {
		t.Fatalf("notified = %v, want the reference's confirmation", notified)
	}
}

// The control swaps the reference's two marks: lucide's Copy at rest and Check
// while the 1.6s confirmation holds (`DeepLinkRow.tsx:47-58`). Both names have
// to be in the app's own closed glyph vocabulary, or the mark would silently
// fall back to the alert circle.
func TestDeepLinkCopyUsesTheReferenceGlyphs(t *testing.T) {
	if got := linkCopyGlyph(false); got != "copy" {
		t.Errorf("linkCopyGlyph(false) = %q, want copy", got)
	}
	if got := linkCopyGlyph(true); got != "check" {
		t.Errorf("linkCopyGlyph(true) = %q, want check", got)
	}
	for _, name := range []string{"copy", "check"} {
		if _, ok := launcher.Glyph(name); !ok {
			t.Errorf("the %q mark is not in the glyph set", name)
		}
	}
}

// The reference draws a deep-link value as a code chip, not as an ordinary
// row label: `.settings-deep-link__value` is the pane's own recess showing
// through the card — a sunken fill with a hairline edge and 4u/8u padding
// (`DeepLinkRow.tsx:67,79`; `settings.css:1524-1535`). The old port handed the
// value to `a.row`, which painted it as plain strong text on the card.
func TestTheDeepLinkValueIsTheReferenceCodeChip(t *testing.T) {
	store := newStore(t)
	// The recess is a few levels of a dark palette; in the light theme the
	// sunken pane and the card composite to the same pixel, so the chip would
	// be indistinguishable no matter how it was drawn.
	if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{CopyText: func(string) {}})
	tt := render(t, a, 720, 900)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	value, ok := tt.Find(deepLinkConnectExample)
	if !ok {
		t.Fatalf("the deep-link value is missing: %q", tt.Texts())
	}
	tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})
	pad := tokens.Theme.Space(1) // 4u, the chip's own vertical padding
	midY := value.Y + value.H/2
	// The column 4u left of the glyphs is inside the chip's horizontal
	// padding; three pads out is the card, clear of the chip's 8u edge.
	chip := sample(tt, value.X-pad, midY)
	card := sample(tt, value.X-3*pad, midY)
	if delta := channelDelta(chip, card); delta < 12 {
		t.Fatalf("the value sits on the card (%v vs %v, delta %d); it is not a recessed chip",
			chip, card, delta)
	}
	top, bottom := fillRun(tt, value.X-pad, value.Y-pad*3, value.Y+value.H+pad*3, chip)
	if top < 0 {
		t.Fatalf("no chip fill in the column left of the glyphs")
	}
	if want := value.Y - pad; absFloat(top-want) > 1.5 {
		t.Errorf("chip top = %v, want the text top less 4u (%v)", top, want)
	}
	if want := value.Y + value.H + pad; absFloat(bottom-want) > 1.5 {
		t.Errorf("chip bottom = %v, want the text bottom plus 4u (%v)", bottom, want)
	}
}

// The chip's value is set in the reference's monospace stack
// (`settings.css:1530`). The headless renderer shapes every element in one
// font, so the family cannot be read back from pixels; this pins the stack the
// row hands to `ui.Text` instead, next to the pixel test above that proves the
// row is the chip.
func TestTheDeepLinkValueUsesTheReferenceCodeFont(t *testing.T) {
	if deepLinkValueFont != "SF Mono, Menlo, Monaco, Consolas, monospace" {
		t.Errorf("deepLinkValueFont = %q, want the reference's stack", deepLinkValueFont)
	}
}

// channelDelta is the largest per-channel distance between two pixels.
func channelDelta(a, b ui.Color) int {
	d := 0
	for _, pair := range [][2]byte{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		diff := int(pair[0]) - int(pair[1])
		if diff < 0 {
			diff = -diff
		}
		if diff > d {
			d = diff
		}
	}
	return d
}

// fillRun returns the top and bottom of the contiguous run of pixels within a
// small tolerance of want in the column at x, between y0 and y1 (−1 if none).
func fillRun(tt *ui.Tester, x, y0, y1 float32, want ui.Color) (float32, float32) {
	top, bottom := float32(-1), float32(-1)
	for y := int(y0); y <= int(y1); y++ {
		if channelDelta(sample(tt, x, float32(y)), want) <= 4 {
			if top < 0 {
				top = float32(y)
			}
			bottom = float32(y)
		}
	}
	return top, bottom
}

func absFloat(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// The reference's copy action is a `.settings-reset` text action with the
// 13px Copy mark on its leading edge (`DeepLinkRow.tsx:47-58`,
// `SettingsRows.tsx:200-216`). The framework's own `ui.Button` has no icon,
// so the old port drew a plain pill; the pixels in the 20u band before the
// label are then fill, not ink. The accent mark is what makes that band
// visibly darker than the card.
func TestTheDeepLinkCopyActionWearsItsIcon(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{CopyText: func(string) {}})
	tt := render(t, a, 720, 900)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	label, ok := tt.Find(en.DeepLinkCopy)
	if !ok {
		t.Fatalf("the deep-link copy control is missing: %q", tt.Texts())
	}
	img := tt.Image()
	bg := sample(tt, label.X-200, label.Y+label.H/2)
	maxDelta := 0
	for y := int(label.Y); y < int(label.Y+label.H); y++ {
		for x := int(label.X - 22); x < int(label.X-2); x++ {
			i := y*img.Stride + x*4
			for _, d := range []int{
				int(img.Pix[i]) - int(bg.R),
				int(img.Pix[i+1]) - int(bg.G),
				int(img.Pix[i+2]) - int(bg.B),
			} {
				if d < 0 {
					d = -d
				}
				if d > maxDelta {
					maxDelta = d
				}
			}
		}
	}
	if maxDelta < 60 {
		t.Fatalf("the 20u band before the label is fill (max channel delta %d), not the copy mark", maxDelta)
	}
}
