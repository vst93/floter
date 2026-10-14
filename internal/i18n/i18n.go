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

// RelativeUnit is one calendar unit a past time can be named in, as
// `Intl.RelativeTimeFormat` took it: the old build handed the unit to the
// platform's own formatter rather than rolling a plural table, and this is
// the vocabulary the two shipped languages implement.
type RelativeUnit string

// The units a freshness line can name, largest-context first when chosen.
const (
	RelativeMinute RelativeUnit = "minute"
	RelativeHour   RelativeUnit = "hour"
	RelativeDay    RelativeUnit = "day"
	RelativeMonth  RelativeUnit = "month"
	RelativeYear   RelativeUnit = "year"
)

// Launcher is the launcher surface's copy.
type Launcher struct {
	// Placeholder is the search field's placeholder.
	Placeholder string
	// Label names the field for assistive technology.
	Label string
	// TriggerHint nudges that a typed word is a plugin command's trigger and
	// a space enters its mode ("{name} · enter"), TriggerHintMore adding the
	// count of commands it could still become.
	// BareTerminal is the held modifier's own row: a terminal session with no
	// command, and the line under it that says so.
	// The tray menu: show the panel, open the settings, re-scan, quit.
	TrayShow     string
	TraySettings string
	TrayReload   string
	TrayQuit     string
	// The mode's chips row carries a one-line hint at its end, as the old
	// build's `.launcher-filter__hint` did: what Tab does, and the keys the
	// list itself answers.
	BrowserFilterHint    string
	ClipboardFilterHint  func(shortcut string) string
	CalculatorFilterHint func(shortcut string) string
	// The scope marker's name: the mode that owns the field, as the old
	// build's `.collapsed-card__scope-name` printed it.
	ScopeBrowser    string
	ScopeClipboard  string
	ScopeCalculator string
	ScopePlugin     string
	// ConfigurePlugin is the gear's own word in a mode's chips row, and
	// ConfigurePluginClose the same control once its panel is open.
	ConfigurePlugin      string
	ConfigurePluginClose string
	// ConfigurePluginHint and ConfigurePluginCloseHint are the sentences the
	// gear and its ✕ carry: a glyph alone does not say whether it configures the
	// plugin or closes the panel that does.
	ConfigurePluginHint      string
	ConfigurePluginCloseHint string
	// FieldSessions and FieldSettings label the field row's two own buttons
	// (the sessions page and the settings), for assistive technology: the old
	// build kept them beside the field, one click from the search row.
	FieldSessions string
	FieldSettings string
	// FieldSessionsHint and FieldSettingsHint are those two buttons' own
	// lines. The settings one carries the user's current open_settings binding,
	// the way the old build's `settings.openHint` interpolated it.
	FieldSessionsHint string
	FieldSettingsHint func(shortcut string) string
	BareTerminal      string
	BareTerminalHint  string
	TriggerHint       func(name string) string
	TriggerHintMore   func(name string, more int) string
	// AliasMatch names the user's alias when it is what carried a match
	// ("Alias: gfm"), the old build's `launcher.aliasMatch`. It rides the
	// row's subtitle, so a row reached by an alias explains itself instead
	// of reading like a stray hit.
	AliasMatch func(alias string) string
	// FilesSection heads the block of rows a dropped file's actions make,
	// FilesMore is the row that reveals the files past the cap (with how many
	// are behind it), FilesMoreHint its own sentence, and RecentSection the
	// empty page's most-launched applications.
	FilesSection  string
	FilesMore     func(count int) string
	FilesMoreHint string
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
	// DirectOutputHint is the tooltip of a row's space keycap: the command's
	// output is shown in the search box rather than in a terminal, so the row
	// carries a bare `␣` and this sentence behind it.
	DirectOutputHint string
	// CommandConflict is the sentence behind a command row's warning dot: the
	// command word belongs to more than one integration, so the row the
	// launcher will not enter says why (the old build's `extensions.conflict`).
	CommandConflict string
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
	// The output list's pagination footer: a page in flight and the end of a
	// list a plugin has finished paging. A list with more to load is quiet —
	// the scroll trigger is the affordance, so it has no word of its own.
	OutputLoading string
	OutputEnd     string

	// The clipboard mode's own feedback lines: the star and the delete.
	ClipboardFavoriteFailed string
	ClipboardDeleteFailed   string
	// ClipboardCopyFailed is a pinned image the launcher could not read back
	// (its file is gone): the copy failed, and the row says so.
	ClipboardCopyFailed string
	ClipboardDeleted    string
	// HistoryDelete names a history row's delete control, and
	// CalculatorDeleted reports one that went.
	HistoryDelete string
	// HistoryPin shows a selected clipboard entry's whole text in a window of
	// its own, HistoryFavorite/HistoryUnfavorite name a history row's own
	// star for assistive technology, and DeleteConfirm is the armed delete's
	// own line: the row's way of asking a second time before it removes.
	HistoryPin        string
	HistoryFavorite   string
	HistoryUnfavorite string
	DeleteConfirm     string
	// HistoryCleared is what the status row says once the typed-command
	// history has been emptied (the field's own Ctrl+Shift+Backspace).
	HistoryCleared string
	// OutputDetach opens a command's page in a window of its own, and
	// OutputDetachHint is the sentence its icon-only control carries (the old
	// build's `pluginWindow.detachHint`).
	OutputDetach     string
	OutputDetachHint string
	// The window that opens for it: its title when the command has no name,
	// the re-run and close controls, and what it says before a run.
	OutputWindowFallback   string
	OutputWindowRerun      string
	OutputWindowClose      string
	OutputWindowIdle       string
	CalculatorDeleted      string
	CalculatorDeleteFailed string
	// CalculatorEmpty is the calculator mode's empty state, and the two
	// failure lines report a history write that did not land.
	CalculatorEmpty          string
	CalculatorRecordFailed   string
	CalculatorFavoriteFailed string
	// CalculatorYesterday is the history row's time word for an entry made the
	// day before: the old build's `calculator.yesterday`, printed where a
	// today entry prints its clock time.
	CalculatorYesterday string
	// The power rows: restart and shut down, and the confirmation the shell
	// shows before either.
	PowerRestart      string
	PowerRestartHint  string
	PowerShutdown     string
	PowerShutdownHint string
	PowerCancel       string
	PowerFailed       string
	// PowerRestartAction and PowerShutdownAction are the actions' own words,
	// which the confirmation's button carries: the old build's confirm row
	// said "Restart", not "Continue".
	PowerRestartAction  string
	PowerShutdownAction string
	// PowerConfirmMessage is the confirmation's one sentence, per action.
	PowerConfirmMessage func(action string) string
	// InstallTool names the row that offers a tool's install command, and
	// InvokeFailed reports a tool that could not be started. The rest are the
	// old build's per-operation errors: what failed, not just that something
	// did.
	InstallTool    func(name string) string
	InvokeFailed   string
	OpenAppFailed  string
	OpenPathFailed string
	OpenURLFailed  string
	OpenTermFailed string
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
	// BrowserNoProfile is the browser mode's empty state when no browser
	// profile was found at all.
	BrowserNoProfile string
	// BrowserDisabled is the browser mode's own answer when the plugin is
	// switched off: what happened, and where to turn it back on.
	BrowserDisabled string
	// The plugin modes' own empty states: a fact about the mode, not a search
	// outcome.
	ClipboardEmpty          string
	ClipboardEmptyFilter    string
	ClipboardEmptyFavorites string
	// ClipboardDisabled is the clipboard mode's own answer when the plugin is
	// switched off: what happened, and where to turn it back on -- the twin of
	// BrowserDisabled (the reference's `clipboard.pageUnavailable`,
	// `i18n.ts:291`).
	ClipboardDisabled        string
	CalculatorEmptyFavorites string
	BrowserEmpty             string
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
	// Close names the close control for assistive technology, CloseHint the
	// same control's own line, and Quit/QuitHint the header's way out of the
	// application.
	Close     string
	CloseHint string
	Quit      string
	QuitHint  string

	PageGeneral      string
	PageSessions     string
	PageShortcuts    string
	PageIntegrations string
	PageAbout        string

	PageGeneralHint      string
	PageSessionsHint     string
	PageShortcutsHint    string
	PageIntegrationsHint string
	PageAboutHint        string

	// PagePlaceholder is the body of the pages P1 routes but does not fill
	// yet: the sidebar route exists, the page says so plainly.
	PagePlaceholder string
	// SaveFailed is a write the plugin refused: the sheet paints the value
	// either way and says so here.
	SaveFailed string
	// SaveFailedAlert is the settings page's own write failing: the change is
	// shown in memory either way, and the banner over the page says the file
	// could not be written. The old build cleared it after five seconds
	// (`settings.saveFailed` + `useTimedReset`).
	SaveFailedAlert string

	GroupAppearance string
	GroupWindow     string
	GroupTerminal   string
	// GroupMenuBar titles the section that holds the two controls describing
	// the one icon the user sees in the menu bar / tray: the visibility switch
	// (R7-10c) and the light/dark app-icon choice (R150, the old
	// `settings.group.menuBar`, "Menu bar").
	GroupMenuBar string
	// GroupTransparency names the transparency card inside the material
	// section (the old `settings.group.transparency`). The card draws no
	// visible title, so this is the group's accessible name.
	GroupTransparency string
	// GroupUpdate titles the About page's updates section (the old
	// `settings.group.update`, "Updates"). The reference drew the update row
	// inside its own section heading and card (`AboutPage.tsx:62-70`) rather
	// than loose on the page, so the one control the user presses has a group
	// to belong to.
	GroupUpdate string

	Theme        string
	ThemeHint    string
	AppIcon      string
	AppIconHint  string
	Language     string
	LanguageHint string
	// Material titles the section that holds the glass effect and the two
	// transparency sliders (the old `settings.material`, "Glass"). The
	// reference drew that group's two explanations as section footnotes
	// (`GlassHint` and `TransparencyHint`) rather than repeating a sentence on
	// each row, so the two sliders carry no inline hint.
	Material         string
	TransparencyHint string
	Glass            string
	GlassHint        string
	MainOpacity      string
	TerminalOpacity  string
	// TransparencyScaleLow and TransparencyScaleHigh name the two ends of the
	// two transparency sliders' axis, the old `settings.transparency.scaleLow`
	// (".scaleHigh") words: "Transparent" at 10% and "Solid" at 100%.
	TransparencyScaleLow  string
	TransparencyScaleHigh string
	Scale                 string
	ScaleHint             string
	// ScaleSmall and ScaleLarge name the ends of the terminal appearance
	// sliders (font size, line height) — the old `settings.scale.small`
	// (".large") words, which the reference offers on their own so a range
	// can label its axis without borrowing the UI-scale control's labels.
	ScaleSmall string
	ScaleLarge string

	// The terminal's appearance, from the old GeneralPage's group.
	TerminalHint string
	FontSize     string
	FontFamily   string
	CursorShape  string
	CursorBlink  string
	LineHeight   string
	Padding      string
	Palette      string
	// PasteSafe names the safe-paste switch: a pasted command loses the one
	// trailing line break that would run it before it was read. It is the
	// terminal drawer's own row (internal/control, AxisPasteSafe).
	PasteSafe string
	// PalettePreview names the miniature terminal under the palette picker:
	// the block that shows the chosen palette's own colors (R43).
	PalettePreview string

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
	// The custom duration row: its label, the unit beside the field, and the
	// button that applies it. SurfaceResidencyCustom is the select's own
	// "custom" entry, and SurfaceResidencyCustomValue is that entry's label
	// while a custom duration is stored, so the select echoes the number the
	// inline field holds.
	SurfaceResidencyCustomLabel string
	SurfaceResidencyCustomUnit  string
	SurfaceResidencyCustomApply string
	SurfaceResidencyCustom      string
	SurfaceResidencyValue       func(seconds uint32) string
	SurfaceResidencyCustomValue func(seconds uint32) string

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
	SessionsClose            string
	SessionsActive           string
	// SessionsSortNewest and SessionsSortOldest name the sessions list's own
	// order control: the word it shows is the order in force, and pressing it
	// swaps to the other (the old page's `terminal.sortNewest` /
	// `terminal.sortOldest`).
	SessionsSortNewest string
	SessionsSortOldest string
	SessionsShow       string
	SessionsNew        string
	// Closing a session kills its program, so it arms first: the question the
	// armed control asks and the way back from it.
	SessionsCloseConfirm string
	SessionsCloseCancel  string

	// The permission dialog the installer shows, and the permission line
	// the integrations list draws.
	PermissionsTitle    func(name string) string
	PermissionsAllow    string
	PermissionsCancel   string
	PermissionsEnforced string
	PermissionsDeclared string
	// The permission review the connect flow shows before a recommended tool
	// is connected: the reference's `permissionsRequired` label,
	// `confirmConnectRecommended` title and `connect` button
	// (i18n.ts:465-469). The boundary sentence is InstallReviewBoundary — the
	// reference uses the one `permissionReviewBoundary` key in both dialogs.
	PermissionsRequired     string
	ConnectRecommendedTitle func(name string) string
	PermissionsConnect      string
	// ChooseSystemToolTitle/-Hint are the tool-selection dialog the connect and
	// reconnect flows show when a manifest's declared executable names resolve
	// to more than one program (`settings.extensions.chooseSystemTool`, a name
	// and a hint, i18n.ts:472-473).
	ChooseSystemToolTitle func(name string) string
	ChooseSystemToolHint  string

	// The Integrations page.
	IntegrationsEmpty     string
	IntegrationsEnable    string
	IntegrationsRunning   string
	IntegrationsOff       string
	IntegrationsBroken    string
	IntegrationsOrphan    string
	IntegrationsHint      string
	IntegrationsUninstall string
	// IntegrationsDisconnect and IntegrationsRemovePackage are the removal
	// verbs the other two integration kinds use: the reference's
	// `settings.extensions.disconnect` (a built-in whose runtime the system
	// owns — the package is not floter's to delete) and
	// `settings.extensions.removePackage` (a linked local package — the files
	// outside floter stay). The custom kind reuses `RemoveDelete`
	// (`settings.extensions.deleteCustom`) and an npm package reuses
	// `IntegrationsUninstall`, exactly as `removalKind` in
	// `src/extensions/ExtensionRow.tsx:109-114,409-418` picked them.
	IntegrationsDisconnect    string
	IntegrationsRemovePackage string
	// IntegrationsDisconnectTitle/-Detail and IntegrationsRemovePackageTitle/
	// -Detail are those two kinds' confirmation-bar words. The reference asked
	// the bar's question with the *same* per-kind key that named the menu item
	// (`src/ExtensionsPanel.tsx:2140-2151` builds one `removalTextKey` per
	// kind; `src/extensions/RemovalConfirmation.tsx:41` reads its Title and
	// body through it), so a card whose button reads "Disconnect" cannot then
	// ask "Uninstall?". The npm kind reuses `RemoveTitle`/`RemoveDetail` and
	// the custom kind its own `RemoveCustom*`, exactly as that switch did.
	IntegrationsDisconnectTitle     func(name string) string
	IntegrationsDisconnectDetail    string
	IntegrationsRemovePackageTitle  func(name string) string
	IntegrationsRemovePackageDetail string
	// IntegrationsKind* is the integration's *type*, the slot the reference
	// spent on the meta row beside the package identity
	// (`src/extensions/ExtensionRow.tsx:255` draws
	// `t(integrationKindKey(extension))`; the words are `i18n.ts:446-453`'s
	// `settings.extensions.integrationKind.*` family). It is a type, not the
	// removal verb: the same card shows "System tool" and offers "Disconnect".
	//
	// Only the four kinds a *connected* card can be are carried: the reference's
	// `integrationKindKey` distinguishes seven, but its `recommended`,
	// `manifest` and `tool` branches require `!connected`, and this build lists
	// those rows in the Detected and Recommended sections with their own
	// renderers rather than as `Integration` cards (`settingsui.go`), so the
	// switch reduces to the four here. Adding the other three words would leave
	// i18n keys nothing reads (`TestNoDeadCopy`).
	IntegrationsKindCustom  string
	IntegrationsKindNPM     string
	IntegrationsKindSystem  string
	IntegrationsKindPackage string
	// DismissNotice labels the toast's close control: the old build's
	// `settings.extensions.dismissNotice`.
	DismissNotice string
	// The base plugins: the built-in functionality the app ships, listed on
	// the Integrations page with the one door to each one's configuration.
	// The names are the old build's own (`settings.clipboardHistory`,
	// `settings.browser`, `settings.calculator`) — a browser is named for
	// what it holds, not for the mode that searches it.
	BasePlugins          string
	BasePluginsHint      string
	BaseClipboard        string
	BaseClipboardHint    string
	BaseClipboardPrivacy string
	BaseBrowser          string
	BaseBrowserHint      string
	BaseCalculator       string
	BaseCalculatorHint   string
	// PluginConfigure opens one built-in plugin's configuration sheet in the
	// launcher, where the mode's own gear opens it.
	PluginConfigure string
	// The componentized uninstall dialog. Its title names the integration
	// being removed (`uninstallComponentsTitle`, "Uninstall {name}"), the way
	// the reference's own dialog did
	// (`src/extensions/ComponentizedUninstallDialog.tsx:70-72`).
	IntegrationsUninstallTitle       func(name string) string
	IntegrationsUninstallDescription string
	// UninstallProgram is the dialog's first row: program files are always
	// removed, so the row is drawn checked and disabled to disclose that
	// (`ComponentizedUninstallDialog.tsx:84-98`, `i18n.ts:532-533`).
	UninstallProgram        string
	UninstallProgramHint    string
	UninstallHostConfig     string
	UninstallHostConfigHint string
	UninstallToolData       string
	UninstallToolDataHint   string
	UninstallArtifacts      string
	UninstallArtifactsHint  string
	IntegrationsCheck       string
	// IntegrationsRecheck is the entry a *broken* connected row offers: it
	// verifies the installed package and repairs it when the record drifted
	// from what is on disk (the reference's `settings.extensions.recheck`, the
	// Wrench on `src/extensions/ExtensionRow.tsx:313-330`).
	IntegrationsRecheck     string
	IntegrationsInstall     string
	IntegrationsPackageHint string
	IntegrationsVersionHint string
	// IntegrationsCommands is the per-command switch list's caption, and
	// IntegrationsCommandsHint explains what a switch does.
	IntegrationsCommands string
	// The orphan operations: adopting a package directory the repository does
	// not name, and deleting one.
	IntegrationsAdopt       string
	IntegrationsPermissions string
	// The permission tiers: what Floter itself refuses, and what it only
	// discloses.
	PermissionsEnforcedLabel   string
	PermissionsEnforcedHint    string
	PermissionsDisclosureLabel string
	PermissionsDisclosureHint  string
	// PermissionBoundary is the note that says the declaration switches isolate
	// nothing: local code runs as the user either way.
	PermissionBoundary string
	// The shipped tools a user can connect with one press.
	IntegrationsRecommended     string
	IntegrationsRecommendedHint string
	IntegrationsConnect         string
	// The detected row's two non-connect actions: the tool has a recipe this
	// machine can install with (opening a terminal on that command), or floter
	// has nothing to offer and the button says so disabled. They are the
	// reference's other two arms of the one detected-row button
	// (`src/extensions/ExtensionRow.tsx:296-302`).
	IntegrationsInstallInTerminal string
	IntegrationsInstallTool       string
	// IntegrationsReconnect is the repair a connected row offers when the
	// program it was bound to no longer resolves: the search path is asked
	// again. IntegrationsReconnectUnboundHint is what that button says when
	// the manifest names no project page either — the search path is the only
	// place left to look (the reference's `reconnectUnboundHint`,
	// `src/extensions/ExtensionRow.tsx:345-370`).
	IntegrationsReconnect            string
	IntegrationsReconnectUnboundHint string
	// The stored health from the lifecycle probes, and the live diagnostics a
	// check produces. They are two rows: the health is what the last probe run
	// recorded (and it survives across runs), the diagnostics are what this
	// session's check just said.
	IntegrationsHealthy     string
	IntegrationsHealth      string
	IntegrationsDegraded    string
	IntegrationsUnhealthy   string
	IntegrationsDiagnostics string
	// The stored report's detail: every probe the last run recorded (its id and
	// how long it took), the failed ones with their exit code, stderr and
	// whether a retry is worth it, and when the run happened.
	IntegrationsHealthChecks    string
	IntegrationsHealthFailures  string
	IntegrationsHealthRetryable string
	IntegrationsHealthExitCode  func(code string) string
	IntegrationsHealthCheckedAt func(when string) string
	// The approval record block (the old host's R7-8): what the recorded
	// approval covered, when it happened, and against which manifest — the
	// on-disk digest beside the recorded one, so "what I approved" and "what
	// is here now" are two readable rows. An unreadable/absent digest is
	// "Not recorded", never a false match.
	ApprovalRecord                string
	ApprovalRecordNone            string
	ApprovalRecordApprovedAt      string
	ApprovalRecordPermissions     string
	ApprovalRecordPermissionsNone string
	ApprovalRecordDigest          string
	ApprovalRecordDigestCurrent   string
	ApprovalRecordDigestUnknown   string
	ApprovalRecordChanged         string
	// IntegrationsFailureDetail is the label of a broken entry's one-line
	// reason (the old host's `settings.extensions.brokenDetail`).
	IntegrationsFailureDetail string
	// The configuration form.
	ConfigOpen     string
	ConfigSave     string
	ConfigSaved    string
	ConfigRequired string
	ConfigNumber   string
	// The export and import of the integration list. Exporting/Importing are the
	// busy lines shown while a transfer is in flight (the reference's
	// `settings.extensions.exporting` / `.importing`).
	IntegrationsExport      string
	IntegrationsImport      string
	IntegrationsExporting   string
	IntegrationsImporting   string
	IntegrationsExported    func(count int) string
	IntegrationsTransferBad string
	// The import report's lines, from the reference's
	// `settings.extensions.import*` keys: the summary of a clean run, the
	// warning that some fields were left out of the export, and the two pieces
	// of a failure notice (the detail and its "and N more" tail).
	IntegrationsImportSummary        func(succeeded, skipped int) string
	IntegrationsImportExcludedFields func(extensions string) string
	IntegrationsImportFailedDetail   func(failures string) string
	IntegrationsImportFailedMore     func(count int) string
	IntegrationsAdoptHint            string
	IntegrationsDeleteOrphan         string
	IntegrationsCommandsHint         string
	// The command alias editor: the field's label, its placeholder and the
	// note that another command claimed the alias first.
	IntegrationsReprobe   string
	IntegrationsInstalled string
	// EditLabel opens the form again on an integration the host generated.
	EditLabel string
	// LocalToolSave is the form's button while it is rewriting rather than
	// connecting.
	LocalToolSave string
	// The run-time parameter form: the button, the "not yet complete" refusal,
	// and the line that says where the output will land.
	// The declared-inputs editor.
	ParamsTitle       string
	ParamsHint        string
	ParamsEmpty       string
	ParamsAdd         string
	ParamsRemove      string
	ParamsID          string
	ParamsLabel       string
	ParamsKind        string
	ParamsPlaceholder string
	ParamsDefault     string
	ParamsFlag        string
	ParamsRequired    string
	ParamsOptions     string
	ParamsOptionsHint string
	ParamsRow         func(number int) string
	ParamsKindText    string
	ParamsKindNumber  string
	ParamsKindBoolean string
	ParamsKindSelect  string
	ParamsKindPath    string
	RunLabel          string
	// The row's Run button (the reference's `customRun` / `customRunUnavailable`)
	// and the form's Cancel (`customRunCancel`).
	RunAction            string
	RunActionUnavailable string
	RunCancel            string
	// The run-time parameter form's own frame: its heading and hint, the
	// required marker beside a row's kind label, the injection note, and the
	// route the manifest declares (one of the two output hints).
	RunValues               string
	RunValuesHint           string
	RunRequired             string
	RunInjectionNote        string
	RunOutputHintTerminal   string
	RunOutputHintBackground string
	// The last background run's output block in the integration's row: its
	// heading, the two ways to fold it, and its empty and truncation lines.
	RunLastOutput          string
	RunViewOutput          string
	RunHideOutput          string
	RunNoOutput            string
	RunOutputTruncated     string
	RunOutputTruncatedTail string
	// A refused run's parameter reason: the four `run_param_*` keys, each
	// interpolating the parameter's label (or its id).
	RunParamRequired      func(label string) string
	RunParamInvalid       func(label string) string
	RunParamUnknown       func(label string) string
	RunParamWindowsUnsafe func(label string) string
	// A failed run is a stable key plus its facts; these are the words. Each
	// takes the payload fields its own key carries.
	RunErrorScriptMissing       func(path string) string
	RunErrorProgramMissing      func(path string) string
	RunErrorNotExecutable       func(path string) string
	RunErrorInterpreterMissing  func(language, names, searched string) string
	RunErrorSpawnFailed         func(path, detail string) string
	RunErrorTimeout             func(seconds string) string
	RunErrorKilled              func(id string) string
	RunErrorIntegrationDisabled func(id string) string
	RunErrorIntegrationBroken   func(id, reason string) string
	RunErrorTaskFailed          func(message string) string
	// R11 · an unavailable runtime binding, the readable half. ErrorCodeLabel
	// words a code the repository recorded (the old host's
	// `settings.extensions.errorCode.*`, this build's `probe_failed` included);
	// the three detail sentences word a keyed binding detail. A code or a
	// detail the dictionary does not know is left as it is.
	ErrorCodeLabel           func(code string) string
	BindingChangedDetail     func(path string) string
	BindingMissingDetail     func(path string) string
	RuntimeUnavailableDetail func(names string) string
	// A completed run's summary: the success sentence names the duration word
	// form, the failure sentence the exit code, and the two output sentences
	// the line count (the truncated one when a stream hit the cap).
	RunSucceeded            func(name, duration string) string
	RunFailed               func(name string, code int) string
	RunOutputLines          func(lines int) string
	RunOutputLinesTruncated func(lines int) string
	// The disable confirmation: stopping an integration is a mark, not a
	// delete, and the notice says so. Its Cancel is the shared
	// `IntegrationsCancel` below — the reference read every inline question's
	// Cancel from one key (`settings.extensions.cancel`, `i18n.ts:554`).
	DisableTitle       func(name string) string
	DisableDescription string
	DisableConfirm     string
	// The removal bar: deleting an integration is destructive, so it is asked
	// inline (the reference's `.extension-discard-bar`). A generated custom
	// integration's own words and the warning only it carries. The bar's
	// confirm button is not here: it is the kind's own removal verb
	// (`removalLabel`), because the reference read the button's label from the
	// same `removalTextKey` that named the menu item. `IntegrationsCancel` is
	// the one Cancel both inline bars draw.
	RemoveTitle         func(name string) string
	RemoveDetail        string
	RemoveCustomTitle   func(name string) string
	RemoveCustomDetail  string
	RemoveCustomWarning string
	RemoveDelete        string
	IntegrationsCancel  string
	// The detected-programs section: what this machine already has and
	// nothing has connected.
	DetectedTitle string
	// The `floter://register` deep link's outcome: the mark the named row
	// wears while the link's moment lasts, and the two sentences a link that
	// names no reachable row gets — one for a program this device does not
	// have, one for a program that is already connected (the reference's
	// `settings.extensions.register*`, `i18n.ts:437-440`).
	RegisterHighlight        string
	RegisterNotFound         func(name string) string
	RegisterAlreadyConnected func(name string) string
	// The local-package install row: a package folder on this machine.
	InstallFolderLabel  string
	InstallFolderHint   string
	InstallFolderChoose string
	DetectedHint        string
	// The local-package review dialog: what the package is and what it asks
	// for, shown before anything is grafted into place (the reference's
	// `settings.extensions.localConfirm*` / `localSource` / `localRuntime` /
	// `customPlatforms` / `permissionReviewBoundary`, i18n.ts:640-645,694,553).
	InstallReviewTitle          func(name string) string
	InstallReviewHint           string
	InstallReviewSource         string
	InstallReviewLocalDirectory string
	InstallReviewRuntime        string
	InstallReviewPlatforms      string
	InstallReviewUnavailable    string
	InstallReviewBoundary       string
	InstallReviewConfirm        string
	// The freshness display: when the command list was last derived, how the
	// derivation went, and how the list moved. "Unknown" is its own word
	// everywhere — a fabricated zero or "no change" is a lie.
	Freshness                string
	FreshnessNever           string
	FreshnessJustNow         string
	FreshnessRelative        func(value int, unit RelativeUnit) string
	FreshnessProbeSource     string
	FreshnessHealthSource    string
	FreshnessResultRunning   string
	FreshnessResultSuccess   string
	FreshnessResultDegraded  string
	FreshnessResultFailed    string
	FreshnessResultUnknown   string
	FreshnessCommands        string
	FreshnessCommandsUnknown string
	FreshnessDeltaIncrease   func(count int) string
	FreshnessDeltaDecrease   func(count int) string
	FreshnessDeltaUnchanged  string
	FreshnessDeltaUnknown    string
	// FreshnessDot is the row's one-word freshness mark: the shared sentence
	// around the state word, and the three words themselves.
	FreshnessDot        func(state string) string
	FreshnessDotSynced  string
	FreshnessDotChanged string
	FreshnessDotUnknown string
	// ReprobeNotice, ReprobeDeltaIncrease and ReprobeDeltaDecrease are the
	// drift re-probe's one toast: the reference's
	// `settings.extensions.reprobeNotice` sentence and its two signed delta
	// words. ViewDetails is that toast's action label, the same sentence the
	// reference's `settings.extensions.viewDetails` carried.
	ReprobeNotice        func(name, delta string) string
	ReprobeDeltaIncrease func(count int) string
	ReprobeDeltaDecrease func(count int) string
	ViewDetails          string
	// The About page's deep-link card: the two forms worth copying, and the
	// sentence a refused link gets (the old build's `settings.deepLinkRejected`).
	DeepLinkTitle    string
	DeepLinkHint     string
	DeepLinkCopy     string
	DeepLinkCopied   string
	DeepLinkRejected string
	// OpenNonWebURL is the generic opener's refusal when a URL is outside
	// http/https/ftp: the old host's `settings.extensions.openNonWebUrl`
	// (`src-tauri/src/commands/actions.rs:44-68`, `i18n.ts:851`).
	OpenNonWebURL func(url string) string
	// The local-tool connection form.
	LocalToolTitle string
	LocalToolHint  string
	LocalToolAdd   string
	LocalToolName  string
	// LocalToolId labels the read-only id shown when an existing generated
	// integration is edited (the reference's `customId`, `i18n.ts:683`). The
	// id is minted by the backend and never editable, so the row names the
	// thing rather than offering a field.
	LocalToolId      string
	LocalToolProgram string
	LocalToolChoose  string
	// The executable box's own words (R—/`CustomIntegrationDrawer.tsx:145`):
	// the placeholder invites a path *or* a search over installed tools, and
	// the hint under the row says a changed program re-runs verification on
	// save. Both are taken verbatim from `i18n.ts:665,687`.
	LocalToolProgramPlaceholder string
	LocalToolProgramHint        string
	LocalToolCommand            string
	LocalToolCommandHint        string
	LocalToolArgs               string
	LocalToolArgsHint           string
	LocalToolVersionArgs        string
	LocalToolVersionArgsHint    string
	LocalToolVersion            string
	// The version box stays editable, so the hint says what it is: a fallback
	// a real probe overwrites on connect (R153).
	LocalToolVersionHint string
	LocalToolDescription string
	// The description is executable-only and honest-empty (R159), so its
	// placeholder says a blank field hides the description.
	LocalToolDescriptionPlaceholder string
	// The declaration/audit switches live behind a default-closed Advanced
	// disclosure, so a brand-new integration does not open on a ceremonial form.
	LocalToolAdvanced            string
	LocalToolAdvancedPermissions string
	// The supported-platforms row limits a connection to some operating
	// systems; at least one has to stay ticked, so the row owns its refusal.
	LocalToolPlatforms        string
	LocalToolPlatformRequired string
	// The connection rules the backend applies before it writes a package,
	// each printed under the row that broke it (CustomIssues).
	LocalToolNameLength            string
	LocalToolCommandShape          string
	LocalToolVersionShape          string
	LocalToolModeInvalid           string
	LocalToolScriptEmpty           string
	LocalToolScriptLanguageInvalid string
	LocalToolOutput                string
	LocalToolTerminal              string
	LocalToolBackground            string
	LocalToolConnect               string
	LocalToolFailed                string
	// The form's content actions, the reference's `extension-custom-form__
	// actions` (`CustomIntegrationDrawer.tsx:145-146`): in script mode the
	// "Copy script" button, in program mode "Copy command" and "Copy execution
	// plan". Each press confirms on its own button for 1.6s
	// (`copyCustomContent` and `copyExecutionPlan`), because a clipboard write
	// is otherwise silent.
	LocalToolCopyScript    string
	LocalToolScriptCopied  string
	LocalToolCopyCommand   string
	LocalToolCommandCopied string
	LocalToolCopyPlan      string
	LocalToolPlanCopied    string
	// The script export: the button beside "Copy script"
	// (`CustomIntegrationDrawer.tsx:145`), the success toast the file dialog
	// answers with (`customScriptExported`, `i18n.ts:651`), and the sentence a
	// write that failed raises instead.
	LocalToolExportScript               string
	LocalToolScriptExported             string
	LocalToolExportFailed               string
	IntegrationsCommandAlias            string
	IntegrationsCommandAliasPlaceholder string
	IntegrationsCommandAliasTaken       string
	IntegrationsUnavailable             string
	// RuntimeUnavailable is the badge the reference's ExtensionRow drew in the
	// meta line when `!extension.runtimeAvailable`
	// (`src/extensions/ExtensionRow.tsx:258-272`): "Tool unavailable" /
	// "工具不可用". It is a state mark, not an action — the row's repair
	// button (drawn separately) is the action.
	RuntimeUnavailable string

	// The Shortcuts page.
	ShortcutsToggle   string
	ShortcutsHint     string
	ShortcutRecord    string
	ShortcutRecording string
	// ShortcutRejected is the reference's `settings.shortcut.rejected` — the
	// one sentence drawn at the top of the page and inside the row whose
	// recording the page refused (another action already answers for the
	// chord).
	ShortcutRejected string

	// The app's own rebindable keys, by action id.
	ShortcutsApp     string
	ShortcutsAppHint string
	ShortcutNames    map[string]string

	// The user-defined shortcuts.
	ShortcutsCustom      string
	ShortcutsCustomHint  string
	ShortcutsNone        string
	ShortcutsEmptyHint   string
	ShortcutsAddFirst    string
	ShortcutsAdd         string
	ShortcutsRemove      string
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
	BrowserSort            string
	BrowserSortHint        string
	BrowserSearchField     string
	BrowserSearchFieldHint string
	BrowserCDPEnabled      string
	BrowserCDPEnabledHint  string
	BrowserCDPPort         string
	BrowserCDPPortHint     string
	BrowserSortOrders      []Option
	BrowserSearchFields    []Option
	// The browser overlay's section headings, as the old schema grouped it:
	// the switch, the data source, what the search matches, open tabs.
	BrowserSectionData   string
	BrowserSectionSearch string
	BrowserSectionTabs   string

	// SectionGeneral is the heading of a configuration's first group.
	SectionGeneral string
	// UnitItems and UnitDays trail a bounded value ("300 items", "30 days").
	UnitItems string
	UnitDays  string

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
	ClipboardClearFailed  string

	CalculatorPlugin         string
	CalculatorMaxItems       string
	CalculatorMaxItemsHint   string
	CalculatorRetention      string
	CalculatorRetentionHint  string
	CalculatorCopyMode       string
	CalculatorCopyModeHint   string
	CalculatorRetentionNever string
	CalculatorRetentionDays  func(days int) string
	CalculatorCopyModes      []Option
	CalculatorClearFailed    string

	// The declared-input editor's own validation lines: what a row still
	// needs, printed under the row that needs it.
	ParamInvalidID    string
	ParamDuplicateID  string
	ParamNeedsOptions string
	ParamNeedsFlag    string
	ParamInvalidFlag  string
	// The local-tool form's own words: the kind of tool (a program on this
	// machine, or a script the host writes), the script's language and text,
	// and the line that says whether the language's toolchain is here.
	LocalToolMode            string
	LocalToolModeProgram     string
	LocalToolModeScript      string
	LocalToolLanguage        string
	LocalToolLanguageHint    string
	LocalToolScript          string
	LocalToolScriptHint      string
	LocalToolRuntimeReady    func(name, version string) string
	LocalToolRuntimeMissing  func(names string) string
	LocalToolRuntimeCompiled string
	// LocalToolRuntimeChecking is what the muted line says while the probe is
	// still running (or has not run yet). The old panel's `check === null`
	// rendered exactly this, and it is deliberately distinct from a verdict:
	// "we have not looked" must never be painted as "missing".
	LocalToolRuntimeChecking string
	// LocalToolRuntimeSaveWarning is what a script's save says when this
	// machine has no toolchain for it: the save goes through, so the sentence
	// is advice rather than a refusal.
	LocalToolRuntimeSaveWarning func(language string) string

	// The About page's update row: the current version line, the state that
	// follows it, and the one control the state offers.
	UpdateCurrentVersion string
	UpdateLatestVersion  string
	UpdateUpToDate       string
	UpdateDisabled       string
	UpdateDownload       string
	UpdateInstalling     string
	UpdateRetry          string
	UpdateFailedRetry    string

	// The About page.
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
	// The four removal outcomes the reference told apart by kind
	// (`src/ExtensionsPanel.tsx:1286-1340`): a generated custom integration is
	// deleted, an npm package is uninstalled, a system-owned built-in is
	// disconnected, and a linked package is removed from the list. One generic
	// "was removed" made the notification say less than the confirmation bar
	// had just promised.
	IntegrationCustomDeleted  func(name string) string
	IntegrationUninstalled    func(name string) string
	IntegrationDisconnected   func(name string) string
	IntegrationPackageRemoved func(name string) string
	IntegrationRemoveFailed   func(name string) string
	IntegrationChecked        func(name string) string
	IntegrationCheckFailed    func(name string) string
	// IntegrationReconnected is the one repair that changed the record: the
	// program moved and the row is bound to where it is now.
	IntegrationReconnected func(name string) string
	// IntegrationRepaired and IntegrationRechecked are the re-check's two
	// answers, the reference's `repairedNotice` / `recheckedNotice`
	// (`src/ExtensionsPanel.tsx:1255-1267`): the record had to be rewritten,
	// or the package verified as it stood.
	IntegrationRepaired  func(name string) string
	IntegrationRechecked func(name string) string
	// A command that ran in the background: its name, and the status line the
	// output view shows.
	CommandFinished func(name string) string
	CommandStatus   func(status string) string
	// The clipboard history's one long action — clearing every entry that is
	// not a favourite — reported under the plugin page's own name, the way
	// the old build's `clipboard_clear_history` did.
	ClipboardCleared     func(name string) string
	ClipboardClearFailed func(name string) string
}

// Terminal is the terminal surface's copy.
type Terminal struct {
	Title string
	// Pin names the control that copies the session's text into a window
	// of its own.
	Pin string
	// SettingsOpen, SettingsOpenHint and SettingsClose name the bar's gear
	// in each of its two states: the control opens the terminal's own
	// appearance drawer *in place*, so the session is never left to change
	// how it looks. SettingsOpenHint is the sentence its tooltip carries.
	SettingsOpen     string
	SettingsOpenHint string
	SettingsClose    string
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
	// Input is the terminal view's own accessible name, so assistive
	// technology announces what the focusable region is (the old build's
	// `aria-label={t("terminal.input")}` on its text input).
	Input string
	// NewCommand and OpenExternal name the two controls the old terminal
	// header carried: return to the launcher, and hand the session to the
	// system's own terminal. Their hints are the sentences their tooltips
	// carry, and each one names the chord that does the same thing — the old
	// header's `{shortcut}` interpolation of the `new_command` /
	// `open_external_terminal` bindings.
	NewCommand       string
	NewCommandHint   func(shortcut string) string
	OpenExternal     string
	OpenExternalHint func(shortcut string) string
	// ProcessExited and ProcessExitedHint are the resident note: the program
	// ended, the output is held on screen, and closing is the user's own
	// decision.
	// SessionAttached and SessionDetached are a running session's own state
	// line: on screen, or running behind the one that is. SessionExited is the
	// same line once the program has ended; the sessions page appends the
	// code only when it is non-zero (see `terminalui.NoteOf`).
	SessionAttached string
	SessionDetached string
	SessionExited   string
	// HeaderExited is the identity line's sentence once the program has
	// ended: the session's title, a middle dot and the code the program
	// returned. It is the old header's `terminal.headerExited`, a different
	// word from ProcessExited, which the floating resident note carries.
	HeaderExited      func(code int) string
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
