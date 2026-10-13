package launcher

import (
	"strings"
	"unicode"
)

// What a launcher row actually prints.
//
// The old build's `row-content.ts` gave every row two optional strings — the
// right-hand `source` slot and the left `subtitle` — and its R12/R14 rule was
// that the subtitle is dropped whenever it is nothing but the row's own *type
// word* or the row's title written again. "QQ音乐 / QQMusic", "Safari浏览器 /
// Safari", "企业微信 / WeCom": each subtitle was the same name in the other
// language, a second line spent on zero information (R14, verbatim:
// 「还是很丑，清爽一点」).
//
// The source half lives in `Item.Source` and is applied by the producers (a row
// that has no provenance word leaves the slot empty — see `commandItems` and
// `toolInstallItems`); the *type word* half is satisfied by construction too,
// since no producer sets a subtitle to its own type word. What this file adds
// is the other half: a subtitle that is a transcription of the title.

// transcriptionKey folds away everything that cannot change *which name* is
// being said: Unicode compatibility (full-width → ASCII), case, whitespace and
// punctuation/symbols. "QQ 音乐" and "qq音乐" collapse to one key; "WeCom" and
// "wecom" do too.
//
// The old build wrote this as `value.normalize("NFKC").toLowerCase()
// .replace(/[\s\p{P}\p{S}]+/gu, "")`. The part of NFKC this rule needs is the
// width fold — the full-width forms U+FF01..U+FF5E map to their ASCII
// counterparts and U+3000 to a plain space — so the fold is written out here
// rather than pulling in a Unicode normalisation table for one subtraction.
func transcriptionKey(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			r -= 0xFEE0
		case r == 0x3000:
			r = ' '
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// isTranscription reports whether subtitle is the row's title written again — a
// case, width, spacing or punctuation variant of it — rather than something new
// about the row.
func isTranscription(title, subtitle string) bool {
	return transcriptionKey(title) == transcriptionKey(subtitle)
}

// rowSubtitle is the row's secondary line as the list draws it, or "" when the
// row is better off without one. It is the one place the transcription rule is
// applied, and both readers go through it — the renderer and the height
// accounting — so a dropped subtitle cannot be drawn without being charged
// (the old build's R43: a row whose subtitle was dropped is drawn at the
// compact height).
func rowSubtitle(item Item) string {
	if item.Detail == "" || isTranscription(item.Title, item.Detail) {
		return ""
	}
	return item.Detail
}
