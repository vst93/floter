package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
)

type appearanceRecorder struct{ fonts, themes, copies int }

func (r *appearanceRecorder) SetFont(terminal.Font)                     { r.fonts++ }
func (r *appearanceRecorder) SetTheme(*terminal.Theme, *terminal.Theme) { r.themes++ }
func (r *appearanceRecorder) SetCopyOnSelect(terminal.CopyOnSelect)     { r.copies++ }

func TestAppearanceRefreshPreservesZoomOnPageReturn(t *testing.T) {
	opts := terminal.Options{Font: terminal.Font{Family: "monospace", Size: 14}, Theme: terminal.LightTheme(), DarkTheme: terminal.DarkTheme()}
	previous := appearanceOf(opts)
	recorder := &appearanceRecorder{}
	for range 10 {
		// Options rebuilds equivalent theme pointers on each page return.
		opts.Theme, opts.DarkTheme = terminal.LightTheme(), terminal.DarkTheme()
		previous.apply(recorder, opts)
	}
	if *recorder != (appearanceRecorder{}) {
		t.Fatalf("unchanged page return reset appearance: %+v", recorder)
	}
	opts.CopyOnSelect = terminal.CopyOnSelectClipboard
	previous.apply(recorder, opts)
	if *recorder != (appearanceRecorder{copies: 1}) {
		t.Fatalf("copy setting reset font or theme: %+v", recorder)
	}
	opts.Font.Size = 16
	previous.apply(recorder, opts)
	if recorder.fonts != 1 || recorder.themes != 0 {
		t.Fatalf("font change was not applied once: %+v", recorder)
	}
	opts.Theme = nil
	previous.apply(recorder, opts)
	if recorder.themes != 1 {
		t.Fatal("theme reset did not reach terminal")
	}
}
