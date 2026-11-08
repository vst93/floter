package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func checkOrphanBusy(t *testing.T, deleting bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a := newTestApp()
			a.Page = PageIntegrations
			a.Integrations = func() []Integration { return []Integration{{ID: "orphan", Name: "待接管工具", Orphan: true}} }
			adopts, deletes := 0, 0
			a.Actions.AdoptIntegration = func(id string) { adopts++; a.BeginDiagnosis(id) }
			a.Actions.DeleteOrphan = func(string) { deletes++ }
			tt := ui.NewTester(a.View, 760, 700)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			copy := i18n.For("en").Settings
			if err := tt.Click(copy.IntegrationsAdopt); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			label, scenario := copy.IntegrationsAdopt, "repeat"
			if deleting {
				label, scenario = copy.IntegrationsDeleteOrphan, "delete"
			}
			if err := tt.Click(label); err == nil {
				tt.Frame()
			}
			if tag := os.Getenv("FLOTER_P13JV_DUMP"); tag != "" {
				file, err := os.Create("/tmp/floter-P13-jv-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
				file.Close()
			}
			if adopts != 1 || deletes != 0 {
				t.Errorf("pending orphan permits %s: adopts=%d deletes=%d", scenario, adopts, deletes)
			}
			a.FinishDiagnosis("orphan")
			tt.Frame()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if deleting && deletes != 1 || !deleting && adopts != 2 {
				t.Errorf("settled orphan lost %s retry: adopts=%d deletes=%d", scenario, adopts, deletes)
			}
		})
	}
}

func TestOrphanBusyRepeatedAdoption(t *testing.T) { checkOrphanBusy(t, false) }
func TestOrphanBusyCompetingDelete(t *testing.T)  { checkOrphanBusy(t, true) }
