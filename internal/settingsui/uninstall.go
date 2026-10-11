package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The componentized uninstall: which parts of an installed integration to
// remove. The program always goes; the three data categories are checkboxes,
// with the old build's rule that selecting all of them removes the data
// directory as a whole.

// uninstallComponentsRow draws the uninstall controls for one integration: the
// plain button (the program goes, the data stays) and the componentized dialog
// for the three data categories.
func (a *App) uninstallComponentsRow(c *ui.Context, copy i18n.Settings, integration Integration) {
	if a.Actions.UninstallComponents == nil || integration.Orphan {
		return
	}
	if a.uninstallOpen[integration.ID] {
		a.uninstallDialog(c, copy, integration)
		return
	}
	if ui.Button(c, copy.IntegrationsUninstall+"…").Clicked() {
		if a.uninstallOpen == nil {
			a.uninstallOpen = map[string]bool{}
		}
		a.uninstallOpen[integration.ID] = true
	}
}

// uninstallDialog draws the componentized uninstall: a checkbox per data
// category, with the program always removed.
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
		ui.Text(c, copy.IntegrationsUninstallTitle+"  ·  "+integration.Name).
			FontSize(t.FontSize).Bold()
		ui.Text(c, copy.IntegrationsUninstallDescription).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		a.setCheckbox(c, copy.UninstallHostConfig, copy.UninstallHostConfigHint, &draft.RemoveHostConfig)
		a.setCheckbox(c, copy.UninstallToolData, copy.UninstallToolDataHint, &draft.RemoveToolData)
		a.setCheckbox(c, copy.UninstallArtifacts, copy.UninstallArtifactsHint, &draft.RemoveArtifacts)
		ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, copy.IntegrationsUninstall).Clicked() {
				if a.Actions.UninstallComponents != nil {
					a.Actions.UninstallComponents(integration.ID, integration.Name, draft)
				}
				a.uninstallOpen[integration.ID] = false
				return
			}
			if ui.Button(c, copy.PermissionsCancel).Clicked() {
				a.uninstallOpen[integration.ID] = false
			}
		})
	})
	a.uninstallDraft[integration.ID] = draft
}

// setCheckbox is one labeled component inside the uninstall dialog: a card
// with the box, the category's name and the line that says what removing it
// costs — the reference's `.extensions-uninstall-component`. The hint used to
// be dropped; a data category the user cannot read about is a category they
// cannot choose deliberately.
func (a *App) setCheckbox(c *ui.Context, label, description string, value *bool) {
	t := c.Theme()
	tokens := a.tokens(c)
	on := *value
	changed := false
	ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(ui.Start).
		Padding(t.Space(3)).Radius(tokens.RadiusSM).
		Border(1, tokens.ControlEdge).Background(tokens.Control).Children(func() {
		if ui.Checkbox(c, &on, "").Changed() {
			changed = true
		}
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, label).FontSize(t.FontSize + 1).FontWeight(600).TextColor(tokens.TextStrong)
			if description != "" {
				ui.Text(c, description).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
		})
	})
	if changed {
		*value = on
	}
}
