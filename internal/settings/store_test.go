package settings

import (
	"errors"
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
  "some_future_setting": 10,
  "custom_shortcuts": {"toggle": "Cmd+Shift+Space"},
  "some_future_key": 7
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
	if extra["some_future_setting"] == nil || extra["some_future_key"] == nil {
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

func TestStoreRollsBackWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	// A regular file where the settings directory should be makes both the
	// MkdirAll and the write fail.
	blocker := filepath.Join(dir, "floter")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "settings.json")

	st, _ := OpenStore(path)
	before := st.Snapshot()
	calls := 0
	st.OnChange(func(Settings) { calls++ })
	if err := st.Update(func(s *Settings) { s.Language = "zh" }); err == nil {
		t.Fatal("Update succeeded over a file, want an error")
	}
	if st.LastError() == nil {
		t.Error("LastError is nil after a failed write")
	}
	if got := st.Snapshot(); !equal(before, got) {
		t.Errorf("the failed write left the store changed: %+v, want %+v", got, before)
	}
	if calls != 0 {
		t.Errorf("a refused write notified listeners %d times", calls)
	}
}

// The reference's slider debounce keeps the new value on screen while the
// write waits, and writes once when the gesture stops. Preview is that first
// half, Commit the second: three previews leave the file untouched and a
// single commit writes the last value.
func TestStorePreviewCoalescesUntilCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	st.save = func(path string, s Settings) error {
		writes++
		return Save(path, s)
	}

	for _, opacity := range []uint8{60, 70, 80} {
		st.Preview(func(s *Settings) { s.MainOpacity = opacity })
		if got := st.Snapshot().MainOpacity; got != opacity {
			t.Fatalf("preview %d: snapshot opacity = %d", opacity, got)
		}
	}
	if writes != 0 {
		t.Fatalf("previews wrote the file %d times", writes)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a preview created the file: %v", err)
	}

	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if writes != 1 {
		t.Errorf("commit wrote the file %d times, want 1", writes)
	}
	if got := st.Snapshot().MainOpacity; got != 80 {
		t.Errorf("committed opacity = %d, want 80", got)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.MainOpacity != 80 {
		t.Errorf("reloaded opacity = %d, want 80", reloaded.MainOpacity)
	}

	if err := st.Commit(); err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	if writes != 1 {
		t.Errorf("an unchanged commit wrote the file again (%d writes)", writes)
	}
}

// A preview that is refused on Commit is not left on screen: the visible value
// rolls back to the last confirmed snapshot and listeners hear the rollback.
func TestStorePreviewRollsBackWhenCommitFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	before := st.Snapshot()
	sentinel := errors.New("disk full")
	st.save = func(string, Settings) error { return sentinel }
	seen := []uint8{}
	st.OnChange(func(s Settings) { seen = append(seen, s.MainOpacity) })

	st.Preview(func(s *Settings) { s.MainOpacity = 80 })
	if got := st.Snapshot().MainOpacity; got != 80 {
		t.Fatalf("preview opacity = %d, want 80", got)
	}
	if err := st.Commit(); !errors.Is(err, sentinel) {
		t.Fatalf("Commit error = %v, want %v", err, sentinel)
	}
	if got := st.Snapshot(); !equal(before, got) {
		t.Errorf("a refused commit left the store changed: %+v, want %+v", got, before)
	}
	if st.LastError() == nil {
		t.Error("LastError is nil after a refused commit")
	}
	if len(seen) != 2 || seen[0] != 80 || seen[1] != before.MainOpacity {
		t.Errorf("listeners saw %v, want [80 %d]", seen, before.MainOpacity)
	}
}

// An immediate Update after a preview commits the preview too, and a refused
// Update rolls the whole store back to the last confirmed snapshot rather than
// leaving the uncommitted preview visible.
func TestStoreUpdateAfterPreviewCommitsTheWholeSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	st.save = func(string, Settings) error { writes++; return nil }

	st.Preview(func(s *Settings) { s.MainOpacity = 80 })
	if err := st.Update(func(s *Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if writes != 1 {
		t.Errorf("writes = %d, want 1", writes)
	}
	got := st.Snapshot()
	if got.MainOpacity != 80 || got.Language != "zh" {
		t.Errorf("committed settings = opacity %d language %q", got.MainOpacity, got.Language)
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

// P11-cp · the store says how many times it wrote the file, so a surface can be
// held to "one write per gesture" instead of "one write per frame". The counter
// counts attempts at the one place every path goes through, so it stays honest
// when a test replaces `save` (the refusal test below leans on that).
func TestTheStoreCountsOnlyTheWritesItAttempts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Writes(); got != 0 {
		t.Fatalf("a freshly opened store reports %d writes, want 0", got)
	}

	// A preview is live in memory and writes nothing: that is the whole point of
	// the gesture's first half.
	for _, opacity := range []uint8{60, 70, 80} {
		st.Preview(func(s *Settings) { s.MainOpacity = opacity })
	}
	if got := st.Writes(); got != 0 {
		t.Fatalf("three previews wrote the file %d times, want 0", got)
	}
	if got := st.Snapshot().MainOpacity; got != 80 {
		t.Fatalf("previewed opacity = %d, want 80", got)
	}

	// Commit is the second half: one write for the whole drag.
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := st.Writes(); got != 1 {
		t.Fatalf("a commit wrote the file %d times, want 1", got)
	}

	// An Update that changes something writes once...
	if err := st.Update(func(s *Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	if got := st.Writes(); got != 2 {
		t.Fatalf("an update wrote the file %d times, want 2", got)
	}
	// ...and one that changes nothing writes none: the store compares before it
	// reaches the file, so a no-op knob cannot cost a write.
	if err := st.Update(func(s *Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	if got := st.Writes(); got != 2 {
		t.Fatalf("a no-op update wrote the file (%d writes, want 2)", got)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := st.Writes(); got != 2 {
		t.Fatalf("a no-op commit wrote the file (%d writes, want 2)", got)
	}

	// Save is the forced flush, and it writes whatever is current.
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	if got := st.Writes(); got != 3 {
		t.Fatalf("a save wrote the file %d times, want 3", got)
	}

	// A refused write is still an attempt, and the value rolls back to the last
	// confirmed snapshot rather than to the refused one.
	st.save = func(string, Settings) error { return os.ErrPermission }
	if err := st.Update(func(s *Settings) { s.Language = "en" }); err != nil {
		if !os.IsPermission(err) {
			t.Fatalf("the refusal came back as %v, want a permission error", err)
		}
	}
	if got := st.Writes(); got != 4 {
		t.Fatalf("a refused write reported %d writes, want 4 (the attempt counts)", got)
	}
	if got := st.Snapshot().Language; got != "zh" {
		t.Fatalf("a refused write left the language %q, want the confirmed \"zh\"", got)
	}
}
