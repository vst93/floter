//go:build !windows

package extensions

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A keyed binding detail is the shape the old host sent and the shape the
// settings surface parses back: the code, then a JSON payload naming the file.
func TestBindingDetailRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		detail string
		code   string
		path   string
		names  []string
	}{
		{BindingChangedDetail("/opt/homebrew/bin/php"), BindingCodeChanged, "/opt/homebrew/bin/php", nil},
		{BindingMissingDetail("/nope/tool"), BindingCodeMissing, "/nope/tool", nil},
		{RuntimeUnavailableDetail([]string{"node", "pwsh"}), BindingCodeRuntimeUnavailable, "", []string{"node", "pwsh"}},
	} {
		if !strings.HasPrefix(tc.detail, tc.code+":{") {
			t.Errorf("detail %q is not keyed", tc.detail)
		}
		parsed, ok := ParseBindingDetail(tc.detail)
		if !ok {
			t.Fatalf("%q did not parse", tc.detail)
		}
		if parsed.Code != tc.code || parsed.Path != tc.path {
			t.Errorf("%q parsed to %+v", tc.detail, parsed)
		}
		if !reflect.DeepEqual(parsed.Names, tc.names) {
			t.Errorf("%q names = %v, want %v", tc.detail, parsed.Names, tc.names)
		}
	}
}

// The parser claims only its own shape. A sentence, another subsystem's keyed
// message, a missing payload field and malformed JSON all stay prose.
func TestParseBindingDetailIsConservative(t *testing.T) {
	for _, detail := range []string{
		"",
		"Executable fingerprint changed at /opt/homebrew/bin/php",
		"run_timeout:{\"seconds\":30}",
		"probe_failed:{\"probe\":\"version\"}",
		"binding-missing",
		"binding-missing:",
		"binding-missing:not json",
		"binding-missing:{",
		"binding-missing:[1,2]",
		"binding-missing:{\"path\":\"\"}",
		"binding-missing:{\"path\":42}",
		"binding-changed:{\"other\":\"/x\"}",
		"runtime-unavailable:{\"names\":[]}",
		"unknown-code:{\"path\":\"/x\"}",
	} {
		if parsed, ok := ParseBindingDetail(detail); ok {
			t.Errorf("ParseBindingDetail(%q) = %+v, want prose", detail, parsed)
		}
	}
}

// The two English sentences an older build persisted are read back into the
// same shape, so a failure recorded before the keys existed still localises.
func TestLegacyBindingDetail(t *testing.T) {
	for _, tc := range []struct {
		detail string
		code   string
		path   string
	}{
		{"Executable fingerprint changed at /opt/homebrew/bin/php", BindingCodeChanged, "/opt/homebrew/bin/php"},
		{"  Executable is no longer available at /nope/tool  ", BindingCodeMissing, "/nope/tool"},
	} {
		parsed, ok := LegacyBindingDetail(tc.detail)
		if !ok {
			t.Fatalf("LegacyBindingDetail(%q) did not parse", tc.detail)
		}
		if parsed.Code != tc.code || parsed.Path != tc.path {
			t.Errorf("%q = %+v", tc.detail, parsed)
		}
		if message, ok := BindingFailureDetail(tc.detail); !ok || message.Code != tc.code {
			t.Errorf("BindingFailureDetail(%q) = %+v, %v", tc.detail, message, ok)
		}
	}
	for _, detail := range []string{
		"",
		"Executable disappeared at /nope/tool",
		"Executable fingerprint changed at ",
		"Provider describe failed: exit 2",
	} {
		if parsed, ok := LegacyBindingDetail(detail); ok {
			t.Errorf("LegacyBindingDetail(%q) = %+v, want prose", detail, parsed)
		}
	}
}

// The keyed form wins over the legacy sentence when both could answer.
func TestBindingFailureDetailPrefersTheKey(t *testing.T) {
	keyed := BindingMissingDetail("/nope/tool")
	parsed, ok := BindingFailureDetail(keyed)
	if !ok || parsed.Path != "/nope/tool" {
		t.Fatalf("keyed detail = %+v, %v", parsed, ok)
	}
}

// The two lookup seams the resolution uses are stubbed, so the test never reads
// this machine's toolchain.
func TestResolveRuntimeReportsKeyedBindingFailures(t *testing.T) {
	defer StubToolLookup(func(names ...string) (string, bool) { return "", false })()
	defer StubInterpreterLookup(func(language string) (string, bool) { return "", false })()

	// A recorded executable that is gone: the code names the file.
	system := Integration{
		Entry: Entry{ID: "dev.floter.sys", ExecutablePath: filepath.Join(t.TempDir(), "gone")},
		Manifest: Manifest{
			ID:      "dev.floter.sys",
			Runtime: Runtime{Type: "system", ExecutableNames: []string{"gone"}},
		},
	}
	_, err := ResolveRuntime(system)
	failure, ok := AsBindingFailure(err)
	if !ok {
		t.Fatalf("system resolution = %v, want a binding failure", err)
	}
	if failure.Code != BindingCodeMissing {
		t.Errorf("code = %q", failure.Code)
	}
	parsed, ok := ParseBindingDetail(failure.Detail)
	if !ok || parsed.Path != system.Entry.ExecutablePath {
		t.Errorf("detail = %q (%+v, %v)", failure.Detail, parsed, ok)
	}

	// A system runtime with no recorded path and nothing on the search path:
	// the searched names travel, so the sentence can say what was looked for.
	system.Entry.ExecutablePath = ""
	system.Manifest.Runtime.ExecutableNames = []string{"php", "php8"}
	_, err = ResolveRuntime(system)
	failure, ok = AsBindingFailure(err)
	if !ok {
		t.Fatalf("unrecorded system resolution = %v", err)
	}
	if failure.Code != BindingCodeRuntimeUnavailable {
		t.Errorf("code = %q", failure.Code)
	}
	parsed, ok = ParseBindingDetail(failure.Detail)
	if !ok || !reflect.DeepEqual(parsed.Names, []string{"php", "php8"}) {
		t.Errorf("detail = %q (%+v, %v)", failure.Detail, parsed, ok)
	}

	// A script runtime with no interpreter lists that language's candidates.
	script := Integration{
		Entry: Entry{ID: "dev.floter.script"},
		Manifest: Manifest{
			ID:      "dev.floter.script",
			Runtime: Runtime{Type: "script", Language: "python", Path: "tool.py"},
		},
	}
	_, err = ResolveRuntime(script)
	failure, ok = AsBindingFailure(err)
	if !ok {
		t.Fatalf("script resolution = %v", err)
	}
	if failure.Code != BindingCodeRuntimeUnavailable {
		t.Errorf("code = %q", failure.Code)
	}
	parsed, ok = ParseBindingDetail(failure.Detail)
	if !ok || len(parsed.Names) == 0 || parsed.Names[0] != "python3" {
		t.Errorf("detail = %q (%+v, %v)", failure.Detail, parsed, ok)
	}

	// A structural failure is classified rather than named: it gets the old
	// host's fallback code with the underlying prose, which is not keyed.
	unknown := Integration{
		Entry:    Entry{ID: "dev.floter.unknown"},
		Manifest: Manifest{ID: "dev.floter.unknown", Runtime: Runtime{Type: "cargo"}},
	}
	_, err = ResolveRuntime(unknown)
	failure, ok = AsBindingFailure(err)
	if !ok || failure.Code != BindingCodeCheckFailed {
		t.Fatalf("unknown runtime = %v (%+v, %v)", err, failure, ok)
	}
	if _, keyed := ParseBindingDetail(failure.Detail); keyed {
		t.Errorf("a check failure must keep its prose: %q", failure.Detail)
	}
	if !strings.Contains(failure.Detail, "cargo") {
		t.Errorf("detail = %q", failure.Detail)
	}
}

// AsBindingFailure is typed: a plain error is not claimed.
func TestAsBindingFailureOnlyClaimsItsOwn(t *testing.T) {
	if _, ok := AsBindingFailure(errors.New("extensions: nope")); ok {
		t.Error("a plain error was claimed")
	}
	wrapped := NewBindingFailure(BindingCodeMissing, BindingMissingDetail("/x"))
	if failure, ok := AsBindingFailure(wrapped); !ok || failure.Code != BindingCodeMissing {
		t.Errorf("wrapped = %+v, %v", failure, ok)
	}
	if got := wrapped.Error(); !strings.Contains(got, BindingCodeMissing) || !strings.Contains(got, "/x") {
		t.Errorf("Error() = %q", got)
	}
}

// Every keyed code has a dictionary label: a row's reason must never be half
// rendered. The other half (the sentences) is pinned by the i18n test.
func TestEveryBindingCodeHasALabel(t *testing.T) {
	for _, code := range BindingDetailCodes() {
		if !knownBindingDetailCode(code) {
			t.Errorf("%q is not in the closed set", code)
		}
	}
	if len(BindingDetailCodes()) != 3 {
		t.Errorf("codes = %v", BindingDetailCodes())
	}
	// The returned slice is a copy: a caller cannot unset the closed set.
	BindingDetailCodes()[0] = "tampered"
	if BindingDetailCodes()[0] != BindingCodeChanged {
		t.Error("BindingDetailCodes leaked its backing array")
	}
}
