package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

// writeFile creates the parent directories and writes content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyConvertsAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	legacyBody := `{
		"theme": "dark",
		"language": "zh",
		"ui_scale": "large",
		"glass_step": "liquid",
		"hotkey": "Ctrl+Alt+Space",
		"clipboard_history_max_items": 42,
		"future_key": true
	}`
	writeFile(t, legacy, legacyBody)

	report, err := MigrateLegacy(newPath, []string{legacy})
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	if !report.Migrated || report.Source != legacy {
		t.Fatalf("report = %+v, want a migration from %s", report, legacy)
	}
	if report.Backup != filepath.Join(dir, "floter", "migrated-from-old.json") {
		t.Errorf("backup = %q", report.Backup)
	}

	got, err := Load(newPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != "dark" || got.Language != "zh" || got.UIScale != "large" || got.GlassStep != "liquid" {
		t.Errorf("converted = %+v", got)
	}
	if got.Extra()["hotkey"] != "Ctrl+Alt+Space" {
		t.Errorf("hotkey = %v, want Ctrl+Alt+Space", got.Extra()["hotkey"])
	}
	if got.Extra()["future_key"] != true {
		t.Errorf("future_key = %v, want true", got.Extra()["future_key"])
	}
	if !contains(report.Fields, "hotkey") || !contains(report.Fields, "clipboard_history_max_items") {
		t.Errorf("report fields = %v", report.Fields)
	}

	// The legacy bytes are preserved verbatim for inspection.
	backup, err := os.ReadFile(report.Backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != legacyBody {
		t.Errorf("backup = %s, want the legacy file verbatim", backup)
	}
}

func TestMigrateLegacyIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	writeFile(t, legacy, `{"theme":"light"}`)

	if _, err := MigrateLegacy(newPath, []string{legacy}); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	first, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}

	// A second run sees the marker and leaves the converted file alone, even
	// though the legacy source still exists.
	report, err := MigrateLegacy(newPath, []string{legacy})
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if report.Migrated || report.Reason != "already-migrated" {
		t.Fatalf("second report = %+v, want already-migrated", report)
	}
	second, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("the settings file changed on the second migration")
	}
}

func TestMigrateLegacyDoesNotOverwriteExistingSettings(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	writeFile(t, newPath, `{"theme":"light","language":"zh"}`)
	writeFile(t, legacy, `{"theme":"dark"}`)

	report, err := MigrateLegacy(newPath, []string{legacy})
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	if report.Migrated || report.Reason != "settings-exist" {
		t.Fatalf("report = %+v, want settings-exist", report)
	}
	got, _ := Load(newPath)
	if got.Theme != "light" || got.Language != "zh" {
		t.Errorf("the existing file was touched: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "floter", "migrated-from-old.json")); !os.IsNotExist(err) {
		t.Error("a no-op migration must not leave a marker")
	}
}

func TestMigrateLegacyFillsMissingFieldsWithDefaults(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	writeFile(t, legacy, `{"theme":"dark"}`)

	if _, err := MigrateLegacy(newPath, []string{legacy}); err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	got, _ := Load(newPath)
	want := Default()
	if got.Theme != "dark" {
		t.Errorf("theme = %q, want dark", got.Theme)
	}
	if got.Language != want.Language || got.GlassStep != want.GlassStep ||
		got.MainOpacity != want.MainOpacity || got.TerminalOpacity != want.TerminalOpacity ||
		got.FontSize != want.FontSize || got.TerminalWidth != want.TerminalWidth {
		t.Errorf("missing fields did not fall back to defaults: %+v", got)
	}
}

func TestMigrateLegacyTreatsAnEmptyNewFileAsAbsent(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	writeFile(t, newPath, "{}")
	writeFile(t, legacy, `{"theme":"dark"}`)

	report, err := MigrateLegacy(newPath, []string{legacy})
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	if !report.Migrated {
		t.Fatalf("report = %+v, want a migration over the empty file", report)
	}
	got, _ := Load(newPath)
	if got.Theme != "dark" {
		t.Errorf("theme = %q, want dark", got.Theme)
	}
}

func TestMigrateLegacyRecoversFromACorruptNewFile(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	writeFile(t, newPath, "{truncated")
	writeFile(t, legacy, `{"theme":"light"}`)

	report, err := MigrateLegacy(newPath, []string{legacy})
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	if !report.Migrated {
		t.Fatalf("report = %+v, want a migration over the corrupt file", report)
	}
	got, _ := Load(newPath)
	if got.Theme != "light" {
		t.Errorf("theme = %q, want light", got.Theme)
	}
}

func TestMigrateLegacyWithoutASourceDoesNothing(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")

	report, err := MigrateLegacy(newPath, []string{filepath.Join(dir, "com.v.floter", "settings.json")})
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	if report.Migrated || report.Reason != "no-legacy" {
		t.Fatalf("report = %+v, want no-legacy", report)
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Error("a no-op migration must not create a settings file")
	}
}

func TestMigrateLegacySplitsThePreGlassStrength(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "floter", "settings.json")
	legacy := filepath.Join(dir, "com.v.floter", "settings.json")
	// A pre-R8 file: no `glass_step`, one strength number.
	writeFile(t, legacy, `{"main_opacity":94,"terminal_opacity":92}`)

	if _, err := MigrateLegacy(newPath, []string{legacy}); err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	got, _ := Load(newPath)
	if got.MainOpacity != 47 || got.TerminalOpacity != 46 {
		t.Errorf("opacity = %d/%d, want 47/46", got.MainOpacity, got.TerminalOpacity)
	}
	if got.GlassStep != "liquid" {
		t.Errorf("glass_step = %q, want liquid", got.GlassStep)
	}
}

func TestLegacySettingsPathsNamesTheBackupAndIdentifierDirs(t *testing.T) {
	// The paths are built with the host's separator (`filepath.Join`), so the
	// expectations are too: a literal "/config/floter" is a Linux-only answer,
	// and on Windows the same list reads `\config\floter`.
	paths := LegacySettingsPaths("/config", "/data")
	want := []string{
		filepath.Join("/config", "floter", "settings.json.backup"),
		filepath.Join("/config", "com.v.floter", "settings.json"),
		filepath.Join("/config", "v.floter", "settings.json"),
		filepath.Join("/config", "floter-go", "settings.json"),
		filepath.Join("/data", "com.v.floter", "settings.json"),
		filepath.Join("/data", "v.floter", "settings.json"),
		filepath.Join("/data", "floter-go", "settings.json"),
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestLoadRecoversFromTheBackupCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	writeFile(t, path, "{truncated")
	writeFile(t, path+backupSuffix, `{"theme":"light","font_size":22}`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != "light" || got.FontSize != 22 {
		t.Errorf("recovered = %+v", got)
	}
}

func TestParseSplitsTheLegacyGlassStrengthOnce(t *testing.T) {
	s, err := Parse([]byte(`{"main_opacity":100,"terminal_opacity":80}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.MainOpacity != 50 || s.TerminalOpacity != 40 || s.GlassStep != "liquid" {
		t.Fatalf("split = %d/%d/%s, want 50/40/liquid", s.MainOpacity, s.TerminalOpacity, s.GlassStep)
	}
	// A file that carries the step is not split, so a re-read is a no-op.
	encoded, err := Encode(s)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	again, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse(encoded): %v", err)
	}
	if again.MainOpacity != 50 || again.TerminalOpacity != 40 {
		t.Errorf("the split ran twice: %+v", again)
	}
}

func TestMigrateLegacyWindowStateConvertsPerContentGeometry(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "floter")
	legacy := filepath.Join(root, legacyGeometryFileName)
	writeFile(t, legacy, `{
		"version": 2,
		"windows": {
			"local.tool\u0000run": {"geometry": {"width": 800.4, "height": 600.6, "x": 10, "y": 20}, "last_used": 3},
			"a title": {"geometry": {"width": 500, "height": 400, "x": -5, "y": 0}, "last_used": 1}
		},
		"default": {"width": 720, "height": 480, "x": 0, "y": 0}
	}`)

	report, err := MigrateLegacyWindowState(dir)
	if err != nil {
		t.Fatalf("MigrateLegacyWindowState: %v", err)
	}
	if !report.Migrated || report.Source != legacy {
		t.Fatalf("report = %+v", report)
	}
	data, err := os.ReadFile(filepath.Join(root, windowStateFileName))
	if err != nil {
		t.Fatalf("read window-state: %v", err)
	}
	var states map[string]mygo.WindowState
	if err := json.Unmarshal(data, &states); err != nil {
		t.Fatalf("window-state is not the framework's shape: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("states = %v, want two", states)
	}
	got := states[DetachedStateKeyPrefix+"local.tool\x00run"]
	if got.Width != 800 || got.Height != 601 || got.X != 10 || got.Y != 20 {
		t.Errorf("geometry = %+v, want 800x601 at 10,20 (rounded)", got)
	}
	if _, ok := states[DetachedStateKeyPrefix+"a title"]; !ok {
		t.Error("the title-keyed geometry was not converted")
	}
}

func TestMigrateLegacyWindowStateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "floter")
	writeFile(t, filepath.Join(root, legacyGeometryFileName),
		`{"version":2,"windows":{"x":{"geometry":{"width":800,"height":600,"x":1,"y":2}}}}`)

	if _, err := MigrateLegacyWindowState(dir); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(root, windowStateFileName))

	report, err := MigrateLegacyWindowState(dir)
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if report.Migrated || report.Reason != "already-migrated" {
		t.Fatalf("report = %+v, want already-migrated", report)
	}
	second, _ := os.ReadFile(filepath.Join(root, windowStateFileName))
	if string(first) != string(second) {
		t.Error("window-state changed on the second migration")
	}
}

func TestMigrateLegacyWindowStateLeavesExistingStateAlone(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "floter")
	writeFile(t, filepath.Join(root, windowStateFileName), `{"existing":{"x":1,"y":2,"width":3,"height":4}}`)
	writeFile(t, filepath.Join(root, legacyGeometryFileName),
		`{"version":2,"windows":{"x":{"geometry":{"width":800,"height":600,"x":1,"y":2}}}}`)

	report, err := MigrateLegacyWindowState(dir)
	if err != nil {
		t.Fatalf("MigrateLegacyWindowState: %v", err)
	}
	if report.Migrated || report.Reason != "state-exists" {
		t.Fatalf("report = %+v, want state-exists", report)
	}
	data, _ := os.ReadFile(filepath.Join(root, windowStateFileName))
	if !strings.Contains(string(data), `"existing"`) {
		t.Errorf("the existing state was overwritten: %s", data)
	}
}

func TestMigrateLegacyWindowStateSkipsAV1Rectangle(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "floter")
	writeFile(t, filepath.Join(root, legacyGeometryFileName),
		`{"width":800,"height":600,"x":1,"y":2}`)

	report, err := MigrateLegacyWindowState(dir)
	if err != nil {
		t.Fatalf("MigrateLegacyWindowState: %v", err)
	}
	if report.Migrated || report.Reason != "no-content-key" {
		t.Fatalf("report = %+v, want no-content-key", report)
	}
	if _, err := os.Stat(filepath.Join(root, windowStateFileName)); !os.IsNotExist(err) {
		t.Error("a v1 rectangle must not produce a window-state file")
	}
}

func TestMigrateDefaultInheritsFromTheIdentifierDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME only moves os.UserConfigDir on Linux")
	}
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	writeFile(t, filepath.Join(config, "com.v.floter", "settings.json"),
		`{"theme":"dark","language":"zh","font_size":20}`)

	report, err := MigrateDefault()
	if err != nil {
		t.Fatalf("MigrateDefault: %v", err)
	}
	if !report.Migrated {
		t.Fatalf("report = %+v, want a migration", report)
	}
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != "dark" || got.Language != "zh" || got.FontSize != 20 {
		t.Errorf("inherited = %+v", got)
	}

	// A second first-run is a no-op: the marker is there.
	report, err = MigrateDefault()
	if err != nil {
		t.Fatalf("second MigrateDefault: %v", err)
	}
	if report.Migrated || report.Reason != "already-migrated" {
		t.Errorf("second report = %+v, want already-migrated", report)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
