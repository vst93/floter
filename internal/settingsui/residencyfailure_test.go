package settingsui

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func checkResidencyFailure(t *testing.T, preset bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			s := settings.Default()
			s.Language, s.Theme, s.SurfaceResidencySeconds = "zh", appearanceName(dark), 45
			if err := settings.Save(path, s); err != nil {
				t.Fatal(err)
			}
			store, err := settings.OpenStore(path)
			if err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			tt := residencyRefreshFrame(a, dark)
			optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("30")
			// Replace only this fixture's file with a directory to reject the
			// atomic rename even when the tests run as root.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			name := "custom-save-failure"
			if preset {
				name = "preset-save-failure"
				optionExitFocus(t, tt, a.copy().SurfaceResidency)
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyHome)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			residencyRefreshDump(t, tt, name, dark)
			if !a.SaveFailedVisible() || store.Snapshot().SurfaceResidencySeconds != 45 || store.Writes() != 1 {
				t.Fatal("fixture did not reject exactly one write")
			}
			if !a.residencyCustomOpen || a.residencyDraft != "30" {
				t.Errorf("%s discarded retry draft: open=%v draft=%q", name, a.residencyCustomOpen, a.residencyDraft)
			}
			if !tt.HasText(a.copy().SurfaceResidencyCustomApply) {
				t.Error("failed save removed retry controls")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if a.residencyCustomOpen {
				optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
				tt.Key(0, ui.KeyEnter)
				if store.Snapshot().SurfaceResidencySeconds != 30 || store.Writes() != 2 {
					t.Error("retained draft could not retry exactly once")
				}
			}
		})
	}
}

func TestResidencyFailurePresetChoice(t *testing.T) { checkResidencyFailure(t, true) }
func TestResidencyFailureCustomCommit(t *testing.T) { checkResidencyFailure(t, false) }
