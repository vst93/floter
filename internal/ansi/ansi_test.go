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

// A parameter that is not a number is ignored, not read as a zero: the
// original reads it as NaN and matches no code, so a malformed parameter must
// not silently reset the style in force (which is what a zero does).
func TestMalformedParametersAreIgnored(t *testing.T) {
	spans := Parse("\x1b[1;?mx")
	if !spans[0].Style.Bold {
		t.Errorf("a malformed parameter reset the style: %+v", spans[0].Style)
	}
	spans = Parse("\x1b[31;?mx")
	if spans[0].Style.FG != (Color{Kind: Named, N: 1}) {
		t.Errorf("a malformed parameter dropped the colour: %+v", spans[0].Style.FG)
	}
	// A malformed extended colour is dropped whole: the foreground keeps
	// whatever it had (nothing here), rather than becoming index 0.
	spans = Parse("\x1b[31;38;5;?mx")
	if spans[0].Style.FG != (Color{Kind: Named, N: 1}) {
		t.Errorf("a malformed index colour was read as a colour: %+v", spans[0].Style.FG)
	}
	spans = Parse("\x1b[38;2;255;?;9mx")
	if spans[0].Style.FG != (Color{}) {
		t.Errorf("a malformed truecolour was read as a colour: %+v", spans[0].Style.FG)
	}
	if !spans[0].Style.Strike {
		t.Errorf("the parameters after it were lost: %+v", spans[0].Style)
	}
}

// A `ESC [` whose next byte is neither a parameter byte nor a final byte is
// not a control sequence: only the ESC is dropped and the rest is text. The
// original reads it that way, so a stray control byte must not swallow the
// characters after it.
func TestNotAControlSequenceKeepsTheText(t *testing.T) {
	if got := Plain("a\x1b[\x07b"); got != "a[\x07b" {
		t.Errorf("Plain = %q", got)
	}
	// A byte above the final range (a UTF-8 lead) is the same case.
	if got := Plain("a\x1b[世b"); got != "a[世b" {
		t.Errorf("Plain = %q", got)
	}
	// A proper sequence still goes whole, and the text after it survives.
	if got := Plain("a\x1b[2Kb"); got != "ab" {
		t.Errorf("Plain = %q", got)
	}
}

// A designation is three bytes: ESC, an intermediate byte, and the value it
// designates. The value is never text — reading only two bytes printed it.
func TestDesignationsAreThreeBytes(t *testing.T) {
	for _, prefix := range []string{
		"\x1b(B", // ASCII into G0
		"\x1b)0", // the line-drawing set into G1
		"\x1b*A",
		"\x1b+B",
		"\x1b-A",
		"\x1b.B",
		"\x1b/A",
		"\x1b#8", // DECALN
		"\x1b%G", // UTF-8
	} {
		if got := Plain(prefix + "hello"); got != "hello" {
			t.Errorf("Plain(%q) = %q, want %q", prefix+"hello", got, "hello")
		}
		// The same designation between two runs of text leaves both.
		if got := Plain("a" + prefix + "b"); got != "ab" {
			t.Errorf("Plain(%q) = %q, want %q", "a"+prefix+"b", got, "ab")
		}
	}
}

// The two-byte escapes stay two bytes: a three-byte reading of `ESC =` would
// eat the character after it.
func TestTwoByteEscapesStayTwoBytes(t *testing.T) {
	for _, prefix := range []string{"\x1b=", "\x1b>", "\x1b7", "\x1b8", "\x1bM", "\x1bE"} {
		if got := Plain(prefix + "hello"); got != "hello" {
			t.Errorf("Plain(%q) = %q, want %q", prefix+"hello", got, "hello")
		}
		if got := Plain("a" + prefix + "b"); got != "ab" {
			t.Errorf("Plain(%q) = %q, want %q", "a"+prefix+"b", got, "ab")
		}
	}
}

// A designation with no value byte is unterminated: the rest is not printable,
// and nothing panics.
func TestTruncatedDesignation(t *testing.T) {
	if got := Plain("before\x1b("); got != "before" {
		t.Errorf("a truncated designation left %q", got)
	}
	if got := Plain("\x1b"); got != "" {
		t.Errorf("a bare escape left %q", got)
	}
}
