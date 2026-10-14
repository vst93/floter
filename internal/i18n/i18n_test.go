package i18n

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestEveryLanguageIsComplete walks the English copy and requires the
// translation to fill the same field: the struct type already makes a missing
// field a compile error, and this catches one left as the zero string.
func TestEveryLanguageIsComplete(t *testing.T) {
	checkComplete(t, "en", reflect.ValueOf(en), reflect.ValueOf(zh))
	checkNoEmptyStrings(t, "en", reflect.ValueOf(en))
	if got := For("zh").Launcher.Placeholder; got == en.Launcher.Placeholder {
		t.Fatal("For(zh) returned the English copy")
	}
}

// checkNoEmptyStrings fails on a declared copy field the base language never
// filled. `checkComplete` only compares the two languages, so a key empty in
// *both* slipped through — and it did: the calculator command's title, the
// power confirmation's cancel label and the whole uninstall dialog shipped
// blank. An empty English field is a bug, not a translation gap.
func checkNoEmptyStrings(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		next := path + "." + field.Name
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.String:
			if strings.TrimSpace(fv.String()) == "" {
				t.Errorf("%s is declared but empty", next)
			}
		case reflect.Struct:
			checkNoEmptyStrings(t, next, fv)
		case reflect.Slice:
			for j := 0; j < fv.Len(); j++ {
				if fv.Index(j).Kind() == reflect.Struct {
					checkNoEmptyStrings(t, fmt.Sprintf("%s[%d]", next, j), fv.Index(j))
				}
			}
		}
	}
}

func checkComplete(t *testing.T, path string, base, translated reflect.Value) {
	t.Helper()
	for i := 0; i < base.NumField(); i++ {
		field := base.Type().Field(i)
		next := path + "." + field.Name
		b, tr := base.Field(i), translated.Field(i)
		switch b.Kind() {
		case reflect.String:
			if b.String() != "" && strings.TrimSpace(tr.String()) == "" {
				t.Errorf("zh is missing %s", next)
			}
		case reflect.Struct:
			checkComplete(t, next, b, tr)
		case reflect.Slice:
			if b.Len() != tr.Len() {
				t.Errorf("zh.%s has %d entries, want %d", next, tr.Len(), b.Len())
				continue
			}
			for i := 0; i < b.Len(); i++ {
				checkComplete(t, fmt.Sprintf("%s[%d]", next, i), b.Index(i), tr.Index(i))
			}
		case reflect.Func:
			if b.IsNil() {
				t.Errorf("en.%s is nil", next)
			}
			if tr.IsNil() {
				t.Errorf("zh.%s is nil", next)
			}
		}
	}
}

func TestForNormalizesTheLanguage(t *testing.T) {
	// zh-CN is not in the whitelist, so it lands on English; only the exact
	// ids select a language, exactly as the loader normalizes.
	if got := For("zh-CN").Launcher.Placeholder; got != en.Launcher.Placeholder {
		t.Errorf("For(zh-CN) = %q, want the English copy", got)
	}
	if got := For("zh").Launcher.Placeholder; got != "输入命令或应用名称" {
		t.Errorf("zh placeholder = %q", got)
	}
	if got := For("fr").Launcher.Placeholder; got != en.Launcher.Placeholder {
		t.Errorf("fr placeholder = %q, want English", got)
	}
	if got := Normalize("zh"); got != ZH {
		t.Errorf("Normalize(zh) = %q", got)
	}
	if got := Normalize(""); got != EN {
		t.Errorf("Normalize() = %q", got)
	}
}

func TestPercent(t *testing.T) {
	for _, tc := range []struct {
		in   uint8
		want string
	}{{0, "0%"}, {7, "7%"}, {47, "47%"}, {100, "100%"}} {
		if got := en.Settings.Percent(tc.in); got != tc.want {
			t.Errorf("Percent(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestScriptRuntimeWordsMatchTheOldDictionary pins the four script-toolchain
// sentences to the old build's own strings: the checking line is the spinner's
// own caption, the ready line carries no "is
// ready" suffix (the drawer appended the compiled note and path instead), the
// missing line names reopening the form as the way to check again, and the
// compiled note is the drawer's two-word em.
func TestScriptRuntimeWordsMatchTheOldDictionary(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"en checking", en.Settings.LocalToolRuntimeChecking, "Checking the local toolchain…"},
		{"en ready", en.Settings.LocalToolRuntimeReady("python3", "3.12.1"), "python3 3.12.1"},
		{"en ready no version", en.Settings.LocalToolRuntimeReady("node", ""), "node"},
		{"en missing", en.Settings.LocalToolRuntimeMissing("node, nodejs"), "Not found on PATH: node, nodejs. Install it, then reopen this form to check again."},
		{"en compiled", en.Settings.LocalToolRuntimeCompiled, "built before running"},
		{"zh checking", zh.Settings.LocalToolRuntimeChecking, "正在检查本机工具链…"},
		{"zh ready", zh.Settings.LocalToolRuntimeReady("python3", "3.12.1"), "python3 3.12.1"},
		{"zh ready no version", zh.Settings.LocalToolRuntimeReady("node", ""), "node"},
		{"zh missing", zh.Settings.LocalToolRuntimeMissing("node, nodejs"), "在 PATH 中未找到：node, nodejs。安装后重新打开此表单即可再次检查。"},
		{"zh compiled", zh.Settings.LocalToolRuntimeCompiled, "运行前先编译"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestCustomShortcutRejectionWordsMatchTheOldDictionary pins both rejection
// sentences in both languages to the old build's own two words
// (`main:src/i18n.ts:412-413,1299-1300`). The reason is the token the shell
// reports, so `conflict`/`occupied` themselves never reach the reader.
func TestCustomShortcutRejectionWordsMatchTheOldDictionary(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"en conflict", en.Settings.ShortcutsRejected("Cmd+J", "conflict"), "Could not bind Cmd+J: Already used by another floter shortcut"},
		{"en occupied", en.Settings.ShortcutsRejected("Cmd+J", "occupied"), "Could not bind Cmd+J: Most likely used by another app"},
		{"zh conflict", zh.Settings.ShortcutsRejected("Cmd+J", "conflict"), "无法绑定 Cmd+J：已被 floter 的其他快捷键占用"},
		{"zh occupied", zh.Settings.ShortcutsRejected("Cmd+J", "occupied"), "无法绑定 Cmd+J：可能已被其他应用占用"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	// An unknown token has no word of its own, so it is shown verbatim rather
	// than dropped: the reader still learns what the shell said. The two
	// words are absent from the old dictionary, so this is the only branch
	// that may carry a token straight through.
	if got := en.Settings.ShortcutsRejected("Cmd+J", "something-new"); got != "Could not bind Cmd+J: something-new" {
		t.Errorf("unknown reason = %q", got)
	}
	if got := zh.Settings.ShortcutsRejected("Cmd+J", "something-new"); got != "无法绑定 Cmd+J：something-new" {
		t.Errorf("unknown reason (zh) = %q", got)
	}
}

// The terminal bar's two controls and the terminal view's accessible name, as
// the old dictionary spelled them (`main:src/i18n.ts:146-147,149-150,
// 1054-1055,1057-1058`). The hints interpolate the user's own chord, so the
// template is what is pinned here and the binding is the view's business.
func TestTerminalBarWordsMatchTheOldDictionary(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"en input", en.Terminal.Input, "Terminal input"},
		{"en new command", en.Terminal.NewCommand, "New command"},
		{"en new command hint", en.Terminal.NewCommandHint("⌘W"), "New command (⌘W)"},
		{"en open external", en.Terminal.OpenExternal, "Continue in System Terminal"},
		{"en open external hint", en.Terminal.OpenExternalHint("⌘N"), "Continue in System Terminal (⌘N)"},
		{"en header exited", en.Terminal.HeaderExited(3), "Exited · code 3"},
		{"en process exited", en.Terminal.ProcessExited(3), "Process exited · code 3"},
		{"zh input", zh.Terminal.Input, "终端输入"},
		{"zh new command", zh.Terminal.NewCommand, "新建命令"},
		{"zh new command hint", zh.Terminal.NewCommandHint("⌘W"), "新建命令（⌘W）"},
		{"zh open external", zh.Terminal.OpenExternal, "在系统终端中继续"},
		{"zh open external hint", zh.Terminal.OpenExternalHint("⌘N"), "在系统终端中继续（⌘N）"},
		{"zh header exited", zh.Terminal.HeaderExited(3), "已退出 · 退出码 3"},
		{"zh process exited", zh.Terminal.ProcessExited(3), "进程已退出 · 退出码 3"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
