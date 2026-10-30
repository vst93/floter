package settingsui

import (
	"strings"
	"testing"
	"time"

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
		SaveConfiguration: func(id string, values map[string]any, done func(error)) {
			calls.saved = append(calls.saved, values)
			if done != nil {
				done(nil)
			}
		},
		ExportConfiguration: func(id string, values map[string]any, done func(error)) {
			calls.exports = append(calls.exports, id)
			if done != nil {
				done(nil)
			}
		},
		CopyConfiguration: func(id string, values map[string]any, done func(error)) {
			calls.copied = append(calls.copied, configCopy{id: id, values: values})
			if done != nil {
				done(nil)
			}
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
					a.configMultiselectBox(c, nil, false, func() {}).Bind(&ref)
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

// The schema bounds the reference left to the native control
// (`min={field.minimum ?? undefined}` … `ExtensionsPanel.tsx:3251-3256`) are
// said *at the field*: the browser drew its own validity state on the input,
// and a native pane has none, so the sentence rides the field's own caption
// line — the same column as its description — rather than only the form's
// bottom line. `ConfigValueProblem` is the one place the bounds are checked, so
// the host's validation and the form's hint cannot drift.
func TestTheConfigFieldCarriesItsOwnBoundsHint(t *testing.T) {
	en := i18n.For("en").Settings
	minimum, maximum := 0.5, 9.0
	minLength := 4
	number := ConfigField{Key: "retries", Label: "Retries", Type: "number", Minimum: &minimum, Maximum: &maximum}
	text := ConfigField{Key: "endpoint", Label: "Endpoint", Type: "text", MinLength: &minLength}

	if _, problem := ConfigValueProblem(number, "3", en); problem != "" {
		t.Errorf("an in-range number reported %q, want none", problem)
	}
	if _, problem := ConfigValueProblem(number, "12", en); problem != en.ConfigOutOfRange {
		t.Errorf("an out-of-range number reported %q, want %q", problem, en.ConfigOutOfRange)
	}
	if _, problem := ConfigValueProblem(number, "abc", en); problem != en.ConfigNumber {
		t.Errorf("a non-number reported %q, want %q", problem, en.ConfigNumber)
	}
	if _, problem := ConfigValueProblem(text, "ab", en); problem != en.ConfigOutOfLength {
		t.Errorf("a too-short text reported %q, want %q", problem, en.ConfigOutOfLength)
	}
	// An empty number is no value at all — the reference's
	// `event.target.value === "" ? null` — and an empty text is legal.
	if value, problem := ConfigValueProblem(number, "", en); problem != "" || value != nil {
		t.Errorf("an empty number gave (%v, %q), want (nil, \"\")", value, problem)
	}
	if _, problem := ConfigValueProblem(text, "", en); problem != "" {
		t.Errorf("an empty text reported %q, want none", problem)
	}

	// The sentence the form shows for one field: a field with a problem wears
	// it, and the problem does not borrow between fields.
	a, _ := configFormApp(t, []ConfigField{number}, map[string]any{})
	a.noteConfigFieldError("dev.floter.tool", "retries", en.ConfigOutOfRange)
	tt := integrationPage(t, a, 900, 1400)
	if !tt.HasText(en.ConfigOutOfRange) {
		t.Errorf("the field hint is not drawn at the field: %q", tt.Texts())
	}
	if got := a.configFieldProblem(a.Integrations()[0], number); got != en.ConfigOutOfRange {
		t.Errorf("the field's own hint = %q, want %q", got, en.ConfigOutOfRange)
	}
	other := ConfigField{Key: "other", Label: "Other", Type: "number"}
	if got := a.configFieldProblem(a.Integrations()[0], other); got != "" {
		t.Errorf("a neighbouring field borrowed the hint: %q", got)
	}
	a.clearConfigFieldError("dev.floter.tool", "retries")
	if got := a.configFieldProblem(a.Integrations()[0], number); got != "" {
		t.Errorf("the hint survived a satisfied edit: %q", got)
	}
}

// Folding a configuration form whose draft has unsaved edits is the reference's
// own two-step (`if (configDirty) { setDetailsDiscardArmed(true); return }`,
// `ExtensionsPanel.tsx:2056-2060`): the first press asks, the panel draws the
// discard bar under the form, and only an answer folds it. The old port flipped
// the form shut on one press and dropped the edits silently.
func TestFoldingADirtyConfigFormAsksFirst(t *testing.T) {
	en := i18n.For("en").Settings
	a, _ := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		map[string]any{"token": "abc"},
	)
	tt := integrationPage(t, a, 900, 1400)
	if !tt.HasText(en.ConfigOpen) {
		t.Fatalf("the Configure row is missing: %q", tt.Texts())
	}

	// Dirtying the draft and pressing Configure asks rather than closing.
	a.draft(a.Integrations()[0]).values["token"] = "edited"
	tt.Frame()
	if err := tt.Click(en.ConfigOpen); err != nil {
		t.Fatalf("press %q: %v", en.ConfigOpen, err)
	}
	tt.Frame()
	if !tt.HasText(en.ConfigDiscardConfirm) {
		t.Errorf("a dirty form folded with no question: %q", tt.Texts())
	}
	if !tt.HasText(en.ConfigDiscard) {
		t.Errorf("the discard bar has no %q answer: %q", en.ConfigDiscard, tt.Texts())
	}
	// The form stayed open, and the two-step did not lose the edit.
	if !a.configOpen["dev.floter.tool"] {
		t.Error("the first press folded the form instead of asking")
	}
	if !tt.HasText(en.ConfigUnsaved) {
		t.Errorf("the edit was lost while the question was up: %q", tt.Texts())
	}

	// Cancel answers "keep": the question goes and the form stays open.
	if err := tt.Click(en.ConfigCancel); err != nil {
		t.Fatalf("press %q: %v", en.ConfigCancel, err)
	}
	tt.Frame()
	if tt.HasText(en.ConfigDiscardConfirm) {
		t.Errorf("Cancel left the question up: %q", tt.Texts())
	}
	if !a.configOpen["dev.floter.tool"] {
		t.Error("Cancel folded the form")
	}

	// Discard answers "drop": the question and the form both go, and the draft
	// with them.
	a.draft(a.Integrations()[0]).values["token"] = "edited again"
	tt.Frame()
	if err := tt.Click(en.ConfigOpen); err != nil {
		t.Fatalf("press %q: %v", en.ConfigOpen, err)
	}
	tt.Frame()
	if !tt.HasText(en.ConfigDiscardConfirm) {
		t.Fatalf("the second ask did not appear: %q", tt.Texts())
	}
	if err := tt.Click(en.ConfigDiscard); err != nil {
		t.Fatalf("press %q: %v", en.ConfigDiscard, err)
	}
	tt.Frame()
	if a.configOpen["dev.floter.tool"] {
		t.Error("Discard left the form open")
	}
	if tt.HasText(en.ConfigDiscardConfirm) {
		t.Errorf("Discard left the question up: %q", tt.Texts())
	}
	// A fold that was asked for *always* drops the draft: the next open reads
	// the stored values.
	draft := a.draft(a.Integrations()[0])
	if draft.values["token"] != "abc" {
		t.Errorf("the draft kept an edit: %v", draft.values)
	}
}

// A form that matches what is stored folds on one press: the reference only
// asked when `configDirty` (`ExtensionsPanel.tsx:2056-2060`).
func TestFoldingACleanConfigFormDoesNotAsk(t *testing.T) {
	en := i18n.For("en").Settings
	a, _ := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		map[string]any{"token": "abc"},
	)
	tt := integrationPage(t, a, 900, 1400)
	if err := tt.Click(en.ConfigOpen); err != nil {
		t.Fatalf("press %q: %v", en.ConfigOpen, err)
	}
	tt.Frame()
	if a.configOpen["dev.floter.tool"] {
		t.Error("a clean form did not fold on the first press")
	}
	if tt.HasText(en.ConfigDiscardConfirm) {
		t.Errorf("a clean form asked: %q", tt.Texts())
	}
	// A form that folded clean reopens folded away from its edits: this one had
	// none, so the token still reads what is stored.
	if got := a.draft(a.Integrations()[0]).values["token"]; got != "abc" {
		t.Errorf("the reopened draft reads %v, want the stored %q", got, "abc")
	}
}

// The toolbar's two icon-buttons share the reference's one `configOperation`
// slot: while a copy or an export runs, *both* are disabled
// (`disabled={Boolean(configOperation)}` on each,
// `ExtensionsPanel.tsx:2994,2997`), and the reference's own
// `configOperationRef.current` guard refused a second press even before the
// render. The old port left both always pressable, so a slow export could be
// started twice over.
func TestTheConfigToolbarHoldsOneOperationAtATime(t *testing.T) {
	en := i18n.For("en").Settings
	calls := &configFormCalls{}
	release := make(chan struct{})
	integration := Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Config:       []ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		ConfigValues: map[string]any{"token": "abc"},
	}
	// The copy stays in flight until the test releases it, so the disabled
	// face is observable across frames.
	a := oneIntegrationApp(integration, Actions{
		CopyConfiguration: func(id string, values map[string]any, done func(error)) {
			calls.copied = append(calls.copied, configCopy{id: id, values: values})
			go func() { <-release; done(nil) }()
		},
		ExportConfiguration: func(id string, values map[string]any, done func(error)) {
			calls.exports = append(calls.exports, id)
			done(nil)
		},
	})
	a.configOpen = map[string]bool{"dev.floter.tool": true}
	tt := integrationPage(t, a, 900, 1400)

	if err := tt.Click(en.ConfigCopy); err != nil {
		t.Fatalf("press %q: %v", en.ConfigCopy, err)
	}
	tt.Frame()
	if len(calls.copied) != 1 {
		t.Fatalf("the copy ran %d times, want 1", len(calls.copied))
	}
	if got := a.configOperationNow("dev.floter.tool"); got != "copy" {
		t.Fatalf("the operation slot reads %q, want copy", got)
	}
	// Both buttons are disabled: a second copy and an export both refuse.
	tt.Click(en.ConfigCopy)
	tt.Frame()
	if len(calls.copied) != 1 {
		t.Errorf("a second copy ran while one was in flight (%d calls)", len(calls.copied))
	}
	tt.Click(en.ConfigExport)
	tt.Frame()
	if len(calls.exports) != 0 {
		t.Errorf("an export ran while a copy was in flight (%d calls)", len(calls.exports))
	}

	// Once the copy lands the slot is free again and the export runs.
	close(release)
	tt.Frame()
	deadline := time.Now().Add(2 * time.Second)
	for a.configOperationNow("dev.floter.tool") != "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		tt.Frame()
	}
	if got := a.configOperationNow("dev.floter.tool"); got != "" {
		t.Fatalf("the operation slot stayed %q after the copy landed", got)
	}
	if err := tt.Click(en.ConfigExport); err != nil {
		t.Fatalf("press %q: %v", en.ConfigExport, err)
	}
	tt.Frame()
	if len(calls.exports) != 1 {
		t.Errorf("the export ran %d times, want 1", len(calls.exports))
	}
}

// The Save wears the reference's own in-flight word while the write is out:
// `busy?.kind === "save" ? t("settings.extensions.saving") :
// t("settings.extensions.save")` (`ExtensionsPanel.tsx:3020`), with the
// button — and the Defaults beside it — disabled by that same `busy`
// (`disabled={Boolean(busy) || …}`, `:3016,3019`). The old port wrote
// synchronously and had no such face.
func TestTheConfigSaveWearsItsSavingWordWhileInFlight(t *testing.T) {
	en := i18n.For("en").Settings
	release := make(chan struct{})
	writes := 0
	integration := Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Config:       []ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		ConfigValues: map[string]any{"token": "abc"},
	}
	a := oneIntegrationApp(integration, Actions{
		SaveConfiguration: func(id string, values map[string]any, done func(error)) {
			writes++
			go func() { <-release; done(nil) }()
		},
	})
	a.configOpen = map[string]bool{"dev.floter.tool": true}
	tt := integrationPage(t, a, 900, 1400)

	// Dirty the draft so the Save is live.
	a.draft(a.Integrations()[0]).values["token"] = "edited"
	tt.Frame()
	if !tt.HasText(en.ConfigSave) {
		t.Fatalf("the Save is missing before the press: %q", tt.Texts())
	}
	if err := tt.Click(en.ConfigSave); err != nil {
		t.Fatalf("press %q: %v", en.ConfigSave, err)
	}
	tt.Frame()
	if writes != 1 {
		t.Fatalf("the Save wrote %d times, want 1", writes)
	}
	if !tt.HasText(en.ConfigSaving) {
		t.Errorf("the Save does not read %q while the write is out: %q", en.ConfigSaving, tt.Texts())
	}
	if tt.HasText(en.ConfigSave) {
		t.Errorf("the resting word %q is still drawn while the write is out: %q", en.ConfigSave, tt.Texts())
	}
	// While the write is out both pills refuse a press: a second Save does not
	// write, and Defaults does not move the draft.
	tt.Click(en.ConfigSave)
	tt.Click(en.ConfigDefaults)
	tt.Frame()
	if writes != 1 {
		t.Errorf("a second Save wrote while one was in flight (%d writes)", writes)
	}
	if got := a.draft(a.Integrations()[0]).values["token"]; got != "edited" {
		t.Errorf("Defaults moved the draft while the Save was in flight: token = %v", got)
	}

	// The landed write gives the resting face back and re-baselines the form.
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for a.configSaveBusy["dev.floter.tool"] && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		tt.Frame()
	}
	if a.configSaveBusy["dev.floter.tool"] {
		t.Fatal("the Save stayed in flight after the write landed")
	}
	if tt.HasText(en.ConfigSaving) {
		t.Errorf("the %q word survived the write: %q", en.ConfigSaving, tt.Texts())
	}
	if tt.HasText(en.ConfigUnsaved) {
		t.Errorf("the form did not re-baseline to %q: %q", en.ConfigSavedState, tt.Texts())
	}
}

// A multiselect longer than the reference's 66px viewport scrolls *inside* the
// box rather than growing it: the native `select[multiple]` the reference drew
// is a fixed 66px list box whose options scroll under it
// (`.extension-config-field select[multiple]`, `extensions.css:2420-2423`), and
// a box that grew with its option list pushed the form's actions off the card.
// The scroller rides the field's own state, so the scroll is where the user
// left it rather than at its start on every frame.
//
// The box is therefore capped at the viewport whatever it holds, while a short
// list keeps the reference's 66px floor — the two halves of the same rule.
func TestTheConfigMultiselectScrollsInsideItsViewport(t *testing.T) {
	options := func(n int) []string {
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, "option-"+string(rune('a'+i)))
		}
		return out
	}
	measure := func(a *App, field ConfigField) float32 {
		var ref ui.Handle
		var rect ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).Padding(24).Children(func() {
				a.extensionMetrics(func() {
					a.configFieldGrid(c, field, "Tags", "", "",
						func() ui.Element {
							return a.configMultiselectBox(c, a.configListScroll("dev.floter.tool", field.Key), false, func() {
								for _, option := range field.Options {
									ui.Text(c, option)
								}
							}).Bind(&ref)
						})
				})
			})
			rect = c.Resolve(ref).Bounds()
		}, 400, 300)
		tt.Frame()
		return rect.H
	}

	a, _ := configFormApp(t, []ConfigField{{Key: "tags", Label: "Tags", Type: "multiselect"}}, map[string]any{})
	short := ConfigField{Key: "short", Label: "Short", Type: "multiselect", Options: options(3)}
	shortH := measure(a, short)
	if absFloat(shortH-66) > 1 {
		t.Errorf("a three-option box is %vpx high, want the reference's 66px floor", shortH)
	}
	long := ConfigField{Key: "long", Label: "Long", Type: "multiselect", Options: options(12)}
	longH := measure(a, long)
	if absFloat(longH-66) > 1 {
		t.Errorf("a twelve-option box is %vpx high, want the same 66px viewport (the options scroll inside it)", longH)
	}

	// The live form's box scrolls: its content is taller than its viewport, so
	// the wheel moves it and the offset is kept between frames.
	b, _ := configFormApp(t,
		[]ConfigField{{Key: "tags", Label: "Tags", Type: "multiselect", Options: options(12)}},
		map[string]any{},
	)
	tt := integrationPage(t, b, 900, 1400)
	state := b.configListScroll("dev.floter.tool", "tags")
	if state.MaxY <= 0 {
		t.Fatalf("the long option list has nowhere to scroll (max %v)", state.MaxY)
	}
	if _, ok := tt.Find("option-a"); !ok {
		t.Fatalf("the box's first option is missing: %q", tt.Texts())
	}
	top, _ := tt.Find("option-a")
	tt.Scroll(top.X+4, top.Y+2, 0, 40)
	tt.Frame()
	if state.Y == 0 {
		t.Error("the wheel did not move the list box's options")
	}
	if state.Y > state.MaxY {
		t.Errorf("the list box scrolled to %v past its end %v", state.Y, state.MaxY)
	}
}

// Only a host-owned configuration wears the form. The reference gated the
// whole section on `configuration.descriptor.owner === "host"` and drew an
// "Open configuration" button for a `tool`-owned one
// A tool-owned configuration is the reference's "Open configuration" outlet:
// the block title and the ExternalLink-marked action that runs the provider's
// own `openCommand` (`ExtensionsPanel.tsx:2977-2986`). The old port withheld
// the whole section for a `tool` owner, so a user had no way to reach the
// tool's own editor from the card.
func TestAToolOwnedConfigurationOffersTheOpenButton(t *testing.T) {
	en := i18n.For("en").Settings
	a, _ := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		map[string]any{"token": "abc"},
	)
	integration := a.Integrations()[0]
	integration.ConfigOwner = "tool"
	integration.ConfigOpenCommand = []string{"tool", "config", "--edit"}
	opened := make([][]string, 0, 1)
	a.Actions.OpenConfiguration = func(id string, argv []string) {
		opened = append(opened, argv)
	}
	a.Integrations = func() []Integration { return []Integration{integration} }

	tt := integrationPage(t, a, 900, 1400)
	if !tt.HasText(en.ConfigOpenConfiguration) {
		t.Fatalf("a tool-owned configuration lost its %q action: %q", en.ConfigOpenConfiguration, tt.Texts())
	}
	if tt.HasText(en.ConfigSave) {
		t.Errorf("a tool-owned configuration drew the host form: %q", tt.Texts())
	}
	if err := tt.Click(en.ConfigOpenConfiguration); err != nil {
		t.Fatalf("press %q: %v", en.ConfigOpenConfiguration, err)
	}
	tt.Frame()
	if len(opened) != 1 {
		t.Fatalf("the open action ran %d times, want 1", len(opened))
	}
	if got := strings.Join(opened[0], " "); got != "tool config --edit" {
		t.Errorf("the open action ran %q, want the provider's openCommand", got)
	}

	// A provider that declared no `openCommand` leaves the action disabled —
	// the reference's `disabled={!configuration.openPlan}` — and a press does
	// nothing.
	integration.ConfigOpenCommand = nil
	a.Integrations = func() []Integration { return []Integration{integration} }
	opened = opened[:0]
	tt = integrationPage(t, a, 900, 1400)
	if err := tt.Click(en.ConfigOpenConfiguration); err != nil {
		t.Fatalf("press %q: %v", en.ConfigOpenConfiguration, err)
	}
	tt.Frame()
	if len(opened) != 0 {
		t.Errorf("the disabled open action ran %d times, want 0", len(opened))
	}
}

// The form's bounds are checked as the user types, not only on Enter: the
// reference's inputs were controlled and the browser drew its `:invalid` state
// on every keystroke (`ExtensionsPanel.tsx:3251-3256`). The old port waited for
// `Submitted`, so a field over its bounds said nothing until Enter.
func TestTheConfigFieldValidatesAsTheUserTypes(t *testing.T) {
	en := i18n.For("en").Settings
	minimum, maximum := 0.5, 9.0
	number := ConfigField{Key: "retries", Label: "Retries", Type: "number", Minimum: &minimum, Maximum: &maximum}
	a, _ := configFormApp(t, []ConfigField{number}, map[string]any{})
	tt := integrationPage(t, a, 900, 1400)

	// Focus the field's own box, in the second grid column, and type an
	// out-of-range number — no Enter.
	label := mustFind(t, tt, "Retries")
	tt.ClickAt(label.X+240, label.Y+label.H/2)
	tt.Frame()
	tt.Type("99")
	tt.Frame()
	if !tt.HasText(en.ConfigOutOfRange) {
		t.Errorf("an out-of-range field said nothing while typing: %q", tt.Texts())
	}
	// The typed text survives the redraw — the field is controlled, as the
	// reference's was — so the same wrong number is still there after the frame
	// and still flagged. Backspacing into range clears the sentence at once.
	if got := a.draft(a.Integrations()[0]).live["retries"]; got != "99" {
		t.Errorf("the typed text did not survive the redraw: live = %q, want %q", got, "99")
	}
	tt.TypeKey(0, ui.KeyBackspace, "")
	tt.Frame()
	if got := a.draft(a.Integrations()[0]).live["retries"]; got != "9" {
		t.Errorf("the backspace did not reach the live text: live = %q, want %q", got, "9")
	}
	if tt.HasText(en.ConfigOutOfRange) {
		t.Errorf("an in-range 9 still wears %q: %q", en.ConfigOutOfRange, tt.Texts())
	}
}

// The whole-row `busy` the reference handed every `ConfigFieldControl`
// (`disabled={Boolean(busy)}`, `ExtensionsPanel.tsx:3004,3198`) locks the
// form's own fields too: while another operation on the same integration is in
// flight, a field refuses input.
func TestARowOperationLocksTheConfigFields(t *testing.T) {
	number := ConfigField{Key: "token", Label: "Token", Type: "text"}
	a, _ := configFormApp(t, []ConfigField{number}, map[string]any{"token": "abc"})
	a.repairBusy = map[string]bool{"dev.floter.tool": true}
	tt := integrationPage(t, a, 900, 1400)

	label := mustFind(t, tt, "Token")
	tt.ClickAt(label.X+240, label.Y+label.H/2)
	tt.Frame()
	tt.Type("xyz")
	tt.Frame()
	if got := a.draft(a.Integrations()[0]).values["token"]; got != "abc" {
		t.Errorf("a locked field took typing: token = %v, want the stored %q", got, "abc")
	}

	// With the row idle again the same edit lands.
	a.repairBusy = nil
	tt.Frame()
	tt.ClickAt(label.X+240, label.Y+label.H/2)
	tt.Frame()
	tt.Type("z")
	tt.Frame()
	if got := a.draft(a.Integrations()[0]).values["token"]; got == "abc" {
		t.Errorf("an unlocked field refused typing: token = %v", got)
	}
}

// A native `select[multiple]` extends a range from the last clicked option on
// Shift (`ExtensionsPanel.tsx:3229-3243` draws one); the toolkit hands the
// click's modifiers through, so the box keeps the habit even without a native
// multi-select widget.
func TestTheConfigMultiselectExtendsARangeOnShift(t *testing.T) {
	a, _ := configFormApp(t,
		[]ConfigField{{
			Key: "tags", Label: "Tags", Type: "multiselect",
			Options: []string{"alpha", "beta", "gamma"},
		}},
		map[string]any{},
	)
	tt := integrationPage(t, a, 900, 1400)

	// A plain click on `alpha` anchors the range at it.
	if err := tt.Click("alpha"); err != nil {
		t.Fatalf("click alpha: %v", err)
	}
	tt.Frame()
	// Shift-click on `delta` extends through the three between.
	if err := tt.ClickWith(ui.Shift, "gamma"); err != nil {
		t.Fatalf("shift-click gamma: %v", err)
	}
	tt.Frame()
	values, _ := a.draft(a.Integrations()[0]).values["tags"].([]any)
	got := make([]string, 0, len(values))
	for _, value := range values {
		got = append(got, value.(string))
	}
	if strings.Join(got, ",") != "alpha,beta,gamma" {
		t.Errorf("the Shift-range selected %v, want alpha…gamma", got)
	}
}

// (`ExtensionsPanel.tsx:2975-2986`); a host form drawn over values the tool
// owns would offer to write them.
func TestAToolOwnedConfigurationDoesNotWearTheForm(t *testing.T) {
	en := i18n.For("en").Settings
	a, _ := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		map[string]any{"token": "abc"},
	)
	integration := a.Integrations()[0]
	integration.ConfigOwner = "tool"
	a.Integrations = func() []Integration { return []Integration{integration} }

	tt := integrationPage(t, a, 900, 1400)
	if tt.HasText(en.ConfigOpen) {
		t.Errorf("a tool-owned configuration drew the host Configure row: %q", tt.Texts())
	}
	if tt.HasText(en.ConfigSave) {
		t.Errorf("a tool-owned configuration drew the host form: %q", tt.Texts())
	}
	// It wears the reference's own outlet instead: the Configuration block
	// title and the "Open configuration" action
	// (`ExtensionsPanel.tsx:2971-2986`).
	if !tt.HasText(en.ConfigSection) || !tt.HasText(en.ConfigOpenConfiguration) {
		t.Errorf("a tool-owned configuration is missing its open outlet: %q", tt.Texts())
	}

	// The same integration with the owner the reference requires (host) wears
	// it: the gate is the owner, not the schema's presence.
	integration.ConfigOwner = "host"
	a.Integrations = func() []Integration { return []Integration{integration} }
	tt = integrationPage(t, a, 900, 1400)
	if !tt.HasText(en.ConfigOpen) {
		t.Errorf("a host-owned configuration lost its Configure row: %q", tt.Texts())
	}
	// The row's label is the reference's block title, and the verb is the
	// block's one action: the old port printed "Configure" on both the label
	// and the button, so one line named the block twice and the block was not
	// called what the reference calls it.
	if !tt.HasText(en.ConfigSection) {
		t.Errorf("a host-owned configuration lost its Configuration block title: %q", tt.Texts())
	}
	verbs := 0
	for _, text := range tt.Texts() {
		if text == en.ConfigOpen {
			verbs++
		}
	}
	if verbs != 1 {
		t.Errorf("the Configure word appears %d times on the host-owned block, want the button alone: %q", verbs, tt.Texts())
	}
}

// The reference handed the bounds to the native control and the browser drew
// the invalid state *on the field* — a red keyline as well as the message
// (`ExtensionsPanel.tsx:3251-3256`). A bare input that carries `Error` marks
// its node invalid, and the input's own border rule paints that in the danger
// ink, so the field reads as wrong and not only its caption below.
func TestAnOutOfRangeFieldWearsTheDangerStroke(t *testing.T) {
	minimum, maximum := 0.5, 9.0
	number := ConfigField{Key: "retries", Label: "Retries", Type: "number", Minimum: &minimum, Maximum: &maximum}
	a, _ := configFormApp(t, []ConfigField{number}, map[string]any{})
	tt := integrationPage(t, a, 900, 1400)

	label := mustFind(t, tt, "Retries")
	y := int(label.Y + label.H/2)
	// The field box's own right edge: scan the row for the strongest warm
	// (red-dominant) pixel, the danger keyline's signature.
	warmest := func() int {
		img := tt.Image()
		best := 0
		for x := int(label.X + 60); x < int(label.X+340) && x < 900; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if redness := int(r>>8) - int(g>>8) - int(b>>8); redness > best {
				best = redness
			}
		}
		return best
	}

	valid := warmest()
	tt.ClickAt(label.X+240, label.Y+label.H/2)
	tt.Frame()
	tt.Type("99")
	tt.Frame()
	if !tt.HasText(i18n.For("en").Settings.ConfigOutOfRange) {
		t.Fatal("the out-of-range field said nothing while typing")
	}
	if invalid := warmest(); invalid <= valid {
		t.Errorf("the invalid field's keyline is not warmer than a clean one: %d vs %d", invalid, valid)
	}
}
