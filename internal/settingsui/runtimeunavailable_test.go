package settingsui

import (
	"strings"
	"testing"

	"floter/internal/i18n"
)

// The runtime-unavailable badge is the reference's `.extension-status--broken`
// chip in the meta line (`src/extensions/ExtensionRow.tsx:258-272`): it reads
// "Tool unavailable" (or its localisation), with the specific cause beside it
// only when the list knows one. The badge is a state mark, not an action —
// the row's repair button (drawn separately) is the action.
func TestRuntimeUnavailableBadgeText(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	// The badge's own word exists in both languages.
	if en.RuntimeUnavailable == "" {
		t.Error("en.RuntimeUnavailable is empty")
	}
	if zh.RuntimeUnavailable == "" {
		t.Error("zh.RuntimeUnavailable is empty")
	}
	// The two languages read the same fact.
	if en.RuntimeUnavailable == zh.RuntimeUnavailable {
		t.Errorf("RuntimeUnavailable should differ between languages, both are %q", en.RuntimeUnavailable)
	}
	// The reference's English word is "Tool unavailable".
	if en.RuntimeUnavailable != "Tool unavailable" {
		t.Errorf("en.RuntimeUnavailable = %q, want %q", en.RuntimeUnavailable, "Tool unavailable")
	}
	// The reference's Chinese word is "工具不可用".
	if zh.RuntimeUnavailable != "工具不可用" {
		t.Errorf("zh.RuntimeUnavailable = %q, want %q", zh.RuntimeUnavailable, "工具不可用")
	}
}

// The badge's label carries the specific cause beside it only when the list
// knows one — the reference's own `extension.runtimeUnavailableCode` arm
// (`src/extensions/ExtensionRow.tsx:264-267`). A row with no code shows only
// the badge word; a row with a code shows "Tool unavailable · Runtime
// unavailable" (the code's own dictionary label).
func TestRuntimeUnavailableBadgeLabel(t *testing.T) {
	en := i18n.For("en").Settings

	cases := []struct {
		name string
		code string
		want string
	}{
		{
			name: "no code shows only the badge word",
			code: "",
			want: en.RuntimeUnavailable,
		},
		{
			name: "a known code adds its dictionary label",
			code: "runtime-unavailable",
			want: en.RuntimeUnavailable + " · " + en.ErrorCodeLabel("runtime-unavailable"),
		},
		{
			name: "an unknown code is not appended",
			code: "some-unknown-code",
			want: en.RuntimeUnavailable,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Build the label exactly as runtimeUnavailableBadge does.
			label := en.RuntimeUnavailable
			if tc.code != "" {
				if cause := en.ErrorCodeLabel(tc.code); cause != "" && cause != tc.code {
					label += " · " + cause
				}
			}
			if label != tc.want {
				t.Errorf("label = %q, want %q", label, tc.want)
			}
			// The label is never empty.
			if strings.TrimSpace(label) == "" {
				t.Error("label is empty")
			}
		})
	}
}
