package extensions

import (
	"encoding/json"
	"errors"
	"strings"
)

// R11 · the keyed half of an unavailable runtime binding.
//
// Before this, a provider whose executable had been upgraded out from under it
// reached the Integrations page as the backend's English prose — the same
// mistake P7-bd fixed for run failures. Now the fact travels as a **stable key
// plus a JSON payload** (`binding-changed:{"path":"/opt/homebrew/bin/php"}`,
// the old host's `runtime_binding.rs` shape) and the surface owns the words, so
// both dictionaries stay symmetric. `internal/settingsui` renders one with
// `failureReason`; nothing else in the app builds a binding sentence.
//
// The same module recognises the two English sentences an older build wrote
// into `brokenReason` (`Executable fingerprint changed at …`), so a failure
// recorded before this change still renders in the user's language instead of
// as a stale sentence. A detail from neither shape is left alone — the caller
// keeps whatever it already had, which is strictly better than swallowing it.

// The codes a binding detail can carry. `binding-changed` and `binding-missing`
// are the old host's `ProviderErrorCode` values; `runtime-unavailable` is the
// third code that host had a dictionary label for, and is the one this build
// uses for a runtime it could not resolve at all (no interpreter for a script,
// a system tool that is not on the search path).
const (
	BindingCodeChanged            = "binding-changed"
	BindingCodeMissing            = "binding-missing"
	BindingCodeRuntimeUnavailable = "runtime-unavailable"
	// BindingCodeCheckFailed classifies a binding failure that is not one of
	// the named cases (the old host's `failure_code` fallback). Its detail is
	// prose, so it is never keyed.
	BindingCodeCheckFailed = "binding-check-failed"
)

// bindingDetailCodes is the closed set of codes a *keyed* detail may use. Only
// these are ever accepted back; anything else is prose that happens to contain
// a colon.
var bindingDetailCodes = []string{
	BindingCodeChanged,
	BindingCodeMissing,
	BindingCodeRuntimeUnavailable,
}

// BindingDetailCodes returns the closed set of keyed codes, for a caller that
// enumerates them (the i18n symmetry test) without copying the list.
func BindingDetailCodes() []string { return append([]string(nil), bindingDetailCodes...) }

// BindingDetail is a parsed binding failure: the code and the facts its
// sentence interpolates. `Path` is set for the two file codes and `Names` for
// the runtime code; a legacy English sentence carries only `Path`.
type BindingDetail struct {
	Code  string
	Path  string
	Names []string
}

// BindingChangedDetail names the executable whose fingerprint moved.
func BindingChangedDetail(path string) string {
	return keyedDetail(BindingCodeChanged, map[string]any{"path": path})
}

// BindingMissingDetail names the executable that is no longer there.
func BindingMissingDetail(path string) string {
	return keyedDetail(BindingCodeMissing, map[string]any{"path": path})
}

// RuntimeUnavailableDetail names the programs that were looked for and not
// found: the interpreter candidates of a script runtime, or a system
// integration's executable names.
func RuntimeUnavailableDetail(names []string) string {
	copied := append([]string{}, names...)
	return keyedDetail(BindingCodeRuntimeUnavailable, map[string]any{"names": copied})
}

// keyedDetail formats one detail the way the old host did: `code:{json}`. A
// payload that cannot be marshalled falls back to `code:{}` — the code is never
// lost.
func keyedDetail(code string, payload map[string]any) string {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte("{}")
	}
	return code + ":" + string(body)
}

// ParseBindingDetail reads a keyed detail back. It is deliberately
// conservative, exactly as `src/extensions/binding-errors.ts` was: the code
// must be one of the closed set, the remainder must parse as a JSON object, and
// the field that code's sentence interpolates must be present and non-empty. A
// plain sentence, a keyed message from another subsystem (`run_…`), and a
// malformed payload all report false.
func ParseBindingDetail(detail string) (BindingDetail, bool) {
	trimmed := strings.TrimSpace(detail)
	separator := strings.Index(trimmed, ":")
	if separator < 0 {
		return BindingDetail{}, false
	}
	code := trimmed[:separator]
	if !knownBindingDetailCode(code) {
		return BindingDetail{}, false
	}
	body := strings.TrimSpace(trimmed[separator+1:])
	if !strings.HasPrefix(body, "{") {
		return BindingDetail{}, false
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return BindingDetail{}, false
	}
	parsed := BindingDetail{Code: code}
	if path, ok := payload["path"].(string); ok {
		parsed.Path = strings.TrimSpace(path)
	}
	if names, ok := payload["names"].([]any); ok {
		for _, item := range names {
			if name, ok := item.(string); ok && name != "" {
				parsed.Names = append(parsed.Names, name)
			}
		}
	}
	switch code {
	case BindingCodeChanged, BindingCodeMissing:
		if parsed.Path == "" {
			return BindingDetail{}, false
		}
	case BindingCodeRuntimeUnavailable:
		if len(parsed.Names) == 0 {
			return BindingDetail{}, false
		}
	}
	return parsed, true
}

func knownBindingDetailCode(code string) bool {
	for _, candidate := range bindingDetailCodes {
		if candidate == code {
			return true
		}
	}
	return false
}

// legacyBindingPrefixes are the English sentences an older build wrote into
// `brokenReason`, recognised so a persisted pre-R11 failure is still localised.
// The keyed form is preferred; this list exists only for the migration window.
var legacyBindingPrefixes = []struct {
	prefix string
	code   string
}{
	{"Executable fingerprint changed at ", BindingCodeChanged},
	{"Executable is no longer available at ", BindingCodeMissing},
}

// LegacyBindingDetail reads one of the old build's English sentences back into
// a binding detail, or reports false for anything else.
func LegacyBindingDetail(detail string) (BindingDetail, bool) {
	trimmed := strings.TrimSpace(detail)
	for _, entry := range legacyBindingPrefixes {
		rest, ok := strings.CutPrefix(trimmed, entry.prefix)
		if !ok {
			continue
		}
		if path := strings.TrimSpace(rest); path != "" {
			return BindingDetail{Code: entry.code, Path: path}, true
		}
	}
	return BindingDetail{}, false
}

// BindingFailureDetail parses a recorded detail in either spelling: the keyed
// form first, then the legacy sentence.
func BindingFailureDetail(detail string) (BindingDetail, bool) {
	if parsed, ok := ParseBindingDetail(detail); ok {
		return parsed, true
	}
	return LegacyBindingDetail(detail)
}

// BindingFailure is an unavailable runtime: the code the repository records on
// the entry, and the detail a surface renders. The detail is keyed when the
// fact is one a dictionary can word, and the backend's own prose otherwise (a
// check failure keeps the OS's text rather than hiding it behind a sentence).
type BindingFailure struct {
	Code   string
	Detail string
}

// Error makes the failure an error. It is for logs and for the fallback path in
// `failureReason`; no surface prints it as a sentence of its own.
func (f *BindingFailure) Error() string {
	switch {
	case f.Code == "":
		return f.Detail
	case f.Detail == "":
		return "extensions: " + f.Code
	default:
		return "extensions: " + f.Code + ": " + f.Detail
	}
}

// NewBindingFailure builds one.
func NewBindingFailure(code, detail string) *BindingFailure {
	return &BindingFailure{Code: code, Detail: detail}
}

// AsBindingFailure is the typed read of a resolution error: true only when the
// error came from the runtime-binding path and therefore carries a code the
// repository can record.
func AsBindingFailure(err error) (*BindingFailure, bool) {
	var failure *BindingFailure
	if errors.As(err, &failure) {
		return failure, true
	}
	return nil, false
}
