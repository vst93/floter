package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/launcher"
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
