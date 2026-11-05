package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

func localParamsFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.toolParamsEditor(c, a.copy()) }) })
	}, 720, 700)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func TestLocalDraftParameterRemovalKeepsRemainingInput(t *testing.T) {
	a := localDraftApp()
	a.toolParams = []toolParam{{ID: "first", Label: "First", Kind: extensions.ParamText}, {ID: "second", Label: "Second", Kind: extensions.ParamText}}
	tt := localParamsFrame(t, a, true)
	// The next plate starts after the preceding remove key, its bottom
	// padding, the 6px plate gap and the next plate's top padding.
	label := mustFind(t, tt, a.copy().ParamsLabel)
	remove := mustFind(t, tt, a.copy().ParamsRemove)
	tt.ClickAt(label.X+40, remove.Y+remove.H+8+6+8+label.H+3+15)
	tt.Frame()
	if !tt.Focused(a.copy().ParamsLabel) {
		t.Fatal("remaining label did not take focus")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	// Removing a preceding declaration shifts the slice but must not transfer
	// the focused editor to a different declaration or discard its selection.
	a.toolParams = a.toolParams[1:]
	tt.Frame()
	tt.Type("Remaining")
	tt.Frame()
	if a.toolParams[0].Label != "Remaining" {
		t.Errorf("remaining parameter lost edit: %q", a.toolParams[0].Label)
	}
}

func TestLocalDraftParameterKindKeepsDefaultInput(t *testing.T) {
	a := localDraftApp()
	a.toolParams = []toolParam{{ID: "target", Kind: extensions.ParamText, Default: "old", Flag: "--target"}}
	tt := localParamsFrame(t, a, true)
	r := mustFind(t, tt, a.copy().ParamsDefault)
	tt.ClickAt(r.X+40, r.Y+r.H+3+15)
	tt.Frame()
	if !tt.Focused(a.copy().ParamsDefault) {
		t.Fatal("default did not take focus")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	a.toolParams[0].Kind = extensions.ParamBoolean
	tt.Frame()
	tt.Type("true")
	tt.Frame()
	if a.toolParams[0].Default != "true" || a.toolParams[0].Flag != "--target" {
		t.Errorf("kind switch moved default edit: default=%q flag=%q", a.toolParams[0].Default, a.toolParams[0].Flag)
	}
}
