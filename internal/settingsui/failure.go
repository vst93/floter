package settingsui

import (
	"strings"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// R11 · the readable half of an unavailable runtime binding.
//
// The repository records the fact as a code plus a detail. When the detail is
// keyed (`binding-changed:{"path":…}`) — or one of the two English sentences an
// older build wrote — it is worded from the dictionary, so neither the code nor
// the payload reaches the user untranslated. A detail this module does not
// recognise is kept as it is, appended to the code's own label: that is
// strictly better than swallowing it.
//
// This is the only place the wording lives, mirroring
// `src/extensions/binding-errors.ts` so both dictionaries stay symmetric.
func failureReason(copy i18n.Settings, code, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail != "" {
		if parsed, ok := extensions.BindingFailureDetail(detail); ok {
			return bindingDetailMessage(copy, parsed)
		}
	}
	label := ""
	if strings.TrimSpace(code) != "" {
		label = copy.ErrorCodeLabel(code)
	}
	return joinReason(label, detail)
}

// bindingDetailMessage words one parsed binding detail. Every code in the
// closed set has exactly one sentence.
func bindingDetailMessage(copy i18n.Settings, detail extensions.BindingDetail) string {
	switch detail.Code {
	case extensions.BindingCodeChanged:
		return copy.BindingChangedDetail(detail.Path)
	case extensions.BindingCodeMissing:
		return copy.BindingMissingDetail(detail.Path)
	case extensions.BindingCodeRuntimeUnavailable:
		return copy.RuntimeUnavailableDetail(strings.Join(detail.Names, " / "))
	}
	return ""
}

// joinReason is the old frontend's `[codeLabel, detail].filter(Boolean).join("
// · ")`: the code's label and whatever prose came with it, never a bare
// separator and never a stray leading dot.
func joinReason(label, detail string) string {
	switch {
	case label == "":
		return detail
	case detail == "":
		return label
	default:
		return label + " · " + detail
	}
}
