package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func residencyRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IT_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-it-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func residencyRefreshFrame(a *App, dark bool) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		ui.Column(c).Padding(20).Children(func() { a.residencyRow(c, a.copy()) })
	}, 720, 400)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

func residencyOpenCustom(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	optionExitFocus(t, tt, a.copy().SurfaceResidency)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	if !a.residencyCustomOpen {
		t.Fatal("custom editor did not open")
	}
}

func TestResidencyRefreshInitialCustom(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 45 }); err != nil {
				t.Fatal(err)
			}
			tt := residencyRefreshFrame(a, dark)
			optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
			residencyRefreshDump(t, tt, "initial-custom", dark)
			if got := refreshRead(tt); got != "45" {
				t.Errorf("stored custom duration opened as %q, want 45", got)
			}
			tt.Key(0, ui.KeyEnter)
			if got := a.Store.Snapshot().SurfaceResidencySeconds; got != 45 {
				t.Errorf("unchanged custom Enter stored %d, want 45", got)
			}
		})
	}
}

func TestResidencyRefreshOpenDraftMenu(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tt := residencyRefreshFrame(a, dark)
			residencyOpenCustom(t, a, tt)
			optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("47")
			optionExitFocus(t, tt, a.copy().SurfaceResidency)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyHome)
			if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 53 }); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			residencyRefreshDump(t, tt, "draft-menu-refresh", dark)
			if tt.HasText(a.copy().SurfaceResidencyValue(20)) {
				t.Error("raw duration refresh left obsolete draft menu open")
			}
			tt.Key(0, ui.KeyEnter)
			if got := a.Store.Snapshot().SurfaceResidencySeconds; got != 53 {
				t.Errorf("stale draft menu overwrote host duration with %d", got)
			}
			if a.residencyDraft != "47" {
				t.Errorf("refresh discarded active draft: %q", a.residencyDraft)
			}
			if !tt.Focused(a.copy().SurfaceResidency) {
				t.Error("refresh lost custom selector keyboard outlet")
			}
		})
	}
}
