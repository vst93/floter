package shortcuts

import (
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Alt+Space":             "Alt+Space",
		"Cmd+Comma":             "Cmd+,",
		"CmdOrCtrl+Shift+Space": "CmdOrCtrl+Shift+Space",
		"CommandOrControl+N":    "CmdOrCtrl+N",
		"Ctrl+Shift+Space":      "Ctrl+Shift+Space",
		"Option+Space":          "Alt+Space",
		"Super+Period":          "Cmd+.",
		"cmdorctrl+shift+t":     "CmdOrCtrl+Shift+T",
		"Cmd+Shift+Space":       "Cmd+Shift+Space",
		"Ctrl+Alt+Delete":       "Ctrl+Alt+Delete",
		"F5":                    "F5",
		"Cmd+F12":               "Cmd+F12",
		"Ctrl+BracketLeft":      "Ctrl+[",
		"Cmd+Semicolon":         "Cmd+;",
		"Alt+Quote":             "Alt+'",
	}
	for input, want := range cases {
		got, ok := Normalize(input)
		if !ok {
			t.Errorf("Normalize(%q) failed", input)
			continue
		}
		if got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
	// The normalization is idempotent, and the modifier order is canonical.
	if got, _ := Normalize("Shift+Super+Space"); got != "Cmd+Shift+Space" {
		t.Errorf("modifier order = %q", got)
	}
	if got, _ := Normalize("Cmd+,"); got != "Cmd+," {
		t.Errorf("idempotent = %q", got)
	}

	for _, input := range []string{"", "   ", "Cmd", "Cmd+", "Cmd+A+B", "Hyper+X", "Cmd+Unknown"} {
		if got, ok := Normalize(input); ok {
			t.Errorf("Normalize(%q) = %q, want a rejection", input, got)
		}
	}
}

func TestNormalizeBinding(t *testing.T) {
	// A whole binding — at least one modifier and exactly one key — passes and
	// is canonicalised exactly as Normalize would spell it.
	kept := map[string]string{
		"Alt+Space":             "Alt+Space",
		"CmdOrCtrl+Shift+Space": "CmdOrCtrl+Shift+Space",
		"cmd+shift+p":           "Cmd+Shift+P",
		"Shift+Super+Space":     "Cmd+Shift+Space",
		"Ctrl+Alt+Delete":       "Ctrl+Alt+Delete",
		"Cmd+Comma":             "Cmd+,",
	}
	for input, want := range kept {
		got, ok := NormalizeBinding(input)
		if !ok {
			t.Errorf("NormalizeBinding(%q) failed", input)
			continue
		}
		if got != want {
			t.Errorf("NormalizeBinding(%q) = %q, want %q", input, got, want)
		}
	}

	// A bare key is not a binding: the old host kept a custom shortcut only
	// when its normalized form still held a `+`.
	// The rest are shapes Normalize is happy to repair but a stored binding
	// must not accept: an empty part, a modifier twice, two keys, no key.
	for _, input := range []string{
		"F5", "c", " ", "",
		"Ctrl++A", "Ctrl+", "+", "Ctrl+Ctrl+A", "Cmd+Super+X",
		"Ctrl+A+B", "Cmd", "Hyper+X", "Cmd+Unknown",
	} {
		if got, ok := NormalizeBinding(input); ok {
			t.Errorf("NormalizeBinding(%q) = %q, want a rejection", input, got)
		}
	}

	// The difference from the lenient path, pinned: Normalize repairs a stray
	// empty part, NormalizeBinding refuses it — a stored binding has to be one
	// the recorder could have produced.
	if got, ok := Normalize("Ctrl++A"); !ok || got != "Ctrl+A" {
		t.Fatalf("Normalize(\"Ctrl++A\") = %q, %v; the lenient path must repair it", got, ok)
	}
}

func TestFromKey(t *testing.T) {
	cases := []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
	}{
		{ui.Super, ui.KeySpace, "Cmd+Space"},
		{ui.Ctrl, ui.KeySpace, "Ctrl+Space"},
		{ui.Super | ui.Shift, ui.KeyS, "Cmd+Shift+S"},
		{ui.Ctrl | ui.Alt, ui.KeyComma, "Ctrl+Alt+,"},
		{ui.Super, ui.KeyF5, "Cmd+F5"},
		{ui.Alt, ui.KeyEscape, "Alt+Esc"},
		{ui.Super, ui.Key0, "Cmd+0"},
		{ui.Shift, ui.KeyS, "Shift+S"},
		{ui.Shift, ui.KeyF5, "Shift+F5"},
		// A bare function key is the one modifier-less chord the recorder
		// accepts, as the old build's `ShortcutRecorder` did.
		{0, ui.KeyF5, "F5"},
		{0, ui.KeyF12, "F12"},
	}
	for _, tc := range cases {
		got, ok := FromKey(tc.mods, tc.key)
		if !ok {
			t.Errorf("FromKey(%v, %v) failed", tc.mods, tc.key)
			continue
		}
		if got != tc.want {
			t.Errorf("FromKey(%v, %v) = %q, want %q", tc.mods, tc.key, got, tc.want)
		}
		// What the recorder produces normalizes to itself.
		if normalized, ok := Normalize(got); !ok || normalized != got {
			t.Errorf("FromKey(%v, %v) = %q, which does not normalize: %q", tc.mods, tc.key, got, normalized)
		}
	}

	// A bare key that is not a function key is not a shortcut.
	if _, ok := FromKey(0, ui.KeyS); ok {
		t.Error("a bare key was accepted")
	}
	if _, ok := FromKey(0, ui.KeySpace); ok {
		t.Error("a bare space was accepted")
	}
	if _, ok := FromKey(ui.Super, ui.KeyUnknown); ok {
		t.Error("an unknown key was accepted")
	}
}

func TestDisplay(t *testing.T) {
	// The display follows the platform, as the old build's formatShortcut did.
	// The platform is named explicitly so all three tables are pinned from any
	// host.
	cases := []struct {
		goos        string
		accelerator string
		want        string
	}{
		// macOS: the glyphs, and the arrows/delete as symbols.
		{"darwin", "Cmd+Comma", "\u2318,"},
		// P10-y 重定基线：旧 `⌘⇧P` → 新 `⇧⌘P`。词序的来源从「存储串的写入序」
		// 改成参考 `formatShortcut` 的规范序（⌃⌥⇧⌘），与 `Badge` 同一个产生者。
		{"darwin", "Cmd+Shift+P", "\u21e7\u2318P"},
		{"darwin", "Cmd+Up", "\u2318\u2191"},
		{"darwin", "Cmd+Down", "\u2318\u2193"},
		{"darwin", "Cmd+Left", "\u2318\u2190"},
		{"darwin", "Cmd+Right", "\u2318\u2192"},
		{"darwin", "Cmd+Backspace", "\u2318\u232b"},
		// Elsewhere: words joined with `+` and no spaces, meta named by the
		// platform.
		{"linux", "Ctrl+Shift+C", "Ctrl+Shift+C"},
		{"windows", "Ctrl+Shift+C", "Ctrl+Shift+C"},
		{"linux", "Super+K", "Super+K"},
		{"windows", "Super+K", "Win+K"},
		// A single character is upper case; the named keys keep their spelling.
		{"linux", "Alt+7", "Alt+7"},
		{"linux", "Ctrl+Space", "Ctrl+Space"},
		// The written order of the stored string never shows: two spellings of
		// one binding draw as one chord, in the reference's order
		// (Control → Alt → Shift → Meta).
		{"darwin", "Shift+Cmd+P", "\u21e7\u2318P"},
		{"darwin", "Ctrl+Alt+Shift+Cmd+K", "\u2303\u2325\u21e7\u2318K"},
		{"darwin", "Cmd+Ctrl+Alt+Shift+K", "\u2303\u2325\u21e7\u2318K"},
		{"linux", "Shift+Ctrl+Q", "Ctrl+Shift+Q"},
		{"windows", "Alt+Ctrl+X", "Ctrl+Alt+X"},
		// Meta is last on every platform, exactly as the reference wrote it.
		{"linux", "Super+Shift+K", "Shift+Super+K"},
		{"windows", "Shift+Super+K", "Shift+Win+K"},
		// `CmdOrCtrl` is the platform's own modifier: Command on macOS, Control
		// everywhere else — in the slot that modifier holds.
		{"darwin", "CmdOrCtrl+Shift+P", "\u21e7\u2318P"},
		{"linux", "Shift+CmdOrCtrl+P", "Ctrl+Shift+P"},
		{"windows", "Ctrl+CmdOrCtrl+P", "Ctrl+P"},
	}
	for _, tc := range cases {
		if got := displayFor(tc.goos, tc.accelerator); got != tc.want {
			t.Errorf("displayFor(%s, %q) = %q, want %q", tc.goos, tc.accelerator, got, tc.want)
		}
	}
	// The exported spelling reads this host's table.
	want := "Super+,"
	switch runtime.GOOS {
	case "darwin":
		want = "\u2318,"
	case "windows":
		want = "Win+,"
	}
	if got := Display("Cmd+Comma"); got != want {
		t.Errorf("Display = %q, want %q", got, want)
	}
	if got := Display("not a shortcut"); got != "not a shortcut" {
		t.Errorf("Display of junk = %q", got)
	}
}

// The badge is compact and platform-formatted: the modifiers joined to the
// key with no spaces, and Enter as ↩ everywhere.
func TestBadge(t *testing.T) {
	cases := []struct {
		goos        string
		accelerator string
		key         string
		want        string
	}{
		{"darwin", "Cmd+1", "1", "\u23181"},
		{"linux", "Ctrl+1", "1", "Ctrl+1"},
		{"windows", "Super+1", "1", "Win+1"},
		{"linux", "Super+1", "1", "Super+1"},
		{"darwin", "Cmd+Backspace", "\u232b", "\u2318\u232b"},
		// Enter is the return arrow on every platform.
		{"linux", "Ctrl+1", "enter", "Ctrl+\u21a9"},
		{"darwin", "Cmd+1", "Enter", "\u2318\u21a9"},
		// The badge and the display are one word order: meta last, whatever
		// order the stored string wrote.
		{"darwin", "Cmd+Shift+P", "P", "\u21e7\u2318P"},
		{"darwin", "Shift+Cmd+P", "P", "\u21e7\u2318P"},
		{"linux", "Shift+Ctrl+P", "P", "Ctrl+Shift+P"},
		{"linux", "Shift+Super+P", "P", "Shift+Super+P"},
		// `CmdOrCtrl` resolves against the *named* platform, not the host: the
		// darwin row is the one that could not be pinned from Linux before.
		{"darwin", "CmdOrCtrl+1", "1", "\u23181"},
		{"windows", "CmdOrCtrl+1", "1", "Ctrl+1"},
		{"linux", "CmdOrCtrl+Shift+1", "1", "Ctrl+Shift+1"},
		// A key `Parse` has no enum for still keeps its modifiers, as the old
		// build's `formatShortcut` drew them.
		{"linux", "Ctrl+PrintScreen", "PrintScreen", "Ctrl+PrintScreen"},
		{"darwin", "Cmd+VolumeUp", "VolumeUp", "\u2318VolumeUp"},
	}
	for _, tc := range cases {
		if got := badgeFor(tc.goos, tc.accelerator, tc.key); got != tc.want {
			t.Errorf("badgeFor(%s, %q, %q) = %q, want %q", tc.goos, tc.accelerator, tc.key, got, tc.want)
		}
	}
	// A digit badge carries the binding's modifiers: the old build's ⌘1.
	if runtime.GOOS == "darwin" {
		if got := Badge("Cmd+1", "1"); got != "\u23181" {
			t.Errorf("Badge = %q", got)
		}
	} else {
		if got := Badge("Ctrl+1", "1"); got != "Ctrl+1" {
			t.Errorf("Badge = %q", got)
		}
	}
	// Enter is the return arrow on every platform, and the modifier glyph
	// follows the platform's spelling of Control: ⌃ on macOS, "Ctrl" elsewhere.
	wantEnter := "Ctrl+\u21a9"
	if runtime.GOOS == "darwin" {
		wantEnter = "\u2303\u21a9"
	}
	if got := Badge("Ctrl+1", "enter"); got != wantEnter {
		t.Errorf("Badge(enter) = %q, want %q", got, wantEnter)
	}
	// An unparseable binding still prints the key alone.
	if got := Badge("nonsense", "3"); got != "3" {
		t.Errorf("Badge(junk) = %q", got)
	}
}

// TestDisplayAndBadgeAgree is the contract itself: one binding has one word
// order. The settings page draws a binding with `Display` and a launcher row
// draws the same binding's badge with `Badge`, so the two faces have to spell
// the modifiers the same way on every platform — the round that introduced this
// test found them disagreeing on `Cmd+Shift+P` (⌘⇧P vs ⇧⌘P).
func TestDisplayAndBadgeAgree(t *testing.T) {
	accelerators := []string{
		"Cmd+1", "Ctrl+1", "Super+1", "CmdOrCtrl+1",
		"Cmd+Shift+P", "Shift+Cmd+P", "Ctrl+Alt+Shift+Cmd+K",
		"Shift+Ctrl+Q", "Alt+Ctrl+X", "Super+Shift+K",
		"CmdOrCtrl+Shift+P", "Shift+CmdOrCtrl+P", "Ctrl+Shift+Backspace",
	}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, accelerator := range accelerators {
			normalized, ok := Normalize(accelerator)
			if !ok {
				t.Fatalf("Normalize(%q) failed", accelerator)
			}
			parts := strings.Split(normalized, "+")
			key := parts[len(parts)-1]
			display := displayFor(goos, accelerator)
			badge := badgeFor(goos, accelerator, key)
			if display != badge {
				t.Errorf("%s: displayFor(%q) = %q, badgeFor = %q — one binding, one word order",
					goos, accelerator, display, badge)
			}
		}
	}
}

func TestEqualAndDuplicate(t *testing.T) {
	// The two spellings of one binding are the same key.
	for _, pair := range [][2]string{
		{"Cmd+Comma", "Cmd+,"},
		{"Alt+Space", "Alt+Space"},
	} {
		if !Equal(pair[0], pair[1]) {
			t.Errorf("Equal(%q, %q) = false", pair[0], pair[1])
		}
	}
	// `CmdOrCtrl` is the modifier the platform binds: the same key as Cmd on
	// macOS and as Ctrl elsewhere.
	portable := "CmdOrCtrl+Shift+P"
	if runtime.GOOS == "darwin" {
		if !Equal(portable, "Cmd+Shift+P") || Equal(portable, "Ctrl+Shift+P") {
			t.Error("CmdOrCtrl did not resolve to Cmd on macOS")
		}
	} else {
		if !Equal(portable, "Ctrl+Shift+P") || Equal(portable, "Cmd+Shift+P") {
			t.Error("CmdOrCtrl did not resolve to Ctrl off macOS")
		}
	}
	if Equal("Cmd+A", "Cmd+B") {
		t.Error("two different keys compared equal")
	}
	// A key neither side can parse compares as trimmed text.
	if !Equal("not a key", " NOT A KEY ") {
		t.Error("unparseable keys did not compare by text")
	}

	keys := []string{"Cmd+Comma", "Alt+Space"}
	if got := Duplicate("Cmd+,", keys, -1); got != "Cmd+Comma" {
		t.Errorf("Duplicate = %q", got)
	}
	if got := Duplicate("Cmd+,", keys, 0); got != "" {
		t.Errorf("Duplicate ignoring its own row = %q", got)
	}
	if got := Duplicate("", keys, -1); got != "" {
		t.Errorf("an empty key = %q", got)
	}
}

func TestParseAndMatch(t *testing.T) {
	cases := []struct {
		accelerator string
		mods        ui.Modifiers
		key         ui.Key
	}{
		{"Cmd+G", ui.Super, ui.KeyG},
		{"Ctrl+Shift+C", ui.Ctrl | ui.Shift, ui.KeyC},
		{"Alt+Space", ui.Alt, ui.KeySpace},
		{"Cmd+,", ui.Super, ui.KeyComma},
		{"Cmd+1", ui.Super, ui.Key1},
		{"F5", 0, ui.KeyF5},
		{"Cmd+Up", ui.Super, ui.KeyUp},
	}
	for _, tc := range cases {
		mods, key, ok := Parse(tc.accelerator)
		if !ok || mods != tc.mods || key != tc.key {
			t.Errorf("Parse(%q) = %v, %v, %v", tc.accelerator, mods, key, ok)
		}
		if !Match(tc.accelerator, tc.mods, tc.key) {
			t.Errorf("Match(%q) = false", tc.accelerator)
		}
	}
	// The portable modifier resolves to the platform's.
	mods, key, ok := Parse("CmdOrCtrl+1")
	if !ok || key != ui.Key1 {
		t.Fatalf("Parse(CmdOrCtrl+1) = %v, %v, %v", mods, key, ok)
	}
	if runtime.GOOS == "darwin" && mods != ui.Super {
		t.Errorf("CmdOrCtrl = %v on macOS", mods)
	}
	if runtime.GOOS != "darwin" && mods != ui.Ctrl {
		t.Errorf("CmdOrCtrl = %v off macOS", mods)
	}

	// A binding without Shift is not the shifted press: the modifiers must
	// match exactly, as the old build's `matchesShortcut` required.
	if Match("Cmd+G", ui.Super|ui.Shift, ui.KeyG) {
		t.Error("a shifted letter matched a binding without Shift")
	}
	if !Match("Cmd+Shift+G", ui.Super|ui.Shift, ui.KeyG) {
		t.Error("the shifted letter did not match its own binding")
	}
	// The delete key is drawn for ⇧⌫: the r55 shortcut only claims the
	// unshifted press, because ⇧⌫ selects backwards.
	if Match("Cmd+Backspace", ui.Super|ui.Shift, ui.KeyBackspace) {
		t.Error("a shifted delete matched the unshifted delete binding")
	}
	if Match("Cmd+Shift+G", ui.Super, ui.KeyG) {
		t.Error("a binding with Shift matched an unshifted letter")
	}
	if Match("Cmd+G", ui.Ctrl, ui.KeyG) {
		t.Error("the wrong modifier matched")
	}
	if Match("Cmd+G", ui.Super, ui.KeyH) {
		t.Error("the wrong key matched")
	}
	if _, _, ok := Parse("not an accelerator"); ok {
		t.Error("junk parsed")
	}
}

// The result family is one binding with a digit swapped in, so a stored
// `select_result` is reduced to the family head — the user's modifiers and the
// digit `1` — and a modifier-less binding is refused. The old build's
// `normalizeResultShortcut` is the reference.
func TestNormalizeResultShortcut(t *testing.T) {
	// The family head keeps the modifiers and forces the digit to 1, whatever
	// digit the stored binding carried.
	cases := map[string]string{
		"Cmd+7":        "Cmd+1",
		"Ctrl+Shift+0": "Ctrl+Shift+1",
		"Alt+Shift+9":  "Alt+Shift+1",
		"Cmd+Delete":   "Cmd+1",
	}
	for input, want := range cases {
		got, ok := NormalizeResultShortcut(input)
		if !ok {
			t.Errorf("NormalizeResultShortcut(%q) failed", input)
			continue
		}
		if got != want {
			t.Errorf("NormalizeResultShortcut(%q) = %q, want %q", input, got, want)
		}
	}
	// A modifier-less digit would launch a row on ordinary number entry, and a
	// bare key would take over typing: both are refused, as is junk.
	for _, input := range []string{"7", "A", "not a key", ""} {
		if got, ok := NormalizeResultShortcut(input); ok {
			t.Errorf("NormalizeResultShortcut(%q) = %q, want a rejection", input, got)
		}
	}
	// Shift alone is a modifier, as the old build's `hasModifier` counted it.
	if got, ok := NormalizeResultShortcut("Shift+3"); !ok || got != "Shift+1" {
		t.Errorf("Shift+3 = %q, %v, want Shift+1", got, ok)
	}
}
