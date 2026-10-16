package settingsui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The configuration form: the settings an integration asks the host to hold,
// rendered from the schema its provider declares.
//
// The form is folded away until the user asks for it, keeps its draft while it
// is open (so a half-filled form survives a redraw), and writes through the
// shell on Save — which is where the values are validated against the schema
// and the password fields are moved into the secrets file.

// configDraft is one integration's form state.
type configDraft struct {
	values  map[string]any
	enabled []string // the multiSelect options that are ticked
	// saved is the values the form last wrote (or read), the baseline the
	// dirty check compares against. The shell refreshes the integration list
	// off the main thread, so until that lands `integration.ConfigValues` is
	// still the *previous* generation; keeping the baseline beside the draft
	// is what lets a Save flip the status to "Saved" at once, exactly as the
	// reference's `setSavedConfigValues(saved.values)` did
	// (`ExtensionsPanel.tsx:2001`).
	saved map[string]any
}

// draft is the form state for an integration, seeded from its loaded values
// the first time it is opened.
func (a *App) draft(integration Integration) *configDraft {
	if a.configDrafts == nil {
		a.configDrafts = map[string]*configDraft{}
	}
	draft, ok := a.configDrafts[integration.ID]
	if !ok {
		values := map[string]any{}
		for _, field := range integration.Config {
			if value, ok := integration.ConfigValues[field.Key]; ok {
				values[field.Key] = value
				continue
			}
			// An unfilled field shows its default, which is what a save
			// would write for it.
			if field.Default != nil {
				values[field.Key] = field.Default
			}
		}
		draft = &configDraft{values: values, saved: cloneConfigValues(values)}
		a.configDrafts[integration.ID] = draft
	} else if draft.saved == nil {
		// A draft opened before this baseline existed: seed it from the
		// values it was created with, so the dirty check has a baseline.
		draft.saved = cloneConfigValues(draft.values)
	}
	return draft
}

// cloneConfigValues is a shallow copy of a value map: the values are JSON
// scalars and lists the draft never mutates in place (a field's value is
// replaced whole), so a shallow copy is the baseline the dirty check needs
// without sharing the map the form edits.
func cloneConfigValues(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

// configFields draws the form for one integration's configuration: the
// fields and the save button, shown while the form is open. The toggle that
// opens it is the integration row's own (see integrationRow).
func (a *App) configFields(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := a.cardTheme(c)
	if len(integration.Config) == 0 {
		return
	}
	if a.configError == nil {
		a.configError = map[string]string{}
	}
	if !a.configOpen[integration.ID] {
		return
	}
	draft := a.draft(integration)
	// The form is a `below` block of the ConfigOpen row, drawn on the card's
	// content column: it carries the row's own inset so the form's card starts
	// on the same column as the row labels above it instead of flush against
	// the card's edge (the reference's `.extension-config-form`,
	// `extensions.css:2369-2373`, is inset the same way).
	inset := a.rowInset(t)
	ui.Column(c).FillWidth().Gap(a.cardSpace(t, 3)).Padding(0, inset, 0, inset).Children(func() {
		// The reference's `.extension-config-form` is one `display: grid` with a
		// 12px gap (`extensions.css:2369-2373`, literal px) holding the toolbar,
		// the fields and the action row as *its own* children, in order — no
		// plane and no rules between them. The old port wrapped the fields in a
		// second filled card, so the form read as a bordered box inside the
		// integration card rather than the reference's flat grid.
		a.configToolbar(c, copy, integration, draft)
		for _, field := range integration.Config {
			a.configField(c, copy, integration, *draft, field)
		}
		if a.configError[integration.ID] != "" {
			ui.Text(c, a.configError[integration.ID]).FontSize(t.FontSize - 1).TextColor(t.Danger)
		}
		a.configActions(c, copy, integration, draft)
	})
}

// configToolbar draws the reference's `.extension-config-toolbar`
// (`extensions.css:2374-2376`, `ExtensionsPanel.tsx:2989-3000`): the status
// word at its start and the two icon-buttons at its end. The status reads
// "Saved" while the draft matches what is stored and "Unsaved changes" — in
// the warning ink, `.extension-config-status--dirty` — while it does not, so
// the user sees whether a save is pending without pressing one. The copy and
// export icon-buttons sit behind it, disabled while a write is in flight.
func (a *App) configToolbar(c *ui.Context, copy i18n.Settings, integration Integration, draft *configDraft) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	dirty := a.configDirty(integration, draft)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		status := copy.ConfigSavedState
		ink := tokens.TextTertiary
		if dirty {
			status, ink = copy.ConfigUnsaved, t.Warning
		}
		ui.Text(c, status).FontSize(tokens.Caption).FontWeight(500).TextColor(ink)
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(0.5)).Children(func() {
			copyDisabled := a.Actions.CopyConfiguration == nil
			copyButton := a.extensionIconButton(c, "copy", copy.ConfigCopy, copyDisabled)
			if copyButton.Clicked() && !copyDisabled {
				a.Actions.CopyConfiguration(integration.ID, draft.values)
			}
			exportDisabled := a.Actions.ExportConfiguration == nil
			exportButton := a.extensionIconButton(c, "file-down", copy.ConfigExport, exportDisabled)
			if exportButton.Clicked() && !exportDisabled {
				a.Actions.ExportConfiguration(integration.ID, draft.values)
			}
		})
	})
}

// configActions draws the reference's `.extension-config-form__actions`
// (`extensions.css:2375`, `ExtensionsPanel.tsx:3015-3022`): a right-aligned
// row whose start is the neutral "Restore defaults" pill (the `RotateCcw`
// mark then the word) and whose end is the primary accent Save. It is
// `justify-content: space-between`, so the two ends answer to the row's own
// edges rather than to each other.
func (a *App) configActions(c *ui.Context, copy i18n.Settings, integration Integration, draft *configDraft) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		defaults := a.extensionGlyphActionButton(c, "rotate-ccw", copy.ConfigDefaults,
			t.Space(7.5), t.Space(3.75), tokens.Body)
		if defaults.Clicked() {
			// The reference's `configurationDefaults` writes every field whose
			// schema declared a default back to it, leaving the others as they
			// are (`ExtensionsPanel.tsx:2013-2016`).
			for _, field := range integration.Config {
				if field.Default != nil {
					draft.values[field.Key] = field.Default
				}
			}
		}
		// The Save is the *primary* accent pill (`.extensions-action-button--primary`,
		// `ExtensionsPanel.tsx:3019-3021`), disabled while the draft matches what is
		// stored or while there is nobody to write it — the reference's
		// `disabled={!configDirty}`.
		saveDisabled := a.Actions.SaveConfiguration == nil || !a.configDirty(integration, draft)
		save := a.extensionPrimaryButton(c, copy.ConfigSave)
		if saveDisabled {
			save = save.Disabled(true).Opacity(0.4).Cursor(ui.CursorDefault)
		}
		if save.Clicked() && !saveDisabled {
			if err := a.Actions.SaveConfiguration(integration.ID, draft.values); err != nil {
				a.configError[integration.ID] = err.Error()
				return
			}
			// The reference sets the saved values to what it just wrote
			// (`setSavedConfigValues(saved.values)`, `ExtensionsPanel.tsx:2001`),
			// so the form reads "Saved" at once rather than waiting on the
			// shell's off-thread refresh.
			a.configError[integration.ID] = ""
			draft.saved = cloneConfigValues(draft.values)
			// The landed save is good news, so it raises the reference's success
			// notice (`showSuccess("configSaved")`, `ExtensionsPanel.tsx:2002`),
			// never the danger ink the error line wears.
			if a.Actions.NotifySuccess != nil {
				a.Actions.NotifySuccess(copy.ConfigSaved)
			}
		}
	})
}

// configDirty reports whether a draft differs from the values it was last
// read or written with: the reference's `configDirty`, a JSON comparison of
// the edited values against the saved ones (`ExtensionsPanel.tsx:969-970`).
//
// The baseline is the draft's own `saved` copy rather than the integration's
// `ConfigValues`, because a Save re-baselines at once (the reference's
// `setSavedConfigValues(saved.values)`) while the shell's list refresh has
// not landed yet; comparing against the stale list would leave the form
// reading "Unsaved changes" for a save that just succeeded. A field the file
// does not mention but whose schema declares a default shows that default
// (`value={configValues[field.key] ?? field.default}`), so the comparison is
// against the *effective* value — the baseline when it has one, the default
// otherwise.
func (a *App) configDirty(integration Integration, draft *configDraft) bool {
	if draft == nil {
		return false
	}
	for _, field := range integration.Config {
		effective, hasEffective := draft.saved[field.Key]
		if !hasEffective {
			effective, hasEffective = field.Default, field.Default != nil
		}
		current, hasCurrent := draft.values[field.Key]
		if !hasEffective {
			// Nothing saved and no default: the field reads empty until the
			// user writes something, and only then is the form dirty.
			if hasCurrent {
				return true
			}
			continue
		}
		if !hasCurrent || !configValuesEqual(effective, current) {
			return true
		}
	}
	return false
}

// configValuesEqual compares two configuration values by their JSON shape: a
// number arrives as `float64` from both the stored file and the draft, a list
// as `[]any`, and text as `string`, so a structural comparison is what tells
// an edit from a redraw.
func configValuesEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

// configFieldGrid lays one config field on the reference's
// `.extension-config-field` grid (`extensions.css:2379-2430`): a fixed 118px
// label column, the control filling the second column, and — when the field
// has one — the description under the control in that same second column
// (`small { grid-column: 2 }`). The label carries the warning `*` a required
// field wears, as the reference's `<b>` did. The 118px column and the 12px/
// 4px gaps are the panel's literal px, taken through `cardSpace`, so the grid
// keeps one geometry at every interface step.
func (a *App) configFieldGrid(c *ui.Context, field ConfigField, label, requiredLabel string, control func() ui.Element) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	grid := ui.Grid(c).FillWidth().
		ColumnTracks(ui.Fixed(a.cardSpace(t, 29.5)), ui.Fr(1)).
		GapX(a.cardSpace(t, 3)).GapY(a.cardSpace(t, 1)).
		AlignItems(ui.Center)
	grid.Children(func() {
		name := ui.Row(c).Gap(a.cardSpace(t, 0.75)).AlignItems(ui.Center).ColumnStart(1)
		name.Children(func() {
			ui.Text(c, label).FontSize(t.FontSize).FontWeight(500).TextColor(tokens.TextSecondary).SingleLine()
			if field.Required {
				// The reference's `b { margin-left: 3px }` marker is the warning
				// `*` with the word "required" as its accessible name
				// (`aria-label={t("settings.extensions.required")}`,
				// `ExtensionsPanel.tsx:3266`). A native pane has no aria label, so
				// the word rides the tooltip the framework does draw.
				ui.Text(c, "*").FontSize(t.FontSize).FontWeight(600).TextColor(t.Warning).
					SingleLine().Tooltip(requiredLabel)
			}
		})
		control().ColumnStart(2)
		if field.Description != "" {
			ui.Text(c, field.Description).FontSize(tokens.Caption).FontWeight(500).
				TextColor(tokens.TextTertiary).ColumnStart(2)
		}
	})
}

// configSelect is the config field's drop-down, on the grid's second column:
// `.extension-config-field select` shares the input's literal box
// (`extensions.css:2402-2417`) but fills its column rather than taking the
// settings rows' fixed width. It is `chooseSelect` without that 220px.
func (a *App) configSelect(c *ui.Context, selected *string, labels []string, label string) ui.Element {
	e := ui.Select(c, selected, labels).Label(label).FillWidth().Radius(999)
	if a.cardUnit > 0 {
		e = e.MinHeight(28).Padding(4, 8)
	}
	return a.fieldFace(c, e)
}

// configField draws one field, by its type. Every field wears the reference's
// `.extension-config-field` two-column grid (see `configFieldGrid`): a 118px
// label column and the control, with the description under the control. The
// old port drew each field as a settings row (label left, control right, the
// description under the *label*), so a form's controls did not share one
// column and the description sat on the wrong side of the field.
func (a *App) configField(c *ui.Context, copy i18n.Settings, integration Integration, draft configDraft, field ConfigField) {
	label := field.Label
	if label == "" {
		label = field.Key
	}
	kind := strings.ToLower(field.Type)

	switch kind {
	case "boolean":
		on := false
		if value, ok := draft.values[field.Key].(bool); ok {
			on = value
		}
		changed := false
		a.configFieldGrid(c, field, label, copy.ConfigRequired, func() ui.Element {
			e := a.switchControl(c, &on).Label(label)
			if e.Changed() {
				changed = true
			}
			return e
		})
		if changed {
			draft.values[field.Key] = on
		}
	case "select":
		// The reference's option list is the schema's options with a leading
		// empty option for a non-required field (`{!field.required && <option
		// value="">}` with `configNone`, `ExtensionsPanel.tsx:3226-3228`), so a
		// non-required choice can be cleared back to "not set".
		labels := configSelectLabels(field, copy.ConfigNone)
		noneOffered := !field.Required && len(labels) == len(field.Options)+1
		current, _ := draft.values[field.Key].(string)
		selected := current
		if selected == "" && noneOffered {
			selected = copy.ConfigNone
		}
		chosen, cleared := "", false
		a.configFieldGrid(c, field, label, copy.ConfigRequired, func() ui.Element {
			e := a.configSelect(c, &selected, labels, label)
			if e.Changed() {
				if noneOffered && selected == copy.ConfigNone {
					cleared = true
					return e
				}
				chosen = selected
			}
			return e
		})
		if cleared {
			// The reference writes `null` for the empty option
			// (`event.target.value === "" ? null : …`), which the schema reads
			// back as "unset".
			draft.values[field.Key] = nil
		} else if chosen != "" {
			draft.values[field.Key] = chosen
		}
	case "multiselect":
		ticked := map[string]bool{}
		if values, ok := draft.values[field.Key].([]any); ok {
			for _, value := range values {
				if text, ok := value.(string); ok {
					ticked[text] = true
				}
			}
		}
		// The reference draws the field as one native multiple `select` — a
		// single list box (`min-height: 66px; border-radius: var(--radius-sm)`,
		// `.extension-config-field select[multiple]`, `extensions.css:2420-2423`).
		// This toolkit has no multi-select widget, so the field is one *box* of
		// the same shape — the `--glass-field` face, the `--radius-sm` corner
		// and the 66px floor — holding the tick options as rows inside it, rather
		// than the loose ruled rows the old port drew. A list box reads as one
		// control, which is what the reference's `select[multiple]` is.
		a.configFieldGrid(c, field, label, copy.ConfigRequired, func() ui.Element {
			return a.configMultiselectBox(c, func() {
				for _, option := range field.Options {
					option := option
					on := ticked[option]
					boxChanged := false
					labelClicked := a.configOptionRow(c, option, &on, &boxChanged)
					if labelClicked {
						on = !on
						boxChanged = true
					}
					if boxChanged {
						ticked[option] = on
						draft.values[field.Key] = tickedToValues(field.Options, ticked)
					}
				}
			})
		})
	case "number":
		value := ""
		if number, ok := draft.values[field.Key].(float64); ok {
			value = strconv.FormatFloat(number, 'f', -1, 64)
		}
		edit := value
		a.configFieldGrid(c, field, label, copy.ConfigRequired, func() ui.Element {
			e := a.extensionField(c, &edit, configFieldMetrics).Placeholder(field.Key).Label(label).FillWidth()
			if e.Submitted() {
				number, err := strconv.ParseFloat(strings.TrimSpace(edit), 64)
				switch {
				case err != nil:
					a.configError[integration.ID] = fmt.Sprintf("%s: %s", label, copy.ConfigNumber)
				case field.Minimum != nil && number < *field.Minimum || field.Maximum != nil && number > *field.Maximum:
					a.configError[integration.ID] = fmt.Sprintf("%s: %s", label, copy.ConfigOutOfRange)
				default:
					a.configError[integration.ID] = ""
					draft.values[field.Key] = number
				}
			}
			return e
		})
	default: // text, password, path
		value, _ := draft.values[field.Key].(string)
		edit := value
		a.configFieldGrid(c, field, label, copy.ConfigRequired, func() ui.Element {
			e := a.extensionField(c, &edit, configFieldMetrics).Placeholder(field.Key).Label(label).FillWidth()
			if kind == "password" {
				e = e.Password()
			}
			if e.Submitted() {
				length := len([]rune(edit))
				if (field.MinLength != nil && length < *field.MinLength) ||
					(field.MaxLength != nil && length > *field.MaxLength) {
					a.configError[integration.ID] = fmt.Sprintf("%s: %s", label, copy.ConfigOutOfLength)
				} else {
					a.configError[integration.ID] = ""
					draft.values[field.Key] = edit
				}
			}
			return e
		})
	}
}

// configSelectLabels builds one select field's option list: the schema's own
// options, with the reference's `configNone` word prepended for a
// non-required field (`{!field.required && <option value="">}`,
// `ExtensionsPanel.tsx:3226-3228`). A required field has no empty option —
// clearing it is not a legal value. It is a named helper rather than inline
// so the rule is one place a test can read.
//
// An option whose own text already is the `none` word drops the sentinel: the
// reference tells the two apart by the DOM value (`""` against the option's
// own text), which a label-only widget cannot. Rather than misread a real
// option as "clear", the empty option is simply not offered and the label
// stands for its own value.
func configSelectLabels(field ConfigField, none string) []string {
	labels := make([]string, 0, len(field.Options)+1)
	if !field.Required && !configOptionMatches(field.Options, none) {
		labels = append(labels, none)
	}
	labels = append(labels, field.Options...)
	return labels
}

// configOptionMatches reports whether any of the field's options already is
// the given word, so the empty option would collide with a real one.
func configOptionMatches(options []string, word string) bool {
	for _, option := range options {
		if option == word {
			return true
		}
	}
	return false
}

// configMultiselectBox draws the multiselect field's one list box: the
// reference's `.extension-config-field select[multiple]` shape — a
// `--glass-field` face under its inset slot, a `--radius-sm` corner and the
// reference's 66px floor (`extensions.css:2420-2423`) — with fn filling the
// tick rows inside it. It is one element, not a column of rows, so the field
// reads as the single native control the reference drew.
func (a *App) configMultiselectBox(c *ui.Context, fn func()) ui.Element {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	box := ui.Column(c).FillWidth().MinHeight(a.cardSpace(t, 16.5)).
		Padding(a.cardSpace(t, 1), a.cardSpace(t, 2)).
		Gap(a.cardSpace(t, 0.5)).
		Border(1, tokens.ControlEdge).
		Radius(tokens.RadiusSM).
		Background(tokens.Field)
	box = tokens.FieldSlot(box)
	box.Children(fn)
	return box
}

// configOptionRow draws one tick option inside the multiselect list box: the
// option's word at the start and the check box at its end, in one compact
// row, so the box reads as the option list it stands in for. It reports
// whether the option's own word was clicked (the label is a hit target, as
// the framework's own Field makes a control's label).
func (a *App) configOptionRow(c *ui.Context, option string, on *bool, changed *bool) bool {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	clicked := false
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Gap(a.cardSpace(t, 2)).MinHeight(a.cardSpace(t, 5)).Children(func() {
		word := ui.Text(c, option).FontSize(tokens.Body).FontWeight(500).TextColor(tokens.TextSecondary)
		if word.Clicked() {
			clicked = true
		}
		if ui.CheckboxBase(c, on).Label(option).Clicked() {
			*changed = true
		}
	})
	return clicked
}

// tickedToValues is a multiselect's ticked options as the value a save writes,
// in the options' order.
func tickedToValues(options []string, ticked map[string]bool) []any {
	var out []any
	for _, option := range options {
		if ticked[option] {
			out = append(out, option)
		}
	}
	return out
}
