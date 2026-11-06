package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNoticeReorderedDisableKeepsAnswerOwner(t *testing.T) { checkNoticeReorder(t, false) }
func TestNoticeReorderedRemoveKeepsAnswerOwner(t *testing.T)  { checkNoticeReorder(t, true) }

func checkNoticeReorder(t *testing.T, remove bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, insert := range []bool{false, true} {
			name := "disable-reorder"
			if remove {
				name = "remove-reorder"
			}
			if insert {
				name += "-insert"
			}
			t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
				a := noticeApp(t, dark)
				copy := i18n.For("en").Settings
				first := Integration{ID: "first", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
				second := first
				second.ID, second.Name = "second", "Build Tools"
				list := []Integration{first, second}
				if insert {
					list = []Integration{first}
				}
				a.Integrations = func() []Integration { return list }
				var disabled, removed []string
				a.Actions.SetIntegrationEnabled = func(id string, _ bool, _ func(error)) { disabled = append(disabled, id) }
				a.Actions.UninstallIntegration = func(id, _, _ string, _ func(error)) { removed = append(removed, id) }
				tt := noticeExitPage(t, a, dark)
				opener := copy.IntegrationsEnable
				if remove {
					opener = first.removalLabel(copy)
				}
				if err := tt.Click(opener); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				answer := copy.DisableConfirm
				if remove {
					answer = first.removalLabel(copy)
				}
				if !tt.Focused(answer) {
					t.Fatal("answer did not take focus before refresh")
				}
				list = []Integration{second, first}
				tt.Frame()
				tt.Frame()
				noticeRefreshDump(t, tt, name, dark)
				if !tt.Focused(answer) {
					t.Error("refresh lost the original answer focus")
				}
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if remove {
					if len(removed) != 1 || removed[0] != first.ID || len(disabled) != 0 {
						t.Errorf("refreshed removal answer lost its owner: removed=%v disabled=%v", removed, disabled)
					}
				} else if len(disabled) != 1 || disabled[0] != first.ID || len(removed) != 0 {
					t.Errorf("refreshed disable answer lost its owner: disabled=%v removed=%v", disabled, removed)
				}
			})
		}
	}
}
