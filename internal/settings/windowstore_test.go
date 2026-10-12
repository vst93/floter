package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

func TestWindowStoreRoundTripsThroughTheSettingsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "floter", fileName)
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	want := map[string]mygo.WindowState{
		DetachedStateKeyPrefix + "local.tool\x00run": {X: 10, Y: 20, Width: 800, Height: 600},
		"pinned": {X: 30, Y: 40, Width: 720, Height: 480, Maximized: true},
	}

	if err := NewWindowStore(store).SaveWindowStates(want); err != nil {
		t.Fatalf("SaveWindowStates: %v", err)
	}

	// The state is in the settings file, under the carried key, with the
	// rest of the settings still there.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("settings is not JSON: %v", err)
	}
	if _, ok := raw[windowStatesKey]; !ok {
		t.Fatalf("settings has no %q key: %s", windowStatesKey, data)
	}
	if _, ok := raw["theme"]; !ok {
		t.Error("writing window state dropped the settings themselves")
	}

	// A store opened over the same file reads them back.
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := NewWindowStore(reopened).LoadWindowStates()
	if err != nil {
		t.Fatalf("LoadWindowStates: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %d states, want %d: %v", len(got), len(want), got)
	}
	for key, state := range want {
		if got[key] != state {
			t.Errorf("state %q = %+v, want %+v", key, got[key], state)
		}
	}
}

func TestWindowStoreRoundTripsInMemory(t *testing.T) {
	store := NewStore(Default())
	want := map[string]mygo.WindowState{"pinned": {X: 1, Y: 2, Width: 3, Height: 4}}
	ws := NewWindowStore(store)
	if err := ws.SaveWindowStates(want); err != nil {
		t.Fatalf("SaveWindowStates: %v", err)
	}
	got, err := ws.LoadWindowStates()
	if err != nil {
		t.Fatalf("LoadWindowStates: %v", err)
	}
	if got["pinned"] != want["pinned"] {
		t.Errorf("state = %+v, want %+v", got["pinned"], want["pinned"])
	}
}

// The migration (windowstate.go) writes window-state.json beside the settings
// file. A settings file that has no states yet is seeded from it, so an
// in-place upgrade keeps the geometry the old build left behind.
func TestWindowStoreSeedsFromTheMigratedFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "floter")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]mygo.WindowState{
		DetachedStateKeyPrefix + "local.tool\x00run": {X: 5, Y: 6, Width: 700, Height: 500},
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(root, windowStateFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(root, fileName))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	got, err := NewWindowStore(store).LoadWindowStates()
	if err != nil {
		t.Fatalf("LoadWindowStates: %v", err)
	}
	if len(got) != 1 || got[DetachedStateKeyPrefix+"local.tool\x00run"] != legacy[DetachedStateKeyPrefix+"local.tool\x00run"] {
		t.Fatalf("seeded states = %+v, want %+v", got, legacy)
	}
}

func TestWindowStoreWithoutStates(t *testing.T) {
	store := NewStore(Default())
	got, err := NewWindowStore(store).LoadWindowStates()
	if err != nil {
		t.Fatalf("LoadWindowStates: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("states = %+v, want none", got)
	}
}
