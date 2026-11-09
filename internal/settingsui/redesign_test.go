package settingsui

import (
	"strings"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestComfortableControlsKeepTheirHitAreas(t *testing.T) {
	for _, scale := range []string{"tiny", "small", "default", "large"} {
		t.Run(scale, func(t *testing.T) {
			s := settings.Default()
			s.UIScale = scale
			a := &App{Store: settings.NewStore(s)}
			value, selected, clicks := "", 0, 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
					if a.actionButton(c, "Apply").Label("Apply action").Clicked() {
						clicks++
					}
					a.textInput(c, &value).Label("Name").FillWidth()
					a.segmentedControl(c, &selected, "One", "Two")
				})
			}, 340, 240)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			for _, label := range []string{"Apply action", "Name", "One", "Two"} {
				box, ok := tt.Find(label)
				if !ok || box.H < 35.99 || box.X < 0 || box.X+box.W > 340 {
					t.Fatalf("%q has an unusable target: %+v", label, box)
				}
			}
			if err := tt.Click("Apply action"); err != nil {
				t.Fatal(err)
			}
			if clicks != 1 {
				t.Fatalf("action called %d times", clicks)
			}
			if err := tt.Click("Two"); err != nil {
				t.Fatal(err)
			}
			if selected != 1 {
				t.Fatal("segment did not change")
			}
			if err := tt.Click("Name"); err != nil {
				t.Fatal(err)
			}
			tt.Type("Workspace")
			if value != "Workspace" {
				t.Fatalf("input did not accept text: %q", value)
			}
		})
	}
}

// A long stored font family must not displace the row label or leave the card.
func TestLongPickerStaysInsideComfortableRow(t *testing.T) {
	for _, scale := range []string{"tiny", "small", "default", "large"} {
		t.Run(scale, func(t *testing.T) {
			s := settings.Default()
			s.UIScale = scale
			a := &App{Store: settings.NewStore(s)}
			family := "Iosevka " + strings.Repeat("Extended ", 12)
			var picker ui.Handle
			var rect ui.Rect
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				ui.Column(c).FillWidth().Padding(16).Children(func() {
					a.card(c, func() {
						a.row(c, "Font family", "", func() {
							a.chooseSelect(c, &family, []string{family, "Monospace"}, "Font picker").Bind(&picker)
						})
					})
				})
				rect = c.Resolve(picker).Bounds()
			}, 360, 150)
			tt.Frame()
			label, ok := tt.Find("Font family")
			if !ok || label.W < 40 || rect.H < 35.99 || rect.X+rect.W > 344.01 || rect.X < label.X+label.W {
				t.Fatalf("long picker escaped its row: label=%+v picker=%+v", label, rect)
			}
			if err := tt.Click("Font picker"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("Monospace") {
				t.Fatal("picker did not open")
			}
		})
	}
}
