package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// The repair a connected row offers is picked from two facts, exactly as the
// reference picked its one button (`src/extensions/ExtensionRow.tsx:345-370`):
// who owns the runtime (the system, so the search path can be asked again, or
// the package itself) and whether the manifest names a project page. A *broken*
// row is not one of these: it shows the re-check instead
// (`src/extensions/ExtensionRow.tsx:313-330`), which is the arm
// `TestABrokenRowOffersTheRecheck` pins at the pixel level.
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
			// A broken row shows the re-check, not the reconnect: the reference
			// drew exactly one entry for this state, and it was the Wrench.
			name: "a broken row shows the re-check, never the reconnect",
			in:   Integration{ID: "moved", RuntimeOwnership: "system", Broken: true, FailureCode: extensions.BindingCodeMissing},
			kind: repairNone,
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
		UninstallComponents:  func(string, string, string, UninstallComponents, func(error)) {},
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
	quiet.Actions = Actions{UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {}}
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

// A broken row's own entry is the re-check, and pressing it reaches the action
// that verifies and repairs the package. The reconnect button the unavailable
// rows carry must not also be drawn on it: the reference drew one entry per
// state, and two repairs that answer different questions on one row read as
// one choice the user cannot make.
func TestABrokenRowOffersTheRecheck(t *testing.T) {
	en := i18n.For("en").Settings
	var repaired []string
	var reconnected []string

	a := newTestApp()
	a.Actions = Actions{
		RepairIntegration:    func(id string) { repaired = append(repaired, id) },
		ReconnectIntegration: func(id string) { reconnected = append(reconnected, id) },
		UninstallComponents:  func(string, string, string, UninstallComponents, func(error)) {},
	}
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "broken", Name: "Broken Tool", Enabled: true, Broken: true,
				RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing},
			{ID: "moved", Name: "Moved Tool", RuntimeOwnership: "system"},
		}
	}
	tt := render(t, a, 760, 1200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	// One re-check (the broken row) and one reconnect (the moved one), each on
	// its own row: a broken row that fell back to the reconnect would make the
	// counts two and one, and the reverse would make them zero and two.
	if got := countText(tt.Texts(), en.IntegrationsRecheck); got != 1 {
		t.Errorf("the page shows %d Re-check buttons, want 1 (%v)", got, tt.Texts())
	}
	if got := countText(tt.Texts(), en.IntegrationsReconnect); got != 1 {
		t.Errorf("the page shows %d Reconnect buttons, want 1 (%v)", got, tt.Texts())
	}

	if err := tt.Click(en.IntegrationsRecheck); err != nil {
		t.Fatalf("click re-check: %v", err)
	}
	tt.Frame()
	if len(repaired) != 1 || repaired[0] != "broken" {
		t.Errorf("repaired = %v, want [broken]", repaired)
	}
	if len(reconnected) != 0 {
		t.Errorf("reconnected = %v, want the re-check to be the only entry pressed", reconnected)
	}

	// A shell with no repair action hides the button rather than drawing a
	// dead one.
	quiet := newTestApp()
	quiet.Actions = Actions{UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {}}
	quiet.Integrations = func() []Integration {
		return []Integration{{ID: "broken", Name: "Broken Tool", Enabled: true, Broken: true,
			RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing}}
	}
	qt := render(t, quiet, 760, 600)
	if err := qt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v", err)
	}
	qt.Frame()
	if qt.HasText(en.IntegrationsRecheck) {
		t.Errorf("a shell with no repair action drew the button (%v)", qt.Texts())
	}
}
