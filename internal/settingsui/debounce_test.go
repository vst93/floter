package settingsui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// A slider drag is one write per pause, not one per pixel. The three
// previewSetting calls stand for three ticks of a drag: each is visible in the
// store at once, the file is untouched while the window runs, and the last
// value lands on disk once the pause passes. (The exact write count is pinned
// in internal/settings' TestStorePreviewCoalescesUntilCommit.)
func TestSliderPreviewDebouncesTheWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	a := New(store, Actions{})
	now := time.Unix(1_700_000_000, 0)
	a.Now = func() time.Time { return now }
	tt := render(t, a, 720, 580)

	for i, opacity := range []uint8{60, 70, 80} {
		now = now.Add(50 * time.Millisecond)
		a.previewSetting(func(s *settings.Settings) { s.MainOpacity = opacity })
		if got := store.Snapshot().MainOpacity; got != opacity {
			t.Fatalf("preview %d: snapshot opacity = %d", i, got)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a preview wrote the file before the window closed: %v", err)
	}

	// Still inside the window: a frame does not write.
	now = now.Add(100 * time.Millisecond)
	tt.Frame()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the frame inside the window wrote the file: %v", err)
	}

	// Past it, the last value lands once.
	now = now.Add(100 * time.Millisecond)
	tt.Frame()
	reloaded, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.MainOpacity != 80 {
		t.Fatalf("the flushed opacity = %d, want 80", reloaded.MainOpacity)
	}

	// A second pass with nothing pending does not rewrite the file: deleting
	// it and advancing the clock leaves it gone.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	tt.Frame()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("an idle frame wrote the file again: %v", err)
	}
}

// The alias field rides the same window: three keystrokes become one action
// call, and a page switch flushes the pending one instead of losing it.
func TestCommandAliasDebouncesAndFlushesOnPageSwitch(t *testing.T) {
	var calls [][3]string
	a := New(newStore(t), Actions{
		SetCommandAlias: func(extensionID, commandID, alias string) {
			calls = append(calls, [3]string{extensionID, commandID, alias})
		},
	})
	now := time.Unix(1_700_000_000, 0)
	a.Now = func() time.Time { return now }
	tt := render(t, a, 720, 580)

	a.debounceAlias("ext", "cmd", "a")
	a.debounceAlias("ext", "cmd", "ab")
	a.debounceAlias("ext", "cmd", "abc")
	if len(calls) != 0 {
		t.Fatalf("the alias was written before the pause: %v", calls)
	}

	now = now.Add(100 * time.Millisecond)
	tt.Frame()
	if len(calls) != 0 {
		t.Fatalf("the alias was written inside the window: %v", calls)
	}

	now = now.Add(100 * time.Millisecond)
	tt.Frame()
	if len(calls) != 1 || calls[0] != [3]string{"ext", "cmd", "abc"} {
		t.Fatalf("the debounced alias = %v, want one abc", calls)
	}

	// A second edit that is still pending is flushed by the page change, not
	// dropped with the old page.
	a.debounceAlias("ext", "cmd", "z")
	now = now.Add(50 * time.Millisecond)
	a.selectPage(PageAbout)
	if len(calls) != 2 || calls[1] != [3]string{"ext", "cmd", "z"} {
		t.Fatalf("the page switch did not flush the alias: %v", calls)
	}
}

// The alias field itself rides the debounce: typing schedules one action
// after the pause, not one per keystroke, and the field keeps the keyboard.
func TestCommandAliasFieldDebouncesTyping(t *testing.T) {
	copy := i18n.For("en").Settings
	var calls [][3]string
	a := New(newStore(t), Actions{
		SetCommandAlias: func(extensionID, commandID, alias string) {
			calls = append(calls, [3]string{extensionID, commandID, alias})
		},
	})
	now := time.Unix(1_700_000_000, 0)
	a.Now = func() time.Time { return now }
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "ext", Name: "V Tools", Enabled: true, Running: true,
			Commands: []Command{{ID: "cmd", Name: "Just run", Enabled: true, Available: true}},
		}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	rect, ok := tt.Find(copy.IntegrationsCommandAliasFor("Just run"))
	if !ok {
		t.Fatalf("the alias field is not on screen: %q", tt.Texts())
	}
	tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	if !tt.Focused(copy.IntegrationsCommandAliasFor("Just run")) {
		t.Fatal("the alias field did not take the keyboard")
	}

	tt.Type("j")
	tt.Frame()
	if len(calls) != 0 {
		t.Fatalf("a keystroke wrote the alias at once: %v", calls)
	}
	now = now.Add(200 * time.Millisecond)
	tt.Frame()
	if len(calls) != 1 || calls[0] != [3]string{"ext", "cmd", "j"} {
		t.Fatalf("the typed alias = %v, want one j", calls)
	}
}

// An immediate set flushes a pending slider preview first, so the two paths
// cannot leave one value in memory and another on disk.
func TestSetFlushesPendingPreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	a := New(store, Actions{})
	a.Now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	a.previewSetting(func(s *settings.Settings) { s.MainOpacity = 80 })
	a.set(func(s *settings.Settings) { s.Language = "zh" })

	got := store.Snapshot()
	if got.MainOpacity != 80 || got.Language != "zh" {
		t.Fatalf("snapshot = opacity %d language %q, want 80/zh", got.MainOpacity, got.Language)
	}
	reloaded, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.MainOpacity != 80 || reloaded.Language != "zh" {
		t.Fatalf("file = opacity %d language %q, want 80/zh", reloaded.MainOpacity, reloaded.Language)
	}
}
