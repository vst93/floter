package shell

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestScriptExportCompletionEveryExit(t *testing.T) {
	for _, outcome := range []string{"cancel", "picker-error", "write-error", "success"} {
		t.Run(outcome, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "weather.py")
			var pickerError error
			switch outcome {
			case "cancel":
				path = ""
			case "picker-error":
				pickerError = errors.New("portal unavailable")
			case "write-error":
				path = dir
			}
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return path, pickerError })
			clearNotices(a)
			if a.Settings.Actions.ExportScriptAsync == nil {
				t.Fatal("script completion is not wired to host")
			}
			done := make(chan struct{}, 2)
			var calls atomic.Int32
			finish := func() { calls.Add(1); done <- struct{}{} }
			a.Settings.Actions.ExportScriptAsync("weather.py", "print('sunny')\n", finish)
			adoptionWait(t, done)
			if calls.Load() != 1 {
				t.Fatalf("completion count=%d", calls.Load())
			}
			notices := noticeTitles(a)
			if outcome == "cancel" && len(notices) != 0 {
				t.Errorf("cancel published result: %q", notices)
			}
			if outcome != "cancel" && len(notices) != 1 {
				t.Errorf("export result count: %q", notices)
			}
			// Completion is the worker boundary: the next request may safely
			// use a new destination and must carry the newly supplied bytes.
			path, pickerError = filepath.Join(dir, "forecast.py"), nil
			a.Settings.Actions.ExportScriptAsync("forecast.py", "print('rain')\n", finish)
			adoptionWait(t, done)
			data, err := os.ReadFile(path)
			if calls.Load() != 2 || err != nil || string(data) != "print('rain')\n" {
				t.Errorf("retry after %s: calls=%d data=%q err=%v", outcome, calls.Load(), data, err)
			}
		})
	}
}
