// Package settings reads floter's settings file.
//
// The disk format is the one the Tauri/Rust build shipped — see
// src-tauri/src/commands/config.rs in this repository. The file is JSON at
//
//	<config dir>/floter/settings.json
//
// where <config dir> is what Rust's `dirs::config_dir()` resolves to:
// %APPDATA% on Windows, ~/Library/Application Support on macOS, and
// $XDG_CONFIG_HOME (falling back to ~/.config) on Linux. Go's
// os.UserConfigDir returns exactly the same directories, so an existing
// user's file is found without any migration.
//
// P0 reads the six fields the launcher shell needs (theme, glass_step,
// language, main_opacity, terminal_opacity, ui_scale) and normalizes them
// with the same whitelists the Rust side used. Every other key in the file
// is kept verbatim so a later write-back cannot drop a setting this round
// does not know about.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	// dirName and fileName match config.rs (`dirs::config_dir().join("floter")`
	// and SETTINGS_FILE_NAME).
	dirName  = "floter"
	fileName = "settings.json"
)

// The shipped defaults, mirrored from config.rs. A file that predates a key
// lands on these, exactly as the Rust `#[serde(default)]` fields did.
const (
	DefaultTheme           = "auto"
	DefaultGlassStep       = "regular"
	DefaultLanguage        = "en"
	DefaultMainOpacity     = 47
	DefaultTerminalOpacity = 46
	DefaultUIScale         = "small"

	// The window-transparency band, from config.rs's MIN/MAX_WINDOW_OPACITY.
	MinWindowOpacity = 10
	MaxWindowOpacity = 100
)

// The terminal's appearance, mirrored from config.rs (and, through it,
// src/terminal/terminal-appearance.ts): the font, the cursor, the line
// height, the inset step, the palette rack and the interaction axes.
const (
	MinFontSize     = 8
	MaxFontSize     = 48
	DefaultFontSize = 14

	// DefaultFontFamily is the platform's monospaced font.
	DefaultFontFamily = "monospace"

	// DefaultCursorShape is "beam"; the plugin's bar.
	DefaultCursorShape = "beam"
	// DefaultCursorBlink is the shipped behaviour; a program may still ask
	// for a blinking cursor, and the user's veto turns it off.
	DefaultCursorBlink = true

	MinTerminalLineHeight     = 0.5
	MaxTerminalLineHeight     = 2.0
	DefaultTerminalLineHeight = 1.2

	// DefaultTerminalPadding is the 3px inset every earlier build shipped.
	DefaultTerminalPadding = "regular"

	// DefaultTerminalTheme inherits the app theme's colors.
	DefaultTerminalTheme = "inherit"

	DefaultTerminalScrollbar = true

	MinTerminalWheelLines     = 1
	MaxTerminalWheelLines     = 8
	DefaultTerminalWheelLines = 3

	DefaultTerminalBold = "font"
	// DefaultTerminalSelectCopy ships on: a finished selection lands on the
	// clipboard without a second gesture.
	DefaultTerminalSelectCopy = true
	DefaultTerminalPasteSafe  = false

	MinTerminalWidth      = 640
	MaxTerminalWidth      = 2560
	DefaultTerminalWidth  = 860
	MinTerminalHeight     = 360
	MaxTerminalHeight     = 1800
	DefaultTerminalHeight = 600
)

// The shipped value domains. `glassSteps` includes `off`, the no-material
// stop R162 added: it is a real stored value and has to round-trip.
var (
	themes     = []string{"dark", "light", "auto"}
	glassSteps = []string{"off", "frosted", "regular", "liquid"}
	languages  = []string{"en", "zh"}

	// The terminal's string domains, from TERMINAL_THEMES, TERMINAL_PADDINGS,
	// BOLD_MODES and the cursor shapes.
	cursorShapes     = []string{"beam", "block", "underline"}
	terminalPaddings = []string{"compact", "regular", "relaxed"}
	terminalPalettes = []string{"inherit", "contrast", "paper", "ink", "fog", "forest", "dusk", "mist", "amber"}
	boldModes        = []string{"font", "bright"}

	// The pre-GLASS-3STOP five-stop vocabulary and its survivors. Mirrors
	// LEGACY_GLASS_STEPS in config.rs and glass-material.ts.
	legacyGlassSteps = map[string]string{
		"low":   "frosted",
		"mid":   "regular",
		"high":  "liquid",
		"deep":  "liquid",
		"jelly": "liquid",
	}

	// UI_SCALE_STEPS in config.rs (and UI_SCALE_FACTORS in src/ui-scale.ts).
	uiScaleFactors = map[string]float64{
		"tiny":    0.8,
		"small":   0.9,
		"default": 1.0,
		"large":   1.1,
	}

	// R47 retired `larger`; it maps to `large`, its nearest survivor.
	legacyUIScales = map[string]string{"larger": "large"}

	// The keys this package owns. Everything else in the file is carried
	// through untouched.
	knownKeys = map[string]bool{
		"theme":                 true,
		"glass_step":            true,
		"language":              true,
		"main_opacity":          true,
		"terminal_opacity":      true,
		"ui_scale":              true,
		"font_size":             true,
		"font_family":           true,
		"cursor_shape":          true,
		"terminal_cursor_blink": true,
		"terminal_line_height":  true,
		"terminal_padding":      true,
		"terminal_theme":        true,
		"terminal_scrollbar":    true,
		"terminal_wheel_lines":  true,
		"terminal_bold":         true,
		"terminal_select_copy":  true,
		"terminal_paste_safe":   true,
		"terminal_width":        true,
		"terminal_height":       true,
	}
)

// Settings is the settings file, already normalized.
type Settings struct {
	Theme           string
	GlassStep       string
	Language        string
	MainOpacity     uint8
	TerminalOpacity uint8
	UIScale         string

	// The terminal's appearance.
	FontSize           int
	FontFamily         string
	CursorShape        string
	CursorBlink        bool
	TerminalLineHeight float64
	TerminalPadding    string
	TerminalTheme      string
	TerminalScrollbar  bool
	TerminalWheelLines int
	TerminalBold       string
	TerminalSelectCopy bool
	TerminalPasteSafe  bool
	TerminalWidth      float64
	TerminalHeight     float64

	// extra holds every key the struct does not own, exactly as it was
	// decoded, so Encode can write it back.
	extra map[string]any
}

// Default is the settings a missing or unreadable file yields: the shipped
// AppSettings::default(), restricted to the P0 subset.
func Default() Settings {
	return Settings{
		Theme:              DefaultTheme,
		GlassStep:          DefaultGlassStep,
		Language:           DefaultLanguage,
		MainOpacity:        DefaultMainOpacity,
		TerminalOpacity:    DefaultTerminalOpacity,
		UIScale:            DefaultUIScale,
		FontSize:           DefaultFontSize,
		FontFamily:         DefaultFontFamily,
		CursorShape:        DefaultCursorShape,
		CursorBlink:        DefaultCursorBlink,
		TerminalLineHeight: DefaultTerminalLineHeight,
		TerminalPadding:    DefaultTerminalPadding,
		TerminalTheme:      DefaultTerminalTheme,
		TerminalScrollbar:  DefaultTerminalScrollbar,
		TerminalWheelLines: DefaultTerminalWheelLines,
		TerminalBold:       DefaultTerminalBold,
		TerminalSelectCopy: DefaultTerminalSelectCopy,
		TerminalPasteSafe:  DefaultTerminalPasteSafe,
		TerminalWidth:      DefaultTerminalWidth,
		TerminalHeight:     DefaultTerminalHeight,
		extra:              map[string]any{},
	}
}

// Path is the settings file's path, or an error when the platform cannot
// name a config directory.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dirName, fileName), nil
}

// Load reads the settings file at path. A missing file is not an error: it
// yields the defaults, as the Rust loader did. A file that cannot be read or
// parsed also yields the defaults, with the error reported so the caller can
// log it.
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Default(), err
	}
	s, err := Parse(data)
	if err != nil {
		return Default(), err
	}
	return s, nil
}

// LoadDefault reads the settings file at the platform's settings path.
func LoadDefault() (Settings, error) {
	path, err := Path()
	if err != nil {
		return Default(), err
	}
	return Load(path)
}

// Parse decodes and normalizes a settings file. Numbers are decoded with
// json.Number so keys carried through the struct keep their exact spelling.
func Parse(data []byte) (Settings, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return Default(), err
	}

	s := Default()
	if v, ok := raw["theme"].(string); ok {
		s.Theme = v
	}
	if v, ok := raw["glass_step"].(string); ok {
		s.GlassStep = v
	}
	if v, ok := raw["language"].(string); ok {
		s.Language = v
	}
	if n, ok := asInt(raw["main_opacity"]); ok {
		s.MainOpacity = clampByte(n)
	}
	if n, ok := asInt(raw["terminal_opacity"]); ok {
		s.TerminalOpacity = clampByte(n)
	}
	if v, ok := raw["ui_scale"].(string); ok {
		s.UIScale = v
	}

	// The terminal's appearance: a present key overrides the shipped
	// default, exactly as the Rust fields' serde defaults did.
	if n, ok := asInt(raw["font_size"]); ok {
		s.FontSize = n
	}
	if v, ok := raw["font_family"].(string); ok {
		s.FontFamily = v
	}
	if v, ok := raw["cursor_shape"].(string); ok {
		s.CursorShape = v
	}
	if v, ok := raw["terminal_cursor_blink"].(bool); ok {
		s.CursorBlink = v
	}
	if v, ok := asFloat(raw["terminal_line_height"]); ok {
		s.TerminalLineHeight = v
	}
	if v, ok := raw["terminal_padding"].(string); ok {
		s.TerminalPadding = v
	}
	if v, ok := raw["terminal_theme"].(string); ok {
		s.TerminalTheme = v
	}
	if v, ok := raw["terminal_scrollbar"].(bool); ok {
		s.TerminalScrollbar = v
	}
	if n, ok := asInt(raw["terminal_wheel_lines"]); ok {
		s.TerminalWheelLines = n
	}
	if v, ok := raw["terminal_bold"].(string); ok {
		s.TerminalBold = v
	}
	if v, ok := raw["terminal_select_copy"].(bool); ok {
		s.TerminalSelectCopy = v
	}
	if v, ok := raw["terminal_paste_safe"].(bool); ok {
		s.TerminalPasteSafe = v
	}
	if v, ok := asFloat(raw["terminal_width"]); ok {
		s.TerminalWidth = v
	}
	if v, ok := asFloat(raw["terminal_height"]); ok {
		s.TerminalHeight = v
	}

	extra := map[string]any{}
	for key, value := range raw {
		if !knownKeys[key] {
			extra[key] = value
		}
	}
	s.extra = extra
	return s.normalized(), nil
}

// Normalize applies every whitelist and clamp to a Settings value. It is
// exported so tests and callers can pin the rules without a file.
func Normalize(s Settings) Settings { return s.normalized() }

func (s Settings) normalized() Settings {
	s.Theme = NormalizeTheme(s.Theme)
	s.GlassStep = NormalizeGlassStep(s.GlassStep)
	s.Language = NormalizeLanguage(s.Language)
	s.MainOpacity = NormalizeOpacity(s.MainOpacity, DefaultMainOpacity)
	s.TerminalOpacity = NormalizeOpacity(s.TerminalOpacity, DefaultTerminalOpacity)
	s.UIScale = NormalizeUIScale(s.UIScale)
	s.FontSize = NormalizeFontSize(s.FontSize)
	s.FontFamily = NormalizeFontFamily(s.FontFamily)
	s.CursorShape = Pick(cursorShapes, s.CursorShape, DefaultCursorShape)
	s.TerminalLineHeight = NormalizeLineHeight(s.TerminalLineHeight)
	s.TerminalPadding = Pick(terminalPaddings, s.TerminalPadding, DefaultTerminalPadding)
	s.TerminalTheme = Pick(terminalPalettes, s.TerminalTheme, DefaultTerminalTheme)
	s.TerminalWheelLines = NormalizeWheelLines(s.TerminalWheelLines)
	s.TerminalBold = Pick(boldModes, s.TerminalBold, DefaultTerminalBold)
	s.TerminalWidth, s.TerminalHeight = NormalizeTerminalSize(s.TerminalWidth, s.TerminalHeight)
	if s.extra == nil {
		s.extra = map[string]any{}
	}
	return s
}

// Extra returns a copy of the keys the struct does not own.
func (s Settings) Extra() map[string]any {
	out := make(map[string]any, len(s.extra))
	for key, value := range s.extra {
		out[key] = value
	}
	return out
}

// Encode serializes the settings back to the file format, merging the known
// fields over the keys carried through from the file that was read. The
// unknown keys are preserved; a key this package owns is rewritten with its
// normalized value.
func Encode(s Settings) ([]byte, error) {
	s = s.normalized()
	out := make(map[string]any, len(s.extra)+21)
	for key, value := range s.extra {
		out[key] = value
	}
	out["theme"] = s.Theme
	out["glass_step"] = s.GlassStep
	out["language"] = s.Language
	out["main_opacity"] = s.MainOpacity
	out["terminal_opacity"] = s.TerminalOpacity
	out["ui_scale"] = s.UIScale
	out["font_size"] = s.FontSize
	out["font_family"] = s.FontFamily
	out["cursor_shape"] = s.CursorShape
	out["terminal_cursor_blink"] = s.CursorBlink
	out["terminal_line_height"] = s.TerminalLineHeight
	out["terminal_padding"] = s.TerminalPadding
	out["terminal_theme"] = s.TerminalTheme
	out["terminal_scrollbar"] = s.TerminalScrollbar
	out["terminal_wheel_lines"] = s.TerminalWheelLines
	out["terminal_bold"] = s.TerminalBold
	out["terminal_select_copy"] = s.TerminalSelectCopy
	out["terminal_paste_safe"] = s.TerminalPasteSafe
	out["terminal_width"] = s.TerminalWidth
	out["terminal_height"] = s.TerminalHeight
	return json.MarshalIndent(out, "", "  ")
}

// Save writes the settings to path, creating the directory if needed. It is
// not called by the P0 app (which is read-only) but is exercised by the
// round-trip tests that pin unknown-key preservation.
func Save(path string, s Settings) error {
	data, err := Encode(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// NormalizeTheme applies the "dark" | "light" | "auto" whitelist. An unknown
// or empty value falls back to the shipped "auto", as normalize_settings did.
func NormalizeTheme(theme string) string {
	for _, candidate := range themes {
		if theme == candidate {
			return theme
		}
	}
	return DefaultTheme
}

// NormalizeGlassStep applies the four-stop whitelist (including `off`) and
// migrates the retired five-stop ids, as normalize_glass_step did.
func NormalizeGlassStep(step string) string {
	trimmed := strings.ToLower(strings.TrimSpace(step))
	for _, candidate := range glassSteps {
		if trimmed == candidate {
			return trimmed
		}
	}
	if mapped, ok := legacyGlassSteps[trimmed]; ok {
		return mapped
	}
	return DefaultGlassStep
}

// NormalizeLanguage applies the "en" | "zh" whitelist.
func NormalizeLanguage(language string) string {
	for _, candidate := range languages {
		if language == candidate {
			return language
		}
	}
	return DefaultLanguage
}

// NormalizeUIScale applies the interface-size whitelist and migrates the
// retired `larger` step, as normalize_ui_scale did.
func NormalizeUIScale(step string) string {
	if _, ok := uiScaleFactors[step]; ok {
		return step
	}
	if mapped, ok := legacyUIScales[step]; ok {
		return mapped
	}
	return DefaultUIScale
}

// UIScaleFactor is the --ui-scale multiplier a step writes, from
// UI_SCALE_STEPS in config.rs. An unknown step yields the shipped step's
// factor, never zero.
func UIScaleFactor(step string) float64 {
	return uiScaleFactors[NormalizeUIScale(step)]
}

// NormalizeOpacity clamps a transparency percentage to the shipped band. A
// stored 0 is "unset" and lands on fallback, exactly as
// normalize_window_opacity did.
func NormalizeOpacity(value, fallback uint8) uint8 {
	if value == 0 {
		return fallback
	}
	if value < MinWindowOpacity {
		return MinWindowOpacity
	}
	if value > MaxWindowOpacity {
		return MaxWindowOpacity
	}
	return value
}

// Pick returns value when it is one of the allowed ids, and the fallback
// otherwise. It is the shape every id whitelist in this file shares.
func Pick(allowed []string, value, fallback string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

// NormalizeFontSize clamps the terminal font size to the shipped band, as
// config.rs did (a stored 0 lands on the floor, not on the default).
func NormalizeFontSize(size int) int {
	if size < MinFontSize {
		return MinFontSize
	}
	if size > MaxFontSize {
		return MaxFontSize
	}
	return size
}

// NormalizeFontFamily trims the family and falls back to the shipped one
// for blank input, so the plugin gets "monospace" rather than "".
func NormalizeFontFamily(family string) string {
	trimmed := strings.TrimSpace(family)
	if trimmed == "" {
		return DefaultFontFamily
	}
	return trimmed
}

// NormalizeLineHeight clamps the row-height multiple; a non-finite value
// (only reachable without the loader) lands on the shipped 1.2.
func NormalizeLineHeight(height float64) float64 {
	if math.IsNaN(height) || math.IsInf(height, 0) {
		return DefaultTerminalLineHeight
	}
	return math.Min(MaxTerminalLineHeight, math.Max(MinTerminalLineHeight, height))
}

// NormalizeWheelLines clamps how many lines one wheel notch scrolls.
func NormalizeWheelLines(lines int) int {
	if lines < MinTerminalWheelLines {
		return MinTerminalWheelLines
	}
	if lines > MaxTerminalWheelLines {
		return MaxTerminalWheelLines
	}
	return lines
}

// NormalizeTerminalSize clamps the stored terminal window size to its band,
// as normalize_terminal_size did.
func NormalizeTerminalSize(width, height float64) (float64, float64) {
	return math.Min(MaxTerminalWidth, math.Max(MinTerminalWidth, width)),
		math.Min(MaxTerminalHeight, math.Max(MinTerminalHeight, height))
}

func clampByte(n int) uint8 {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return uint8(n)
}

func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func asFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case float64:
		return v, true
	default:
		return 0, false
	}
}
