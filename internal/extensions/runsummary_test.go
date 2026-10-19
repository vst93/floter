//go:build !windows

package extensions

import (
	"math"
	"testing"
)

// The duration word form: sub-second runs read in whole milliseconds, longer
// ones to one decimal of a second, and a value a clock cannot have produced
// degrades to "0 ms".
func TestFormatRunDuration(t *testing.T) {
	cases := []struct {
		ms   float64
		want string
	}{
		{math.NaN(), "0 ms"},
		{math.Inf(1), "0 ms"},
		{math.Inf(-1), "0 ms"},
		{-1, "0 ms"},
		{0, "0 ms"},
		{420, "420 ms"},
		{999.0, "999 ms"},
		{999.4, "999 ms"},
		{999.6, "1000 ms"},
		{1000, "1.0 s"},
		{3400, "3.4 s"},
		{9999, "10.0 s"},
	}
	for _, c := range cases {
		if got := FormatRunDuration(c.ms); got != c.want {
			t.Errorf("FormatRunDuration(%v) = %q, want %q", c.ms, got, c.want)
		}
	}
}

// The line count joins both streams in capture order, and a single trailing
// newline does not invent an extra line.
func TestRunOutputLineCount(t *testing.T) {
	cases := []struct {
		name string
		run  CapturedRun
		want int
	}{
		{"empty", CapturedRun{}, 0},
		{"only a newline is one line", CapturedRun{Stdout: "\n"}, 1},
		{"two lines", CapturedRun{Stdout: "a\nb"}, 2},
		{"a trailing newline does not count", CapturedRun{Stdout: "a\n"}, 1},
		{"two trailing newlines leave one blank line", CapturedRun{Stdout: "a\n\n"}, 2},
		{"stderr joins stdout in order", CapturedRun{Stdout: "a\n", Stderr: "b"}, 2},
		{"a lone stderr line counts", CapturedRun{Stderr: "x\ny\n"}, 2},
		{"truncation does not change the count", CapturedRun{Stdout: "a\nb\n", Truncated: true}, 2},
	}
	for _, c := range cases {
		if got := RunOutputLineCount(c.run); got != c.want {
			t.Errorf("%s: RunOutputLineCount = %d, want %d", c.name, got, c.want)
		}
	}
}

// Whether a run said anything at all, on either stream.
func TestRunHasOutput(t *testing.T) {
	if RunHasOutput(CapturedRun{}) {
		t.Error("an empty run has no output")
	}
	if !RunHasOutput(CapturedRun{Stdout: "x"}) {
		t.Error("stdout counts")
	}
	if !RunHasOutput(CapturedRun{Stderr: "x"}) {
		t.Error("stderr counts")
	}
}
