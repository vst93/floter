package launcher

import (
	"fmt"
	"testing"

	"floter/internal/extensions"
)

// A background run changes the list band and nothing above it.
//
// Three states of one page: the query typed, the run in flight, and the run's
// output on screen. The search bar — the field, its clear key, its settings
// key — is byte-identical in all three. The user asked for exactly this
// (「理论上搜索栏部分应该不动或有一个执行的 loading 动画，然后变化的是下方
// 列表部分」), and the two ways this build used to break it are pinned here:
//
//   - `View` returned from the whole panel while `a.output != nil`, so the field
//     row was never built (no field at all, and a blank band above the strip);
//   - `triggerHintText` suppressed the inline nudge for `a.output != nil`, so
//     the field *jumped* 110px wider the moment a run landed.
func TestABackgroundRunMovesOnlyTheListBand(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	var land func(extensions.CapturedRun, error)
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		land = done // held: the run stays in flight for the frame under test
	}
	tt := render(t, a)
	tt.Frame()
	a.Query = "list"
	tt.Frame()
	tt.Frame()

	// The search bar's own controls, in the three states: one string per
	// control, so two states compare with `==` and a failure prints the
	// difference rather than two addresses.
	bar := func() map[string]string {
		out := map[string]string{}
		for _, label := range []string{"Search", "Clear", "Settings"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%q is not drawn: %q", label, tt.Texts())
			}
			out[label] = fmt.Sprintf("%.2f/%.2f/%.2f/%.2f", r.X, r.Y, r.W, r.H)
		}
		return out
	}
	same := func(stage string, got, want map[string]string) {
		t.Helper()
		for label, wantBox := range want {
			if got[label] != wantBox {
				t.Errorf("%s: %q moved %s -> %s", stage, label, wantBox, got[label])
			}
		}
	}
	typed := bar()

	// ① In flight: the band says so, the bar does not move.
	a.activate(a.Results())
	tt.Frame()
	if !tt.HasText(a.copy().OutputRunning) {
		t.Errorf("the run in flight says nothing in the list band: %q", tt.Texts())
	}
	same("in flight", bar(), typed)

	// ② The run lands: the band shows its output, the bar still does not move.
	if land == nil {
		t.Fatal("the run never started")
	}
	land(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: "a\nb"}, nil)
	tt.Frame()
	if !a.InOutputView() {
		t.Fatal("the output is not on screen after the run landed")
	}
	same("output landed", bar(), typed)

	// ③ Editing the field is a new question: the list comes back and the output
	//    goes. The nudge is a word about *that* query, so it follows the query
	//    (a query that matches no command has no nudge) — this half asserts the
	//    band, not the bar's width, which is the query's own business.
	tt.Type("x")
	tt.Frame()
	if a.InOutputView() {
		t.Error("editing the field left the old run's output in the band")
	}
	if _, ok := tt.Find("Search"); !ok {
		t.Error("editing the field dropped the search bar")
	}
}
