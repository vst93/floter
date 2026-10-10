// Package ansi reads the SGR escape sequences a command's output carries: the
// colours, weights and decorations a CLI writes for a terminal, as spans a
// surface can draw.
//
// The old build's R65 made the same point for the launcher's direct output:
// 「这种直出命令需要在非列表模式下（文本模式下）兼容输出文本的一些格式，有些是带格式
// 的终端字符」. A CLI's stdout is not plain prose — `git diff`, `ls --color` and
// most modern tools emit ANSI SGR — and rendered verbatim those bytes are
// visible garbage. This is the reading, and it is deliberately not a terminal
// emulator: the only sequences interpreted are the ones that change how a
// character *looks*; every other escape (cursor moves, erases, OSC titles) is
// dropped rather than printed, so `\x1b[2K` cannot become a line of noise.
package ansi

import (
	"strconv"
	"strings"
)

// Kind is what a colour names.
type Kind uint8

const (
	// Default is the surface's own foreground or background.
	Default Kind = iota
	// Named is one of the sixteen colours of a terminal palette: 0-7 are the
	// ordinary ones (black, red, green, yellow, blue, magenta, cyan, white)
	// and 8-15 their bright kinds.
	Named
	// Indexed is the 256-colour cube: 0-15 repeat the named colours, 16-231
	// are a 6×6×6 cube and 232-255 a grey ramp.
	Indexed
	// RGB is a 24-bit "truecolour".
	RGB
)

// Color is one colour a style may carry.
type Color struct {
	Kind    Kind
	N       uint8 // Named and Indexed
	R, G, B uint8 // RGB
}

// Style is what a run of characters looks like.
type Style struct {
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Strike    bool
	// Inverse swaps the foreground and the background.
	Inverse bool
	FG, BG  Color
}

// Span is a run of characters with one style.
type Span struct {
	Text  string
	Style Style
}

// Parse reads text as a terminal reads its SGR sequences, returning the runs
// of characters with the style in force when each was written. Text with no
// escapes comes back as one span with the zero style, so a caller can always
// draw the answer.
func Parse(text string) []Span {
	var (
		out   []Span
		style Style
		run   strings.Builder
	)
	flush := func() {
		if run.Len() == 0 {
			return
		}
		out = append(out, Span{Text: run.String(), Style: style})
		run.Reset()
	}
	for i := 0; i < len(text); {
		if text[i] != 0x1b {
			// A rune's bytes, not a byte: the text may be any language.
			_, size := decodeRune(text[i:])
			run.WriteString(text[i : i+size])
			i += size
			continue
		}
		next := i + 1
		if next >= len(text) {
			break
		}
		switch text[next] {
		case '[':
			// A control sequence: parameters, then one final byte.
			j := next + 1
			for j < len(text) && !isFinal(text[j]) {
				j++
			}
			if j >= len(text) {
				// Unterminated: the rest is not printable.
				i = len(text)
				continue
			}
			if text[j] == 'm' {
				flush()
				style = applySGR(style, text[next+1:j])
			}
			i = j + 1
		default:
			// Any other escape is a two-byte sequence (or an OSC string,
			// which ends at BEL or ST): dropped, never printed.
			i = skipEscape(text, i)
		}
	}
	flush()
	if len(out) == 0 {
		return []Span{{Text: ""}}
	}
	return out
}

// Plain is the text of spans with every escape read and dropped: what the
// output *says*, for copying to the clipboard or showing in a plain field.
func Plain(text string) string {
	spans := Parse(text)
	var out strings.Builder
	for _, span := range spans {
		out.WriteString(span.Text)
	}
	return out.String()
}

// isFinal reports whether a byte ends a control sequence: the parameter and
// intermediate bytes are 0x20-0x3f, and a final byte is 0x40-0x7e.
func isFinal(b byte) bool { return b >= 0x40 && b <= 0x7e }

// skipEscape drops the escape sequence starting at i and returns the index
// after it.
func skipEscape(text string, i int) int {
	next := i + 1
	if next >= len(text) {
		return len(text)
	}
	switch text[next] {
	case ']', 'P', 'X', '^', '_':
		// A string sequence: it ends at BEL or at ESC \.
		j := next + 1
		for j < len(text) {
			if text[j] == 0x07 {
				return j + 1
			}
			if text[j] == 0x1b && j+1 < len(text) && text[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return len(text)
	default:
		return next + 1
	}
}

// applySGR applies one SGR parameter list to a style. Unknown parameters are
// ignored rather than dropped with the text: a sequence a terminal would not
// understand must not eat the characters after it.
func applySGR(style Style, params string) Style {
	fields := strings.Split(params, ";")
	values := make([]int, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			// An empty parameter is a zero: `ESC[m` is a reset, and
			// `ESC[1;;4m` means bold, reset, underline.
			values = append(values, 0)
			continue
		}
		// A colon-separated colour (`38:5:1`) is read as its parts.
		if strings.Contains(field, ":") {
			for _, part := range strings.Split(field, ":") {
				values = append(values, atoi(part))
			}
			continue
		}
		values = append(values, atoi(field))
	}
	for i := 0; i < len(values); i++ {
		switch code := values[i]; {
		case code == 0:
			style = Style{}
		case code == 1:
			style.Bold = true
		case code == 2:
			style.Dim = true
		case code == 3:
			style.Italic = true
		case code == 4:
			style.Underline = true
		case code == 7:
			style.Inverse = true
		case code == 9:
			style.Strike = true
		case code == 22:
			style.Bold, style.Dim = false, false
		case code == 23:
			style.Italic = false
		case code == 24:
			style.Underline = false
		case code == 27:
			style.Inverse = false
		case code == 29:
			style.Strike = false
		case code >= 30 && code <= 37:
			style.FG = Color{Kind: Named, N: uint8(code - 30)}
		case code == 39:
			style.FG = Color{}
		case code >= 40 && code <= 47:
			style.BG = Color{Kind: Named, N: uint8(code - 40)}
		case code == 49:
			style.BG = Color{}
		case code >= 90 && code <= 97:
			style.FG = Color{Kind: Named, N: uint8(code - 90 + 8)}
		case code >= 100 && code <= 107:
			style.BG = Color{Kind: Named, N: uint8(code - 100 + 8)}
		case code == 38 || code == 48:
			color, used := readExtended(values[i+1:])
			if used == 0 {
				continue
			}
			if code == 38 {
				style.FG = color
			} else {
				style.BG = color
			}
			i += used
		}
	}
	return style
}

// readExtended reads the colour after a 38/48 introducer: `5;n` (a palette
// index) or `2;r;g;b` (truecolour). It reports how many values it consumed.
func readExtended(values []int) (Color, int) {
	if len(values) == 0 {
		return Color{}, 0
	}
	switch values[0] {
	case 5:
		if len(values) < 2 {
			return Color{}, 0
		}
		return Color{Kind: Indexed, N: clampByte(values[1])}, 2
	case 2:
		if len(values) < 4 {
			return Color{}, 0
		}
		return Color{
			Kind: RGB,
			R:    clampByte(values[1]),
			G:    clampByte(values[2]),
			B:    clampByte(values[3]),
		}, 4
	}
	return Color{}, 0
}

func clampByte(n int) uint8 {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return uint8(n)
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// decodeRune reads one UTF-8 rune and reports its size in bytes. Invalid bytes
// are one byte, so a caller never loops forever on malformed text.
func decodeRune(s string) (rune, int) {
	for i := 1; i <= len(s) && i <= 4; i++ {
		if i == len(s) || s[i]&0xc0 != 0x80 {
			return 0, i
		}
	}
	return 0, 1
}
