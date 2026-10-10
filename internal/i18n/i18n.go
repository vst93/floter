// Package i18n holds floter's user-facing copy for the languages the app
// ships (en, zh).
//
// English is the source of truth, and a key missing from the other language
// is a bug rather than a blank label. That is enforced by the type system —
// every language builds the same Copy struct, so a field cannot be left out;
// the tests pin the strings a screen is allowed to show.
package i18n

// Language is a normalized language id: exactly the two the settings
// whitelist allows.
type Language string

const (
	// EN and ZH are the shipped languages.
	EN Language = "en"
	ZH Language = "zh"
)

// Normalize maps any stored language value onto the whitelist, exactly as
// settings.NormalizeLanguage does ("fr" and "" land on English).
func Normalize(language string) Language {
	if Language(language) == ZH {
		return ZH
	}
	return EN
}

// Launcher is the launcher surface's copy.
type Launcher struct {
	// Placeholder is the search field's placeholder.
	Placeholder string
	// Label names the field for assistive technology.
	Label string
	// TriggerHint nudges that a typed word is a plugin command's trigger and
	// a space enters its mode ("{name} · enter"), TriggerHintMore adding the
	// count of commands it could still become.
	TriggerHint     func(name string) string
	TriggerHintMore func(name string, more int) string
	// FilesSection heads the block of rows a dropped file's actions make,
	// and RecentSection the empty page's most-launched applications.
	FilesSection  string
	RecentSection string
	// Hint is the empty result area's message.
	Hint string
	// NoResults is shown when a query matches nothing.
	NoResults string
	// Clear names the field's clear button for assistive technology.
	Clear string
	// Copied is shown after the calculator's answer goes to the clipboard.
	Copied string
	// MenuView and MenuLauncher label the application menu's own items.
	MenuView     string
	MenuLauncher string
	// CommandModeHint is the field's hint while an extension command's
	// arguments are typed.
	CommandModeHint string
	// ResultsLabel names the results list for assistive technology.
	ResultsLabel string
	// The built-in commands the P1 catalog offers.
	CommandSettings       string
	CommandSettingsHint   string
	CommandTerminal       string
	CommandTerminalHint   string
	CommandClipboard      string
	CommandClipboardHint  string
	CommandCalculator     string
	CommandCalculatorHint string
	CommandBrowser        string
	CommandBrowserHint    string
	CommandQuit           string
	CommandQuitHint       string
	// The captured-output view: its empty state, its status words and its
	// key hint.
	OutputEmpty     string
	OutputSucceeded string
	OutputTimedOut  string
	OutputTruncated string
	OutputHint      string
	OutputListHint  string
	OutputFailed    string

	// The clipboard mode's own feedback lines: the star and the delete.
	ClipboardFavoriteFailed string
	ClipboardDeleteFailed   string
	ClipboardDeleted        string
	// HistoryDelete names a history row's delete control, and
	// CalculatorDeleted reports one that went.
	HistoryDelete          string
	CalculatorDeleted      string
	CalculatorDeleteFailed string
	// CalculatorEmpty is the calculator mode's empty state, and the two
	// failure lines report a history write that did not land.
	CalculatorEmpty          string
	CalculatorRecordFailed   string
	CalculatorFavoriteFailed string
	// The power rows: restart and shut down, and the confirmation the shell
	// shows before either.
	PowerRestart       string
	PowerRestartHint   string
	PowerShutdown      string
	PowerShutdownHint  string
	PowerConfirmTitle  func(action string) string
	PowerConfirmDetail string
	PowerConfirmButton string
	PowerCancel        string
	PowerFailed        string
	// InstallTool names the row that offers a tool's install command, and
	// InvokeFailed reports a tool that could not be started.
	InstallTool  func(name string) string
	InvokeFailed string
	// The action bar: what the field itself is asking for.
	OpenInBrowser string
	OpenInFiles   string
	OpenFile      string
	RunInShell    string
	// The files mode's three actions and its folder mark.
	FileOpen     string
	FileCd       string
	FileCopyPath string
	FileFolder   string
	// BrowserTab labels a live-tab row in the browser mode, and
	// BrowserNoProfile is the mode's empty state when no browser profile was
	// found at all.
	BrowserTab       string
	BrowserNoProfile string
	// The plugin modes' filter chips: the browser's range, the clipboard's
	// six kinds and the calculator's two.
	BrowserAll                string
	BrowserBookmarks          string
	BrowserHistory            string
	BrowserTabs               string
	ClipboardFilterAll        string
	ClipboardFilterFavorites  string
	ClipboardTypeText         string
	ClipboardTypeImage        string
	ClipboardTypeLink         string
	ClipboardTypeFiles        string
	CalculatorFilterAll       string
	CalculatorFilterFavorites string
	// ShortcutSettings and ShortcutTerminal are the accelerator labels the
	// rows show for the two surfaces that have one.
	ShortcutSettings string
}

// Option is one choice of an enumerated setting: the id stored in the
// settings file and the label the control shows.
type Option struct {
	ID    string
	Label string
}

// Settings is the settings surface's copy.
type Settings struct {
	Title string
	Close string

	PageGeneral      string
	PageSessions     string
	PageShortcuts    string
	PagePlugins      string
	PageIntegrations string
	PageAbout        string

	PageGeneralHint      string
	PageSessionsHint     string
	PageShortcutsHint    string
	PagePluginsHint      string
	PageIntegrationsHint string
	PageAboutHint        string

	// PagePlaceholder is the body of the pages P1 routes but does not fill
	// yet: the sidebar route exists, the page says so plainly.
	PagePlaceholder string

	GroupAppearance string
	GroupWindow     string
	GroupTerminal   string

	Theme               string
	AppIcon             string
	AppIconHint         string
	Language            string
	LanguageHint        string
	Glass               string
	GlassHint           string
	MainOpacity         string
	MainOpacityHint     string
	TerminalOpacity     string
	TerminalOpacityHint string
	Scale               string
	ScaleHint           string

	// The terminal's appearance, from the old GeneralPage's group.
	TerminalHint string
	FontSize     string
	FontFamily   string
	CursorShape  string
	CursorBlink  string
	LineHeight   string
	Padding      string
	Palette      string

	// The enumerated choices, each id paired with its label.
	Themes       []Option
	Languages    []Option
	GlassSteps   []Option
	Scales       []Option
	CursorShapes []Option
	Paddings     []Option
	Palettes     []Option
	AppIcons     []Option

	// The window behaviour.
	LaunchAtStartup       string
	LaunchAtStartupHint   string
	HideOnBlur            string
	HideOnBlurHint        string
	SurfaceResidency      string
	SurfaceResidencyHint  string
	SurfaceResidencyOff   string
	SurfaceResidencyNever string
	SurfaceResidencyValue func(seconds uint32) string

	// The Sessions page.
	// The launcher's own switches: whether an empty query offers the recent
	// applications, whether the tray icon shows, and whether the PATH commands
	// join the search.
	ShowRecentInLauncher     string
	ShowRecentInLauncherHint string
	ShowMenubarIcon          string
	ShowMenubarIconHint      string
	ShowCommandsInSearch     string
	ShowCommandsInSearchHint string
	SessionsNone             string
	SessionsRunning          string
	SessionsClose            string
	SessionsActive           string

	// The permission dialog the installer shows, and the permission line
	// the integrations list draws.
	PermissionsTitle    func(name string) string
	PermissionsAllow    string
	PermissionsCancel   string
	PermissionsEnforced string
	PermissionsDeclared string

	// The Integrations page.
	IntegrationsEmpty     string
	IntegrationsEnable    string
	IntegrationsRunning   string
	IntegrationsOff       string
	IntegrationsBroken    string
	IntegrationsOrphan    string
	IntegrationsHint      string
	IntegrationsUninstall string
	// The componentized uninstall dialog.
	IntegrationsUninstallTitle       string
	IntegrationsUninstallDescription string
	UninstallHostConfig              string
	UninstallHostConfigHint          string
	UninstallToolData                string
	UninstallToolDataHint            string
	UninstallArtifacts               string
	UninstallArtifactsHint           string
	IntegrationsCheck                string
	IntegrationsInstall              string
	IntegrationsPackageHint          string
	IntegrationsVersionHint          string
	IntegrationsRemoveTitle          func(name string) string
	IntegrationsRemoveDetail         string
	// IntegrationsCommands is the per-command switch list's caption, and
	// IntegrationsCommandsHint explains what a switch does.
	IntegrationsCommands string
	// The orphan operations: adopting a package directory the repository does
	// not name, and deleting one.
	IntegrationsAdopt       string
	IntegrationsPermissions string
	// The shipped tools a user can connect with one press.
	IntegrationsRecommended     string
	IntegrationsRecommendedHint string
	IntegrationsConnect         string
	// The stored health from the lifecycle probes.
	IntegrationsHealthy   string
	IntegrationsHealth    string
	IntegrationsDegraded  string
	IntegrationsUnhealthy string
	// The configuration form.
	ConfigOpen     string
	ConfigSave     string
	ConfigSaved    string
	ConfigRequired string
	ConfigNumber   string
	// The export and import of the integration list.
	IntegrationsExport       string
	IntegrationsImport       string
	IntegrationsExported     func(count int, path string) string
	IntegrationsImported     func(succeeded, failed, skipped int) string
	IntegrationsTransferBad  string
	IntegrationsAdoptHint    string
	IntegrationsDeleteOrphan string
	IntegrationsCommandsHint string
	// The command alias editor: the field's label, its placeholder and the
	// note that another command claimed the alias first.
	IntegrationsReprobe                 string
	IntegrationsInstalled               string
	IntegrationsCommandAlias            string
	IntegrationsCommandAliasPlaceholder string
	IntegrationsCommandAliasTaken       string
	IntegrationsUnavailable             string

	// The Shortcuts page.
	ShortcutsToggle   string
	ShortcutsHint     string
	ShortcutRecord    string
	ShortcutRecording string

	// The app's own rebindable keys, by action id.
	ShortcutsApp     string
	ShortcutsAppHint string
	ShortcutNames    map[string]string

	// The user-defined shortcuts.
	ShortcutsCustom      string
	ShortcutsCustomHint  string
	ShortcutsNone        string
	ShortcutsAdd         string
	ShortcutsRemove      string
	ShortcutsAction      string
	ShortcutsCommand     string
	ShortcutsCommandHint string
	ShortcutsRecordKey   string
	ShortcutsReset       string
	ShortcutsRejected    func(key, reason string) string
	ShortcutsActions     []Option

	// The Plugins page: the built-in plugins' own settings.
	BrowserPlugin          string
	BrowserPluginHint      string
	BrowserEnabled         string
	BrowserTarget          string
	BrowserTargetHint      string
	BrowserAuto            string
	BrowserCustomDir       string
	BrowserCustomDirHint   string
	BrowserHistoryDays     string
	BrowserHistoryDaysHint string
	BrowserHistoryAll      string
	BrowserSort            string
	BrowserSortHint        string
	BrowserSearchField     string
	BrowserSearchFieldHint string
	BrowserCDP             string
	BrowserCDPEnabled      string
	BrowserCDPEnabledHint  string
	BrowserCDPPort         string
	BrowserCDPPortHint     string
	BrowserSortOrders      []Option
	BrowserSearchFields    []Option

	ClipboardPlugin       string
	ClipboardPluginHint   string
	ClipboardEnabled      string
	ClipboardMaxItems     string
	ClipboardMaxItemsHint string
	ClipboardClear        string
	ClipboardClearHint    string
	ClipboardClearTitle   string
	ClipboardClearDetail  string
	ClipboardClearButton  string
	ClipboardClearCancel  string

	CalculatorPlugin         string
	CalculatorPluginHint     string
	CalculatorMaxItems       string
	CalculatorMaxItemsHint   string
	CalculatorRetention      string
	CalculatorRetentionHint  string
	CalculatorCopyMode       string
	CalculatorCopyModeHint   string
	CalculatorRetentionNever string
	CalculatorRetentionDays  func(days int) string
	CalculatorCopyModes      []Option

	// The About page's update check.
	UpdateCheck      string
	UpdateChecking   string
	UpdateCurrent    string
	UpdateDisabled   string
	UpdateFailed     string
	UpdateAvailable  func(version string) string
	UpdateInstall    string
	UpdateInstalling func(percent int) string
	UpdateInstalled  string

	// The About page.
	AboutVersion      string
	AboutFramework    string
	AboutScheme       string
	AboutSettingsFile string
	AboutProject      string

	// Percent renders an opacity value ("47%").
	Percent func(n uint8) string
}

// Notifications is the copy of the system notifications a background action
// raises when the panel is hidden. A visible panel shows the result itself, so
// these never duplicate what is on screen.
type Notifications struct {
	// Title is the notification's title: the app's own name.
	Title string
	// The integration tasks, by outcome.
	IntegrationInstalled     func(name string) string
	IntegrationInstallFailed func(name string) string
	IntegrationRemoved       func(name string) string
	IntegrationRemoveFailed  func(name string) string
	IntegrationChecked       func(name string) string
	IntegrationCheckFailed   func(name string) string
	// A command that ran in the background: its name, and the status line the
	// output view shows.
	CommandFinished func(name string) string
	CommandStatus   func(status string) string
}

// Terminal is the terminal surface's copy.
type Terminal struct {
	Title string
	// Pin names the control that copies the session's text into a window
	// of its own.
	Pin string
	// Hint is the empty state shown while no session runs.
	Hint string
	// EmptyTitle, EmptyNew and EmptyBack are the empty state's title, the
	// control that opens a blank session, and the one that returns to the
	// launcher.
	EmptyTitle string
	EmptyNew   string
	EmptyBack  string
	// Close names the close control for assistive technology.
	Close string
	// NewCommand and OpenExternal name the two controls the old terminal
	// header carried: return to the launcher, and hand the session to the
	// system's own terminal.
	NewCommand       string
	NewCommandHint   string
	OpenExternal     string
	OpenExternalHint string
	// ProcessExited and ProcessExitedHint are the resident note: the program
	// ended, the output is held on screen, and closing is the user's own
	// decision.
	ProcessExited     func(code int) string
	ProcessExitedHint string
}

// Copy is every language's string set.
type Copy struct {
	Launcher      Launcher
	Settings      Settings
	Terminal      Terminal
	Notifications Notifications
}

// NotificationsFor returns the system notifications' copy for a language.
func NotificationsFor(language string) Notifications {
	return For(language).Notifications
}

// For returns the copy of a language, normalized like the settings loader.
func For(language string) Copy {
	switch Normalize(language) {
	case ZH:
		return zh
	default:
		return en
	}
}

func percentEN(n uint8) string { return itoa(n) + "%" }
func itoa(n uint8) string {
	if n == 0 {
		return "0"
	}
	var digits [3]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
