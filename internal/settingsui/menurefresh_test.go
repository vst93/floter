package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func menuRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IR_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ir-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkMenuRefresh(t *testing.T, component, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{})
			copy := i18n.For("zh").Settings
			options := []i18n.Option{{ID: "plugin:clipboard", Label: "剪贴板"}, {ID: "plugin:browser", Label: "浏览器"}, {ID: "plugin:calculator", Label: "计算器"}, {ID: commandAction, Label: "命令行"}}
			if component == "palette" {
				options = []i18n.Option{{ID: "inherit", Label: "跟随外观"}, {ID: "contrast", Label: "高对比度"}, {ID: "dracula", Label: "德古拉"}}
			}
			if scenario == "duplicate" {
				options[1].Label = options[0].Label
			}
			current, writes := options[0].ID, 0
			label := copy.ShortcutsAction
			if component == "palette" {
				label = copy.Palette
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Padding(20).Children(func() {
					apply := func(next string) { current = next; writes++ }
					if component == "palette" {
						copy.Palettes = options
						a.paletteRow(c, copy, current, apply)
					} else {
						copy.ShortcutsActions = options
						a.customActionPicker(c, copy, current, apply)
					}
				})
			}, 720, 400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			optionExitFocus(t, tt, label)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyDown)
			want := options[1].ID
			switch scenario {
			case "external":
				current, want = options[2].ID, options[2].ID
			case "reorder":
				options[0], options[1] = options[1], options[0]
			}
			tt.Frame()
			tt.Frame()
			menuRefreshDump(t, tt, component+"-"+scenario, dark)
			if writes != 0 {
				t.Fatalf("refresh wrote %d choices", writes)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if current != want {
				t.Errorf("%s %s selected %q, want %q", component, scenario, current, want)
			}
			wantWrites := 1
			if scenario == "external" {
				wantWrites = 0
			}
			if writes != wantWrites {
				t.Errorf("%s %s wrote %d choices, want %d", component, scenario, writes, wantWrites)
			}
			if !tt.Focused(label) {
				t.Error("choice lost the keyboard trigger")
			}
		})
	}
}

func TestCustomMenuExternalSelection(t *testing.T) { checkMenuRefresh(t, "custom", "external") }
func TestCustomMenuReorder(t *testing.T)           { checkMenuRefresh(t, "custom", "reorder") }

func TestPaletteMenuExternalSelection(t *testing.T) { checkMenuRefresh(t, "palette", "external") }
func TestPaletteMenuReorder(t *testing.T)           { checkMenuRefresh(t, "palette", "reorder") }

func TestCustomMenuDuplicateLabels(t *testing.T)  { checkMenuRefresh(t, "custom", "duplicate") }
func TestPaletteMenuDuplicateLabels(t *testing.T) { checkMenuRefresh(t, "palette", "duplicate") }

func TestParameterMenuExternalSelection(t *testing.T) { checkFixedMenuExternal(t, false) }
func TestResidencyMenuExternalSelection(t *testing.T) { checkFixedMenuExternal(t, true) }

func checkFixedMenuExternal(t *testing.T, residency bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{})
			copy := i18n.For("zh").Settings
			current := "文本"
			label, name := copy.ParamsKind, "param-external"
			if residency {
				label, name = copy.SurfaceResidency, "residency-external"
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Padding(20).Children(func() {
					if residency {
						a.residencyRow(c, copy)
					} else {
						a.paramSelect(c, &current, []string{"文本", "数字", "开关"}, label)
					}
				})
			}, 720, 400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			optionExitFocus(t, tt, label)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyDown)
			current = "开关"
			if residency {
				if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 120 }); err != nil {
					t.Fatal(err)
				}
			}
			tt.Frame()
			tt.Frame()
			menuRefreshDump(t, tt, name, dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if residency {
				if got := a.Store.Snapshot().SurfaceResidencySeconds; got != 120 {
					t.Errorf("open residency menu overwrote external 120 with %d", got)
				}
			} else if current != "开关" {
				t.Errorf("open parameter menu overwrote external kind with %q", current)
			}
			if !tt.Focused(label) {
				t.Error("external change lost trigger focus")
			}
		})
	}
}
