package settingsui

import (
	"testing"

	"floter/internal/i18n"
)

// The integration's *type* is its own word, not the removal verb. The
// reference's meta row spent a slot on `t(integrationKindKey(extension))`
// (`src/extensions/ExtensionRow.tsx:220-222`, the kind words in
// `i18n.ts:446-453`), so a system integration reads "System tool" while its
// button reads "Disconnect" — two different facts that one generic word would
// collapse. This is the pure mapping; the pixel acceptance follows below.
func TestTheKindWordFollowsTheIntegrationType(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	cases := []struct {
		name string
		in   Integration
		en   string
		zh   string
	}{
		{
			name: "a generated custom integration",
			in:   Integration{ID: "custom", Generated: true, DistributionSource: "local"},
			en:   en.IntegrationsKindCustom, zh: zh.IntegrationsKindCustom,
		},
		{
			// Generated wins over npm, exactly as the reference walked it.
			name: "generated beats npm",
			in:   Integration{ID: "custom-npm", Generated: true, DistributionSource: "npm"},
			en:   en.IntegrationsKindCustom, zh: zh.IntegrationsKindCustom,
		},
		{
			name: "an npm package",
			in:   Integration{ID: "npm", DistributionSource: "npm"},
			en:   en.IntegrationsKindNPM, zh: zh.IntegrationsKindNPM,
		},
		{
			name: "a system-owned built-in",
			in:   Integration{ID: "sys", DistributionSource: "built-in", RuntimeOwnership: "system"},
			en:   en.IntegrationsKindSystem, zh: zh.IntegrationsKindSystem,
		},
		{
			// A built-in floter bundles and owns itself: it is a package.
			name: "a bundled built-in",
			in:   Integration{ID: "bundled", DistributionSource: "built-in", RuntimeOwnership: "bundled"},
			en:   en.IntegrationsKindPackage, zh: zh.IntegrationsKindPackage,
		},
		{
			name: "a linked local package",
			in:   Integration{ID: "local", DistributionSource: "local"},
			en:   en.IntegrationsKindPackage, zh: zh.IntegrationsKindPackage,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.integrationKind(en); got != tc.en {
				t.Errorf("integrationKind(en) = %q, want %q", got, tc.en)
			}
			if got := tc.in.integrationKind(zh); got != tc.zh {
				t.Errorf("integrationKind(zh) = %q, want %q", got, tc.zh)
			}
			// The type word and the removal verb are different facts about the
			// same row, so a card that reused one for the other is caught here.
			if tc.in.integrationKind(en) == tc.in.removalLabel(en) {
				t.Errorf("the kind and the removal verb both read %q", tc.en)
			}
		})
	}
}

// identityLine is the row the reference drew: identity first, then the kind.
func TestTheAppIdentityLineCarriesTheTypeWord(t *testing.T) {
	en := i18n.For("en").Settings
	in := Integration{ID: "sys", Publisher: "floter", Version: "1.2.3", DistributionSource: "built-in", RuntimeOwnership: "system"}
	if got, want := in.identityLine(en), "floter · 1.2.3 · sys · "+en.IntegrationsKindSystem; got != want {
		t.Errorf("identityLine() = %q, want %q", got, want)
	}
	// No identity fragments (an orphan's bare row) never leaves a dangling
	// separator: just the kind word.
	bare := Integration{Generated: true}.identityLine(en)
	if bare != en.IntegrationsKindCustom {
		t.Errorf("a bare row reads %q, want %q", bare, en.IntegrationsKindCustom)
	}
}

// The same pick reaches the card: each integration's meta row carries its own
// type word. A pure mapping can be right while the card still prints nothing
// or a hard-coded word — this is the pixel-level acceptance.
func TestEachIntegrationCardShowsItsOwnKind(t *testing.T) {
	en := i18n.For("en").Settings
	a := newTestApp()
	a.Actions = Actions{
		UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {},
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

	for _, want := range []string{
		en.IntegrationsKindCustom,
		en.IntegrationsKindNPM,
		en.IntegrationsKindSystem,
		en.IntegrationsKindPackage,
	} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q in %v", want, tt.Texts())
		}
	}
	// The system card names its type *and* still offers its own verb: the type
	// word did not displace the removal verb (F4a) it sits beside.
	if !tt.HasText(en.IntegrationsDisconnect) {
		t.Fatal("no system row still offers Disconnect")
	}
}
