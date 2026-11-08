package settingsui

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
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
	// kinds retires editor-only state when a schema reuses a key for another
	// control. Stored values remain owned by the existing save contract.
	kinds map[string]string
	// Snapshot pointer-valued bounds so an in-place schema update is visible.
	constraints map[string]configConstraints
	// multiAnchor is each multiselect list box's last clicked option value,
	// keyed by the field. A native `select[multiple]` extends a range from that
	// anchor on Shift-click (`Ctrl`/`Cmd`-click toggles one, a plain click
	// starts over), and the toolkit hands the click's modifiers through, so a
	// range is the one selection habit this box can keep.
	multiAnchor map[string]string
	// saved is the values the form last wrote (or read), the baseline the
	// dirty check compares against. The shell refreshes the integration list
	// off the main thread, so until that lands `integration.ConfigValues` is
	// still the *previous* generation; keeping the baseline beside the draft
	// is what lets a Save flip the status to "Saved" at once, exactly as the
	// reference's `setSavedConfigValues(saved.values)` did
	// (`ExtensionsPanel.tsx:2001`).
	saved map[string]any
	// observed is the last effective host generation, separate from saved:
	// a successful save can precede the shell's refreshed integration list.
	observed map[string]any
}

type configConstraints struct {
	numbers [2]float64
	lengths [2]int
	present [4]bool
}

func constraintsOf(field ConfigField) configConstraints {
	var out configConstraints
	for i, value := range []*float64{field.Minimum, field.Maximum} {
		if value != nil {
			out.numbers[i], out.present[i] = *value, true
		}
	}
	for i, value := range []*int{field.MinLength, field.MaxLength} {
		if value != nil {
			out.lengths[i], out.present[i+2] = *value, true
		}
	}
	return out
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
// `values`: what a landed Save needs, since the write moves every value behind
// the fields' backs at once.
func (d *configDraft) clearLive() { d.live = nil }

// clearLiveField drops one field's live text, so that field falls back to its
// `value`. Restore defaults uses this rather than `clearLive`: it writes back
// only the fields whose schema declared a default and leaves the others as they
// are (`ExtensionsPanel.tsx:2013-2016`), so a field it does not touch must keep
// the half-typed text the user was still editing. Dropping every field's live
// text would strand a no-default field mid-edit — its value stays untouched but
// its display snaps back to empty, losing what the user had typed.
func (d *configDraft) clearLiveField(field ConfigField) {
	if d.live != nil {
		delete(d.live, field.Key)
	}
}

// multiAnchorAt is one multiselect field's last clicked option index, or -1
// when it has none yet.
func (d *configDraft) multiAnchorAt(field ConfigField) int {
	if d.multiAnchor == nil {
		return -1
	}
	value, ok := d.multiAnchor[field.Key]
	if !ok {
		return -1
	}
	for index, option := range field.Options {
		if option == value {
			return index
		}
	}
	// A removed option cannot lend its anchor to the next option in the list.
	delete(d.multiAnchor, field.Key)
	return -1
}

// setMultiAnchor records the option a range extends from.
func (d *configDraft) setMultiAnchor(field ConfigField, index int) {
	if d.multiAnchor == nil {
		d.multiAnchor = map[string]string{}
	}
	if index < 0 || index >= len(field.Options) {
		delete(d.multiAnchor, field.Key)
		return
	}
	d.multiAnchor[field.Key] = field.Options[index]
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
	incoming := make(map[string]any, len(integration.Config))
	acceptedEdit := false
	for _, field := range integration.Config {
		value, exists := integration.ConfigValues[field.Key]
		if !exists {
			value, exists = field.Default, field.Default != nil
		}
		if exists {
			incoming[field.Key] = value
		}
		previous, hadPrevious := draft.observed[field.Key]
		if draft.observed == nil || (hadPrevious == exists && configValuesEqual(previous, value)) {
			continue
		}
		current, hasCurrent := draft.values[field.Key]
		saved, hasSaved := draft.saved[field.Key]
		edit, editing := draft.live[field.Key]
		if exists && hasCurrent && configValuesEqual(current, value) {
			// An external writer can land the exact same edit. It is now
			// saved too; only partial/invalid display text keeps its ownership.
			if parsed, problem := ConfigValueProblem(field, edit, a.copy()); !editing || (problem == "" && configValuesEqual(parsed, value)) {
				acceptedEdit = acceptedEdit || !hasSaved || !configValuesEqual(current, saved)
				draft.saved[field.Key] = value
				draft.clearLiveField(field)
				delete(draft.multiAnchor, field.Key)
				a.clearConfigFieldError(integration.ID, field.Key)
				continue
			}
		}
		if editing {
			// Returning to the saved value withdraws this edit. A numeric
			// spelling such as 5.0 is the same value, but 1e is still a draft.
			parsed, problem := ConfigValueProblem(field, edit, a.copy())
			if problem == "" && hasCurrent == hasSaved && configValuesEqual(current, saved) && configValuesEqual(parsed, saved) {
				draft.clearLiveField(field)
				a.clearConfigFieldError(integration.ID, field.Key)
				editing = false
			}
		}
		if editing || hasCurrent != hasSaved || !configValuesEqual(current, saved) {
			// Preserve the local edit, but compare it with what is stored now.
			// Returning to the new host value must read Saved; restoring an
			// obsolete default must remain an actual change.
			if exists {
				draft.saved[field.Key] = value
			} else {
				delete(draft.saved, field.Key)
			}
			continue
		}
		// Only a genuinely new host generation can replace an untouched
		// field. Local edits and partial text keep their existing ownership.
		// Replacing a selection ends the old Shift-click gesture too.
		delete(draft.multiAnchor, field.Key)
		hadProblem := a.configFieldProblem(integration, field) != ""
		a.clearConfigFieldError(integration.ID, field.Key)
		if exists {
			draft.values[field.Key], draft.saved[field.Key] = value, value
		} else {
			delete(draft.values, field.Key)
			delete(draft.saved, field.Key)
		}
		// A field already checked by Enter still describes the current host
		// value. Replacing one invalid value with another cannot erase its hint.
		if hadProblem && exists {
			text, _ := value.(string)
			if number, ok := value.(float64); ok {
				text = formatFieldNumber(number)
			}
			if _, problem := ConfigValueProblem(field, text, a.copy()); problem != "" {
				a.noteConfigFieldError(integration.ID, field.Key, problem)
			}
		}
	}
	draft.observed = incoming
	// A refreshed schema may introduce fields after this draft was opened.
	// Initialize only absent keys; an explicit nil is an existing empty/invalid
	// edit and must not be replaced by the default on the next frame.
	for _, field := range integration.Config {
		if _, exists := draft.values[field.Key]; exists {
			continue
		}
		value, exists := integration.ConfigValues[field.Key]
		if !exists {
			value = field.Default
		}
		if exists || value != nil {
			draft.values[field.Key] = value
			draft.saved[field.Key] = value
		}
	}
	// An external writer can finish the exact edit whose save failed here.
	// Retire that failure only once the whole draft is confirmed; unrelated
	// refreshes, partial edits and in-flight saves retain their own feedback.
	if acceptedEdit && len(draft.live) == 0 && !a.configSaveBusy[integration.ID] && !a.configDirty(integration, draft) {
		delete(a.configError, integration.ID)
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
	// A removed list never renders another frame to retire its range anchor.
	// Keep values and saved edits; only range gestures belong to visible lists.
	if draft := a.configDrafts[integration.ID]; draft != nil {
		active := make(map[string]bool)
		fields := make(map[string]bool)
		for _, field := range integration.Config {
			fields[field.Key] = true
			if strings.EqualFold(field.Type, "multiselect") {
				active[field.Key] = true
			}
		}
		for key := range draft.live {
			if !fields[key] {
				delete(draft.live, key)
				a.clearConfigFieldError(integration.ID, key)
			}
		}
		for key := range draft.multiAnchor {
			if !active[key] {
				delete(draft.multiAnchor, key)
			}
		}
		prefix := integration.ID + "\x00"
		// Enter can validate a stored value without creating live text.
		// Retire that field's error independently when its schema disappears.
		for key := range a.configFieldErrors {
			if strings.HasPrefix(key, prefix) && !fields[strings.TrimPrefix(key, prefix)] {
				delete(a.configFieldErrors, key)
			}
		}
		for key := range a.configListScrolls {
			if strings.HasPrefix(key, prefix) && !active[strings.TrimPrefix(key, prefix)] {
				delete(a.configListScrolls, key)
			}
		}
	}
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
	// The draft's lifetime owns the editors' undo history as well as the values.
	form := ui.Column(c.Key(draft)).FillWidth().Gap(a.cardSpace(t, 3))
	a.rememberOperationFocus(c, integration.ID, "save", form).Children(func() {
		// The reference's `.extension-config-form` is one `display: grid` with a
		// 12px gap (`extensions.css:2369-2373`, literal px) holding the toolbar,
		// the fields and the action row as *its own* children, in order — no
		// plane and no rules between them. The old port wrapped the fields in a
		// second filled card, so the form read as a bordered box inside the
		// integration card rather than the reference's flat grid.
		a.configToolbar(c, copy, integration, draft)
		feedback := a.permissionFeedback(integration) + approvalFeedback(integration)
		for _, field := range integration.Config {
			grid := a.configField(c, copy, integration, draft, field, strconv.Quote(integration.Error)+strconv.Quote(integration.Description)+feedback)
			a.rememberOperationFocus(c, integration.ID, "field", grid)
			feedback += strconv.Quote(field.Label) + strconv.Quote(field.Description) +
				strconv.Quote(configProblemText(a.configFieldProblem(integration, field), copy))
		}
		if a.configError[integration.ID] != "" {
			// The failure the shell reported on the Save is the reference's own
			// `.extensions-notice--error` sentence under the fields
			// (`ExtensionsPanel.tsx:2674`), not a bare red line.
			a.noticeLine(c, a.configError[integration.ID])
		}
		// Keep the actions' identity when a failure notice is inserted above it.
		a.configActions(c.Key("config.actions"), copy, integration, draft,
			strconv.Quote(integration.Error)+strconv.Quote(integration.Description)+feedback+strconv.Quote(a.configError[integration.ID]))
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
	busy := a.configOperationNow(integration.ID) != ""
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		status := copy.ConfigSavedState
		ink := tokens.TextTertiary
		if dirty {
			status, ink = copy.ConfigUnsaved, t.Warning
		}
		// P13-el：写入在飞的那几秒里 baseline 还没重设（`finishConfigSave` 才
		// re-baseline），所以 `dirty` 仍为真，这一格此前仍在用**按下之前**那句话
		// 指控用户——而卡片底部的药丸已经在说 "Saving…" 并且熄了灯。同一件事两个
		// 控件说两句话，响的那一句还是过时的。在飞时词换成 Save 已经在用的那个
		// `ConfigSaving`（不新增文案），墨落到与落地后的 "Saved" **同一档**：于是
		// 「在飞 → 落地」是纯换词、零换墨，而「按下 → 在飞」把警报关掉。
		if a.configSaveBusy[integration.ID] {
			status, ink = copy.ConfigSaving, tokens.TextTertiary
		}
		// P13-em（`docs/liquid-glass.md` §6.108）：失败时这张卡上有**两处**告警——
		// 这个词，和字段下面那条错误带子（`noticeLine`：`Control` 面 + `Warning`
		// keyline + `alert` 图标 + `Warning` 句子）。带子扛着全部信息（错误原文），
		// 而词在这一格只是「改动还没存住」这条**状态**，它自己一个字都没变
		// （失败后 `dirty` 仍为真，所以词仍是 `ConfigUnsaved`——改词要新增 i18n
		// 文案，踩红线）。§6.107 的判据反过来用：词只在它是**唯一载体**时才是
		// 告警；带子在场时它不是，于是退回陈述档，与 "Saved" / "Saving…" 同一档。
		if a.configError[integration.ID] != "" {
			ink = tokens.TextTertiary
		}
		ui.Text(c, status).FontSize(tokens.Caption).FontWeight(500).TextColor(ink)
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(0.5)).Children(func() {
			copyDisabled := a.Actions.CopyConfiguration == nil || busy
			var copyButton ui.Element
			a.formFeedbackOutlet(c.Key("config-copy-outlet"), strconv.Quote(integration.Error)+strconv.Quote(integration.Description)+a.permissionFeedback(integration)+approvalFeedback(integration), func() {
				copyButton = a.extensionIconButton(c, "copy", copy.ConfigCopy, copyDisabled)
			})
			a.rememberOperationFocus(c, integration.ID, "transfer", copyButton)
			if copyButton.Clicked() && !copyDisabled {
				a.startConfigOperation(integration.ID, "copy", func(done func(error)) {
					a.Actions.CopyConfiguration(integration.ID, draft.values, done)
				})
			}
			exportDisabled := a.Actions.ExportConfiguration == nil || busy
			var exportButton ui.Element
			a.formFeedbackOutlet(c.Key("config-export-outlet"), strconv.Quote(integration.Error)+strconv.Quote(integration.Description)+a.permissionFeedback(integration)+approvalFeedback(integration), func() {
				exportButton = a.extensionIconButton(c, "file-down", copy.ConfigExport, exportDisabled)
			})
			a.rememberOperationFocus(c, integration.ID, "transfer", exportButton)
			if exportButton.Clicked() && !exportDisabled {
				a.startConfigOperation(integration.ID, "export", func(done func(error)) {
					a.Actions.ExportConfiguration(integration.ID, draft.values, done)
				})
			}
			// A withdrawn host capability has no future retry on this icon.
			// A busy transfer does: keep that operation's keyboard identity.
			if !busy {
				if copyButton.Focused() && copyDisabled && !exportDisabled {
					exportButton.Focus()
				} else if exportButton.Focused() && exportDisabled && !copyDisabled {
					copyButton.Focus()
				}
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
	a.configOperationMu.Lock()
	if a.configOperations == nil {
		a.configOperations = map[string]string{}
	}
	a.configOperations[id] = operation
	a.configOperationMu.Unlock()
	run(func(error) { a.finishConfigOperation(id) })
}

// finishConfigOperation releases the slot and asks for a frame, so the
// buttons' disabled face is gone with it, exactly as the reference's `finally`
// did.
func (a *App) finishConfigOperation(id string) {
	a.configOperationMu.Lock()
	if a.configOperations != nil {
		delete(a.configOperations, id)
	}
	a.configOperationMu.Unlock()
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// configOperationNow reports whether an integration has a copy or export in
// flight, for the shell's own hand-off and the tests.
func (a *App) configOperationNow(id string) string {
	a.configOperationMu.RLock()
	defer a.configOperationMu.RUnlock()
	return a.configOperations[id]
}

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
func (a *App) configActions(c *ui.Context, copy i18n.Settings, integration Integration, draft *configDraft, feedback ...string) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		// Both pills are disabled while an operation is in flight
		// (`disabled={Boolean(busy) || Boolean(configOperation)}` on the
		// Defaults, `disabled={Boolean(busy) || !configDirty}` on the Save),
		// where `busy` is one per-integration item held for every mutation of
		// that row (`ExtensionsPanel.tsx:3016,3019`).
		busy := a.configLocked(integration.ID) || a.configOperationNow(integration.ID) != "" || a.configSaveBusy[integration.ID]
		// P13-s（§6.2b-6）：`busy` 不再用组透明度：它只传给面，面走 `ControlDim`
		// 并熄灯，字自己淡到 40%。
		var defaults ui.Element
		a.formFeedbackOutlet(c.Key("config-defaults-outlet"), strings.Join(feedback, ""), func() {
			defaults = a.extensionGlyphActionButton(c, "rotate-ccw", copy.ConfigDefaults,
				t.Space(7.5), t.Space(3.75), tokens.Body, busy)
		})
		if busy {
			defaults = defaults.Disabled(true).Cursor(ui.CursorDefault)
		}
		if defaults.Clicked() && !busy {
			// The reference's `configurationDefaults` writes every field whose
			// schema declared a default back to it, leaving the others as they
			// are (`ExtensionsPanel.tsx:2013-2016`). A restored field shows its
			// live text, so the reset also drops *that field's* text: otherwise
			// the new default would sit behind a stale half-typed value. A field
			// with no default is left exactly as the reference left it — including
			// the text the user is still typing — so its live entry is untouched.
			for _, field := range integration.Config {
				if field.Default != nil {
					draft.clearLiveField(field)
					delete(draft.multiAnchor, field.Key)
					draft.values[field.Key] = field.Default
					// The old edit's validity does not describe the restored default.
					a.clearConfigFieldError(integration.ID, field.Key)
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
		var save ui.Element
		a.formFeedbackOutlet(c.Key("config-save-outlet"), strings.Join(feedback, ""), func() { save = a.extensionPrimaryButton(c, saveLabel, saveDisabled) })
		if save.Clicked() && !saveDisabled {
			if a.configSaveBusy == nil {
				a.configSaveBusy = map[string]bool{}
			}
			a.configSaveBusy[integration.ID] = true
			delete(a.configError, integration.ID)
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
func (a *App) configFieldGrid(c *ui.Context, field ConfigField, label, requiredLabel, hint string, control func() ui.Element, feedback ...string) ui.Element {
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
				// P13-az：必填标记的提示是浮面（§6.20 20-3）。
				star := ui.Text(c, "*").FontSize(t.FontSize).FontWeight(600).TextColor(t.Warning).SingleLine()
				tokens.Tip(c, star, requiredLabel)
			}
		})
		// Grid percentages resolve against the whole grid in mygo. Give the
		// control a cell-sized parent so FillWidth excludes the label and gap.
		ui.Column(c).ColumnStart(2).MinWidth(0).Children(func() {
			// Reveal the field beyond the page fade on entry. A list keeps its
			// separate inner scroll position while its whole box becomes visible.
			a.formFeedbackOutlet(c.Key("config-control-outlet"), strings.Join(feedback, ""), func() { control() })
		})
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
	return grid
}

// configSelect is the config field's drop-down, on the grid's second column:
// `.extension-config-field select` shares the input's literal box
// (`extensions.css:2402-2417`) but fills its column rather than taking the
// settings rows' fixed width. It is `chooseSelect` without that 220px.
//
// `lit` is the field material's state, the same one `extensionFieldLit` takes:
// this drop-down sits in the same form and locks with it, so it must go unlit
// with the fields beside it (P13-cb).
func (a *App) configSelect(c *ui.Context, selected *string, labels []string, label string, lit bool) ui.Element {
	e := ui.Select(c, selected, labels).Label(label).FillWidth().Radius(999)
	if a.cardUnit > 0 {
		e = e.MinHeight(28).Padding(4, 8)
	}
	return a.fieldFace(c, e, lit)
}

func (a *App) configSelectFace(c *ui.Context, e ui.Element, lit bool) ui.Element {
	if a.cardUnit > 0 {
		e = e.MinHeight(28).Padding(4, 8)
	}
	return a.fieldFace(c, e, lit)
}

// configField draws one field, by its type. Every field wears the reference's
// `.extension-config-field` two-column grid (see `configFieldGrid`): a 118px
// label column and the control, with the description under the control. The
// old port drew each field as a settings row (label left, control right, the
// description under the *label*), so a form's controls did not share one
// column and the description sat on the wrong side of the field.
func (a *App) configField(c *ui.Context, copy i18n.Settings, integration Integration, draft *configDraft, field ConfigField, feedback ...string) ui.Element {
	// A schema reorder moves the field, not its keyboard selection or history.
	kind := strings.ToLower(field.Type)
	if draft.kinds == nil {
		draft.kinds = map[string]string{}
	}
	if previous, exists := draft.kinds[field.Key]; exists && previous != kind {
		draft.clearLiveField(field)
		a.clearConfigFieldError(integration.ID, field.Key)
	}
	draft.kinds[field.Key] = kind
	c = c.Key("config.field." + field.Key + "\x00" + kind)
	// Recheck a stored-value error only when its constraints actually change.
	// Host errors and untouched fields retain their existing validation state.
	constraints := constraintsOf(field)
	previous, observed := draft.constraints[field.Key]
	if draft.constraints == nil {
		draft.constraints = map[string]configConstraints{}
	}
	draft.constraints[field.Key] = constraints
	recheck := observed && previous != constraints && a.configFieldProblem(integration, field) != ""
	edit, editing := draft.live[field.Key]
	if !editing && recheck {
		edit, _ = draft.values[field.Key].(string)
		if number, ok := draft.values[field.Key].(float64); ok {
			edit = formatFieldNumber(number)
		}
	}
	if editing || recheck {
		if _, problem := ConfigValueProblem(field, edit, copy); problem != "" {
			a.noteConfigFieldError(integration.ID, field.Key, problem)
		} else {
			a.clearConfigFieldError(integration.ID, field.Key)
		}
	}
	label := field.Label
	if label == "" {
		label = field.Key
	}
	locked := a.configLocked(integration.ID)
	var grid ui.Element
	fieldGrid := func(c *ui.Context, field ConfigField, label, requiredLabel, hint string, control func() ui.Element) ui.Element {
		return a.configFieldGrid(c, field, label, requiredLabel, hint, control, feedback...)
	}

	switch kind {
	case "boolean":
		on := false
		if value, ok := draft.values[field.Key].(bool); ok {
			on = value
		}
		changed := false
		grid = fieldGrid(c, field, label, copy.ConfigRequired, "", func() ui.Element {
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
		labels, values := configSelectChoices(field, copy.ConfigNone)
		current, _ := draft.values[field.Key].(string)
		selected := configChoice{value: current, none: current == "" && !field.Required}
		changed := false
		grid = fieldGrid(c, field, label, copy.ConfigRequired, "", func() ui.Element {
			e := valueSelect(c, &selected, labels, values, current, a.tokens(c), !locked && len(values) > 0).Label(label).FillWidth().Radius(999)
			e = a.configSelectFace(c, e, !locked && len(values) > 0)
			if locked {
				// P13-t：一道淡（框架的 0.5），与同一张表单的字段、开关同档。
				// P13-cb：灯已经在 `!locked` 那一步交给材质了，这里只挂状态。
				e = lockedControl(e)
			}
			if e.Changed() && !locked {
				changed = true
			}
			return e
		})
		if changed && selected.none {
			// The reference writes `null` for the empty option
			// (`event.target.value === "" ? null : …`), which the schema reads
			// back as "unset".
			draft.values[field.Key] = nil
		} else if changed {
			draft.values[field.Key] = selected.value
		}
	case "multiselect":
		// A value has one row and one range position even if a schema repeats it.
		field.Options = uniqueConfigOptions(field.Options)
		// Observe removals even when no click occurs in that frame. A value
		// reintroduced later must not resurrect a retired range anchor.
		draft.multiAnchorAt(field)
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
		grid = fieldGrid(c, field, label, copy.ConfigRequired, "", func() ui.Element {
			marker := ui.Box(c).Absolute().Size(0, 0).PassThrough()
			focus := ui.Local(marker, "option.focus", func() ui.Handle { return ui.Handle{} })
			hadFocus := focus.FocusWithin(c)
			box := a.configMultiselectBox(c, a.configListScroll(integration.ID, field.Key), locked, func() {
				for index, option := range field.Options {
					index, option := index, option
					on := ticked[option]
					boxChanged := false
					labelClicked, mods, row := a.configOptionRow(c.Key("config.option."+option), option, &on, &boxChanged, locked, index)
					if row.FocusWithin() {
						row.Bind(focus)
					}
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
			// A removed option leaves no keyboard position. Use the existing
			// group as a neutral Tab origin; Space here must not tick a peer.
			if !locked && hadFocus && !c.Resolve(*focus).Valid() {
				box.Focus()
			}
			box.Label(label)
			if box.Focused() {
				box.Focusable()
			}
			return box
		})
	case "number":
		value := ""
		if number, ok := draft.values[field.Key].(float64); ok {
			value = formatFieldNumber(number)
		}
		edit := draft.liveText(field, value)
		problem := configProblemText(a.configFieldProblem(integration, field), copy)
		grid = fieldGrid(c, field, label, copy.ConfigRequired, problem, func() ui.Element {
			e := a.extensionFieldLit(c, &edit, configFieldMetrics, !locked).Placeholder(field.Key).Label(label).FillWidth()
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
				// P13-cb：灯已经在上面那行的 `!locked` 里交给材质了（`fieldFace` 的
				// `DimFace` 臂），这里只挂状态——两道判断读的是同一个 `locked`。
				e = lockedControl(e.TextColor(a.tokens(c).Disabled))
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
				if number, err := strconv.ParseFloat(text, 64); err == nil && text != "" && !math.IsNaN(number) && !math.IsInf(number, 0) {
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
		problem := configProblemText(a.configFieldProblem(integration, field), copy)
		grid = fieldGrid(c, field, label, copy.ConfigRequired, problem, func() ui.Element {
			e := a.extensionFieldLit(c, &edit, configFieldMetrics, !locked).Placeholder(field.Key).Label(label).FillWidth()
			if kind == "password" {
				e = e.Password()
			}
			if problem != "" {
				e = e.Error(problem)
			}
			if locked {
				// P13-t：一道淡，与上面的数字字段、开关、下拉同档。
				// P13-cb：同上——材质已经在 `!locked` 里定了，这里只挂状态。
				e = lockedControl(e.TextColor(a.tokens(c).Disabled))
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
	return grid
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
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
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

// Only the three local validation messages follow the current locale. Host
// errors remain verbatim, and switching language does not edit the draft.
func configProblemText(problem string, copy i18n.Settings) string {
	en, zh := i18n.For("en").Settings, i18n.For("zh").Settings
	switch problem {
	case en.ConfigNumber, zh.ConfigNumber:
		return copy.ConfigNumber
	case en.ConfigOutOfRange, zh.ConfigOutOfRange:
		return copy.ConfigOutOfRange
	case en.ConfigOutOfLength, zh.ConfigOutOfLength:
		return copy.ConfigOutOfLength
	default:
		return problem
	}
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
// Clearing has its own identity, so a real value may equal the translated
// none word. Quote that real value to distinguish it visually from the action.
func configSelectLabels(field ConfigField, none string) []string {
	labels, _ := configSelectChoices(field, none)
	return labels
}

type configChoice struct {
	value string
	none  bool
}

func configSelectChoices(field ConfigField, none string) ([]string, []configChoice) {
	labels := make([]string, 0, len(field.Options)+1)
	values := make([]configChoice, 0, len(field.Options)+1)
	if !field.Required {
		labels = append(labels, none)
		values = append(values, configChoice{none: true})
	}
	for _, option := range field.Options {
		// The optional empty schema value is the same clear action, not an
		// extra blank row whose selection gets swallowed by an empty guard.
		if option == "" && !field.Required {
			continue
		}
		label := option
		if label == "" {
			label = none
		}
		if option == none && !field.Required {
			label = strconv.Quote(option)
		}
		labels = append(labels, label)
		values = append(values, configChoice{value: option})
	}
	return labels, values
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
		Radius(tokens.RadiusSM).
		Clip()
	// P13-m（`docs/liquid-glass.md` §6.2b-2 第 9-15 行）：这个箱子的注释一直写着
	// 「与 `fieldFace`/`sheetFieldSlot` 同面」，而那两个字段自 P12-c 起就是 E1 的
	// 一对面（顶受光边 + 底对侧暗线）——只有它停在了「一块被画框圈住的平涂」。
	// P13-i / P13-l 的单行 grep 照不到它，因为 `Border(...)` 与
	// `Background(tokens.Control)` 不在同一行。外圈 `ControlEdge` 不动：受光边是
	// **内线**，画框是外线（§1）。
	//
	// P13-cc（§6.46）：那两行此后不再在这里重复——面、外圈与那一对线都由
	// `fieldFace` 决定（它原本就在 `Clip()` 之后叠内阴影，顺序不变）。
	box = a.fieldFace(c, box, true)
	box.Children(func() {
		ui.Scroll(c).FillWidth().MaxHeight(viewport - 2*padY - 2*border).TrackScroll(scroll).Children(func() {
			ui.Column(c).FillWidth().Gap(a.cardSpace(t, 0.5)).Children(fn)
		})
		// P13-av（`docs/liquid-glass.md` §6.18）：56px 的视口两端原是平刀切——
		// 一行 20px 的选项在视口上下两条边被水平切断。它是「滚动收口」这一族的
		// 最后一块面（§6.16 / §6.17 的同族），所以边上也走 `glass.ScrollEdge`。
		//
		// 与那两处**故意不同**的两点：
		// ① 边高取 3u(12px)。视口是原生 `select[multiple]` 的 66px 地板算出来的
		//    56px（§6.16 的 4u 会吃掉它 21%），而一行的盒高是 20px——12px 只软化
		//    它下面那六成，不会把一行读没。2u 实测不够：最坏值顶 44/106（41%）、
		//    底 67/106（63%），底边压不住（P13-av 报告）。
		// ② 门必须开在滚动位置 `0 < Y < MaxY` 上，与 §6.17 同一条：这块箱子的
		//    66px 是原生地板，padding 不许长（长 1px 就从视口偷走一行），所以
		//    §6.16 那条「边落在预留 padding 上」的几何自门控在这里不成立。
		// `scroll` 可以是 nil（`configform_test` 的空滚动状态探针就传 nil）：那时
		// 没有偏移可读，也就没有「哪一头在被切」这件事，两条边都不画。
		face := configListEdgeFace(tokens, t)
		// 两道边骑的是**视口**的两端，不是箱子的 padding 框：绝对定位的 y 从
		// padding box 起算（`ui/layout.go` 的 `layoutAbsolute`），而 `.Clip()` 裁的
		// 是 border box，所以 `Top(0)` 画在箱子的 4px 内边距上——上面那 8px 里有
		// 4px 落在空白里，下面那 8px 整个落在空白里，视口一行都没淡到（实测贴
		// `Top(0)/Bottom(0)` 时下沿仍是 106/106 = 100%）。
		if scroll != nil && scroll.Y > 0 {
			ui.Box(c).Absolute().Top(padY).Left(0).Right(0).Height(a.cardSpace(t, 3)).PassThrough().
				Material(glass.ScrollEdge{Background: face})
		}
		if scroll != nil && scroll.Y < scroll.MaxY {
			ui.Box(c).Absolute().Bottom(padY).Left(0).Right(0).Height(a.cardSpace(t, 3)).PassThrough().
				Material(glass.ScrollEdge{Bottom: true, Background: face})
		}
	})
	// P13-t：锁定的档位落在**控件根**上，整棵子树（箱面、选项文字、勾选框）同乘
	// 框架的 0.5。这一臂曾经只有 `.Opacity(0.6)`、没有 `Disabled`，于是同一个
	// 锁定表单里字段 0.3、这个框 0.6、框里的勾选框 0.25。
	if locked {
		box = lockedControl(box)
	}
	return box
}

// configListEdgeFace is the colour the list box's scroll edges lay down: its own
// face, composited opaque.
//
// 为什么不直接把箱子的 `Control` 交给 `glass.ScrollEdge`：`Control` 是一层 0.07
// 的薄罩，而 `ui.Color.Alpha` 是**乘**——那道 0.85 会被它自己的 0.07 乘成 0.06，
// 边淡了个寂寞（§6.17 的 `runOutputEdgeFace` 是同一条反直觉的第一个受害处）。
//
// 为什么整条链一起压：箱子面不是一个字面值，而是一条——窗口地 → 设置体面板
// （`Sunken`）→ 扩展卡（`Control`）→ 箱子自己（`Control`）。按顺序压完整条链的
// 不透明结果才是这块箱子真正看上去的面；只混其中两层会得到近黑或近白。
func configListEdgeFace(tokens theme.Tokens, t *ui.Theme) ui.Color {
	pane := theme.FaceOver(t.Background, tokens.Sunken)
	card := theme.FaceOver(pane, tokens.Control)
	return theme.FaceOver(card, tokens.Control)
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
func (a *App) configOptionRow(c *ui.Context, option string, on *bool, changed *bool, locked bool, index ...int) (bool, ui.Modifiers, ui.Element) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	clicked := false
	var mods ui.Modifiers
	label := option
	if label == "" {
		label = a.copy().ConfigNone
	}
	// This list has a 12px fade, distinct from the page's 16px fade.
	// Reveal the row and its 3px keyboard ring without enlarging its hit area:
	// neighbouring rows are only 22px apart, so a padded parent steals clicks.
	row := ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Gap(a.cardSpace(t, 2)).MinHeight(a.cardSpace(t, 5))
	var outlet ui.Element
	position := ui.Local(row, "option.position", func() int { return -1 })
	moved := len(index) > 0 && *position != index[0]
	if len(index) > 0 {
		*position = index[0]
	}
	row.Children(func() {
		outlet = ui.Box(c.Key("option-focus-envelope")).Absolute().Top(-15).Bottom(-15).
			Left(0).Right(0).PassThrough()
		word := ui.Text(c, label).FontSize(tokens.Body).FontWeight(500).TextColor(tokens.TextSecondary).
			Grow(1).MinWidth(0).SingleLine()
		if word.Clicked() && !locked {
			clicked = true
			mods = word.ClickModifiers()
		}
		// CheckboxBase alone has neither a face nor a hit area. Keep the
		// existing native checkbox's size, check mark and focus treatment.
		box := ui.Checkbox(c, on, "").Label(label)
		if clicked {
			box.Focus()
		}
		// Reveal a moved keyboard target once. Ordinary wheel scrolling
		// remains free to move past an unchanged focused option.
		if moved && box.Focused() {
			outlet.ScrollIntoView()
		}
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
	was := ui.Local(row, "option-focused", func() bool { return false })
	focused := row.FocusWithin()
	if focused && !*was {
		outlet.ScrollIntoView()
	}
	*was = focused
	return clicked, mods, row
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

func uniqueConfigOptions(options []string) []string {
	seen := make(map[string]bool, len(options))
	unique := make([]string, 0, len(options))
	for _, option := range options {
		if !seen[option] {
			seen[option] = true
			unique = append(unique, option)
		}
	}
	return unique
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
