package shell

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"floter/internal/launcher"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestSystemThemeUpdatesLiveSurface(t *testing.T) {
	for _, surface := range []Surface{SurfaceLauncher, SurfaceSettings, SurfaceTerminal} {
		t.Run(fmt.Sprint(surface), func(t *testing.T) {
			s := settings.Default()
			s.Theme = "auto"
			a := New(Options{Store: settings.NewStore(s)})
			a.Surf = surface
			tt := ui.NewTester(a.View, 720, 580)
			tt.Frame()
			tt.Advance(time.Second)
			for _, dark := range []bool{true, false, true} {
				tt.SetDark(dark)
				tt.Frame()
				fresh := New(Options{Store: settings.NewStore(s)})
				fresh.Surf = surface
				ref := ui.NewTester(fresh.View, 720, 580)
				ref.SetDark(dark)
				ref.Frame()
				ref.Advance(time.Second)
				if !bytes.Equal(tt.Image().Pix, ref.Image().Pix) {
					t.Errorf("system dark=%t retained stale colors", dark)
				}
			}
		})
	}
}

func TestOutputWindowsSwitchThemeImmediately(t *testing.T) {
	for _, kind := range []string{"pinned", "detached"} {
		t.Run(kind, func(t *testing.T) {
			s := settings.Default()
			s.Theme = "light"
			a := New(Options{Store: settings.NewStore(s)})
			view := func(a *App) func(*ui.Context) {
				if kind == "pinned" {
					state := a.newPinned("Note", "Output text")
					return func(c *ui.Context) { a.pinnedView(c, state) }
				}
				state := &detachedOutput{title: "Output", view: launcher.OutputView{Text: "Output text"}}
				return func(c *ui.Context) { a.detachedView(c, state) }
			}
			tt := ui.NewTester(view(a), 720, 460)
			tt.Frame()
			tt.Advance(time.Second)
			for _, appearance := range []string{"dark", "light"} {
				if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				fresh := New(Options{Store: settings.NewStore(a.Store.Snapshot())})
				ref := ui.NewTester(view(fresh), 720, 460)
				ref.Frame()
				ref.Advance(time.Second)
				if !bytes.Equal(tt.Image().Pix, ref.Image().Pix) {
					t.Errorf("%s retained stale colors", appearance)
				}
			}
		})
	}
}
