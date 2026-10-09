package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNormalizesAndKeepsUnknownKeys(t *testing.T) {
	// A file shaped like a real one: the P0 subset plus keys this round does
	// not know (shortcuts, launch_counts, a nested plugin block) and a legacy
	// glass id.
	raw := []byte(`{
		"theme": "light",
		"glass_step": "jelly",
		"language": "zh",
		"main_opacity": 0,
		"terminal_opacity": 250,
		"ui_scale": "larger",
		"shortcuts": {"toggle-window": "Cmd+Shift+Space"},
		"launch_counts": {"Safari": 12},
		"browser_plugin": {"target": "brave"},
		"future_key": true
	}`)

	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if s.Theme != "light" {
		t.Errorf("Theme = %q, want light", s.Theme)
	}
	// `jelly` is a pre-GLASS-3STOP id and collapses onto `liquid`.
	if s.GlassStep != "liquid" {
		t.Errorf("GlassStep = %q, want liquid (legacy jelly migrates)", s.GlassStep)
	}
	if s.Language != "zh" {
		t.Errorf("Language = %q, want zh", s.Language)
	}
	// 0 is "unset" and lands on the shipped default; 250 clamps to 100.
	if s.MainOpacity != DefaultMainOpacity {
		t.Errorf("MainOpacity = %d, want %d", s.MainOpacity, DefaultMainOpacity)
	}
	if s.TerminalOpacity != MaxWindowOpacity {
		t.Errorf("TerminalOpacity = %d, want %d", s.TerminalOpacity, MaxWindowOpacity)
	}
	// R47's retired `larger` maps to `large`.
	if s.UIScale != "large" {
		t.Errorf("UIScale = %q, want large", s.UIScale)
	}

	extra := s.Extra()
	for _, key := range []string{"shortcuts", "launch_counts", "browser_plugin", "future_key"} {
		if _, ok := extra[key]; !ok {
			t.Errorf("unknown key %q was dropped", key)
		}
	}
	for _, key := range []string{"theme", "glass_step", "language", "main_opacity", "ui_scale"} {
		if _, ok := extra[key]; ok {
			t.Errorf("known key %q leaked into Extra", key)
		}
	}
}

func TestUnknownValuesFallBackToTheShippedDefaults(t *testing.T) {
	s, err := Parse([]byte(`{"theme":"blue","glass_step":"ultra","language":"fr","ui_scale":"huge"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Theme != DefaultTheme {
		t.Errorf("Theme = %q, want %q", s.Theme, DefaultTheme)
	}
	if s.GlassStep != DefaultGlassStep {
		t.Errorf("GlassStep = %q, want %q", s.GlassStep, DefaultGlassStep)
	}
	if s.Language != DefaultLanguage {
		t.Errorf("Language = %q, want %q", s.Language, DefaultLanguage)
	}
	if s.UIScale != DefaultUIScale {
		t.Errorf("UIScale = %q, want %q", s.UIScale, DefaultUIScale)
	}
}

func TestGlassStepWhitelistIncludesOff(t *testing.T) {
	// R162's `off` is a real stored stop: the loader must round-trip it or a
	// restart would silently rewrite the user's choice back to the default.
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		s, err := Parse([]byte(`{"glass_step":"` + step + `"}`))
		if err != nil {
			t.Fatalf("Parse(%q): %v", step, err)
		}
		if s.GlassStep != step {
			t.Errorf("glass_step %q normalized to %q", step, s.GlassStep)
		}
	}

	// The pre-GLASS-3STOP five-stop table, both thin and heavy ends.
	legacy := map[string]string{
		"low":   "frosted",
		"mid":   "regular",
		"high":  "liquid",
		"deep":  "liquid",
		"jelly": "liquid",
	}
	for old, want := range legacy {
		if got := NormalizeGlassStep(old); got != want {
			t.Errorf("NormalizeGlassStep(%q) = %q, want %q", old, got, want)
		}
	}
	// Case and whitespace are trimmed, as normalize_glass_step did.
	if got := NormalizeGlassStep("  OFF "); got != "off" {
		t.Errorf("NormalizeGlassStep trimmed = %q, want off", got)
	}
}

func TestOpacityClampBand(t *testing.T) {
	cases := []struct {
		in       uint8
		fallback uint8
		want     uint8
	}{
		{0, DefaultMainOpacity, DefaultMainOpacity}, // unset
		{1, DefaultMainOpacity, MinWindowOpacity},   // below the floor
		{9, DefaultMainOpacity, MinWindowOpacity},
		{10, DefaultMainOpacity, 10},
		{47, DefaultMainOpacity, 47},
		{100, DefaultMainOpacity, 100},
		{101, DefaultMainOpacity, MaxWindowOpacity},
		{255, DefaultMainOpacity, MaxWindowOpacity},
	}
	for _, tc := range cases {
		if got := NormalizeOpacity(tc.in, tc.fallback); got != tc.want {
			t.Errorf("NormalizeOpacity(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestUIScaleFactorsMatchTheShippedSteps(t *testing.T) {
	cases := map[string]float64{
		"tiny":    0.8,
		"small":   0.9,
		"default": 1.0,
		"large":   1.1,
		"larger":  1.1, // retired, maps to large
		"":        0.9, // unknown, the shipped default
	}
	for step, want := range cases {
		if got := UIScaleFactor(step); got != want {
			t.Errorf("UIScaleFactor(%q) = %v, want %v", step, got, want)
		}
	}
}

func TestEncodeRoundTripPreservesUnknownKeys(t *testing.T) {
	raw := []byte(`{
		"theme": "dark",
		"glass_step": "frosted",
		"language": "en",
		"main_opacity": 30,
		"terminal_opacity": 46,
		"ui_scale": "default",
		"clipboard_history_max_items": 200,
		"show_menubar_icon": true,
		"plugin_command_switches": {"browser": {"enabled": true}},
		"custom_shortcuts": [{"action": "toggle-window"}]
	}`)

	first, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	encoded, err := Encode(first)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse(encoded): %v", err)
	}

	if first.Theme != second.Theme || first.GlassStep != second.GlassStep ||
		first.Language != second.Language || first.MainOpacity != second.MainOpacity ||
		first.TerminalOpacity != second.TerminalOpacity || first.UIScale != second.UIScale {
		t.Errorf("known fields changed across the round trip: %+v vs %+v", first, second)
	}

	wantExtra := first.Extra()
	gotExtra := second.Extra()
	if len(wantExtra) != len(gotExtra) {
		t.Fatalf("Extra length %d, want %d (%v vs %v)", len(gotExtra), len(wantExtra), gotExtra, wantExtra)
	}
	for key, want := range wantExtra {
		got, ok := gotExtra[key]
		if !ok {
			t.Errorf("key %q lost in the round trip", key)
			continue
		}
		if !jsonEqual(t, want, got) {
			t.Errorf("key %q = %v, want %v", key, got, want)
		}
	}
}

func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(filepath.Join(dir, "nope", "settings.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !sameSettings(s, Default()) {
		t.Errorf("Load(missing) = %+v, want defaults", s)
	}
}

func TestLoadUnreadableFileYieldsDefaultsAndReports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err == nil {
		t.Fatal("Load(bad json) returned no error")
	}
	if !sameSettings(s, Default()) {
		t.Errorf("Load(bad json) = %+v, want defaults", s)
	}
}

func TestSaveWritesAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "floter", "settings.json")

	s, err := Parse([]byte(`{"glass_step":"off","language":"zh","custom_key":"kept"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if back.GlassStep != "off" || back.Language != "zh" {
		t.Errorf("reloaded = %+v", back)
	}
	if back.Extra()["custom_key"] != "kept" {
		t.Errorf("custom_key = %v, want kept", back.Extra()["custom_key"])
	}
}

func TestPathIsTheOldSettingsFile(t *testing.T) {
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	// config.rs: dirs::config_dir()/floter/settings.json.
	if filepath.Base(path) != "settings.json" {
		t.Errorf("base = %q, want settings.json", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != "floter" {
		t.Errorf("dir = %q, want floter", filepath.Dir(path))
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path %q is not absolute", path)
	}
}

func TestParseDecodesNumbersWithoutLosingPrecision(t *testing.T) {
	// launch_counts can hold large integers; json.Number keeps them exact
	// through a load/save cycle.
	s, err := Parse([]byte(`{"launch_counts":{"x":9007199254740993}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	encoded, err := Encode(s)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(encoded), "9007199254740993") {
		t.Errorf("large integer lost precision: %s", encoded)
	}
}

func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	aj, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal %v: %v", a, err)
	}
	bj, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal %v: %v", b, err)
	}
	return string(aj) == string(bj)
}

func sameSettings(a, b Settings) bool {
	return a.Theme == b.Theme && a.GlassStep == b.GlassStep && a.Language == b.Language &&
		a.MainOpacity == b.MainOpacity && a.TerminalOpacity == b.TerminalOpacity &&
		a.UIScale == b.UIScale && len(a.extra) == len(b.extra)
}

func TestTerminalAppearanceDefaults(t *testing.T) {
	got := Default()
	if got.FontSize != 14 || got.FontFamily != "monospace" || got.CursorShape != "beam" {
		t.Errorf("font defaults = %d %q %q", got.FontSize, got.FontFamily, got.CursorShape)
	}
	if !got.CursorBlink || got.TerminalLineHeight != 1.2 || got.TerminalPadding != "regular" {
		t.Errorf("terminal defaults = %+v", got)
	}
	if got.TerminalTheme != "inherit" || !got.TerminalScrollbar || got.TerminalWheelLines != 3 {
		t.Errorf("terminal defaults = %+v", got)
	}
	if got.TerminalBold != "font" || !got.TerminalSelectCopy || got.TerminalPasteSafe {
		t.Errorf("terminal defaults = %+v", got)
	}
	if got.TerminalWidth != 860 || got.TerminalHeight != 600 {
		t.Errorf("terminal size = %v x %v", got.TerminalWidth, got.TerminalHeight)
	}
}

func TestParseReadsTheTerminalAppearance(t *testing.T) {
	got, err := Parse([]byte(`{
  "font_size": 18,
  "font_family": "  JetBrains Mono  ",
  "cursor_shape": "block",
  "terminal_cursor_blink": false,
  "terminal_line_height": 1.4,
  "terminal_padding": "relaxed",
  "terminal_theme": "amber",
  "terminal_scrollbar": false,
  "terminal_wheel_lines": 5,
  "terminal_bold": "bright",
  "terminal_select_copy": false,
  "terminal_paste_safe": true,
  "terminal_width": 900,
  "terminal_height": 640
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.FontSize != 18 || got.FontFamily != "JetBrains Mono" || got.CursorShape != "block" {
		t.Errorf("font = %d %q %q", got.FontSize, got.FontFamily, got.CursorShape)
	}
	if got.CursorBlink || got.TerminalLineHeight != 1.4 || got.TerminalPadding != "relaxed" {
		t.Errorf("terminal = %+v", got)
	}
	if got.TerminalTheme != "amber" || got.TerminalScrollbar || got.TerminalWheelLines != 5 {
		t.Errorf("terminal = %+v", got)
	}
	if got.TerminalBold != "bright" || got.TerminalSelectCopy || !got.TerminalPasteSafe {
		t.Errorf("terminal = %+v", got)
	}
	if got.TerminalWidth != 900 || got.TerminalHeight != 640 {
		t.Errorf("terminal size = %v x %v", got.TerminalWidth, got.TerminalHeight)
	}
	// The keys are ours now, so they must not ride along in extra.
	if len(got.Extra()) != 0 {
		t.Errorf("owned keys leaked into extra: %v", got.Extra())
	}
}

func TestTerminalAppearanceNormalization(t *testing.T) {
	cases := []struct {
		name string
		in   Settings
		want Settings
	}{
		{
			name: "clamps the font size to the band",
			in:   Default(), // font_size 14 -> 8/48
			want: Default(),
		},
	}
	_ = cases

	low := Default()
	low.FontSize = 1
	low.TerminalWheelLines = 0
	low.TerminalLineHeight = 0.1
	low.TerminalWidth, low.TerminalHeight = 1, 1
	got := Normalize(low)
	if got.FontSize != MinFontSize {
		t.Errorf("font size = %d, want the %d floor", got.FontSize, MinFontSize)
	}
	if got.TerminalWheelLines != MinTerminalWheelLines {
		t.Errorf("wheel lines = %d", got.TerminalWheelLines)
	}
	if got.TerminalLineHeight != MinTerminalLineHeight {
		t.Errorf("line height = %v", got.TerminalLineHeight)
	}
	if got.TerminalWidth != MinTerminalWidth || got.TerminalHeight != MinTerminalHeight {
		t.Errorf("size = %v x %v", got.TerminalWidth, got.TerminalHeight)
	}

	high := Default()
	high.FontSize = 1000
	high.TerminalWheelLines = 99
	high.TerminalLineHeight = 9
	high.TerminalWidth, high.TerminalHeight = 99999, 99999
	high.FontFamily = "   "
	high.CursorShape = "square"
	high.TerminalPadding = "wide"
	high.TerminalTheme = "solarized"
	high.TerminalBold = "loud"
	got = Normalize(high)
	if got.FontSize != MaxFontSize || got.TerminalWheelLines != MaxTerminalWheelLines {
		t.Errorf("font/wheel = %d/%d", got.FontSize, got.TerminalWheelLines)
	}
	if got.TerminalLineHeight != MaxTerminalLineHeight {
		t.Errorf("line height = %v", got.TerminalLineHeight)
	}
	if got.TerminalWidth != MaxTerminalWidth || got.TerminalHeight != MaxTerminalHeight {
		t.Errorf("size = %v x %v", got.TerminalWidth, got.TerminalHeight)
	}
	if got.FontFamily != DefaultFontFamily {
		t.Errorf("blank family = %q, want the default", got.FontFamily)
	}
	if got.CursorShape != DefaultCursorShape || got.TerminalPadding != DefaultTerminalPadding {
		t.Errorf("ids = %q/%q", got.CursorShape, got.TerminalPadding)
	}
	if got.TerminalTheme != DefaultTerminalTheme || got.TerminalBold != DefaultTerminalBold {
		t.Errorf("ids = %q/%q", got.TerminalTheme, got.TerminalBold)
	}
}

func TestTerminalAppearanceRoundTrips(t *testing.T) {
	s := Default()
	s.FontSize = 20
	s.CursorBlink = false
	s.TerminalTheme = "forest"
	s.TerminalSelectCopy = false
	data, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if back.FontSize != 20 || back.CursorBlink || back.TerminalTheme != "forest" || back.TerminalSelectCopy {
		t.Errorf("round trip = %+v", back)
	}
	if len(back.Extra()) != 0 {
		t.Errorf("owned keys leaked into extra: %v", back.Extra())
	}
}

func TestWindowBehaviourDefaultsAndParsing(t *testing.T) {
	got := Default()
	if got.SurfaceResidencySeconds != DefaultSurfaceResidencySeconds {
		t.Errorf("residency = %d, want %d", got.SurfaceResidencySeconds, DefaultSurfaceResidencySeconds)
	}
	if got.HideOnBlur != DefaultHideOnBlur() {
		t.Errorf("hide on blur = %v, want the platform default %v", got.HideOnBlur, DefaultHideOnBlur())
	}

	// A missing key keeps the shipped default rather than zero.
	s, err := Parse([]byte(`{"theme": "dark"}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.SurfaceResidencySeconds != DefaultSurfaceResidencySeconds {
		t.Errorf("missing residency = %d", s.SurfaceResidencySeconds)
	}
	if !s.HideOnBlur {
		t.Error("missing hide_on_blur fell to false")
	}

	// Present keys win, and 0 is a real value (off).
	s, err = Parse([]byte(`{"hide_on_blur": false, "surface_residency_seconds": 0}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.HideOnBlur || s.SurfaceResidencySeconds != 0 {
		t.Errorf("parsed = %+v", s)
	}

	// The sentinel survives; anything above the day cap is clamped.
	if got := NormalizeResidencySeconds(SurfaceResidencyNever); got != SurfaceResidencyNever {
		t.Errorf("never = %d, want the sentinel", got)
	}
	if got := NormalizeResidencySeconds(999_999); got != MaxSurfaceResidencySeconds {
		t.Errorf("above the cap = %d, want %d", got, MaxSurfaceResidencySeconds)
	}
	if got := NormalizeResidencySeconds(45); got != 45 {
		t.Errorf("a custom value = %d, want 45", got)
	}

	// Both keys are ours now: they must not ride along in extra.
	if len(s.Extra()) != 0 {
		t.Errorf("owned keys leaked into extra: %v", s.Extra())
	}
}
