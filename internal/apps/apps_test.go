package apps

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
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

// icnsEntry is one entry of the container a test builds.
type icnsEntry struct {
	kind    string
	payload []byte
}

// icnsContainer builds an .icns file around the given entries.
func icnsContainer(t *testing.T, entries []icnsEntry) []byte {
	t.Helper()
	var body bytes.Buffer
	for _, entry := range entries {
		if len(entry.kind) != 4 {
			t.Fatalf("kind %q is not four bytes", entry.kind)
		}
		length := make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(entry.payload)+8))
		body.WriteString(entry.kind)
		body.Write(length)
		body.Write(entry.payload)
	}
	out := bytes.NewBufferString("icns")
	total := make([]byte, 4)
	binary.BigEndian.PutUint32(total, uint32(body.Len()+8))
	out.Write(total)
	out.Write(body.Bytes())
	return out.Bytes()
}

// pngBytes is a real PNG of the given square size.
func pngBytes(t *testing.T, size int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
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
