package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func copyLifecycleDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13II_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ii-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func copyLifecycleApp(t *testing.T, dark bool) *App {
	t.Helper()
	s := settings.Default()
	s.Language, s.Theme = "zh", appearanceName(dark)
	return New(settings.NewStore(s), Actions{ConnectLocalTool: func(CustomTool) {}, EditCustomTool: func(string, CustomTool) {}})
}

func TestCopyLifecycleNewEditorRejectsOldSuccess(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			a.beginEdit("local.first", tool)
			calls := 0
			a.Actions.CopyText = func(value string) {
				calls++
				if value != "weather" {
					t.Errorf("copied %q", value)
				}
				a.beginEdit("local.second", tool)
			}
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			copyLifecycleDump(t, tt, "copy-new-editor", dark)
			if a.toolEditing != "local.second" || !tt.HasText("local.second") || calls != 1 {
				t.Fatal("new editor was not retained")
			}
			if tt.HasText(copy.LocalToolCommandCopied) || a.copiedContent != "" {
				t.Error("old copy returned success into new editor")
			}
			a.Actions.CopyText = func(string) { calls++ }
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 || !tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("fresh copy failed")
			}
			if copiedContentDuration != 1600*time.Millisecond {
				t.Error("copy window changed")
			}
		})
	}
}

func TestCopyLifecycleUnavailableContentIsInert(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.beginEdit("local.weather", callbackTool("weather"))
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			copyLifecycleDump(t, tt, "copy-unavailable", dark)
			if tt.Focused(copy.LocalToolCopyCommand) {
				t.Error("unavailable content copy still accepts focus")
			}
			if a.copiedContent != "" {
				t.Error("unavailable copy confirmed")
			}
			calls := 0
			a.Actions.CopyText = func(value string) {
				calls++
				if value != "weather" {
					t.Errorf("copied %q", value)
				}
			}
			tt.Frame()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 || !tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("restored copy did not dispatch once")
			}
		})
	}
}
