package plist

import (
	"encoding/base64"
	"testing"
	"time"
)

// The two fixtures are the same Info.plist in both encodings, written by
// Python's plistlib: the binary one is what macOS actually writes for a
// bundle, and the XML one is what a hand-made or older bundle holds.
const (
	binaryInfoB64 = "YnBsaXN0MDDaAQIDBAUGBwgJCgsMExQTFRYXGBlfEBNDRkJ1bmRsZURpc3BsYXlOYW1lXxAVQ0ZCdW5kbGVEb2N1bWVudFR5cGVzXxASQ0ZCdW5kbGVFeGVjdXRhYmxlXxASQ0ZCdW5kbGVJZGVudGlmaWVyXENGQnVuZGxlTmFtZV8QD0NGQnVuZGxlVmVyc2lvbl8QFkxTTWluaW11bVN5c3RlbVZlcnNpb25fEBdOU0hpZ2hSZXNvbHV0aW9uQ2FwYWJsZVtTb21lSW50ZWdlclhTb21lUmVhbGVW/l9ifxaPkVZooQ3SDg8QEV8QEENGQnVuZGxlVHlwZU5hbWVfEBJMU0l0ZW1Db250ZW50VHlwZXNVSW1hZ2WhElpwdWJsaWMucG5nWUdyYXBoRWRpdF8QFWNvbS5leGFtcGxlLmdyYXBoZWRpdFMyLjFUMTMuMAkQKiM/+AAAAAAAAAAIAB0AMwBLAGAAdQCCAJQArQDHANMA3ADnAOkA7gEBARYBHAEeASkBMwFLAU8BVAFVAVcAAAAAAAACAQAAAAAAAAAaAAAAAAAAAAAAAAAAAAABYA=="
	xmlInfoB64    = "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPCFET0NUWVBFIHBsaXN0IFBVQkxJQyAiLS8vQXBwbGUvL0RURCBQTElTVCAxLjAvL0VOIiAiaHR0cDovL3d3dy5hcHBsZS5jb20vRFREcy9Qcm9wZXJ0eUxpc3QtMS4wLmR0ZCI+CjxwbGlzdCB2ZXJzaW9uPSIxLjAiPgo8ZGljdD4KCTxrZXk+Q0ZCdW5kbGVEaXNwbGF5TmFtZTwva2V5PgoJPHN0cmluZz7lm77lvaLnvJbovpHlmag8L3N0cmluZz4KCTxrZXk+Q0ZCdW5kbGVEb2N1bWVudFR5cGVzPC9rZXk+Cgk8YXJyYXk+CgkJPGRpY3Q+CgkJCTxrZXk+Q0ZCdW5kbGVUeXBlTmFtZTwva2V5PgoJCQk8c3RyaW5nPkltYWdlPC9zdHJpbmc+CgkJCTxrZXk+TFNJdGVtQ29udGVudFR5cGVzPC9rZXk+CgkJCTxhcnJheT4KCQkJCTxzdHJpbmc+cHVibGljLnBuZzwvc3RyaW5nPgoJCQk8L2FycmF5PgoJCTwvZGljdD4KCTwvYXJyYXk+Cgk8a2V5PkNGQnVuZGxlRXhlY3V0YWJsZTwva2V5PgoJPHN0cmluZz5HcmFwaEVkaXQ8L3N0cmluZz4KCTxrZXk+Q0ZCdW5kbGVJZGVudGlmaWVyPC9rZXk+Cgk8c3RyaW5nPmNvbS5leGFtcGxlLmdyYXBoZWRpdDwvc3RyaW5nPgoJPGtleT5DRkJ1bmRsZU5hbWU8L2tleT4KCTxzdHJpbmc+R3JhcGhFZGl0PC9zdHJpbmc+Cgk8a2V5PkNGQnVuZGxlVmVyc2lvbjwva2V5PgoJPHN0cmluZz4yLjE8L3N0cmluZz4KCTxrZXk+TFNNaW5pbXVtU3lzdGVtVmVyc2lvbjwva2V5PgoJPHN0cmluZz4xMy4wPC9zdHJpbmc+Cgk8a2V5Pk5TSGlnaFJlc29sdXRpb25DYXBhYmxlPC9rZXk+Cgk8dHJ1ZS8+Cgk8a2V5PlNvbWVJbnRlZ2VyPC9rZXk+Cgk8aW50ZWdlcj40MjwvaW50ZWdlcj4KCTxrZXk+U29tZVJlYWw8L2tleT4KCTxyZWFsPjEuNTwvcmVhbD4KPC9kaWN0Pgo8L3BsaXN0Pgo="
)

func fixture(t *testing.T, encoded string) any {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	value, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return value
}

func TestParseBothEncodings(t *testing.T) {
	for _, encoded := range []string{binaryInfoB64, xmlInfoB64} {
		value := fixture(t, encoded)
		if got := String(value, "CFBundleDisplayName"); got != "图形编辑器" {
			t.Errorf("display name = %q", got)
		}
		if got := String(value, "CFBundleName"); got != "GraphEdit" {
			t.Errorf("name = %q", got)
		}
		if got := String(value, "CFBundleIdentifier"); got != "com.example.graphedit" {
			t.Errorf("identifier = %q", got)
		}
		if got := String(value, "CFBundleVersion"); got != "2.1" {
			t.Errorf("version = %q", got)
		}
		record, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("value = %T", value)
		}
		if number, ok := record["SomeInteger"].(int64); !ok || number != 42 {
			t.Errorf("integer = %#v", record["SomeInteger"])
		}
		if number, ok := record["SomeReal"].(float64); !ok || number != 1.5 {
			t.Errorf("real = %#v", record["SomeReal"])
		}
		if flag, ok := record["NSHighResolutionCapable"].(bool); !ok || !flag {
			t.Errorf("bool = %#v", record["NSHighResolutionCapable"])
		}
		types, ok := record["CFBundleDocumentTypes"].([]any)
		if !ok || len(types) != 1 {
			t.Fatalf("array = %#v", record["CFBundleDocumentTypes"])
		}
		nested, ok := types[0].(map[string]any)
		if !ok || String(nested, "CFBundleTypeName") != "Image" {
			t.Errorf("nested dict = %#v", types[0])
		}
		contents, ok := nested["LSItemContentTypes"].([]any)
		if !ok || len(contents) != 1 || contents[0] != "public.png" {
			t.Errorf("nested array = %#v", nested["LSItemContentTypes"])
		}
	}
}

func TestParseRejectsJunk(t *testing.T) {
	for _, data := range []string{"", "not a plist", "bplist00", "<?xml version=\"1.0\"?><plist><dict>", "bplist00" + string(make([]byte, 40))} {
		if value, err := Parse([]byte(data)); err == nil {
			t.Errorf("Parse(%q) = %#v", data, value)
		}
	}
	// A missing key, or one of the wrong type, reads as the empty string.
	value := fixture(t, xmlInfoB64)
	if got := String(value, "Nope"); got != "" {
		t.Errorf("missing key = %q", got)
	}
	if got := String(value, "SomeInteger"); got != "" {
		t.Errorf("non-string key = %q", got)
	}
	if got := String("not a dict", "CFBundleName"); got != "" {
		t.Errorf("non-dict value = %q", got)
	}
}

func TestParseDateAndData(t *testing.T) {
	// Dates are the one value whose encoding differs most: seconds since
	// 2001 in binary, RFC 3339 in XML.
	binaryDate := `<?xml version="1.0"?><plist version="1.0"><dict><key>when</key><date>2024-01-02T03:04:05Z</date><key>blob</key><data>aGk=</data></dict></plist>`
	value, err := Parse([]byte(binaryDate))
	if err != nil {
		t.Fatal(err)
	}
	record := value.(map[string]any)
	when, ok := record["when"].(time.Time)
	if !ok || !when.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("date = %#v", record["when"])
	}
	if blob, ok := record["blob"].([]byte); !ok || string(blob) != "aGk=" {
		t.Errorf("data = %#v", record["blob"])
	}
}
