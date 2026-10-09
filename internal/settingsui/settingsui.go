// Package settingsui draws the settings surface: a sidebar of pages beside
// a body.
//
// The General page's controls bind to the settings store, which writes
// every change back to the same JSON file while preserving the keys this
// build does not own (see internal/settings); the terminal group drives the
// terminal surface's appearance (see internal/terminalui). The other pages
// route but say they arrive later.
package settingsui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
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
	// CloseSession ends the running terminal session, from the Sessions
	// page.
	CloseSession func()
}

// Session is one running session, as the Sessions page shows it.
type Session struct {
	Title   string
	Running bool
}

// About is what the About page shows: the identity of the build and where
// its files live.
type About struct {
	Name         string
	Version      string
	Framework    string
	Scheme       string
	SettingsPath string
	RepoURL      string
}

// App is the settings surface's state.
type App struct {
	Store   *settings.Store
	Actions Actions
	About   About

	// Sessions reports the running sessions; nil when the shell has none
	// (tests).
	Sessions func() []Session
	// Shortcut is the global summon key the shell registered.
	Shortcut string

	// Page is the chosen page, an index into the sidebar.
	Page int

	// Sidebar holds the page list's identity, its choice and its focus.
	Sidebar ui.ListState
	// Body keeps the form's scroll offset across frames.
	Body ui.ScrollState
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

// SelectedPage is the chosen page.
func (a *App) SelectedPage() int { return a.Page }

// page describes one sidebar entry in the current language.
type page struct {
	Title string
	Hint  string
}

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

// body builds the chosen page: a hint pinned over the content, which
// scrolls under it behind a soft scroll edge.
func (a *App) body(c *ui.Context, copy i18n.Settings) {
	p := a.page(a.Page)
	t := c.Theme()

	header := t.Space(6)
	edge := header + t.Space(2)
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c.Key("settings.body")).TrackScroll(&a.Body).Fill().
			Padding(edge, 0, 0, 0).Children(func() {
			switch a.Page {
			case PageGeneral:
				a.general(c, copy)
			case PageSessions:
				a.sessions(c, copy)
			case PageShortcuts:
				a.shortcuts(c, copy)
			case PageAbout:
				a.about(c, copy)
			default:
				ui.Column(c).FillWidth().Padding(t.Space(4)).Center().Children(func() {
					ui.Text(c, copy.PagePlaceholder).FontSize(t.FontSize).TextColor(t.TextMuted)
				})
			}
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(edge).PassThrough().
			Material(glass.ScrollEdge{Background: t.Background})
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Height(header).Children(func() {
			ui.Text(c, p.Hint).FontSize(t.FontSize).TextColor(t.TextMuted)
			ui.Divider(c).Padding(t.Space(0.5), 0)
		})
	})
}

// sessions lists the running terminal sessions.
func (a *App) sessions(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	running := a.runningSessions()
	if len(running) == 0 {
		ui.Column(c).FillWidth().Padding(t.Space(4)).Center().Children(func() {
			ui.Text(c, copy.SessionsNone).FontSize(t.FontSize).TextColor(t.TextMuted)
		})
		return
	}
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(1), 0).Children(func() {
		for _, session := range running {
			row := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).
				Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius)
			row.Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, session.Title).FontSize(t.FontSize)
					ui.Text(c, copy.SessionsRunning).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
				if a.Actions.CloseSession != nil {
					if ui.Button(c, copy.SessionsClose).Clicked() {
						a.Actions.CloseSession()
					}
				}
			})
		}
		ui.Text(c, copy.SessionsActive).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
	})
}

// shortcuts lists the shortcuts the app answers, with the key each one
// currently holds.
func (a *App) shortcuts(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		a.keyValue(c, copy.ShortcutsToggle, a.summonShortcut())
		ui.Text(c, copy.ShortcutsHint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		ui.Column(c).Height(t.Space(2)).Children(func() {})
	})
}

// about shows the build's identity and where its files live.
func (a *App) about(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	about := a.About
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Text(c, strings.TrimSpace(about.Name+" "+about.Version)).FontSize(t.FontSize + 4).Bold()
		a.keyValue(c, copy.AboutVersion, about.Version)
		a.keyValue(c, copy.AboutFramework, about.Framework)
		a.keyValue(c, copy.AboutScheme, about.Scheme)
		a.keyValue(c, copy.AboutSettingsFile, about.SettingsPath)
		if about.RepoURL != "" {
			ui.Row(c).Gap(t.Space(2)).Children(func() {
				ui.Text(c, copy.AboutProject).Width(120).TextColor(t.TextMuted).FontSize(t.FontSize)
				ui.Link(c, about.RepoURL, about.RepoURL).FontSize(t.FontSize)
			})
		}
	})
}

// keyValue is one labeled line of read-only information: the label in a
// fixed column, the value beside it, selectable so it can be copied.
func (a *App) keyValue(c *ui.Context, label, value string) {
	t := c.Theme()
	if value == "" {
		return
	}
	ui.Row(c).Gap(t.Space(2)).Children(func() {
		ui.Text(c, label).Width(120).TextColor(t.TextMuted).FontSize(t.FontSize)
		ui.Text(c, value).FontSize(t.FontSize).Grow(1).Selectable()
	})
}

// runningSessions is what the Sessions page lists.
func (a *App) runningSessions() []Session {
	if a.Sessions == nil {
		return nil
	}
	return a.Sessions()
}

// summonShortcut is the global shortcut the shell registered, or a dash
// while it is unknown (a test build).
func (a *App) summonShortcut() string {
	if a.Shortcut == "" {
		return "-"
	}
	return a.Shortcut
}

// general is the General page: the settings the loader owns, each a real
// control writing straight through the store.
func (a *App) general(c *ui.Context, copy i18n.Settings) {
	s := a.Store.Snapshot()

	ui.Form(c, func() {
		ui.Fieldset(c, copy.GroupAppearance, func() {
			a.pick(c, copy.Theme, "", copy.Themes, s.Theme,
				func(id string) { a.set(func(s *settings.Settings) { s.Theme = id }) })
			a.pick(c, copy.Language, copy.LanguageHint, copy.Languages, s.Language,
				func(id string) { a.set(func(s *settings.Settings) { s.Language = id }) })
			a.pick(c, copy.Glass, copy.GlassHint, copy.GlassSteps, s.GlassStep,
				func(id string) { a.set(func(s *settings.Settings) { s.GlassStep = id }) })
			a.slider(c, copy.MainOpacity, copy.MainOpacityHint,
				float64(s.MainOpacity), settings.MinWindowOpacity, settings.MaxWindowOpacity,
				func(v float64) string { return copy.Percent(clampPercent(v)) },
				func(v float64) { a.set(func(s *settings.Settings) { s.MainOpacity = clampPercent(v) }) })
			a.slider(c, copy.TerminalOpacity, copy.TerminalOpacityHint,
				float64(s.TerminalOpacity), settings.MinWindowOpacity, settings.MaxWindowOpacity,
				func(v float64) string { return copy.Percent(clampPercent(v)) },
				func(v float64) {
					a.set(func(s *settings.Settings) { s.TerminalOpacity = clampPercent(v) })
				})
		})

		ui.Fieldset(c, copy.GroupWindow, func() {
			a.pick(c, copy.Scale, copy.ScaleHint, copy.Scales, s.UIScale,
				func(id string) { a.set(func(s *settings.Settings) { s.UIScale = id }) })
			a.checkbox(c, copy.HideOnBlur, s.HideOnBlur,
				func(on bool) { a.set(func(s *settings.Settings) { s.HideOnBlur = on }) })
			a.choose(c, copy.SurfaceResidency, copy.SurfaceResidencyHint,
				residencyOptions(copy, s.SurfaceResidencySeconds),
				strconv.FormatUint(uint64(s.SurfaceResidencySeconds), 10),
				func(id string) {
					seconds, ok := parseResidency(id)
					if !ok {
						return
					}
					a.set(func(s *settings.Settings) { s.SurfaceResidencySeconds = seconds })
				})
		})

		ui.Fieldset(c, copy.GroupTerminal, func() {
			a.slider(c, copy.FontSize, "", float64(s.FontSize),
				settings.MinFontSize, settings.MaxFontSize,
				func(v float64) string { return fmt.Sprintf("%d", int(math.Round(v))) },
				func(v float64) {
					a.set(func(s *settings.Settings) { s.FontSize = int(math.Round(v)) })
				})
			a.text(c, copy.FontFamily, "", settings.DefaultFontFamily, s.FontFamily,
				func(v string) { a.set(func(s *settings.Settings) { s.FontFamily = v }) })
			a.pick(c, copy.CursorShape, "", copy.CursorShapes, s.CursorShape,
				func(id string) { a.set(func(s *settings.Settings) { s.CursorShape = id }) })
			a.checkbox(c, copy.CursorBlink, s.CursorBlink,
				func(on bool) { a.set(func(s *settings.Settings) { s.CursorBlink = on }) })
			a.slider(c, copy.LineHeight, "", s.TerminalLineHeight,
				settings.MinTerminalLineHeight, settings.MaxTerminalLineHeight,
				func(v float64) string { return fmt.Sprintf("%.1f×", v) },
				func(v float64) {
					a.set(func(s *settings.Settings) { s.TerminalLineHeight = v })
				})
			a.pick(c, copy.Padding, "", copy.Paddings, s.TerminalPadding,
				func(id string) { a.set(func(s *settings.Settings) { s.TerminalPadding = id }) })
			a.choose(c, copy.Palette, copy.TerminalHint, copy.Palettes, s.TerminalTheme,
				func(id string) { a.set(func(s *settings.Settings) { s.TerminalTheme = id }) })
		})
	})
}

// pick builds one labeled segmented choice: options[i].Label stands for
// options[i].ID.
func (a *App) pick(c *ui.Context, label, description string, options []i18n.Option, current string, apply func(string)) {
	index := optionIndex(options, current)
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	chosen := ""
	field := ui.Field(c, label, func() {
		e := ui.Segmented(c, &index, labels...).Label(label)
		if e.Changed() && index >= 0 && index < len(options) {
			chosen = options[index].ID
		}
	})
	if description != "" {
		field.Description(description)
	}
	if chosen != "" {
		apply(chosen)
	}
}

// choose builds one labeled drop-down choice, for lists too long to line up
// as segments.
func (a *App) choose(c *ui.Context, label, description string, options []i18n.Option, current string, apply func(string)) {
	index := optionIndex(options, current)
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	selected := ""
	if index >= 0 && index < len(options) {
		selected = options[index].Label
	}
	chosen := ""
	field := ui.Field(c, label, func() {
		e := ui.Select(c, &selected, labels).Label(label)
		if e.Changed() {
			for _, option := range options {
				if option.Label == selected {
					chosen = option.ID
					break
				}
			}
		}
	})
	if description != "" {
		field.Description(description)
	}
	if chosen != "" {
		apply(chosen)
	}
}

// slider builds one labeled slider with its value beside it.
func (a *App) slider(c *ui.Context, label, description string, value, lo, hi float64, format func(float64) string, apply func(float64)) {
	v := value
	chosen := false
	field := ui.Field(c, label, func() {
		ui.Row(c).Gap(c.Theme().Space(2)).AlignItems(ui.Center).Children(func() {
			e := ui.Slider(c, &v, lo, hi).Grow(1)
			if e.Changed() {
				chosen = true
			}
			ui.Text(c, format(v)).FontSize(c.Theme().FontSize).TextColor(c.Theme().TextMuted)
		})
	})
	if description != "" {
		field.Description(description)
	}
	if chosen {
		apply(v)
	}
}

// checkbox builds one self-labeling check box in the form's content column:
// the switch of the old panel, in the control this toolkit makes directly
// clickable by its own text.
func (a *App) checkbox(c *ui.Context, label string, on bool, apply func(bool)) {
	value := on
	changed := false
	ui.Field(c, "", func() {
		// Changed applies the pending input to value before returning, so
		// the new state is usable in this same pass.
		if ui.Checkbox(c, &value, label).Changed() {
			changed = true
		}
	})
	if changed {
		apply(value)
	}
}

// text builds one labeled text field.
func (a *App) text(c *ui.Context, label, description, placeholder, value string, apply func(string)) {
	edit := value
	field := ui.Field(c, label, func() {
		e := ui.TextInput(c, &edit).Placeholder(placeholder).Label(label).Width(220)
		if e.Submitted() {
			apply(edit)
		}
	})
	if description != "" {
		field.Description(description)
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

// parseResidency reads a residency option id: the presets' seconds, the
// "never" sentinel, or a custom number, all as decimal text.
func parseResidency(id string) (uint32, bool) {
	seconds, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(seconds), true
}

// residencyOptions is the residency select's choices: the presets, the
// "never" sentinel, and the stored value when it is a custom number the
// presets do not name (shown first, so the control never lies about what
// is stored).
func residencyOptions(copy i18n.Settings, current uint32) []i18n.Option {
	options := []i18n.Option{
		{ID: "0", Label: copy.SurfaceResidencyOff},
	}
	for _, seconds := range []uint32{10, 20, 30, 60, 120} {
		options = append(options, i18n.Option{
			ID:    strconv.FormatUint(uint64(seconds), 10),
			Label: copy.SurfaceResidencyValue(seconds),
		})
	}
	options = append(options, i18n.Option{
		ID:    strconv.FormatUint(uint64(settings.SurfaceResidencyNever), 10),
		Label: copy.SurfaceResidencyNever,
	})
	id := strconv.FormatUint(uint64(current), 10)
	for _, option := range options {
		if option.ID == id {
			return options
		}
	}
	return append([]i18n.Option{{ID: id, Label: copy.SurfaceResidencyValue(current)}}, options...)
}

func optionIndex(options []i18n.Option, id string) int {
	for i, option := range options {
		if option.ID == id {
			return i
		}
	}
	return 0
}
