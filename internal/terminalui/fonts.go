package terminalui

// Which monospace faces this machine actually has.
//
// The picker should not offer a font the user cannot see: a list of
// candidates that all fall through to the generic monospace is worse than a
// short, honest one. The old build probed with the browser's own font
// fallback; a native build reads the font files on the machine instead — the
// naming convention (a file called `JetBrainsMono-Regular.ttf`) is enough to
// answer "is this family installed?", which is all the picker asks.
//
// The answer is memoised: it cannot change within a session without a font
// being installed, and the walk is not free.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"floter/internal/xdg"
)

// MonoFontCandidates are the faces the picker offers when they are installed,
// in the order the old build listed them.
var MonoFontCandidates = []string{
	"JetBrains Mono",
	"Fira Code",
	"Cascadia Mono",
	"SF Mono",
	"Menlo",
	"Monaco",
	"Consolas",
	"Hack",
	"Source Code Pro",
	"IBM Plex Mono",
	"Roboto Mono",
	"Ubuntu Mono",
	"Noto Sans Mono",
	"Inconsolata",
	"DejaVu Sans Mono",
	"Liberation Mono",
	"Courier New",
}

// FallbackFontFamilies are the shipped choices when nothing was detected: the
// platform's own monospace default plus the faces that are present on most
// machines of each kind.
var FallbackFontFamilies = []string{
	"JetBrains Mono",
	"SF Mono",
	"Cascadia Mono",
	"Menlo",
	"Consolas",
	"DejaVu Sans Mono",
	"Liberation Mono",
	"Courier New",
}

// fontDirs are the directories a platform keeps its font files in.
func fontDirs() []string {
	home, _ := os.UserHomeDir()
	return fontDirsFor(runtime.GOOS, home, os.Getenv("SystemRoot"), os.Getenv("LOCALAPPDATA"), xdg.DataHome())
}

// fontDirsFor is fontDirs with the environment passed in, so each platform's
// table can be asserted from any host — the same shape the browser and
// application tables use (`browser.chromiumBrowsers`, `apps`' platform tables).
//
// The Linux arm reads the *data directory* rather than assuming one: the fonts
// a user installs for themselves live in `$XDG_DATA_HOME/fonts` (that is the
// directory fontconfig itself reads), and `$XDG_DATA_HOME` is set to something
// other than `~/.local/share` often enough — a relocated home, a container, a
// machine whose dotfiles pin it — that spelling the fallback in here would hide
// every font the user installed. `xdg.DataHome` is the one answer to "where is
// the per-user data directory" in this build, including the specification's
// rule that a *relative* value is ignored.
func fontDirsFor(goos, home, systemRoot, localAppData, dataHome string) []string {
	switch goos {
	case "darwin":
		return []string{
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(home, "Library", "Fonts"),
		}
	case "windows":
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		dirs := []string{filepath.Join(systemRoot, "Fonts")}
		if localAppData != "" {
			dirs = append(dirs, filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	default:
		return []string{
			filepath.Join(dataHome, "fonts"),
			filepath.Join(home, ".fonts"),
			"/usr/share/fonts",
			"/usr/local/share/fonts",
		}
	}
}

// fontExtensions are the file kinds a font is stored as. A name in one of
// these is a family the machine has.
var fontExtensions = map[string]bool{
	".ttf": true, ".otf": true, ".ttc": true, ".otc": true,
	".dfont": true, ".pfb": true, ".pcf.gz": true, ".pcf": true,
}

// The font probe is the app's fourth "memo + background pass + ask for a
// frame" (see apps/icons.go, launcher/thumb.go, shell/detectedtools.go): the
// walk is disk IO over whole font directories, and it used to run on the
// drawing thread.
//
// Two things were wrong with the old `sync.Once`:
//
//   - **It blocked a frame.** `FontFamilyOptions` is asked while the settings
//     page's Terminal card and the terminal's appearance drawer are *built*,
//     i.e. once per painted frame. `detectMonospaceFonts` measures ~504µs on
//     this machine (`BenchmarkFontWalk`) — thirty milliseconds a second of pure
//     directory walking while either surface is up, and several times that on a
//     cold page cache, a network home directory, or the Windows and macOS
//     source sets this Linux box does not have. That is the shape of
//     「很多界面经常卡死」 the rest of this campaign closed one call site at a
//     time; this was the last one left on a per-frame path.
//   - **It could never see a new font.** A `sync.Once` is for life, so a face
//     installed after the first frame stayed missing from the picker until the
//     app restarted — the 「磁盘上有、UI 上没有」 family this repo has recorded
//     against four dead terminal knobs, here in the one control whose whole job
//     is to list what the machine has.
//
// The replacement is one clock and one goroutine: a frame reads the memo and
// nothing else; a cold or stale memo starts a single background pass that
// answers with what it has (the shipped fallback list on the very first frame,
// which is what a machine with none of the candidates shows anyway); the pass
// lands off-thread and asks for one frame.

// fontWalkTTL is how long a font answer stands. Detection is a directory walk,
// not a file watch, so a face installed while the app is running is picked up
// the next time anything asks after the TTL — two minutes is short enough that
// "I just installed JetBrains Mono" is answered by the next visit to the
// picker, and long enough that a page left open walks twice a minute at worst.
const fontWalkTTL = 2 * time.Minute

// fontWalk is the process's one font answer, and the pass filling it (there is
// one terminal UI per process, exactly as there is one inventory).
var fontWalk struct {
	mu       sync.Mutex
	names    []string
	computed time.Time
	pending  bool
}

// fontWalkInvalidate is asked for a frame when a background pass lands, so a
// picker that is up redraws with the detected faces instead of waiting for the
// user's next keystroke. The shell wires it to its window (see New).
var (
	fontWalkInvalidateMu sync.Mutex
	fontWalkInvalidate   func()
)

// SetFontWalkInvalidate installs the callback that asks for a frame when a
// background font pass lands.
func SetFontWalkInvalidate(fn func()) {
	fontWalkInvalidateMu.Lock()
	fontWalkInvalidate = fn
	fontWalkInvalidateMu.Unlock()
}

// DetectMonospaceFonts reports which of the candidates the machine has, by
// name, and starts the walk when the answer is cold or stale. It returns nil
// when nothing has been detected yet and nothing was cached — the caller falls
// back to the shipped list — and it never touches the disk on the calling
// thread.
func DetectMonospaceFonts() []string {
	fontWalk.mu.Lock()
	names, computed, pending := fontWalk.names, fontWalk.computed, fontWalk.pending
	stale := computed.IsZero() || time.Since(computed) > fontWalkTTL
	start := stale && !pending
	if start {
		fontWalk.pending = true
	}
	fontWalk.mu.Unlock()
	if start {
		go runFontWalk()
	}
	return names
}

// runFontWalk is the one background pass: it walks, records the answer and asks
// for a frame. A failed walk records an empty (not nil) answer with a fresh
// timestamp, so a machine whose font directories cannot be read does not retry
// the walk on every frame.
func runFontWalk() {
	names := detectMonospaceFonts()
	if names == nil {
		names = []string{}
	}
	fontWalk.mu.Lock()
	fontWalk.names, fontWalk.computed, fontWalk.pending = names, time.Now(), false
	fontWalk.mu.Unlock()
	fontWalkInvalidateMu.Lock()
	invalidate := fontWalkInvalidate
	fontWalkInvalidateMu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// detectMonospaceFonts walks the platform's font directories once.
func detectMonospaceFonts() []string {
	names := map[string]bool{}
	for _, dir := range fontDirs() {
		collectFontNames(dir, names, 0)
	}
	if len(names) == 0 {
		return nil
	}
	var out []string
	for _, candidate := range MonoFontCandidates {
		if names[fontKey(candidate)] {
			out = append(out, candidate)
		}
	}
	return out
}

// maxFontWalkDepth bounds the walk: font directories nest a few levels, and a
// symlink loop must not turn the probe into a crawl.
const maxFontWalkDepth = 4

// collectFontNames adds every font file's family-ish name from a directory
// tree.
func collectFontNames(dir string, names map[string]bool, depth int) {
	if depth > maxFontWalkDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			collectFontNames(filepath.Join(dir, entry.Name()), names, depth+1)
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !fontExtensions[ext] {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		// A file is named after its family with its style appended
		// (`JetBrainsMono-Bold`, `DejaVuSansMono`): the family half is what
		// the candidate is matched against, so the style suffix comes off.
		if index := strings.IndexAny(base, "-_"); index > 0 {
			base = base[:index]
		}
		names[fontKey(base)] = true
	}
}

// fontKey normalizes a family or file name for comparison: lower case, letters
// and digits only, so `JetBrains Mono`, `JetBrainsMono` and `jetbrainsmono`
// are one key.
func fontKey(name string) string {
	var out strings.Builder
	for _, char := range strings.ToLower(name) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			out.WriteRune(char)
		}
	}
	return out.String()
}

// FontFamilyOptions is the picker's list: the faces detected on this machine,
// in the candidate order, falling back to the shipped list when the walk
// found nothing. The current value is always present, so a hand-set family is
// never lost from the picker.
func FontFamilyOptions(current string) []string {
	detected := DetectMonospaceFonts()
	if len(detected) == 0 {
		detected = FallbackFontFamilies
	}
	out := append([]string{}, detected...)
	if current != "" {
		for _, option := range out {
			if strings.EqualFold(option, current) {
				return out
			}
		}
		out = append(out, current)
	}
	return out
}
