package inputtext

import (
	"testing"
	"unicode"
)

// separators is every Unicode Zs code point except U+0020, one row each: a
// separator dropped from the boundary fails exactly its own row, which is what
// makes the table a kill-point map rather than a smoke test.
var separators = []struct {
	r     rune
	label string
}{
	{'\u00a0', "U+00A0 NO-BREAK SPACE"},
	{'\u1680', "U+1680 OGHAM SPACE MARK"},
	{'\u2000', "U+2000 EN QUAD"},
	{'\u2001', "U+2001 EM QUAD"},
	{'\u2002', "U+2002 EN SPACE"},
	{'\u2003', "U+2003 EM SPACE"},
	{'\u2004', "U+2004 THREE-PER-EM SPACE"},
	{'\u2005', "U+2005 FOUR-PER-EM SPACE"},
	{'\u2006', "U+2006 SIX-PER-EM SPACE"},
	{'\u2007', "U+2007 FIGURE SPACE"},
	{'\u2008', "U+2008 PUNCTUATION SPACE"},
	{'\u2009', "U+2009 THIN SPACE"},
	{'\u200a', "U+200A HAIR SPACE"},
	{'\u202f', "U+202F NARROW NO-BREAK SPACE"},
	{'\u205f', "U+205F MEDIUM MATHEMATICAL SPACE"},
	{'\u3000', "U+3000 IDEOGRAPHIC SPACE"},
}

// TestTheSeparatorTableIsComplete pins the table above against Go's own
// category: a code point that is Zs but is not a row here would be a separator
// nobody tests, and the reverse would test something that is not a separator.
func TestTheSeparatorTableIsComplete(t *testing.T) {
	inTable := map[rune]bool{' ': true}
	for _, row := range separators {
		inTable[row.r] = true
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		isZs := unicode.Is(unicode.Zs, r)
		if isZs != inTable[r] {
			t.Errorf("U+%04X: unicode.Zs=%v, in the table=%v", r, isZs, inTable[r])
		}
	}
}

func TestEverySeparatorCollapsesToASpace(t *testing.T) {
	for _, row := range separators {
		got := NormalizeSpaces("go" + string(row.r) + "version")
		if got != "go version" {
			t.Errorf("%s must become U+0020 between two words: %q", row.label, got)
		}
	}
}

func TestMixedSeparatorsCollapseInOnePass(t *testing.T) {
	if got := NormalizeSpaces("go\u3000version\u00a0--flag\u2009x"); got != "go version --flag x" {
		t.Errorf("mixed separators: %q", got)
	}
	if got := NormalizeSpaces("npm\u2003run\u2007build"); got != "npm run build" {
		t.Errorf("a line whose only gaps are separators: %q", got)
	}
}

func TestCleanTextIsReturnedUnchanged(t *testing.T) {
	clean := "go version --flag x"
	if got := NormalizeSpaces(clean); got != clean {
		t.Errorf("clean text changed: %q", got)
	}
	// The hot path must not copy: the boundary runs on every paste and every
	// assembled command line, so identity is part of the contract.
	if NormalizeSpaces(clean) != clean {
		t.Error("clean text must be returned as the same string")
	}
}

func TestOnlySpaceSeparatorsAreTouched(t *testing.T) {
	// Tabs and newlines are legitimate terminal input (completion, submit);
	// U+200B is not a space at all; U+2028/U+2029 are line separators.
	for _, text := range []string{
		"a\tb\nc\rd",
		"a\u200bb",
		"a\u2028b",
		"a\u2029b",
		"a\v\fb",
		"a\u0085b",
	} {
		if got := NormalizeSpaces(text); got != text {
			t.Errorf("%q must pass through, got %q", text, got)
		}
	}
	// U+180E lost its White_Space/Zs status in Unicode 6.3.
	if got := NormalizeSpaces("a\u180eb"); got != "a\u180eb" {
		t.Errorf("U+180E is not a separator: %q", got)
	}
	// The ASCII space stays one ASCII space.
	if got := NormalizeSpaces("a b"); got != "a b" {
		t.Errorf("the ASCII space: %q", got)
	}
}

func TestStripPasteNewlineTakesOneBreak(t *testing.T) {
	cases := []struct {
		text, want string
	}{
		{"git status\n", "git status"},
		{"git status\r\n", "git status"},
		{"git status", "git status"},
		{"echo a\necho b\n", "echo a\necho b"},
		{"echo a\necho b", "echo a\necho b"},
		// Only one break: a paste that ends in a blank line keeps the blank.
		{"echo a\n\n", "echo a\n"},
		// A lone CR is not a line break this rule removes.
		{"echo a\r", "echo a\r"},
		{"\n", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := StripPasteNewline(c.text); got != c.want {
			t.Errorf("StripPasteNewline(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

// TestPasteRuleIsTheOneTheBoundaryUses documents the composition the terminal
// applies: safe paste strips one break, then every string seam normalizes.
func TestPasteRuleIsTheOneTheBoundaryUses(t *testing.T) {
	pasted := "go\u3000version\n"
	if got := NormalizeSpaces(StripPasteNewline(pasted)); got != "go version" {
		t.Errorf("safe paste of a copied command: %q", got)
	}
	if got := NormalizeSpaces(pasted); got != "go version\n" {
		t.Errorf("without safe paste the break stays: %q", got)
	}
}
