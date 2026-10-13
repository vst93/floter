package launcher

import (
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"

	"floter/internal/extensions"
)

// jsWhitespaceLiteral is JavaScript's `\s` written out a second time, on
// purpose: the test checks the switch in isWordGap against this literal rather
// than against the production constant, so a code point added to one and not
// the other is caught.
const jsWhitespaceLiteral = "\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003" +
	"\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

// The word-gap set is JavaScript's `\s` and nothing else: every rune in the
// whole range is checked, so neither a missing gap nor a borrowed one survives.
func TestWordGapsAreJavaScriptsWhitespace(t *testing.T) {
	want := map[rune]bool{}
	for _, r := range jsWhitespaceLiteral {
		want[r] = true
	}
	if len(want) != 25 {
		t.Fatalf("the JS `\\s` set has %d code points, want 25", len(want))
	}
	if jsWhitespace != jsWhitespaceLiteral {
		t.Errorf("jsWhitespace = %q, want %q", jsWhitespace, jsWhitespaceLiteral)
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if isWordGap(r) != want[r] {
			t.Errorf("U+%04X: isWordGap=%v, want %v", r, isWordGap(r), want[r])
		}
	}
	// The code points the Zs category and Go's unicode.IsSpace disagree about:
	// JavaScript's `\s` is neither of those, so none of these is a gap.
	for _, r := range []rune{'\u0085', '\u180e', '\u200b'} {
		if isWordGap(r) {
			t.Errorf("U+%04X is not a JavaScript `\\s` gap", r)
		}
	}
}

// A command line splits on the launcher's gaps, not on the shell's U+0020: a
// full-width space from a Chinese input method separates words, and a
// multi-byte gap is decoded whole rather than as three bytes.
func TestWordGapsSeparateACommandLine(t *testing.T) {
	for _, gap := range []string{"\u00a0", "\u3000", "\u2028", "\ufeff", "\n"} {
		line := "jv" + gap + "-f" + gap + "file.json"
		want := []string{"jv", "-f", "file.json"}
		if got := splitArgs(line); !reflect.DeepEqual(got, want) {
			t.Errorf("splitArgs(%q) = %#v, want %#v", line, got, want)
		}
		if got := firstWord(line); got != "jv" {
			t.Errorf("firstWord(%q) = %q, want jv", line, got)
		}
	}
	// A gap inside quotes is part of the word, exactly as a plain space is.
	if got := splitArgs("jv \"a\u3000b\""); !reflect.DeepEqual(got, []string{"jv", "a\u3000b"}) {
		t.Errorf("a quoted gap = %#v", got)
	}
	// A multi-byte gap must not be cut in half: every token is valid UTF-8.
	for _, tok := range splitArgs("a\u3000b\u00a0c\u2028d") {
		if !utf8.ValidString(tok) {
			t.Errorf("token %q is not valid UTF-8", tok)
		}
	}
}

// The trigger split sees the same gaps, so `clip\u3000foo` enters the mode the
// way `clip foo` does, and the word is consumed on entry as it always is.
func TestEveryWordGapEntersTheMode(t *testing.T) {
	for _, gap := range []string{" ", "\t", "\n", "\u00a0", "\u3000", "\u2028", "\ufeff"} {
		a := testApp()
		a.Query = "clip" + gap + "foo"
		a.syncTypedMode()
		if !a.clipboard {
			t.Errorf("clip%qfoo did not enter the clipboard mode", gap)
			continue
		}
		if a.modeWord != "clip" {
			t.Errorf("clip%qfoo recorded trigger %q, want clip", gap, a.modeWord)
		}
	}
}

// An extension command's trigger word splits the same way, and the command
// mode keeps the rest as its argv (`jv\u3000-f` opens the mode on `-f`).
func TestAFullWidthSpaceEntersACommandMode(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{
		{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
	})
	a.Query = "jv\u3000-f"
	a.syncTypedMode()
	if a.mode == nil || a.mode.Command.ID != "jv" {
		t.Fatalf("jv\\u3000-f did not enter the command mode: %+v", a.mode)
	}
	if a.Query != "-f" {
		t.Errorf("the command's argv = %q, want -f", a.Query)
	}
}

// A command's name reduces to its first word on the same gaps, so a multi-word
// name joined by a full-width space is summonable by that first word.
func TestWordGapsReduceATriggerToItsFirstWord(t *testing.T) {
	entry := extensions.CommandEntry{
		Command: extensions.Command{
			ID:      "acme",
			Name:    "Search\u3000Acme",
			Aliases: []string{"sa\u00a0now"},
		},
	}
	want := []string{"acme", "Search", "sa"}
	if got := commandTriggers(entry); !reflect.DeepEqual(got, want) {
		t.Errorf("commandTriggers = %#v, want %#v", got, want)
	}
}

// The completion path keeps a multi-byte gap whole when it swaps the word being
// typed, and a line that ends in any gap counts as "a new word starts here".
func TestTrailingWordGapsCount(t *testing.T) {
	if got := replaceLastWord("jv\u3000-f", "file"); got != "jv\u3000file" {
		t.Errorf("replaceLastWord = %q, want %q", got, "jv\u3000file")
	}
	if got := replaceLastWord("jv -f", "file"); got != "jv file" {
		t.Errorf("replaceLastWord = %q, want %q", got, "jv file")
	}
	if got := replaceLastWord("jv", "file"); got != "file" {
		t.Errorf("replaceLastWord = %q, want file", got)
	}

	a := testApp()
	a.Query = "jv -f\u3000"
	if got := a.currentWord(); got != "" {
		t.Errorf("currentWord after a full-width gap = %q, want empty", got)
	}
	tokens := a.commandTokens()
	if len(tokens) == 0 || tokens[len(tokens)-1] != "" {
		t.Errorf("commandTokens after a full-width gap = %#v, want a trailing empty token", tokens)
	}
}
