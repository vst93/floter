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
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/terminalui"
)

// The settings pages, in sidebar order.
const (
	PageGeneral = iota
	PageSessions
	PageShortcuts
	PagePlugins
	PageIntegrations
	PageAbout

	pageCount
)

// pageNames are the ids the `last_settings_page` setting stores, in the same
// order as the pages: reopening the settings surface returns to the page the
// user last looked at.
var pageNames = [pageCount]string{"general", "sessions", "shortcuts", "plugins", "integrations", "about"}

// PageByName resolves a stored page id, reporting whether it is one this build
// knows.
func PageByName(name string) (int, bool) {
	for i, page := range pageNames {
		if page == name {
			return i, true
		}
	}
	return PageGeneral, false
}

// IntegrationsPage is the page index for a caller outside the package.
func IntegrationsPage() int { return PageIntegrations }

// PageName is the stored id of a page index.
func PageName(page int) string {
	if page < 0 || page >= pageCount {
		return pageNames[PageGeneral]
	}
	return pageNames[page]
}

// Actions are the shell's callbacks.
type Actions struct {
	// Close leaves the settings surface (Escape, the close button).
	Close func()
	// CloseSession ends the running terminal session, from the Sessions
	// page.
	CloseSession func()
	// SetIntegrationEnabled turns an installed extension on or off.
	SetIntegrationEnabled func(id string, enabled bool)
	// UninstallIntegration removes an installed extension.
	UninstallIntegration func(id string, name string)
	// UninstallComponents removes the parts of an installed extension the
	// checkboxes name. Nil falls back to UninstallIntegration.
	UninstallComponents func(id string, name string, components UninstallComponents)
	// DiagnoseIntegration asks an integration's provider to check itself.
	DiagnoseIntegration func(id string)
	// ReprobeCommands re-runs the connect-time help derivation of a
	// generated custom integration and rebuilds its command list. Nil
	// hides the button.
	ReprobeCommands func(id string)
	// SetShortcut records a new accelerator for an action id.
	SetShortcut func(id, accelerator string)
	// InstallFromRegistry installs a package from the npm registry.
	InstallFromRegistry func(name, constraint string)
	// SetCommandEnabled turns one of an integration's commands on or off: a
	// command only appears in the launcher while its switch is on.
	SetCommandEnabled func(extensionID, commandID string, enabled bool)
	// SetCommandAlias records one command's alias (an empty alias removes
	// the entry) and re-hands the launcher its command list.
	SetCommandAlias func(extensionID, commandID, alias string)
	// AdoptIntegration grafts an orphan package into the repository, and
	// DeleteOrphan removes an orphan package directory.
	AdoptIntegration func(id string)
	DeleteOrphan     func(id string)
	// CheckForUpdates asks the build's update feed for a newer version, and
	// InstallUpdate downloads and installs the one it found.
	CheckForUpdates func()
	InstallUpdate   func()
	// ExportIntegrations writes the installed integrations to a file the user
	// picks, and ImportIntegrations applies one they pick.
	ExportIntegrations func()
	ImportIntegrations func()
	// Detected lists the programs the machine has that are not yet connected,
	// and ConnectDetected connects the one the user picked. Nil hides the
	// section.
	Detected        func() []DetectedTool
	ConnectDetected func(tool DetectedTool)
	// Freshness reports what is known about one integration's command list
	// (the help-probe sidecar and the health report); nil hides the row.
	Freshness func(id string) (extensions.Freshness, bool)
	// CopyText puts text on the clipboard, for the About page's deep links.
	CopyText func(text string)
	// ConnectRecommended installs one of the shipped tool packages, and
	// ConnectLocalTool connects a program already on this machine.
	ConnectRecommended func(id string)
	ConnectLocalTool   func(tool CustomTool)
	// ChooseProgram asks the user to point at a program; nil hides the picker.
	ChooseProgram func() (string, error)
	// SaveConfiguration validates and stores an integration's configuration
	// values, moving its password fields into the secrets file.
	SaveConfiguration func(id string, values map[string]any) error
	// ClearClipboardHistory drops every clipboard entry that is not a
	// favourite.
	ClearClipboardHistory func()
	// ResetShortcuts restores the shipped bindings, and
	// SetShortcutsSuspended releases the global shortcuts while a recorder
	// waits for a key (so pressing the current one is recorded rather than
	// acted on).
	ResetShortcuts        func()
	SetShortcutsSuspended func(suspended bool)
	// CustomShortcuts reports the user-defined global shortcuts, and
	// SetCustomShortcuts persists a new list and registers it, returning the
	// keys the system refused.
	CustomShortcuts    func() []settings.CustomShortcut
	SetCustomShortcuts func([]settings.CustomShortcut) []CustomShortcutRejection
	// SetPage records the page the user switched to (the
	// `last_settings_page` setting).
	SetPage func(name string)
	// BrowserTargets lists the browsers the browser plugin can be pointed at,
	// discovered from the machine; nil offers only the automatic choice.
	BrowserTargets func() []i18n.Option
}

// Integration is one installed extension, as the Integrations page lists it.
type Integration struct {
	ID          string
	Name        string
	Description string
	Publisher   string
	Version     string
	ToolVersion string

	Enabled bool
	// Running means it contributes commands; Broken that its runtime is
	// unusable; Error why its provider could not be asked.
	Running bool
	Broken  bool
	Error   string
	// Orphan is a package directory no repository entry names.
	Orphan bool
	// Permissions are the ids the package declares, and Enforced which of
	// them the host itself decides.
	Permissions []string
	Enforced    map[string]bool
	// Diagnosis is what the last health check said, empty when none ran.
	Diagnosis string
	// DiagnosisFailed marks a check that reported a problem.
	DiagnosisFailed bool
	// Health is the stored lifecycle probe report's status, empty when the
	// manifest declares no probes. It survives across runs (the repository
	// keeps it), so a row shows how the tool was last found to be.
	Health string
	// Commands are the commands the integration declares, each with the
	// switch state the launcher reads.
	Commands []Command
	// Config is the schema the integration declares for its host-owned
	// configuration, and ConfigValues the values this machine holds (a
	// password's real value, which the form hides as it is typed). Both are
	// empty when the integration declares none.
	Config       []ConfigField
	ConfigValues map[string]any
	// Generated marks an integration the host itself created from a local
	// tool's own help: only these can be re-probed, so only these get the
	// button.
	Generated bool
}

// ConfigField is one field of an integration's host-owned configuration, as
// the settings form renders it.
type ConfigField struct {
	Key         string
	Label       string
	Type        string // text, password, path, select, multiSelect, boolean, number
	Description string
	Required    bool
	Default     any
	Options     []string
	Minimum     *float64
	Maximum     *float64
	MinLength   *int
	MaxLength   *int
}

// Command is one of an integration's commands, as the switch list shows it.
type Command struct {
	ID          string
	Name        string
	Description string
	// Enabled is whether the command may be summoned from the launcher.
	Enabled bool
	// Available is whether the integration's runtime resolves right now: a
	// command whose runtime is missing is still listed, and says so.
	Available bool
	// Alias is what the user types to summon this command. Taken marks an
	// alias another command claimed first — the conflict policy is first
	// come first served, so the field keeps the user's text but the row says
	// it is not live.
	Alias string
	Taken bool
}

// UninstallComponents says which parts of an installed integration to remove.
// The program always goes; the three data categories are the user's choice.
type UninstallComponents struct {
	// RemoveHostConfig removes the host-owned configuration (config.json and
	// the secrets generations).
	RemoveHostConfig bool
	// RemoveToolData removes the tool's own data (sessions, completions,
	// health).
	RemoveToolData bool
	// RemoveArtifacts removes generated artifacts (logs, caches,
	// user-generated files).
	RemoveArtifacts bool
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

	// cardRows counts the rows the card being built has drawn, so each row
	// after the first draws the rule above itself. It is the build's own
	// cursor, reset by card and never read outside it.
	cardRows int

	// disableConfirm marks the integrations whose switch was turned off and
	// is waiting for the user's answer.
	disableConfirm map[string]bool

	// The local-tool form's draft: what the user has typed, and the
	// permissions they have ticked. It lives on the panel so a keystroke
	// survives the rebuild, and it is cleared once the tool is connected.
	toolOpen        bool
	toolName        string
	toolProgram     string
	toolCommand     string
	toolArgs        string
	toolVersionArgs string
	toolVersion     string
	toolDescription string
	toolOutput      string
	toolPermissions map[string]bool

	// Sessions reports the running sessions; nil when the shell has none
	// (tests).
	Sessions func() []Session
	// Recommended reports the shipped tool packages and whether each is
	// installed; nil when the shell has none (tests).
	Recommended func() []RecommendedTool
	// Integrations reports the installed extensions; nil when the shell has
	// none (tests).
	Integrations func() []Integration
	// Shortcut is the global summon key the shell registered, and
	// ShortcutID the action it belongs to.
	Shortcut   string
	ShortcutID string
	// recording is set while the recorder waits for a key combination.
	recording bool

	// Page is the chosen page, an index into the sidebar.
	Page int

	// UpdateStatus is what the last update check said, and UpdateReady
	// whether an update is waiting to be installed.
	UpdateStatus string
	UpdateReady  bool
	// uninstallOpen is which integrations have the componentized uninstall
	// dialog open, and uninstallDraft its checkboxes.
	uninstallOpen  map[string]bool
	uninstallDraft map[string]UninstallComponents
	// auditOpen is which integrations have their permission audit unfolded,
	// configOpen which configuration forms are open, configDrafts their form
	// state, and configError the last save's error.
	auditOpen    map[string]bool
	configOpen   map[string]bool
	configDrafts map[string]*configDraft
	configError  map[string]string
	// shortcutsSuspended is the last state reported to the shell, so the
	// recorder releases the global keys once and takes them back once.
	shortcutsSuspended bool
	// TransferStatus is what the last export or import said.
	TransferStatus string

	// Sidebar holds the page list's identity, its choice and its focus.
	Sidebar ui.ListState
	// Body keeps the form's scroll offset across frames.
	Body ui.ScrollState

	// installName and installVersion are the npm install field's contents.
	installName    string
	installVersion string

	// The custom-shortcut row being made: the recorded key, the chosen action
	// and, for a command line, its text. customFeedback is the last rejection.
	customRecording bool
	customKey       string
	customAction    string
	customCommand   string
	customFeedback  string
}

// CustomShortcutRejection is a key the system would not bind.
type CustomShortcutRejection struct {
	Key    string
	Reason string
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
	case PagePlugins:
		return page{c.PagePlugins, c.PagePluginsHint}
	case PageIntegrations:
		return page{c.PageIntegrations, c.PageIntegrationsHint}
	case PageAbout:
		return page{c.PageAbout, c.PageAboutHint}
	default:
		return page{c.PageGeneral, c.PageGeneralHint}
	}
}

// syncShortcutSuspension tells the shell whether a recorder is waiting for a
// key. While one is, the global shortcuts are released: pressing the key that
// currently summons floter must be recorded, not acted on.
func (a *App) syncShortcutSuspension() {
	if a.Actions.SetShortcutsSuspended == nil {
		return
	}
	suspended := a.recording || a.customRecording
	if suspended == a.shortcutsSuspended {
		return
	}
	a.shortcutsSuspended = suspended
	a.Actions.SetShortcutsSuspended(suspended)
}

// View builds the settings surface: the page list beside the body.
func (a *App) View(c *ui.Context) {
	defer a.syncShortcutSuspension()
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	t := c.Theme()

	// While a recorder waits, Escape belongs to it: a shortcut of the view is
	// handled before the focused element's input, so both recorders have to
	// yield or their cancel key would close the panel instead.
	if !a.recording && !a.customRecording && c.Shortcut(0, ui.KeyEscape) && a.Actions.Close != nil {
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

// permissionTier draws one tier of the permission audit: its label and the
// one-line explanation of what the tier means, then a row per permission. An
// enforced permission's row is a plane with the accent edge — a *check* the
// host owns — while a declared one is the plain neutral pane: an FYI, not a
// promise.
func (a *App) permissionTier(c *ui.Context, label, hint string, enforced bool, language string, permissions []string) {
	if len(permissions) == 0 {
		return
	}
	t := c.Theme()
	tokens := a.tokens(c)
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Text(c, label).FontSize(t.FontSize - 1).FontWeight(650).TextColor(tokens.TextStrong)
		if hint != "" {
			ui.Text(c, hint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
		a.card(c, func() {
			for _, permission := range permissions {
				permission := permission
				a.row(c, i18n.PermissionLabel(language, permission), i18n.PermissionDescription(language, permission), func() {
					mark := "\u25cb"
					if enforced {
						mark = "\u25cf"
					}
					color := t.TextMuted
					if enforced {
						color = t.Accent
					}
					ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)
					ui.Text(c, mark).FontSize(t.FontSize - 3).TextColor(color)
				})
			}
		})
	})
}

// mustGlyph is the shared glyph vocabulary with a hard failure: the names here
// are program constants, so a missing one is a bug rather than a runtime
// condition.
func mustGlyph(name string) *ui.SVG {
	glyph, ok := launcher.Glyph(name)
	if !ok {
		glyph, _ = launcher.Glyph("alert")
	}
	return glyph
}

// sidebarRow builds one page entry: a leading mark, the page's name at weight
// 580, and the chosen page on the resting control fill — the sidebar's own
// selection, since the list is not asked to paint one.
func (a *App) sidebarRow(c *ui.Context, i int) {
	t := c.Theme()
	tokens := a.tokens(c)
	selected := i == a.Page
	row := ui.Row(c).FillWidth().MinHeight(t.Space(8.5)).
		Padding(0, t.Space(2)).Gap(t.Space(2)).
		Radius(tokens.RadiusSM).AlignItems(ui.Center).
		Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	if selected {
		row.Background(tokens.Control)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	color := t.TextMuted
	if selected {
		color = tokens.TextStrong
	}
	row.Children(func() {
		if glyph, ok := pageGlyph(i); ok {
			ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(color)
		}
		ui.Text(c, a.page(i).Title).FontSize(t.FontSize).FontWeight(580).
			TextColor(color).Ellipsis("\u2026").SingleLine()
	})
	if row.Clicked() {
		a.selectPage(i)
	}
}

// pageGlyph is the mark a settings page leads with: the same vocabulary the
// launcher's rows speak, so a page and the thing it configures look alike.
func pageGlyph(page int) (*ui.SVG, bool) {
	switch page {
	case PageGeneral:
		return launcher.Glyph("settings")
	case PageSessions:
		return launcher.Glyph("terminal")
	case PageShortcuts:
		return launcher.Glyph("star")
	case PagePlugins:
		return launcher.Glyph("globe")
	case PageIntegrations:
		return launcher.Glyph("file")
	case PageAbout:
		return launcher.Glyph("alert")
	}
	return nil, false
}

// selectPage switches pages and remembers the choice, so the next visit to
// the settings surface reopens where the user left off.
func (a *App) selectPage(page int) {
	a.Page = page
	if a.Actions.SetPage != nil {
		a.Actions.SetPage(PageName(page))
	}
}

// body builds the chosen page: a hint pinned over the content, which
// scrolls under it behind a soft scroll edge.
func (a *App) body(c *ui.Context, copy i18n.Settings) {
	p := a.page(a.Page)
	t := c.Theme()

	// The page's own header: its name at the display register and the one-line
	// hint under it, as the reference's `.settings-page__title` /
	// `.settings-page__subtitle` did.
	header := t.Space(10)
	edge := header + t.Space(2)
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c.Key("settings.body")).TrackScroll(&a.Body).Fill().
			Padding(edge, 0, 0, 0).Children(func() {
			switch a.Page {
			case PageGeneral:
				a.general(c, copy)
			case PageSessions:
				a.sessions(c, copy)
			case PageIntegrations:
				a.integrations(c, copy)
			case PageShortcuts:
				a.shortcuts(c, copy)
			case PagePlugins:
				a.plugins(c, copy)
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
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Height(header).
			Padding(0, 0, t.Space(1), 0).Children(func() {
			ui.Text(c, p.Title).FontSize(t.FontSize + 4).FontWeight(650).TextColor(t.Text)
			if p.Hint != "" {
				ui.Text(c, p.Hint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
			ui.Divider(c).Padding(t.Space(1), 0)
		})
	})
}

// plugins draws the built-in plugins' own settings: the browser plugin's
// block and the clipboard's switch and capacity. Every control writes through
// the store, so a change lands in settings.json at once and the shell's
// listener applies it.
func (a *App) plugins(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	browser := settings.BrowserPluginOf(a.Store.Snapshot())
	clipboard := settings.ClipboardOf(a.Store.Snapshot())

	ui.Column(c).FillWidth().Gap(t.Space(4)).Children(func() {
		a.section(c, copy.BrowserPlugin, "", func() {
			a.card(c, func() {
				a.checkbox(c, copy.BrowserEnabled, browser.Enabled, func(on bool) {
					a.setBrowser(func(p *settings.BrowserPlugin) { p.Enabled = on })
				})
				a.choose(c, copy.BrowserTarget, copy.BrowserTargetHint, a.browserTargets(), browser.Target,
					func(id string) { a.setBrowser(func(p *settings.BrowserPlugin) { p.Target = id }) })
				a.text(c, copy.BrowserCustomDir, copy.BrowserCustomDirHint, "/path/to/profile", browser.CustomBaseDir,
					func(value string) { a.setBrowser(func(p *settings.BrowserPlugin) { p.CustomBaseDir = value }) })
				a.slider(c, copy.BrowserHistoryDays, copy.BrowserHistoryDaysHint, float64(browser.HistoryDays), 0, 365,
					func(v float64) string {
						if v < 1 {
							return copy.BrowserHistoryAll
						}
						return fmt.Sprintf("%d", int(v))
					},
					func(v float64) { a.setBrowser(func(p *settings.BrowserPlugin) { p.HistoryDays = int(v) }) })
				a.choose(c, copy.BrowserSort, copy.BrowserSortHint, copy.BrowserSortOrders, browser.SortOrder,
					func(id string) { a.setBrowser(func(p *settings.BrowserPlugin) { p.SortOrder = id }) })
				a.choose(c, copy.BrowserSearchField, copy.BrowserSearchFieldHint, copy.BrowserSearchFields, browser.SearchField,
					func(id string) { a.setBrowser(func(p *settings.BrowserPlugin) { p.SearchField = id }) })
			})
			// The CDP block is its own card: it configures a different
			// transport, not another browser option.
			a.card(c, func() {
				a.checkbox(c, copy.BrowserCDPEnabled, browser.CDPEnabled, func(on bool) {
					a.setBrowser(func(p *settings.BrowserPlugin) { p.CDPEnabled = on })
				})
				a.text(c, copy.BrowserCDPPort, copy.BrowserCDPPortHint, "9222", fmt.Sprintf("%d", browser.CDPPort),
					func(value string) {
						port, err := strconv.Atoi(strings.TrimSpace(value))
						if err != nil {
							return
						}
						a.setBrowser(func(p *settings.BrowserPlugin) { p.CDPPort = port })
					})
			})
		})
		a.section(c, copy.ClipboardPlugin, "", func() {
			a.card(c, func() {
				a.checkbox(c, copy.ClipboardEnabled, clipboard.Enabled, func(on bool) {
					a.set(func(s *settings.Settings) {
						state := settings.ClipboardOf(*s)
						state.Enabled = on
						s.SetClipboard(state)
					})
				})
				a.slider(c, copy.ClipboardMaxItems, copy.ClipboardMaxItemsHint, float64(clipboard.MaxItems),
					float64(settings.MinClipboardMaxItems), float64(settings.MaxClipboardMaxItems),
					func(v float64) string { return fmt.Sprintf("%d", int(v)) },
					func(v float64) {
						a.set(func(s *settings.Settings) {
							state := settings.ClipboardOf(*s)
							state.MaxItems = int(v)
							s.SetClipboard(state)
						})
					})
				if a.Actions.ClearClipboardHistory != nil {
					labelClicked := a.row(c, copy.ClipboardClear, copy.ClipboardClearHint, func() {
						if ui.Button(c, copy.ClipboardClear).Clicked() {
							a.Actions.ClearClipboardHistory()
						}
					})
					if labelClicked {
						a.Actions.ClearClipboardHistory()
					}
				}
			})
		})
		a.calculatorCard(c, copy)
	})
}

// calculatorCard draws the calculator plugin's block: the capacity, the age
// window and what Enter copies.
func (a *App) calculatorCard(c *ui.Context, copy i18n.Settings) {
	calculator := settings.CalculatorPluginOf(a.Store.Snapshot())
	retention := make([]i18n.Option, 0, len(settings.CalculatorRetentionDays))
	for _, days := range settings.CalculatorRetentionDays {
		retention = append(retention, i18n.Option{ID: fmt.Sprintf("%d", days), Label: copy.CalculatorRetentionDays(days)})
	}
	a.section(c, copy.CalculatorPlugin, copy.CalculatorPluginHint, func() {
		a.card(c, func() {
			a.slider(c, copy.CalculatorMaxItems, copy.CalculatorMaxItemsHint, float64(calculator.MaxItems),
				float64(settings.MinCalculatorMaxItems), float64(settings.MaxCalculatorMaxItems),
				func(v float64) string { return fmt.Sprintf("%d", int(v)) },
				func(v float64) {
					a.setCalculator(func(p *settings.CalculatorPlugin) { p.MaxItems = int(v) })
				})
			a.choose(c, copy.CalculatorRetention, copy.CalculatorRetentionHint, retention,
				fmt.Sprintf("%d", calculator.RetentionDays),
				func(id string) {
					days, err := strconv.Atoi(id)
					if err != nil {
						return
					}
					a.setCalculator(func(p *settings.CalculatorPlugin) { p.RetentionDays = days })
				})
			a.pick(c, copy.CalculatorCopyMode, copy.CalculatorCopyModeHint, copy.CalculatorCopyModes,
				calculator.CopyMode,
				func(id string) { a.setCalculator(func(p *settings.CalculatorPlugin) { p.CopyMode = id }) })
		})
	})
}

// setCalculator writes one change to the calculator plugin's block.
func (a *App) setCalculator(mutate func(*settings.CalculatorPlugin)) {
	a.set(func(s *settings.Settings) {
		plugin := settings.CalculatorPluginOf(*s)
		mutate(&plugin)
		s.SetCalculatorPlugin(plugin)
	})
}

// setBrowser writes one change to the browser plugin's block.
func (a *App) setBrowser(mutate func(*settings.BrowserPlugin)) {
	a.set(func(s *settings.Settings) {
		plugin := settings.BrowserPluginOf(*s)
		mutate(&plugin)
		s.SetBrowserPlugin(plugin)
	})
}

// browserTargets lists the target choices: the automatic one first, then the
// browsers discovery found.
func (a *App) browserTargets() []i18n.Option {
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	options := []i18n.Option{{ID: settings.DefaultBrowserTarget, Label: copy.BrowserAuto}}
	if a.Actions.BrowserTargets != nil {
		options = append(options, a.Actions.BrowserTargets()...)
	}
	return options
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
	ui.Column(c).FillWidth().Gap(t.Space(4)).Children(func() {
		a.section(c, copy.PageSessions, copy.SessionsActive, func() {
			a.card(c, func() {
				for _, session := range running {
					session := session
					a.row(c, session.Title, copy.SessionsRunning, func() {
						if a.Actions.CloseSession != nil {
							if ui.Button(c, copy.SessionsClose).Clicked() {
								a.Actions.CloseSession()
							}
						}
					})
				}
			})
		})
	})
}

// integrations lists the installed extensions, each with its state and a
// switch. An orphan package directory is listed too, with the two operations
// it can take.
func (a *App) integrations(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	integrations := a.installedIntegrations()
	ui.Column(c).FillWidth().Gap(t.Space(4)).Children(func() {
		ui.Text(c, copy.IntegrationsHint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		a.localToolSection(c, copy)
		a.detectedSection(c, copy)
		a.recommendedRow(c, copy)
		a.transferRow(c, copy)
		a.installRow(c, copy)
		// The PATH commands' own switch sits with the integrations, as the old
		// panel had it: it decides whether the search offers them at all.
		a.card(c, func() {
			a.checkbox(c, copy.ShowCommandsInSearch, settings.ShowCommandsInSearch(a.Store.Snapshot()),
				func(on bool) {
					a.set(func(s *settings.Settings) { s.SetShowCommandsInSearch(on) })
				})
		})
		if len(integrations) == 0 {
			ui.Column(c).FillWidth().Padding(t.Space(4)).Center().Children(func() {
				ui.Text(c, copy.IntegrationsEmpty).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
			return
		}
		a.section(c, copy.IntegrationsInstalled, "", func() {
			for _, integration := range integrations {
				a.integrationRow(c, copy, integration)
			}
		})
	})
}

// RecommendedTool is one shipped package, as the page lists it.
type RecommendedTool struct {
	ID          string
	Name        string
	Description string
	// Installed is true when the inventory already has it.
	Installed bool
}

// recommendedRow lists the shipped tool packages that are not installed yet,
// each with the one press that installs it. Nothing is shown once everything
// shipped is installed.
func (a *App) recommendedRow(c *ui.Context, copy i18n.Settings) {
	if a.Actions.ConnectRecommended == nil || a.Recommended == nil {
		return
	}
	var pending []RecommendedTool
	for _, tool := range a.Recommended() {
		if !tool.Installed {
			pending = append(pending, tool)
		}
	}
	if len(pending) == 0 {
		return
	}
	a.section(c, copy.IntegrationsRecommended, copy.IntegrationsRecommendedHint, func() {
		a.card(c, func() {
			for _, tool := range pending {
				tool := tool
				a.row(c, tool.Name, tool.Description, func() {
					if ui.Button(c, copy.IntegrationsConnect).Clicked() {
						a.Actions.ConnectRecommended(tool.ID)
					}
				})
			}
		})
	})
}

// transferRow is the export and import of the whole list: a backup of what is
// installed and how it is configured, and the way back in on another machine.
func (a *App) transferRow(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).
		Padding(0, 0, t.Space(1), 0).Children(func() {
		if a.Actions.ExportIntegrations != nil {
			if ui.Button(c, copy.IntegrationsExport).Clicked() {
				a.TransferStatus = ""
				a.Actions.ExportIntegrations()
			}
		}
		if a.Actions.ImportIntegrations != nil {
			if ui.Button(c, copy.IntegrationsImport).Clicked() {
				a.TransferStatus = ""
				a.Actions.ImportIntegrations()
			}
		}
		if a.TransferStatus != "" {
			ui.Text(c, a.TransferStatus).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
	})
}

// installRow is the npm install field: a package name, an optional version,
// and the button that asks the shell to install it.
func (a *App) installRow(c *ui.Context, copy i18n.Settings) {
	if a.Actions.InstallFromRegistry == nil {
		return
	}
	t := c.Theme()
	name, version := a.installName, a.installVersion
	requested := false
	ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).
		Padding(0, 0, t.Space(1), 0).Children(func() {
		ui.TextInput(c, &name).Placeholder(copy.IntegrationsPackageHint).Label(copy.IntegrationsPackageHint).Grow(1)
		ui.TextInput(c, &version).Placeholder(copy.IntegrationsVersionHint).Width(160)
		if ui.Button(c, copy.IntegrationsInstall).Clicked() {
			requested = true
		}
	})
	a.installName, a.installVersion = name, version
	if requested && strings.TrimSpace(name) != "" {
		a.Actions.InstallFromRegistry(strings.TrimSpace(name), strings.TrimSpace(version))
		a.installName, a.installVersion = "", ""
		a.Body.Y = 0
	}
}

// integrationRow is one integration as a card: its identity and state, what
// it declares, what it can run (each command a row with its switch and its
// alias), and the row of doors — check, re-scan, uninstall.
func (a *App) integrationRow(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := c.Theme()
	a.card(c, func() {
		a.row(c, integration.Name, integration.Description, func() {
			ui.Text(c, integration.state(copy)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		})
		a.row(c, integration.identity(), "", func() {})

		if len(integration.Permissions) > 0 && !integration.Orphan {
			labelClicked := a.row(c, copy.IntegrationsPermissions, a.permissionLine(integration, copy), func() {
				if ui.Button(c, copy.IntegrationsPermissions).Clicked() {
					a.toggleAudit(integration.ID)
				}
			}, func() { a.permissionAudit(c, copy, integration) })
			if labelClicked {
				a.toggleAudit(integration.ID)
			}
		}

		// The freshness of the command list: when it was last derived, how
		// that went, and how the list moved. Nothing is shown for an
		// integration nothing has ever probed — "unknown" is its own state,
		// not a row of zeroes.
		if freshness, ok := a.freshnessOf(integration); ok {
			a.row(c, copy.Freshness, a.freshnessDetail(copy, freshness), func() {
				ui.Text(c, freshnessState(copy, freshness)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			})
		}

		if integration.Diagnosis != "" {
			color := t.TextMuted
			if integration.DiagnosisFailed {
				color = t.Danger
			}
			a.row(c, copy.IntegrationsHealth, integration.Diagnosis, func() {
				ui.Text(c, integration.state(copy)).FontSize(t.FontSize - 1).TextColor(color)
			})
		}
		if integration.Error != "" {
			a.row(c, copy.IntegrationsHealth, integration.Error, func() {
				ui.Text(c, "\u25cf").FontSize(t.FontSize - 3).TextColor(t.Danger)
			})
		}

		if len(integration.Config) > 0 && !integration.Orphan {
			opened := a.row(c, copy.ConfigOpen, "", func() {
				if ui.Button(c, copy.ConfigOpen).Clicked() {
					a.toggleConfig(integration.ID)
				}
			}, func() { a.configFields(c, copy, integration) })
			if opened {
				a.toggleConfig(integration.ID)
			}
		}

		if len(integration.Commands) > 0 && !integration.Orphan {
			for _, command := range integration.Commands {
				command := command
				label := command.Name
				if label == "" {
					label = command.ID
				}
				// A command whose runtime does not resolve says so on its own
				// line rather than lying about being ready.
				if !command.Available {
					label += "  \u00b7  " + copy.IntegrationsUnavailable
				}
				on := command.Enabled
				changed := false
				labelClicked := a.row(c, label, "", func() {
					if ui.Switch(c, &on).Label(label).Changed() {
						changed = true
					}
				}, func() {
					// The alias editor rides the command list itself: "this
					// command, this alias" is edited where the command is
					// already named, and the value lands on submit.
					a.row(c, copy.IntegrationsCommandAlias, "", func() {
						alias := command.Alias
						if ui.TextInput(c, &alias).Placeholder(copy.IntegrationsCommandAliasPlaceholder).
							Label(copy.IntegrationsCommandAlias).Width(180).Submitted() &&
							a.Actions.SetCommandAlias != nil {
							a.Actions.SetCommandAlias(integration.ID, command.ID, alias)
						}
						if command.Taken {
							ui.Text(c, copy.IntegrationsCommandAliasTaken).
								FontSize(t.FontSize - 1).TextColor(t.Danger)
						}
					})
				})
				if labelClicked {
					on = !on
					changed = true
				}
				if changed && a.Actions.SetCommandEnabled != nil {
					a.Actions.SetCommandEnabled(integration.ID, command.ID, on)
				}
			}
		}

		if integration.Orphan {
			// An orphan is a package directory the repository does not name:
			// the row offers to graft it in or to remove it, and nothing
			// else — there is no record to enable, check or uninstall.
			a.row(c, copy.IntegrationsOrphan, "", func() {
				if a.Actions.AdoptIntegration != nil {
					if ui.Button(c, copy.IntegrationsAdopt).Clicked() {
						a.Actions.AdoptIntegration(integration.ID)
					}
				}
				if a.Actions.DeleteOrphan != nil {
					if ui.Button(c, copy.IntegrationsDeleteOrphan).Clicked() {
						a.Actions.DeleteOrphan(integration.ID)
					}
				}
			})
			return
		}

		enabled := integration.Enabled
		enableChanged := false
		enableLabelClicked := a.row(c, copy.IntegrationsEnable, "", func() {
			if a.Actions.SetIntegrationEnabled != nil {
				on := enabled
				if ui.Switch(c, &on).Label(copy.IntegrationsEnable).Changed() {
					// Turning an integration *off* stops whatever it is
					// running, so it asks first — a mark the user did not mean
					// is a command dead mid-flight. Turning one on is no
					// question at all.
					if !on {
						a.requestDisable(integration.ID)
					} else {
						enabled, enableChanged = on, true
					}
				}
			}
			if a.Actions.DiagnoseIntegration != nil {
				if ui.Button(c, copy.IntegrationsCheck).Clicked() {
					a.Actions.DiagnoseIntegration(integration.ID)
				}
			}
			// A generated custom integration's command list came from the
			// tool's own help at connect time; the button re-runs that
			// derivation, so a tool that shipped new subcommands comes back
			// with them without a disconnect.
			if integration.Generated && a.Actions.ReprobeCommands != nil {
				if ui.Button(c, copy.IntegrationsReprobe).Clicked() {
					a.Actions.ReprobeCommands(integration.ID)
				}
			}
			if a.Actions.UninstallComponents != nil {
				a.uninstallComponentsRow(c, copy, integration)
			} else if a.Actions.UninstallIntegration != nil {
				if ui.Button(c, copy.IntegrationsUninstall).Clicked() {
					a.Actions.UninstallIntegration(integration.ID, integration.Name)
				}
			}
		})
		if enableLabelClicked {
			if enabled {
				a.requestDisable(integration.ID)
			} else {
				enabled, enableChanged = !enabled, true
			}
		}
		if enableChanged && a.Actions.SetIntegrationEnabled != nil {
			a.Actions.SetIntegrationEnabled(integration.ID, enabled)
		}
		if a.disableConfirm[integration.ID] {
			a.disableConfirmation(c, copy, integration)
		}
	})
}

// toggleConfig folds or unfolds one integration's configuration form.
func (a *App) toggleConfig(id string) {
	if a.configOpen == nil {
		a.configOpen = map[string]bool{}
	}
	a.configOpen[id] = !a.configOpen[id]
	a.configError[id] = ""
}

// toggleAudit folds or unfolds one integration's permission audit.
func (a *App) toggleAudit(id string) {
	if a.auditOpen == nil {
		a.auditOpen = map[string]bool{}
	}
	a.auditOpen[id] = !a.auditOpen[id]
}

// permissionAudit draws the per-permission explanation under a row: what each
// permission allows, and whether the host itself decides it. The list is folded
// away until the user asks for it, so a row stays a row.
func (a *App) permissionAudit(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := c.Theme()
	language := a.Store.Snapshot().Language
	if ui.Button(c, copy.IntegrationsPermissions).Clicked() {
		if a.auditOpen == nil {
			a.auditOpen = map[string]bool{}
		}
		a.auditOpen[integration.ID] = !a.auditOpen[integration.ID]
	}
	if !a.auditOpen[integration.ID] {
		return
	}
	// Two tiers, the host's own decision first: what floter refuses until it
	// is approved, then what the tool merely declares. The split is the
	// message — a user must be able to tell a check from an FYI.
	var enforced, disclosure []string
	for _, permission := range integration.Permissions {
		if integration.Enforced[permission] {
			enforced = append(enforced, permission)
			continue
		}
		disclosure = append(disclosure, permission)
	}
	ui.Column(c).FillWidth().Gap(t.Space(3)).Padding(0, 0, t.Space(1), 0).Children(func() {
		a.permissionTier(c, copy.PermissionsEnforcedLabel, copy.PermissionsEnforcedHint, true, language, enforced)
		a.permissionTier(c, copy.PermissionsDisclosureLabel, copy.PermissionsDisclosureHint, false, language, disclosure)
	})
}

// permissionLine names the package's permissions, marking the ones the host
// itself decides.
func (a *App) permissionLine(integration Integration, copy i18n.Settings) string {
	language := a.Store.Snapshot().Language
	parts := make([]string, 0, len(integration.Permissions))
	for _, permission := range integration.Permissions {
		label := i18n.PermissionLabel(language, permission)
		if integration.Enforced[permission] {
			label += " (" + copy.PermissionsEnforced + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " · ")
}

// state is the integration's one-word state.
func (i Integration) state(copy i18n.Settings) string {
	switch {
	case i.Orphan:
		return copy.IntegrationsOrphan
	case i.Broken:
		return copy.IntegrationsBroken
	case i.Enabled:
		return copy.IntegrationsRunning
	default:
		return copy.IntegrationsOff
	}
}

// identity is the publisher and version line.
func (i Integration) identity() string {
	parts := []string{}
	if i.Publisher != "" {
		parts = append(parts, i.Publisher)
	}
	if i.Version != "" {
		parts = append(parts, i.Version)
	}
	if i.ToolVersion != "" && i.ToolVersion != i.Version {
		parts = append(parts, "tool "+i.ToolVersion)
	}
	if i.ID != "" {
		parts = append(parts, i.ID)
	}
	return strings.Join(parts, " · ")
}

func (a *App) installedIntegrations() []Integration {
	if a.Integrations == nil {
		return nil
	}
	return a.Integrations()
}

// shortcuts lists the shortcuts the app answers, with the key each one
// currently holds and a recorder for the one the app can change.
func (a *App) shortcuts(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(4)).Children(func() {
		a.section(c, copy.ShortcutsToggle, copy.ShortcutsHint, func() {
			a.card(c, func() {
				a.shortcutRow(c, copy)
			})
			if a.Actions.ResetShortcuts != nil {
				a.card(c, func() {
					labelClicked := a.row(c, copy.ShortcutsReset, "", func() {
						a.action(c, copy.ShortcutsReset, func() {})
					})
					_ = labelClicked
				})
			}
		})
		a.section(c, copy.ShortcutsApp, copy.ShortcutsAppHint, func() {
			a.card(c, func() {
				for _, action := range settings.ShortcutActions {
					if action == settings.ShortcutToggleWindow {
						continue // the summon key has its own card above
					}
					a.shortcutMapRow(c, copy, action)
				}
			})
		})
		a.customSection(c, copy)
	})
}

// shortcutMapRow is one of the app's own keys: its name, its binding, and a
// recorder that rebinds that action.
func (a *App) shortcutMapRow(c *ui.Context, copy i18n.Settings, action string) {
	a.row(c, copy.ShortcutNames[action], "", func() {
		a.shortcutControl(c, copy, action, a.binding(action))
	})
}

// binding is one action's accelerator, from the settings.
func (a *App) binding(action string) string {
	return settings.Shortcut(a.Store.Snapshot(), action)
}

// shortcutRow is one shortcut: its name, its keys, and the recorder.
func (a *App) shortcutRow(c *ui.Context, copy i18n.Settings) {
	a.row(c, copy.ShortcutsToggle, "", func() {
		a.shortcutControl(c, copy, "", a.summonShortcut())
	})
}

// shortcutControl is a row's trailing slot: the recorder while the action is
// being recorded (capturing the next chord, Escape to give up), the current
// keys and the Record button otherwise.
func (a *App) shortcutControl(c *ui.Context, copy i18n.Settings, action, value string) {
	t := c.Theme()
	recording := a.recording && (action == "" || a.ShortcutID == action)
	if recording {
		capture := ui.Box(c).Focusable().Padding(t.Space(1), t.Space(2)).Radius(a.tokens(c).RadiusSM).
			Background(t.Surface).Border(1, t.Accent).Label(copy.ShortcutRecording)
		capture.Children(func() {
			ui.Text(c, copy.ShortcutRecording).FontSize(t.FontSize)
		})
		capture.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyDown {
				return false
			}
			if ev.Key == ui.KeyEscape {
				a.recording = false
				return true
			}
			accelerator, ok := shortcuts.FromKey(ev.Mods, ev.Key)
			if !ok {
				return true // a key without a modifier is not a shortcut
			}
			a.recording = false
			if a.Actions.SetShortcut != nil {
				if action == "" {
					a.Actions.SetShortcut(settings.ShortcutToggleWindow, accelerator)
				} else {
					a.Actions.SetShortcut(action, accelerator)
				}
			}
			return true
		})
		capture.Focus()
		return
	}
	ui.Text(c, shortcuts.Display(value)).FontSize(t.FontSize).TextColor(t.TextMuted)
	if ui.Button(c, copy.ShortcutRecord).Clicked() {
		a.recording, a.ShortcutID = true, action
	}
}

// updateRow is the About page's update check: the button, the line the last
// check left behind, and the install button when there is something to
// install.
func (a *App) updateRow(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
		if a.Actions.CheckForUpdates != nil {
			if ui.Button(c, copy.UpdateCheck).Clicked() {
				a.UpdateStatus = copy.UpdateChecking
				a.UpdateReady = false
				a.Actions.CheckForUpdates()
			}
		}
		if a.UpdateReady && a.Actions.InstallUpdate != nil {
			if ui.Button(c, copy.UpdateInstall).Clicked() {
				a.UpdateStatus = copy.UpdateInstalling(0)
				a.Actions.InstallUpdate()
			}
		}
		if a.UpdateStatus != "" {
			ui.Text(c, a.UpdateStatus).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
	})
}

// about shows the build's identity and where its files live.
func (a *App) about(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	about := a.About
	ui.Column(c).FillWidth().Gap(t.Space(4)).Children(func() {
		// The build's identity reads as the panel's large title; everything
		// else is rows of one card, and the update check is the card that
		// acts.
		ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
			ui.Text(c, strings.TrimSpace(about.Name+" "+about.Version)).FontSize(t.FontSize + 4).FontWeight(650)
			ui.Text(c, copy.PageAbout).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		})
		a.card(c, func() {
			a.keyValue(c, copy.AboutVersion, about.Version)
			a.keyValue(c, copy.AboutFramework, about.Framework)
			a.keyValue(c, copy.AboutScheme, about.Scheme)
			a.keyValue(c, copy.AboutSettingsFile, about.SettingsPath)
			if about.RepoURL != "" {
				a.row(c, copy.AboutProject, "", func() {
					ui.Link(c, about.RepoURL, about.RepoURL).FontSize(t.FontSize)
				})
			}
		})
		a.card(c, func() {
			a.updateRow(c, copy)
		})
		// The deep-link lines: a URL scheme is invisible when it works and
		// invisible when it does not, so the About page is the one place a
		// user can find out that floter answers `floter://` at all.
		a.section(c, copy.DeepLinkTitle, copy.DeepLinkHint, func() {
			a.card(c, func() {
				for _, link := range []string{
					"floter://connect?manifest=/path/to/tool.json",
					"floter://register?cmd=rg",
				} {
					link := link
					a.row(c, link, "", func() {
						if ui.Button(c, copy.DeepLinkCopy).Clicked() {
							if a.Actions.CopyText != nil {
								a.Actions.CopyText(link)
							}
						}
					})
				}
			})
		})
	})
}

// keyValue is one labeled line of read-only information: the label in a
// fixed column, the value beside it, selectable so it can be copied.
// keyValue is one About row: the label at the start, the selectable value at
// the end (wrapped so a long path still reads).
func (a *App) keyValue(c *ui.Context, label, value string) {
	t := c.Theme()
	if value == "" {
		return
	}
	a.row(c, label, "", func() {
		ui.Text(c, value).FontSize(t.FontSize).TextColor(t.TextMuted).Selectable()
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

	ui.Column(c).FillWidth().Gap(c.Theme().Space(4)).Children(func() {
		a.section(c, copy.GroupAppearance, "", func() {
			a.card(c, func() {
				a.pick(c, copy.Theme, "", copy.Themes, s.Theme,
					func(id string) { a.set(func(s *settings.Settings) { s.Theme = id }) })
				a.pick(c, copy.AppIcon, copy.AppIconHint, copy.AppIcons, a.appIcon(),
					func(id string) {
						a.set(func(s *settings.Settings) { s.SetExtra("app_icon", id) })
					})
				a.pick(c, copy.Language, copy.LanguageHint, copy.Languages, s.Language,
					func(id string) { a.set(func(s *settings.Settings) { s.Language = id }) })
				a.pick(c, copy.Glass, copy.GlassHint, copy.GlassSteps, s.GlassStep,
					func(id string) { a.set(func(s *settings.Settings) { s.GlassStep = id }) })
			})
			// The two transparency sliders are their own card under the same
			// title — the old panel's "two transparency groups", spaced like
			// cards rather than like rows.
			a.card(c, func() {
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
		})

		a.section(c, copy.GroupWindow, "", func() {
			a.card(c, func() {
				a.pick(c, copy.Scale, copy.ScaleHint, copy.Scales, s.UIScale,
					func(id string) { a.set(func(s *settings.Settings) { s.UIScale = id }) })
				a.checkbox(c, copy.LaunchAtStartup, s.LaunchAtStartup,
					func(on bool) { a.set(func(s *settings.Settings) { s.LaunchAtStartup = on }) })
				a.checkbox(c, copy.HideOnBlur, s.HideOnBlur,
					func(on bool) { a.set(func(s *settings.Settings) { s.HideOnBlur = on }) })
				a.checkbox(c, copy.ShowRecentInLauncher, settings.ShowRecentInLauncher(s),
					func(on bool) { a.set(func(s *settings.Settings) { s.SetShowRecentInLauncher(on) }) })
				a.checkbox(c, copy.ShowMenubarIcon, settings.ShowMenubarIcon(s),
					func(on bool) { a.set(func(s *settings.Settings) { s.SetShowMenubarIcon(on) }) })
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
				// The presets plus a free-form duration: the field says how
				// long, in seconds, and the button applies it — the old
				// panel's "Custom duration in seconds" row.
				seconds := ""
				if s.SurfaceResidencySeconds != 0 {
					seconds = strconv.FormatUint(uint64(s.SurfaceResidencySeconds), 10)
				}
				a.row(c, copy.SurfaceResidencyCustomLabel, "", func() {
					edit := seconds
					ui.TextInput(c, &edit).Label(copy.SurfaceResidencyCustomLabel).
						Placeholder(copy.SurfaceResidencyCustomUnit).Width(90)
					if ui.Button(c, copy.SurfaceResidencyCustomApply).Clicked() {
						if value, err := strconv.ParseUint(strings.TrimSpace(edit), 10, 32); err == nil {
							a.set(func(s *settings.Settings) { s.SurfaceResidencySeconds = uint32(value) })
						}
					}
				})
			})
		})

		a.section(c, copy.GroupTerminal, "", func() {
			a.card(c, func() {
				a.slider(c, copy.FontSize, "", float64(s.FontSize),
					settings.MinFontSize, settings.MaxFontSize,
					func(v float64) string { return fmt.Sprintf("%d", int(math.Round(v))) },
					func(v float64) {
						a.set(func(s *settings.Settings) { s.FontSize = int(math.Round(v)) })
					})
				// The family is a picker of the faces this machine actually
				// has (detected from its font files), plus the current value
				// so a hand-set family is never lost.
				families := make([]i18n.Option, 0, len(terminalui.FontFamilyOptions(s.FontFamily)))
				for _, family := range terminalui.FontFamilyOptions(s.FontFamily) {
					families = append(families, i18n.Option{ID: family, Label: family})
				}
				a.choose(c, copy.FontFamily, "", families, s.FontFamily,
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
	a.row(c, label, description, func() {
		e := ui.Segmented(c, &index, labels...).Label(label)
		if e.Changed() && index >= 0 && index < len(options) {
			chosen = options[index].ID
		}
	})
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
	a.row(c, label, description, func() {
		e := ui.Select(c, &selected, labels).Label(label).Width(180)
		if e.Changed() {
			for _, option := range options {
				if option.Label == selected {
					chosen = option.ID
					break
				}
			}
		}
	})
	if chosen != "" {
		apply(chosen)
	}
}

// slider builds one stacked row: the label and its hint above, the track and
// its value at full width under them, as the reference's range row.
func (a *App) slider(c *ui.Context, label, description string, value, lo, hi float64, format func(float64) string, apply func(float64)) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := t.Space(3.5)
	v := value
	chosen := false
	first := a.cardRows == 0
	a.cardRows++
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).MarginX(inset).Shrink(0)
		}
		ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(2.25), inset).Children(func() {
			ui.Text(c, label).FontSize(t.FontSize).FontWeight(580).TextColor(tokens.TextStrong)
			if description != "" {
				ui.Text(c, description).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
			ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				e := ui.Slider(c, &v, lo, hi).Grow(1)
				if e.Changed() {
					chosen = true
				}
				ui.Text(c, format(v)).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
		})
	})
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
	// The label is the row's own (the reference puts it at the start) and
	// the control is the switch at the end, so the check box here is the
	// switch alone.
	if a.row(c, label, "", func() {
		if ui.Switch(c, &value).Label(label).Changed() {
			changed = true
		}
	}) {
		// The row's own label is a hit target: clicking it toggles, as the
		// framework's Field does for its control.
		value = !value
		changed = true
	}
	if changed {
		apply(value)
	}
}

// text builds one labeled text field.
func (a *App) text(c *ui.Context, label, description, placeholder, value string, apply func(string)) {
	edit := value
	a.row(c, label, description, func() {
		e := ui.TextInput(c, &edit).Placeholder(placeholder).Label(label).Width(220)
		if e.Submitted() {
			apply(edit)
		}
	})
}

// appIcon is the stored app icon, normalized to the two the app ships (dark
// is the default).
func (a *App) appIcon() string {
	value, _ := a.Store.Snapshot().Extra()["app_icon"].(string)
	if value == "light" {
		return "light"
	}
	return "dark"
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

// CustomTool is a local tool the user wants to connect: what the form
// collected. The host mints the id and derives the commands.
type CustomTool struct {
	Name        string
	Program     string
	Command     string
	Args        string
	VersionArgs string
	Version     string
	Description string
	Output      string
	Permissions []string
}

// localToolSection is the "connect a program on this machine" form: the words
// that name it, the file itself, its arguments, and what it declares. The
// section is folded away until the user asks for it, so the page stays a list
// of what is installed.
func (a *App) localToolSection(c *ui.Context, copy i18n.Settings) {
	if a.Actions.ConnectLocalTool == nil {
		return
	}
	a.section(c, copy.LocalToolTitle, copy.LocalToolHint, func() {
		if ui.Button(c, copy.LocalToolAdd).Clicked() {
			a.toolOpen = !a.toolOpen
		}
		if !a.toolOpen {
			return
		}
		if a.toolPermissions == nil {
			a.toolPermissions = map[string]bool{}
		}
		name, program, command := a.toolName, a.toolProgram, a.toolCommand
		args, versionArgs, version := a.toolArgs, a.toolVersionArgs, a.toolVersion
		description := a.toolDescription
		a.card(c, func() {
			a.row(c, copy.LocalToolName, "", func() {
				ui.TextInput(c, &name).Label(copy.LocalToolName).Width(240).Grow(1).Placeholder("Ripgrep")
			})
			a.row(c, copy.LocalToolProgram, "", func() {
				ui.TextInput(c, &program).Label(copy.LocalToolProgram).Width(240).Grow(1).
					Placeholder("/usr/bin/rg")
				if a.Actions.ChooseProgram != nil {
					if ui.Button(c, copy.LocalToolChoose).Clicked() {
						if picked, err := a.Actions.ChooseProgram(); err == nil && picked != "" {
							program = picked
						}
					}
				}
			})
			a.row(c, copy.LocalToolCommand, copy.LocalToolCommandHint, func() {
				ui.TextInput(c, &command).Label(copy.LocalToolCommand).Width(160).Placeholder("rg")
			})
			a.row(c, copy.LocalToolArgs, copy.LocalToolArgsHint, func() {
				ui.TextInput(c, &args).Label(copy.LocalToolArgs).Width(220).Placeholder("--floter")
			})
			a.row(c, copy.LocalToolVersionArgs, copy.LocalToolVersionArgsHint, func() {
				ui.TextInput(c, &versionArgs).Label(copy.LocalToolVersionArgs).Width(160).Placeholder("--version")
			})
			a.row(c, copy.LocalToolVersion, "", func() {
				ui.TextInput(c, &version).Label(copy.LocalToolVersion).Width(120).Placeholder("0.0.1")
			})
			a.row(c, copy.LocalToolDescription, "", func() {
				ui.TextInput(c, &description).Label(copy.LocalToolDescription).Width(240).Grow(1)
			})
			a.row(c, copy.LocalToolOutput, "", func() {
				index := 0
				if a.toolOutput == routeBackground {
					index = 1
				}
				if ui.Segmented(c, &index, copy.LocalToolTerminal, copy.LocalToolBackground).Changed() {
					if index == 0 {
						a.toolOutput = ""
					} else {
						a.toolOutput = routeBackground
					}
				}
			})
			// What the tool declares: one row per permission, the host's own
			// two marked as the checks they are (see the permission tiers).
			for _, permission := range extensions.AllPermissions {
				permission := permission
				on := a.toolPermissions[permission]
				label := i18n.PermissionLabel(a.Store.Snapshot().Language, permission)
				labelClicked := a.row(c, label, "", func() {
					if ui.Switch(c, &on).Label(label).Changed() {
						a.toolPermissions[permission] = on
					}
				})
				if labelClicked {
					a.toolPermissions[permission] = !on
				}
			}
		})
		requested := ui.Button(c, copy.LocalToolConnect).Clicked()
		a.toolName, a.toolProgram, a.toolCommand = name, program, command
		a.toolArgs, a.toolVersionArgs, a.toolVersion = args, versionArgs, version
		a.toolDescription = description
		if requested {
			permissions := []string{}
			for _, permission := range extensions.AllPermissions {
				if a.toolPermissions[permission] {
					permissions = append(permissions, permission)
				}
			}
			tool := CustomTool{
				Name:        strings.TrimSpace(name),
				Program:     strings.TrimSpace(program),
				Command:     strings.TrimSpace(command),
				Args:        strings.TrimSpace(args),
				VersionArgs: strings.TrimSpace(versionArgs),
				Version:     strings.TrimSpace(version),
				Description: strings.TrimSpace(description),
				Output:      a.toolOutput,
				Permissions: permissions,
			}
			if tool.Name == "" || tool.Program == "" {
				return // the form says what is missing by staying put
			}
			a.Actions.ConnectLocalTool(tool)
			a.toolOpen = false
			a.toolName, a.toolProgram, a.toolCommand = "", "", ""
			a.toolArgs, a.toolVersionArgs, a.toolVersion, a.toolDescription = "", "", "", ""
			a.toolPermissions = map[string]bool{}
		}
	})
}

// routeBackground is the manifest's own spelling for a command whose output
// the launcher shows instead of the terminal.
const routeBackground = "background"

// freshnessOf reads what is known about an integration's command list, for
// the card's own row. Nothing is shown for an integration nothing has probed.
func (a *App) freshnessOf(integration Integration) (extensions.Freshness, bool) {
	freshness := a.Actions.Freshness
	if freshness == nil {
		return extensions.Freshness{}, false
	}
	value, ok := freshness(integration.ID)
	if !ok {
		return extensions.Freshness{}, false
	}
	return value, true
}

// freshnessState is the row's trailing word: how the last derivation went.
func freshnessState(copy i18n.Settings, freshness extensions.Freshness) string {
	switch freshness.Result {
	case extensions.FreshnessRunning:
		return copy.FreshnessResultRunning
	case extensions.FreshnessSuccess:
		return copy.FreshnessResultSuccess
	case extensions.FreshnessDegraded:
		return copy.FreshnessResultDegraded
	case extensions.FreshnessFailed:
		return copy.FreshnessResultFailed
	default:
		return copy.FreshnessResultUnknown
	}
}

// freshnessDetail is the row's grey line: when the list was last derived and
// how it moved. Every "we do not know" has its own words — a first scan has
// nothing to compare against, and a never-probed integration has no time at
// all.
func (a *App) freshnessDetail(copy i18n.Settings, freshness extensions.Freshness) string {
	parts := []string{}
	if freshness.AtSeconds > 0 {
		when := copy.FreshnessJustNow
		if age := time.Since(time.Unix(freshness.AtSeconds, 0)); age > time.Minute {
			when = copy.FreshnessAgo(humanAge(age))
		}
		source := copy.FreshnessProbeSource
		if freshness.Source == extensions.FreshnessHealth {
			source = copy.FreshnessHealthSource
		}
		parts = append(parts, when+"  \u00b7  "+source)
	} else {
		parts = append(parts, copy.FreshnessNever)
	}
	commands := copy.FreshnessCommandsUnknown
	if freshness.CommandCount != nil {
		commands = strconv.Itoa(*freshness.CommandCount)
	}
	parts = append(parts, copy.FreshnessCommands+" "+commands)
	switch freshness.Delta {
	case extensions.DeltaIncrease:
		parts = append(parts, copy.FreshnessDeltaIncrease(freshness.DeltaCount()))
	case extensions.DeltaDecrease:
		parts = append(parts, copy.FreshnessDeltaDecrease(freshness.DeltaCount()))
	case extensions.DeltaUnchanged:
		parts = append(parts, copy.FreshnessDeltaUnchanged)
	default:
		parts = append(parts, copy.FreshnessDeltaUnknown)
	}
	return strings.Join(parts, "  \u00b7  ")
}

// humanAge is a coarse age a person reads: minutes, hours, days.
func humanAge(age time.Duration) string {
	switch {
	case age >= 48*time.Hour:
		return strconv.Itoa(int(age.Hours()/24)) + "d"
	case age >= 2*time.Hour:
		return strconv.Itoa(int(age.Hours())) + "h"
	case age >= 2*time.Minute:
		return strconv.Itoa(int(age.Minutes())) + "m"
	default:
		return "1m"
	}
}

// DetectedTool is one program the machine has that is not yet connected.
type DetectedTool struct {
	// Name is what the user calls it and Path where it lives.
	Name        string
	Path        string
	Description string
	// VersionArgs is how the tool reports its version, when the caller knows
	// better than the default.
	VersionArgs string
}

// detectedSection lists the programs this machine has that nothing has
// connected yet, each with the one press that connects it: discovery is the
// first link of the same loop the install and invoke rows close, so a tool
// the machine already has does not need its path typed by hand.
func (a *App) detectedSection(c *ui.Context, copy i18n.Settings) {
	if a.Actions.Detected == nil || a.Actions.ConnectDetected == nil {
		return
	}
	tools := a.Actions.Detected()
	if len(tools) == 0 {
		return
	}
	a.section(c, copy.DetectedTitle, copy.DetectedHint, func() {
		a.card(c, func() {
			for _, tool := range tools {
				tool := tool
				a.row(c, tool.Name, tool.Description, func() {
					if ui.Button(c, copy.IntegrationsConnect).Clicked() {
						a.Actions.ConnectDetected(tool)
					}
				}, func() {
					a.row(c, tool.Path, "", func() {})
				})
			}
		})
	})
}

// requestDisable opens the confirmation for turning an integration off.
func (a *App) requestDisable(id string) {
	if a.disableConfirm == nil {
		a.disableConfirm = map[string]bool{}
	}
	a.disableConfirm[id] = true
}

// disableConfirmation is the notice under an integration's switch: stopping it
// is a mark, not a delete — the files and data stay, and it can be turned back
// on whenever the user likes — but whatever it is running stops at once, so
// the question is asked rather than assumed.
func (a *App) disableConfirmation(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := c.Theme()
	tokens := a.tokens(c)
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(2), t.Space(3.5)).
		Margin(0, t.Space(3.5)).Radius(tokens.RadiusSM).
		Background(tokens.Control).Border(1, tokens.ControlEdge).Children(func() {
		ui.Text(c, copy.DisableTitle(integration.Name)).FontSize(t.FontSize).FontWeight(580).
			TextColor(tokens.TextStrong)
		ui.Text(c, copy.DisableDescription).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, copy.DisableCancel).Clicked() {
				a.disableConfirm[integration.ID] = false
			}
			if ui.Button(c, copy.DisableConfirm).Clicked() {
				a.disableConfirm[integration.ID] = false
				if a.Actions.SetIntegrationEnabled != nil {
					a.Actions.SetIntegrationEnabled(integration.ID, false)
				}
			}
		})
	})
}
