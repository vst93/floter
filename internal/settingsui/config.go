package settingsui

import (
	"encoding/json"
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
	values map[string]any
	// live is each text or number field's displayed text while it is being
	// edited, keyed by the field's key. The reference's own inputs were
	// *controlled* by the form state — `value={displayJson(value ?? "")}` with
	// `onChange` writing straight back (`ExtensionsPanel.tsx:3218-3258`) — so a
	// half-typed value that the schema bounds have not accepted yet was still
	// what the field showed, and the browser drew its validity state on it. A
	// native pane keeps the text in this map so it survives the redraw the same
	// way, while `values` holds what a save would carry.
	live map[string]string
	// multiAnchor is each multiselect list box's last clicked option index,
	// keyed by the field. A native `select[multiple]` extends a range from that
	// anchor on Shift-click (`Ctrl`/`Cmd`-click toggles one, a plain click
	// starts over), and the toolkit hands the click's modifiers through, so a
	// range is the one selection habit this box can keep.
	multiAnchor map[string]int
	// saved is the values the form last wrote (or read), the baseline the
	// dirty check compares against. The shell refreshes the integration list
	// off the main thread, so until that lands `integration.ConfigValues` is
	// still the *previous* generation; keeping the baseline beside the draft
	// is what lets a Save flip the status to "Saved" at once, exactly as the
	// reference's `setSavedConfigValues(saved.values)` did
	// (`ExtensionsPanel.tsx:2001`).
	saved map[string]any
}

// liveText is one field's displayed text: what the user has typed, or the
// fallback the field's stored value renders as.
func (d *configDraft) liveText(field ConfigField, fallback string) string {
	if d.live != nil {
		if text, ok := d.live[field.Key]; ok {
			return text
		}
	}
	return fallback
}

// setLive records one field's displayed text, so a half-typed value survives
// the redraw.
func (d *configDraft) setLive(field ConfigField, text string) {
	if d.live == nil {
		d.live = map[string]string{}
	}
	d.live[field.Key] = text
}

// clearLive drops every field's live text, so the fields fall back to their
// `values`: what Restore defaults and a landed Save need, since both move the
// values behind the fields' backs.
func (d *configDraft) clearLive() { d.live = nil }

// multiAnchorAt is one multiselect field's last clicked option index, or -1
// when it has none yet.
func (d *configDraft) multiAnchorAt(field ConfigField) int {
	if d.multiAnchor == nil {
		return -1
	}
	index, ok := d.multiAnchor[field.Key]
	if !ok {
		return -1
	}
	return index
}

// setMultiAnchor records the option a range extends from.
func (d *configDraft) setMultiAnchor(field ConfigField, index int) {
	if d.multiAnchor == nil {
		d.multiAnchor = map[string]int{}
	}
	d.multiAnchor[field.Key] = index
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

// configLocked reports whether the form's fields must refuse input: the
// reference passed one whole-row `busy` to every `ConfigFieldControl`
// (`disabled={Boolean(busy)}`, `ExtensionsPanel.tsx:3004,3198`), and that
// `busy` is held for *any* mutation of the row — the re-check, the toggle,
// the uninstall — not only for the form's own save. So a field, a switch and
// a select are locked while another operation on the same integration is in
// flight, exactly as the row's action cluster is (see `rowBusy`).
func (a *App) configLocked(id string) bool { return a.rowBusy(id) }

// lockedControl puts the **one** fade on a form control that cannot be touched
// (`docs/rounds/P13-t-spec.md`). It is the sibling of `theme.Tokens.DimFace`:
// there the material says "there but unlit", here the *state* says "there but
// not now", and both mean **one fade and no second one**.
//
// The framework's painter already multiplies a `Disabled` subtree by 0.5
// (`ui/paint.go:257-259`), so this helper must not add an `Opacity`: the old
// arms multiplied that 0.5 by their own 0.6 and drew locked controls at an
// effective 0.3 — a locked field's face stepped off the ground by 5 levels
// instead of 9, and its E1 pair vanished (measured 230 → 242 instead of 238).
// The multiselect box went the other way: it carried the 0.6 and *no* flag, so
// the box and its option words sat at 0.6 while the tick boxes inside them sat
// at 0.25 — three depths for one word, on one form.
//
// One flag at the control's root is what makes the whole subtree land on one
// rung. Wheel scrolling is unaffected (`ui/input.go:515-536` scrolls a
// disabled subtree), so a locked list of long options can still be read.
//
// P13-u：它不只服务配置表单——设置页自己的禁用控件（会话小格、确认带胶囊）也用
// 同一条算术；区别只在材质：**面**再叠 `DimFace`（熄灯），**槽**与**透明面**
// 什么都不叠（没有受光边可熄）。
func lockedControl(e ui.Element) ui.Element {
	return e.Disabled(true).Cursor(ui.CursorDefault)
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
	// The form is a `below` block of the ConfigOpen row; `cardRow` puts every
	// such block on the row's content column (P11-bf), so the form carries no
	// inset of its own — one place answers where that column starts.
	ui.Column(c).FillWidth().Gap(a.cardSpace(t, 3)).Children(func() {
		// The reference's `.extension-config-form` is one `display: grid` with a
		// 12px gap (`extensions.css:2369-2373`, literal px) holding the toolbar,
		// the fields and the action row as *its own* children, in order — no
		// plane and no rules between them. The old port wrapped the fields in a
		// second filled card, so the form read as a bordered box inside the
		// integration card rather than the reference's flat grid.
		a.configToolbar(c, copy, integration, draft)
		for _, field := range integration.Config {
			a.configField(c, copy, integration, draft, field)
		}
		if a.configError[integration.ID] != "" {
			// The failure the shell reported on the Save is the reference's own
			// `.extensions-notice--error` sentence under the fields
			// (`ExtensionsPanel.tsx:2674`), not a bare red line.
			a.noticeLine(c, a.configError[integration.ID])
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
// export icon-buttons sit behind it, disabled while either is in flight: the
// reference's one `configOperation` item disables both
// (`disabled={Boolean(configOperation)}`, `ExtensionsPanel.tsx:2994,2997`).
func (a *App) configToolbar(c *ui.Context, copy i18n.Settings, integration Integration, draft *configDraft) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	dirty := a.configDirty(integration, draft)
	busy := a.configOperations[integration.ID] != ""
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		status := copy.ConfigSavedState
		ink := tokens.TextTertiary
		if dirty {
			status, ink = copy.ConfigUnsaved, t.Warning
		}
		ui.Text(c, status).FontSize(tokens.Caption).FontWeight(500).TextColor(ink)
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(0.5)).Children(func() {
			copyDisabled := a.Actions.CopyConfiguration == nil || busy
			copyButton := a.extensionIconButton(c, "copy", copy.ConfigCopy, copyDisabled)
			if copyButton.Clicked() && !copyDisabled {
				a.startConfigOperation(integration.ID, "copy", func(done func(error)) {
					a.Actions.CopyConfiguration(integration.ID, draft.values, done)
				})
			}
			exportDisabled := a.Actions.ExportConfiguration == nil || busy
			exportButton := a.extensionIconButton(c, "file-down", copy.ConfigExport, exportDisabled)
			if exportButton.Clicked() && !exportDisabled {
				a.startConfigOperation(integration.ID, "export", func(done func(error)) {
					a.Actions.ExportConfiguration(integration.ID, draft.values, done)
				})
			}
		})
	})
}

// startConfigOperation marks one integration's copy or export as in flight and
// hands the host the callback that settles it, on the main thread. The
// reference claimed its one `configOperation` slot before starting it and kept
// it until `finally` (`ExtensionsPanel.tsx:2019-2048`), which is what holds the
// two icon-buttons disabled for as long as the work runs.
func (a *App) startConfigOperation(id, operation string, run func(done func(error))) {
	if a.configOperations == nil {
		a.configOperations = map[string]string{}
	}
	a.configOperations[id] = operation
	run(func(error) { a.finishConfigOperation(id) })
}

// finishConfigOperation releases the slot and asks for a frame, so the
// buttons' disabled face is gone with it, exactly as the reference's `finally`
// did.
func (a *App) finishConfigOperation(id string) {
	if a.configOperations != nil {
		delete(a.configOperations, id)
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// configOperationNow reports whether an integration has a copy or export in
// flight, for the shell's own hand-off and the tests.
func (a *App) configOperationNow(id string) string { return a.configOperations[id] }

// noticeLine draws one `.extensions-notice` sentence on the form's own column:
// the framework's warning mark then the words, the pane and the warning keyline
// the reference's `.extensions-notice--error` wears (`extensions.css:146-178`).
func (a *App) noticeLine(c *ui.Context, sentence string) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	band := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).
		Padding(t.Space(2), t.Space(3)).Radius(tokens.RadiusSM).
		Background(tokens.Control).Border(1, tokens.ControlEdge)
	band.InsetShadow(0, 0, 0, 1, t.Warning)
	band.Children(func() {
		ui.Icon(c, mustGlyph("alert")).Size(t.Space(3.75), t.Space(3.75)).TextColor(t.Warning)
		ui.Text(c, sentence).FontSize(tokens.Body).TextColor(t.Warning).Grow(1).LineHeight(1.45)
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
		// Both pills are disabled while an operation is in flight
		// (`disabled={Boolean(busy) || Boolean(configOperation)}` on the
		// Defaults, `disabled={Boolean(busy) || !configDirty}` on the Save),
		// where `busy` is one per-integration item held for every mutation of
		// that row (`ExtensionsPanel.tsx:3016,3019`).
		busy := a.configLocked(integration.ID) || a.configOperations[integration.ID] != "" || a.configSaveBusy[integration.ID]
		// P13-s（§6.2b-6）：`busy` 不再用组透明度：它只传给面，面走 `ControlDim`
		// 并熄灯，字自己淡到 40%。
		defaults := a.extensionGlyphActionButton(c, "rotate-ccw", copy.ConfigDefaults,
			t.Space(7.5), t.Space(3.75), tokens.Body, busy)
		if busy {
			defaults = defaults.Disabled(true).Cursor(ui.CursorDefault)
		}
		if defaults.Clicked() && !busy {
			// The reference's `configurationDefaults` writes every field whose
			// schema declared a default back to it, leaving the others as they
			// are (`ExtensionsPanel.tsx:2013-2016`). A field mid-edit shows its
			// live text, so the reset also drops that text: otherwise the new
			// default would sit behind a stale half-typed value.
			draft.clearLive()
			for _, field := range integration.Config {
				if field.Default != nil {
					draft.values[field.Key] = field.Default
				}
			}
		}
		// The Save is the *primary* accent pill (`.extensions-action-button--primary`,
		// `ExtensionsPanel.tsx:3019-3021`), disabled while the draft matches what is
		// stored or while there is nobody to write it — the reference's
		// `disabled={!configDirty}`. While the write is out its label is the
		// reference's `configSaving` word rather than `save`
		// (`busy?.kind === "save" ? t("settings.extensions.saving") :
		// t("settings.extensions.save")`, `ExtensionsPanel.tsx:3020`).
		saveDisabled := a.Actions.SaveConfiguration == nil || !a.configDirty(integration, draft) || busy
		saveLabel := copy.ConfigSave
		if a.configSaveBusy[integration.ID] {
			saveLabel = copy.ConfigSaving
		}
		// P13-aq：Save 的 disabled 现在由 `extensionPrimaryButton` 自己给（它同时负责
		// 熄灯）；这里不再叠一道 `.Opacity(0.4)`——那是框架 0.5 之外的第二次淡。
		save := a.extensionPrimaryButton(c, saveLabel, saveDisabled)
		if save.Clicked() && !saveDisabled {
			if a.configSaveBusy == nil {
				a.configSaveBusy = map[string]bool{}
			}
			a.configSaveBusy[integration.ID] = true
			a.Actions.SaveConfiguration(integration.ID, draft.values, func(err error) {
				a.finishConfigSave(integration.ID, err, copy)
			})
		}
	})
}

// finishConfigSave settles the form after a save reports back, on the main
// thread: the in-flight word gives way, the failure — if any — goes on the
// form's notice line, and a landed write re-baselines the draft and raises the
// success notice, exactly as the reference's `saveConfiguration` did
// (`setSavedConfigValues(saved.values)` then `showSuccess("configSaved")`,
// `ExtensionsPanel.tsx:1993-2007`).
func (a *App) finishConfigSave(id string, err error, copy i18n.Settings) {
	if a.configSaveBusy != nil {
		delete(a.configSaveBusy, id)
	}
	if err != nil {
		a.configError[id] = err.Error()
	} else {
		// The reference sets the saved values to what it just wrote, so the form
		// reads "Saved" at once rather than waiting on the shell's off-thread
		// refresh.
		a.configError[id] = ""
		if draft, ok := a.configDrafts[id]; ok {
			draft.saved = cloneConfigValues(draft.values)
			// The reference set the displayed values to what it just wrote
			// (`setConfigValues(saved.values)`), so a field mid-edit falls back
			// to the saved value rather than keeping a live text that now equals
			// it anyway.
			draft.clearLive()
		}
		// The landed save is good news, so it raises the reference's success
		// notice, never the danger ink the error line wears.
		if a.Actions.NotifySuccess != nil {
			a.Actions.NotifySuccess(copy.ConfigSaved)
		}
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
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
func (a *App) configFieldGrid(c *ui.Context, field ConfigField, label, requiredLabel, hint string, control func() ui.Element) {
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
		// The schema's own bounds, said *at the field*: the reference handed
		// `min`/`max`/`minLength`/`maxLength` to the native control
		// (`ExtensionsPanel.tsx:3251-3256`) and let the browser draw its validity
		// state on the field itself. A native pane has no such state, so the
		// sentence goes on the field's own caption line — the same column as the
		// description — rather than only at the bottom of the form.
		if hint != "" {
			ui.Text(c, hint).FontSize(tokens.Caption).FontWeight(500).
				TextColor(t.Warning).ColumnStart(2)
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
func (a *App) configField(c *ui.Context, copy i18n.Settings, integration Integration, draft *configDraft, field ConfigField) {
	label := field.Label
	if label == "" {
		label = field.Key
	}
	kind := strings.ToLower(field.Type)
	locked := a.configLocked(integration.ID)

	switch kind {
	case "boolean":
		on := false
		if value, ok := draft.values[field.Key].(bool); ok {
			on = value
		}
		changed := false
		a.configFieldGrid(c, field, label, copy.ConfigRequired, "", func() ui.Element {
			e := a.switchControl(c, &on).Label(label)
			if locked {
				// P13-t（`docs/rounds/P13-t-spec.md`）：一道淡，不是两道——见
				// `lockedControl`。旧的两道把开关轨道的台阶从 −84 压成 −52。
				e = lockedControl(e)
			}
			if e.Changed() && !locked {
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
		a.configFieldGrid(c, field, label, copy.ConfigRequired, "", func() ui.Element {
			e := a.configSelect(c, &selected, labels, label)
			if locked {
				// P13-t：一道淡（框架的 0.5），与同一张表单的字段、开关同档。
				e = lockedControl(e)
			}
			if e.Changed() && !locked {
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
		a.configFieldGrid(c, field, label, copy.ConfigRequired, "", func() ui.Element {
			box := a.configMultiselectBox(c, a.configListScroll(integration.ID, field.Key), locked, func() {
				for index, option := range field.Options {
					index, option := index, option
					on := ticked[option]
					boxChanged := false
					labelClicked, mods := a.configOptionRow(c, option, &on, &boxChanged, locked)
					if labelClicked {
						on = !on
						boxChanged = true
					}
					if !boxChanged || locked {
						continue
					}
					// A native `select[multiple]` extends a range from the last
					// clicked option on Shift, toggles one on Ctrl/Cmd, and starts
					// over on a plain click; the box keeps that habit (the toolkit
					// hands the click's modifiers through). A range sets each
					// option between anchor and target, as the browser does.
					if mods&ui.Shift != 0 && draft.multiAnchorAt(field) >= 0 {
						from, to := draft.multiAnchorAt(field), index
						if from > to {
							from, to = to, from
						}
						for i := from; i <= to && i < len(field.Options); i++ {
							ticked[field.Options[i]] = true
						}
					} else {
						ticked[option] = on
					}
					draft.setMultiAnchor(field, index)
					draft.values[field.Key] = tickedToValues(field.Options, ticked)
				}
			})
			return box
		})
	case "number":
		value := ""
		if number, ok := draft.values[field.Key].(float64); ok {
			value = strconv.FormatFloat(number, 'f', -1, 64)
		}
		edit := draft.liveText(field, value)
		problem := a.configFieldProblem(integration, field)
		a.configFieldGrid(c, field, label, copy.ConfigRequired, problem, func() ui.Element {
			e := a.extensionField(c, &edit, configFieldMetrics).Placeholder(field.Key).Label(label).FillWidth()
			if problem != "" {
				// The reference handed the bounds to the native control and let
				// the browser draw its validity state *on the field* — a red
				// keyline (`:invalid`) as well as the message. `Error` carries
				// the same stroke on a bare input: it marks the node invalid,
				// which the input's own border rule paints in the danger ink
				// (the sentence itself is the grid's caption line above).
				e = e.Error(problem)
			}
			if locked {
				// P13-t：一道淡。0.3 下这块字段的 E1 一对面双双归零（顶内线 = 面
				// = 242、底内线 = 面），槽读不出来；0.5 下仍是 238（−9）与 −7 的对侧。
				e = lockedControl(e)
			}
			// The reference's input is controlled: every keystroke writes the
			// field's own value back, so the bounds are checked as the user types
			// and the sentence at the field appears at once — that is the
			// browser's `:invalid` state the reference relied on
			// (`ExtensionsPanel.tsx:3251-3256`). The typed value lands in the
			// draft regardless (a number that is empty or unparsable becomes
			// `null`, exactly as the browser's `valueAsNumber` did), so the form
			// reads "Unsaved changes" and a Save carries what is on screen; a
			// value the schema refuses is the *host's* save-time refusal, which
			// the form's own error line reports.
			if e.Changed() && !locked {
				draft.setLive(field, edit)
				// The draft keeps the raw `valueAsNumber` the browser would have
				// stored, so the host's own save-time validation is what refuses
				// an out-of-range number.
				text := strings.TrimSpace(edit)
				if number, err := strconv.ParseFloat(text, 64); err == nil && text != "" {
					draft.values[field.Key] = number
				} else {
					draft.values[field.Key] = nil
				}
				if _, problem := ConfigValueProblem(field, edit, copy); problem != "" {
					a.noteConfigFieldError(integration.ID, field.Key, problem)
				} else {
					a.clearConfigFieldError(integration.ID, field.Key)
				}
			}
			if e.Submitted() && !locked {
				value, problem := ConfigValueProblem(field, edit, copy)
				if problem == "" {
					draft.values[field.Key] = value
					a.clearConfigFieldError(integration.ID, field.Key)
				} else {
					a.noteConfigFieldError(integration.ID, field.Key, problem)
				}
			}
			return e
		})
	default: // text, password, path
		value, _ := draft.values[field.Key].(string)
		edit := draft.liveText(field, value)
		problem := a.configFieldProblem(integration, field)
		a.configFieldGrid(c, field, label, copy.ConfigRequired, problem, func() ui.Element {
			e := a.extensionField(c, &edit, configFieldMetrics).Placeholder(field.Key).Label(label).FillWidth()
			if kind == "password" {
				e = e.Password()
			}
			if problem != "" {
				e = e.Error(problem)
			}
			if locked {
				// P13-t：一道淡，与上面的数字字段、开关、下拉同档。
				e = lockedControl(e)
			}
			// A text field is controlled the same way: every keystroke is the
			// field's text, the bounds are checked at once, and the typed text —
			// not a sanitized one — is the draft value, as the browser's
			// `event.target.value` was (`ExtensionsPanel.tsx:3253-3257`).
			if e.Changed() && !locked {
				draft.setLive(field, edit)
				draft.values[field.Key] = edit
				if _, problem := ConfigValueProblem(field, edit, copy); problem != "" {
					a.noteConfigFieldError(integration.ID, field.Key, problem)
				} else {
					a.clearConfigFieldError(integration.ID, field.Key)
				}
			}
			if e.Submitted() && !locked {
				draft.values[field.Key] = edit
				if _, problem := ConfigValueProblem(field, edit, copy); problem != "" {
					a.noteConfigFieldError(integration.ID, field.Key, problem)
				} else {
					a.clearConfigFieldError(integration.ID, field.Key)
				}
			}
			return e
		})
	}
}

// ConfigValueProblem checks one field's submitted text against the schema bounds
// the reference handed to the native control (`min`/`max` on the number input,
// `minLength`/`maxLength` on a text one, `ExtensionsPanel.tsx:3251-3256`) and
// returns the value a save would carry plus the sentence to show at the field,
// empty when the text is acceptable. A number field that is empty carries no
// value at all, exactly as the reference's `event.target.value === "" ? null`
// did; a text field keeps its text, empty included.
//
// It is exported from the panel so the host's own validation and the form's
// own hint cannot drift into two rules, and so a test can pin the bounds
// without driving a keystroke through the toolkit's editor.
func ConfigValueProblem(field ConfigField, edit string, copy i18n.Settings) (any, string) {
	switch strings.ToLower(field.Type) {
	case "number":
		if strings.TrimSpace(edit) == "" {
			return nil, ""
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(edit), 64)
		if err != nil {
			return nil, copy.ConfigNumber
		}
		if field.Minimum != nil && number < *field.Minimum || field.Maximum != nil && number > *field.Maximum {
			return nil, copy.ConfigOutOfRange
		}
		return number, ""
	default:
		// `minlength` is not checked on an empty value, as the HTML constraint it
		// mirrors (`minLength={field.type === "text" ? … : undefined}`,
		// `ExtensionsPanel.tsx:3255`) is not either: an untouched field is empty,
		// not too short.
		if edit == "" {
			return edit, ""
		}
		length := len([]rune(edit))
		if field.MinLength != nil && length < *field.MinLength || field.MaxLength != nil && length > *field.MaxLength {
			return nil, copy.ConfigOutOfLength
		}
		return edit, ""
	}
}

// configFieldProblem is the sentence shown at one field, or empty.
func (a *App) configFieldProblem(integration Integration, field ConfigField) string {
	return a.configFieldErrors[configFieldErrorKey(integration.ID, field.Key)]
}

// configFieldErrorKey names one field's own hint, so two integrations — or two
// fields of one — cannot borrow each other's sentence.
func configFieldErrorKey(id, key string) string { return id + "\x00" + key }

// noteConfigFieldError puts a sentence at one field. The reference's own
// `detailError` — which is what the form's bottom line carries — was untouched
// by a keystroke (`ExtensionsPanel.tsx:3011`), so a host-side failure stays
// where it was put: the field's hint is about the schema, the bottom line about
// the write.
func (a *App) noteConfigFieldError(id, key, problem string) {
	if a.configFieldErrors == nil {
		a.configFieldErrors = map[string]string{}
	}
	a.configFieldErrors[configFieldErrorKey(id, key)] = problem
}

// clearConfigFieldError drops one field's sentence once an edit satisfies it.
func (a *App) clearConfigFieldError(id, key string) {
	if a.configFieldErrors != nil {
		delete(a.configFieldErrors, configFieldErrorKey(id, key))
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
// reference's `.extension-config-field select[multiple]` shape — a field face,
// a `--radius-sm` corner and the reference's 66px floor
// (`extensions.css:2420-2423`) — with fn filling the tick rows inside it. It is
// one element, not a column of rows, so the field reads as the single native
// control the reference drew.
//
// P8-c 随 §0(b) 重定基线：面 `tokens.Field` + `FieldSlot` 凹痕叠层 → `Control`
// 一档（与 `fieldFace`/`sheetFieldSlot` 同面）。同一个应用里字段只有一种材质：
// `--glass-field-shadow` 在浅色下的白边是 0.85，与合约 §1 的 0.04 上限互斥，
// 而列表箱跟其他字段是同一族控件。几何（66px 地板、`RadiusSM`、1px 边、
// 4px/8px 的 padding 对）零改动。
//
// The box is a *viewport*, not a growing list: a native `select[multiple]` of
// the reference's 66px keeps that height however many options it holds and
// scrolls the rest under it, so a long option list cannot push the form's
// actions off the card. The inner scroller is therefore capped at the 66px
// viewport less the box's own 4px vertical padding, and a short list keeps the
// same 66px floor rather than shrinking. The scroller rides the box's own
// state, so a scroll stays where the user left it.
func (a *App) configMultiselectBox(c *ui.Context, scroll *ui.ScrollState, locked bool, fn func()) ui.Element {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	padY, padX := a.cardSpace(t, 1), a.cardSpace(t, 2)
	border := float32(1) // the box's own 1px keyline, inside its 66px
	viewport := a.cardSpace(t, 16.5)
	box := ui.Column(c).FillWidth().MinHeight(viewport).
		Padding(padY, padX).
		Border(1, tokens.ControlEdge).
		Radius(tokens.RadiusSM).
		Background(tokens.Control).
		Clip()
	// P13-m（`docs/liquid-glass.md` §6.2b-2 第 9-15 行）：这个箱子的注释一直写着
	// 「与 `fieldFace`/`sheetFieldSlot` 同面」，而那两个字段自 P12-c 起就是 E1 的
	// 一对面（顶受光边 + 底对侧暗线）——只有它停在了「一块被画框圈住的平涂」。
	// P13-i / P13-l 的单行 grep 照不到它，因为 `Border(...)` 与
	// `Background(tokens.Control)` 不在同一行。外圈 `ControlEdge` 不动：受光边是
	// **内线**，画框是外线（§1）。
	box = tokens.LitFace(box)
	box.Children(func() {
		ui.Scroll(c).FillWidth().MaxHeight(viewport - 2*padY - 2*border).TrackScroll(scroll).Children(func() {
			ui.Column(c).FillWidth().Gap(a.cardSpace(t, 0.5)).Children(fn)
		})
	})
	// P13-t：锁定的档位落在**控件根**上，整棵子树（箱面、选项文字、勾选框）同乘
	// 框架的 0.5。这一臂曾经只有 `.Opacity(0.6)`、没有 `Disabled`，于是同一个
	// 锁定表单里字段 0.3、这个框 0.6、框里的勾选框 0.25。
	if locked {
		box = lockedControl(box)
	}
	return box
}

// configOptionRow draws one tick option inside the multiselect list box: the
// option's word at the start and the check box at its end, in one compact
// row, so the box reads as the option list it stands in for. It reports
// whether the option was clicked — the word is a hit target, as the
// framework's own Field makes a control's label — and the modifier keys the
// click held, so the box can extend a range the way a native
// `select[multiple]` does. A locked box refuses every tick, the whole-row
// `busy` the reference handed the native `select[multiple]`
// (`disabled={disabled}`, `ExtensionsPanel.tsx:3222,3234`).
func (a *App) configOptionRow(c *ui.Context, option string, on *bool, changed *bool, locked bool) (bool, ui.Modifiers) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	clicked := false
	var mods ui.Modifiers
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Gap(a.cardSpace(t, 2)).MinHeight(a.cardSpace(t, 5)).Children(func() {
		word := ui.Text(c, option).FontSize(tokens.Body).FontWeight(500).TextColor(tokens.TextSecondary)
		if word.Clicked() && !locked {
			clicked = true
			mods = word.ClickModifiers()
		}
		box := ui.CheckboxBase(c, on).Label(option)
		// P13-t：这里曾经在 `locked` 时再挂一次 `Disabled(true)`。列表框自己已经是
		// `Disabled`（`config.go` 的 multiselect 臂），`IsDisabled` 走祖先链
		// （`ui/element.go:1349-1356`），点击与 hover 一样被祖先挡住；再挂一次就是
		// 0.5 × 0.5 = 0.25——一个框里的勾选框比箱面深一档，正是这一轮要收掉的
		// 「同一件事三种深度」。`locked` 仍然只用在两条点击守卫上。
		if box.Clicked() && !locked {
			*changed = true
			mods = box.ClickModifiers()
		}
	})
	return clicked, mods
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

// configListScroll keeps one multiselect list box's offset, so an option list
// longer than the box's 66px viewport scrolls where the user left it rather
// than jumping to its start on every redraw.
func (a *App) configListScroll(id, key string) *ui.ScrollState {
	if a.configListScrolls == nil {
		a.configListScrolls = map[string]*ui.ScrollState{}
	}
	name := id + "\x00" + key
	state, ok := a.configListScrolls[name]
	if !ok {
		state = &ui.ScrollState{}
		a.configListScrolls[name] = state
	}
	return state
}
