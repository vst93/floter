//go:build !windows

package apps

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitDesktopExec(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"firefox %u", []string{"firefox"}},
		{"firefox --new-window %U", []string{"firefox", "--new-window"}},
		{"env FOO=bar app", []string{"env", "FOO=bar", "app"}},
		{`sh -c "echo hi" %f`, []string{"sh", "-c", "echo hi"}},
		{"app with space", []string{"app", "with", "space"}},
		{`"/opt/My App/bin" %F`, []string{"/opt/My App/bin"}},
		{"printf 100%%", []string{"printf", "100%"}},
		{"", nil},
		{"   ", nil},
	}
	for _, tc := range cases {
		if got := SplitDesktopExec(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitDesktopExec(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDarwin(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Safari.app", "Contents", "MacOS"), 0o755)
	write(t, filepath.Join(root, "Safari.app", "Contents", "Info.plist"), "<plist/>")
	os.MkdirAll(filepath.Join(root, "Utilities", "Terminal.app"), 0o755)
	// A nested helper bundle inside a bundle must not become its own entry.
	os.MkdirAll(filepath.Join(root, "Safari.app", "Contents", "PlugIns", "Helper.app"), 0o755)
	write(t, filepath.Join(root, "not-an-app.txt"), "x")

	got := ScanDarwin(root)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	want := []string{"Safari", "Terminal"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("ScanDarwin names = %v, want %v", names, want)
	}
	if got[0].Path != filepath.Join(root, "Safari.app") {
		t.Errorf("Safari path = %q", got[0].Path)
	}
}

func TestScanWindows(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "Notepad.lnk"), "")
	write(t, filepath.Join(root, "Accessories", "Paint.lnk"), "")
	write(t, filepath.Join(root, "readme.txt"), "x")

	got := ScanWindows(root)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"Notepad", "Paint"}) {
		t.Errorf("ScanWindows names = %v", names)
	}
}

func TestScanLinux(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "firefox.desktop"), `
[Desktop Entry]
Type=Application
Name=Firefox
Exec=/usr/bin/firefox %u
Icon=firefox
`)
	write(t, filepath.Join(root, "hidden.desktop"), `
[Desktop Entry]
Name=Hidden
Exec=/usr/bin/hidden
Hidden=true
`)
	write(t, filepath.Join(root, "nodisplay.desktop"), `
[Desktop Entry]
Name=No Display
Exec=/usr/bin/nodisplay
NoDisplay=true
`)
	write(t, filepath.Join(root, "noexec.desktop"), `
[Desktop Entry]
Name=No Exec
`)
	write(t, filepath.Join(root, "other-section.desktop"), `
[Desktop Action new]
Name=New Window
Exec=/usr/bin/firefox --new-window
`)
	write(t, filepath.Join(root, "localized.desktop"), `
[Desktop Entry]
Name=Files
Name[zh]=文件
Exec=nautilus %U
`)

	got := ScanLinux(root)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"Files", "Firefox"}) {
		t.Errorf("ScanLinux names = %v", names)
	}
	for _, a := range got {
		if a.Name == "Firefox" {
			if !reflect.DeepEqual(a.Exec, []string{"/usr/bin/firefox"}) {
				t.Errorf("Firefox exec = %#v", a.Exec)
			}
		}
		if a.Name == "Files" {
			// The scan keeps the localized name and folds it into the pinyin
			// initials (the old build filled `initials` from name+localizedName).
			if a.Localized != "文件" {
				t.Errorf("Files localized = %q", a.Localized)
			}
			if a.Initials != "fileswj" {
				t.Errorf("Files initials = %q", a.Initials)
			}
		}
	}
}

// A `.desktop` entry's localized Name/Comment follow the old build's Chinese
// locale ladder (zh_CN > zh_SG > zh > zh_TW > zh_HK); an unlocalized value is
// only a fallback a localized one may override, and a localized name equal to
// the plain one is not a second name.
func TestDesktopLocalizedNameAndComment(t *testing.T) {
	parse := func(t *testing.T, body string) App {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "app.desktop")
		write(t, path, body)
		app, ok := ParseDesktopFile(path)
		if !ok {
			t.Fatalf("ParseDesktopFile rejected %q", body)
		}
		return app
	}

	t.Run("the best Chinese name wins", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Files
Name[zh_TW]=檔案
Name[zh_CN]=文件
Exec=nautilus %U
`)
		if app.Name != "Files" || app.Localized != "文件" {
			t.Errorf("Name=%q Localized=%q", app.Name, app.Localized)
		}
	})

	t.Run("a locale outside the ladder is ignored", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Files
Name[fr]=Fichiers
Exec=nautilus %U
`)
		if app.Localized != "" {
			t.Errorf("Localized=%q, want empty", app.Localized)
		}
	})

	t.Run("a localized name equal to the plain one is dropped", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Files
Name[zh_CN]=Files
Exec=nautilus %U
`)
		if app.Localized != "" {
			t.Errorf("Localized=%q, want empty", app.Localized)
		}
	})

	t.Run("a localized comment overrides the plain one", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Files
Comment=Browse your files
Comment[zh_TW]=瀏覽檔案
Comment[zh_CN]=浏览文件
Exec=nautilus %U
`)
		if app.Comment != "浏览文件" {
			t.Errorf("Comment=%q", app.Comment)
		}
	})

	t.Run("a plain comment stands when nothing localized exists", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Files
Comment=Browse your files
Exec=nautilus %U
`)
		if app.Comment != "Browse your files" {
			t.Errorf("Comment=%q", app.Comment)
		}
	})

	t.Run("an empty comment is dropped", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Files
Comment=
Exec=nautilus %U
`)
		if app.Comment != "" {
			t.Errorf("Comment=%q, want empty", app.Comment)
		}
	})

	t.Run("desktop-entry escapes are unescaped", func(t *testing.T) {
		app := parse(t, `
[Desktop Entry]
Name=Text\sEditor
Comment=Line\nOne
Exec=editor
`)
		if app.Name != "Text Editor" || app.Comment != "Line\nOne" {
			t.Errorf("Name=%q Comment=%q", app.Name, app.Comment)
		}
	})
}

// A Linux `.desktop` entry is searchable by the Latin spellings it advertises
// besides its name: GenericName, StartupWMClass, the program Exec/TryExec run,
// each Keywords word, and the application half of its desktop-file id. The
// original collected exactly these (`linux.rs:423-440`) and filtered them
// through `build_aliases` (`mod.rs:326`).
func TestDesktopSearchAliases(t *testing.T) {
	parse := func(t *testing.T, file, body string) App {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, file)
		write(t, path, body)
		app, ok := ParseDesktopFile(path)
		if !ok {
			t.Fatalf("ParseDesktopFile rejected %q", body)
		}
		return app
	}

	t.Run("every source is collected", func(t *testing.T) {
		app := parse(t, "org.gnome.Nautilus.desktop", `
[Desktop Entry]
Type=Application
Name=Files
Name[zh]=文件
GenericName=File Manager
Keywords=files;manager;folder;文件管理;
StartupWMClass=org.gnome.Nautilus
Exec=/usr/bin/nautilus %U
TryExec=nautilus
`)
		for _, alias := range []string{
			"File Manager",       // GenericName
			"org.gnome.Nautilus", // StartupWMClass
			"nautilus",           // Exec's program
			"manager",            // Keywords
			"folder",             // Keywords
			"gnome",              // desktop-id's vendor
		} {
			if !containsString(app.Aliases, alias) {
				t.Errorf("aliases %v lack %q", app.Aliases, alias)
			}
		}
		// The desktop-id's application half is `Nautilus`, which collapses
		// into the `nautilus` the Exec already contributed (the dedup is
		// case-insensitive, as the original's `eq_ignore_ascii_case`).
		count := 0
		for _, alias := range app.Aliases {
			if strings.EqualFold(alias, "nautilus") {
				count++
			}
		}
		if count != 1 {
			t.Errorf("aliases %v hold %d nautilus spellings, want 1", app.Aliases, count)
		}
		// `Keywords=files` repeats the name the row already shows, and a
		// Chinese keyword has no Latin letter to be reachable by.
		if containsString(app.Aliases, "files") {
			t.Errorf("aliases %v kept the name", app.Aliases)
		}
		if containsString(app.Aliases, "文件管理") {
			t.Errorf("aliases %v kept a non-Latin keyword", app.Aliases)
		}
	})

	t.Run("a candidate equal to a name is dropped and repeats collapse", func(t *testing.T) {
		app := parse(t, "nautilus.desktop", `
[Desktop Entry]
Name=Files
Name[zh]=文件
GenericName=Nautilus
Exec=/usr/bin/nautilus %U
`)
		// `Nautilus` and `nautilus` are the same spelling; the desktop-id
		// stem is `nautilus` again.
		if len(app.Aliases) != 1 || app.Aliases[0] != "Nautilus" {
			t.Errorf("aliases = %v, want [Nautilus]", app.Aliases)
		}
	})

	t.Run("the desktop-id's application half is an alias", func(t *testing.T) {
		app := parse(t, "org.gnome.Nautilus.desktop", `
[Desktop Entry]
Name=Files
Exec=gio
`)
		for _, alias := range []string{"Nautilus", "gnome"} {
			if !containsString(app.Aliases, alias) {
				t.Errorf("aliases %v lack %q", app.Aliases, alias)
			}
		}
	})

	t.Run("executableName strips the path and arguments", func(t *testing.T) {
		if got := executableName([]string{"/usr/bin/google-chrome-stable", "%U"}); got != "google-chrome-stable" {
			t.Errorf("executableName = %q", got)
		}
		if got := executableName(nil); got != "" {
			t.Errorf("executableName(nil) = %q", got)
		}
	})
}

// ScanLinux fills the alias set from the desktop entry it just parsed, so the
// search reaches an application by a name other than the one on its row.
func TestScanLinuxFillsAliases(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "org.gnome.Nautilus.desktop"), `
[Desktop Entry]
Type=Application
Name=文件
Exec=nautilus %U
Keywords=files;folder
`)
	got := ScanLinux(root)
	if len(got) != 1 {
		t.Fatalf("apps = %+v", got)
	}
	for _, alias := range []string{"nautilus", "gnome", "files", "folder"} {
		if !containsString(got[0].Aliases, alias) {
			t.Errorf("aliases %v lack %q", got[0].Aliases, alias)
		}
	}
	// The pinyin initials stay a name-only key: the aliases are already Latin.
	if got[0].Initials != "wj" {
		t.Errorf("initials = %q", got[0].Initials)
	}
}

func TestParseDesktopFileRejectsJunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "junk.desktop")
	write(t, path, "not a desktop file at all")
	if _, ok := ParseDesktopFile(path); ok {
		t.Error("a junk file parsed as an application")
	}
	if _, ok := ParseDesktopFile(filepath.Join(dir, "missing.desktop")); ok {
		t.Error("a missing file parsed as an application")
	}
}

func TestDedupeSortsAndCaps(t *testing.T) {
	in := []App{
		{Name: "zebra", Path: "/z"},
		{Name: "Alpha", Path: "/a"},
		{Name: "alpha", Path: "/a"}, // same path: one survives
		{Name: "beta", Path: "/b"},
	}
	got := dedupe(in)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"Alpha", "beta", "zebra"}) {
		t.Errorf("dedupe = %v", names)
	}
}

// A macOS bundle's names come out of its Info.plist: the Latin spelling is the
// searchable name, the localized one is what a row shows, and the identifier's
// pieces are aliases.
func TestBundleNamesFromInfoPlist(t *testing.T) {
	root := t.TempDir()
	// A bundle whose Info.plist holds the Chinese name (as an application
	// published for a Chinese audience does), with the English one in en.lproj.
	bundle := filepath.Join(root, "企业微信.app")
	write(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0"?>
<plist version="1.0"><dict>
  <key>CFBundleDisplayName</key><string>企业微信</string>
  <key>CFBundleName</key><string>企业微信</string>
  <key>CFBundleExecutable</key><string>WeCom</string>
  <key>CFBundleIdentifier</key><string>com.tencent.WeWorkMac</string>
</dict></plist>`)
	write(t, filepath.Join(bundle, "Contents", "Resources", "en.lproj", "InfoPlist.strings"),
		`"CFBundleDisplayName" = "WeCom";`)
	// A bundle with a Latin name and a Chinese localization: the localization
	// is what a Chinese desktop shows.
	safari := filepath.Join(root, "Safari.app")
	write(t, filepath.Join(safari, "Contents", "Info.plist"), `<?xml version="1.0"?>
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Safari</string>
  <key>CFBundleIdentifier</key><string>com.apple.Safari</string>
</dict></plist>`)
	write(t, filepath.Join(safari, "Contents", "Resources", "zh-Hans.lproj", "InfoPlist.strings"),
		`"CFBundleName" = "Safari 浏览器";`)
	// A bundle with no Info.plist at all falls back to its folder name.
	write(t, filepath.Join(root, "Plain.app", "Contents", "MacOS", "Plain"), "")

	found := map[string]App{}
	for _, app := range ScanDarwin(root) {
		found[filepath.Base(app.Path)] = app
	}
	if len(found) != 3 {
		t.Fatalf("apps = %+v", found)
	}

	wecom := found["企业微信.app"]
	if wecom.Name != "WeCom" || wecom.Localized != "企业微信" {
		t.Errorf("wecom = %+v", wecom)
	}
	for _, alias := range []string{"企业微信", "WeCom", "WeWorkMac", "tencent"} {
		if !containsString(wecom.Aliases, alias) {
			t.Errorf("wecom aliases %v lack %q", wecom.Aliases, alias)
		}
	}

	safariApp := found["Safari.app"]
	if safariApp.Name != "Safari" || safariApp.Localized != "Safari 浏览器" {
		t.Errorf("safari = %+v", safariApp)
	}
	for _, alias := range []string{"Safari", "apple"} {
		if !containsString(safariApp.Aliases, alias) {
			t.Errorf("safari aliases %v lack %q", safariApp.Aliases, alias)
		}
	}

	plain := found["Plain.app"]
	if plain.Name != "Plain" || plain.Localized != "" {
		t.Errorf("plain = %+v", plain)
	}
}

// An application's icon comes from the bundle's CFBundleIconFile (the largest
// PNG in its .icns) or, on Linux, from the desktop entry's Icon=.
func TestIcons(t *testing.T) {
	// A bundle whose icon is named in Info.plist.
	root := t.TempDir()
	bundle := filepath.Join(root, "Iconed.app")
	write(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0"?>
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Iconed</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
</dict></plist>`)
	small := pngBytes(t, 16)
	large := pngBytes(t, 64)
	writeBytes(t, filepath.Join(bundle, "Contents", "Resources", "AppIcon.icns"),
		icnsContainer(t, []icnsEntry{{kind: "icp4", payload: small}, {kind: "ic08", payload: large}}))

	found := ScanDarwin(root)
	if len(found) != 1 {
		t.Fatalf("apps = %+v", found)
	}
	if !strings.HasSuffix(found[0].IconPath, "AppIcon.icns") {
		t.Errorf("icon path = %q", found[0].IconPath)
	}
	if got := Icon(found[0]); !bytes.Equal(got, large) {
		t.Errorf("the largest PNG entry was not chosen: %d bytes", len(got))
	}
	// The second read is the cache, and returns the same bytes.
	if got := Icon(found[0]); !bytes.Equal(got, large) {
		t.Errorf("cached icon = %d bytes", len(got))
	}

	// A bundle with no icon named falls back to the only .icns it has.
	other := filepath.Join(root, "Fallback.app")
	write(t, filepath.Join(other, "Contents", "Info.plist"), `<?xml version="1.0"?><plist version="1.0"><dict/></plist>`)
	writeBytes(t, filepath.Join(other, "Contents", "Resources", "Whatever.icns"),
		icnsContainer(t, []icnsEntry{{kind: "ic07", payload: large}}))
	found = ScanDarwin(root)
	for _, app := range found {
		if app.Name == "Fallback" && !strings.HasSuffix(app.IconPath, "Whatever.icns") {
			t.Errorf("fallback icon path = %q", app.IconPath)
		}
	}

	// A desktop entry's Icon= is resolved: an absolute path is used as it is,
	// and a name is looked for in the icon directories.
	iconPath := filepath.Join(t.TempDir(), "custom.png")
	writeBytes(t, iconPath, large)
	entry := filepath.Join(t.TempDir(), "app.desktop")
	write(t, entry, "[Desktop Entry]\nType=Application\nName=App\nExec=app\nIcon="+iconPath+"\n")
	app, ok := ParseDesktopFile(entry)
	if !ok || app.Icon != iconPath {
		t.Fatalf("desktop icon = %+v", app)
	}
	if got := desktopIconFile(iconPath); got != iconPath {
		t.Errorf("absolute icon resolved to %q", got)
	}
	if got := desktopIconFile("no-such-icon-name-anywhere"); got != "" {
		t.Errorf("an unknown icon name resolved to %q", got)
	}
	if got := desktopIconFile(""); got != "" {
		t.Errorf("an empty icon name resolved to %q", got)
	}
}

func TestICNSRejectsJunk(t *testing.T) {
	for _, data := range [][]byte{
		nil,
		[]byte("not an icon"),
		[]byte("icns\x00\x00\x00\x08"),
		[]byte("icns\xff\xff\xff\xff" + "ic07"),
	} {
		if got := LargestPNGFromICNS(data); got != nil {
			t.Errorf("LargestPNGFromICNS(%q) = %d bytes", data, len(got))
		}
	}
}

// The name decision is made by the script each name is written in, not by
// where it was read from, because bundles put them either way round.
func TestResolveBundleNames(t *testing.T) {
	cases := []struct {
		bundle, english, chinese, name, localized string
	}{
		{"Safari", "Safari", "Safari 浏览器", "Safari", "Safari 浏览器"},
		{"企业微信", "WeCom", "", "WeCom", "企业微信"},
		{"企业微信", "WeCom", "WeCom", "WeCom", "企业微信"},
		{"UU远程", "", "UU Remote", "UU Remote", "UU远程"},
		{"Solo", "", "", "Solo", ""},
		{"Solo", "Solo", "", "Solo", ""},
	}
	for _, tc := range cases {
		name, localized := resolveBundleNames(tc.bundle, tc.english, tc.chinese)
		if name != tc.name || localized != tc.localized {
			t.Errorf("resolveBundleNames(%q, %q, %q) = %q, %q; want %q, %q",
				tc.bundle, tc.english, tc.chinese, name, localized, tc.name, tc.localized)
		}
	}
}

func TestStringsValueAndAliases(t *testing.T) {
	text := `/* comment */
"CFBundleName" = "From strings";
"CFBundleDisplayName" = "Displayed";
`
	if got := stringsValue(text, "CFBundleDisplayName", "CFBundleName"); got != "Displayed" {
		t.Errorf("stringsValue = %q", got)
	}
	if got := stringsValue("nothing here", "CFBundleName"); got != "" {
		t.Errorf("stringsValue on junk = %q", got)
	}

	if got := identifierAliases("com.apple.Safari"); !reflect.DeepEqual(got, []string{"Safari", "apple"}) {
		t.Errorf("identifierAliases = %v", got)
	}
	if got := identifierAliases("firefox"); !reflect.DeepEqual(got, []string{"firefox"}) {
		t.Errorf("identifierAliases(firefox) = %v", got)
	}
	if got := identifierAliases(""); got != nil {
		t.Errorf("identifierAliases(empty) = %v", got)
	}
}

func containsString(list []string, want string) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}

// A bundle's pinyin initials: the Latin search key of a Chinese name, and the
// initials key of the localized name folded into the same string.
func TestComputeInitials(t *testing.T) {
	if got := computeInitials("网易云音乐", ""); got != "wyyyy" {
		t.Errorf("initials = %q", got)
	}
	if got := computeInitials("企业微信", ""); got != "qywx" {
		t.Errorf("initials = %q", got)
	}
	// The localized name is folded into the same key: either can be the one
	// the user thinks in.
	if got := computeInitials("WeCom", "企业微信"); got != "wecomqywx" {
		t.Errorf("folded = %q", got)
	}
	// Separators contribute nothing, so a query typed as one word still
	// matches across them.
	if got := computeInitials("Visual Studio Code", ""); got != "visualstudiocode" {
		t.Errorf("latin = %q", got)
	}
}

// A .strings file in UTF-16 (what Xcode writes for Chinese names) is decoded,
// and its display name is read.
func TestStringsFileUTF16(t *testing.T) {
	dir := t.TempDir()
	utf16 := encodeUTF16LE(`"CFBundleDisplayName" = "图形编辑器";`)
	if err := os.MkdirAll(filepath.Join(dir, "zh.lproj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zh.lproj", "InfoPlist.strings"), utf16, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readStringsFile(filepath.Join(dir, "zh.lproj", "InfoPlist.strings")); got != "图形编辑器" {
		t.Errorf("utf-16 name = %q", got)
	}
	// The unquoted key form is read too.
	write(t, filepath.Join(dir, "en.lproj", "InfoPlist.strings"),
		"CFBundleDisplayName = WeCom;\n")
	if got := readStringsFile(filepath.Join(dir, "en.lproj", "InfoPlist.strings")); got != "WeCom" {
		t.Errorf("unquoted key = %q", got)
	}
	// A comment is stripped rather than read as an entry.
	write(t, filepath.Join(dir, "Base.lproj", "InfoPlist.strings"),
		"/* \"CFBundleDisplayName\" = \"not the name\"; */\n"+
			`"CFBundleDisplayName" = "Real Name";`+"\n")
	if got := readStringsFile(filepath.Join(dir, "Base.lproj", "InfoPlist.strings")); got != "Real Name" {
		t.Errorf("after a comment = %q", got)
	}
}

// encodeUTF16LE is a string's UTF-16LE bytes with the BOM, as Xcode writes.
func encodeUTF16LE(text string) []byte {
	out := []byte{0xff, 0xfe}
	for _, char := range text {
		out = append(out, byte(char), byte(char>>8))
	}
	return out
}

// Only `Application` entries launch: a `Type=Link` or `Type=Directory` entry is
// not an application, while an absent `Type` is kept (the original rejected
// only an explicit non-Application type, linux.rs:395-408).
func TestDesktopEntryTypeGate(t *testing.T) {
	parse := func(t *testing.T, body string) (App, bool) {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "entry.desktop")
		write(t, path, body)
		return ParseDesktopFile(path)
	}

	for _, kind := range []string{"Link", "Directory"} {
		if _, ok := parse(t, "[Desktop Entry]\nType="+kind+"\nName=Thing\nExec=thing\n"); ok {
			t.Errorf("Type=%s parsed as an application", kind)
		}
	}
	if _, ok := parse(t, "[Desktop Entry]\nType=Application\nName=Thing\nExec=thing\n"); !ok {
		t.Error("Type=Application was rejected")
	}
	if _, ok := parse(t, "[Desktop Entry]\nName=Thing\nExec=thing\n"); !ok {
		t.Error("an entry with no Type was rejected")
	}
	// A localized `Type[zh]` is not the type (the original matched only the
	// bare key), so it must not reject the entry.
	if _, ok := parse(t, "[Desktop Entry]\nName=Thing\nExec=thing\nType[zh]=Link\n"); !ok {
		t.Error("Type[zh]=Link rejected an entry")
	}
}

// `executableAvailable` is the `TryExec=` probe: a path must be an executable
// file, a bare name must be on PATH.
func TestExecutableAvailable(t *testing.T) {
	dir := t.TempDir()
	exec := filepath.Join(dir, "nautilus")
	if err := os.WriteFile(exec, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !executableAvailable(exec) {
		t.Errorf("executable file %q not found", exec)
	}
	if executableAvailable(plain) {
		t.Errorf("non-executable file %q accepted", plain)
	}
	if executableAvailable(filepath.Join(dir, "missing")) {
		t.Error("a missing path was accepted")
	}
	if executableAvailable(dir) {
		t.Error("a directory was accepted")
	}

	t.Setenv("PATH", dir)
	if !executableAvailable("nautilus") {
		t.Error("bare name not found on PATH")
	}
	if executableAvailable("ghost") {
		t.Error("a name absent from PATH was accepted")
	}
}

// ScanLinux drops entries whose `TryExec=` program is not installed, but keeps
// the ones that are.
func TestScanLinuxDropsUnavailableTryExec(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "nautilus"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	root := t.TempDir()
	write(t, filepath.Join(root, "installed.desktop"), "[Desktop Entry]\nType=Application\nName=Installed\nExec=nautilus %U\nTryExec=nautilus\n")
	write(t, filepath.Join(root, "ghost.desktop"), "[Desktop Entry]\nType=Application\nName=Ghost\nExec=ghost %U\nTryExec=ghost\n")

	got := ScanLinux(root)
	if len(got) != 1 || got[0].Name != "Installed" {
		t.Fatalf("ScanLinux = %+v, want only Installed", got)
	}
}

// A D-Bus-activatable entry is launched through `gio launch`, so the session
// bus owns it and single-instance activation works; an ordinary entry runs its
// own argv; an app with neither is opened by path.
func TestDesktopLaunchCommand(t *testing.T) {
	dbus := App{Path: "/usr/share/applications/org.gnome.Nautilus.desktop", Exec: []string{"nautilus", "%U"}, DBusActivatable: true}
	if program, args := dbus.launchCommand(); program != "gio" || !reflect.DeepEqual(args, []string{"launch", dbus.Path}) {
		t.Errorf("dbus launchCommand = %q %v", program, args)
	}
	plain := App{Path: "/usr/share/applications/foo.desktop", Exec: []string{"/usr/bin/foo", "--flag"}}
	if program, args := plain.launchCommand(); program != "/usr/bin/foo" || !reflect.DeepEqual(args, []string{"--flag"}) {
		t.Errorf("exec launchCommand = %q %v", program, args)
	}
	bare := App{Path: "/Applications/Thing.app"}
	if program, args := bare.launchCommand(); program != "" || args != nil {
		t.Errorf("path launchCommand = %q %v", program, args)
	}
}

// The app-open path is internal/spawn's, not this package's own copy of it: the
// detachment rules (own session on Unix, no console on Windows, the Windows
// `.cmd`/`.bat` interpreter hop) live in one package, and this is the wiring
// that reaches them — the home directory as the working directory, the entry's
// program and argv, and the `gio` fallback for a D-Bus-activatable entry whose
// `gio` is missing. The port used to restate the detachment here, which is why
// the Windows rules could not reach this path at all.
func TestTheAppLaunchPathIsTheSpawnPackagesOwn(t *testing.T) {
	type call struct {
		dir     string
		program string
		args    []string
	}
	var calls []call
	original := launchProgram
	launchProgram = func(dir, program string, args ...string) error {
		calls = append(calls, call{dir: dir, program: program, args: args})
		if program == "gio" {
			return errors.New("no gio on this machine")
		}
		return nil
	}
	defer func() { launchProgram = original }()

	dbus := App{
		Path:            "/usr/share/applications/org.gnome.Nautilus.desktop",
		Exec:            []string{"nautilus", "--new-window"},
		DBusActivatable: true,
	}
	if err := dbus.Open(); err != nil {
		t.Fatalf("Open of a D-Bus entry whose gio is missing must fall back to Exec, got %v", err)
	}
	want := []call{
		{dir: homeDir(), program: "gio", args: []string{"launch", dbus.Path}},
		{dir: homeDir(), program: "nautilus", args: []string{"--new-window"}},
	}
	if len(calls) != len(want) {
		t.Fatalf("launches = %+v, want %+v", calls, want)
	}
	for i := range want {
		if calls[i].dir != want[i].dir || calls[i].program != want[i].program ||
			!reflect.DeepEqual(calls[i].args, want[i].args) {
			t.Errorf("launch %d = %+v, want %+v", i, calls[i], want[i])
		}
	}

	// An ordinary entry is one launch, in the user's home, with its own argv.
	calls = nil
	plain := App{Path: "/usr/share/applications/foo.desktop", Exec: []string{"/usr/bin/foo", "--flag"}}
	if err := plain.Open(); err != nil {
		t.Fatalf("Open of an Exec entry: %v", err)
	}
	if len(calls) != 1 || calls[0].program != "/usr/bin/foo" || !reflect.DeepEqual(calls[0].args, []string{"--flag"}) {
		t.Errorf("plain launch = %+v, want one launch of /usr/bin/foo --flag", calls)
	}
}
