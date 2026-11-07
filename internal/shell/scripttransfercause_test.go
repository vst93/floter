package shell

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func checkScriptTransferCause(t *testing.T, picker bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			dir := t.TempDir()
			destination := dir
			failure := error(nil)
			want := os.WriteFile(destination, nil, 0o600)
			name := "script-write"
			if picker {
				failure = errors.New("file picker unavailable: restart the desktop portal")
				want = failure
				name = "script-picker"
			}
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return destination, failure })
			clearNotices(a)
			a.Settings.Actions.ExportScript("diagnose.py", "print('ready')\n")
			toast := waitForToast(t, a, "floter-notice")
			if toast.Title != a.SettingsCopy().LocalToolExportFailed {
				t.Errorf("script refusal lost heading: %q", toast.Title)
			}
			tt := transferCauseFrame(t, a, dark)
			transferCauseDump(t, tt, name, dark)
			if !tt.HasText(want.Error()) {
				t.Errorf("%s hides failure cause: %s", name, want)
			}
			if !tt.HasText(a.SettingsCopy().LocalToolExportFailed) {
				t.Error("script failure heading is not visible")
			}
			clearNotices(a)
			failure = nil
			destination = filepath.Join(dir, "diagnose.py")
			a.Settings.Actions.ExportScript("diagnose.py", "print('ready')\n")
			if got := waitForToast(t, a, scriptExportNoticeID); got.Type != "success" {
				t.Errorf("script retry toast=%+v", got)
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != "print('ready')\n" {
				t.Errorf("script retry changed content: %q %v", data, err)
			}
		})
	}
}

func TestScriptExportPickerExplainsFailure(t *testing.T) { checkScriptTransferCause(t, true) }
func TestScriptExportWriteExplainsFailure(t *testing.T)  { checkScriptTransferCause(t, false) }
