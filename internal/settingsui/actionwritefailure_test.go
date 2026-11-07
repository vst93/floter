package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func actionWriteFailureApp(t *testing.T, dark bool) (*App, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	s := settings.Default()
	s.Language, s.Theme = "zh", appearanceName(dark)
	s.SetShortcut(settings.ShortcutNewCommand, "Ctrl+Shift+B")
	if err := settings.Save(path, s); err != nil {
		t.Fatal(err)
	}
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.Actions.SetShortcut = func(id, key string) { _ = store.Update(func(s *settings.Settings) { s.SetShortcut(id, key) }) }
	a.Actions.ResetShortcuts = func() { _ = store.Update(func(s *settings.Settings) { s.SetExtra("shortcuts", map[string]any{}) }) }
	a.Actions.SetPluginEnabled = func(id string, enabled bool) {
		_ = store.Update(func(s *settings.Settings) { v := settings.ClipboardOf(*s); v.Enabled = enabled; s.SetClipboard(v) })
	}
	a.Actions.SetCommandEnabled = func(ext, id string, enabled bool) {
		_ = store.Update(func(s *settings.Settings) { s.SetCommandSwitch(ext, id, enabled) })
	}
	a.Actions.SetCommandAlias = func(ext, id, alias string) {
		_ = store.Update(func(s *settings.Settings) { s.SetCommandAlias(id, alias) })
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	return a, path
}

func actionWriteFrame(a *App, dark bool, scenario string) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		a.expireSettingsWrites(c)
		ui.Column(c).Padding(20).Gap(12).Children(func() {
			a.saveAlert(c, a.copy())
			switch scenario {
			case "record":
				a.shortcutMapRow(c, a.copy(), settings.ShortcutNewCommand)
			case "reset":
				a.shortcuts(c, a.copy())
			case "plugin":
				a.basePluginCard(c, a.copy(), a.basePlugins(a.copy())[0])
			default:
				for _, id := range []string{"build", "serve"} {
					command := Command{ID: id, Name: id, Enabled: settings.CommandSwitchesOf(a.settings()).Enabled("local.tools", id), Alias: settings.CommandAliasesOf(a.settings())[id]}
					a.commandIdentity(c, a.copy(), "local.tools", command)
					a.commandAliasEditor(c, a.copy(), "local.tools", command)
				}
			}
		})
	}, 720, 520)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func actionWriteDump(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IU_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-iu-" + scenario + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkActionWriteFailure(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, path := actionWriteFailureApp(t, dark)
			tt := actionWriteFrame(a, dark, scenario)
			trigger := func() {
				switch scenario {
				case "record":
					optionExitFocus(t, tt, a.copy().ShortcutRecord)
					tt.Key(0, ui.KeyEnter)
					tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				case "reset":
					if err := tt.Click(a.copy().ShortcutsReset); err != nil {
						t.Fatal(err)
					}
				case "plugin":
					optionExitFocus(t, tt, a.copy().BaseClipboard)
					tt.Key(0, ui.KeySpace)
				case "command":
					optionExitFocus(t, tt, a.copy().IntegrationsCommandEnable("build"))
					tt.Key(0, ui.KeySpace)
				}
			}
			beforeClipboard := settings.ClipboardOf(a.Store.Snapshot()).Enabled
			beforeCommand := settings.CommandSwitchesOf(a.Store.Snapshot()).Enabled("local.tools", "build")
			trigger()
			tt.Frame()
			tt.Frame()
			actionWriteDump(t, tt, scenario, dark)
			if a.Store.LastError() == nil || a.Store.Writes() != 1 {
				t.Fatalf("%s did not attempt one failed write: %d / %v", scenario, a.Store.Writes(), a.Store.LastError())
			}
			if !a.SaveFailedVisible() || !tt.HasText(a.copy().SaveFailedAlert) {
				t.Errorf("%s write failure has no visible recovery feedback", scenario)
			}
			if scenario == "record" && a.rejectedShortcut != "" {
				t.Error("disk failure mislabeled as shortcut conflict")
			}
			if settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand) != "Ctrl+Shift+B" {
				t.Error("failed action changed stored shortcut")
			}
			if settings.ClipboardOf(a.Store.Snapshot()).Enabled != beforeClipboard || settings.CommandSwitchesOf(a.Store.Snapshot()).Enabled("local.tools", "build") != beforeCommand {
				t.Error("failed toggle changed stored switch")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			a.dismissSaveFailed()
			tt.Frame()
			trigger()
			tt.Frame()
			tt.Frame()
			if a.Store.LastError() != nil || a.Store.Writes() != 2 || a.SaveFailedVisible() {
				t.Errorf("%s cannot retry after recovery: %d / %v", scenario, a.Store.Writes(), a.Store.LastError())
			}
		})
	}
}
