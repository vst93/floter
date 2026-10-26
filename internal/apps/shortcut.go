package apps

import (
	"encoding/binary"
	"os"
	"strings"
)

// Reading a Windows shortcut's target.
//
// A `.lnk` keeps the program it launches in its own `LinkInfo` structure
// (MS-SHLLINK §2.3), and the reference decodes exactly that much
// (`main:src-tauri/src/commands/apps/windows.rs:557-647`, `shortcut_target`):
// it is the part that carries a plain local path, and a shortcut that stores
// its target only as an item-id list or an environment-variable block returns
// none — in which case the shortcut is *kept*, because Windows itself still
// resolves it when the file is opened.
//
// The port never read the target at all, and that one omission cost three
// things the reference derives from it:
//
//   - the executable's name as a search alias, which is what keeps a
//     Chinese-named shortcut ("企业微信.lnk" → `WXWork.exe`) reachable from a
//     Latin keyboard (`windows.rs:355-360` `build_aliases`, `:265-270`
//     `executable_name`);
//   - the filter that drops the uninstallers, help files and web links sharing
//     the Start Menu with real applications (`is_launchable_target`,
//     `windows.rs:539-549`);
//   - the dedupe key: two shortcuts pointing at one program are one row.
//
// Parsing is pure and platform-free on purpose (the same reason
// `chromiumBrowsers` and the terminal tables are): every Windows rule here can
// then be asserted on any host, from a shortcut built in a temp directory,
// rather than only on the one machine that has a Start Menu.

// The MS-SHLLINK header, and the two flags this build looks at.
const (
	lnkHeaderSize         = 0x4C
	lnkFlagHasIDList      = 0x00000001
	lnkFlagHasLinkInfo    = 0x00000002
	lnkInfoVolumeAndLocal = 0x00000001 // LinkInfo: VolumeID + LocalBasePath present
	lnkInfoHeaderUnicode  = 0x24       // LinkInfoHeaderSize at/above this carries unicode offsets
	lnkInfoUnicodeBase    = 28         // offset of LocalBasePathOffsetUnicode
	lnkInfoUnicodeSuffix  = 32         // offset of CommonPathSuffixOffsetUnicode
)

// ShortcutTarget reads the local path a `.lnk` launches, reporting whether it
// could be read. The bytes are the file's own: a shortcut is small, and it is
// read once per scan.
func ShortcutTarget(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return shortcutTargetFromBytes(data)
}

// shortcutTargetFromBytes is ShortcutTarget for bytes already in hand.
//
// A header that is not the one size MS-SHLLINK declares is not a shortcut this
// build can read, and neither is one that keeps its target somewhere other than
// `LinkInfo` — both answer false, and the caller keeps the row.
func shortcutTargetFromBytes(data []byte) (string, bool) {
	if len(data) < lnkHeaderSize {
		return "", false
	}
	if readUint32(data, 0) != lnkHeaderSize {
		return "", false
	}
	flags := readUint32(data, 20)
	cursor := uint64(lnkHeaderSize)
	if flags&lnkFlagHasIDList != 0 {
		// The item-id list sits between the header and the LinkInfo, and its
		// own two-byte size decides where it ends.
		if cursor+2 > uint64(len(data)) {
			return "", false
		}
		size := uint64(binary.LittleEndian.Uint16(data[cursor:]))
		cursor += 2 + size
	}
	if flags&lnkFlagHasLinkInfo == 0 || cursor >= uint64(len(data)) {
		return "", false
	}

	info := data[cursor:]
	infoSize := uint64(readUint32(info, 0))
	if infoSize == 0 || infoSize > uint64(len(info)) {
		return "", false
	}
	info = info[:infoSize]
	headerSize := readUint32(info, 4)
	if readUint32(info, 8)&lnkInfoVolumeAndLocal == 0 {
		return "", false
	}

	// The unicode offsets exist only from the 0x24 header onward; older
	// shortcuts carry the ANSI path alone, which is what the reference reads
	// in the same order (`windows.rs:592-610`).
	ansiBase, ansiSuffix := readUint32(info, 16), readUint32(info, 24)
	var unicodeBase, unicodeSuffix uint32
	if headerSize >= lnkInfoHeaderUnicode {
		unicodeBase = readUint32(info, lnkInfoUnicodeBase)
		unicodeSuffix = readUint32(info, lnkInfoUnicodeSuffix)
	}
	base, ok := linkString(info, unicodeBase, ansiBase)
	if !ok {
		return "", false
	}
	suffix, _ := linkString(info, unicodeSuffix, ansiSuffix)
	target := base + suffix
	if target == "" {
		return "", false
	}
	return target, true
}

// linkString reads one LinkInfo string, preferring the unicode offset and
// falling back to the ANSI one (the reference's own order).
func linkString(info []byte, unicodeOffset, ansiOffset uint32) (string, bool) {
	if unicodeOffset != 0 && uint64(unicodeOffset) < uint64(len(info)) {
		bytes := info[unicodeOffset:]
		units := make([]uint16, 0, len(bytes)/2)
		for i := 0; i+1 < len(bytes); i += 2 {
			unit := binary.LittleEndian.Uint16(bytes[i:])
			if unit == 0 {
				break
			}
			units = append(units, unit)
		}
		return string(utf16Decode(units)), true
	}
	if ansiOffset == 0 || uint64(ansiOffset) >= uint64(len(info)) {
		return "", false
	}
	bytes := info[ansiOffset:]
	end := len(bytes)
	for i, b := range bytes {
		if b == 0 {
			end = i
			break
		}
	}
	return string(bytes[:end]), true
}

// utf16Decode turns UTF-16 code units into runes, pairing surrogates; an
// unpaired one becomes U+FFFD rather than shrinking the string.
func utf16Decode(units []uint16) []rune {
	out := make([]rune, 0, len(units))
	for i := 0; i < len(units); i++ {
		unit := units[i]
		if unit >= 0xD800 && unit < 0xDC00 && i+1 < len(units) {
			low := units[i+1]
			if low >= 0xDC00 && low < 0xE000 {
				out = append(out, rune(0x10000+(uint32(unit)-0xD800)<<10+(uint32(low)-0xDC00)))
				i++
				continue
			}
		}
		if unit >= 0xD800 && unit < 0xE000 {
			out = append(out, '\uFFFD')
			continue
		}
		out = append(out, rune(unit))
	}
	return out
}

// readUint32 reads a little-endian uint32 at offset, or 0 when the bytes are
// not there: every field in this file is optional and a short file is a file
// this build cannot read, which the caller reports as "no target".
func readUint32(data []byte, offset uint64) uint32 {
	if offset+4 > uint64(len(data)) {
		return 0
	}
	return binary.LittleEndian.Uint32(data[offset:])
}

// IsLaunchableTarget reports whether a shortcut's target is something a user
// launches. The Start Menu also holds the uninstallers, the read-me links and
// the web shortcuts every installer drops beside its program, and the reference
// drops exactly those: a target must be a program this system can start
// (`.exe`/`.bat`/`.cmd`), and its name must not read `uninst`/`unins`
// (`windows.rs:539-549`).
//
// The port is faithful to that rule except in two places it decided otherwise,
// and both are pinned in `startmenufilter_test.go` rather than left to be
// rediscovered: a trailing space is trimmed off the name before the extension
// is read (the reference asks `ends_with` of the whole string, and a
// `LinkInfo` base path may carry one), and a target that is *only* an extension
// (`.exe`) names nothing here (Rust's `file_stem(".exe")` is `.exe`, so the
// reference answers true). Neither is reachable; the answers are recorded so a
// later "match the reference exactly" pass changes a decision, not a mystery.
func IsLaunchableTarget(target string) bool {
	name := shortcutBaseName(target)
	switch strings.ToLower(extensionOf(name)) {
	case ".exe", ".bat", ".cmd":
	default:
		return false
	}
	stem := strings.ToLower(strings.TrimSuffix(name, extensionOf(name)))
	return !strings.Contains(stem, "uninst") && !strings.Contains(stem, "unins")
}

// ShortcutTargetName is the program a target names, without its path or
// extension: `C:\Program Files\WXWork\WXWork.exe` is WXWork — the reference's
// `executable_name` (`windows.rs:321-324`), which is Rust's `file_stem` and so
// drops *any* final extension, not just a launcher suffix.
//
// That is a different question from `progname.Stem`'s, which strips exactly
// `.exe`/`.cmd`/`.bat`/`.com` and lowercases, because there the answer becomes a
// connected program's command word (`report.tar.gz` must not turn into
// `report.tar`). The two rules are pinned against each other — with the numbers
// they disagree on — in `filestem_test.go`, so merging them goes red instead of
// silently renaming programs.
//
// A name with no extension at all names nothing: `windows.rs:321` would hand
// back the whole name, but every caller here has already passed a target through
// `IsLaunchableTarget` or the registry's `.exe` test, so this stays empty.
func ShortcutTargetName(target string) string {
	base := shortcutBaseName(target)
	stem := fileStem(base)
	if stem == base {
		return ""
	}
	return stem
}

// fileStem is a path element without its final extension, and the element
// itself when there is nothing to drop — Rust's `Path::file_stem`, which both
// `ShortcutTargetName` (`windows.rs:321`) and `desktopFileStem`
// (`linux.rs:434-440`) are built on. A leading dot is not an extension
// separator, so `.desktop` keeps its name; a trailing dot is, so `app.` is
// `app`; and only the last segment goes, so `report.tar.gz` is `report.tar`.
func fileStem(base string) string {
	if i := strings.LastIndexByte(base, '.'); i > 0 {
		return base[:i]
	}
	return base
}

// shortcutBaseName is a Windows path's last element. Both separators are split
// on rather than `filepath.Base`'s one: this file's rules are exercised on
// every host (see the header), and on Linux `filepath.Base` would hand back the
// whole `C:\\Program Files\\App\\App.exe` as one name.
func shortcutBaseName(path string) string {
	path = strings.TrimSpace(path)
	if i := strings.LastIndexAny(path, `\/`); i >= 0 {
		path = path[i+1:]
	}
	return path
}

// extensionOf is a path element's extension, the final dot onward.
func extensionOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		return name[i:]
	}
	return ""
}
