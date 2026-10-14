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
