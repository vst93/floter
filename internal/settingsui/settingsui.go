// Package settingsui draws the settings surface: a sidebar of pages beside
// a body, with the General page's controls bound to the settings store.
//
// P1 routes every page the old panel had and fills General; the others show
// a short note. Every control writes through the store, which preserves the
// keys this build does not own — see internal/settings.
package settingsui

import (
	"math"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// The settings pages, in sidebar order. P1 fills General.
const (
	PageGeneral = iota
	PageSessions
	PageShortcuts
	PageIntegrations
	PageAbout

	pageCount
)

// Actions are the shell's callbacks.
type Actions struct {
	// Close leaves the settings surface (Escape, the close button).
	Close func()
}

// App is the settings surface's state.
type App struct {
	Store   *settings.Store
	Actions Actions

	// Page is the chosen page, an index into the sidebar.
	Page int

	// Sidebar holds the page list's identity, its choice and its focus.
	Sidebar ui.ListState
}

// New builds the settings surface over a store.
func New(store *settings.Store, actions Actions) *App {
	a := &App{Store: store, Actions: actions}
	a.Sidebar.Selected = &a.Page
	a.Sidebar.Label = func(row int) string { return a.page(row).Title }
	return a
}

// FocusSidebar asks for the keyboard focus on the page list.
func (a *App) FocusSidebar() { a.Sidebar.Handle.Focus() }

// page describes one sidebar entry in the current language.
type page struct {
	Title string
	Hint  string
}

// Page is the chosen page index.
func (a *App) SelectedPage() int { return a.Page }

// page returns the i-th page's copy.
func (a *App) page(i int) page {
	c := i18n.For(a.Store.Snapshot().Language).Settings
	switch i {
	case PageSessions:
		return page{c.PageSessions, c.PageSessionsHint}
	case PageShortcuts:
		return page{c.PageShortcuts, c.PageShortcutsHint}
	case PageIntegrations:
		return page{c.PageIntegrations, c.PageIntegrationsHint}
	case PageAbout:
		return page{c.PageAbout, c.PageAboutHint}
	default:
		return page{c.PageGeneral, c.PageGeneralHint}
	}
}

// View builds the settings surface: the page list beside the body.
func (a *App) View(c *ui.Context) {
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	t := c.Theme()

	if c.Shortcut(0, ui.KeyEscape) && a.Actions.Close != nil {
		a.Actions.Close()
	}

	ui.Row(c).Fill().AlignItems(ui.Stretch).Gap(t.Space(3)).Children(func() {
		ui.Column(c).Width(180).Gap(t.Space(0.5)).Children(func() {
			ui.Text(c, copy.Title).FontSize(t.FontSize+4).Bold().Padding(0, t.Space(1), t.Space(1), t.Space(1))
			ui.List(c.Key("settings.sidebar"), &a.Sidebar, pageCount, func(i int) {
				a.sidebarRow(c, i)
			}).Grow(1).Label(copy.Title)
		})
		ui.Divider(c).FillHeight()
		ui.Column(c).Grow(1).Children(func() {
			a.body(c, copy)
		})
	})
}

// sidebarRow builds one page entry. The list paints the chosen row.
func (a *App) sidebarRow(c *ui.Context, i int) {
	t := c.Theme()
	row := ui.Row(c).Padding(t.Space(1.5), t.Space(1.5))
	row.Children(func() {
		ui.Text(c, a.page(i).Title).FontSize(t.FontSize)
	})
	if row.Clicked() {
		a.Page = i
	}
}

// body builds the chosen page.
func (a *App) body(c *ui.Context, copy i18n.Settings) {
	p := a.page(a.Page)
	t := c.Theme()

	ui.Column(c).Fill().Gap(t.Space(1)).Children(func() {
		ui.Text(c, p.Hint).FontSize(t.FontSize).TextColor(t.TextMuted)
		ui.Divider(c)
		if a.Page != PageGeneral {
			ui.Column(c).Fill().Grow(1).Center().Children(func() {
				ui.Text(c, copy.PagePlaceholder).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			a.general(c, copy)
		})
	})
}

// general is the General page: the six settings the P0 loader owns, each a
// real control writing straight through the store.
func (a *App) general(c *ui.Context, copy i18n.Settings) {
	s := a.Store.Snapshot()

	ui.Form(c, func() {
		a.pick(c, copy.Theme, "",
			[]string{copy.ThemeAuto, copy.ThemeLight, copy.ThemeDark},
			[]string{"auto", "light", "dark"}, s.Theme,
			func(id string) { a.set(func(s *settings.Settings) { s.Theme = id }) })

		a.pick(c, copy.Language, copy.LanguageHint,
			[]string{"English", "中文"},
			[]string{"en", "zh"}, s.Language,
			func(id string) { a.set(func(s *settings.Settings) { s.Language = id }) })

		a.pick(c, copy.Glass, copy.GlassHint,
			[]string{copy.GlassOff, copy.GlassFrosted, copy.GlassRegular, copy.GlassLiquid},
			[]string{"off", "frosted", "regular", "liquid"}, s.GlassStep,
			func(id string) { a.set(func(s *settings.Settings) { s.GlassStep = id }) })

		a.opacity(c, copy.MainOpacity, copy.MainOpacityHint, s.MainOpacity,
			func(v uint8) { a.set(func(s *settings.Settings) { s.MainOpacity = v }) })

		a.opacity(c, copy.TerminalOpacity, copy.TerminalOpacityHint, s.TerminalOpacity,
			func(v uint8) { a.set(func(s *settings.Settings) { s.TerminalOpacity = v }) })

		a.pick(c, copy.Scale, copy.ScaleHint,
			[]string{copy.ScaleTiny, copy.ScaleSmall, copy.ScaleDefault, copy.ScaleLarge},
			[]string{"tiny", "small", "default", "large"}, s.UIScale,
			func(id string) { a.set(func(s *settings.Settings) { s.UIScale = id }) })
	})
}

// pick builds one labeled segmented choice: labels[i] stands for ids[i].
func (a *App) pick(c *ui.Context, label, description string, labels, ids []string, current string, apply func(string)) {
	index := indexOf(ids, current)
	chosen := ""
	field := ui.Field(c, label, func() {
		e := ui.Segmented(c, &index, labels...).Label(label)
		if e.Changed() {
			chosen = ids[index]
		}
	})
	if description != "" {
		field.Description(description)
	}
	if chosen != "" {
		apply(chosen)
	}
}

// opacity builds one labeled transparency slider, 10..100 percent.
func (a *App) opacity(c *ui.Context, label, description string, value uint8, apply func(uint8)) {
	v := float64(value)
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	chosen := false
	field := ui.Field(c, label, func() {
		ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
			e := ui.Slider(c, &v, settings.MinWindowOpacity, settings.MaxWindowOpacity).Grow(1)
			if e.Changed() {
				chosen = true
			}
			ui.Text(c, copy.Percent(clampPercent(v))).FontSize(c.Theme().FontSize).TextColor(c.Theme().TextMuted)
		})
	})
	if description != "" {
		field.Description(description)
	}
	if chosen {
		apply(clampPercent(v))
	}
}

// set writes one settings change through the store.
func (a *App) set(mutate func(*settings.Settings)) {
	_ = a.Store.Update(mutate)
}

func clampPercent(v float64) uint8 {
	n := uint8(math.Round(v))
	if n < settings.MinWindowOpacity {
		return settings.MinWindowOpacity
	}
	if n > settings.MaxWindowOpacity {
		return settings.MaxWindowOpacity
	}
	return n
}

func indexOf(ids []string, value string) int {
	for i, id := range ids {
		if id == value {
			return i
		}
	}
	return 0
}
