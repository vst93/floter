package settingsui

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestResidencyRefreshAfterCommittedPreset(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tt := residencyRefreshFrame(a, dark)
			residencyOpenCustom(t, a, tt)
			optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("30")
			tt.Key(0, ui.KeyEnter)
			if a.residencyCustomOpen || a.Store.Snapshot().SurfaceResidencySeconds != 30 {
				t.Fatal("preset submission did not close the custom editor")
			}
			for _, value := range []uint32{45, 120} {
				if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = value }); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
			}
			actionWriteDump(t, tt, "residency-pristine", dark)
			if a.residencyCustomOpen || !tt.HasText(a.copy().SurfaceResidencyValue(120)) {
				t.Error("committed preset left stale draft ownership on a later host refresh")
			}
		})
	}
}
