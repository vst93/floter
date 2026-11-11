package launcher

import (
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

// A run does not end the command mode: Enter on a fresh list re-runs it.
//
// The report 「回车之后输入框内应该保持原来的逻辑，而不是自动切回到最初的搜索
// 状态，这样可以方便用户及时调整输入重新执行」 names the second half of the
// bar's move. `runMode` used to call `leaveCommand` on its way to the run, so
// Enter on `-m` dropped the scope marker and handed the arguments back to the
// ordinary search — the line the user had just run became a query matching
// nothing, and running it again meant finding the command row a second time.
//
// The reference never leaves: a run is an event *inside* the mode, and Enter on
// the list after it runs the same command with the edited arguments
// (`main:src/App.tsx:1484-1489`). Escape is the way out, and it keeps the text.
func TestARunKeepsTheCommandModeSoTheArgumentsCanBeEdited(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "v", Name: "V"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	var args [][]string
	var land func(extensions.CapturedRun, error)
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, argv []string, done func(extensions.CapturedRun, error)) {
		args = append(args, argv)
		land = done // held: the run is in flight for the frames that follow
	}
	tt := render(t, a)
	tt.Frame()
	tt.Type("v ttt")
	tt.Frame()
	if a.mode == nil {
		t.Fatalf("typing the trigger word did not enter the mode: %q", tt.Texts())
	}

	// Enter runs the typed arguments…
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(args) != 1 {
		t.Fatalf("Enter ran the command %d times, want once", len(args))
	}
	// …and the mode is still the field's owner, with the text where it was.
	if a.mode == nil {
		t.Errorf("the run ended the command mode: %q", tt.Texts())
	}
	if a.Query != "ttt" {
		t.Errorf("the run changed the field to %q, want the arguments it ran", a.Query)
	}
	// The scope marker is the mode's own claim on the field; the plain search
	// page has none, so its absence is exactly the "back to the first search
	// state" the report refuses.
	if !tt.HasText("V") {
		t.Errorf("the mode's scope marker left with the run: %q", tt.Texts())
	}

	// The run lands. The output is the list's content for *this* line; the
	// mode and the text are still the user's.
	land(extensions.CapturedRun{Command: []string{"v"}, Success: true, Stdout: "a\nb"}, nil)
	tt.Frame()
	if !a.InOutputView() {
		t.Fatalf("the run's output did not open: %q", tt.Texts())
	}
	if a.mode == nil || a.Query != "ttt" {
		t.Errorf("the output took the mode with it: mode=%v query=%q", a.mode != nil, a.Query)
	}

	// Editing the arguments abandons the old output — it described the *old*
	// line — and hands the band back to the mode's list.
	tt.Type("x")
	tt.Frame()
	if a.InOutputView() {
		t.Errorf("a keystroke left the old run's output up: %q", tt.Texts())
	}
	if a.mode == nil {
		t.Errorf("editing the arguments left the mode: %q", tt.Texts())
	}

	// Enter on the fresh list runs the edited line, without a trip back to the
	// command row: that is the whole point of keeping the mode.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(args) != 2 {
		t.Fatalf("the second Enter ran %d times, want twice (%v)", len(args), args)
	}
	if got := args[1]; len(got) != 1 || got[0] != "tttx" {
		t.Errorf("the re-run got %q, want the edited arguments", got)
	}
}

// Escape is still the way out, and it keeps what the user typed.
//
// The mode and the text are two different things: leaving the first must not
// edit the second (the reference's R31 rule, `main:src/App.tsx:1151-1161`).
// Escape peels one layer at a time — the output, then the mode, then the query
// — which is the ladder the panel already had; a run no longer takes the first
// two steps for the user.
func TestEscapeLeavesTheModeAndKeepsTheArguments(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "v", Name: "V"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Success: true, Stdout: "a"}, nil)
	}
	tt := render(t, a)
	tt.Frame()
	tt.Type("v ttt")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.InOutputView() {
		t.Fatalf("the run's output did not open: %q", tt.Texts())
	}

	// The first press closes the output and nothing else.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.InOutputView() {
		t.Errorf("Escape left the output up")
	}
	if a.mode == nil {
		t.Errorf("closing the output also left the mode")
	}

	// The second leaves the mode, with the arguments still in the field.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.mode != nil {
		t.Errorf("Escape did not leave the mode")
	}
	if a.Query != "ttt" {
		t.Errorf("Escape changed the field to %q, want the arguments", a.Query)
	}
}
