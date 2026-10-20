package apps

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"
)

// The Windows scan's second source, exercised without Windows: the text `reg
// query /s` prints, the UTF-16 PowerShell writes it in, and the rule that
// decides which uninstall entry names a program a user can launch.
//
// These run on any host on purpose (see registry.go's header): the platform-free
// half is the half that kept being wrong — the port read no registry at all, so
// every program without a Start Menu shortcut was simply absent from the list.

func TestParseRegistryProgramsReadsWhatRegPrints(t *testing.T) {
	text := "HKEY_CURRENT_USER\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\CloudMusic\r\n" +
		"    DisplayName    REG_SZ    网易云音乐\r\n" +
		"    DisplayIcon    REG_SZ    C:\\Program Files\\CloudMusic\\cloudmusic.exe,0\r\n" +
		"    InstallLocation    REG_SZ    C:\\Program Files\\CloudMusic\r\n" +
		"\r\n" +
		"HKEY_CURRENT_USER\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Runtime\r\n" +
		"    DisplayName    REG_SZ    Some Runtime\r\n" +
		"    SystemComponent    REG_DWORD    0x1\r\n" +
		"\r\n" +
		"HKEY_CURRENT_USER\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Empty\r\n" +
		"    Publisher    REG_SZ    Nobody\r\n"
	programs := ParseRegistryPrograms(text)
	if len(programs) != 2 {
		t.Fatalf("parsed %d programs, want 2 (the empty key names no program): %+v", len(programs), programs)
	}
	music := programs[0]
	if music.DisplayName != "网易云音乐" {
		t.Errorf("DisplayName = %q", music.DisplayName)
	}
	if music.DisplayIcon != `C:\Program Files\CloudMusic\cloudmusic.exe,0` {
		t.Errorf("DisplayIcon = %q", music.DisplayIcon)
	}
	if music.InstallLocation != `C:\Program Files\CloudMusic` {
		t.Errorf("InstallLocation = %q", music.InstallLocation)
	}
	if music.SystemComponent {
		t.Error("the music player was read as a system component")
	}
	if !programs[1].SystemComponent {
		t.Errorf("SystemComponent 0x1 was not read: %+v", programs[1])
	}
}

func TestRegistryEntryNamesTheProgramItInstalled(t *testing.T) {
	dir := t.TempDir()
	// The program the display name advertises, and a helper beside it that the
	// containment test must not pick.
	main := filepath.Join(dir, "cloudmusic.exe")
	if err := os.WriteFile(main, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cloudmusic-updater.exe"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		program RegistryProgram
		want    string
	}{
		{
			// `DisplayIcon` with the icon index still names the executable.
			name: "icon with index",
			program: RegistryProgram{
				DisplayName: "网易云音乐",
				DisplayIcon: main + ",0",
			},
			want: main,
		},
		{
			// No `DisplayIcon`: the install directory is searched for an
			// executable whose name the display name contains.
			name: "install location",
			program: RegistryProgram{
				DisplayName:     "CloudMusic Player",
				InstallLocation: dir,
			},
			want: main,
		},
		{
			// A Chinese display name contains no Latin letters, so the
			// containment test finds nothing — the reference has the same
			// limit, and it is `DisplayIcon` (which every installer that
			// names a program in Chinese writes) that covers this case.
			name: "chinese name without an icon",
			program: RegistryProgram{
				DisplayName:     "网易云音乐",
				InstallLocation: dir,
			},
		},
		{
			// A system component is never offered as an application.
			name: "system component",
			program: RegistryProgram{
				DisplayName:     "网易云音乐",
				DisplayIcon:     main,
				SystemComponent: true,
			},
		},
		{
			// An entry with no display name is not a row at all.
			name:    "no display name",
			program: RegistryProgram{DisplayIcon: main},
		},
		{
			// A `DisplayIcon` that is only an icon file leaves nothing to run,
			// and no install directory means no fallback.
			name: "icon file only",
			program: RegistryProgram{
				DisplayName: "Something",
				DisplayIcon: filepath.Join(dir, "readme.txt"),
			},
		},
		{
			// A display name the directory's executables do not contain.
			name: "name does not match",
			program: RegistryProgram{
				DisplayName:     "完全不同",
				InstallLocation: dir,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.program.Executable(); got != tc.want {
				t.Errorf("Executable() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecodeRegistryOutputReadsPowerShellsUnicode(t *testing.T) {
	text := "HKEY_CURRENT_USER\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\A\r\n" +
		"    DisplayName    REG_SZ    网易云音乐\r\n"
	// What `Out-File -Encoding Unicode` writes: a little-endian BOM and UTF-16.
	units := utf16.Encode([]rune(text))
	le := []byte{0xFF, 0xFE}
	for _, unit := range units {
		le = append(le, byte(unit), byte(unit>>8))
	}
	if got := DecodeRegistryOutput(le); got != text {
		t.Errorf("little-endian dump decoded to %q", got)
	}
	// UTF-16 without a mark: the zero high bytes are the tell.
	if got := DecodeRegistryOutput(le[2:]); got != text {
		t.Errorf("markless little-endian dump decoded to %q", got)
	}
	// A big-endian dump.
	be := []byte{0xFE, 0xFF}
	for _, unit := range units {
		be = append(be, byte(unit>>8), byte(unit))
	}
	if got := DecodeRegistryOutput(be); got != text {
		t.Errorf("big-endian dump decoded to %q", got)
	}
	// Plain bytes: a dump that came back as UTF-8 is not mojibake.
	plain := "DisplayName    REG_SZ    Safari\n"
	if got := DecodeRegistryOutput([]byte(plain)); got != plain {
		t.Errorf("plain dump decoded to %q", got)
	}
}

// The scan's two sources must not list one program twice, and the registry's
// rows carry the same derived fields the shortcut scan's do.
func TestScanRegistryProgramsSkipsWhatTheStartMenuListed(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "WXWork.exe")
	if err := os.WriteFile(program, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "cloudmusic.exe")
	if err := os.WriteFile(other, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "HKEY_CURRENT_USER\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\WeCom\r\n" +
		"    DisplayName    REG_SZ    企业微信\r\n" +
		"    DisplayIcon    REG_SZ    " + program + ",0\r\n" +
		"HKEY_CURRENT_USER\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Music\r\n" +
		"    DisplayName    REG_SZ    网易云音乐\r\n" +
		"    DisplayIcon    REG_SZ    " + other + ",0\r\n"
	registryMu.Lock()
	registryTexts, registryAt, registryQueried = []string{text}, time.Now(), true
	registryMu.Unlock()
	t.Cleanup(ResetRegistryCache)

	// The Start Menu already lists 企业微信 by the same target, so only the
	// music player is new.
	existing := []App{{Name: "企业微信", Path: filepath.Join(dir, "企业微信.lnk"), target: program}}
	got := scanRegistryPrograms(existing)
	if len(got) != 1 {
		t.Fatalf("the registry added %d rows, want 1 (the Start Menu's program is not listed twice): %+v",
			len(got), names(got))
	}
	music := got[0]
	if music.Name != "网易云音乐" {
		t.Errorf("Name = %q, want the display name", music.Name)
	}
	if music.Path != other {
		t.Errorf("Path = %q, want the executable (%q)", music.Path, other)
	}
	if music.Initials == "" {
		t.Error("the registry's row has no pinyin initials: a Chinese name is unreachable from a Latin keyboard")
	}
	wantAlias := "cloudmusic"
	found := false
	for _, alias := range music.Aliases {
		if alias == wantAlias {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want the executable's own name %q among them", music.Aliases, wantAlias)
	}
}
