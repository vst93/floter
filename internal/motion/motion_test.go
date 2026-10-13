package motion

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// An ambient loop's value is read through this package so both of the app's
// loops (the terminal bar's status dot and the launcher's paging ring) rest
// together on a desktop that asked for less motion. Two things are pinned
// here: the resting value, and that the framework is never asked for the
// loop's phase in that case — a registered loop goes on asking for frames
// whether or not the view draws its value, so reading it and then ignoring it
// would cost exactly the frames the preference is about.
func TestValueRestsUnderReduceMotion(t *testing.T) {
	const want = 0.5
	cases := []struct {
		name   string
		reduce bool
		want   float32
		calls  int
	}{
		{name: "the desktop animates", want: want, calls: 1},
		{name: "less motion asked for", reduce: true, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The tester renders one frame as it is made and one as the
			// preferences change; the assertions run on the frame after, at
			// the preferences under test.
			settled := false
			tt := ui.NewTester(func(c *ui.Context) {
				if !settled {
					return
				}
				calls := 0
				got := Value(c, func() float32 {
					calls++
					return want
				})
				if got != tc.want {
					t.Errorf("value = %v, want %v", got, tc.want)
				}
				if tc.reduce && calls != 0 {
					t.Errorf("the phase was asked for %d times under reduce motion", calls)
				}
				if !tc.reduce && calls == 0 {
					t.Error("the phase was never asked for")
				}
			}, 200, 100)
			tt.SetPreferences(ui.Preferences{ReduceMotion: tc.reduce})
			// A frame builds the view up to three times (an event handled while
			// building rebuilds it), so each pass asserts for itself.
			settled = true
			tt.Frame()
		})
	}
}
