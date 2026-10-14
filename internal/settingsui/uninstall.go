package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The componentized uninstall: which parts of an installed integration to
// remove. The program always goes — the dialog says so on a checked, disabled
// row — and the three data categories are checkboxes, with the old build's
// rule that selecting all of them removes the data directory as a whole.

// uninstallComponentsRow draws the uninstall controls for one integration: the
// button that expands the componentized dialog (each of the integration's
// parts, the program included, named before anything is removed).
func (a *App) uninstallComponentsRow(c *ui.Context, copy i18n.Settings, integration Integration) {
	if a.Actions.UninstallComponents == nil || integration.Orphan {
		return
	}
	if a.uninstallOpen[integration.ID] {
		a.uninstallDialog(c, copy, integration)
		return
	}
	if ui.Button(c, integration.removalLabel(copy)+"…").Clicked() {
		if a.uninstallOpen == nil {
			a.uninstallOpen = map[string]bool{}
		}
		a.uninstallOpen[integration.ID] = true
	}
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

// uninstallDialog draws the componentized uninstall: the reference's own
// title ("Uninstall {name}"), its hint, one row per component, and the two
// actions in the reference's order — Cancel first, the danger button last
// (`ComponentizedUninstallDialog.tsx:119-141`).
func (a *App) uninstallDialog(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := c.Theme()
	if a.uninstallDraft == nil {
		a.uninstallDraft = map[string]UninstallComponents{}
	}
	draft, ok := a.uninstallDraft[integration.ID]
	if !ok {
		draft = UninstallComponents{RemoveHostConfig: true, RemoveToolData: true, RemoveArtifacts: true}
		a.uninstallDraft[integration.ID] = draft
	}
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(0, t.Space(2), 0, 0).Children(func() {
		ui.Text(c, copy.IntegrationsUninstallTitle(integration.Name)).
			FontSize(t.FontSize).Bold()
		ui.Text(c, copy.IntegrationsUninstallDescription).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		for _, part := range uninstallParts(copy, &draft) {
			a.uninstallPartRow(c, part)
		}
		ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, copy.PermissionsCancel).Clicked() {
				a.uninstallOpen[integration.ID] = false
				return
			}
			if ui.Button(c, integration.removalLabel(copy)).Clicked() {
				if a.Actions.UninstallComponents != nil {
					a.Actions.UninstallComponents(integration.ID, integration.Name, integration.removalKind(), draft)
				}
				a.uninstallOpen[integration.ID] = false
				return
			}
		})
	})
	a.uninstallDraft[integration.ID] = draft
}

// uninstallPartRow is one labeled component inside the uninstall dialog: a
// card with the box, the category's name and the line that says what
// removing it costs — the reference's `.extensions-uninstall-component`. The
// program row is drawn checked and disabled (`uninstallProgram`'s hint is the
// disclosure); the other three are the user's choice. The hint used to be
// dropped; a data category the user cannot read about is a category they
// cannot choose deliberately.
func (a *App) uninstallPartRow(c *ui.Context, part uninstallPart) {
	t := c.Theme()
	tokens := a.tokens(c)
	on := true
	if part.Value != nil {
		on = *part.Value
	}
	changed := false
	ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(ui.Start).
		Padding(t.Space(3)).Radius(tokens.RadiusSM).
		Border(1, tokens.ControlEdge).Background(tokens.Control).Children(func() {
		box := ui.Checkbox(c, &on, "").Label(part.Label)
		if part.Fixed {
			box.Disabled(true)
		} else if box.Changed() {
			changed = true
		}
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, part.Label).FontSize(t.FontSize + 1).FontWeight(600).TextColor(tokens.TextStrong)
			if part.Hint != "" {
				ui.Text(c, part.Hint).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
		})
	})
	if changed && part.Value != nil {
		*part.Value = on
	}
}
