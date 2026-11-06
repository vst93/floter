package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func lifetimeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HO_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ho-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRevertedConfig(t *testing.T, kind, original, edit, external string, initial, next any) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "value", Label: "配置值", Type: kind}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}, ConfigValues: map[string]any{"value": initial}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, field.Label)
			tt.Type(edit)
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type(original)
			tt.Frame()
			if a.configDirty(integration, a.draft(integration)) {
				t.Fatal("reverted edit is still dirty")
			}
			integration.ConfigValues[field.Key] = next
			tt.Frame()
			lifetimeDump(t, tt, "reverted-"+kind, dark)
			if got := refreshRead(tt); got != external {
				t.Errorf("reverted %s masks host update: %q, want %q", kind, got, external)
			}
			if !tt.Focused(field.Label) {
				t.Error("host update stole field focus")
			}
			tt.Type(edit)
			tt.Frame()
			integration.ConfigValues[field.Key] = initial
			tt.Frame()
			if got := refreshRead(tt); got != edit {
				t.Errorf("real edit was discarded: %q", got)
			}
		})
	}
}

func TestDraftLifetimeRevertedTextFollowsHost(t *testing.T) {
	checkRevertedConfig(t, "text", "/workspace/initial", "/workspace/local", "/workspace/external", "/workspace/initial", "/workspace/external")
}

func TestDraftLifetimeRevertedNumberFollowsHost(t *testing.T) {
	checkRevertedConfig(t, "number", "5.0", "12", "40", 5.0, 40.0)
}

func TestDraftLifetimeRemovedNumberRetiresPartialInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "limit", Label: "结果上限", Type: "number"}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}, ConfigValues: map[string]any{"limit": 5.0}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("1e")
			tt.Frame()
			integration.Config = nil
			tt.Frame()
			integration.Config = []ConfigField{field}
			tt.Frame()
			lifetimeDump(t, tt, "removed-number", dark)
			configIdentityFocus(t, tt, field.Label)
			if got := refreshRead(tt); got != "" {
				t.Errorf("removed number resurrected partial text: %q", got)
			}
			if tt.HasText(a.copy().ConfigNumber) {
				t.Error("removed number resurrected old parse error")
			}
			tt.Type("8")
			tt.Frame()
			if a.draft(integration).values[field.Key] != 8.0 {
				t.Error("readded field cannot be edited")
			}
		})
	}
}

func TestDraftLifetimeHostAcknowledgesMatchingEdit(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "directory", Label: "工作目录", Type: "text"}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}, ConfigValues: map[string]any{"directory": "/workspace/old"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("/workspace/shared")
			tt.Frame()
			integration.ConfigValues[field.Key] = "/workspace/shared"
			tt.Frame()
			lifetimeDump(t, tt, "host-ack", dark)
			if a.configDirty(integration, a.draft(integration)) || tt.HasText(a.copy().ConfigUnsaved) {
				t.Error("host-confirmed edit still says Unsaved changes")
			}
			integration.ConfigValues[field.Key] = "/workspace/latest"
			tt.Frame()
			if got := refreshRead(tt); got != "/workspace/latest" {
				t.Errorf("host-confirmed edit masks next generation: %q", got)
			}
		})
	}
}

func TestDraftLifetimeRemovedListStartsAtTop(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "sources", Label: "搜索来源", Type: "multiselect"}
			for i := 0; i < 12; i++ {
				field.Options = append(field.Options, fmt.Sprintf("来源 %02d", i))
			}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}}
			a, tt := configIdentityFrame(t, dark, &integration)
			state := a.configListScroll(integration.ID, field.Key)
			state.Y = state.MaxY
			tt.Frame()
			if state.Y <= 0 {
				t.Fatal("list did not scroll")
			}
			integration.Config = nil
			tt.Frame()
			integration.Config = []ConfigField{field}
			tt.Frame()
			lifetimeDump(t, tt, "removed-list", dark)
			if got := a.configListScroll(integration.ID, field.Key).Y; got != 0 {
				t.Errorf("readded list starts at old offset %g", got)
			}
		})
	}
}

func TestDraftLifetimeNormalizedTextDoesNotWrite(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "directory", Label: "工作目录", Kind: plugincfg.Text}
			_, sheet, tt := inputStateSheet(t, dark, field, "/workspace/current")
			calls := 0
			sheet.Change = func(string, any) { calls++; sheet.Failed = true }
			pluginIdentityFocus(t, tt, field.Label)
			tt.Type("  /workspace/current  ")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			lifetimeDump(t, tt, "normalized-text", dark)
			if calls != 0 || sheet.Failed {
				t.Errorf("normalized same text wrote %d times and failed=%v", calls, sheet.Failed)
			}
			if got := refreshRead(tt); got != "/workspace/current" {
				t.Errorf("confirmed text keeps redundant whitespace: %q", got)
			}
			tt.Type(" /workspace/new ")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if calls != 1 {
				t.Errorf("different text dispatched %d writes, want 1", calls)
			}
			if got := refreshRead(tt); got != " /workspace/new " {
				t.Errorf("failed real edit was discarded: %q", got)
			}
		})
	}
}
