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
