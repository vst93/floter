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
