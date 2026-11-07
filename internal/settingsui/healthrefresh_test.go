package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func healthRefreshUI(t *testing.T, dark bool) (*App, *ui.Tester) {
	t.Helper()
	a := newTestApp()
	a.Integrations = func() []Integration {
		return []Integration{{ID: "health", Name: "Build Tools", Enabled: true, Broken: true,
			RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing,
			Health: "Degraded", HealthReport: testHealthReport()}}
	}
	a.Page = PageIntegrations
	tt := ui.NewTester(a.View, 960, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	return a, tt
}

func dumpHealthRefreshUI(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JR_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jr-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHealthRefreshRejectsRepeatedPress(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, tt := healthRefreshUI(t, dark)
			calls := 0
			a.Actions.ReprobeHealth = func(id string) {
				if a.BeginHealthReprobe(id) {
					calls++
				}
			}
			tt.Frame()
			tt.Frame()
			label := i18n.For("en").Settings.IntegrationsHealthReprobe
			before, ok := tt.Find(label)
			if !ok {
				t.Fatal("refresh entry is missing")
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if err := tt.Click(label); err == nil {
				tt.Frame()
			}
			dumpHealthRefreshUI(t, tt, "repeat", dark)
			if calls != 1 {
				t.Errorf("health refresh starts twice while pending: %d", calls)
			}
			if a.BeginHealthReprobe("health") {
				t.Error("host accepts a duplicate pending health refresh")
			}
			after, ok := tt.Find(label)
			if !ok || after != before {
				t.Errorf("pending refresh changes its geometry: before=%v after=%v", before, after)
			}
			a.FinishHealthReprobe("health")
			tt.Frame()
			tt.Frame()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 {
				t.Error("settled refresh cannot run again")
			}
			a.FinishHealthReprobe("health")
		})
	}
}

func TestHealthRefreshLocksCompetingActions(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, tt := healthRefreshUI(t, dark)
			repairs, checks := 0, 0
			a.Actions.ReprobeHealth = func(id string) { a.BeginHealthReprobe(id) }
			a.Actions.RepairIntegration = func(string, func(error)) { repairs++ }
			a.Actions.DiagnoseIntegration = func(string) { checks++ }
			tt.Frame()
			tt.Frame()
			copy := i18n.For("en").Settings
			if err := tt.Click(copy.IntegrationsHealthReprobe); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			for _, label := range []string{copy.IntegrationsRecheck, copy.IntegrationsCheck} {
				if err := tt.Click(label); err == nil {
					tt.Frame()
				}
			}
			dumpHealthRefreshUI(t, tt, "competing", dark)
			if repairs != 0 || checks != 0 {
				t.Errorf("health refresh permits competing row actions: repairs=%d checks=%d", repairs, checks)
			}
			if !a.BeginHealthReprobe("other") {
				t.Error("health refresh blocks an unrelated integration")
			}
			a.FinishHealthReprobe("other")
			a.FinishHealthReprobe("health")
			tt.Frame()
			tt.Frame()
			if err := tt.Click(copy.IntegrationsRecheck); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if repairs != 1 {
				t.Error("settled health refresh leaves re-check locked")
			}
		})
	}
}
