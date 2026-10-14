package settingsui

import (
	"testing"

	"floter/internal/i18n"
)

// The `floter://register` link marks the card it named and leaves every other
// card alone: the reference's `extension-row__register-dot` was drawn only for
// `highlighted` rows (`src/extensions/ExtensionRow.tsx:196-205`). A pure flag
// can be set while the card still prints nothing, so this is the pixel-level
// acceptance.
func TestOnlyTheRegisterTargetWearsTheLinkMark(t *testing.T) {
	en := i18n.For("en").Settings
	a := newTestApp()
	a.Actions = Actions{
		UninstallComponents: func(string, string, string, UninstallComponents) {},
	}
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "named", Name: "Named Tool", Highlighted: true},
			{ID: "other", Name: "Other Tool"},
		}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	if !tt.HasText(en.RegisterHighlight) {
		t.Fatalf("the named card is missing %q in %v", en.RegisterHighlight, tt.Texts())
	}
	if got := countText(tt.Texts(), en.RegisterHighlight); got != 1 {
		t.Errorf("the link mark appears %d times, want exactly the one named card", got)
	}
}

// countText is how many drawn labels are exactly want.
func countText(texts []string, want string) int {
	n := 0
	for _, text := range texts {
		if text == want {
			n++
		}
	}
	return n
}
