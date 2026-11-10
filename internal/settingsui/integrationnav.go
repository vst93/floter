package settingsui

import (
	"strings"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

// Overview and details are separate viewports, but drafts remain owned by App.
// A refresh can reorder entries without replacing the selected editor.
type integrationKey struct {
	ID         string
	Occurrence int
}

type integrationNavigation struct {
	tab, addMethod int
	query          string
	detail         *integrationKey
	listY          float32
	returnTo       *integrationKey
	highlight      string
}

// Navigation is lighter and shorter than a form's segmented value control.
// Keep native segment keyboard semantics and one Tab stop.
func (a *App) integrationTabs(c *ui.Context, selected *int, labels ...string) {
	tokens := a.tokens(c)
	parts := ui.SegmentedBase(c, selected, len(labels))
	parts.Track.FillWidth().Gap(4).Children(func() {
		for i, label := range labels {
			segment := parts.Segment(i).Grow(1).Basis(0).MinWidth(0).MinHeight(30).
				Padding(0, 8).Radius(tokens.RadiusSM).Center().Label(label)
			if i == *selected {
				segment.Background(tokens.RowSelected)
				tokens.LitFace(segment)
			} else if segment.Hovered() {
				segment.Background(tokens.ControlHover)
				tokens.LitFace(segment)
			}
			segment.Children(func() {
				ui.Text(c, label).FontSize(tokens.Body).FontWeight(500).TextColor(tokens.TextStrong).SingleLine().Ellipsis("…")
			})
		}
	})
}

func integrationKeys(items []Integration) []integrationKey {
	keys := make([]integrationKey, len(items))
	seen := make(map[string]int, len(items))
	for i, item := range items {
		keys[i] = integrationKey{item.ID, seen[item.ID]}
		seen[item.ID]++
	}
	return keys
}

func (a *App) settingsScrollKey() any {
	if a.Page != PageIntegrations {
		return bodyPage(a.Page)
	}
	n := &a.integrationNav
	if n.detail != nil {
		return *n.detail
	}
	return struct{ Tab, Method int }{n.tab, n.addMethod}
}

func (a *App) openIntegrationDetail(key integrationKey) {
	n := &a.integrationNav
	n.listY = a.Body.Y
	n.detail = &key
	n.returnTo = nil
	a.Body.Y = 0
}

func (a *App) closeIntegrationDetail() {
	n := &a.integrationNav
	n.returnTo = n.detail
	n.detail = nil
	n.tab = 0
	a.Body.Y = n.listY
	a.cancelConfirmations()
}

func (a *App) syncIntegrationNavigation(items []Integration) {
	n := &a.integrationNav
	var keys []integrationKey
	if n.detail != nil {
		keys = integrationKeys(items)
	}
	found, highlight := n.detail == nil, ""
	for i, item := range items {
		if n.detail != nil && keys[i] == *n.detail {
			found = true
		}
		if item.Highlighted {
			highlight = item.ID
			if highlight != n.highlight {
				if keys == nil {
					keys = integrationKeys(items)
				}
				n.tab = 0
				a.openIntegrationDetail(keys[i])
				found = true
			}
		}
	}
	n.highlight = highlight
	if !found {
		a.closeIntegrationDetail()
	}
}

func (a *App) integrationToolbar(c *ui.Context, copy i18n.Settings) {
	n := &a.integrationNav
	if n.detail != nil {
		return
	}
	tab := n.tab
	a.integrationTabs(c.Key("tabs"), &tab, copy.IntegrationsMine, copy.IntegrationsAdd, copy.IntegrationsTransfer)
	if tab != n.tab {
		if n.tab == 1 && n.addMethod == 1 {
			a.toolPageEpoch++
		}
		a.flushSettingsWrites()
		if n.tab == 0 {
			n.listY = a.Body.Y
		}
		n.tab = tab
		a.Body.Y = 0
		if tab == 0 {
			a.Body.Y = n.listY
		}
		a.cancelConfirmations()
	}
	if n.tab == 0 {
		if a.textInput(c.Key("filter"), &n.query).FillWidth().MinHeight(32).Height(32).Label(copy.IntegrationsSearch).Placeholder(copy.IntegrationsSearch).Changed() {
			a.Body.Y, n.listY = 0, 0
		}
	} else if n.tab == 1 {
		method := n.addMethod
		a.integrationTabs(c.Key("method"), &method, copy.IntegrationsDiscover, copy.IntegrationsLocal, copy.IntegrationsPackages)
		if method != n.addMethod {
			if n.addMethod == 1 {
				a.toolPageEpoch++
			}
			n.addMethod, a.Body.Y = method, 0
		}
	}
}

func integrationMatches(item Integration, query string) bool {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return true
	}
	text := item.Name + " " + item.ID + " " + item.Description
	for _, command := range item.Commands {
		text += " " + command.Name + " " + command.ID
	}
	text = strings.ToLower(text)
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func (a *App) integrations(c *ui.Context, copy i18n.Settings) {
	a.integrationContent(c, copy, a.installedIntegrations())
}

func (a *App) integrationContent(c *ui.Context, copy i18n.Settings, items []Integration) {
	a.integrationLead = ""
	a.restoreMissingNoticeFocus(c, items)
	n := &a.integrationNav
	a.extensionMetrics(func() {
		if n.detail != nil {
			keys := integrationKeys(items)
			for i, item := range items {
				if keys[i] == *n.detail {
					a.integrationRow(c.Key("detail").Key(keys[i]), copy, item)
					return
				}
			}
			return
		}
		ui.Column(c).FillWidth().Gap(c.Theme().Space(4)).Children(func() {
			switch n.tab {
			case 1:
				switch n.addMethod {
				case 1:
					a.localToolSection(c.Key("local"), copy)
				case 2:
					a.installFolderRow(c.Key("folder"), copy)
					a.installRow(c.Key("registry"), copy)
				default:
					if a.Actions.Detected == nil || len(a.Actions.Detected()) == 0 {
						a.extensionHint(c, copy.IntegrationsDiscoverEmpty)
					}
					a.detectedSection(c.Key("detected"), copy)
					a.recommendedRow(c.Key("recommended"), copy)
				}
			case 2:
				a.transferRow(c.Key("transfer"), copy)
			default:
				a.integrationOverview(c.Key("overview"), copy, items)
			}
		})
	})
}

func (a *App) integrationOverview(c *ui.Context, copy i18n.Settings, items []Integration) {
	a.cardCompact = true
	defer func() { a.cardCompact = false }()
	n := &a.integrationNav
	count := 0
	var plugins []basePlugin
	if a.Actions.OpenPluginConfig != nil || a.Actions.SetPluginEnabled != nil {
		for _, plugin := range a.basePlugins(copy) {
			if integrationMatches(Integration{ID: plugin.ID, Name: plugin.Name, Description: plugin.Hint}, n.query) {
				plugins = append(plugins, plugin)
			}
		}
	}
	if len(plugins) > 0 {
		count += len(plugins)
		a.extensionSection(c.Key("builtins"), copy.BasePlugins, len(plugins), func() {
			// These three plugins already expose their entire management surface
			// through Configure. Keep their switches within reach of the list.
			a.card(c, func() {
				for _, plugin := range plugins {
					a.basePluginSummary(c.Key(plugin.ID), copy, plugin)
				}
			})
		})
	}
	keys := integrationKeys(items)
	var matches []int
	for i, item := range items {
		if integrationMatches(item, n.query) {
			matches = append(matches, i)
		}
	}
	count += len(matches)
	if len(matches) > 0 {
		a.extensionSection(c.Key("installed"), copy.IntegrationsInstalled, len(matches), func() {
			a.card(c, func() {
				for _, index := range matches {
					a.integrationSummary(c.Key(keys[index]), copy, items[index], keys[index])
				}
			})
		})
	}
	if count == 0 {
		message := copy.IntegrationsEmpty
		if strings.TrimSpace(n.query) != "" {
			message = copy.IntegrationsNoMatches
		}
		a.emptyState(c, "package", message, "", a.cardSpace(c.Theme(), 5), 0, false, nil)
	}
	if strings.TrimSpace(n.query) == "" {
		a.card(c.Key("commands-search"), func() {
			a.checkbox(c, copy.ShowCommandsInSearch, copy.ShowCommandsInSearchHint, settings.ShowCommandsInSearch(a.settings()), func(on bool) { a.set(func(s *settings.Settings) { s.SetShowCommandsInSearch(on) }) })
		})
	}
}

func (a *App) basePluginSummary(c *ui.Context, copy i18n.Settings, plugin basePlugin) {
	hint, ink := plugin.Hint, rowInkQuiet
	if plugin.Privacy != "" {
		hint, ink = plugin.Privacy, rowInkWarning
	}
	on, changed := plugin.Enabled, false
	clicked := a.rowState(c, plugin.Name, hint, ink, func() {
		ui.Row(c).Gap(a.cardTheme(c).Space(2)).AlignItems(ui.Center).Children(func() {
			if a.Actions.OpenPluginConfig != nil {
				if a.extensionIconButton(c, "settings", copy.PluginConfigure+" "+plugin.Name, false).Clicked() {
					a.Actions.OpenPluginConfig(plugin.ID)
				}
			}
			if plugin.Toggleable && a.Actions.SetPluginEnabled != nil {
				changed = a.switchControl(c, &on).Label(plugin.Name).Changed()
			}
		})
	})
	if clicked && plugin.Toggleable && a.Actions.SetPluginEnabled != nil {
		on, changed = !on, true
	}
	if changed {
		a.observeSettingsWrite(func() { a.Actions.SetPluginEnabled(plugin.ID, on) })
	}
}

func (a *App) integrationSummary(c *ui.Context, copy i18n.Settings, item Integration, key integrationKey) {
	t := a.cardTheme(c)
	var manage ui.Element
	clicked := a.row(c, item.Name, item.Description, func() {
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			a.integrationStatus(c, copy, item)
			manage = a.extensionIconButton(c, "chevron-right", copy.IntegrationsManage+" "+item.Name, false)
		})
	})
	if a.integrationNav.returnTo != nil && *a.integrationNav.returnTo == key {
		manage.Focus()
		a.integrationNav.returnTo = nil
	}
	if clicked || manage.Clicked() {
		a.openIntegrationDetail(key)
	}
}
