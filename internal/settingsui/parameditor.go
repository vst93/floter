package settingsui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// The declared-inputs editor: the reference's `.extension-param-editor`
// (`main:src/extensions/CustomIntegrationDrawer.tsx:81-104`, styles
// `main:src/styles/extensions.css:1854-1883`).
//
// The reference is deliberately *not* a stack of settings rows. A parameter is
// a small record, so each one is a hairline plate holding an auto-fit grid of
// compact labelled fields (label, id, kind, placeholder, default, flag), then
// one controls line — a 14px required checkbox at its start, a 28px remove key
// at its end — then the choice options when the kind is `select`, and the row's
// own refusal under the fields that caused it. The heading above the plates is
// the *argument editor's* heading: a caption-step title at the start and a 28px
// add key at the end (`__heading` / `__add`, `:1855-1856`), not a settings
// section title with a text action.
//
// The port drew every parameter as a card row whose label was "Input 1" and
// whose six fields were six more card rows under it — a form one column wide
// and six rows deep per parameter, with the required toggle in the row's
// trailing slot. This file restores the reference's record shape; the panel's
// own literal px metrics come through `cardTheme`/`cardSpace`, so the plate,
// the fields and the checkbox keep the reference's numbers at every interface
// step.

// paramEditorHeading is `.extension-argument-editor__heading`'s floor (20px)
// and the add key's box (`.extension-argument-editor__add`, 28px).
const (
	paramEditorHeadingMin = 5    // 20u
	paramEditorIconBox    = 7    // 28u
	paramEditorGlyph      = 3.25 // 13u
	paramEditorFieldMin   = 35   // 140u
	paramEditorPlatePad   = 2    // 8u
	paramEditorPlateGap   = 1.5  // 6u
	paramEditorCheckBox   = 3.5  // 14u
	paramEditorCheckGap   = 1    // 4u
	paramEditorOptionsMin = 16   // 64u
	paramEditorEmptyMin   = 7    // 28u
)

// paramEditorMono is the reference's textarea stack in the custom form
// (`.extension-custom-form textarea { font-family: "SF Mono", Menlo, … }`,
// `extensions.css:1786-1792`): one option per line is code, not prose.
const paramEditorMono = "SF Mono, Menlo, Monaco, Consolas, monospace"

// toolParamsEditor edits the declared inputs of a connection: the reference's
// argument-editor heading with its add key, one caption hint line, and then one
// plate per parameter. A flag that is not one argv token is refused at connect
// time, so the editor does not police the spelling here.
func (a *App) toolParamsEditor(c *ui.Context, copy i18n.Settings) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)

	// `.extension-argument-editor__heading`: the list's title at the caption
	// step (600, muted) with the add key at its end. The key is the reference's
	// 28px icon button, not a text action — a list you add rows to keeps one
	// vocabulary with the argument editor beside it.
	heading := ui.Row(c).FillWidth().MinHeight(t.Space(paramEditorHeadingMin)).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(t.Space(2))
	heading.Children(func() {
		ui.Text(c, copy.ParamsTitle).FontSize(tokens.Caption).FontWeight(600).
			TextColor(t.TextMuted)
		add := a.paramIconButton(c, "plus", copy.ParamsAdd)
		if add.Clicked() {
			a.toolParams = append(a.toolParams, toolParam{Kind: extensions.ParamText})
		}
	})
	// `.extension-param-editor__hint`: one caption line under the heading, in
	// the tertiary ink at 1.35 (`extensions.css:1869`).
	if hint := copy.ParamsHint; hint != "" {
		ui.Text(c, hint).FontSize(tokens.Caption).TextColor(tokens.TextTertiary).Wrap()
	}

	if len(a.toolParams) == 0 {
		// `.extension-argument-editor__empty` (`:1860`): a dashed hairline box,
		// 8px of padding and the tertiary caption — the same "a list with no
		// rows yet" face the argument editor uses.
		empty := ui.Column(c).FillWidth().MinHeight(t.Space(paramEditorEmptyMin)).
			Padding(t.Space(paramEditorPlatePad)).Radius(tokens.RadiusSM).
			Border(1, tokens.Hairline).BorderStyle(ui.BorderDashed)
		empty.Children(func() {
			ui.Text(c, copy.ParamsEmpty).FontSize(tokens.Caption).
				TextColor(tokens.TextTertiary)
		})
		return
	}

	// The declared rows' own validation, answered before a save asks for it:
	// the line prints inside the plate that still needs it, so a typo is named
	// where it happened rather than bounced back from the backend.
	issues := extensions.ParamRowIssues(a.editorParams())
	removed := -1
	ui.Column(c).FillWidth().Gap(t.Space(paramEditorPlateGap)).Children(func() {
		for index := range a.toolParams {
			index := index
			param := &a.toolParams[index]
			if a.paramPlate(c, copy, param, issues[index]) {
				removed = index
			}
		}
	})
	if removed >= 0 {
		a.toolParams = append(a.toolParams[:removed], a.toolParams[removed+1:]...)
	}
}

// paramPlate draws one parameter definition: the hairline record plate with its
// fields grid, its controls line, the choice options and the row's refusal. It
// reports whether the remove key was pressed this frame.
func (a *App) paramPlate(c *ui.Context, copy i18n.Settings, param *toolParam, issue extensions.ParamRowIssue) bool {
	t := a.cardTheme(c)
	tokens := a.tokens(c)

	// The row's working copies: every control binds one, and the plate writes
	// them back once, so a keystroke and a click in the same frame cannot
	// overwrite each other.
	id, label, flag, options := param.ID, param.Label, param.Flag, param.Options
	placeholder, def := param.Placeholder, param.Default
	kind := param.Kind
	required := param.Required
	removed := false

	plate := ui.Column(c).FillWidth().Gap(t.Space(paramEditorPlateGap)).
		Padding(t.Space(paramEditorPlatePad)).Radius(tokens.RadiusSM).
		Border(1, tokens.Hairline)
	plate.Children(func() {
		// `.extension-param-editor__fields`: `repeat(auto-fit, minmax(140px,
		// 1fr))` with a `6px 8px` gap. A wrap row of 140u-minimum growing cells
		// is the same layout in this framework: a full line of cells, and a lone
		// last cell taking the line's remainder.
		ui.Row(c).FillWidth().Wrap().GapX(t.Space(2)).GapY(t.Space(paramEditorPlateGap)).
			Children(func() {
				a.paramField(c, copy.ParamsLabel, func() {
					a.extensionField(c, &label, customFieldMetrics).Label(copy.ParamsLabel)
				})
				a.paramField(c, copy.ParamsID, func() {
					a.extensionField(c, &id, customFieldMetrics).Label(copy.ParamsID).Placeholder("target")
				})
				a.paramField(c, copy.ParamsKind, func() {
					labels := paramKindLabels(copy)
					chosen := runParamKindLabel(copy, kind)
					a.paramSelect(c, &chosen, labels, copy.ParamsKind)
					if chosen != runParamKindLabel(copy, kind) {
						kind = paramKindFromLabel(copy, chosen)
					}
				})
				// The reference hides the placeholder field for a boolean: its
				// value is its flag, so a field hint means nothing.
				if kind != extensions.ParamBoolean {
					a.paramField(c, copy.ParamsPlaceholder, func() {
						a.extensionField(c, &placeholder, customFieldMetrics).Label(copy.ParamsPlaceholder)
					})
				}
				a.paramField(c, copy.ParamsDefault, func() {
					a.extensionField(c, &def, customFieldMetrics).Label(copy.ParamsDefault)
				})
				a.paramField(c, copy.ParamsFlag, func() {
					a.extensionField(c, &flag, customFieldMetrics).Label(copy.ParamsFlag).Placeholder("--target")
				})
			})
		// `.extension-param-editor__controls`: the required checkbox at the
		// start and the remove key at the end (`:1880`).
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Gap(t.Space(2)).Children(func() {
			a.paramRequiredToggle(c, copy, &required)
			if a.paramIconButton(c, "close", copy.ParamsRemove).Clicked() {
				removed = true
			}
		})
		if kind == extensions.ParamSelect {
			// `.extension-param-editor__options` (`:1875-1879`): one option per
			// line in a 64px-floor multi-line box, so a choice may contain a
			// comma (`1,000`) without being split by the codec.
			ui.Column(c).FillWidth().Gap(t.Space(0.75)).Children(func() {
				ui.Text(c, copy.ParamsOptions).FontSize(tokens.Caption).FontWeight(600).
					TextColor(t.TextMuted)
				box := ui.TextArea(c, &options).Label(copy.ParamsOptions).
					FillWidth().MinHeight(t.Space(paramEditorOptionsMin)).
					Radius(tokens.RadiusSM).Font(paramEditorMono)
				a.fieldFace(c, box)
			})
		}
		if message := paramIssueMessage(copy, issue); message != "" {
			a.paramEditorError(c, message)
		}
	})
	param.ID, param.Label, param.Flag, param.Options = id, label, flag, options
	param.Placeholder, param.Default = placeholder, def
	param.Kind, param.Required = kind, required
	return removed
}

// paramField is one cell of the plate's fields grid: the caption label above
// its control, a 3px stack (`extensions.css:1872`: `display:grid; gap:3px`).
func (a *App) paramField(c *ui.Context, label string, field func()) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ui.Column(c).MinWidth(t.Space(paramEditorFieldMin)).Grow(1).
		Gap(t.Space(0.75)).Children(func() {
		ui.Text(c, label).FontSize(tokens.Caption).FontWeight(600).
			TextColor(t.TextMuted).SingleLine()
		field()
	})
}

// paramSelect is one of the plate's drop-downs: the reference's
// `.extension-custom-form select` (`extensions.css:1783-1789`) — the field
// material in the form's pill shape and its 30px/3px/7px box.
func (a *App) paramSelect(c *ui.Context, selected *string, labels []string, label string) ui.Element {
	e := ui.Select(c, selected, labels).Label(label).FillWidth().Radius(999)
	if a.cardUnit > 0 {
		m := customFieldMetrics
		e = e.MinHeight(m.minHeight).Padding(m.padY, m.padX)
	}
	return a.fieldFace(c, e)
}

// paramRequiredToggle is `.extension-param-editor__toggle` (`:1881-1882`): a
// 14px checkbox with the accent fill and a 4px gap to its caption word. The
// framework's `ui.Checkbox` draws a 16u box with an 8u gap, which is the
// settings rows' size, not the panel's literal one — so the box is drawn here
// and `CheckboxBase` keeps the click, the Space key and the role.
func (a *App) paramRequiredToggle(c *ui.Context, copy i18n.Settings, required *bool) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	on := *required
	row := ui.CheckboxBase(c, required).Label(copy.ParamsRequired).
		Gap(t.Space(paramEditorCheckGap)).AlignItems(ui.Center)
	row.Children(func() {
		box := ui.Box(c).Size(t.Space(paramEditorCheckBox), t.Space(paramEditorCheckBox)).Shrink(0)
		if on {
			box.Background(t.Accent)
			box.DrawOver(func(p *ui.Painter, r ui.Rect) {
				var check ui.Path
				check.MoveTo(r.X+r.W*0.22, r.Y+r.H*0.52)
				check.LineTo(r.X+r.W*0.43, r.Y+r.H*0.73)
				check.LineTo(r.X+r.W*0.79, r.Y+r.H*0.29)
				p.StrokePath(&check, 1.5, t.AccentText)
			})
		} else {
			box.Background(tokens.Field).Border(1, tokens.ControlEdge)
		}
		ui.Text(c, copy.ParamsRequired).FontSize(tokens.Caption).FontWeight(500).
			TextColor(tokens.TextSecondary)
	})
}

// paramIconButton is `.extensions-icon-button` in the argument editor's own
// 28px box (`extensions.css:1856,1859`): a transparent square with the
// panel's 9px corner and a 13px mark, lighting the hover fill and the strong
// ink under the pointer.
func (a *App) paramIconButton(c *ui.Context, glyphName, label string) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).Label(label).
		Size(t.Space(paramEditorIconBox), t.Space(paramEditorIconBox)).
		Border(1, ui.Transparent).Radius(tokens.RadiusSM).
		Background(ui.Transparent).Center().
		Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	ink := tokens.TextSecondary
	if button.Hovered() {
		ink = t.Text
		button = button.Border(1, tokens.ControlEdge).Background(tokens.ControlHover)
		tokens.Lens(button)
	}
	button.Children(func() {
		ui.Icon(c, mustGlyph(glyphName)).Size(t.Space(paramEditorGlyph), t.Space(paramEditorGlyph)).
			TextColor(ink)
	})
	return button
}

// paramEditorError is `.extension-param-editor__error` (`:1883`): the warning
// caption behind a 12px alert mark, in the plate's own column rather than in a
// padded row line.
func (a *App) paramEditorError(c *ui.Context, message string) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.25)).Children(func() {
		ui.Icon(c, mustGlyph("alert")).Size(12, 12).TextColor(t.Warning)
		ui.Text(c, message).FontSize(tokens.Caption).TextColor(t.Warning).Wrap()
	})
}

// paramKindFromLabel is the picker's answer back to a kind id: `ui.Select`
// binds the option's own word, so the plate maps the chosen label through the
// same list the picker was handed.
func paramKindFromLabel(copy i18n.Settings, label string) string {
	for index, word := range paramKindLabels(copy) {
		if word == label {
			return paramKinds[index]
		}
	}
	return extensions.ParamText
}
