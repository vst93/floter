package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

func quietConfigGrid(t *testing.T, dark bool, language, state string) (*ui.Tester, []ConfigField) {
	t.Helper()
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), language
	a := New(settings.NewStore(s), Actions{})
	labels := []string{"Endpoint", "Access token", "Working directory", "Result limit", "Notifications", "Mode", "Output formats"}
	if language == "zh" {
		labels = []string{"服务地址", "访问令牌", "工作目录", "结果上限", "通知", "模式", "输出格式"}
	}
	kinds := []string{"text", "password", "path", "number", "boolean", "select", "multiselect"}
	fields := make([]ConfigField, len(kinds))
	for i, kind := range kinds {
		fields[i] = ConfigField{Key: kind, Label: labels[i], Type: kind}
	}
	fields[5].Options = []string{"quiet", "verbose"}
	fields[6].Options = []string{"json", "text", "markdown"}
	integration := Integration{
		ID: "local.search", Config: fields,
		ConfigValues: map[string]any{
			"text": "https://localhost:8080", "password": "local-token", "path": "/workspace",
			"number": float64(100), "boolean": true, "select": "quiet", "multiselect": []any{"text"},
		},
	}
	a.configOpen = map[string]bool{integration.ID: true}
	if state == "saving" {
		a.configSaveBusy = map[string]bool{integration.ID: true}
	}
	if state == "locked" {
		a.toggleBusy = map[string]bool{integration.ID: true}
	}
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).FillWidth().Padding(20).Children(func() {
			a.extensionMetrics(func() {
				a.card(c, func() {
					ui.Column(c).FillWidth().Padding(12).Children(func() {
						a.configFields(c, i18n.For(language).Settings, integration)
					})
				})
			})
		})
	}, 720, 560)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt, fields
}

// A field owns its control feedback. The transparent label/control grid must
// never manufacture a second, row-wide hover face, including while saving.
func TestTheConfigGridStaysQuietAroundItsControls(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, language := range []string{"en", "zh"} {
			for _, state := range []string{"idle", "saving", "locked"} {
				tt, fields := quietConfigGrid(t, dark, language, state)
				for _, field := range fields {
					label := mustFind(t, tt, field.Label)
					// Literal extension grid: 118px label + 12px gap. Read the
					// middle of that gap, inside the grid and outside the control.
					x, y := int(label.X+124), int(label.Y+label.H/2)
					tt.Move(0, 0)
					tt.Frame()
					tt.Advance(300 * time.Millisecond)
					tt.Frame()
					rest := imagePixel(tt.Image(), x, y)
					settlePointer(tt, label)
					hover := imagePixel(tt.Image(), x, y)
					tt.Press(label.X+1, label.Y+label.H/2)
					tt.Frame()
					tt.Advance(300 * time.Millisecond)
					tt.Frame()
					pressed := imagePixel(tt.Image(), x, y)
					if hover != rest || pressed != rest {
						t.Errorf("%s/%s/%s/%s: grid changed face: rest=%v hover=%v pressed=%v",
							appearanceName(dark), language, state, field.Type, rest, hover, pressed)
					}
					if got := mustFind(t, tt, field.Label); got != label {
						t.Errorf("%s: label/control geometry moved: %v -> %v", field.Type, label, got)
					}
					tt.Release(label.X+1, label.Y+label.H/2)
					tt.Frame()
					t.Logf("%s/%s/%s/%s: label=%v gap(%d,%d)=%v/%v/%v",
						appearanceName(dark), language, state, field.Type, label, x, y, rest, hover, pressed)
				}
			}
		}
	}
}

func TestDumpP13GpConfigGrid(t *testing.T) {
	tag := os.Getenv("FLOTER_P13GP_DUMP")
	if tag == "" {
		t.Skip("opt-in screenshot dump: FLOTER_P13GP_DUMP=before or after")
	}
	for _, dark := range []bool{false, true} {
		for _, state := range []string{"saving", "locked"} {
			tt, fields := quietConfigGrid(t, dark, "zh", state)
			settlePointer(tt, mustFind(t, tt, fields[0].Label))
			name := "/tmp/floter-P13-gp-settings-" + appearanceName(dark) + "-" + state + "-" + tag + ".png"
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			closeErr := f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			t.Log(name)
		}
	}
}
