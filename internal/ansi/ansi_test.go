package ansi

import (
	"strings"
	"testing"
)

// Text with no escapes is one span: a caller can always draw the answer, and
// the common case costs nothing.
func TestPlainTextIsOneSpan(t *testing.T) {
	spans := Parse("hello 世界")
	if len(spans) != 1 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "hello 世界" {
		t.Errorf("text = %q", spans[0].Text)
	}
	if spans[0].Style != (Style{}) {
		t.Errorf("style = %+v", spans[0].Style)
	}
	if got := Plain("hello 世界"); got != "hello 世界" {
		t.Errorf("Plain = %q", got)
	}
}

// A colour sequence styles the run after it, and the reset returns to the
// surface's own colours.
func TestColourAndReset(t *testing.T) {
	spans := Parse("\x1b[31mred\x1b[0m plain")
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "red" || spans[0].Style.FG != (Color{Kind: Named, N: 1}) {
		t.Errorf("the red run = %+v", spans[0])
	}
	if spans[1].Text != " plain" || spans[1].Style != (Style{}) {
		t.Errorf("the plain run = %+v", spans[1])
	}
}

// The weights and decorations a CLI writes are read, and their own "off" codes
// undo them one at a time.
func TestWeightsAndDecorations(t *testing.T) {
	spans := Parse("\x1b[1;3;4;9mstyled\x1b[22;23;24;29mback")
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	style := spans[0].Style
	if !style.Bold || !style.Italic || !style.Underline || !style.Strike {
		t.Errorf("style = %+v", style)
	}
	if spans[1].Style != (Style{}) {
		t.Errorf("the codes did not undo: %+v", spans[1].Style)
	}
}

// The bright colours, the backgrounds and the inverse flag.
func TestBrightBackgroundAndInverse(t *testing.T) {
	spans := Parse("\x1b[91;104;7mx")
	style := spans[0].Style
	if style.FG != (Color{Kind: Named, N: 9}) {
		t.Errorf("fg = %+v", style.FG)
	}
	if style.BG != (Color{Kind: Named, N: 12}) {
		t.Errorf("bg = %+v", style.BG)
	}
	if !style.Inverse {
		t.Error("inverse was not read")
	}
}

// The 256-colour and truecolour forms are read as the terminal reads them.
func TestExtendedColours(t *testing.T) {
	spans := Parse("\x1b[38;5;196mindexed\x1b[0m \x1b[38;2;18;52;86mtrue\x1b[0m")
	if len(spans) != 3 {
		t.Fatalf("spans = %+v", spans)
	}
	if got := spans[0].Style.FG; got != (Color{Kind: Indexed, N: 196}) {
		t.Errorf("indexed = %+v", got)
	}
	if got := spans[2].Style.FG; got != (Color{Kind: RGB, R: 18, G: 52, B: 86}) {
		t.Errorf("truecolour = %+v", got)
	}
	// The colon form is the same colour.
	spans = Parse("\x1b[38:5:196mx")
	if got := spans[0].Style.FG; got != (Color{Kind: Indexed, N: 196}) {
		t.Errorf("colon form = %+v", got)
	}
	spans = Parse("\x1b[48:2:18:52:86mx")
	if got := spans[0].Style.BG; got != (Color{Kind: RGB, R: 18, G: 52, B: 86}) {
		t.Errorf("colon truecolour = %+v", got)
	}
}

// Every escape that is not an SGR is dropped, never printed: a cursor move or
// an erase must not become a line of noise.
func TestOtherEscapesAreDropped(t *testing.T) {
	spans := Parse("a\x1b[2Kb\x1b[1Ac\x1b]0;title\x07d\x1b\\e")
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.Text)
	}
	if got := text.String(); got != "abcde" {
		t.Errorf("text = %q", got)
	}
}

// An SGR the parser does not know leaves the style alone rather than eating
// the characters after it.
func TestUnknownCodesAreIgnored(t *testing.T) {
	spans := Parse("\x1b[1;999;31mx")
	if len(spans) != 1 {
		t.Fatalf("spans = %+v", spans)
	}
	if !spans[0].Style.Bold || spans[0].Style.FG != (Color{Kind: Named, N: 1}) {
		t.Errorf("style = %+v", spans[0].Style)
	}
	if spans[0].Text != "x" {
		t.Errorf("text = %q", spans[0].Text)
	}
}

// A bare escape and an unterminated sequence end the text rather than printing
// control bytes.
func TestTruncatedSequences(t *testing.T) {
	if got := Plain("before\x1b"); got != "before" {
		t.Errorf("a bare escape left %q", got)
	}
	if got := Plain("before\x1b[31"); got != "before" {
		t.Errorf("an unterminated sequence left %q", got)
	}
}

// An empty parameter is a zero: `ESC[m` resets, and a doubled `;;` in the
// middle resets what came before it.
func TestEmptyParametersAreZeros(t *testing.T) {
	spans := Parse("\x1b[31mred\x1b[mplain")
	if len(spans) != 2 || spans[1].Style != (Style{}) {
		t.Fatalf("spans = %+v", spans)
	}
	spans = Parse("\x1b[1;;31mx")
	if spans[0].Style.Bold {
		t.Errorf("the empty parameter did not reset: %+v", spans[0].Style)
	}
	if spans[0].Style.FG != (Color{Kind: Named, N: 1}) {
		t.Errorf("the colour after it was lost: %+v", spans[0].Style)
	}
}

// A sequence in the middle of a multibyte character's neighbourhood does not
// split it.
func TestMultibyteText(t *testing.T) {
	spans := Parse("世界\x1b[32m绿色\x1b[0m结束")
	if len(spans) != 3 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "世界" || spans[1].Text != "绿色" || spans[2].Text != "结束" {
		t.Errorf("spans = %+v", spans)
	}
	if got := Plain("世界\x1b[32m绿色\x1b[0m结束"); got != "世界绿色结束" {
		t.Errorf("Plain = %q", got)
	}
}
