package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/launcher"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func fetchDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IH_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ih-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkConfigFetch(t *testing.T, empty bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, owner, old := configEpochApp(dark)
			var latest *launcher.Config
			name := "fetch-new"
			if empty {
				name = "fetch-empty"
			} else {
				latest = &launcher.Config{Schema: old.Schema, Values: plugincfg.Values{"enabled": false, "limit": float64(25)}}
			}
			calls := 0
			owner.Actions.ConfigFor = func(plugin string) *launcher.Config {
				calls++
				if calls == 1 {
					// The public deep-link/configuration entry can be called again
					// by the host while the first lookup is being answered.
					owner.OpenConfig(plugin)
					return old
				}
				return latest
			}
			tt := configEpochFrame(t, dark, owner, a)
			fetchDump(t, tt, name, dark)
			if calls != 2 {
				t.Fatalf("lookups=%d, want 2", calls)
			}
			if owner.Config() != latest {
				t.Errorf("%s lookup overwrote latest configuration: open=%v", name, owner.ConfigOpen())
			}
			if empty && tt.HasText("保留上限") {
				t.Error("empty latest lookup resurrected old controls")
			}
			if !empty && owner.Config().Values["limit"] != float64(25) {
				t.Error("latest limit 25 was replaced by old limit 10")
			}
			tt.Key(0, ui.KeyEscape)
			if owner.ConfigOpen() {
				t.Error("Escape did not close current configuration")
			}
			// Subsequent opens still work after either latest result.
			owner.Actions.ConfigFor = func(string) *launcher.Config { return old }
			owner.EnterClipboard()
			owner.OpenConfig(settings.CustomPluginClipboard)
			tt.Frame()
			if owner.Config() != old || !tt.HasText("保留上限") {
				t.Error("fresh configuration request could not recover")
			}
		})
	}
}

func TestConfigFetchKeepsNewerSheet(t *testing.T)       { checkConfigFetch(t, false) }
func TestConfigFetchKeepsNewerEmptyResult(t *testing.T) { checkConfigFetch(t, true) }
