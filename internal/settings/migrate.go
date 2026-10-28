// First-run migration from an earlier floter build.
//
// The mygo build reads <config dir>/floter/settings.json — the same file the
// Tauri build wrote — so for the common case the settings are inherited by
// Load alone and there is nothing to convert. This file exists for the cases
// that are *not* the same file:
//
//   - the recovery copy the Tauri build kept beside the primary file
//     (`settings.json.backup`), which Load also reads;
//   - the identifier-named directories a Tauri build may have used instead
//     (`<config|data dir>/com.v.floter/settings.json`);
//   - a file written before the glass step existed, whose single "glass
//     strength" number has to be split across two controls (handled in Parse).
//
// The migration runs once, before the store opens. It never overwrites a
// settings file that already holds user data, copies whatever it read to
// `migrated-from-old.json` so a failure can be inspected, and leaves that copy
// as the marker that says the migration already ran. A corrupt primary file
// counts as "no data": the migration recovers it from a legacy source rather
// than leaving the user with defaults.
package settings

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"floter/internal/pathid"
	"floter/internal/xdg"
)

// migratedFileName is the copy of the legacy file a migration leaves behind,
// and the marker that makes the next run a no-op.
const migratedFileName = "migrated-from-old.json"

// MigrationReport is what a first-run migration did, or why it did nothing.
// It is returned rather than logged so a caller (or a test) can assert on it.
type MigrationReport struct {
	// Migrated is true when a legacy file was converted and written.
	Migrated bool
	// Source is the legacy file that was read, empty when none was.
	Source string
	// Backup is where the legacy file was copied, empty when none was.
	Backup string
	// Reason explains a no-op: "already-migrated", "settings-exist" or
	// "no-legacy".
	Reason string
	// Fields are the setting keys carried over, sorted.
	Fields []string
}

// MigrateDefault runs the migration against the platform's paths. It is a
// no-op when the user already has settings or a migration already ran.
func MigrateDefault() (MigrationReport, error) {
	path, err := Path()
	if err != nil {
		return MigrationReport{Reason: "no-config-dir"}, err
	}
	configDir, _ := os.UserConfigDir()
	return MigrateLegacy(path, LegacySettingsPaths(configDir, platformDataDir()))
}

// LegacySettingsPaths lists the settings files an earlier build may have left,
// most authoritative first: the recovery copy beside the primary file, then
// the identifier-named config and data directories a Tauri build defaults to.
// Paths that do not exist are harmless — the migration skips them.
func LegacySettingsPaths(configDir, dataDir string) []string {
	var paths []string
	if configDir != "" {
		floter := filepath.Join(configDir, dirName)
		paths = append(paths, filepath.Join(floter, fileName+backupSuffix))
		for _, id := range identifierDirs {
			paths = append(paths, filepath.Join(configDir, id, fileName))
		}
	}
	if dataDir != "" {
		for _, id := range identifierDirs {
			paths = append(paths, filepath.Join(dataDir, id, fileName))
		}
	}
	return paths
}

// identifierDirs are the directory names a Tauri build uses for app data: the
// bundle identifier from tauri.conf.json, and the two spellings that predate
// it.
var identifierDirs = []string{"com.v.floter", "v.floter", "floter-go"}

// MigrateLegacy converts an earlier build's settings into newPath. It is
// idempotent (a completed migration leaves migrated-from-old.json behind and
// is never repeated) and conservative (a newPath that already holds user data
// is left untouched).
func MigrateLegacy(newPath string, legacyPaths []string) (MigrationReport, error) {
	if newPath == "" {
		return MigrationReport{Reason: "no-config-dir"}, nil
	}
	dir := filepath.Dir(newPath)
	backupPath := filepath.Join(dir, migratedFileName)

	if fileExists(backupPath) {
		return MigrationReport{Reason: "already-migrated", Backup: backupPath}, nil
	}
	if !isEmptySettingsFile(newPath) {
		return MigrationReport{Reason: "settings-exist"}, nil
	}

	source := ""
	for _, candidate := range legacyPaths {
		if candidate == "" || pathid.Same(candidate, newPath) {
			continue
		}
		if !isEmptySettingsFile(candidate) {
			source = candidate
			break
		}
	}
	if source == "" {
		return MigrationReport{Reason: "no-legacy"}, nil
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return MigrationReport{Reason: "read-failed"}, err
	}
	converted, err := Parse(data)
	if err != nil {
		return MigrationReport{Reason: "parse-failed"}, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return MigrationReport{Reason: "mkdir-failed"}, err
	}
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return MigrationReport{Reason: "backup-failed"}, err
	}
	if err := Save(newPath, converted); err != nil {
		return MigrationReport{Reason: "save-failed"}, err
	}

	return MigrationReport{
		Migrated: true,
		Source:   source,
		Backup:   backupPath,
		Fields:   sortedKeys(data),
	}, nil
}

// sortedKeys names the keys a legacy file carried, sorted, so the report says
// what was actually converted rather than listing every field this build owns.
func sortedKeys(data []byte) []string {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// isEmptySettingsFile reports whether path is absent, blank, unparseable or an
// empty JSON object — the states in which a migration may write over it. A
// file with any key at all is user data and is never touched.
func isEmptySettingsFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return true
	}
	var raw map[string]any
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return true
	}
	return len(raw) == 0
}

// fileExists reports whether path names an existing file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// platformDataDir is the per-user data directory (as opposed to the config
// directory): %LOCALAPPDATA% on Windows, ~/Library/Application Support on
// macOS, $XDG_DATA_HOME or ~/.local/share on Linux. A Tauri build's default
// app-data directory is identifier-based, so the migration looks here too.
func platformDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return dataDirFor(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), xdg.DataHome())
}

// dataDirFor is platformDataDir with the environment passed in, so the table
// can be asserted from any host.
//
// The Linux arm delegates to the one answer this build has for "where is the
// per-user data directory" (`xdg.DataHome`, which also enforces the
// specification's rule that a *relative* `$XDG_DATA_HOME` is ignored). It used
// to spell the same two paths out by hand — and without that rule, so a
// `XDG_DATA_HOME=rel/dir` made the legacy import look for
// `com.v.floter/settings.json` under the process's current directory: nothing
// found when the user launched floter from somewhere else, or another
// directory's settings imported when they launched it from the wrong place.
func dataDirFor(goos, home, localAppData, dataHome string) string {
	switch goos {
	case "windows":
		return localAppData
	case "darwin":
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support")
	default:
		return dataHome
	}
}
