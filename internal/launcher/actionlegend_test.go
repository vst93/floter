package launcher

import (
	"testing"
)

// The action bar draws one chord, and it is one that works.
//
// The footer used to carry a second keycap beside its own chord: `⌘K` /
// `Ctrl+K`, the reference's "actions surface" legend. That surface has never
// been built in this build, so the keycap named a chord nothing answers — and
// on the "run it in a shell" row it read as a second way into the terminal,
// which is what the user reported (「去掉进入终端的 super+k 快捷键保留一个
// super+回车就行了」).
//
// This pin is the *absence*: no keycap in the footer spells a chord the app
// does not answer. The row's own chord is asserted where it belongs (the
// keycap-shape pins in `rowtrailing_test.go` / `raycast_test.go`).
func TestTheActionBarDrawsOnlyTheChordItAnswers(t *testing.T) {
	a := testApp()
	barBreathApps(a)
	a.Query = "app"
	tt := render(t, a)
	tt.Frame()
	tt.Frame()

	// The footer's own chord: the select_result family head plus Enter, which
	// is what `enterBadge` draws and what Enter on the bar runs.
	enter := keycapLabels(a.enterBadge())
	if len(enter) < 2 {
		t.Fatalf("the footer's chord %q is not a chord: %v", a.enterBadge(), enter)
	}
	for _, label := range enter {
		if !tt.HasText(label) {
			t.Errorf("the footer's own chord is missing the key %q: %q", label, tt.Texts())
		}
	}

	// …and no second legend: the `K` plate the actions legend drew is gone.
	if tt.HasText("K") {
		t.Errorf("the footer still draws the `K` of an unbound actions legend: %q", tt.Texts())
	}
}
