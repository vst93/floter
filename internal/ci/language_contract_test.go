package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// TestLanguageWhitelistHasOneAnswer is the contract `internal/i18n/language.go`
// states: "which languages does floter ship" is one table.
//
// Before P11-v the same question had four answers and they happened to agree:
// `settings.languages` (the slice the loader looped over), `i18n.Normalize`
// (a hardcoded `== ZH`), and the picker's `Languages: []Option{…}` spelled out
// in both Copy values. The one change that makes them disagree is the one
// #692 asks for — adding a language — and then a settings file could hold an id
// the loader accepts and the interface copy renders as English, or a picker
// that offers a language the loader throws away.
//
// The banned shapes are the *tables*, not the ids: a `Language` comparison
// somewhere is fine (`permissions.go` asks `Normalize(language) == ZH`), what
// may not exist outside `internal/i18n` is a second list of them.
func TestLanguageWhitelistHasOneAnswer(t *testing.T) {
	needles := []string{
		`[]string{"en", "zh"}`, // the loader's whitelist slice
		`{"en", "English"}`,    // the picker's option table, English copy
		`{"zh", "中文"}`,         // the picker's option table, Chinese copy
		`Language("en")`,       // a fresh typed literal standing in for EN
		`Language("zh")`,       // … or ZH
	}

	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single table lives here.
				if d.Name() == "i18n" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel := strings.TrimPrefix(filepath.ToSlash(path), "../../")
			for index, line := range strings.Split(string(data), "\n") {
				code := line
				if comment := strings.Index(code, "//"); comment >= 0 {
					code = code[:comment]
				}
				for _, needle := range needles {
					if strings.Contains(code, needle) {
						t.Errorf("%s:%d spells the language whitelist again (%s): read it from i18n.Languages/LanguageOptions/Normalize (see internal/i18n/language.go)",
							rel, index+1, needle)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}

// TestTheTwoLanguagePackagesAgree is the behavioral half: whatever a settings
// file stores, the loader's landing spot and the interface copy's must be the
// same language. `settings` forwards to `i18n` now, so this pins the forwarding
// — a future "just normalize it here" in either package turns it red.
func TestTheTwoLanguagePackagesAgree(t *testing.T) {
	shipped := map[string]bool{}
	for _, language := range i18n.Languages() {
		shipped[string(language)] = true
	}
	if len(shipped) == 0 {
		t.Fatal("i18n.Languages() is empty")
	}

	for _, stored := range []string{"", "en", "zh", "zh-CN", "zh-Hans", "fr", "EN", " zh", "zh "} {
		fromSettings := settings.NormalizeLanguage(stored)
		fromCopy := string(i18n.Normalize(stored))
		if fromSettings != fromCopy {
			t.Errorf("stored %q: settings lands on %q but the copy lands on %q", stored, fromSettings, fromCopy)
		}
		if !shipped[fromSettings] {
			t.Errorf("stored %q: settings lands on %q, which is not a shipped language (%v)", stored, fromSettings, i18n.Languages())
		}
		if got := i18n.For(stored); got.Launcher.Placeholder != i18n.For(fromSettings).Launcher.Placeholder {
			t.Errorf("stored %q renders different copy than its normalized language %q", stored, fromSettings)
		}
	}

	// The picker's ids are exactly the whitelist: a settings file may only hold
	// what the picker can show, and the picker may only show what loads.
	options := i18n.LanguageOptions()
	if len(options) != len(i18n.Languages()) {
		t.Fatalf("LanguageOptions() has %d entries, Languages() has %d", len(options), len(i18n.Languages()))
	}
	for i, language := range i18n.Languages() {
		if options[i].ID != string(language) {
			t.Errorf("LanguageOptions()[%d].ID = %q, Languages()[%d] = %q", i, options[i].ID, i, language)
		}
		if got := settings.NormalizeLanguage(options[i].ID); got != options[i].ID {
			t.Errorf("the picker offers %q but the loader turns it into %q", options[i].ID, got)
		}
	}
}
