package launcher

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The keycap: Raycast's pressable legend, and the launcher's one way of
// printing a key (`docs/raycast-design.md` §4.5).
//
// A `⌘1` result badge and the action bar's `⌘↩` are *keys*, not sentences: each
// key is its own 20x20 plate with a 5px corner, the keycap face and its
// hairline, the label in the caption step at weight 500 in the tertiary ink,
// and a combination's keys sit 2u apart with no `+` between them. The port
// printed the whole chord as one run of text in the row slot, which reads as a
// label that happens to contain a symbol rather than as the key the user can
// press.
const (
	// keycapUnits is a keycap's own box: 20u, i.e. `Space(5)`.
	keycapUnits = 5
	// keycapGapUnits is the seam between a combination's keys: 2u.
	keycapGapUnits = 0.5
)

// keycapLabels splits a badge into the keys it is made of: the platform's own
// spelling is what decides, because that is what `shortcuts.Badge` formatted.
//
// macOS writes a chord as the modifier glyphs followed by the key (`⇧⌘1`), so
// every glyph is one key and an ASCII tail is one more. Every other platform
// writes it with `+` (`Ctrl+Shift+1`), so the split is that separator. A badge
// that is a single key is one label either way.
func keycapLabels(badge string) []string {
	if badge == "" {
		return nil
	}
	if runtime.GOOS == "darwin" {
		return splitGlyphRun(badge)
	}
	parts := strings.Split(badge, "+")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{badge}
	}
	return out
}

// splitGlyphRun splits a macOS badge into its symbolic keys and its trailing
// ASCII run: `⇧⌘1` is three keys, while `⌘F5` is two (`⌘`, `F5`).
func splitGlyphRun(badge string) []string {
	var out []string
	var ascii strings.Builder
	flush := func() {
		if ascii.Len() > 0 {
			out = append(out, ascii.String())
			ascii.Reset()
		}
	}
	for _, r := range badge {
		if r < 0x80 {
			ascii.WriteRune(r)
			continue
		}
		flush()
		out = append(out, string(r))
	}
	flush()
	return out
}

// keycap draws one key: the 20x20 plate, its hairline, and the label centred
// in it. A label wider than the plate — `Ctrl` on the platforms that spell the
// modifiers in words — grows the plate along its own padding, because 20u is
// the *legend's* size and a word must stay readable; the height never moves.
func keycap(c *ui.Context, tokens theme.Tokens, t *ui.Theme, label string) ui.Element {
	key := ui.Row(c).Shrink(0).MinWidth(t.Space(keycapUnits)).Height(t.Space(keycapUnits)).
		Padding(0, t.Space(1)).Radius(tokens.KeycapRadius).
		Border(1, tokens.KeycapEdge).Background(tokens.KeycapFace).
		AlignItems(ui.Center).Justify(ui.Center)
	key.Children(func() {
		ui.Text(c, label).FontSize(tokens.Caption).FontWeight(500).
			TextColor(tokens.TextTertiary).SingleLine().Shrink(0)
	})
	return key
}
