package apps

import (
	"os"
	"path/filepath"
	"strings"

	"floter/internal/pinyin"
	"floter/internal/plist"
)

// Reading a macOS bundle's names, so a search finds an application by any of
// the names the user might know it by.
//
// Which name is which matters: the launcher titles a row with the name the
// user sees on their own desktop and subtitles it with the Latin one, so an
// application published for a Chinese audience (whose `Info.plist` holds the
// Chinese name and whose `en.lproj` holds the English one) is findable either
// way. The decision is made by the script each name is written in, not by
// where it was read from, because bundles put them either way round.

// The localization directories read for the Latin name, in order.
// `Base.lproj` is last: it is the development language, which is English for
// most bundles but not all, so a real `en.lproj` is the better answer.
var englishLocales = []string{"en.lproj", "en_US.lproj", "English.lproj", "Base.lproj"}

// The localization directories read for the Chinese name, in order.
var chineseLocales = []string{"zh-Hans.lproj", "zh_CN.lproj", "zh-Hant.lproj", "zh_TW.lproj"}

// bundleInfo is what a scan needs out of one `.app` bundle.
type bundleInfo struct {
	// Name is the Latin spelling, Localized the one the user's desktop shows
	// when it differs.
	Name      string
	Localized string
	// Aliases are the extra spellings search may match: the executable, the
	// identifier's segments, the folder name.
	Aliases []string
	// IconPath is the bundle's icon file, empty when it has none.
	IconPath string
}

// readBundle reads a bundle's names. A bundle without a readable Info.plist
// (or with one this build cannot parse) falls back to its folder name, which
// is what a scan had before it read anything.
func readBundle(path string) bundleInfo {
	fallback := strings.TrimSuffix(filepath.Base(path), ".app")
	if fallback == "" {
		fallback = "Application"
	}
	info := readInfoPlist(filepath.Join(path, "Contents", "Info.plist"))
	bundleName := firstString(info, "CFBundleDisplayName", "CFBundleName")
	if bundleName == "" {
		bundleName = fallback
	}
	name, localized := resolveBundleNames(bundleName,
		localizedName(path, englishLocales), localizedName(path, chineseLocales))

	aliases := []string{bundleName, fallback}
	for _, key := range []string{"CFBundleName", "CFBundleExecutable"} {
		if value := plist.String(info, key); value != "" && value != bundleName {
			aliases = append(aliases, value)
		}
	}
	aliases = append(aliases, identifierAliases(plist.String(info, "CFBundleIdentifier"))...)
	if localized != "" && localized != name {
		aliases = append(aliases, localized)
	}
	iconPath := iconFileIn(filepath.Join(path, "Contents", "Resources"), plist.String(info, "CFBundleIconFile"))
	return bundleInfo{Name: name, Localized: localized, Aliases: uniqueStrings(aliases), IconPath: iconPath}
}

// resolveBundleNames decides which of the names is the Latin one and which the
// localized one.
//
//   - `Safari` with a Chinese localization is the ordinary case: the
//     localization is the title and Safari the subtitle.
//   - 企业微信 ships no Chinese localization *because Chinese is what its
//     Info.plist says*, and its `en.lproj` carries `WeCom`: the bundle's own
//     name is the title and the English one the subtitle.
//   - An application with one name keeps the row to itself.
//
// A localization that turns out to be the Latin name again counts for nothing:
// WeCom ships a `zh-Hant` name of `WeCom`, and taking it at face value would
// leave 企业微信 titled in English on a Chinese desktop.
func resolveBundleNames(bundleName, english, chinese string) (string, string) {
	name := english
	if name == "" {
		name = bundleName
	}
	display := chinese
	if display == "" || display == name {
		display = bundleName
	}
	if hasCJK(name) && !hasCJK(display) {
		name, display = display, name
	}
	if display == name {
		return name, ""
	}
	return name, display
}

// hasCJK reports whether a name is written in Han characters, kana or Hangul —
// the scripts a row is titled with, as against the Latin spelling beneath it.
func hasCJK(value string) bool {
	for _, r := range value {
		switch {
		case r >= 0x2e80 && r <= 0x9fff, // radicals, kana, CJK unified ideographs
			r >= 0xac00 && r <= 0xd7af, // Hangul syllables
			r >= 0xf900 && r <= 0xfaff: // compatibility ideographs
			return true
		}
	}
	return false
}

// localizedName is the bundle's display name in the first of the locales that
// provides one. Only the listed directories are read: an earlier build fell
// back to whichever `.lproj` came first on disk, which handed the launcher a
// German or Japanese name for an application localized into neither language
// the UI speaks.
func localizedName(path string, locales []string) string {
	resources := filepath.Join(path, "Contents", "Resources")
	for _, locale := range locales {
		file := filepath.Join(resources, locale, "InfoPlist.strings")
		if name := readStringsFile(file); name != "" {
			return name
		}
	}
	return ""
}

// readStringsFile reads a bundle's localized names: an `InfoPlist.strings`
// file, which is a property list, or the plain `"key" = "value";` text an
// older bundle may hold.
func readStringsFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if value, err := plist.Parse(data); err == nil {
		if name := firstString(value, "CFBundleDisplayName", "CFBundleName"); name != "" {
			return name
		}
	}
	return stringsValue(string(data), "CFBundleDisplayName", "CFBundleName")
}

// stringsValue pulls a `"key" = "value";` pair out of a .strings file's text,
// for the files that are not property lists.
func stringsValue(text string, keys ...string) string {
	pairs := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "//") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.Trim(strings.TrimSpace(name), "\"")
		value = strings.TrimSuffix(strings.TrimSpace(value), ";")
		pairs[key] = strings.Trim(strings.TrimSpace(value), "\"")
	}
	// The keys are asked for in preference order, so a file carrying both
	// names answers with the display name.
	for _, key := range keys {
		if value := pairs[key]; value != "" {
			return value
		}
	}
	return ""
}

// identifierAliases are the searchable pieces of a bundle identifier:
// `com.apple.Safari` is searched as `Safari` and `apple`, and a two-segment
// identifier keeps everything (`firefox` and `mozilla.firefox` both match
// `mozilla`).
func identifierAliases(identifier string) []string {
	var segments []string
	for _, segment := range strings.Split(identifier, ".") {
		if segment = strings.TrimSpace(segment); segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return nil
	}
	application := segments[len(segments)-1]
	if len(segments) <= 2 {
		return []string{application}
	}
	return []string{application, segments[len(segments)-2]}
}

// readInfoPlist reads a bundle's Info.plist; a missing or unreadable file is
// an empty value, never an error, because a bundle without one is still an
// application the user can open.
func readInfoPlist(path string) any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	value, err := plist.Parse(data)
	if err != nil {
		return nil
	}
	return value
}

// firstString is the first of the keys that holds a non-empty string.
func firstString(value any, keys ...string) string {
	for _, key := range keys {
		if text := plist.String(value, key); text != "" {
			return text
		}
	}
	return ""
}

// computeInitials is the launcher's Latin search key for a name: every ASCII
// letter and digit kept as it is, every CJK character replaced by the first
// letter of its pinyin, and everything else dropped. The original and the
// localized name are folded into one key, because either can be the one the
// user thinks in.
func computeInitials(name, localized string) string {
	combined := name
	if localized != "" {
		combined = name + " " + localized
	}
	return pinyin.Initials(combined)
}

// uniqueStrings drops empty and repeated entries, keeping the order.
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
