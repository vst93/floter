package shell

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func shortcutExitDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JB_DUMP"); tag != "" {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		f, err := os.Create("/tmp/floter-P13-jb/" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkShortcutExit(t *testing.T, custom bool, exit string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			h.app.Surf = SurfaceSettings
			tt := ui.NewTester(func(c *ui.Context) {
				surface := launcher.Resolve(h.app.Store.Snapshot(), c.Theme().Dark, c.Preferences())
				c.SetTheme(surface.Tokens.Theme)
				h.app.surface(c)
			}, 960, 1100)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			copy := i18n.For("en").Settings
			// Type through the actual editor, then leave it unsubmitted.
			editingClick(t, tt, copy.ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("say unfinished")
			key, name := "Ctrl+Alt+J", "builtin-"+exit
			if custom {
				key, name = "Ctrl+Shift+B", "custom-"+exit
			}
			lifecycleRecord(t, h, tt, key)
			switch exit {
			case "page":
				editingClick(t, tt, copy.PageSessions)
			case "close":
				editingClick(t, tt, copy.Close)
				if h.app.Surf != SurfaceLauncher {
					t.Fatal("close did not leave settings")
				}
			case "hide":
				hidden := 0
				h.app.hideWindow = func() { hidden++ }
				h.app.Hide()
				if hidden != 1 {
					t.Fatal("window was not hidden")
				}
			}
			if h.app.Settings.RecordingShortcut() || h.app.shortcutsSuspended {
				t.Error("invisible recorder still suspends global shortcuts")
			}
			if h.live["Ctrl+Alt+J"] == nil || h.live["Ctrl+Shift+B"] == nil {
				t.Error("exit did not restore the summon and custom registrations")
			}
			if exit == "page" {
				editingClick(t, tt, copy.PageShortcuts)
				shortcutExitDump(t, tt, name, dark)
				if tt.HasText(copy.ShortcutRecording) {
					t.Error("return resurrected the old recording")
				}
				editingClick(t, tt, copy.PageSessions)
				// Escape must close this page, not cancel an invisible recorder.
				tt.Key(0, ui.KeyEscape)
				tt.Frame()
				if h.app.Surf != SurfaceLauncher {
					t.Error("invisible recording swallowed page Escape")
				}
			}
			// Reopen through the real host entry, then the real sidebar.
			h.app.Open(SurfaceSettings)
			tt.Frame()
			tt.Frame()
			editingClick(t, tt, copy.PageShortcuts)
			tt.Frame()
			tt.Frame()
			if exit != "page" {
				shortcutExitDump(t, tt, name, dark)
			}
			if tt.HasText(copy.ShortcutRecording) {
				t.Error("reopen resurrected the old recording")
				tt.Key(0, ui.KeyEscape)
				tt.Frame()
			}
			editingClick(t, tt, copy.ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(ui.Cmd, ui.KeyC)
			if tt.Clipboard() != "say unfinished" {
				t.Errorf("exit lost command draft: %q", tt.Clipboard())
			}
			entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
			if len(entries) != 1 || entries[0].Key != "Ctrl+Shift+B" || entries[0].Action != "say original" {
				t.Fatalf("exit submitted or changed binding: %v", entries)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if settings.CustomShortcutsOf(h.app.Store.Snapshot())[0].Action != "say unfinished" {
				t.Error("retained command cannot submit after exit")
			}
			if h.app.Settings.Page != settingsui.PageShortcuts {
				t.Fatal("wrong return page")
			}
		})
	}
}
