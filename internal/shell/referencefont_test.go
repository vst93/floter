package shell

import (
	"os"
	"testing"
)

// referenceFontTests are the shell's assertions that read absolute type boxes:
// a line box's own height, the centre of a row's text. Those numbers are the
// *shipped UI font's* line boxes, so a runner whose font set resolves
// differently measures the same view differently (Segoe's line box is taller
// than DejaVu's, so the same 56u band centres its title half a pixel lower) and
// the assertion goes red on the environment rather than on the build.
//
// The headless CI runners this project uses carry no UI font of their own, so
// there these tests can only mislead: they skip with the reason instead of
// going red, and they stay the ladder's anchors on a machine that has the
// shipped font. The same rule (and the same wording) as
// `settingsui.referenceMetricTests`, which is where the rest of the ladder
// lives; the fix for each of them is to compare within the frame it measures —
// the ratio of two boxes, the ordering of two steps — rather than a literal
// pixel count.
var referenceFontTests = map[string]bool{
	"TestSettingsHeaderSharesTheLauncherFieldRowBand": true,
}

// skipWithoutTheReferenceFont skips the tests whose assertions are the
// reference font's own measurements when the environment is a headless CI
// runner, where that font is not the one resolving.
func skipWithoutTheReferenceFont(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") == "" || !referenceFontTests[t.Name()] {
		return
	}
	t.Skip("the type ladder's boxes are the shipped UI font's line boxes, which this headless runner does not carry")
}
