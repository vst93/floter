package shell

import (
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestShortcutLifecycleRecorderHandoff(t *testing.T) {
	for _, scenario := range []string{"summon-to-custom", "custom-to-summon", "summon-to-draft"} {
		for _, dark := range []bool{false, true} {
			t.Run(scenario+map[bool]string{false: "/light", true: "/dark"}[dark], func(t *testing.T) {
				h := newShortcutLifecycle(t, "Ctrl+Alt+J")
				tt := h.frame(dark)
				copy := i18n.For("en").Settings
				first, second := "Ctrl+Alt+J", shortcuts.Display("Ctrl+Shift+B")
				if scenario == "custom-to-summon" {
					first, second = "Ctrl+Shift+B", shortcuts.Display("Ctrl+Alt+J")
				}
				if scenario == "summon-to-draft" {
					second = copy.ShortcutsAdd
				}
				lifecycleRecord(t, h, tt, first)
				if err := tt.Click(second); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				lifecycleDump(t, tt, scenario, dark)
				prompts := 0
				for _, text := range tt.Texts() {
					if text == copy.ShortcutRecording {
						prompts++
					}
				}
				if prompts != 2 {
					t.Errorf("recorder handoff left %d prompt entries, want one pane (2 entries)", prompts)
				}
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				tt.Frame()
				tt.Frame()
				if h.app.shortcutsSuspended || tt.HasText(copy.ShortcutRecording) {
					t.Error("completed handoff left another recorder holding global shortcuts")
				}
				entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
				wantSummon, wantCustom := "Ctrl+Alt+J", "Ctrl+Shift+B"
				if scenario == "custom-to-summon" {
					wantSummon = "Ctrl+Shift+Z"
				}
				if scenario == "summon-to-custom" {
					wantCustom = "Ctrl+Shift+Z"
				}
				if SummonShortcut(h.app.Store.Snapshot()) != wantSummon || len(entries) != 1 || entries[0].Key != wantCustom || entries[0].Action != "say original" {
					t.Errorf("handoff recorded into wrong owner: summon=%s custom=%v", SummonShortcut(h.app.Store.Snapshot()), entries)
				}
				if h.live[wantSummon] == nil || h.live[wantCustom] == nil {
					t.Error("completed handoff did not restore confirmed keys")
				}
			})
		}
	}
}
