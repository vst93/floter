// Package desktopentry reads freedesktop `.desktop` files. It is the one
// answer to "what does this entry say": the application scan and the tool
// inventory both ask it, so the same file cannot mean two different things in
// two lists.
//
// The reading follows the original implementation
// (`src-tauri/src/commands/apps/linux.rs` on `main`): the string escapes of the
// Desktop Entry Specification are undone on every string value, the `Name`
// and `Comment` locale ladders are the same five Chinese locales, and an
// `Exec=` line is an argv declaration rather than a split on spaces.
package desktopentry

import (
	"os"
	"path/filepath"
	"strings"

	"floter/internal/textlines"
)

// locales is the locale ladder for an entry's localized `Name` and `Comment`,
// Chinese first so the launcher can show the same localized/original pair it
// shows on macOS. The reference uses the same list.
var locales = []string{"zh_CN", "zh_SG", "zh", "zh_TW", "zh_HK"}

// Entry is what one `.desktop` file says. Empty fields are absent from the
// file; `Localized` is the best `Name[locale]` value, which a caller shows
// beside `Name` rather than instead of it.
type Entry struct {
	Name            string
	Localized       string
	Comment         string
	Exec            string
	Icon            string
	TryExec         string
	GenericName     string
	Keywords        string
	StartupWMClass  string
	Type            string
	DBusActivatable bool
	Hidden          bool
	NoDisplay       bool
}

// Parse reads one `.desktop` file. It reports false only when the file cannot
// be read: whether the entry describes something launchable is `Usable`'s
// question, not the parser's.
func Parse(path string) (Entry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, false
	}
	return ParseContent(string(data)), true
}

// ParseContent reads the text of one `.desktop` file.
func ParseContent(content string) Entry {
	var (
		entry Entry
		// The ranks of the localized values taken so far: the lowest rank
		// seen wins, so an entry that ships both `zh_CN` and `zh_TW` text does
		// not end up mixing the two.
		nameRank    = len(locales)
		commentRank = len(locales)
		commentSet  bool
		inMain      bool
	)
	for _, line := range textlines.Split(content) {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "["):
			// Only the main group describes the application itself; the
			// trailing `[Desktop Action …]` groups are separate entries.
			inMain = line == "[Desktop Entry]"
			continue
		}
		if !inMain {
			continue
		}
		rawKey, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(rawKey)
		value = strings.TrimSpace(value)
		base, locale, hasLocale := strings.Cut(key, "[")
		if hasLocale {
			locale = strings.TrimSuffix(locale, "]")
		}
		switch base {
		case "Name":
			if !hasLocale {
				entry.Name = Unescape(value)
				continue
			}
			if rank := localeRank(locale); rank >= 0 && rank < nameRank {
				nameRank = rank
				entry.Localized = Unescape(value)
			}
		case "Comment":
			if !hasLocale {
				// The untranslated value is the fallback a localized one may
				// override; the first one written is the entry's own.
				if !commentSet {
					entry.Comment = Unescape(value)
					commentSet = true
				}
				continue
			}
			if rank := localeRank(locale); rank >= 0 && rank < commentRank {
				commentRank = rank
				entry.Comment = Unescape(value)
				commentSet = true
			}
		case "Exec":
			if !hasLocale {
				entry.Exec = Unescape(value)
			}
		case "Icon":
			if !hasLocale {
				entry.Icon = value
			}
		case "TryExec":
			if !hasLocale {
				entry.TryExec = Unescape(value)
			}
		// The untranslated spellings only: these are search keys for a Latin
		// keyboard, and the localized variants are already covered by the name
		// and its pinyin initials.
		case "GenericName":
			if !hasLocale {
				entry.GenericName = Unescape(value)
			}
		case "Keywords":
			if !hasLocale {
				entry.Keywords = Unescape(value)
			}
		case "StartupWMClass":
			if !hasLocale {
				entry.StartupWMClass = Unescape(value)
			}
		case "Type":
			if !hasLocale {
				entry.Type = value
			}
		case "DBusActivatable":
			if !hasLocale {
				entry.DBusActivatable = isTrue(value)
			}
		case "Hidden":
			if !hasLocale {
				entry.Hidden = isTrue(value)
			}
		case "NoDisplay":
			if !hasLocale {
				entry.NoDisplay = isTrue(value)
			}
		}
	}
	return entry
}

// isTrue reads a `.desktop` boolean. The specification spells it `true`; the
// comparison is case-insensitive because entries written by hand do not always
// honour that, and a `Hidden=True` line means the same thing.
func isTrue(value string) bool { return strings.EqualFold(value, "true") }

// localeRank is the position of a locale in the ladder, or -1 when the entry's
// locale is not one the launcher translates to.
func localeRank(locale string) int {
	for i, candidate := range locales {
		if candidate == locale {
			return i
		}
	}
	return -1
}

// Unescape undoes the Desktop Entry Specification's string escapes: `\s` is a
// space, `\n`/`\t`/`\r` the control characters, `\\` a backslash. Any other
// escaped character is kept as written, backslash included.
func Unescape(value string) string {
	if !strings.Contains(value, "\\") {
		return value
	}
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			out.WriteByte(value[i])
			continue
		}
		if i+1 >= len(value) {
			out.WriteByte('\\')
			continue
		}
		i++
		switch value[i] {
		case 's':
			out.WriteByte(' ')
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		default:
			out.WriteByte('\\')
			out.WriteByte(value[i])
		}
	}
	return out.String()
}

// Argv splits an `Exec=` value into the argv it declares: whitespace separates,
// double quotes group, a backslash escapes the next character, and the field
// codes the entry interpolates its arguments through are dropped. `%%` is a
// literal percent sign and a code the specification does not define is kept as
// written. An unterminated quote, or a trailing backslash, makes the line
// unusable and yields nothing.
func Argv(exec string) []string {
	var (
		argv    []string
		current strings.Builder
		quoted  bool
	)
	for i := 0; i < len(exec); i++ {
		character := exec[i]
		switch {
		case character == '"':
			quoted = !quoted
		case character == '\\':
			if i+1 >= len(exec) {
				return nil
			}
			i++
			current.WriteByte(exec[i])
		case (character == ' ' || character == '\t') && !quoted:
			if token := stripFieldCodes(current.String()); token != "" {
				argv = append(argv, token)
			}
			current.Reset()
		default:
			current.WriteByte(character)
		}
	}
	if quoted {
		return nil
	}
	if token := stripFieldCodes(current.String()); token != "" {
		argv = append(argv, token)
	}
	return argv
}

// stripFieldCodes drops the `%f`/`%U`/… placeholders a launcher would fill in
// and keeps a literal `%%` as `%`. A code the specification does not define is
// not the launcher's to interpret, so it is passed through as written.
func stripFieldCodes(value string) string {
	if !strings.Contains(value, "%") {
		return value
	}
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			out.WriteByte(value[i])
			continue
		}
		i++
		if i >= len(value) {
			break
		}
		switch value[i] {
		case '%':
			out.WriteByte('%')
		case 'f', 'F', 'u', 'U', 'd', 'D', 'n', 'N', 'i', 'c', 'k', 'v', 'm':
			// A placeholder: the launcher opens applications with no files.
		default:
			out.WriteByte('%')
			out.WriteByte(value[i])
		}
	}
	return out.String()
}

// ArgvFor is the argv of this entry's `Exec=` line.
func (entry Entry) ArgvFor() []string { return Argv(entry.Exec) }

// Usable reports whether the entry describes something the launcher can list
// and run: it names itself, it runs something, it is not the user's deletion
// marker (`Hidden`) or an entry the desktop hides (`NoDisplay`), and its `Type`
// is `Application` — a `Link=` or `Directory=` entry is not something to
// launch. A missing `Type` is an application, as the reference treats it.
func (entry Entry) Usable() bool {
	if entry.Name == "" || entry.Hidden || entry.NoDisplay {
		return false
	}
	if entry.Type != "" && entry.Type != "Application" {
		return false
	}
	return len(entry.ArgvFor()) > 0
}

// FileID is a `.desktop` entry's id: its path below the applications root with
// the separators turned into dashes (`kde4/konsole.desktop` →
// `kde4-konsole.desktop`). It is the key the search path is unique on — the
// same id in two roots is one application — and the reference computes it the
// same way (`desktop_file_id`, `linux.rs:265`).
func FileID(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "" || relative == "." {
		return filepath.Base(path)
	}
	return strings.ReplaceAll(filepath.ToSlash(relative), "/", "-")
}
