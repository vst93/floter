package apps

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

// buildShortcut assembles a minimal `.lnk`: the 0x4C-byte header, an optional
// item-id list, and a LinkInfo holding the target. It is the bytes
// `ScanWindows` reads, so the whole Windows read path can be exercised on any
// host.
//
// `unicodePath` writes the target as UTF-16 (the shape every modern Windows
// version produces); an empty one leaves the ANSI path alone, which is the
// older shape the reader also has to answer.
func buildShortcut(ansiPath, unicodePath, suffix string) []byte {
	linkInfoHeaderSize := uint32(0x1C)
	if unicodePath != "" {
		linkInfoHeaderSize = 0x24
	}
	// LinkInfo header (0x1C bytes of fields, padded to headerSize), then the
	// two strings.
	infoHeader := make([]byte, linkInfoHeaderSize)
	var body []byte
	ansiOffset := uint32(linkInfoHeaderSize)
	body = append(body, []byte(ansiPath)...)
	body = append(body, 0)
	var ansiSuffixOffset uint32 = 0
	if suffix != "" {
		ansiSuffixOffset = ansiOffset + uint32(len(ansiPath)) + 1
		body = append(body, []byte(suffix)...)
		body = append(body, 0)
	}
	var unicodeBaseOffset, unicodeSuffixOffset uint32
	if unicodePath != "" {
		unicodeBaseOffset = uint32(linkInfoHeaderSize) + uint32(len(body))
		for _, unit := range utf16.Encode([]rune(unicodePath)) {
			var bytes [2]byte
			binary.LittleEndian.PutUint16(bytes[:], unit)
			body = append(body, bytes[:]...)
		}
		body = append(body, 0, 0)
		if suffix != "" {
			unicodeSuffixOffset = unicodeBaseOffset + uint32(len(utf16.Encode([]rune(unicodePath)))*2) + 2
			for _, unit := range utf16.Encode([]rune(suffix)) {
				var bytes [2]byte
				binary.LittleEndian.PutUint16(bytes[:], unit)
				body = append(body, bytes[:]...)
			}
			body = append(body, 0, 0)
		}
	}

	info := make([]byte, 0, len(infoHeader)+len(body))
	info = append(info, infoHeader...)
	info = append(info, body...)
	binary.LittleEndian.PutUint32(info[0:], uint32(len(info)))     // LinkInfoSize
	binary.LittleEndian.PutUint32(info[4:], linkInfoHeaderSize)    // LinkInfoHeaderSize
	binary.LittleEndian.PutUint32(info[8:], lnkInfoVolumeAndLocal) // LinkInfoFlags
	binary.LittleEndian.PutUint32(info[16:], ansiOffset)           // LocalBasePathOffset
	binary.LittleEndian.PutUint32(info[24:], ansiSuffixOffset)     // CommonPathSuffixOffset
	if linkInfoHeaderSize >= lnkInfoHeaderUnicode {
		binary.LittleEndian.PutUint32(info[lnkInfoUnicodeBase:], unicodeBaseOffset)
		binary.LittleEndian.PutUint32(info[lnkInfoUnicodeSuffix:], unicodeSuffixOffset)
	}

	header := make([]byte, lnkHeaderSize)
	binary.LittleEndian.PutUint32(header[0:], lnkHeaderSize) // HeaderSize
	binary.LittleEndian.PutUint32(header[20:], lnkFlagHasLinkInfo)
	return append(header, info...)
}

func writeShortcut(t *testing.T, dir, name string, bytes []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAShortcutTargetIsRead(t *testing.T) {
	path := writeShortcut(t, t.TempDir(), "WeChat.lnk",
		buildShortcut(`C:\Program Files\WXWork\WXWork.exe`, "", ""))
	target, ok := ShortcutTarget(path)
	if !ok || target != `C:\Program Files\WXWork\WXWork.exe` {
		t.Fatalf("target = %q (ok=%v)", target, ok)
	}
}

func TestTheUnicodePathWins(t *testing.T) {
	// 企业微信's own shortcut: the unicode path is the one Windows wrote, and
	// the ANSI one is lossy in a non-Chinese code page.
	path := writeShortcut(t, t.TempDir(), "企微.lnk",
		buildShortcut(`C:\PROGRA~1\WXWork\WXWork.exe`, `C:\Program Files\企微\企微.exe`, ""))
	target, ok := ShortcutTarget(path)
	if !ok || target != `C:\Program Files\企微\企微.exe` {
		t.Fatalf("target = %q (ok=%v)", target, ok)
	}
}

func TestASuffixIsPartOfTheTarget(t *testing.T) {
	path := writeShortcut(t, t.TempDir(), "App.lnk",
		buildShortcut(`C:\app\`, "", `app.exe`))
	target, ok := ShortcutTarget(path)
	if !ok || target != `C:\app\app.exe` {
		t.Fatalf("target = %q (ok=%v)", target, ok)
	}
}

func TestAnUnreadableShortcutKeepsItsRow(t *testing.T) {
	dir := t.TempDir()
	// Not a shortcut at all; a shortcut whose target lives in an item-id list
	// only (no LinkInfo); and a truncated one.
	writeShortcut(t, dir, "Empty.lnk", []byte("not a shortcut"))
	headerOnly := make([]byte, lnkHeaderSize)
	binary.LittleEndian.PutUint32(headerOnly[0:], lnkHeaderSize)
	binary.LittleEndian.PutUint32(headerOnly[20:], lnkFlagHasIDList)
	writeShortcut(t, dir, "IDList.lnk", headerOnly)

	for _, name := range []string{"Empty.lnk", "IDList.lnk"} {
		if target, ok := ShortcutTarget(filepath.Join(dir, name)); ok {
			t.Errorf("%s answered %q, want no target", name, target)
		}
	}
	// And they are still rows: Windows resolves them itself when opened.
	found := ScanWindows(dir)
	if len(found) != 2 {
		t.Fatalf("ScanWindows kept %d rows, want 2: %v", len(found), names(found))
	}
}

func TestUninstallersAreNotRows(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{`C:\Program Files\App\App.exe`, true},
		{`C:\Program Files\App\app.EXE`, true},
		{`C:\app\run.bat`, true},
		{`C:\app\run.cmd`, true},
		{`C:\Program Files\App\unins000.exe`, false},
		{`C:\Program Files\App\Uninstall.exe`, false},
		{`C:\Program Files\App\uninst.exe`, false},
		{`C:\app\readme.txt`, false},
		{`C:\app\app.exe,0`, false}, // an icon index is not a path
		{`https://example.com`, false},
	}
	for _, tc := range cases {
		if got := IsLaunchableTarget(tc.target); got != tc.want {
			t.Errorf("IsLaunchableTarget(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}

func TestTheTargetNameIsTheAlias(t *testing.T) {
	cases := map[string]string{
		`C:\Program Files\WXWork\WXWork.exe`: "WXWork",
		`C:\app\run.bat`:                     "run",
		`C:\app\no-extension`:                "",
		`C:\app\.hidden`:                     "",
	}
	for target, want := range cases {
		if got := ShortcutTargetName(target); got != want {
			t.Errorf("ShortcutTargetName(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestScanWindowsDerivesWhatTheTargetSays(t *testing.T) {
	root := t.TempDir()
	// A Chinese-named shortcut whose program has a Latin name: the executable
	// is the alias that keeps it typeable, and the name's pinyin initials are
	// its Chinese front door.
	writeShortcut(t, root, "企业微信.lnk",
		buildShortcut(`C:\Program Files\WXWork\WXWork.exe`, "", ""))
	// Two shortcuts, one program: one row.
	writeShortcut(t, root, "WeCom.lnk",
		buildShortcut(`C:\Program Files\WXWork\WXWork.exe`, "", ""))
	// The Start Menu's own installer leftovers are not applications.
	writeShortcut(t, root, "Uninstall WeCom.lnk",
		buildShortcut(`C:\Program Files\WXWork\unins000.exe`, "", ""))
	writeShortcut(t, root, "Read me.lnk",
		buildShortcut(`C:\Program Files\WXWork\readme.txt`, "", ""))

	found := ScanWindows(root)
	got := names(found)
	// Both shortcuts point at WXWork.exe, so the first one win the dedupe; the
	// name dedupe is a second net for equal stems.
	if len(got) == 0 || (got[0] != "企业微信" && got[0] != "WeCom") {
		t.Fatalf("the program's rows = %v", got)
	}
	for _, name := range got {
		if name == "Uninstall WeCom" || name == "Read me" {
			t.Errorf("%q is not an application", name)
		}
	}
	if len(got) != 1 {
		t.Errorf("one program, %d rows: %v", len(got), got)
	}
	row := found[0]
	if !containsFold(row.Aliases, "WXWork") {
		t.Errorf("aliases = %v, want the executable WXWork", row.Aliases)
	}
	if row.Initials == "" {
		t.Errorf("the row has no pinyin initials: %+v", row)
	}
}

func TestScanWindowsDropsAnAliasThatIsTheName(t *testing.T) {
	root := t.TempDir()
	writeShortcut(t, root, "Notepad.lnk",
		buildShortcut(`C:\Windows\notepad.exe`, "", ""))
	found := ScanWindows(root)
	if len(found) != 1 {
		t.Fatalf("rows = %v", names(found))
	}
	// `notepad` repeats the visible name `Notepad` but for case, and
	// `build_aliases` drops what the user can already see — so there is no
	// alias, rather than a case variant of the title.
	if len(found[0].Aliases) != 0 {
		t.Errorf("aliases = %v, want none", found[0].Aliases)
	}
}

// names is a scan's row titles, for assertions and messages.
func names(found []App) []string {
	out := make([]string, 0, len(found))
	for _, app := range found {
		out = append(out, app.Name)
	}
	return out
}

// containsFold reports whether values holds want, ignoring case.
func containsFold(values []string, want string) bool {
	for _, value := range values {
		if foldEqual(value, want) {
			return true
		}
	}
	return false
}

// foldEqual is a case-insensitive string comparison, spelled here so the test
// file needs no extra import.
func foldEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := 0; i < len(left); i++ {
		a, b := left[i], right[i]
		if 'A' <= a && a <= 'Z' {
			a += 'a' - 'A'
		}
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}
