package i18n

// This file is the whole answer to "which languages does floter ship, what is
// a stored value normalized to, and what does the settings picker offer".
//
// Before P11-v those three questions had four answers: `settings.languages` was
// the whitelist the loader used, `Normalize` re-answered it with a hardcoded
// `== ZH`, and each of the two Copy values spelled the picker's option table
// out again (`copy.go`, en and zh). They agreed, so nothing was broken — but
// adding a language (the open #692 Japanese question) meant editing four places
// and hoping. One table, read four ways, is the shape that cannot diverge.

// Default is the language an unrecognized stored value lands on.
const Default = EN

// languageTable is the shipped languages in picker order, each paired with its
// own endonym — the name the language gives itself, which is why both Copy
// values show the same labels rather than translating them.
var languageTable = []struct {
	id      Language
	endonym string
}{
	{EN, "English"},
	{ZH, "中文"},
}

// Languages lists the shipped language ids in picker order.
func Languages() []Language {
	out := make([]Language, len(languageTable))
	for i, entry := range languageTable {
		out[i] = entry.id
	}
	return out
}

// LanguageOptions is the settings picker's id/label table, read off the same
// whitelist: the ids are exactly the ones a settings file may store.
func LanguageOptions() []Option {
	out := make([]Option, len(languageTable))
	for i, entry := range languageTable {
		out[i] = Option{ID: string(entry.id), Label: entry.endonym}
	}
	return out
}

// Normalize maps any stored language value onto the whitelist, exactly as
// settings.NormalizeLanguage does ("fr" and "" land on English). Only the exact
// ids select a language — "zh-CN" is not in the table.
func Normalize(language string) Language {
	for _, entry := range languageTable {
		if Language(language) == entry.id {
			return entry.id
		}
	}
	return Default
}
