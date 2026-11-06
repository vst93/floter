package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func refreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HN_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hn-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func refreshRead(tt *ui.Tester) string {
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyC)
	return tt.Clipboard()
}

func TestInputRefreshUnchangedTextEnterReleasesDraft(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "directory", Label: "工作目录", Kind: plugincfg.Text}
			_, sheet, tt := inputStateSheet(t, dark, field, "/workspace/initial")
			calls := 0
			sheet.Change = func(string, any) { calls++ }
			pluginIdentityFocus(t, tt, field.Label)
			tt.Type("/workspace/temporary")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/workspace/initial")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			sheet.Values[field.Key] = "/workspace/external"
			tt.Frame()
			refreshDump(t, tt, "text-refresh", dark)
			if got := refreshRead(tt); got != "/workspace/external" {
				t.Errorf("unchanged Enter masks external text: %q", got)
			}
			if calls != 0 {
				t.Errorf("unchanged Enter dispatched %d writes", calls)
			}
		})
	}
}

func TestInputRefreshAcceptedNumberFollowsHost(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "limit", Label: "结果上限", Kind: plugincfg.Number, Max: 100, Step: 1, Unit: "项"}
			_, sheet, tt := inputStateSheet(t, dark, field, 5.0)
			inputStateNumberFocus(t, tt, field.Label)
			tt.Type("25")
			tt.Frame()
			if sheet.Values[field.Key] != 25.0 {
				t.Fatal("initial edit was not accepted")
			}
			sheet.Values[field.Key] = 40.0
			tt.Frame()
			refreshDump(t, tt, "number-refresh", dark)
			if got := refreshRead(tt); got != "40" {
				t.Errorf("accepted number masks external value: %q", got)
			}
			tt.Type("1e")
			tt.Frame()
			sheet.Values[field.Key] = 50.0
			tt.Frame()
			if got := refreshRead(tt); got != "1e" {
				t.Errorf("external update discarded partial input: %q", got)
			}
			sheet.Change = func(string, any) { sheet.Failed = true }
			tt.Type("60")
			tt.Frame()
			sheet.Values[field.Key] = 55.0
			tt.Frame()
			if got := refreshRead(tt); got != "60" {
				t.Errorf("external update discarded refused input: %q", got)
			}
		})
	}
}

func TestInputRefreshSameNumberDoesNotWrite(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "limit", Label: "结果上限", Kind: plugincfg.Number, Max: 100, Step: 1, Unit: "项"}
			_, sheet, tt := inputStateSheet(t, dark, field, 25.0)
			calls := 0
			sheet.Change = func(string, any) { calls++; sheet.Failed = true }
			inputStateNumberFocus(t, tt, field.Label)
			tt.Type("25.0")
			tt.Frame()
			refreshDump(t, tt, "number-noop", dark)
			if calls != 0 {
				t.Errorf("same number dispatched %d writes", calls)
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("26")
			tt.Frame()
			if calls != 1 {
				t.Errorf("real number change dispatched %d writes, want 1", calls)
			}
		})
	}
}

func TestInputRefreshSameSliderStepDoesNotWrite(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "limit", Label: "结果上限", Kind: plugincfg.Slider, Max: 100, Step: 20, Unit: "项"}
			_, sheet, tt := inputStateSheet(t, dark, field, 40.0)
			var sent []any
			sheet.Change = func(_ string, value any) { sent = append(sent, value); sheet.Failed = true }
			r := mustFind(t, tt, "40 项")
			tt.ClickAt(280, r.Y+r.H/2)
			tt.Frame()
			refreshDump(t, tt, "slider-noop", dark)
			if len(sent) != 0 {
				t.Errorf("same slider step dispatched %v", sent)
			}
			// Accept the real move so its new host value can settle. Refusing
			// every drag frame intentionally leaves the host at the old step.
			sheet.Failed = false
			sheet.Change = func(key string, value any) { sent = append(sent, value); sheet.Values[key] = value }
			tt.ClickAt(520, r.Y+r.H/2)
			tt.Frame()
			if len(sent) != 1 || sent[0] == 40.0 {
				t.Errorf("real slider move did not write once: %v", sent)
			}
		})
	}
}
