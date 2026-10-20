package apps

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// The Windows registry's uninstall entries, the scan's second source.
//
// The Start Menu is not the whole of what is installed: an installer that ships
// no shortcut (a driver's control panel, a per-user program installed into a
// folder nobody put in `Programs`, a program the user un-ticked "create a
// shortcut" for) is only visible through the uninstall keys. The reference reads
// all three of them (`collect_registry_programs`, `windows.rs:260-330`) and the
// port never did — so a Windows launch list that skipped those programs looked
// like a search whose catalogue was half empty, which is the shape of
// 「Windows 下现在几乎没有办法正常使用」.
//
// What lives here is the platform-free half: the flat text `reg query /s`
// prints, the UTF-16 it is written in, and the rule that decides which entry
// names a program a user can launch. Every one of those is asserted on any host
// (see registry_test.go), which is why the parser takes a string rather than a
// process.

// registryPrograms is the registry's own contribution to a Windows scan: the
// three uninstall keys, read at most once per 30 minutes.
//
// The query is three helper processes, which is the whole reason for the memo:
// a scan runs on every summon the sources look stale, and three `reg query /s`
// launches per summon is a visible delay for a list that changes when the user
// installs something. Six minutes is the reference's own figure
// (`max_cache_age()`, `windows.rs:85`: 30 minutes) and it is also the answer to
// its second half — an installer that only edits values inside an existing child
// key does not move the parent's timestamp, so the *time* is the fallback that
// catches it (see ResetRegistryCache for the user's own "I installed something").
const registryCacheAge = 30 * time.Minute

var (
	registryMu      sync.Mutex
	registryTexts   []string
	registryAt      time.Time
	registryQueried bool
)

// ResetRegistryCache forgets the registry's memo, so the next scan asks the
// registry again. The tray's rescan is the user saying a program was installed
// a moment ago, which is the one moment the 30-minute window is wrong.
func ResetRegistryCache() {
	registryMu.Lock()
	registryQueried = false
	registryTexts = nil
	registryAt = time.Time{}
	registryMu.Unlock()
}

// registrySources is the decoded text of every uninstall key, memoized for
// registryCacheAge.
func registrySources() []string {
	registryMu.Lock()
	defer registryMu.Unlock()
	if registryQueried && time.Since(registryAt) < registryCacheAge {
		return registryTexts
	}
	registryTexts = queryRegistry()
	registryAt = time.Now()
	registryQueried = true
	return registryTexts
}

// RegistryProgram is one uninstall entry, as `reg query` prints it.
type RegistryProgram struct {
	DisplayName     string
	DisplayIcon     string
	InstallLocation string
	SystemComponent bool
}

// ParseRegistryPrograms reads the flat text `reg query <key> /s` prints: a
// `HKEY_…` line per subkey, then one indented `name    REG_TYPE    value` line
// per value. Anything else is skipped, so a truncated or localized dump still
// yields the entries it does carry.
func ParseRegistryPrograms(text string) []RegistryProgram {
	var out []RegistryProgram
	var current RegistryProgram
	flush := func() {
		if current.DisplayName != "" {
			out = append(out, current)
		}
		current = RegistryProgram{}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "HKEY_") {
			// A new subkey: the entry above it is complete.
			flush()
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		// `reg` separates the value's three columns with four spaces; the value
		// itself may contain any number of them.
		parts := strings.SplitN(trimmed, "    ", 3)
		if len(parts) < 3 {
			continue
		}
		value := strings.TrimSpace(parts[2])
		switch strings.TrimSpace(parts[0]) {
		case "DisplayName":
			current.DisplayName = value
		case "DisplayIcon":
			current.DisplayIcon = value
		case "InstallLocation":
			if value != "" {
				current.InstallLocation = value
			}
		case "SystemComponent":
			current.SystemComponent = strings.HasSuffix(value, "1")
		}
	}
	flush()
	return out
}

// Executable is the program the entry names, empty when it names none this
// build can launch — the reference's `RegistryProgram::executable`
// (`windows.rs:428-478`). `DisplayIcon` is usually the main executable, with an
// optional icon index (`C:\app\app.exe,0`); failing that, the install directory
// is searched for an executable whose own name the display name contains, which
// is how "网易云音乐" finds `cloudmusic.exe`.
func (p RegistryProgram) Executable() string {
	if p.SystemComponent || strings.TrimSpace(p.DisplayName) == "" {
		return ""
	}
	if p.DisplayIcon != "" {
		icon := strings.Trim(strings.TrimSpace(p.DisplayIcon), `"`)
		candidate := icon
		if i := strings.LastIndexByte(icon, ','); i > 0 {
			if index := strings.TrimPrefix(strings.TrimSpace(icon[i+1:]), "-"); index != "" && digitsOnly(index) {
				candidate = icon[:i]
			}
		}
		candidate = strings.Trim(strings.TrimSpace(candidate), `"`)
		if isExistingExecutable(candidate) {
			return candidate
		}
	}
	location := strings.Trim(strings.TrimSpace(p.InstallLocation), `"`)
	if location == "" {
		return ""
	}
	entries, err := os.ReadDir(location)
	if err != nil {
		return ""
	}
	needle := normalizedKey(p.DisplayName)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(location, entry.Name())
		if !isExistingExecutable(path) {
			continue
		}
		stem := normalizedKey(ShortcutTargetName(entry.Name()))
		if stem != "" && strings.Contains(needle, stem) {
			return path
		}
	}
	return ""
}

// normalizedKey is a name reduced to its lowercase letters and digits, the form
// the display-name containment test compares in.
func normalizedKey(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// isExistingExecutable reports whether path is a `.exe` file that is there. The
// extension test is case-insensitive because Windows writes both (`APP.EXE`).
func isExistingExecutable(path string) bool {
	if path == "" || !strings.EqualFold(extensionOf(shortcutBaseName(path)), ".exe") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// digitsOnly reports whether value is a run of ASCII digits.
func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// DecodeRegistryOutput decodes what `Out-File -Encoding Unicode` wrote: UTF-16
// with a byte-order mark. A dump that came back as plain bytes is read as UTF-8
// rather than as mojibake, and a UTF-16 dump without a mark is detected by its
// zero high bytes (`windows.rs:381-412`).
func DecodeRegistryOutput(data []byte) string {
	littleEndian := true
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:
		data = data[2:]
	case len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF:
		data = data[2:]
		littleEndian = false
	default:
		pairs := len(data) / 2
		if pairs > 64 {
			pairs = 64
		}
		zeroHigh := 0
		for i := 0; i < pairs; i++ {
			if data[2*i+1] == 0 {
				zeroHigh++
			}
		}
		if pairs == 0 || zeroHigh <= pairs/2 {
			return string(data)
		}
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if littleEndian {
			units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	return string(utf16.Decode(units))
}
