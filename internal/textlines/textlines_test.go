package textlines

import (
	"reflect"
	"strings"
	"testing"
)

// TestSplitRemovesTheLineFraming pins the two bytes that differ per platform:
// the CR of a CRLF, and the UTF-8 BOM a Windows editor puts at the start of a
// file. Both must be gone before a parser classifies a token.
func TestSplitRemovesTheLineFraming(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"unix lines", "a\nb\n", []string{"a", "b", ""}},
		{"windows lines", "a\r\nb\r\n", []string{"a", "b", ""}},
		{"mixed lines", "a\r\nb\n", []string{"a", "b", ""}},
		{"no trailing newline", "a\r\nb", []string{"a", "b"}},
		{"bom on the first line", "\ufeffa\nb", []string{"a", "b"}},
		{"bom and crlf", "\ufeff[Desktop Entry]\r\nName=x\r\n", []string{"[Desktop Entry]", "Name=x", ""}},
		{"empty blob is one empty line", "", []string{""}},
		{"lone cr is not a line break", "a\rb", []string{"a\rb"}},
		// Only the framing is removed: a second CR, or a BOM that is not at the
		// very start of the blob, is content the parser must keep.
		{"one cr only", "a\r\r\n", []string{"a\r", ""}},
		{"bom mid-file stays", "a\n\ufeffb", []string{"a", "\ufeffb"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Split(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Split(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSplitKeepsCleanTextByteForByte guards the other direction: on the
// platforms and files that never had framing, the result is exactly what
// strings.Split said, so every caller's existing parse stays identical.
func TestSplitKeepsCleanTextByteForByte(t *testing.T) {
	text := "Usage:\n  -n int\n          count\n"
	want := strings.Split(text, "\n")
	if got := Split(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("Split(clean text) = %q, want strings.Split's %q", got, want)
	}
}
