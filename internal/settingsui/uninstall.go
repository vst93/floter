package settingsui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// modalDialog shows a dialog over the window while *open is true, with the
// reference's own modal contract: Escape closes it, but a press on the scrim
// does not. It is the framework's own DialogBase shape — an `Absolute`
// backdrop at inset 0 that centers the panel, marked `Modal()` so the focus
// moves in, Tab stays inside and the window's shortcuts behind it wait — with
// one change: the scrim press is swallowed rather than answered.
//
// DialogBase itself cannot serve the second half: its backdrop is
// `dismissed := back.Clicked() || back.OverlayShortcut(0, KeyEscape)`
// (`mygo@v0.5.1/ui/base.go:481`), so the two gestures are indistinguishable
// once the call returns. The reference's overlay carried `role="dialog"` /
// `aria-modal` and no `onClick` at all
// (`ComponentizedUninstallDialog.tsx:55-63`): a press outside the panel did
// nothing, and only Escape or an explicit answer left. For a destructive
// question that difference is the whole point — one stray press must not
// answer "cancel".
//
// The press is still swallowed, not passed on: the backdrop is the topmost
// element under the pointer, so the hit chain never reaches the row's button
// behind the overlay. The returned `dismissed` reports the one gesture that
// does close it.
func (a *App) modalDialog(c *ui.Context, open *bool, fn func(backdrop, panel ui.Element)) (panel ui.Element, dismissed bool) {
	if !*open {
		return ui.Element{}, false
	}
	ui.Overlay(c, func() {
		back := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().Modal()
		if back.OverlayShortcut(0, ui.KeyEscape) {
			*open = false
			dismissed = true
		}
		back.Children(func() {
			panel = ui.Box(c).Role(ui.RoleDialog)
			panel.Children(func() { fn(back, panel) })
		})
	})
	return panel, dismissed
}

// checkIcon is the reference's confirm-button glyph (`<Check size={14}
// strokeWidth={2} />` in `ComponentizedUninstallDialog.tsx:160`): a stroked
// 24×24 path, the mark every destructive confirm in the family leads with, so a
// confirm is recognized before its word is read.
var checkIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`))

// The componentized uninstall: which parts of an installed integration to
// remove. The program always goes — the dialog says so on a checked, disabled
// row — and the three data categories are checkboxes, with the old build's
// rule that selecting all of them removes the data directory as a whole.

// uninstallComponentsRow draws the uninstall controls for one integration:
// when the componentized dialog is open it is the dialog itself — the door
// into it is the kind's own removal bar (`requestRemove` →
// `removeConfirmation`), whose confirm opens this, exactly as the reference's
// `confirmRemoval` routed npm and custom on to the dialog
// (`src/ExtensionsPanel.tsx:1286-1304`).
func (a *App) uninstallComponentsRow(c *ui.Context, copy i18n.Settings, integration Integration) {
	if !a.uninstallOpen[integration.ID] {
		return
	}
	a.uninstallDialog(c, copy, integration)
}

// uninstallParts is the dialog's four rows in the reference's own order:
// program files, host configuration, tool data, generated artifacts
// (`ComponentizedUninstallDialog.tsx:84-115`). Program files are always
// removed, so that row carries no value and is drawn checked and disabled —
// the reference's own first row (`:84-98`), whose hint is how the dialog
// discloses that the program always goes. The other three point at the
// draft's fields, so the drawing code never has to know which is which.
type uninstallPart struct {
	Label string
	Hint  string
	// Fixed marks the program row: always removed, with no value to toggle.
	Fixed bool
	// Value is the draft field a removable row writes; nil for the fixed row.
	Value *bool
}

func uninstallParts(copy i18n.Settings, draft *UninstallComponents) []uninstallPart {
	return []uninstallPart{
		{Label: copy.UninstallProgram, Hint: copy.UninstallProgramHint, Fixed: true},
		{Label: copy.UninstallHostConfig, Hint: copy.UninstallHostConfigHint, Value: &draft.RemoveHostConfig},
		{Label: copy.UninstallToolData, Hint: copy.UninstallToolDataHint, Value: &draft.RemoveToolData},
		{Label: copy.UninstallArtifacts, Hint: copy.UninstallArtifactsHint, Value: &draft.RemoveArtifacts},
	}
}

// uninstallDialog draws the componentized uninstall. The reference floats it
// over a full-window scrim — `.extensions-dialog-overlay` is a fixed inset-0
// grid at `z-index: 40`, `place-items: center`, 20px of padding and
// `background: var(--surface-scrim)` (`ComponentizedUninstallDialog.css:6-16`)
// — so DialogBase supplies that backdrop, the capture-phase Escape and the
// focus restore on close, and the pane below carries the reference's own
// `.extensions-dialog` look. The pane is a 17px-padded column between
// `--input-stroke` and `--glass-float`, rounded `--ext-radius` and thrown on
// `--elev-3` (`:17-34`), and the uninstall variant clamps it to the
// reference's own 420–480px band (`.extensions-uninstall-dialog`, `:36-39`).
// Inside it: the reference's own title ("Uninstall {name}"), its hint, one row
// per component, and the two actions in the reference's order — Cancel first,
// the danger button last, at the row's right end
// (`ComponentizedUninstallDialog.tsx:119-141`).
//
// The dialog's blocks are not one even gutter: the reference hangs the hint
// 6px under the title (`.extensions-dialog-hint { margin: 6px 0 0 }`, `:46-51`),
// the component list 16px under both (`.extensions-uninstall-components
// { margin: 16px 0 }`, `:53-58`), and the actions at the list's own 16px
// bottom margin. One uniform gap drew all four blocks 4px apart, which is
// neither of the reference's two numbers.
//
// `open` is true because the caller (`uninstallComponentsRow`) already gates
// the dialog on `uninstallOpen`; DialogBase needs a pointer so that Escape or
// a press on the scrim can close it, and Dismissed reports that back so the
// map is cleared. The cancel button takes the focus on mount
// (`cancelRef.current?.focus()`), which DialogBase restores to the opener on
// close. The returned element is the dialog's own box, so a test can read the
// padding and the width band off it (the pattern `sectionMaterial` and `action`
// use).
func (a *App) uninstallDialog(c *ui.Context, copy i18n.Settings, integration Integration) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	if a.uninstallDraft == nil {
		a.uninstallDraft = map[string]UninstallComponents{}
	}
	draft, ok := a.uninstallDraft[integration.ID]
	if !ok {
		draft = UninstallComponents{RemoveHostConfig: true, RemoveToolData: true, RemoveArtifacts: true}
		a.uninstallDraft[integration.ID] = draft
	}
	// busy is the reference's own prop: both actions carry `disabled={busy}`
	// while a removal is in flight (`ComponentizedUninstallDialog.tsx:126,142`).
	busy := a.uninstallBusy[integration.ID]
	open := true
	// pane and cancel are the focus bookkeeping: the reference gives the
	// cancel button the focus on mount, and the request must land after the
	// overlay has recorded its opener, or closing would restore the focus to
	// the dialog itself. Binding the pane lets the request stop once the
	// focus is in it, so Tab still reaches the confirm button.
	var pane, cancel ui.Handle
	panelEl, dismissed := a.modalDialog(c, &open, func(backdrop, panel ui.Element) {
		// `.extensions-dialog-overlay { inset: 0; place-items: center; padding:
		// 20px; background: var(--surface-scrim) }` (`:6-16`). The scrim is the
		// reference's own `rgba(0, 0, 0, 0.34)` (`base.css:699`) in every theme.
		backdrop.Background(ui.RGBA(0, 0, 0, 0.34)).Padding(t.Space(5))
		// `.extensions-dialog` (`:17-34`): the pane's own material and edge.
		// `max-height: calc(100% - 40px)` and `color: var(--text-primary)`
		// (`:17-34`): the overlay's own 20px padding on each side makes 100% of
		// its content box exactly that expression, so the pane can never grow
		// past the window it centers in.
		panel.FillWidth().MaxHeightPercent(100).
			Padding(t.Space(4.25)).MinWidth(t.Space(105)).MaxWidth(t.Space(120)).
			Radius(tokens.RadiusSM).Border(1, tokens.InputStroke).
			Background(tokens.Float).TextColor(t.Text).Bind(&pane)
		// The floater's throw and its lit top rim, as `--elev-3` composed them —
		// the same two shadows the terminal panel's own note carries.
		// P11-an：两条 throw 都乘 `--elev-shadow-scale`（`theme.ElevThrow`），
		// 于是透明度滑块往下拉时，对话框的投影跟着变轻。
		theme.ElevThrow(panel, t, a.settings().MainOpacity)
		// P12-c（§6.2 第 2-7 行）：这层浮层此前戴的是 **E1 的** `LensRim`（0.09）。
		// `LensRim` 是「长在底上的面」的冠线，而这张面板是 **E2**——它压在其他内容
		// 上、带着两层影、还能关掉（§2 三条判据里的第三条）。E2 的冠线是
		// `EdgeHighlight`（深 0.20 / 浅 0.92）：比 E1 的线亮一档，因为它是浮起来的
		// 那张纸的边，而不是贴着的那个块的边。两层影 `ElevThrow` 已经在场，这一
		// 行只换线。
		panel.InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)
		// `overflow-y: auto` (`:17-34`): a short window scrolls the questions
		// instead of pushing the actions outside the pane. The scroll viewport
		// is the pane's content box, so the 17px padding stays put exactly as
		// the reference's scroll container keeps it.
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			// `.extensions-dialog-title { color: var(--text-strong); font-size: 13px }`
			// (`:41-45`): the ladder's title step, not the body step this dialog
			// used to draw its heading at.
			ui.Text(c, copy.IntegrationsUninstallTitle(integration.Name)).
				FontSize(tokens.Title).Bold().TextColor(tokens.TextStrong)
			// `.extensions-dialog-hint { margin: 6px 0 0; font-size: 10.5px;
			// line-height: 1.45 }` (`:46-51`).
			ui.Box(c).FillWidth().Padding(t.Space(1.5), 0, 0, 0).Children(func() {
				ui.Text(c, copy.IntegrationsUninstallDescription).
					FontSize(uninstallSubBody(t)).TextColor(t.TextMuted).LineHeight(1.45)
			})
			// `.extensions-uninstall-components { gap: 8px; margin: 16px 0 }`.
			ui.Column(c).FillWidth().Gap(t.Space(2)).Padding(t.Space(4), 0).Children(func() {
				for _, part := range uninstallParts(copy, &draft) {
					a.uninstallPartRow(c, part)
				}
			})
			// A failed removal leaves the pane open with the failure on itself:
			// the reference kept the dialog mounted on error and told the user
			// through the app's notice stack
			// (`showError(errorMessage(error))`, `src/ExtensionsPanel.tsx:1314-1341`).
			// On this surface the same sentence rides the family's own error band
			// (`.extensions-notice--error`, `src/styles/extensions.css:174-178`).
			if message := a.uninstallError[integration.ID]; message != "" {
				a.noticeBand(c, true, message, func(*ui.Element) {})
			}
			// `.extensions-dialog-actions { justify-content: flex-end; gap: 7px }`
			// (`:74-78`): the pair sits at the row's right end, seven units apart.
			ui.Row(c).FillWidth().Gap(t.Space(1.75)).Justify(ui.End).AlignItems(ui.Center).Children(func() {
				// The reference focuses Cancel on mount
				// (`cancelRef.current?.focus({ preventScroll: true })`), the one
				// target that keeps the answer to "remove all of this?" a deliberate
				// one. Both actions carry the reference's `busy` prop, so a question
				// already being answered cannot be answered again.
				if a.uninstallActionButton(c, copy.PermissionsCancel, false, busy, nil).Bind(&cancel).Clicked() {
					a.uninstallOpen[integration.ID] = false
					return
				}
				// The confirm verb is the reference's own
				// `settings.extensions.uninstall` — one word for every kind the
				// dialog serves (`ComponentizedUninstallDialog.tsx:161`), not the
				// per-kind verb the row and the bar carry.
				if a.uninstallActionButton(c, copy.IntegrationsUninstall, true, busy, checkIcon).Clicked() {
					if a.Actions.UninstallComponents == nil {
						// The dialog only opens when the shell wired the componentized
						// action, so this arm is the headless-test fallback.
						a.uninstallOpen[integration.ID] = false
						return
					}
					if a.uninstallBusy == nil {
						a.uninstallBusy = map[string]bool{}
					}
					if a.uninstallError != nil {
						delete(a.uninstallError, integration.ID)
					}
					a.uninstallBusy[integration.ID] = true
					id := integration.ID
					// The dialog stays up until the shell reports back: the reference
					// awaited the mutation and only then closed the dialog, so the
					// busy face is visible for exactly as long as the work runs.
					a.Actions.UninstallComponents(id, integration.Name, integration.removalKind(), draft, func(err error) {
						a.finishComponentizedUninstall(id, err)
					})
					if a.Actions.Invalidate != nil {
						a.Actions.Invalidate()
					}
					return
				}
			})
		})
	})
	if dismissed {
		if a.uninstallOpen == nil {
			a.uninstallOpen = map[string]bool{}
		}
		a.uninstallOpen[integration.ID] = false
	}
	// Give Cancel the focus while the pane does not already hold it: the
	// request applies after the overlay recorded its opener, so closing gives
	// the focus back to the control that opened the dialog.
	if panelEl.Valid() && !pane.FocusWithin(c) {
		cancel.Focus(c)
	}
	a.uninstallDraft[integration.ID] = draft
	return panelEl
}

// finishComponentizedUninstall settles the dialog after the shell reports a
// componentized removal back on the main thread. A nil error is the
// reference's success arm: the dialog closes (the reference dropped
// `uninstallDialogTarget` only after `await` and raised its own notice, which
// the shell does here). A non-nil error keeps the dialog up with the failure
// on it and both answer pills usable again — the reference's `runMutation`
// `finally` cleared `busy` either way, but nothing unmounted the dialog
// (`src/ExtensionsPanel.tsx:1314-1341`).
func (a *App) finishComponentizedUninstall(id string, err error) {
	delete(a.uninstallBusy, id)
	if err != nil {
		if a.uninstallError == nil {
			a.uninstallError = map[string]string{}
		}
		a.uninstallError[id] = err.Error()
	} else {
		if a.uninstallError != nil {
			delete(a.uninstallError, id)
		}
		if a.uninstallOpen == nil {
			a.uninstallOpen = map[string]bool{}
		}
		a.uninstallOpen[id] = false
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// uninstallActionButton draws one of the dialog's two actions as the
// reference's `.extensions-action-button` (`extensions.css:1313-1385`): a
// standalone pill at the family's 30px control height, 10px of side padding,
// a `--stroke-control` hairline, the `--glass-control` plane and `--elev-0`,
// with the `--extensions-inline-gap` between its mark and its word, and
// `--text-body` at 600 in `--text-strong`. It is deliberately not the
// framework's filled `Button`, which spends the surface fill, pads 14px a
// side, rounds 9px and weights 400 — four numbers the reference does not
// write. The danger variant is the *ink* alone, `--text-warning`, not a red
// plane: the reference's destructive action is quieter than the framework's.
// Hover lifts the plane to `--glass-control-hover`; the hairline stays the
// neutral `--stroke-control` (P8-d: it used to rise to `--accent-edge`, which
// §5 forbids on hover — see the note at the hover arm).
//
// `icon` leads the word when it is not nil — `<Check size={14} strokeWidth={2} />`
// is the reference's first child, so the mark a destructive confirm is
// recognized by comes before the word it confirms. `busy` is the reference's
// own prop: both actions carry `disabled={busy}`
// (`ComponentizedUninstallDialog.tsx:126,142`), so a question already being
// answered cannot be answered again.
func (a *App) uninstallActionButton(c *ui.Context, label string, danger bool, busy bool, icon *ui.SVG) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	// The pill's corner is half the 30px control height (the reference writes
	// `border-radius: 999px`, which the browser clamps to exactly that). The
	// reference pins both `height` and `min-height` to the family's 30px
	// control step, so the height is set, not just floored.
	button := ui.ButtonBase(c).Label(label).
		Height(t.Space(7.5)).Padding(0, t.Space(2.5)).Gap(t.Space(1.25)).
		Radius(t.Space(3.75))
	ink, edge, plane := tokens.TextStrong, tokens.ControlEdge, tokens.Control
	if danger {
		ink = t.Warning
	}
	// `:hover:not(:disabled)` (`extensions.css:1340-1344`): a disabled pill
	// keeps its resting face under the pointer.
	//
	// P8-d 重定基线（§5 禁止侧「行 hover/按下」与「弹层危险动作的彩色环」）：
	// hover 曾把边抬到 `tokens.AccentEdge`。accent 在这里只回答「指针在这
	// 条上」，而那既不是选中也不是开启；危险语义由 `t.Warning` 的**墨与
	// 文案**说，不靠 hover 变色相。所以 hover 边仍是 `ControlEdge`，只抬面
	// （`ControlHover`）——与 P8-b 录制器静息 hover 同一个处理；而且危险按钮的
	// 墨在 hover 时**仍是 `t.Warning`**（旧值会把它换成 `t.Text`）：如果 hover
	// 把警告墨换成普通墨，那就等于用「变色相」代替了语义。
	if !busy && button.Hovered() {
		edge, plane = tokens.ControlEdge, tokens.ControlHover
		if !danger {
			ink = t.Text
		}
	}
	button.Border(1, edge).Background(plane).
		Transition(ui.ElementTransition{Colors: true, Position: true, Duration: 120 * time.Millisecond})
	// `.extensions-action-button:active:not(:disabled) { transform: scale(0.97) }`
	// (`extensions.css:138-140`): the press sinks the pill a hair, over the
	// same `--dur-2` transform transition.
	if !busy && button.Pressed() {
		button.Scale(0.97, 0.97)
	}
	if busy {
		// `.extensions-action-button:disabled { opacity: 0.4; cursor: default;
		// pointer-events: none }` (`extensions.css:1369-1373`). The disabled
		// flag carries the pointer-events half; the cursor is the other one the
		// framework does not paint for us.
		//
		// P13-u：那个 `.4` 不再叠上去——框架的画笔已经给 `Disabled` 的子树乘了
		// 0.5（`ui/paint.go:257-259`），两道乘起来是有效 0.2：面离地面只剩 4 级、
		// E1 那一对线双双掉进噪声。同族的 `extensionActionFace`（P13-s）与
		// `busyActionButton` 早就只留一道淡，这一处是同一页上的尾巴。
		button.Disabled(true).Cursor(ui.CursorDefault)
	}
	// The control's own `--elev-0`: the same E1 pair the cards on the surface
	// carry, so the pill reads as a pane of the same glass. `theme.Tokens.LitFace`
	// rather than the four-layer `Lens`: the ring was this pill's own
	// `Border(1, edge)` painted a second time, and it covered the 0.09 rim.
	// P13-u：在飞的面换 `DimFace`——底对侧还在、受光边熄了（受光边是「这块面
	// 可以被碰」的记号，§3），与同族其余禁用面同一句话。
	if busy {
		tokens.DimFace(button)
	} else {
		tokens.LitFace(button)
	}
	button.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(t.Space(3.5), t.Space(3.5))
		}
		// `.extensions-action-button { line-height: 1 }`
		// (`extensions.css:1338`).
		ui.Text(c, label).FontSize(tokens.Body).FontWeight(600).TextColor(ink).LineHeight(1)
	})
	return button
}

// uninstallSubBody is the reference's 10.5px step: the one register below the
// body that `.extensions-dialog-hint` and `.extensions-component-hint` both
// write (`ComponentizedUninstallDialog.css:46-51,113-118`). It is half a unit
// under `--text-body` (11px), so the body step's own value less a half carries
// it at every interface step; the dialog used to draw both hints at the body
// step itself, a half point larger than the reference's line.
func uninstallSubBody(t *ui.Theme) float32 {
	return t.FontSize - 0.5
}

// uninstallPartRow is one labeled component inside the uninstall dialog: a
// card with the box, the category's name and the line that says what
// removing it costs — the reference's `.extensions-uninstall-component`. The
// program row is drawn checked and disabled (`uninstallProgram`'s hint is the
// disclosure); the other three are the user's choice. The hint used to be
// dropped; a data category the user cannot read about is a category they
// cannot choose deliberately.
//
// The card carries the reference's own pointer ladder — hover lifts it to
// `--glass-control-hover` on the `--accent-edge` keyline, the press sinks it
// to `--glass-control-press` — over a 150ms colour transition, and the
// disabled program row is the reference's flat `opacity: 0.6`
// (`ComponentizedUninstallDialog.css:60-83`). The card was a static box: it
// was the one control in the panel that did not answer the pointer. Its own
// `--elev-0` lens rides the raised states, but the reference's press rule sets
// `box-shadow: none` as it sinks (`:74-77`), so the press is the one rung that
// paints the fill without the glass.
//
// The returned element is the card's own box, so a test can read its padding
// and its row gap off it.
func (a *App) uninstallPartRow(c *ui.Context, part uninstallPart) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	on := true
	if part.Value != nil {
		on = *part.Value
	}
	changed := false
	// `.extensions-uninstall-component { padding: 12px; gap: 4px 12px;
	// border-radius: 8px; transition: background 150ms ease, border-color
	// 150ms ease }` (`:60-73`).
	card := ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(ui.Start).
		Padding(t.Space(3)).Radius(t.Space(2)).
		Border(1, tokens.ControlEdge).Background(tokens.Control).
		Transition(ui.ElementTransition{Colors: true, Duration: 150 * time.Millisecond})
	switch {
	case part.Fixed:
		// `:has(input:disabled) { cursor: not-allowed; opacity: 0.6 }`.
		card.Opacity(0.6)
		card.Cursor(ui.CursorNotAllowed)
		tokens.LitFace(card)
	case card.Pressed():
		// The press rung comes first: CSS paints `:active` over `:hover`,
		// and a pointer held down is both. `:active` also drops the card's
		// `--elev-0` shadow, so the pressed pane is the fill alone.
		card.Background(tokens.ControlPress)
	case card.Hovered():
		// P8-d 重定基线（§5 禁止侧）：hover 边曾是 `tokens.AccentEdge`。一张被
		// 勾选的卡片不是「被选中/被打开」的控件状态，hover 更是 §5 明写的禁止
		// 项——边保持 `ControlEdge`，只抬面（`ControlHover`）。
		card.Background(tokens.ControlHover)
		card.Border(1, tokens.ControlEdge)
		tokens.LitFace(card)
	default:
		// E1 on all three rungs (fixed / hover / resting): the card's own
		// `Border(1, ControlEdge)` is its frame, and `LitFace` puts the lit edge
		// above and the shade below — no ring, which was that border a second
		// time and buried the rim (see `rows.go`'s `extensionActionFace`).
		tokens.LitFace(card)
	}
	card.Children(func() {
		// `.extensions-uninstall-component { grid-template-columns: 20px 1fr;
		// gap: 4px 12px }` with the checkbox itself `width: 16px; height:
		// 16px; margin-top: 2px` (`:60-91`). The 20px column is what keeps
		// the label's column aligned across the four rows even though the box
		// is only 16px wide; laying the row out as a flex pair left the label
		// 4px tighter than the reference. The 2px top margin drops the box
		// off the card's top edge exactly as the grid's does.
		box := ui.Checkbox(c, &on, "").Label(part.Label).
			Width(t.Space(5)).Padding(t.Space(0.5), 0, 0, 0)
		if part.Fixed {
			box.Disabled(true)
			// `input[type="checkbox"]:disabled { cursor: not-allowed }`
			// (`:92-94`): the box inside the disabled card carries the rung
			// itself, so the whole row reads as one inert control.
			box.Cursor(ui.CursorNotAllowed)
		} else if box.Changed() {
			changed = true
		}
		ui.Column(c).Grow(1).Gap(t.Space(1)).Children(func() {
			// `.extensions-component-label { font-size: 12px; font-weight: 600 }`
			// — the ladder's emphasis step, not the body step plus one.
			ui.Text(c, part.Label).FontSize(tokens.Emphasis).FontWeight(600).TextColor(tokens.TextStrong)
			if part.Hint != "" {
				// `.extensions-component-hint { line-height: 1.4 }`
				// (`:113-118`).
				ui.Text(c, part.Hint).FontSize(uninstallSubBody(t)).TextColor(t.TextMuted).LineHeight(1.4)
			}
		})
	})
	if changed && part.Value != nil {
		*part.Value = on
	}
	return card
}
