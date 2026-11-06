package settingsui

import (
	"image/png"
	"math"
	"os"
	"strconv"
	"testing"

	"floter/internal/plugincfg"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func conflictDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HP_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hp-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPluginReopenHistory(t *testing.T, kind plugincfg.Kind, initial any, edit, external string, next any) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			s := settings.Default()
			s.Theme, s.Language = appearanceName(dark), "zh"
			a := New(settings.NewStore(s), Actions{})
			field := plugincfg.Field{Key: "value", Label: "配置值", Kind: kind}
			sheet := PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.reopen", Title: "配置", Fields: []plugincfg.Field{field}}, Values: plugincfg.Values{"value": initial}}
			calls := 0
			sheet.Change = func(key string, value any) { calls++; sheet.Values[key] = value }
			open := true
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				if open {
					a.PluginSheet(c, sheet)
				}
			}, 720, 220)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			focus := pluginIdentityFocus
			name := "text-reopen"
			if kind == plugincfg.Number {
				focus = inputStateNumberFocus
				name = "number-reopen"
			}
			focus(t, tt, field.Label)
			tt.Type(edit)
			tt.Frame()
			open = false
			a.ClosePluginSheet()
			tt.Frame()
			tt.Frame()
			sheet.Values[field.Key] = next
			calls = 0
			open = true
			tt.Frame()
			focus(t, tt, field.Label)
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Frame()
			conflictDump(t, tt, name, dark)
			if got := refreshRead(tt); got != external {
				t.Errorf("reopened %s resurrected old undo: %q, want %q", name, got, external)
			}
			if calls != 0 {
				t.Errorf("reopened undo rewrote host %d times", calls)
			}
		})
	}
}

func TestConflictRecoveryPluginTextReopenRetiresUndo(t *testing.T) {
	checkPluginReopenHistory(t, plugincfg.Text, "/old", "/abandoned", "/external", "/external")
}

func TestConflictRecoveryPluginNumberReopenRetiresUndo(t *testing.T) {
	checkPluginReopenHistory(t, plugincfg.Number, 5.0, "1e", "40", 40.0)
}

func TestConflictRecoveryIntegrationExtremeNumberReadable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "limit", Label: "结果上限", Type: "number"}
			in := Integration{ID: "local.extreme", Config: []ConfigField{field}, ConfigValues: map[string]any{field.Key: 1e308}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			conflictDump(t, tt, "integration-extreme", dark)
			if got := refreshRead(tt); got != "1e+308" {
				t.Errorf("integration number expands to %d characters, want 1e+308", len(got))
			}
			if a.configDirty(in, a.draft(in)) || in.ConfigValues[field.Key] != 1e308 {
				t.Error("display changed stored value")
			}
		})
	}
}

func TestConflictRecoveryPluginTinyNumberReadable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "ratio", Label: "换算系数", Kind: plugincfg.Number}
			_, sheet, tt := inputStateSheet(t, dark, field, 1e-308)
			calls := 0
			sheet.Change = func(string, any) { calls++ }
			inputStateNumberFocus(t, tt, field.Label)
			conflictDump(t, tt, "plugin-tiny", dark)
			if got := refreshRead(tt); got != "1e-308" {
				t.Errorf("tiny number expands to %d characters, want 1e-308", len(got))
			}
			for _, value := range []float64{math.SmallestNonzeroFloat64, -1e-308, 1e-7, 1e-6, 0, -1e308} {
				sheet.Values[field.Key] = value
				tt.Frame()
				got, err := strconv.ParseFloat(refreshRead(tt), 64)
				if err != nil || got != value {
					t.Errorf("number round trip = %g, %v, want %g", got, err, value)
				}
			}
			if calls != 0 {
				t.Errorf("display wrote %d times", calls)
			}
		})
	}
}

func TestConflictRecoveryPluginWithdrawnTextFollowsHost(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "directory", Label: "工作目录", Kind: plugincfg.Text}
			_, sheet, tt := inputStateSheet(t, dark, field, "/old")
			calls := 0
			sheet.Change = func(string, any) { calls++ }
			pluginIdentityFocus(t, tt, field.Label)
			tt.Type("/local")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/old")
			tt.Frame()
			sheet.Values[field.Key] = "/external"
			tt.Frame()
			conflictDump(t, tt, "plugin-withdrawn", dark)
			if got := refreshRead(tt); got != "/external" {
				t.Errorf("withdrawn plugin text masks host: %q", got)
			}
			if calls != 0 || !tt.Focused(field.Label) {
				t.Error("refresh wrote or stole focus")
			}
		})
	}
}

func TestConflictRecoveryPluginMatchingHostReleasesText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "directory", Label: "工作目录", Kind: plugincfg.Text}
			_, sheet, tt := inputStateSheet(t, dark, field, "/old")
			pluginIdentityFocus(t, tt, field.Label)
			tt.Type("/shared")
			tt.Frame()
			sheet.Values[field.Key] = "/shared"
			tt.Frame()
			sheet.Values[field.Key] = "/latest"
			tt.Frame()
			conflictDump(t, tt, "plugin-confirmed", dark)
			if got := refreshRead(tt); got != "/latest" {
				t.Errorf("host-confirmed plugin text masks next value: %q", got)
			}
			tt.Type("/unsent")
			tt.Frame()
			sheet.Values[field.Key] = "/other"
			tt.Frame()
			if got := refreshRead(tt); got != "/unsent" {
				t.Errorf("conflict discarded plugin text: %q", got)
			}
		})
	}
}

func TestConflictRecoveryTextUsesCurrentSavedValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "directory", Label: "工作目录", Type: "path"}
			in := Integration{ID: "local.conflict", Config: []ConfigField{field}, ConfigValues: map[string]any{field.Key: "/old"}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("/local")
			tt.Frame()
			in.ConfigValues[field.Key] = "/external"
			tt.Frame()
			if got := refreshRead(tt); got != "/local" {
				t.Fatalf("conflict lost real edit: %q", got)
			}
			tt.Type("/external")
			tt.Frame()
			conflictDump(t, tt, "text-current", dark)
			if a.configDirty(in, a.draft(in)) {
				t.Error("returning to external text still says Unsaved changes")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/old")
			tt.Frame()
			if !a.configDirty(in, a.draft(in)) {
				t.Error("obsolete original text falsely says Saved")
			}
		})
	}
}

func TestConflictRecoveryNumberDefaultUsesCurrentSavedValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "limit", Label: "结果上限", Type: "number", Default: 5.0}
			in := Integration{ID: "local.conflict", Config: []ConfigField{field}, ConfigValues: map[string]any{field.Key: 5.0}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("1e")
			tt.Frame()
			in.ConfigValues[field.Key] = 40.0
			tt.Frame()
			if got := refreshRead(tt); got != "1e" {
				t.Fatalf("conflict lost partial number: %q", got)
			}
			if err := tt.Click(a.copy().ConfigDefaults); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			conflictDump(t, tt, "number-default", dark)
			if !a.configDirty(in, a.draft(in)) {
				t.Error("restoring obsolete default falsely says Saved")
			}
			configIdentityFocus(t, tt, field.Label)
			tt.Type("40.0")
			tt.Frame()
			if a.configDirty(in, a.draft(in)) {
				t.Error("returning to external number still says Unsaved changes")
			}
		})
	}
}
