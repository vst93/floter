package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// The repair a connected row offers is picked from two facts, exactly as the
// reference picked its one button (`src/extensions/ExtensionRow.tsx:345-370`):
// who owns the runtime (the system, so the search path can be asked again, or
// the package itself) and whether the manifest names a project page. One
// deliberate departure is pinned here too — a row broken *by its binding*
// keeps the repair, because this build has no separate install-repair button
// and the row would otherwise be a dead end.
func TestTheRepairFollowsTheRuntimeOwnerAndThePage(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	cases := []struct {
		name    string
		in      Integration
		kind    repairKind
		en      string
		zh      string
		tooltip string
	}{
		{
			name: "the runtime resolves, so there is nothing to repair",
			in:   Integration{ID: "ok", RuntimeOwnership: "system", RuntimeAvailable: true},
			kind: repairNone,
		},
		{
			name: "a system runtime the machine still owns is looked for again",
			in:   Integration{ID: "sys", RuntimeOwnership: "system"},
			kind: repairReconnect, en: en.IntegrationsReconnect, zh: zh.IntegrationsReconnect,
			tooltip: en.IntegrationsReconnectUnboundHint,
		},
		{
			name: "a system runtime with a page still reconnects",
			in:   Integration{ID: "sys-page", RuntimeOwnership: "system", Homepage: "https://example.com"},
			kind: repairReconnect, en: en.IntegrationsReconnect, zh: zh.IntegrationsReconnect,
			tooltip: en.IntegrationsReconnect,
		},
		{
			name: "a bundled runtime floter does not own can only point at the page",
			in:   Integration{ID: "bundled", RuntimeOwnership: "bundled", Homepage: "https://example.com"},
			kind: repairHomepage, en: en.IntegrationsInstallTool, zh: zh.IntegrationsInstallTool,
			tooltip: en.IntegrationsInstallTool,
		},
		{
			name: "a bundled runtime with no page has nowhere to send anyone",
			in:   Integration{ID: "bundled-quiet", RuntimeOwnership: "bundled"},
			kind: repairNone,
		},
		{
			name: "a probe failure shows its diagnosis, not a repair",
			in:   Integration{ID: "probe", RuntimeOwnership: "system", Broken: true, FailureCode: "probe_failed"},
			kind: repairNone,
		},
		{
			// The departure: the check that recorded this is what hid the row's
			// only way back, so a binding failure keeps the repair.
			name: "a binding failure keeps the repair that fixes it",
			in:   Integration{ID: "moved", RuntimeOwnership: "system", Broken: true, FailureCode: extensions.BindingCodeMissing},
			kind: repairReconnect, en: en.IntegrationsReconnect, zh: zh.IntegrationsReconnect,
			tooltip: en.IntegrationsReconnectUnboundHint,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.repairKind(); got != tc.kind {
				t.Fatalf("repairKind() = %v, want %v", got, tc.kind)
			}
			if tc.kind == repairNone {
				return
			}
			if got := tc.in.repairLabel(en); got != tc.en {
				t.Errorf("repairLabel(en) = %q, want %q", got, tc.en)
			}
			if got := tc.in.repairLabel(zh); got != tc.zh {
				t.Errorf("repairLabel(zh) = %q, want %q", got, tc.zh)
			}
			if got := tc.in.repairTooltip(en); got != tc.tooltip {
				t.Errorf("repairTooltip(en) = %q, want %q", got, tc.tooltip)
			}
		})
	}
}

// The mapping above can be right while the card still prints one hard-coded
// word for every row, so this is the pixel-level acceptance: each row that
// should carry a repair carries its own, each row that should not carries
// none, and pressing one reaches the action its arm names.
func TestEachUnavailableRowShowsItsOwnRepair(t *testing.T) {
	en := i18n.For("en").Settings
	var reconnected []string
	var opened []string

	a := newTestApp()
	a.Actions = Actions{
		ReconnectIntegration: func(id string) { reconnected = append(reconnected, id) },
		OpenHomepage:         func(url string) { opened = append(opened, url) },
		UninstallComponents:  func(string, string, UninstallComponents) {},
	}
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "sys", Name: "System Tool", RuntimeOwnership: "system"},
			{ID: "sys-page", Name: "Paged System Tool", RuntimeOwnership: "system", Homepage: "https://example.com/sys"},
			{ID: "bundled", Name: "Bundled Tool", RuntimeOwnership: "bundled", Homepage: "https://example.com/bundled"},
			{ID: "bundled-quiet", Name: "Quiet Tool", RuntimeOwnership: "bundled"},
			{ID: "ok", Name: "Working Tool", RuntimeOwnership: "system", RuntimeAvailable: true},
		}
	}
	tt := render(t, a, 760, 1400)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	// Two system rows reconnect and one bundled row can only offer its page;
	// the two remaining rows offer nothing at all. Counting is what pins the
	// second half: a card that fell back to one generic verb would move the
	// counts, and a card that drew the button for a row it should not would
	// too.
	if got := countText(tt.Texts(), en.IntegrationsReconnect); got != 2 {
		t.Errorf("the page shows %d Reconnect buttons, want 2 (%v)", got, tt.Texts())
	}
	if got := countText(tt.Texts(), en.IntegrationsInstallTool); got != 1 {
		t.Errorf("the page shows %d Install-tool-first buttons, want 1 (%v)", got, tt.Texts())
	}

	// Pressing them reaches the arm each names: the reconnect row its own id,
	// the page row its own url.
	if err := tt.Click(en.IntegrationsReconnect); err != nil {
		t.Fatalf("click reconnect: %v", err)
	}
	tt.Frame()
	if len(reconnected) != 1 || reconnected[0] != "sys" {
		t.Errorf("reconnected = %v, want [sys]", reconnected)
	}
	if err := tt.Click(en.IntegrationsInstallTool); err != nil {
		t.Fatalf("click install tool first: %v", err)
	}
	tt.Frame()
	if len(opened) != 1 || opened[0] != "https://example.com/bundled" {
		t.Errorf("opened = %v, want [https://example.com/bundled]", opened)
	}

	// A nil action hides its button rather than drawing a dead one: the shell
	// that cannot reconnect must not offer it.
	quiet := newTestApp()
	quiet.Actions = Actions{UninstallComponents: func(string, string, UninstallComponents) {}}
	quiet.Integrations = func() []Integration {
		return []Integration{{ID: "sys", Name: "System Tool", RuntimeOwnership: "system"}}
	}
	qt := render(t, quiet, 760, 600)
	if err := qt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v", err)
	}
	qt.Frame()
	if qt.HasText(en.IntegrationsReconnect) {
		t.Errorf("a shell with no reconnect action drew the button (%v)", qt.Texts())
	}
}
