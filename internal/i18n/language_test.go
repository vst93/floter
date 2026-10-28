package i18n

import "testing"

// TestLanguageOptionsComeFromTheWhitelist pins the shape of the one language
// table: the picker offers exactly the ids the loader accepts, in the same
// order, each with a label that is its own endonym (so no two languages can
// share one, and neither can be blank).
func TestLanguageOptionsComeFromTheWhitelist(t *testing.T) {
	languages := Languages()
	options := LanguageOptions()
	if len(languages) != len(options) {
		t.Fatalf("Languages() has %d entries, LanguageOptions() has %d", len(languages), len(options))
	}
	if len(languages) == 0 {
		t.Fatal("Languages() is empty")
	}

	seenIDs := map[string]bool{}
	seenLabels := map[string]bool{}
	for i := range languages {
		if options[i].ID != string(languages[i]) {
			t.Errorf("LanguageOptions()[%d].ID = %q, Languages()[%d] = %q", i, options[i].ID, i, languages[i])
		}
		if seenIDs[options[i].ID] {
			t.Errorf("id %q is listed twice", options[i].ID)
		}
		seenIDs[options[i].ID] = true
		if options[i].Label == "" {
			t.Errorf("language %q has no label", options[i].ID)
		}
		if seenLabels[options[i].Label] {
			t.Errorf("label %q is listed twice", options[i].Label)
		}
		seenLabels[options[i].Label] = true
		if got := Normalize(options[i].ID); got != languages[i] {
			t.Errorf("Normalize(%q) = %q, want %q", options[i].ID, got, languages[i])
		}
	}

	if Normalize("") != Default || Normalize("fr") != Default || Normalize("zh-CN") != Default {
		t.Errorf("unknown values must land on the default %q: got %q, %q, %q",
			Default, Normalize(""), Normalize("fr"), Normalize("zh-CN"))
	}
	if !seenIDs[string(Default)] {
		t.Errorf("the default %q is not a shipped language", Default)
	}
}

// TestBothCopiesShowTheSameLanguagePicker pins that the picker's labels are
// endonyms rather than copy: the two language packs must carry the same table.
// They read the same function now, so this only fires if one of them stops.
func TestBothCopiesShowTheSameLanguagePicker(t *testing.T) {
	enOptions := en.Settings.Languages
	zhOptions := zh.Settings.Languages
	if len(enOptions) != len(zhOptions) {
		t.Fatalf("en lists %d languages, zh lists %d", len(enOptions), len(zhOptions))
	}
	for i := range enOptions {
		if enOptions[i] != zhOptions[i] {
			t.Errorf("language option %d differs between copies: en %v, zh %v", i, enOptions[i], zhOptions[i])
		}
	}
}
