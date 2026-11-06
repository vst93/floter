package settingsui

import "testing"

func TestConfigRefreshRemovedValueUsesCurrentDefault(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, fallback := range []string{"", "/workspace/default"} {
			t.Run(appearanceName(dark)+fallback, func(t *testing.T) {
				field := ConfigField{Key: "directory", Label: "工作目录", Type: "path"}
				if fallback != "" {
					field.Default = fallback
				}
				integration := Integration{ID: "local.refresh", Config: []ConfigField{field}, ConfigValues: map[string]any{field.Key: "/workspace/old"}}
				a, tt := configIdentityFrame(t, dark, &integration)
				delete(integration.ConfigValues, field.Key)
				tt.Frame()
				configIdentityFocus(t, tt, field.Label)
				name := "config-removed"
				if fallback != "" {
					name += "-default"
				}
				refreshDump(t, tt, name, dark)
				if got := refreshRead(tt); got != fallback {
					t.Errorf("removed host value remains visible: %q, want %q", got, fallback)
				}
				if a.configDirty(integration, a.draft(integration)) {
					t.Error("external removal marked untouched field dirty")
				}
			})
		}
	}
}

func TestConfigRefreshUpdatesOnlyUntouchedValues(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.refresh", Config: []ConfigField{
				{Key: "directory", Label: "工作目录", Type: "path"},
				{Key: "limit", Label: "结果上限", Type: "number"},
			}, ConfigValues: map[string]any{"directory": "/workspace/old", "limit": 5.0}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "结果上限")
			tt.Type("1e")
			tt.Frame()
			integration.ConfigValues["directory"] = "/workspace/external"
			integration.ConfigValues["limit"] = 80.0
			tt.Frame()
			if got := refreshRead(tt); got != "1e" {
				t.Errorf("external refresh destroyed live input: %q", got)
			}
			configIdentityFocus(t, tt, "工作目录")
			refreshDump(t, tt, "config-external", dark)
			if got := refreshRead(tt); got != "/workspace/external" {
				t.Errorf("external host value hidden by old draft: %q", got)
			}
			tt.Type("/workspace/local")
			tt.Frame()
			integration.ConfigValues["directory"] = "/workspace/another"
			tt.Frame()
			if got := refreshRead(tt); got != "/workspace/local" {
				t.Errorf("external refresh destroyed local edit: %q", got)
			}
			// Saving re-baselines immediately; the old integration list must
			// not undo it before the host's refreshed list arrives.
			a.configError = map[string]string{}
			a.finishConfigSave(integration.ID, nil, a.copy())
			tt.Frame()
			if got := refreshRead(tt); got != "/workspace/local" {
				t.Errorf("old host generation rolled back saved edit: %q", got)
			}
			integration.ConfigValues["directory"] = "/workspace/local"
			tt.Frame()
			integration.ConfigValues["directory"] = "/workspace/latest"
			tt.Frame()
			if got := refreshRead(tt); got != "/workspace/latest" {
				t.Errorf("new host generation remains hidden after save: %q", got)
			}
		})
	}
}
