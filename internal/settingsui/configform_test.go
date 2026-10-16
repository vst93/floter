package settingsui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/theme"
)

// configExtSchema projects the form's own field shapes onto the host's, so a
// test can drive the host's document builder with the same schema the
// production shell does.
func configExtSchema(schema []ConfigField) []extensions.ConfigField {
	out := make([]extensions.ConfigField, 0, len(schema))
	for _, field := range schema {
		options := make([]any, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, option)
		}
		out = append(out, extensions.ConfigField{
			Key: field.Key, Type: field.Type, Label: field.Label, Description: field.Description,
			Required: field.Required, Default: field.Default, Options: options,
			Minimum: field.Minimum, Maximum: field.Maximum,
			MinLength: field.MinLength, MaxLength: field.MaxLength,
		})
	}
	return out
}

// configFormCalls records what the form asked the host to do, one list per
// kind of action, so a test reads the calls without threading four return
// values through every helper.
type configFormCalls struct {
	saved   []map[string]any
	exports []string
	copied  []configCopy
	success []string
}

// configFormApp is one integration whose config form is open, the fixture the
// config-form tests render against: SaveConfiguration records the last write,
// ExportConfiguration records the last export, CopyConfiguration records the
// copy request, and NotifySuccess records the confirmation the form raised.
func configFormApp(t *testing.T, schema []ConfigField, values map[string]any) (*App, *configFormCalls) {
	t.Helper()
	calls := &configFormCalls{}
	integration := Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Config: schema, ConfigValues: values,
	}
	a := oneIntegrationApp(integration, Actions{
		SaveConfiguration: func(id string, values map[string]any) error {
			calls.saved = append(calls.saved, values)
			return nil
		},
		ExportConfiguration: func(id string, values map[string]any) {
			calls.exports = append(calls.exports, id)
		},
		CopyConfiguration: func(id string, values map[string]any) {
			calls.copied = append(calls.copied, configCopy{id: id, values: values})
		},
		NotifySuccess: func(text string) {
			calls.success = append(calls.success, text)
		},
	})
	a.configOpen = map[string]bool{"dev.floter.tool": true}
	return a, calls
}

// configCopy is one CopyConfiguration call, as the form made it: the id and
// the draft values.
type configCopy struct {
	id     string
	values map[string]any
}

// The config form carries the reference's toolbar above its fields: the status
// word at the start and the copy/export icon-buttons at the end
// (`.extension-config-toolbar`, `extensions.css:2374-2376`,
// `ExtensionsPanel.tsx:2989-3000`). The old port drew neither — only the Save
// button below the fields — so a user could not see whether the form was
// clean, nor copy or export a configuration.
//
// The status word reads "Saved" for a draft that matches what is stored and
// "Unsaved changes" once the draft moves (the reference's `configDirty`,
// `ExtensionsPanel.tsx:969-970`).
func TestTheConfigFormWearsTheToolbarAndItsDirtyWord(t *testing.T) {
	en := i18n.For("en").Settings
	a, _ := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		map[string]any{"token": "abc"},
	)
	tt := integrationPage(t, a, 900, 1400)

	if !tt.HasText(en.ConfigSavedState) {
		t.Errorf("the config form has no status word %q: %q", en.ConfigSavedState, tt.Texts())
	}
	if tt.HasText(en.ConfigUnsaved) {
		t.Errorf("a clean form reads %q while it matches what is stored: %q", en.ConfigUnsaved, tt.Texts())
	}
	// The two icon-buttons are named for assistive technology, so their
	// presence is readable without the glyphs.
	if !tt.HasText(en.ConfigCopy) {
		t.Errorf("the toolbar has no copy button labeled %q", en.ConfigCopy)
	}
	if !tt.HasText(en.ConfigExport) {
		t.Errorf("the toolbar has no export button labeled %q", en.ConfigExport)
	}

	// Editing the draft — the same map the form holds — flips the status to
	// the dirty word.
	draft := a.draft(a.Integrations()[0])
	draft.values["token"] = "changed"
	tt.Frame()
	if !tt.HasText(en.ConfigUnsaved) {
		t.Errorf("an edited form does not read %q: %q", en.ConfigUnsaved, tt.Texts())
	}
}

// The config form's action row is the reference's
// `.extension-config-form__actions` (`extensions.css:2375`,
// `ExtensionsPanel.tsx:3015-3022`): a neutral "Restore defaults" pill at its
// start and the primary accent Save at its end. The old port drew the Save
// alone, with no way to put the fields back to their schema defaults.
//
// Pressing "Restore defaults" writes the schema's defaults into the draft (the
// reference's `configurationDefaults`, `ExtensionsPanel.tsx:2013-2016`),
// which moves the form off the stored values and therefore turns the status
// word dirty — and writes nothing to disk.
func TestTheConfigFormActionsAreDefaultsAndTheDirtyGatedSave(t *testing.T) {
	en := i18n.For("en").Settings
	a, calls := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text", Default: "seed"}},
		map[string]any{"token": "edited"},
	)
	tt := integrationPage(t, a, 900, 1400)

	if !tt.HasText(en.ConfigDefaults) {
		t.Fatalf("the config action row has no %q pill: %q", en.ConfigDefaults, tt.Texts())
	}
	// Seeded from the stored value, the form matches what is stored.
	if !tt.HasText(en.ConfigSavedState) {
		t.Errorf("an untouched form is not %q: %q", en.ConfigSavedState, tt.Texts())
	}

	if err := tt.Click(en.ConfigDefaults); err != nil {
		t.Fatalf("press %q: %v", en.ConfigDefaults, err)
	}
	tt.Frame()
	// The draft now holds the schema default ("seed"), which differs from the
	// stored value ("edited"), so the form reads dirty.
	if !tt.HasText(en.ConfigUnsaved) {
		t.Errorf("after Restore defaults the form does not read %q: the draft did not take the schema default; %q",
			en.ConfigUnsaved, tt.Texts())
	}
	if len(calls.saved) != 0 {
		t.Errorf("Restore defaults wrote a save (%d calls); it must only change the draft", len(calls.saved))
	}

	// The Save is what writes: pressing it hands the draft to the shell.
	if err := tt.Click(en.ConfigSave); err != nil {
		t.Fatalf("press %q: %v", en.ConfigSave, err)
	}
	tt.Frame()
	if len(calls.saved) != 1 {
		t.Fatalf("the Save wrote %d times, want 1", len(calls.saved))
	}
	if got := calls.saved[0]["token"]; got != "seed" {
		t.Errorf("the Save wrote token=%v, want the drafted default %q", got, "seed")
	}
	// The landed save is good news, so it raises the reference's success
	// notice (`ExtensionsPanel.tsx:2002-2003`) — never the danger ink the
	// error line wears.
	if len(calls.success) != 1 || calls.success[0] != en.ConfigSaved {
		t.Errorf("the save raised %q, want the single success notice %q", calls.success, en.ConfigSaved)
	}
	// The save re-baselines the draft (`setSavedConfigValues(saved.values)`,
	// `ExtensionsPanel.tsx:2001`), so the status returns to "Saved" at once —
	// not after the shell's off-thread list refresh, which has not run here.
	tt.Frame()
	if tt.HasText(en.ConfigUnsaved) {
		t.Errorf("after a landed Save the form still reads %q; it must re-baseline to %q",
			en.ConfigUnsaved, en.ConfigSavedState)
	}
}

// The copy icon-button dispatches the form's draft to the host's
// `CopyConfiguration` (`ExtensionsPanel.tsx:2019-2031`), and the host's
// `extensions.ExportConfiguration` builds the reference's
// `ConfigurationExportDocument` — the id, the config generation and the
// values — with a password replaced by the placeholder rather than the
// secret. The old port drew no copy button at all.
//
// The test presses the button and reads what the form handed the host, then
// runs the document builder the host uses over the same inputs, so the doc's
// shape and its redaction are both pinned.
func TestTheConfigCopyButtonCopiesTheExportDocument(t *testing.T) {
	en := i18n.For("en").Settings
	schema := []ConfigField{
		{Key: "token", Label: "Token", Type: "text"},
		{Key: "secret", Label: "Secret", Type: "password"},
	}
	values := map[string]any{"token": "abc", "secret": "hunter2"}
	a, calls := configFormApp(t, schema, values)
	tt := integrationPage(t, a, 900, 1400)

	if err := tt.Click(en.ConfigCopy); err != nil {
		t.Fatalf("press %q: %v", en.ConfigCopy, err)
	}
	tt.Frame()
	if len(calls.copied) != 1 {
		t.Fatalf("the copy button made %d host calls, want 1", len(calls.copied))
	}
	call := calls.copied[0]
	if call.id != "dev.floter.tool" {
		t.Errorf("the copy names the integration %q, want dev.floter.tool", call.id)
	}
	if call.values["token"] != "abc" {
		t.Errorf("the copy handed token=%v, want the draft's %q", call.values["token"], "abc")
	}
	if call.values["secret"] != "hunter2" {
		t.Errorf("the form must hand the host the secret and let it redact; got %v", call.values["secret"])
	}

	// The document the host builds from those values.
	document, err := extensions.ExportConfiguration("dev.floter.tool", 3, configExtSchema(schema), call.values)
	if err != nil {
		t.Fatalf("the host cannot build the export document: %v", err)
	}
	text := string(document)
	for _, want := range []string{`"extensionId": "dev.floter.tool"`, `"formatVersion": 1`, `"configVersion": 3`, `"token": "abc"`} {
		if !strings.Contains(text, want) {
			t.Errorf("the exported document lacks %s:\n%s", want, text)
		}
	}
	// The password never travels: the placeholder stands in for it.
	if strings.Contains(text, "hunter2") {
		t.Errorf("the exported document carries the password:\n%s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Errorf("the exported document does not redact the password field:\n%s", text)
	}
}

// The config form's select is the reference's `select`, which for a
// non-required field prepends an empty option (`{!field.required && <option
// value="">}`, with the `configNone` word; `ExtensionsPanel.tsx:3226-3228`).
// A required field has no such option. The old port built the option list
// straight from the schema, so a non-required choice could never be cleared
// back to "not set".
//
// The test reads the option list the form builds through `configSelectLabels`
// and then the live trigger's text, so both the rule and what the user sees
// are pinned.
func TestTheConfigSelectCarriesTheNoneOptionOnlyWhenNotRequired(t *testing.T) {
	en := i18n.For("en").Settings

	optional := ConfigField{Key: "mode", Type: "select", Options: []string{"fast", "slow"}}
	req := ConfigField{Key: "mode", Type: "select", Required: true, Options: []string{"fast", "slow"}}

	if got := configSelectLabels(optional, en.ConfigNone); len(got) != 3 || got[0] != en.ConfigNone {
		t.Errorf("the non-required select's options = %q, want a leading %q then the schema options", got, en.ConfigNone)
	}
	if got := configSelectLabels(req, en.ConfigNone); len(got) != 2 || got[0] == en.ConfigNone {
		t.Errorf("the required select's options = %q, want the schema options with no %q", got, en.ConfigNone)
	}

	a, _ := configFormApp(t, []ConfigField{optional}, map[string]any{})
	tt := integrationPage(t, a, 900, 1200)
	if !tt.HasText(en.ConfigNone) {
		t.Errorf("a non-required select shows no %q option: %q", en.ConfigNone, tt.Texts())
	}

	ab, _ := configFormApp(t, []ConfigField{req}, map[string]any{})
	tt2 := integrationPage(t, ab, 900, 1200)
	if tt2.HasText(en.ConfigNone) {
		t.Errorf("a required select shows the %q empty option: %q", en.ConfigNone, tt2.Texts())
	}
}

// The config form's multiselect is the reference's `select[multiple]`: one
// list box (`min-height: 66px; border-radius: var(--radius-sm)`,
// `.extension-config-field select[multiple]`, `extensions.css:2420-2423`).
// The toolkit has no multi-select widget, so the field stands in with one box
// of that shape holding the tick rows — not the loose ruled settings rows the
// old port drew (which put the options on the label column as card rows).
//
// The test reads the box's own geometry, bound directly: the box is a
// `--radius-sm` (9px) corner — the reference's list box shape, not a 999px
// pill — and clears the reference's 66px floor.
func TestTheConfigMultiselectIsOneListBox(t *testing.T) {
	a, _ := configFormApp(t,
		[]ConfigField{{
			Key: "tags", Label: "Tags", Type: "multiselect",
			Options: []string{"alpha", "beta", "gamma"},
		}},
		map[string]any{},
	)

	// The box at the panel's own metric: its floor is the reference's literal
	// 66px, whatever the interface step.
	for _, step := range []string{"small", "large"} {
		store := stepStore(t, step)
		a.Store = store
		tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})
		var ref ui.Handle
		var rect ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			ui.Column(c).Padding(24).Children(func() {
				a.extensionMetrics(func() {
					a.configMultiselectBox(c, func() {}).Bind(&ref)
				})
			})
			rect = c.Resolve(ref).Bounds()
		}, 400, 200)
		tt.Frame()
		if absFloat(rect.H-66) > 1 {
			t.Errorf("[%s] the multiselect box is %vpx high, want the reference's 66px list-box floor", step, rect.H)
		}
	}

	// The live form puts the option rows inside the box on the control column,
	// well right of the 118px label column.
	tt := integrationPage(t, a, 900, 1400)
	label, ok := tt.Find("Tags")
	if !ok {
		t.Fatalf("the multiselect's label is missing: %q", tt.Texts())
	}
	alpha, ok := tt.Find("alpha")
	if !ok {
		t.Fatalf("the multiselect's first option is missing: %q", tt.Texts())
	}
	if alpha.X <= label.X+60 {
		t.Errorf("the option at x=%v sits on/near the label column at x=%v; it belongs inside the list box in the second column",
			alpha.X, label.X)
	}
}
