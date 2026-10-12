// Migration of the Tauri build's detached-window geometry into the
// window-state file the mygo build reads.
//
// The old build remembered a detached output window's placement per content in
// `<config dir>/floter/plugin-window-geometry.json`. The mygo build asks the
// framework to remember it under a `StateKey`, which lands in
// `<config dir>/floter/window-state.json`. The two files are different shapes,
// so without this conversion the first detach after the move opens at the
// default 720×460 instead of where the user left it.
package settings

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// DetachedStateKeyPrefix is the key a detached output window's geometry is
// remembered under, in window-state.json. The shell composes it with the old
// build's content identity, so the same window keeps its placement.
const DetachedStateKeyPrefix = "detached:\x00"

const (
	windowStateFileName    = "window-state.json"
	legacyGeometryFileName = "plugin-window-geometry.json"
	migratedWindowFileName = "migrated-window-geometry.json"
)

// WindowStateReport is what a window-state migration did, or why it did not.
type WindowStateReport struct {
	Migrated bool
	Source   string
	Backup   string
	Reason   string
	Keys     []string
}

// MigrateDefaultWindowState runs the window-state migration against the
// platform's config directory.
func MigrateDefaultWindowState() (WindowStateReport, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return WindowStateReport{Reason: "no-config-dir"}, err
	}
	return MigrateLegacyWindowState(configDir)
}

// MigrateLegacyWindowState converts `plugin-window-geometry.json` into
// `window-state.json`, keyed the way the shell keys detached windows. It is
// idempotent and never overwrites a window-state file that already exists.
//
// The old v1 file (a single bare rectangle, no content key) is left alone: a
// rectangle with no content identity cannot be assigned to a window the new
// build would look up. A v1 file that the old build itself upgraded to v2 has
// its `default` entry, which is likewise unmappable; only per-content entries
// are converted.
func MigrateLegacyWindowState(configDir string) (WindowStateReport, error) {
	if configDir == "" {
		return WindowStateReport{Reason: "no-config-dir"}, nil
	}
	root := filepath.Join(configDir, dirName)
	newPath := filepath.Join(root, windowStateFileName)
	legacyPath := filepath.Join(root, legacyGeometryFileName)
	backupPath := filepath.Join(root, migratedWindowFileName)

	if fileExists(backupPath) {
		return WindowStateReport{Reason: "already-migrated", Backup: backupPath}, nil
	}
	if !isEmptySettingsFile(newPath) {
		return WindowStateReport{Reason: "state-exists"}, nil
	}
	if !fileExists(legacyPath) {
		return WindowStateReport{Reason: "no-legacy"}, nil
	}

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return WindowStateReport{Reason: "read-failed"}, err
	}
	var store legacyGeometryStore
	if err := json.Unmarshal(data, &store); err != nil {
		return WindowStateReport{Reason: "parse-failed"}, err
	}
	states := map[string]windowState{}
	for content, entry := range store.Windows {
		if content == "" {
			continue
		}
		states[DetachedStateKeyPrefix+content] = toWindowState(entry.Geometry)
	}
	if len(states) == 0 {
		return WindowStateReport{Reason: "no-content-key"}, nil
	}

	encoded, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return WindowStateReport{Reason: "encode-failed"}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return WindowStateReport{Reason: "mkdir-failed"}, err
	}
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return WindowStateReport{Reason: "backup-failed"}, err
	}
	if err := os.WriteFile(newPath, encoded, 0o644); err != nil {
		return WindowStateReport{Reason: "save-failed"}, err
	}

	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return WindowStateReport{
		Migrated: true,
		Source:   legacyPath,
		Backup:   backupPath,
		Keys:     keys,
	}, nil
}

// legacyGeometry is one remembered placement, in logical coordinates.
type legacyGeometry struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// legacyStoredGeometry is one content key's placement plus when it was last
// written.
type legacyStoredGeometry struct {
	Geometry legacyGeometry `json:"geometry"`
	LastUsed int64          `json:"last_used"`
}

// legacyGeometryStore is the old v2 file: a placement per content key, plus
// the `default` a v1 file migrated into.
type legacyGeometryStore struct {
	Version int                             `json:"version"`
	Windows map[string]legacyStoredGeometry `json:"windows"`
	Default *legacyGeometry                 `json:"default"`
}

// windowState is the framework's own per-key shape.
type windowState struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func toWindowState(g legacyGeometry) windowState {
	return windowState{
		X:      int(math.Round(g.X)),
		Y:      int(math.Round(g.Y)),
		Width:  int(math.Round(g.Width)),
		Height: int(math.Round(g.Height)),
	}
}
