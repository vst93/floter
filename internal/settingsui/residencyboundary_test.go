package settingsui

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestResidencyBoundaryNumericCommit(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			for _, tc := range []struct {
				text string
				want uint32
			}{{"4294967296", 86400}, {"-5", 0}, {"45.6", 46}, {"1e2", 100}, {"4294967295", settings.SurfaceResidencyNever}, {"NaN", 10}, {"Inf", 10}} {
				t.Run(tc.text, func(t *testing.T) {
					a := copyLifecycleApp(t, dark)
					tt := residencyRefreshFrame(a, dark)
					residencyOpenCustom(t, a, tt)
					optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type(tc.text)
					tt.Key(0, ui.KeyEnter)
					tt.Frame()
					if tc.text == "4294967296" {
						residencyRefreshDump(t, tt, "numeric-boundary", dark)
					}
					if got := a.Store.Snapshot().SurfaceResidencySeconds; got != tc.want {
						t.Errorf("typed %s stored %d, want reference-normalized %d", tc.text, got, tc.want)
					}
				})
			}
		})
	}
}

func TestResidencyBoundaryClosedHostCustom(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tt := residencyRefreshFrame(a, dark)
			if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 53 }); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			// Explicitly ask for the already displayed Custom value. It must
			// expose its editable duration, just like reopening the page.
			optionExitFocus(t, tt, a.copy().SurfaceResidency)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			residencyRefreshDump(t, tt, "host-custom-editor", dark)
			if !a.residencyCustomOpen {
				t.Error("host custom value cannot open its editor by selecting Custom")
			} else {
				optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
				if got := refreshRead(tt); got != "53" {
					t.Errorf("host custom field reads %q", got)
				}
			}
			if got := a.Store.Snapshot().SurfaceResidencySeconds; got != 53 {
				t.Errorf("opening host value wrote %d", got)
			}
		})
	}
}
