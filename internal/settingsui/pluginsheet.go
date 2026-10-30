package settingsui

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
)

// The generic plugin configuration sheet: the old build's R29 overlay, in the
// settings pages' own card language (R59).
//
// 「配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个新的
// 完全独立的页面。」 A plugin ships a `plugincfg.Schema` — an ordered list of
// fields — and this renders it: the same `SettingsCard` and `SettingsRow`
// language the pages use, the same controls, the same tokens. Nothing here
// knows a plugin's name: every label, bound and option comes from the schema,
// and every change goes back through the callbacks the launcher owns.
//
// The one field kind that is not a value is the action: a button that arms on
// the first press and runs on the second, so a plugin can offer "clear
// history" without this file growing a plugin-specific branch.

// PluginSheet is one frame of the overlay: what to draw, and what to report.
type PluginSheet struct {
	// Schema is the plugin's configuration, nil when there is none.
	Schema *plugincfg.Schema
	// Values are the values as the plugin's settings hold them.
	Values plugincfg.Values
	// Armed is the action key whose button has been pressed once, empty for
	// none.
	Armed string
	// Failed shows the one line under the fields: the last write was refused.
	Failed bool
	// ActionFailed is the action key whose command was refused, empty for
	// none: that row prints the field's own failure line, the way the old
	// build's `ActionControl` kept its own `failed` state beside the button.
	ActionFailed string
	// Change writes one field; the sheet paints the value either way and the
	// caller reports whether the plugin took it.
	Change func(key string, value any)
	// Arm and Run are the action's two presses.
	Arm func(key string)
	Run func(key string)
	// Unarm is the way back from an armed button.
	Unarm func(key string)
}

// PluginSheet draws a plugin's configuration: the title, the failure line,
// and each section's heading and card of rows. It is drawn where the result
// list would be — the launcher gives it the band and scrolls it.
func (a *App) PluginSheet(c *ui.Context, sheet PluginSheet) {
	if sheet.Schema == nil {
		return
	}
	t := c.Theme()

	// The sheet's own scroller: a schema longer than the band scrolls inside
	// it rather than growing the window past the band it was given.
	ui.Scroll(c.Key("plugin.config")).TrackScroll(&a.PluginScroll).FillWidth().
		Padding(t.Space(1), t.Space(4), t.Space(3), t.Space(4)).Children(func() {
		ui.Column(c).FillWidth().Gap(t.Space(3)).Children(func() {
			// The header: the plugin's own name at the meta register — one
			// step *below* a section heading, so the hierarchy a reader sees
			// is name (meta) > section (title) > field (label) > help, as the
			// old build's `.plugin-config__title` had it (body 600, muted).
			ui.Row(c).FillWidth().Height(t.Space(7)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, sheet.Schema.Title).FontSize(t.FontSize).FontWeight(600).
					TextColor(t.TextMuted).SingleLine()
			})
			if sheet.Failed {
				// The status line is the caption step in the warning colour
				// (`.plugin-config-action__status`): a muted line, never a
				// red banner.
				ui.Text(c, i18n.For(a.language()).Settings.SaveFailed).
					FontSize(t.FontSize - 1).TextColor(t.Warning)
			}
			for _, section := range plugincfg.Sections(sheet.Schema) {
				section := section
				a.sheetSection(c, sheet, section)
			}
		})
	})
}

// sheetSection draws one group: its heading (when it has one) and the card
// holding its fields.
func (a *App) sheetSection(c *ui.Context, sheet PluginSheet, section plugincfg.Section) {
	t := c.Theme()
	// The section is the settings pages' own: an 8u gap between the title and
	// its card, and the title at the title step (620, strong) — the old build's
	// `.plugin-config__section` / `__section-title`.
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		if section.Title != "" {
			// The group title starts on the column the row labels under it
			// start on: the card's own inset, added to the scroller's.
			ui.Text(c, section.Title).FontSize(t.FontSize+2).FontWeight(620).
				TextColor(a.tokens(c).TextStrong).Padding(0, t.Space(3.5))
		}
		a.card(c, func() {
			for _, field := range section.Fields {
				field := field
				a.sheetField(c, sheet, field)
			}
		})
	})
}

// sheetField draws one field with the control its kind names.
func (a *App) sheetField(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	value := sheet.Values[field.Key]
	switch field.Kind {
	case plugincfg.Toggle:
		on, _ := value.(bool)
		a.checkbox(c, field.Label, field.Help, on, func(next bool) { sheet.Change(field.Key, next) })
	case plugincfg.Select:
		current, _ := value.(string)
		a.choose(c, field.Label, field.Help, optionsOf(field.Options), current,
			func(next string) { sheet.Change(field.Key, next) })
	case plugincfg.Radio:
		current, _ := value.(string)
		a.sheetRadio(c, sheet, field, current)
	case plugincfg.Checkboxes:
		chosen := stringsOf(value)
		a.sheetCheckboxes(c, sheet, field, chosen)
	case plugincfg.Slider:
		number, _ := value.(float64)
		a.sheetSlider(c, sheet, field, number)
	case plugincfg.Number:
		number, _ := value.(float64)
		a.sheetNumber(c, sheet, field, number)
	case plugincfg.Text:
		text, _ := value.(string)
		edit := text
		// The reference stacks a free-text field: it takes the row's full
		// width, not a trailing slot (`.plugin-config-text { width: 100% }`,
		// `plugin-config.css:110-113`; `STACKED_KINDS` in
		// `plugins/controls.tsx:291-296`).
		a.stackedRow(c, field.Label, field.Help, func() {
			input := a.sheetTextField(c, field.Placeholder, &edit).Label(field.Label)
			if input.Submitted() {
				sheet.Change(field.Key, edit)
			}
		})
	case plugincfg.Action:
		a.sheetAction(c, sheet, field)
	}
}

// sheetRadio is a stacked row: the label and help above, the choices under
// them, as the old build's radio group drew.
func (a *App) sheetRadio(c *ui.Context, sheet PluginSheet, field plugincfg.Field, current string) {
	labels := make([]string, len(field.Options))
	index := 0
	for i, option := range field.Options {
		labels[i] = option.Label
		if option.Value == current {
			index = i
		}
	}
	chosen := index
	changed := false
	a.stackedRow(c, field.Label, field.Help, func() {
		a.segmented(c, labels, chosen, func(i int) {
			if i >= 0 && i < len(field.Options) {
				chosen = i
				changed = true
			}
		})
	})
	if changed {
		sheet.Change(field.Key, field.Options[chosen].Value)
	}
}

// sheetCheckboxes is the same stacked shape, with a check box per option.
func (a *App) sheetCheckboxes(c *ui.Context, sheet PluginSheet, field plugincfg.Field, current []string) {
	t := c.Theme()
	on := make([]bool, len(field.Options))
	for i, option := range field.Options {
		for _, value := range current {
			if value == option.Value {
				on[i] = true
			}
		}
	}
	pressed := -1
	a.stackedRow(c, field.Label, field.Help, func() {
		ui.Row(c).FillWidth().Gap(t.Space(3)).Wrap().Children(func() {
			for i, option := range field.Options {
				i, option := i, option
				checked := on[i]
				if ui.Checkbox(c, &checked, option.Label).Changed() {
					on[i] = checked
					pressed = i
				}
			}
		})
	})
	if pressed >= 0 {
		next := make([]string, 0, len(field.Options))
		for i, option := range field.Options {
			if on[i] {
				next = append(next, option.Value)
			}
		}
		sheet.Change(field.Key, next)
	}
}

// sheetSlider is the old build's capacity row: the label and help above, the
// track and its value under them, the value quantized to the schema's step
// and trailed by the schema's unit word.
func (a *App) sheetSlider(c *ui.Context, sheet PluginSheet, field plugincfg.Field, value float64) {
	t := c.Theme()
	tokens := a.tokens(c)
	v := value
	changed := false
	a.stackedRow(c, field.Label, field.Help, func() {
		// The reference's track-and-readout row: a 10u gap between the track
		// and its value (`.plugin-config-slider { gap: 10u }`,
		// `plugin-config.css:165-172`).
		ui.Row(c).FillWidth().Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			e := ui.Slider(c, &v, field.Min, field.Max).Grow(1)
			if e.Changed() {
				changed = true
			}
			// The value is the reference's `.plugin-config-slider__value`
			// (`plugin-config.css:174-192`): the caption step, muted ink and
			// tabular figures, right-aligned in a 48u column so a dragged
			// number does not shift its neighbours.
			ui.Row(c).MinWidth(t.Space(12)).Justify(ui.End).Children(func() {
				ui.Text(c, boundedText(field, v)).FontSize(tokens.Caption).
					TextColor(t.TextMuted).FontFeatures("tnum").SingleLine()
			})
		})
	})
	if changed {
		sheet.Change(field.Key, quantize(v, field.Step))
	}
}

// sheetNumber is a trailing row: the label and help at the start, the
// reference's number control at the end.
func (a *App) sheetNumber(c *ui.Context, sheet PluginSheet, field plugincfg.Field, value float64) {
	v := value
	changed := false
	a.row(c, field.Label, field.Help, func() {
		var slotChanged bool
		_, slotChanged = a.sheetNumberSlot(c, field, &v)
		changed = slotChanged
	})
	if changed {
		sheet.Change(field.Key, clampNumber(field, v))
	}
}

// sheetNumberSlot draws the reference's number control (R59): the unit rides
// *inside* the field slot beside the input, and the input right-aligns against
// it (`plugin-config.css:120-164`). It is deliberately **not** the framework's
// `ui.NumberInput`: that control draws its own bordered pane and a pair of
// `−`/`+` steppers the reference never had, so inside the reference's 28u slot
// it read as a box within a box. A parseable keystroke applies at once and the
// caller clamps the reported value to the schema's bounds; while the box keeps
// the keyboard, its in-progress text lives in `pluginNumberDrafts` so a
// half-typed or emptied value is not overwritten by the stored one mid-edit.
//
// It returns the slot for a caller that wants to bind it, and whether the
// frame's typing changed the value.
func (a *App) sheetNumberSlot(c *ui.Context, field plugincfg.Field, value *float64) (ui.Element, bool) {
	t := c.Theme()
	tokens := a.tokens(c)
	if a.pluginNumberDrafts == nil {
		a.pluginNumberDrafts = map[string]string{}
	}
	draft, ok := a.pluginNumberDrafts[field.Key]
	if !ok {
		draft = formatFieldNumber(*value)
	}
	changed := false
	// The number box is the field slot plus the reference's 4u gap between
	// the input and its unit (`.plugin-config-number { gap: 4u }`,
	// `plugin-config.css:153-157`).
	slot := a.sheetFieldSlot(c).Gap(t.Space(1))
	slot.Children(func() {
		// The input is the reference's own `.plugin-config-number__input`: 52u
		// wide, right-aligned, tabular figures, no box of its own (the slot is
		// the box).
		text := draft
		in := ui.TextInputBase(c, &text).Width(t.Space(13)).
			TextAlign(ui.End).FontFeatures("tnum").Label(field.Label)
		if in.Changed() {
			a.pluginNumberDrafts[field.Key] = text
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
				*value = parsed
				changed = true
			}
		} else if !in.Focused() {
			// The keyboard is elsewhere: the field reads the stored value
			// again, so a refused write never strands a half-typed one.
			delete(a.pluginNumberDrafts, field.Key)
		}
		if field.Unit != "" {
			// `.plugin-config-number__unit` is the caption step in the muted
			// register (`plugin-config.css:159-164`), not the control's body
			// step.
			ui.Text(c, field.Unit).FontSize(tokens.Caption).
				TextColor(t.TextMuted).SingleLine()
		}
	})
	return slot, changed
}

// sheetFieldSlot draws the overlay's field box (R59, `plugin-config.css:97-118`):
// the reference's 28u control — a text or number control's value lives inside
// it; the control itself is the caller's child, so the slot never draws a
// second surface under it.
//
// P8-b 重定基线：面 `--glass-field` + `--glass-field-shadow` 凹槽 → 合约 §6.1 的
// 低光泽一档。几何（28u 高、`RadiusSM` 圆角、`ControlEdge` 发丝、8u 侧 padding）
// 全部保留，只把面收敛到 `Control`/`KeycapFace` 一档：一个字段槽不需要额外的
// 凹痕叠层，因为它坐在已经比卡更亮的面里。
func (a *App) sheetFieldSlot(c *ui.Context) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	slot := ui.Row(c).MinHeight(t.Space(7)).AlignItems(ui.Center).
		Padding(0, t.Space(2)).Radius(tokens.RadiusSM).
		Border(1, tokens.ControlEdge).Background(tokens.Control)
	// P12-c（§6.2 第 2-4 行）：弹层的字段槽与设置页的 `fieldFace` 是同一个
	// 材质——同一个应用里字段只有一种面。`LitFace` 的顶受光边 + 底对侧暗线
	// 也是同一对数值，不另起一套。
	return tokens.LitFace(slot)
}

// sheetTextField draws the overlay's own text control (R59,
// `plugin-config.css:97-118`). It is deliberately not the framework's
// `ui.TextInput`: the reference's field is a 28u slot cut into the glass — a
// `--glass-field` fill, a `--glass-control-edge` hairline, the `--radius-sm`
// corner, the body step and 8u of side padding, with the field's hairline and
// inset shadow telling the user the value lives inside it. The input itself is
// the framework's look-free base, so its own padding, surface and border do
// not fight the slot.
func (a *App) sheetTextField(c *ui.Context, placeholder string, value *string) ui.Element {
	slot := a.sheetFieldSlot(c).FillWidth()
	slot.Children(func() {
		ui.TextInputBase(c, value).Placeholder(placeholder).Grow(1)
	})
	return slot
}

// sheetActionButton draws one of the overlay's action buttons (R38,
// `plugin-config.css:223-260`). It is deliberately **not** the framework's
// filled `Button`: the reference's control is a quiet hairline pill that
// spends no accent fill — a 28u height, 10u of side padding, a
// `--glass-control-edge` hairline, a `999px` corner (half the 28u height, so
// the shape is a pill), the `--glass-control` plane and the `--text-caption`
// step in `--text-primary`. The danger variant is the one the destructive
// command wears: `color-mix(in srgb, var(--text-warning) 45%, transparent)` on
// its hairline, `--text-warning` on its ink, and a transparent plane, so it
// reads as the destructive action without becoming the overlay's primary one.
// Hover lifts either face to `--glass-control-hover`.
func (a *App) sheetActionButton(c *ui.Context, label string, danger bool, run func()) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).
		MinHeight(t.Space(7)).
		Padding(0, t.Space(2.5)).
		Radius(tokens.RadiusSM)
	ink := t.Text
	edge := tokens.ControlEdge
	plane := tokens.Control
	if danger {
		ink = t.Warning
		edge = t.Warning.Alpha(0.45)
		plane = ui.Transparent
	}
	// P13-q（`docs/liquid-glass.md` §5）：弹层里的胶囊也有第三档。按下时面沉到
	// `--glass-control-press` 并按 `.extensions-action-button` 的几何收 0.97；
	// `danger` 臂静息的面是透明的，按下时才有一块能被压下去的面（白线仍只在
	// 有面时画，§1 禁令 3）。
	pressed := button.Pressed()
	if pressed {
		plane = tokens.ControlPress
	} else if button.Hovered() {
		plane = tokens.ControlHover
	}
	button.Border(1, edge).Background(plane)
	// P13-p（`docs/liquid-glass.md` §6.2b-4 第 9-20 行）：弹层里的一切是 E1（§2 判据 1），
	// 而这枚胶囊是本页唯一没戴那一对线的——同族的 `extensionActionFace`（9-1）、
	// `barButton`（9-5）、卸载对话框的动作胶囊（9-6）都戴了。受光边只在**有面**的
	// 时候画：`danger` 臂静息的面是透明的（`ui.Transparent`），在一块透明的面上
	// 画一条 0.09 的白线就是 §1 禁令 3 的「类玻璃边」；指针把面点亮之后它才有一块
	// 能站起来的玻璃（悬停档与中性臂同一个面）。外圈的发丝一个字节不动。
	if plane.A > 0 {
		tokens.LitFace(button)
	}
	if pressed {
		button = button.Scale(0.97, 0.97)
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(tokens.Caption).TextColor(ink)
	})
	if button.Clicked() {
		run()
	}
	return button
}

// sheetAction is the two-step button: the first press arms it (the button
// becomes the confirm question, with the way back beside it), the second
// runs the command. The status line is the sheet's own failure caption.
func (a *App) sheetAction(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	t := c.Theme()
	armed := sheet.Armed == field.Key
	// The row's own label is a hit target, as it is for every other kind: it
	// arms the button, so the whole row answers the first press.
	clicked := a.row(c, field.Label, field.Help, func() {
		// The reference's cluster: inline-flex, flex-end, a 6u gap
		// (`.plugin-config-action`, `plugin-config.css:218-224`).
		ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Wrap().Children(func() {
			if armed {
				// The armed button is the old build's danger button: the
				// same quiet hairline pill, its own warning face.
				a.sheetActionButton(c, field.Confirm, true, func() { sheet.Run(field.Key) })
				a.sheetActionButton(c, field.Cancel, false, func() { sheet.Unarm(field.Key) })
				return
			}
			if sheet.ActionFailed == field.Key && field.Failed != "" {
				// The refused run's line sits beside the button, not in the
				// sheet's write-failure slot: the old build's
				// `.plugin-config-action__status`, the field's own wording.
				ui.Text(c, field.Failed).FontSize(a.tokens(c).Caption).TextColor(t.Warning)
			}
			a.sheetActionButton(c, field.Label, true, func() { sheet.Arm(field.Key) })
		})
	})
	if clicked && !armed {
		sheet.Arm(field.Key)
	}
}

// optionsOf converts a schema's choices to the settings pages' option shape,
// so one dropdown helper serves both.
func optionsOf(options []plugincfg.Option) []i18n.Option {
	out := make([]i18n.Option, 0, len(options))
	for _, option := range options {
		out = append(out, i18n.Option{ID: option.Value, Label: option.Label})
	}
	return out
}

// boundedText is a bounded value with its unit: "300 items", "30 days".
func boundedText(field plugincfg.Field, value float64) string {
	text := strconv.Itoa(int(math.Round(value)))
	if field.Unit != "" {
		text += " " + field.Unit
	}
	return text
}

// quantize rounds a dragged value to the schema's step, so a slider that
// steps by ten lands on round numbers.
func quantize(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return math.Round(value/step) * step
}

// formatFieldNumber is the number a number field edits: the shortest decimal
// spelling of the value, so "30" shows as "30" and not "30.000000".
func formatFieldNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// clampNumber holds a typed value inside the schema's bounds, as the
// framework's number input clamped a stepped one.
func clampNumber(field plugincfg.Field, value float64) float64 {
	if value < field.Min {
		return field.Min
	}
	if value > field.Max {
		return field.Max
	}
	return value
}

func stringsOf(value any) []string {
	list, _ := value.([]string)
	return list
}

// language is the stored interface language, for the copy the sheet owns.
func (a *App) language() string { return a.settings().Language }
