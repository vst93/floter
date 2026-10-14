package settingsui

import "testing"

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
