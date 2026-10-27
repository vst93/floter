package plist

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"testing"
)

// A property list is one document with two encodings, and a reader must give
// one answer. These pins are the places where this one did not: the same bytes
// written by `plistlib` in either encoding disagreed on the display name and on
// the data blob, and a valid binary document was rejected when its length
// happened to put the offset table on a whitespace byte.

// One document in both encodings, written by Python's plistlib: a name holding
// a non-BMP character (🎮 is a surrogate pair in UTF-16) and a data blob whose
// base64 spelling is `AAFoaQ==`.
const (
	nonBMPBinaryB64 = "YnBsaXN0MDDSAQIDBF8QE0NGQnVuZGxlRGlzcGxheU5hbWVUYmxvYmjYPN+uACAARwBhAG0AZQBzRAABaGkIDSMoOQAAAAAAAAEBAAAAAAAAAAUAAAAAAAAAAAAAAAAAAAA+"
	nonBMPXMLB64    = "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPCFET0NUWVBFIHBsaXN0IFBVQkxJQyAiLS8vQXBwbGUvL0RURCBQTElTVCAxLjAvL0VOIiAiaHR0cDovL3d3dy5hcHBsZS5jb20vRFREcy9Qcm9wZXJ0eUxpc3QtMS4wLmR0ZCI+CjxwbGlzdCB2ZXJzaW9uPSIxLjAiPgo8ZGljdD4KCTxrZXk+Q0ZCdW5kbGVEaXNwbGF5TmFtZTwva2V5PgoJPHN0cmluZz7wn46uIEdhbWVzPC9zdHJpbmc+Cgk8a2V5PmJsb2I8L2tleT4KCTxkYXRhPgoJQUFGb2FRPT0KCTwvZGF0YT4KPC9kaWN0Pgo8L3BsaXN0Pgo="
)

func TestTheSameDocumentReadsTheSameInBothEncodings(t *testing.T) {
	for name, encoded := range map[string]string{"binary": nonBMPBinaryB64, "xml": nonBMPXMLB64} {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		value, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: Parse: %v", name, err)
		}
		// A surrogate pair is one character: 🎮, not two U+FFFD.
		if got := String(value, "CFBundleDisplayName"); got != "🎮 Games" {
			t.Errorf("%s: display name = %q, want %q", name, got, "🎮 Games")
		}
		// `<data>` is base64 in XML and raw bytes in binary; both are the same
		// four bytes here.
		record, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s: value = %T", name, value)
		}
		blob, ok := record["blob"].([]byte)
		if !ok || string(blob) != "\x00\x01hi" {
			t.Errorf("%s: blob = %q, want %q", name, blob, "\x00\x01hi")
		}
	}
}

// binaryDocument assembles a one-object binary property list around an object
// body, padded so the offset table can be made to land on any byte.
func binaryDocument(object []byte, pad int) []byte {
	body := append([]byte("bplist00"), object...)
	body = append(body, make([]byte, pad)...)
	offset := len(body)
	body = append(body, byte(8)) // the object lives at offset 8, one byte wide
	trailer := make([]byte, 6)   // unused
	trailer = append(trailer, 1, 1)
	trailer = binary.BigEndian.AppendUint64(trailer, 1)
	trailer = binary.BigEndian.AppendUint64(trailer, 0)
	trailer = binary.BigEndian.AppendUint64(trailer, uint64(offset))
	return append(body, trailer...)
}

// The binary encoding's header is at offset 0 and its trailer ends with the
// offset table's position: a document whose offset table starts at 0x0a used to
// be rejected, because the trailing newline byte was trimmed as whitespace.
func TestABinaryDocumentIsNotTrimmed(t *testing.T) {
	for _, tc := range []struct {
		pad  int
		last byte
	}{{0, 0x0a}, {4, 0x0e}} {
		document := binaryDocument([]byte{0x10, 0x2a}, tc.pad) // integer 42
		if last := document[len(document)-1]; last != tc.last {
			t.Fatalf("pad %d: last byte = %#x, want %#x", tc.pad, last, tc.last)
		}
		value, err := Parse(document)
		if err != nil {
			t.Fatalf("pad %d (last byte %#x): Parse: %v", tc.pad, document[len(document)-1], err)
		}
		if number, ok := value.(int64); !ok || number != 42 {
			t.Errorf("pad %d: value = %#v, want 42", tc.pad, value)
		}
	}
}

// An integer or real whose width came from the extension marker counts bytes,
// not log2(bytes): the marker's own field is the only place the two agree.
func TestExtendedWidthsReadTheByteCount(t *testing.T) {
	integer := []byte{0x1f, 0x13}
	integer = binary.BigEndian.AppendUint64(integer, 8)
	integer = binary.BigEndian.AppendUint64(integer, 42)
	value, err := Parse(binaryDocument(integer, 0))
	if err != nil {
		t.Fatalf("extended integer: Parse: %v", err)
	}
	if number, ok := value.(int64); !ok || number != 42 {
		t.Errorf("extended integer = %#v, want 42", value)
	}

	real := []byte{0x2f, 0x10, 0x08}
	real = binary.BigEndian.AppendUint64(real, math.Float64bits(1.5))
	value, err = Parse(binaryDocument(real, 0))
	if err != nil {
		t.Fatalf("extended real: Parse: %v", err)
	}
	if number, ok := value.(float64); !ok || number != 1.5 {
		t.Errorf("extended real = %#v, want 1.5", value)
	}
}

// A document that refers to itself, or nests past the limit, is an error: it
// used to recurse until the process died of a stack overflow.
func TestADeepOrSelfReferencingDocumentIsAnError(t *testing.T) {
	// Object 0 is an array holding one reference — to object 0.
	cyclic := []byte{0xa1, 0x00}
	value, err := Parse(binaryDocument(cyclic, 33-len("bplist00")-len(cyclic)))
	if err == nil {
		t.Fatalf("a self-referencing document parsed as %#v", value)
	}
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "nesting") {
		t.Errorf("self-reference error = %v, want ErrInvalid about nesting", err)
	}

	// The XML encoding nests through the same recursion.
	deep := `<?xml version="1.0"?><plist version="1.0">` +
		strings.Repeat("<array>", 200) + strings.Repeat("</array>", 200) + `</plist>`
	if _, err := Parse([]byte(deep)); !errors.Is(err, ErrInvalid) {
		t.Errorf("200 levels: err = %v, want ErrInvalid", err)
	}

	// A document within the limit still reads.
	shallow := `<?xml version="1.0"?><plist version="1.0">` +
		strings.Repeat("<array>", 10) + `<string>ok</string>` +
		strings.Repeat("</array>", 10) + `</plist>`
	value, err = Parse([]byte(shallow))
	if err != nil {
		t.Fatalf("10 levels: Parse: %v", err)
	}
	for i := 0; i < 10; i++ {
		array, ok := value.([]any)
		if !ok || len(array) != 1 {
			t.Fatalf("level %d = %#v", i, value)
		}
		value = array[0]
	}
	if value != "ok" {
		t.Errorf("innermost value = %#v, want \"ok\"", value)
	}
}

// An unpaired surrogate is one replacement character, not a rune the string
// cannot hold: `string(rune(0xD83D))` writes three bytes of mojibake.
func TestAnUnpairedSurrogateIsOneReplacement(t *testing.T) {
	value, err := Parse(binaryDocument([]byte{0x61, 0xd8, 0x3d}, 0))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, ok := value.(string); !ok || got != "\uFFFD" {
		t.Errorf("lone high surrogate = %q (%d runes), want one U+FFFD", got, len([]rune(got)))
	}
}

// `<data>` that is not base64 is a malformed document, not a silent blob of
// text.
func TestXMLDataMustBeBase64(t *testing.T) {
	document := `<?xml version="1.0"?><plist version="1.0"><dict><key>blob</key><data>not base64!!</data></dict></plist>`
	if value, err := Parse([]byte(document)); !errors.Is(err, ErrInvalid) {
		t.Errorf("Parse = %#v, %v, want ErrInvalid", value, err)
	}
	// Line breaks and padding are what a real writer emits, and an empty blob
	// is a blob of no bytes.
	document = `<?xml version="1.0"?><plist version="1.0"><dict><key>a</key><data>` + "\n\tAAFoaQ==\n" + `</data><key>b</key><data></data></dict></plist>`
	value, err := Parse([]byte(document))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	record := value.(map[string]any)
	if blob := record["a"].([]byte); string(blob) != "\x00\x01hi" {
		t.Errorf("wrapped base64 = %q", blob)
	}
	if blob := record["b"].([]byte); len(blob) != 0 {
		t.Errorf("empty data = %q", blob)
	}
}
