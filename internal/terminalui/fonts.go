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
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(home, "Library", "Fonts"),
		}
	case "windows":
		systemRoot := os.Getenv("SystemRoot")
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		dirs := []string{filepath.Join(systemRoot, "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	default:
		return []string{
			filepath.Join(home, ".local", "share", "fonts"),
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

// detectedFonts is the memoised answer, and the once that fills it.
var (
	fontsOnce   sync.Once
	fontsCached []string
)

// DetectMonospaceFonts reports which of the candidates the machine has, by
// name. It returns nil when the font directories cannot be read, and the
// caller then falls back to the shipped list.
func DetectMonospaceFonts() []string {
	fontsOnce.Do(func() { fontsCached = detectMonospaceFonts() })
	return fontsCached
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
