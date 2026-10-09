package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreUpdateWritesBackAndKeepsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "floter", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{
  "theme": "light",
  "glass_step": "liquid",
  "language": "zh",
  "main_opacity": 47,
  "terminal_opacity": 46,
  "ui_scale": "small",
  "residency_seconds": 10,
  "custom_shortcuts": {"toggle": "Cmd+Shift+Space"},
  "terminal_width": 860
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if got := st.Snapshot().Theme; got != "light" {
		t.Fatalf("theme = %q, want light", got)
	}

	if err := st.Update(func(s *Settings) { s.Theme = "dark"; s.MainOpacity = 80 }); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// The in-memory value changed at once.
	got := st.Snapshot()
	if got.Theme != "dark" || got.MainOpacity != 80 {
		t.Errorf("snapshot = theme %q opacity %d", got.Theme, got.MainOpacity)
	}

	// The file has the new values, the untouched known ones, and every
	// unknown key.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Theme != "dark" || reloaded.MainOpacity != 80 {
		t.Errorf("reloaded = theme %q opacity %d", reloaded.Theme, reloaded.MainOpacity)
	}
	if reloaded.Language != "zh" || reloaded.GlassStep != "liquid" {
		t.Errorf("untouched fields drifted: %+v", reloaded)
	}
	extra := reloaded.Extra()
	if extra["residency_seconds"] == nil || extra["terminal_width"] == nil {
		t.Errorf("unknown keys dropped: %v", extra)
	}
	if nested, ok := extra["custom_shortcuts"].(map[string]any); !ok || nested["toggle"] != "Cmd+Shift+Space" {
		t.Errorf("nested unknown key dropped: %v", extra["custom_shortcuts"])
	}

	// A hand-edited bad value is normalized on the way in, exactly as the
	// loader would.
	if err := st.Update(func(s *Settings) { s.GlassStep = "jelly"; s.MainOpacity = 3 }); err != nil {
		t.Fatal(err)
	}
	got = st.Snapshot()
	if got.GlassStep != "liquid" {
		t.Errorf("glass step = %q, want the legacy jelly -> liquid", got.GlassStep)
	}
	if got.MainOpacity != MinWindowOpacity {
		t.Errorf("opacity = %d, want the %d floor", got.MainOpacity, MinWindowOpacity)
	}
}

func TestStoreUpdateWithoutAChangeTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.OnChange(func(Settings) { calls++ })

	if err := st.Update(func(s *Settings) { s.Theme = NormalizeTheme(s.Theme) }); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("listener ran %d times for a no-op", calls)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a no-op created the file: %v", err)
	}

	if err := st.Update(func(s *Settings) { s.Theme = "light" }); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("listener ran %d times, want 1", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file was not written: %v", err)
	}
}

func TestStoreListenerSeesTheNewValueAndCanLeave(t *testing.T) {
	st := NewStore(Default())
	var seen []string
	off := st.OnChange(func(s Settings) { seen = append(seen, s.Language) })

	if err := st.Update(func(s *Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	off()
	if err := st.Update(func(s *Settings) { s.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "zh" {
		t.Errorf("listener saw %v, want [zh]", seen)
	}
}

func TestStoreKeepsTheValueWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	// A regular file where the settings directory should be makes both the
	// MkdirAll and the write fail.
	blocker := filepath.Join(dir, "floter")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "settings.json")

	st, _ := OpenStore(path)
	if err := st.Update(func(s *Settings) { s.Language = "zh" }); err == nil {
		t.Fatal("Update succeeded over a file, want an error")
	}
	if st.LastError() == nil {
		t.Error("LastError is nil after a failed write")
	}
	if got := st.Snapshot().Language; got != "zh" {
		t.Errorf("in-memory language = %q, want zh even after the failed write", got)
	}
}

func TestStoreSnapshotIsACopy(t *testing.T) {
	st := NewStore(Default())
	snap := st.Snapshot()
	snap.Theme = "light"
	snap.Extra()["x"] = 1
	_ = snap
	if got := st.Snapshot().Theme; got != DefaultTheme {
		t.Errorf("mutating a snapshot changed the store: %q", got)
	}
	if err := st.Update(func(s *Settings) { s.Extra()["y"] = 2 }); err != nil {
		t.Fatal(err)
	}
	if got := st.Snapshot().Extra(); len(got) != 0 {
		t.Errorf("Extra() handed out the map: %v", got)
	}
}
