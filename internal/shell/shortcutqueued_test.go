package shell

import (
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func queuedDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JA_DUMP"); tag != "" {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		f, err := os.Create("/tmp/floter-P13-ja-" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQueuedCustomRemovedBindingCannotDispatch(t *testing.T) { checkQueuedCustomEdit(t, false) }
func TestQueuedCustomReplacedActionCannotDispatch(t *testing.T) { checkQueuedCustomEdit(t, true) }

func checkQueuedCustomEdit(t *testing.T, replace bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			commands := []string{}
			h.app.silentCommand = func(s string) error { commands = append(commands, s); return nil }
			old := h.live["Ctrl+Shift+B"]
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			name := "custom-removed"
			if replace {
				name = "custom-replaced"
				editingClick(t, tt, copy.ShortcutsCommand)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("say replacement")
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
				if len(entries) != 1 || entries[0].Action != "say replacement" {
					t.Fatalf("edit not committed: %v", entries)
				}
			} else {
				editingClick(t, tt, copy.ShortcutsRemove)
				if len(settings.CustomShortcutsOf(h.app.Store.Snapshot())) != 0 {
					t.Fatal("delete not committed")
				}
			}
			old() // native delivery was already queued when the key was released
			queuedDump(t, tt, name, dark)
			if len(commands) != 0 {
				t.Fatalf("retired custom callback dispatched: %v", commands)
			}
			if replace {
				h.live["Ctrl+Shift+B"]()
				if len(commands) != 1 || commands[0] != "say replacement" {
					t.Fatalf("current action unavailable: %v", commands)
				}
			}
		})
	}
}

func TestQueuedCustomSuspensionCannotDispatch(t *testing.T) {
	h := newShortcutLifecycle(t, "Ctrl+Alt+J")
	commands := []string{}
	h.app.silentCommand = func(s string) error { commands = append(commands, s); return nil }
	old := h.live["Ctrl+Shift+B"]
	// The host suspension is also used before the view reports its recorder.
	h.app.setShortcutsSuspended(true)
	old()
	if len(commands) != 0 {
		t.Fatalf("suspended native callback dispatched: %v", commands)
	}
	h.app.setShortcutsSuspended(false)
	h.live["Ctrl+Shift+B"]()
	if len(commands) != 1 {
		t.Fatalf("resumed current callback unavailable: %v", commands)
	}
}

func TestQueuedCustomFailedRegistrationCannotDispatch(t *testing.T) {
	h := newShortcutLifecycle(t, "Ctrl+Alt+J")
	commands := []string{}
	h.app.silentCommand = func(s string) error { commands = append(commands, s); return nil }
	register := h.app.registerShortcut
	var refused func()
	h.blocked["Ctrl+Shift+Z"] = true
	h.app.registerShortcut = func(key string, fn func()) error {
		if key == "Ctrl+Shift+Z" {
			refused = fn
		}
		return register(key, fn)
	}
	tt := h.frame(false)
	copy := i18n.For("en").Settings
	editingClick(t, tt, copy.ShortcutsAdd)
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
	tt.Frame()
	// The draft defaults to a plugin; use a command to observe dispatch without a native window.
	editingClick(t, tt, copy.ShortcutsActions[0].Label)
	editingClick(t, tt, copy.ShortcutsActions[len(copy.ShortcutsActions)-1].Label)
	editingClick(t, tt, copy.ShortcutsCommand)
	tt.Type("say refused")
	editingClick(t, tt, copy.ShortcutsBind)
	if refused == nil || !tt.HasText(copy.ShortcutsOccupied) {
		t.Fatal("did not reach native refusal")
	}
	refused()
	if len(commands) != 0 {
		t.Fatalf("refused registration callback dispatched: %v", commands)
	}
	h.live["Ctrl+Shift+B"]()
	if len(commands) != 1 || commands[0] != "say original" {
		t.Fatalf("accepted neighbor unavailable: %v", commands)
	}
}

func TestQueuedSummonCannotInterruptRecording(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			old := h.live["Ctrl+Alt+J"]
			tt := h.frame(dark)
			lifecycleRecord(t, h, tt, "Ctrl+Alt+J")
			queuedSummon(t, h.app, old, false)
			queuedDump(t, tt, "summon-recording", dark)
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			tt.Frame()
			queuedSummon(t, h.app, h.live["Ctrl+Alt+J"], true)
		})
	}
}

// Stop Toggle at its existing repeat gate, so no native window is needed.
// Reaching the app clock proves that the stale callback reached Toggle.
func queuedSummon(t *testing.T, a *App, fn func(), want bool) {
	t.Helper()
	if fn == nil {
		t.Fatal("missing native callback")
	}
	clock := time.Unix(1000, 0)
	calls := 0
	oldWin, oldNow, oldAt := a.Win, a.now, a.toggleAt
	a.Win = &mygo.Window{}
	a.toggleAt = clock
	a.now = func() time.Time { calls++; return clock }
	defer func() { a.Win, a.now, a.toggleAt = oldWin, oldNow, oldAt }()
	fn()
	if (calls > 0) != want {
		t.Errorf("queued summon reached Toggle = %d, want dispatch %v", calls, want)
	}
}
