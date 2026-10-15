package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The disable confirmation is the reference's notice band, not a stacked card:
// `DisableConfirmation.tsx` renders `{disableTitle} {disableDescription}` in a
// single span, with the alert mark first and the two answers on the same row.
// The confirm button carries the reference's own word —
// `settings.extensions.disable` = "Disable integration" — not a bare "Disable".
func TestDisableBarIsOneNoticeSentence(t *testing.T) {
	toggled := []bool{}
	a := New(newStore(t), Actions{
		SetIntegrationEnabled: func(id string, enabled bool) { toggled = append(toggled, enabled) },
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the enable switch is missing: %v", err)
	}
	tt.Frame()

	// One node holds the title and the consequence together; the old card
	// drew them as two separate texts ("Disable V Tools?" over the sentence).
	const sentence = "Disable V Tools? Floter will stop any running commands immediately. This only marks the integration as disabled \u2014 its files and data are kept, and you can re-enable it at any time."
	if !tt.HasText(sentence) {
		t.Fatalf("the band is not one sentence: %q", tt.Texts())
	}
	// The confirm word is the reference's, verbatim.
	if _, ok := tt.Find("Disable integration"); !ok {
		t.Fatalf("the confirm button says the wrong word: %q", tt.Texts())
	}

	// The two answers sit beside the sentence on one row — the horizontal band,
	// not the old column that put them below it.
	text, okText := tt.Find(sentence)
	cancel, okCancel := tt.Find("Cancel")
	confirm, okConfirm := tt.Find("Disable integration")
	if !okText || !okCancel || !okConfirm {
		t.Fatalf("the band's parts are not all findable: %q", tt.Texts())
	}
	if cancel.X <= text.X {
		t.Errorf("the answers are not beside the sentence: text x=%v cancel x=%v", text.X, cancel.X)
	}
	if d := cancel.Y - confirm.Y; d < -1 || d > 1 {
		t.Errorf("the answers are not on one row: cancel y=%v confirm y=%v", cancel.Y, confirm.Y)
	}

	if err := tt.Click("Disable integration"); err != nil {
		t.Fatalf("the confirm button: %v", err)
	}
	tt.Frame()
	if len(toggled) != 1 || toggled[0] {
		t.Errorf("confirm did not disable: %v", toggled)
	}
}

// The removal bar is the *same* band as the disable bar: `RemovalConfirmation.tsx:38-44`
// is `DisableConfirmation`'s own `.extensions-notice .extension-discard-bar`
// (alert mark, one sentence, answers on the same row) with the error tint and a
// danger confirm. The old build drew it as a stacked card — a bold title over a
// muted body with the buttons below — and drew the reference's `Description`
// text, which its bar never showed (`:41` renders the Title only).
func TestRemovalBarIsTheSameNoticeBand(t *testing.T) {
	en := i18n.For("en").Settings
	removed := []string{}
	a := New(newStore(t), Actions{
		UninstallIntegration: func(id, name, kind string) { removed = append(removed, id) },
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "pkg", Name: "V Tools", DistributionSource: "local"}}
	}
	tt := render(t, a, 720, 1200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(en.IntegrationsRemovePackage); err != nil {
		t.Fatalf("the row's removal button is missing: %v", err)
	}
	tt.Frame()
	if len(removed) != 0 {
		t.Fatalf("the removal happened without asking: %v", removed)
	}

	// The whole question is one node, and it is the kind's title alone: the
	// reference's `removePackageDescription` belongs to no reader it had.
	const sentence = "Remove V Tools?"
	if !tt.HasText(sentence) {
		t.Fatalf("the band's sentence is missing: %q", tt.Texts())
	}
	if tt.HasText(en.IntegrationsRemovePackageDetail) {
		t.Errorf("the band drew a body the reference never showed: %q", tt.Texts())
	}

	// The answers sit beside the sentence on one row, Cancel first and the
	// danger button to its right — the same row the disable band uses.
	text, okText := tt.Find(sentence)
	cancel, okCancel := tt.Find(en.IntegrationsCancel)
	if !okText || !okCancel {
		t.Fatalf("the band's parts are not all findable: %q", tt.Texts())
	}
	if cancel.X <= text.X {
		t.Errorf("the answers are not beside the sentence: text x=%v cancel x=%v", text.X, cancel.X)
	}
	tt.ClickAt(cancel.X+cancel.W+30, cancel.Y+cancel.H/2)
	tt.Frame()
	if len(removed) != 1 || removed[0] != "pkg" {
		t.Errorf("the band's danger button did not confirm: %v", removed)
	}
	if tt.HasText(sentence) {
		t.Errorf("the band stayed open after confirming: %q", tt.Texts())
	}
}

// bandInkWarmth is the mean red-minus-blue over a text box. The reference's
// notice band paints its ink with `currentColor`, so the error variant
// (`--text-warning`, an amber) reads warmer than the neutral disable band on
// the very same pane.
func bandInkWarmth(tt *ui.Tester, box ui.Rect) float64 {
	img := tt.Image()
	var sum float64
	var n int
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		if y < 0 || y >= img.Rect.Dy() {
			continue
		}
		for x := int(box.X); x < int(box.X+box.W); x++ {
			if x < 0 || x >= img.Rect.Dx() {
				continue
			}
			i := y*img.Stride + x*4
			sum += float64(int(img.Pix[i]) - int(img.Pix[i+2]))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// The removal band wears `extensions-notice--error`: the same pane and the same
// one-sentence shape, but the ink (and the keyline, and the danger button's
// label) is `--text-warning` — the reference's `.extensions-notice--error {
// color: var(--text-warning) }`. The old card painted a neutral title over a
// muted body, so nothing on it said the action was a loss.
func TestTheRemovalBandWearsTheErrorTint(t *testing.T) {
	en := i18n.For("en").Settings

	disable := New(newStore(t), Actions{SetIntegrationEnabled: func(string, bool) {}})
	disable.Integrations = func() []Integration {
		return []Integration{{ID: "v", Name: "V Tools", Enabled: true, Running: true}}
	}
	dt := render(t, disable, 720, 1200)
	if err := dt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	dt.Frame()
	if err := dt.Click("Enabled"); err != nil {
		t.Fatalf("the enable switch is missing: %v", err)
	}
	dt.Frame()
	neutralSentence := en.DisableTitle("V Tools") + " " + en.DisableDescription
	neutralBox, ok := dt.Find(neutralSentence)
	if !ok {
		t.Fatalf("the disable band's sentence is missing: %q", dt.Texts())
	}

	remove := New(newStore(t), Actions{UninstallIntegration: func(string, string, string) {}})
	remove.Integrations = func() []Integration {
		return []Integration{{ID: "pkg", Name: "V Tools", DistributionSource: "local"}}
	}
	rt := render(t, remove, 720, 1200)
	if err := rt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	rt.Frame()
	if err := rt.Click(en.IntegrationsRemovePackage); err != nil {
		t.Fatalf("the row's removal button is missing: %v", err)
	}
	rt.Frame()
	dangerSentence := "Remove V Tools?"
	dangerBox, ok := rt.Find(dangerSentence)
	if !ok {
		t.Fatalf("the removal band's sentence is missing: %q", rt.Texts())
	}

	neutral := bandInkWarmth(dt, neutralBox)
	danger := bandInkWarmth(rt, dangerBox)
	t.Logf("band ink warmth: neutral=%.2f danger=%.2f", neutral, danger)
	if danger <= neutral+8 {
		t.Errorf("the removal band is not warning-tinted: neutral=%.2f danger=%.2f", neutral, danger)
	}

	// One word for one answer: both bars draw their Cancel from the same
	// string, as the reference's single `settings.extensions.cancel` did.
	if _, ok := dt.Find(en.IntegrationsCancel); !ok {
		t.Errorf("the disable band's Cancel is missing: %q", dt.Texts())
	}
	if _, ok := rt.Find(en.IntegrationsCancel); !ok {
		t.Errorf("the removal band's Cancel is missing: %q", rt.Texts())
	}
}

// TestBarButtonsUseCaptionStep 验证确认条按钮使用 caption 字号（~14px 行高），而非面板按钮的 body 字号（~17px）。
func TestBarButtonsUseCaptionStep(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{SetIntegrationEnabled: func(string, bool) {}})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "v", Name: "V Tools", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 1200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the enable switch is missing: %v", err)
	}
	tt.Frame()
	textBox, ok := tt.Find(en.DisableConfirm)
	if !ok {
		t.Fatalf("the bar's confirm is missing: %q", tt.Texts())
	}
	if textBox.H > 16 {
		t.Errorf("the bar's button text is not the caption step: %v", textBox.H)
	}
}

// TestRemovalConfirmWearsWarningNotDanger 验证确认条 confirm 按钮的行为（点击后确认 removal）。
func TestRemovalConfirmWearsWarningNotDanger(t *testing.T) {
	en := i18n.For("en").Settings
	removed := []string{}
	a := New(newStore(t), Actions{
		UninstallIntegration: func(id, name, kind string) { removed = append(removed, id) },
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "pkg", Name: "V Tools", DistributionSource: "local"}}
	}
	tt := render(t, a, 720, 1200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(en.IntegrationsRemovePackage); err != nil {
		t.Fatalf("the row's removal button is missing: %v", err)
	}
	tt.Frame()
	if _, ok := tt.Find("Remove V Tools?"); !ok {
		t.Fatalf("the removal band's sentence is missing: %q", tt.Texts())
	}
	cancel, ok := tt.Find(en.IntegrationsCancel)
	if !ok {
		t.Fatalf("the cancel button is missing: %q", tt.Texts())
	}
	tt.ClickAt(cancel.X+cancel.W+20, cancel.Y+cancel.H/2)
	tt.Frame()
	if len(removed) != 1 || removed[0] != "pkg" {
		t.Errorf("the confirm button did not confirm: %v", removed)
	}
	if tt.HasText("Remove V Tools?") {
		t.Errorf("the band stayed open after confirming: %q", tt.Texts())
	}
}
