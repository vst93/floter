// Package launcher draws the P0 launcher shell: a search field over an
// empty result area with a hint. It has no search, no keyboard navigation
// and no action on Enter yet — P1 fills that in.
package launcher

// Strings is the launcher's copy for one language. The placeholder and the
// label match src/i18n.ts (`input.placeholder`); the empty-state hint is
// this round's.
type Strings struct {
	// Hint is the empty result area's message.
	Hint string
	// Placeholder is the search field's placeholder.
	Placeholder string
	// Label names the field for assistive technology.
	Label string
}

// StringsFor returns the copy for a language, normalized the same way the
// settings loader does. An unknown language falls back to English.
func StringsFor(language string) Strings {
	switch language {
	case "zh":
		return Strings{
			Hint:        "输入以搜索",
			Placeholder: "输入命令或应用名称",
			Label:       "搜索",
		}
	default:
		return Strings{
			Hint:        "Type to search",
			Placeholder: "Type a command or app name",
			Label:       "Search",
		}
	}
}
