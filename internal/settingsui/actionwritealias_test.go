package settingsui

import (
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"os"
	"testing"
	"time"
)

func TestActionWriteFailureAliases(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, mixed := range []bool{false, true} {
			name := "alias-single"
			if mixed {
				name = "alias-mixed"
			}
			t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
				a, path := actionWriteFailureApp(t, dark)
				now := time.Unix(1700000000, 0)
				a.Now = func() time.Time { return now }
				original := a.Actions.SetCommandAlias
				a.Actions.SetCommandAlias = func(ext, id, alias string) {
					if mixed && id == "serve" {
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
					}
					original(ext, id, alias)
				}
				tt := actionWriteFrame(a, dark, "alias")
				optionExitFocus(t, tt, a.copy().IntegrationsCommandAliasFor("build"))
				tt.Type("compile")
				if mixed {
					optionExitFocus(t, tt, a.copy().IntegrationsCommandAliasFor("serve"))
					tt.Type("preview")
				}
				now = now.Add(179 * time.Millisecond)
				tt.Frame()
				if a.Store.Writes() != 0 {
					t.Fatal("alias wrote before 180ms")
				}
				now = now.Add(time.Millisecond)
				tt.Frame()
				tt.Frame()
				tt.Frame()
				actionWriteDump(t, tt, name, dark)
				wantWrites := uint64(1)
				if mixed {
					wantWrites = 2
				}
				if a.Store.Writes() != wantWrites {
					t.Fatalf("alias writes = %d want %d", a.Store.Writes(), wantWrites)
				}
				if !a.SaveFailedVisible() || !tt.HasText(a.copy().SaveFailedAlert) {
					t.Errorf("%s lost the failed alias write behind a later commit", name)
				}
				aliases := settings.CommandAliasesOf(a.Store.Snapshot())
				if aliases["build"] != "" || mixed && aliases["serve"] != "preview" {
					t.Errorf("alias batch persisted wrong values: %v", aliases)
				}
				if !mixed {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				a.dismissSaveFailed()
				tt.Frame()
				optionExitFocus(t, tt, a.copy().IntegrationsCommandAliasFor("build"))
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("compile")
				a.Flush()
				tt.Frame()
				if settings.CommandAliasesOf(a.Store.Snapshot())["build"] != "compile" || a.SaveFailedVisible() {
					t.Error("alias cannot retry through Flush")
				}
			})
		}
	}
}
