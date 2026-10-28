package desktopentry

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// write puts a fixture on disk and returns its path.
func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The fields of one entry are read one way: the Chinese locale ladder picks the
// best `Name[locale]`/`Comment[locale]`, the plain values are the fallback, and
// the specification's string escapes are undone on every string value — the
// reading the reference implementation (`linux.rs`, `DesktopEntry::parse`) does.
func TestTheDesktopEntryFieldsHaveOneReading(t *testing.T) {
	entry := ParseContent(`[Desktop Entry]
Name=Files\sManager
Name=Files
Name[fr]=Fichiers
Name[zh_TW]=檔案
Name[zh_CN]=文件
Comment=Plain comment
Comment[zh_TW]=台灣註解
Comment[zh_CN]=中文註解
Comment[zh_SG]=新加坡註解
Exec=nautilus --new-window %U
Icon=system-file-manager
TryExec=nautilus
GenericName=File\sManager
Keywords=files;folders;
StartupWMClass=Org-Gnome-Nautilus
Type=Application
DBusActivatable=true
Hidden=False
NoDisplay=false

[Desktop Action new-window]
Name=New Window
Exec=nautilus --new-window
`)

	if entry.Name != "Files" {
		t.Errorf("Name = %q, want the last plain value %q", entry.Name, "Files")
	}
	if entry.Localized != "文件" {
		t.Errorf("Localized = %q, want the zh_CN value", entry.Localized)
	}
	if entry.Comment != "中文註解" {
		t.Errorf("Comment = %q, want the best-ranked zh_CN value", entry.Comment)
	}
	if entry.Exec != "nautilus --new-window %U" {
		t.Errorf("Exec = %q", entry.Exec)
	}
	if entry.Icon != "system-file-manager" {
		t.Errorf("Icon = %q", entry.Icon)
	}
	if entry.TryExec != "nautilus" {
		t.Errorf("TryExec = %q", entry.TryExec)
	}
	if entry.GenericName != "File Manager" {
		t.Errorf("GenericName = %q, want the \\s escape undone", entry.GenericName)
	}
	if entry.Keywords != "files;folders;" {
		t.Errorf("Keywords = %q", entry.Keywords)
	}
	if entry.StartupWMClass != "Org-Gnome-Nautilus" {
		t.Errorf("StartupWMClass = %q", entry.StartupWMClass)
	}
	if entry.Type != "Application" || !entry.DBusActivatable {
		t.Errorf("Type = %q, DBusActivatable = %v", entry.Type, entry.DBusActivatable)
	}
	if entry.Hidden || entry.NoDisplay {
		t.Errorf("Hidden = %v, NoDisplay = %v; `False` is not true", entry.Hidden, entry.NoDisplay)
	}
}

// A localized value is only a fallback target for the ladder: a locale the
// launcher does not translate to is ignored, and a plain comment is kept when
// no ladder locale is present.
func TestTheDesktopEntryLocaleLadderIsTheOnlyFallback(t *testing.T) {
	entry := ParseContent("[Desktop Entry]\nName=Plain\nName[fr]=Français\nComment=Plain comment\nComment[fr]=Commentaire\nExec=x\n")
	if entry.Localized != "" {
		t.Errorf("Localized = %q, want nothing for an untranslated locale", entry.Localized)
	}
	if entry.Comment != "Plain comment" {
		t.Errorf("Comment = %q, want the plain value", entry.Comment)
	}
}

// The specification's escapes are undone on every string value, and an escape
// it does not define keeps its backslash.
func TestTheDesktopEntryEscapesAreUndone(t *testing.T) {
	for _, testCase := range []struct {
		in   string
		want string
	}{
		{"a\\sb", "a b"},
		{"a\\nb", "a\nb"},
		{"a\\tb", "a\tb"},
		{"a\\rb", "a\rb"},
		{"a\\\\b", "a\\b"},
		{"a\\qb", "a\\qb"},
		{"a\\", "a\\"},
		{"plain", "plain"},
	} {
		if got := Unescape(testCase.in); got != testCase.want {
			t.Errorf("Unescape(%q) = %q, want %q", testCase.in, got, testCase.want)
		}
	}
}

// An Exec line is an argv declaration: whitespace separates, double quotes
// group, a backslash escapes, the field codes are dropped, an unknown code is
// kept as written, and an unterminated quote yields nothing. A single quote is
// an ordinary character — the specification gives it no meaning.
func TestTheDesktopEntryExecIsAnArgv(t *testing.T) {
	for _, testCase := range []struct {
		name string
		in   string
		want []string
	}{
		{"a bare program", "rg %F", []string{"rg"}},
		{"a quoted path", `"/opt/Some App/app" %U`, []string{"/opt/Some App/app"}},
		{"a backslash escapes the next character", `my\sapp %f`, []string{"mysapp"}},
		{"a literal percent", "printf 100%%", []string{"printf", "100%"}},
		{"every defined field code", "x %f %F %u %U %d %D %n %N %i %c %k %v %m", []string{"x"}},
		{"an unknown field code", "rg %z", []string{"rg", "%z"}},
		{"a single quote is a character", "it's --flag", []string{"it's", "--flag"}},
		{"an empty quoted argument is dropped", `rg ""`, []string{"rg"}},
		{"an unterminated quote", `rg "unclosed`, nil},
		{"a trailing backslash", `rg \`, nil},
		{"blank", "   ", nil},
	} {
		if got := Argv(testCase.in); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s: Argv(%q) = %#v, want %#v", testCase.name, testCase.in, got, testCase.want)
		}
	}
}

// `Exec=` is a string value, so its escapes are undone before it is split: a
// `\s` is a space the argv separator sees.
func TestTheDesktopEntryExecIsUnescapedBeforeItIsSplit(t *testing.T) {
	entry := ParseContent(`[Desktop Entry]
Name=Editor
Exec=/opt/Some\sApp/editor --new-window
`)
	if got, want := entry.ArgvFor(), []string{"/opt/Some", "App/editor", "--new-window"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %#v, want %#v", got, want)
	}
}

// What the launcher can list and run: a name, something to run, not the user's
// deletion marker, not an entry the desktop hides, and an application type.
func TestTheDesktopEntryUsableRule(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		content string
		want    bool
	}{
		{"an application", "[Desktop Entry]\nName=A\nExec=a\nType=Application\n", true},
		{"a missing Type is an application", "[Desktop Entry]\nName=A\nExec=a\n", true},
		{"no Name", "[Desktop Entry]\nExec=a\nType=Application\n", false},
		{"no Exec", "[Desktop Entry]\nName=A\nType=Application\n", false},
		{"an Exec with no program", "[Desktop Entry]\nName=A\nExec=%f\nType=Application\n", false},
		{"Hidden", "[Desktop Entry]\nName=A\nExec=a\nHidden=True\n", false},
		{"NoDisplay", "[Desktop Entry]\nName=A\nExec=a\nNoDisplay=true\n", false},
		{"a link", "[Desktop Entry]\nName=A\nExec=xdg-open https://example.com\nType=Link\n", false},
	} {
		if got := ParseContent(testCase.content).Usable(); got != testCase.want {
			t.Errorf("%s: Usable = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

// Parse reports false only for a file it cannot read; whether the entry is
// launchable is a separate question.
func TestParseReportsOnlyUnreadableFiles(t *testing.T) {
	if _, ok := Parse(filepath.Join(t.TempDir(), "absent.desktop")); ok {
		t.Error("a missing file parsed as an entry")
	}
	path := write(t, "empty.desktop", "")
	entry, ok := Parse(path)
	if !ok {
		t.Fatal("an empty file did not parse")
	}
	if entry.Usable() {
		t.Error("an empty file is not a usable entry")
	}
}

// A desktop-file id is the path below the root with the separators flattened,
// and a path that is not below the root is its own id.
func TestTheDesktopEntryFileID(t *testing.T) {
	root := filepath.Join("usr", "share", "applications")
	if got := FileID(root, filepath.Join(root, "kde4", "konsole.desktop")); got != "kde4-konsole.desktop" {
		t.Errorf("FileID = %q, want kde4-konsole.desktop", got)
	}
	if got := FileID(root, filepath.Join(root, "gimp.desktop")); got != "gimp.desktop" {
		t.Errorf("FileID = %q, want gimp.desktop", got)
	}
	// The root itself has no relative path to flatten.
	if got := FileID(root, root); got != filepath.Base(root) {
		t.Errorf("FileID = %q, want %q", got, filepath.Base(root))
	}
}
