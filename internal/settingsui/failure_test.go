package settingsui

import (
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// The reason a broken row shows is worded from the dictionary: the code and the
// keyed payload never reach the user, and both languages read the same fact.
func TestFailureReasonWordsABindingDetail(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	for _, tc := range []struct {
		name      string
		code      string
		detail    string
		english   string
		chinese   string
		inEnglish []string
	}{
		{
			name:      "changed",
			code:      extensions.BindingCodeChanged,
			detail:    extensions.BindingChangedDetail("/opt/homebrew/bin/php"),
			english:   "Executable changed: /opt/homebrew/bin/php — possibly a tool upgrade; click Re-check to recover",
			chinese:   "可执行文件已变更：/opt/homebrew/bin/php（可能是工具升级所致，点「重新检测」即可恢复）",
			inEnglish: []string{"/opt/homebrew/bin/php"},
		},
		{
			name:      "missing",
			code:      extensions.BindingCodeMissing,
			detail:    extensions.BindingMissingDetail("/nope/tool"),
			english:   "Executable not found: /nope/tool",
			chinese:   "可执行文件不存在：/nope/tool",
			inEnglish: []string{"/nope/tool"},
		},
		{
			name:      "runtime",
			code:      extensions.BindingCodeRuntimeUnavailable,
			detail:    extensions.RuntimeUnavailableDetail([]string{"node", "nodejs"}),
			english:   "Runtime unavailable: no node / nodejs found on the search path",
			chinese:   "运行时不可用：搜索路径中找不到 node / nodejs",
			inEnglish: []string{"node / nodejs"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := failureReason(en, tc.code, tc.detail); got != tc.english {
				t.Errorf("en = %q, want %q", got, tc.english)
			}
			if got := failureReason(zh, tc.code, tc.detail); got != tc.chinese {
				t.Errorf("zh = %q, want %q", got, tc.chinese)
			}
			// Neither the raw key nor the JSON body survives into the words.
			for _, copy := range []i18n.Settings{en, zh} {
				got := failureReason(copy, tc.code, tc.detail)
				if strings.Contains(got, tc.code) || strings.Contains(got, "{\"") {
					t.Errorf("the reason leaked the key or the payload: %q", got)
				}
			}
		})
	}
}

// A failure recorded before the keyed shape existed is still localised: the old
// build's two English sentences are recognised by their prefix.
func TestFailureReasonMigratesALegacySentence(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	legacy := "Executable fingerprint changed at /usr/local/bin/tool"
	for _, tc := range []struct {
		copy i18n.Settings
		want string
	}{
		{en, "Executable changed: /usr/local/bin/tool — possibly a tool upgrade; click Re-check to recover"},
		{zh, "可执行文件已变更：/usr/local/bin/tool（可能是工具升级所致，点「重新检测」即可恢复）"},
	} {
		if got := failureReason(tc.copy, extensions.BindingCodeChanged, legacy); got != tc.want {
			t.Errorf("reason = %q, want %q", got, tc.want)
		}
	}
	// The legacy English sentence itself is never shown.
	if got := failureReason(zh, extensions.BindingCodeChanged, legacy); strings.Contains(got, "fingerprint changed at") {
		t.Errorf("the stored sentence survived: %q", got)
	}
}

// A detail the dictionary does not know is kept as it is, appended to the
// code's label — swallowing it would hide the only fact there is.
func TestFailureReasonKeepsProseItCannotWord(t *testing.T) {
	en := i18n.For("en").Settings
	if got := failureReason(en, "binding-check-failed", "the script path leaves the package"); got != "Binding check failed · the script path leaves the package" {
		t.Errorf("reason = %q", got)
	}
	// A code with no label of its own is still shown verbatim, never dropped.
	if got := failureReason(en, "some-new-code", "raw detail"); got != "some-new-code · raw detail" {
		t.Errorf("unknown code = %q", got)
	}
	if got := failureReason(en, "probe_failed", ""); got != "Health check failed" {
		t.Errorf("probe reason = %q", got)
	}
	if got := failureReason(en, "", ""); got != "" {
		t.Errorf("empty reason = %q", got)
	}
	if got := failureReason(en, "", "bare detail"); got != "bare detail" {
		t.Errorf("detail only = %q", got)
	}
}

// Every code the repository can record on a binding failure has a label in
// both languages, and each one's sentence is worded rather than echoed: a row
// must never be half rendered.
func TestEveryBindingCodeHasADictionaryLabel(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		copy := i18n.For(language).Settings
		for _, code := range extensions.BindingDetailCodes() {
			if label := copy.ErrorCodeLabel(code); label == "" || label == code {
				t.Errorf("%s has no label for %q (got %q)", language, code, label)
			}
		}
		// The three detail sentences, each with the payload its code carries.
		for _, sentence := range []string{
			copy.BindingChangedDetail("/x"),
			copy.BindingMissingDetail("/x"),
			copy.RuntimeUnavailableDetail("node"),
		} {
			if strings.TrimSpace(sentence) == "" || !strings.Contains(sentence, "/x") && !strings.Contains(sentence, "node") {
				t.Errorf("%s has an empty detail sentence: %q", language, sentence)
			}
		}
	}
}

// The card shows the recorded reason under the old build's label, so a user can
// tell a check from an FYI and read why the integration is unavailable.
func TestTheRowRendersTheRecordedReason(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{
			{
				ID: "dev.floter.broken", Name: "Broken Tool", Enabled: true, Broken: true,
				FailureCode:   extensions.BindingCodeChanged,
				FailureDetail: extensions.BindingChangedDetail("/usr/local/bin/tool"),
			},
			{
				ID: "dev.floter.legacy", Name: "Legacy Tool", Enabled: true, Broken: true,
				FailureCode:   extensions.BindingCodeMissing,
				FailureDetail: "Executable is no longer available at /nope/tool",
			},
			{
				ID: "dev.floter.runtime", Name: "Runtime Tool", Enabled: true, Broken: true,
				FailureCode:   extensions.BindingCodeRuntimeUnavailable,
				FailureDetail: extensions.RuntimeUnavailableDetail([]string{"node", "nodejs"}),
			},
			{
				ID: "dev.floter.prose", Name: "Prose Tool", Enabled: true, Broken: true,
				FailureCode:   extensions.BindingCodeRuntimeUnavailable,
				FailureDetail: "the provider did not answer",
			},
		}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	for _, want := range []string{
		en.IntegrationsFailureDetail,
		"Executable changed: /usr/local/bin/tool — possibly a tool upgrade; click Re-check to recover",
		"Executable not found: /nope/tool",
		"Runtime unavailable: no node / nodejs found on the search path",
		"Runtime unavailable · the provider did not answer",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	for _, unwanted := range []string{
		"Executable is no longer available at /nope/tool",
		"binding-changed",
	} {
		if tt.HasText(unwanted) {
			t.Errorf("the raw form is on screen: %q", unwanted)
		}
	}
}
