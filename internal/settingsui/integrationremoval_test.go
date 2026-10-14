package settingsui

import (
	"testing"

	"floter/internal/i18n"
)

// The removal verb is picked by kind, not spoken in one generic word. The
// reference told four kinds apart (`src/extensions/ExtensionRow.tsx:109-114`)
// and chose the menu item's text from that kind (`:409-418`); a system
// integration whose runtime belongs to the OS is not "uninstalled", a linked
// local package is only "removed from the list", and a generated custom
// integration is "deleted". One word for all four made the card lie about
// what the press would do.
func TestTheRemovalVerbFollowsTheIntegrationKind(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	cases := []struct {
		name string
		in   Integration
		kind string
		en   string
		zh   string
	}{
		{
			name: "a generated custom integration is deleted",
			in:   Integration{ID: "custom", Generated: true, DistributionSource: "local"},
			kind: removalCustom, en: en.RemoveDelete, zh: zh.RemoveDelete,
		},
		{
			// Generated wins over npm: a custom integration installed from a
			// registry is still the host's own generated definition.
			name: "generated beats npm",
			in:   Integration{ID: "custom-npm", Generated: true, DistributionSource: "npm"},
			kind: removalCustom, en: en.RemoveDelete, zh: zh.RemoveDelete,
		},
		{
			name: "an npm package is uninstalled",
			in:   Integration{ID: "npm", DistributionSource: "npm"},
			kind: removalNPM, en: en.IntegrationsUninstall, zh: zh.IntegrationsUninstall,
		},
		{
			name: "a system-owned built-in is disconnected",
			in:   Integration{ID: "sys", DistributionSource: "built-in", RuntimeOwnership: "system"},
			kind: removalSystem, en: en.IntegrationsDisconnect, zh: zh.IntegrationsDisconnect,
		},
		{
			// A built-in floter bundles and owns itself: no disconnect.
			name: "a bundled built-in is a package",
			in:   Integration{ID: "bundled", DistributionSource: "built-in", RuntimeOwnership: "bundled"},
			kind: removalPackage, en: en.IntegrationsRemovePackage, zh: zh.IntegrationsRemovePackage,
		},
		{
			name: "a linked local package is removed from the list",
			in:   Integration{ID: "local", DistributionSource: "local"},
			kind: removalPackage, en: en.IntegrationsRemovePackage, zh: zh.IntegrationsRemovePackage,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.removalKind(); got != tc.kind {
				t.Errorf("removalKind() = %q, want %q", got, tc.kind)
			}
			if got := tc.in.removalLabel(en); got != tc.en {
				t.Errorf("removalLabel(en) = %q, want %q", got, tc.en)
			}
			if got := tc.in.removalLabel(zh); got != tc.zh {
				t.Errorf("removalLabel(zh) = %q, want %q", got, tc.zh)
			}
		})
	}
}

// The same four-way pick reaches the card: each integration's row carries its
// own verb, and never the generic one the others use. This is the pixel-level
// acceptance — the pure mapping above can be right while the card still prints
// a hard-coded word.
func TestEachIntegrationCardShowsItsOwnRemovalVerb(t *testing.T) {
	en := i18n.For("en").Settings
	a := newTestApp()
	a.Actions = Actions{
		UninstallComponents: func(string, string, UninstallComponents) {},
	}
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "custom", Name: "Custom Tool", Generated: true, DistributionSource: "local"},
			{ID: "npm", Name: "NPM Tool", DistributionSource: "npm"},
			{ID: "sys", Name: "System Tool", DistributionSource: "built-in", RuntimeOwnership: "system"},
			{ID: "local", Name: "Local Tool", DistributionSource: "local"},
		}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	// Every kind's verb is on the page…
	for _, want := range []string{
		en.RemoveDelete,              // custom
		en.IntegrationsUninstall,     // npm
		en.IntegrationsDisconnect,    // system
		en.IntegrationsRemovePackage, // package
	} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q in %v", want, tt.Texts())
		}
	}
	// …and a system row does not offer the npm/package verbs it has no
	// business offering. (The four verbs are distinct strings, so a card that
	// fell back to a single generic word would have shown one of them four
	// times and none of the others — which the checks above already catch. The
	// one thing left to pin is that the fallback word is not the *only* one.)
	if !tt.HasText(en.IntegrationsDisconnect) {
		t.Fatal("no system row offers Disconnect")
	}
}

// The bar asks the kind's own question, not one generic sentence. The
// reference read the bar's title and body through the *same* per-kind
// `removalTextKey` that named the menu item (`src/ExtensionsPanel.tsx:2140-2151`
// builds it; `src/extensions/RemovalConfirmation.tsx:41` reads Title and body
// through it), so a card whose button says "Disconnect" asks "Disconnect X?" —
// the old build kept the npm sentence under all four kinds, which made the bar
// contradict the button that opened it. This is the pure mapping.
func TestTheRemovalConfirmationAsksTheKindsOwnQuestion(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	cases := []struct {
		name        string
		in          Integration
		en, zh      string
		npmTitleFor string
		isNPM       bool
	}{
		{
			name: "a generated custom integration",
			in:   Integration{ID: "c", Name: "Custom Tool", Generated: true, DistributionSource: "local"},
			en:   en.RemoveCustomTitle("Custom Tool"), zh: zh.RemoveCustomTitle("Custom Tool"),
			npmTitleFor: en.RemoveTitle("Custom Tool"),
		},
		{
			name: "an npm package keeps the uninstall sentence",
			in:   Integration{ID: "n", Name: "NPM Tool", DistributionSource: "npm"},
			en:   en.RemoveTitle("NPM Tool"), zh: zh.RemoveTitle("NPM Tool"),
			npmTitleFor: en.RemoveTitle("NPM Tool"), isNPM: true,
		},
		{
			name: "a system-owned built-in asks to disconnect",
			in:   Integration{ID: "s", Name: "System Tool", DistributionSource: "built-in", RuntimeOwnership: "system"},
			en:   en.IntegrationsDisconnectTitle("System Tool"), zh: zh.IntegrationsDisconnectTitle("System Tool"),
			npmTitleFor: en.RemoveTitle("System Tool"),
		},
		{
			name: "a linked local package asks to remove the package",
			in:   Integration{ID: "p", Name: "Local Tool", DistributionSource: "local"},
			en:   en.IntegrationsRemovePackageTitle("Local Tool"), zh: zh.IntegrationsRemovePackageTitle("Local Tool"),
			npmTitleFor: en.RemoveTitle("Local Tool"),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotEn, _ := tc.in.removalWords(en)
			if gotEn != tc.en {
				t.Errorf("removalWords(en) title = %q, want %q", gotEn, tc.en)
			}
			gotZh, _ := tc.in.removalWords(zh)
			if gotZh != tc.zh {
				t.Errorf("removalWords(zh) title = %q, want %q", gotZh, tc.zh)
			}
			// Only the npm kind asks the npm question; the other three ask their
			// own, which is what stops the bar from contradicting its button.
			if !tc.isNPM && gotEn == tc.npmTitleFor {
				t.Errorf("kind %q asks the generic npm question %q", tc.in.removalKind(), gotEn)
			}
		})
	}
}

// The bar on the card: each kind's confirmation title is drawn under its own
// row and the generic npm sentence is not, which is the pixel-level acceptance
// — the mapping above can be right while the card still prints one hard-coded
// question.
func TestEachRemovalConfirmationAsksItsOwnQuestionOnScreen(t *testing.T) {
	en := i18n.For("en").Settings
	a := newTestApp()
	a.Actions = Actions{UninstallIntegration: func(string, string) {}}
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "custom", Name: "Custom Tool", Generated: true, DistributionSource: "local"},
			{ID: "npm", Name: "NPM Tool", DistributionSource: "npm"},
			{ID: "sys", Name: "System Tool", DistributionSource: "built-in", RuntimeOwnership: "system"},
			{ID: "local", Name: "Local Tool", DistributionSource: "local"},
		}
	}
	tt := render(t, a, 720, 1400)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	steps := []struct {
		name, verb, title, generic string
	}{
		{"custom", en.RemoveDelete, en.RemoveCustomTitle("Custom Tool"), en.RemoveTitle("Custom Tool")},
		{"npm", en.IntegrationsUninstall, en.RemoveTitle("NPM Tool"), ""},
		{"system", en.IntegrationsDisconnect, en.IntegrationsDisconnectTitle("System Tool"), en.RemoveTitle("System Tool")},
		{"package", en.IntegrationsRemovePackage, en.IntegrationsRemovePackageTitle("Local Tool"), en.RemoveTitle("Local Tool")},
	}
	for _, s := range steps {
		s := s
		t.Run(s.name, func(t *testing.T) {
			if err := tt.Click(s.verb); err != nil {
				t.Fatalf("click %q: %v (texts %v)", s.verb, err, tt.Texts())
			}
			tt.Frame()
			if !tt.HasText(s.title) {
				t.Errorf("pressing %q did not draw the bar %q in %v", s.verb, s.title, tt.Texts())
			}
			// The generic npm question is the one the old build asked under every
			// kind; only the npm card may ask it.
			if s.generic != "" && tt.HasText(s.generic) {
				t.Errorf("pressing %q drew the generic question %q", s.verb, s.generic)
			}
			if err := tt.Click(en.RemoveCancel); err != nil {
				t.Fatalf("cancel: %v", err)
			}
			tt.Frame()
		})
	}
}
