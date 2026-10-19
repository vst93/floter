package settingsui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The Integrations panel's command list was the last settings surface never
// compared against the reference field by field. `main:src/ExtensionsPanel.tsx`
// draws one plate per command — `.extension-command-list__row` — and
// `main:src/styles/extensions.css` names every piece:
//
//	.extension-command-list__row       hairline edge, `--glass-control` fill,
//	                                    `--ext-radius` corner, 10px/12px pad
//	.extension-command-list__row--off  opacity 0.62 while the switch is off
//	.extension-command-list__head      identity left, switch right (flex-start)
//	.extension-command-list code       the command name, caption, secondary
//	.extension-command-list span       description / triggers, caption, muted
//	.extension-command-list__triggers  "Summon with: …" at opacity 0.85
//	.extension-command-alias           the alias editor, full card width
//	.extension-command-alias__label    caption, muted
//	.extension-command-alias__taken    the muted caption note
//
// The port drew one flat settings row per command with a `· runtime
// unavailable` suffix it invented, and hung the alias editor off that row as a
// *second* ruled row. These pin the reference's shape.

// commandListIntegration is a three-command integration for the tests below: one
// enabled command with a description and a declared alias, one command whose
// switch is off with neither, and the host's own synthesized configuration
// command (never declared by the provider, so it is not a command of this list
// at all — the reference draws it as the "Configuration" block instead).
func commandListIntegration() Integration {
	return Integration{
		ID: "ext", Name: "V Tools", Enabled: true, Running: true,
		DistributionSource: "local", RuntimeOwnership: "system",
		Commands: []Command{
			{ID: "jv", Name: "JSON Viewer", Description: "View JSON", Aliases: []string{"jview", "viewer"}, Enabled: true, Available: true, Declared: true},
			{ID: "build", Name: "Build", Enabled: false, Declared: true},
			{ID: "configuration", Name: "Configuration", Description: "Edit this integration's configuration"},
		},
	}
}

// revealCommandList renders the Integrations page and returns the tester on it.
func revealCommandList(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// The command list is the reference's *plates*, not a column of ruled settings
// rows: every command carries its own hairline-edged, control-filled card, and
// a command whose switch is off is dimmed as a whole — the reference's
// `.extension-command-list__row--off { opacity: 0.62 }`. The port drew plain
// settings rows with no plate at all, so the two commands were indistinguishable
// from the rows above them.
func TestTheCommandListDrawsTheReferencePlates(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration { return []Integration{commandListIntegration()} }
	tt := revealCommandList(t, a)

	// There are no ruled settings rows for the commands any more: the section
	// heading is the reference's `h4`, and the two commands are plates.
	if !tt.HasText(copy.IntegrationsCommands) || !tt.HasText(copy.IntegrationsCommandsHint) {
		t.Fatalf("the command list caption/hint = %q", tt.Texts())
	}
	// Each plate's own box: a control plane distinct from the card behind it.
	enabledName, ok := tt.Find("JSON Viewer")
	if !ok {
		t.Fatalf("the first command's name is missing: %q", tt.Texts())
	}
	offName, ok := tt.Find("Build")
	if !ok {
		t.Fatalf("the second command's name is missing: %q", tt.Texts())
	}
	// The plates are wider than their text and start left of it, so a pixel
	// just inside the plate's own edge is the plate's plane.
	if _, ok := tt.Find("View JSON"); !ok {
		t.Errorf("the description is missing: %q", tt.Texts())
	}
	// The reference prints the summon vocabulary: id, name and aliases, each
	// reduced to its first token.
	if want := copy.IntegrationsCommandsTriggers("jv, JSON, jview, viewer"); !tt.HasText(want) {
		t.Errorf("the trigger line %q is missing: %q", want, tt.Texts())
	}
	// A command the provider never described says so rather than leaving the
	// line blank (the reference's `noDescription`).
	if want := copy.IntegrationsNoDescription; !tt.HasText(want) {
		t.Errorf("the no-description line %q is missing: %q", want, tt.Texts())
	}
	// The host's synthesized configuration command is not in the provider
	// registry, so the list has no plate for it: no summon line, and neither of
	// the plate's two controls (the launcher switch and the alias field). The
	// reference's list *is* the registry (`ExtensionsPanel.tsx:2777-2871` over
	// `externalCommands`), and a tool-owned configuration is the block above
	// with an "Open configuration" action (`:2971-2986`).
	if want := copy.IntegrationsCommandsTriggers("configuration, Configuration"); tt.HasText(want) {
		t.Errorf("the synthesized command printed a summon line %q: %q", want, tt.Texts())
	}
	if want := copy.IntegrationsCommandEnable("Configuration"); tt.HasText(want) {
		t.Errorf("the synthesized command has a launcher switch %q: %q", want, tt.Texts())
	}
	if want := copy.IntegrationsCommandAliasFor("Configuration"); tt.HasText(want) {
		t.Errorf("the synthesized command has an alias field %q: %q", want, tt.Texts())
	}
	// The alias editor is the reference's own field, named for its command.
	if want := copy.IntegrationsCommandAliasFor("JSON Viewer"); !tt.HasText(want) {
		t.Errorf("the alias field %q is missing: %q", want, tt.Texts())
	}
	if want := copy.IntegrationsCommandAliasFor("Build"); !tt.HasText(want) {
		t.Errorf("the alias field %q is missing: %q", want, tt.Texts())
	}
	// The switch's accessible name is the reference's `commandEnable` wording.
	if want := copy.IntegrationsCommandEnable("JSON Viewer"); !tt.HasText(want) {
		t.Errorf("the switch %q is missing: %q", want, tt.Texts())
	}

	// The plate's own plane: the plate is a `--glass-control` fill, so a pixel
	// inside it differs from the page's own fill to the plate's left.
	platePlane := pixelAt(tt, int(enabledName.X)-6, int(enabledName.Y+enabledName.H/2))
	pagePlane := pixelAt(tt, 2, int(enabledName.Y+enabledName.H/2))
	if platePlane == pagePlane {
		t.Errorf("the command plate paints no plane: %v vs the page %v", platePlane, pagePlane)
	}

	// The dimmed plate: the off command's whole plate is at 0.62, so its fill
	// is lighter than the enabled command's own plate.
	offPlate := pixelAt(tt, int(offName.X)-6, int(offName.Y+offName.H/2))
	if offPlate == platePlane {
		t.Errorf("the off command's plate is not dimmed: %v == %v", offPlate, platePlane)
	}
}

// The switch — not the row's label — is what toggles a command. The reference
// gives the switch its own `aria-label` (`commandEnable`) and wires the click
// to it alone, so a press on the command's *name* does not flip it. The port
// wired the whole row's label, which also meant the row's label text was a
// second, unlabelled control.
func TestOnlyTheCommandSwitchTogglesTheCommand(t *testing.T) {
	copy := i18n.For("en").Settings
	toggled := [][3]string{}
	a := New(newStore(t), Actions{
		SetCommandEnabled: func(extensionID, commandID string, enabled bool) {
			toggled = append(toggled, [3]string{extensionID, commandID, boolText(enabled)})
		},
	})
	a.Integrations = func() []Integration { return []Integration{commandListIntegration()} }
	tt := revealCommandList(t, a)

	// A press on the command's own name is not a switch flip.
	if err := tt.Click("JSON Viewer"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(toggled) != 0 {
		t.Fatalf("the command's name toggled the switch: %v", toggled)
	}
	// The switch itself flips it, and reports the *next* state.
	if err := tt.Click(copy.IntegrationsCommandEnable("Build")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(toggled) != 1 || toggled[0] != [3]string{"ext", "build", "true"} {
		t.Errorf("toggled = %v, want the off command turned on", toggled)
	}
}

// The trigger vocabulary is the reference's `pluginCommandTriggers`: the id,
// the name and every declared alias, each reduced to its first
// whitespace-separated token and de-duplicated case-insensitively (the mode is
// entered by one word followed by a space, so a multi-word name contributes
// only its first word).
func TestCommandTriggerWordsMatchTheReferenceVocabulary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command Command
		want    []string
	}{
		{
			"id, name and aliases",
			Command{ID: "jv", Name: "JSON Viewer", Aliases: []string{"jview", "viewer"}},
			[]string{"jv", "JSON", "jview", "viewer"},
		},
		{
			"a multi-word name contributes its first token only",
			Command{ID: "search", Name: "Search Acme", Aliases: []string{"acme"}},
			[]string{"search", "acme"},
		},
		{
			"a blank name leaves the id and aliases",
			Command{ID: "jv", Aliases: []string{"viewer"}},
			[]string{"jv", "viewer"},
		},
		{
			"case-insensitive duplicates collapse",
			Command{ID: "jv", Name: "JV", Aliases: []string{"jViewer", "jv"}},
			[]string{"jv", "jViewer"},
		},
		{
			"blank aliases are dropped",
			Command{ID: "jv", Aliases: []string{"  ", "viewer"}},
			[]string{"jv", "viewer"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := commandTriggerWords(tc.command)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("commandTriggerWords = %v, want %v", got, tc.want)
			}
		})
	}
}

// boolText is strconv.FormatBool under a local name, so the test reads.
func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
