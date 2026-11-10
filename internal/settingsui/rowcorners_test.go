package settingsui

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestSettingsMiddleRowStatesKeepRoundedCorners(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, state := range []string{"hover", "pressed", "selected"} {
			a := newTestApp()
			selected := false
			tt := ui.NewTester(func(c *ui.Context) {
				tokens := a.tokens(c)
				c.SetTheme(tokens.Theme)
				c.Root().Background(tokens.Theme.Background)
				ui.Column(c).Absolute().Top(20).Left(20).Width(300).Children(func() {
					a.card(c, func() {
						for i := range 3 {
							face := rowFace{}
							if i == 1 && selected {
								face.FaceSet = true
								face.Face = tokens.RowSelected
								face.Bar = tokens.Theme.Accent
							}
							a.cardRow(c, ui.Center, nil, face, func() {}, func() { ui.Text(c, "Row") })
						}
					})
				})
			}, 340, 240)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Move(0, 0)
			tt.Frame()
			corner := tt.Image().RGBAAt(20, 82)
			center := tt.Image().RGBAAt(300, 110)
			switch state {
			case "hover":
				tt.Move(300, 110)
			case "pressed":
				tt.Press(22, 110)
			case "selected":
				selected = true
			}
			tt.Frame()
			if got := tt.Image().RGBAAt(20, 82); got != corner {
				t.Errorf("dark=%v %s middle-row corner painted outside its curve: %v -> %v", dark, state, corner, got)
			}
			if got := tt.Image().RGBAAt(300, 110); state == "hover" && got != center {
				t.Errorf("dark=%v hover changed the row background: %v -> %v", dark, center, got)
			} else if state != "hover" && got == center {
				t.Errorf("dark=%v %s lost its state background", dark, state)
			}
		}
	}
}
