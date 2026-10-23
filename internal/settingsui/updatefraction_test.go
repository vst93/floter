package settingsui

import "testing"

// TestUpdateFractionIsTheOneDownloadFraction: the About page's own track and
// the system's progress surface read the same fraction from here, so this is
// where the three edge cases live — no total, more bytes than expected (the
// feed's total was short), and a negative count. Every caller clamps by
// reading this function rather than repeating the arithmetic.
func TestUpdateFractionIsTheOneDownloadFraction(t *testing.T) {
	cases := []struct {
		name string
		p    *UpdateProgress
		want float64
	}{
		{"no progress", nil, 0},
		{"no total", &UpdateProgress{Downloaded: 1024, Total: 0}, 0},
		{"half", &UpdateProgress{Downloaded: 2, Total: 4}, 0.5},
		{"over the total", &UpdateProgress{Downloaded: 9, Total: 4}, 1},
		{"negative", &UpdateProgress{Downloaded: -1, Total: 4}, 0},
	}
	for _, c := range cases {
		if got := UpdateFraction(c.p); got != c.want {
			t.Errorf("%s: UpdateFraction = %v, want %v", c.name, got, c.want)
		}
	}
}
