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
	CommandSettings      string
	CommandSettingsHint  string
	CommandTerminal      string
	CommandTerminalHint  string
	CommandClipboard     string
	CommandClipboardHint string
	CommandBrowser       string
	CommandBrowserHint   string
	CommandQuit          string
	CommandQuitHint      string
	// BrowserTab labels a live-tab row in the browser mode, and
	// BrowserNoProfile is the mode's empty state when no browser profile was
	// found at all.
	BrowserTab       string
	BrowserNoProfile string
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
	SessionsNone    string
	SessionsRunning string
	SessionsClose   string
	SessionsActive  string

	// The permission dialog the installer shows, and the permission line
	// the integrations list draws.
	PermissionsTitle    func(name string) string
	PermissionsAllow    string
	PermissionsCancel   string
	PermissionsEnforced string
	PermissionsDeclared string

	// The Integrations page.
	IntegrationsEmpty        string
	IntegrationsEnable       string
	IntegrationsRunning      string
	IntegrationsOff          string
	IntegrationsBroken       string
	IntegrationsOrphan       string
	IntegrationsHint         string
	IntegrationsUninstall    string
	IntegrationsCheck        string
	IntegrationsInstall      string
	IntegrationsPackageHint  string
	IntegrationsVersionHint  string
	IntegrationsRemoveTitle  func(name string) string
	IntegrationsRemoveDetail string

	// The Shortcuts page.
	ShortcutsToggle   string
	ShortcutsHint     string
	ShortcutRecord    string
	ShortcutRecording string

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

	// The About page.
	AboutVersion      string
	AboutFramework    string
	AboutScheme       string
	AboutSettingsFile string
	AboutProject      string

	// Percent renders an opacity value ("47%").
	Percent func(n uint8) string
}

// Terminal is the terminal surface's copy.
type Terminal struct {
	Title string
	// Pin names the control that copies the session's text into a window
	// of its own.
	Pin string
	// Hint is the empty state shown while no session runs.
	Hint string
	// Close names the close control for assistive technology.
	Close string
}

// Copy is every language's string set.
type Copy struct {
	Launcher Launcher
	Settings Settings
	Terminal Terminal
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
