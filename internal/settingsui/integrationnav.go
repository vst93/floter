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
//
// There are two ranks of navigation on this page — the page's own three tabs
// (My tools / Add tools / Import & export) and, inside the Add tools tab, the
// three ways to add (Detected / Local tool / Package) — and they used to be the
// *same* control drawn twice, one row above the other, so the second read as a
// repeat of the first rather than as its children (「这里的两层菜单样式和交互太
// 丑了」). The ranks are now told apart the way a hierarchy is told apart
// everywhere else in this app: the parent is a row of words on the sheet's own
// surface with a rule under the chosen one, and only the child wears a plate.
// The parent is `primary` and the child is not.
func (a *App) integrationTabs(c *ui.Context, selected *int, primary bool, labels ...string) {
	tokens := a.tokens(c)
	t := c.Theme()
	parts := ui.SegmentedBase(c, selected, len(labels))
	track := parts.Track.FillWidth().Gap(4)
	if primary {
		// The page's own rank: no plate, no track, and a rule under the chosen
		// word — the tab shape, not the segmented-control shape. The track's own
		// bottom hairline is the bar the chosen tab's rule sits on, which is what
		// makes the row read as a tab bar rather than as three floating words.
		track.Background(ui.Transparent).InsetShadow(0, -1, 0, 0, tokens.HairlineSoft)
		track.Children(func() {
			for i, label := range labels {
				i, label := i, label
				segment := parts.Segment(i).Grow(1).Basis(0).MinWidth(0).MinHeight(34).
					Padding(0, 8).Center().Label(label)
				chosen := i == *selected
				ink := t.TextMuted
				if chosen {
					ink = tokens.TextStrong
				} else if segment.Hovered() {
					ink = t.Text
				}
				segment.Children(func() {
					ui.Text(c, label).FontSize(tokens.Body).FontWeight(600).TextColor(ink).
						SingleLine().Ellipsis("…")
				})
				// The chosen tab's own underline: an inset stroke on the segment's
				// bottom edge, so it is the segment's last row of pixels and moves
				// with the word. An absolutely positioned child would be placed in
				// the segment's *padding* box (this row is `Center()`ed, so the
				// segment's content box is the word), which put the rule through
				// the label instead of under it.
				if chosen {
					// The rule is the segment's own bottom edge: an inset stroke,
					// so it is the last rows of pixels the segment paints and it
					// cannot land through the label. The segment is a third of the
					// row wide, so the rule is that wide too — the tab bar's own
					// grammar, where the chosen tab is marked by the rule under
					// the third of the bar it owns.
					segment.InsetShadow(0, -2, 0, 0, t.Accent)
				}
			}
		})
		return
	}
	// The child rank keeps the plate: it is the control that actually narrows
	// the page, and the parent above it no longer competes with it.
	track.Children(func() {
		for i, label := range labels {
			i, label := i, label
			segment := parts.Segment(i).Grow(1).Basis(0).MinWidth(0).MinHeight(28).
				Padding(0, 8).Radius(tokens.RadiusSM).Center().Label(label)
			if i == *selected {
				segment.Background(tokens.RowSelected)
				tokens.LitFace(segment)
			} else if segment.Hovered() {
				segment.Background(tokens.ControlHover)
				tokens.LitFace(segment)
			}
			segment.Children(func() {
				ink := tokens.TextSecondary
				if i == *selected {
					ink = tokens.TextStrong
				}
				ui.Text(c, label).FontSize(tokens.Body).FontWeight(500).TextColor(ink).SingleLine().Ellipsis("…")
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
	a.integrationTabs(c.Key("tabs"), &tab, true, copy.IntegrationsMine, copy.IntegrationsAdd, copy.IntegrationsTransfer)
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
	// The second rank rides directly under the first: one block of navigation,
	// two ranks in it. Spacing them by the page's own gap (Space(4)) made the
	// child look like the first section of the page rather than the parent's
	// children, which is the other half of 「两层菜单…太丑了」.
	switch n.tab {
	case 0:
		if a.textInput(c.Key("filter"), &n.query).FillWidth().MinHeight(32).Height(32).Label(copy.IntegrationsSearch).Placeholder(copy.IntegrationsSearch).Changed() {
			a.Body.Y, n.listY = 0, 0
		}
	case 1:
		method := n.addMethod
		ui.Box(c).FillWidth().Padding(c.Theme().Space(0.5), 0, 0, 0).Children(func() {
			a.integrationTabs(c.Key("method"), &method, false, copy.IntegrationsDiscover, copy.IntegrationsLocal, copy.IntegrationsPackages)
		})
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

// OpenAddTools, OpenLocalTool and OpenLocalToolForm put the Integrations page
// into the state a caller wants to render, for a test or a deep link that has
// no pointer: the Add tools tab, its Local tool sub-tab, and that section's
// open form. They are the same three writes the tabs and the Add button make,
// so a frame driven by them is the frame the user reaches.
func (a *App) OpenAddTools() {
	n := &a.integrationNav
	n.detail = nil
	n.tab, n.addMethod = 1, 0
	a.Body.Y = 0
}

// OpenLocalTool is OpenAddTools on the Local tool sub-tab.
func (a *App) OpenLocalTool() {
	a.OpenAddTools()
	a.integrationNav.addMethod = 1
	a.Body.Y = 0
}

// OpenLocalToolForm is OpenLocalTool with its form open.
func (a *App) OpenLocalToolForm() {
	a.OpenLocalTool()
	a.toolOpen = true
	a.toolIssue = false
}
