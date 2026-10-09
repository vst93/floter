// Package i18n holds floter's user-facing copy for the languages the app
// ships (en, zh).
//
// The shape follows the Rust/TypeScript build's src/i18n.ts: English is the
// source of truth, and a key missing from the other language is a bug rather
// than a blank label. Here that is enforced by the type system — every
// language builds the same Copy struct, so a field cannot be left out; the
// tests pin the strings a screen is allowed to show.
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
	// Placeholder is the search field's placeholder (src/i18n.ts
	// input.placeholder).
	Placeholder string
	// Label names the field for assistive technology.
	Label string
	// Hint is the empty result area's message.
	Hint string
	// NoResults is shown when a query matches nothing.
	NoResults string
	// Clear names the field's clear button for assistive technology.
	Clear string
	// ResultsLabel names the results list for assistive technology.
	ResultsLabel string
	// The built-in commands the P1 catalog offers.
	CommandSettings     string
	CommandSettingsHint string
	CommandTerminal     string
	CommandTerminalHint string
	CommandQuit         string
	CommandQuitHint     string
	// ShortcutSettings and ShortcutTerminal are the accelerator labels the
	// rows show for the two surfaces that have one.
	ShortcutSettings string
}

// Settings is the settings surface's copy.
type Settings struct {
	Title string
	Close string

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

	GroupAppearance string
	GroupWindow     string

	Theme               string
	ThemeAuto           string
	ThemeLight          string
	ThemeDark           string
	Language            string
	LanguageHint        string
	Glass               string
	GlassHint           string
	GlassOff            string
	GlassFrosted        string
	GlassRegular        string
	GlassLiquid         string
	MainOpacity         string
	MainOpacityHint     string
	TerminalOpacity     string
	TerminalOpacityHint string
	Scale               string
	ScaleHint           string
	ScaleTiny           string
	ScaleSmall          string
	ScaleDefault        string
	ScaleLarge          string

	// Percent renders an opacity value ("47%").
	Percent func(n uint8) string
}

// Terminal is the terminal surface's copy.
type Terminal struct {
	Title string
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
