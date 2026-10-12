package extensions

import (
	"math"
	"strconv"
	"strings"
)

// The pure half of a finished run's summary: how long it took, whether it said
// anything, and how many lines it said. The launcher's output view and the
// completion notification read the same projections, so the two cannot phrase
// one run differently.
//
// The word forms are the old build's (`run-routing.ts`): a sub-second run is
// read in whole milliseconds ("420 ms"), anything longer rounds to one decimal
// of a second ("3.4 s"), and a value a clock cannot have produced (not finite,
// or negative) degrades to "0 ms" rather than rendering a NaN.

// FormatRunDuration phrases a duration in milliseconds for display. A
// sub-second run is read in whole milliseconds; a longer one to one decimal of
// a second; a non-finite or negative value is "0 ms".
func FormatRunDuration(milliseconds float64) string {
	if math.IsNaN(milliseconds) || math.IsInf(milliseconds, 0) || milliseconds < 0 {
		return "0 ms"
	}
	if milliseconds < 1000 {
		return strconv.FormatFloat(math.Round(milliseconds), 'f', -1, 64) + " ms"
	}
	return strconv.FormatFloat(milliseconds/1000, 'f', 1, 64) + " s"
}

// RunHasOutput reports whether a run produced anything at all on either
// stream. It drives the empty state and whether the completion notification
// offers a line count.
func RunHasOutput(run CapturedRun) bool {
	return len(run.Stdout) > 0 || len(run.Stderr) > 0
}

// RunOutputLineCount counts the lines a run's combined output produced. Both
// streams read in the order they were captured, exactly one trailing newline
// does not invent an extra empty line, and a stream of a single newline still
// counts as one line. An empty capture is zero lines, so a silent script says
// "0 lines" rather than "1 line".
func RunOutputLineCount(run CapturedRun) int {
	text := run.Stdout + run.Stderr
	if text == "" {
		return 0
	}
	if strings.HasSuffix(text, "\n") {
		text = text[:len(text)-1]
	}
	return strings.Count(text, "\n") + 1
}
