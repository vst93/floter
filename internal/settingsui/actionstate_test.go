package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

func actionStateDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13GZ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-gz-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSessionResumeMarkerBelongsToItsButton(t *testing.T) {
	for _, dark := range []bool{false, true} {
		show := false
		tt, _ := lensProbe(t, dark, 260, 120, func(c *ui.Context, a *App) {
			ui.Row(c).Gap(12).Children(func() {
				a.sessionIconButton(c, "play", "Resume", false, show, false, func() {})
				a.sessionIconButton(c, "trash", "Delete", true, false, false, func() {})
			})
		})
		plain := tt.Image()
		show = true
		tt.Frame()
		marked := tt.Image()
		button := mustFind(t, tt, "Resume")
		box := image.Rect(int(button.X), int(button.Y), int(button.X+button.W), int(button.Y+button.H))
		changed := 0
		outside := 0
		for y := 0; y < marked.Bounds().Dy(); y++ {
			for x := 0; x < marked.Bounds().Dx(); x++ {
				if plain.RGBAAt(x, y) != marked.RGBAAt(x, y) {
					changed++
					if !image.Pt(x, y).In(box) {
						outside++
					}
				}
			}
		}
		actionStateDump(t, tt, "resume", dark)
		if changed == 0 || outside != 0 {
			t.Errorf("%s: resume marker changed %d pixels, %d outside its button %v", appearanceName(dark), changed, outside, box)
		}
	}
}

func TestSessionReadOnlyTitleDoesNotInviteActivation(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, state := range []struct {
			name                     string
			running, active, handler bool
		}{
			{"active", true, true, true}, {"exited", false, false, true},
			{"no-handler", true, false, false}, {"resumable", true, false, true},
		} {
			t.Run(appearanceName(dark)+"/"+state.name, func(t *testing.T) {
				calls := 0
				a := New(newStore(t), Actions{})
				if state.handler {
					a.Actions.SelectSession = func(int) { calls++ }
				}
				a.Sessions = func() []Session {
					return []Session{{ID: 7, Title: "State session", Running: state.running, Active: state.active}}
				}
				tt, _ := renderIn(t, a, 720, 560, dark)
				if err := tt.Click(i18n.For("en").Settings.PageSessions); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				label := mustFind(t, tt, "State session")
				rest := rowInsetFace(tt.Image(), label)
				settlePointer(tt, label)
				hover := rowInsetFace(tt.Image(), label)
				available := state.running && !state.active && state.handler
				if (hover != rest) != available {
					t.Errorf("read-only title feedback: available=%v rest=%v hover=%v", available, rest, hover)
				}
				if err := tt.Click("State session"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if (calls == 1) != available {
					t.Errorf("activation calls=%d, available=%v", calls, available)
				}
				if state.name == "active" {
					actionStateDump(t, tt, "session", dark)
				}
			})
		}
	}
}
