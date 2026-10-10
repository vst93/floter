package extensions

import (
	"strings"
)

// The `--help` inference tier: when a tool is connected, the host runs it
// once with `--help` and reads what it printed. The parser is deliberately
// conservative and generic — it recognizes option-definition lines across
// the common CLI styles (clap, argparse, cobra, Go flag) and silently
// ignores everything it does not understand. A tool must never fail to
// connect, nor be suggested a wrong parameter, because of this step:
// garbage in, empty (or partial) out.

// maxDerivedArguments bounds a help output's option hints, so a pathological
// help cannot flood the launcher's completions.
const maxDerivedArguments = 40

// maxDerivedSubcommands bounds a listing's entries.
const maxDerivedSubcommands = 40

// maxSubcommandProbes is how many subcommands get their own second-level
// help probe at connect time. Tools with huge plugin lists must not turn
// one connection into a process storm.
const maxSubcommandProbes = 12

// DerivedArgument is one option the help text describes.
type DerivedArgument struct {
	// Names are the spellings, short and long (`-o`, `--output`).
	Names []string
	// Kind is "flag", "string", "path" or "directory", as the schema spells
	// them.
	Kind string
	// Description is what the help said the option is for.
	Description string
	// TakesValue and ValueHint say whether it takes one and what shape it
	// is.
	TakesValue bool
	ValueHint  string
}

// DerivedSubcommand is one entry a listing-style help output describes.
type DerivedSubcommand struct {
	// Name is the sanitized token: lowercase ASCII alphanumerics plus `-`
	// and `_`, starting with a letter or digit — the same charset the
	// generated commands use.
	Name string
	// Aliases came from an `(aliases: a, b)` group.
	Aliases []string
	// Description came from the indented follow-up line (v style) or the
	// same-line remainder after a wide gap (cobra style).
	Description string
	// Arguments are the subcommand's own probed flags; empty unless the
	// second-level probe managed it within budget.
	Arguments []DerivedArgument
}

// HelpDerivation is everything the host understood from one tool's help:
// the root option hints plus one entry per listed subcommand.
type HelpDerivation struct {
	RootArguments []DerivedArgument
	Subcommands   []DerivedSubcommand
}

// Empty reports whether the derivation understood nothing at all — the
// drift re-probe's signal that the tool is no longer answering the way it
// did at connect time.
func (d HelpDerivation) Empty() bool {
	return len(d.RootArguments) == 0 && len(d.Subcommands) == 0
}

// DeriveArguments parses raw help output into ordered argument hints.
//
// Line-based by design: each line is classified independently, with one
// narrow exception — an indented plain-text line following an option whose
// description is still empty is treated as its description (Go flag style).
func DeriveArguments(helpOutput string) []DerivedArgument {
	helpOutput = stripANSI(helpOutput)
	var arguments []DerivedArgument
	seen := map[string]bool{}
	for _, line := range strings.Split(helpOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || skippedLine(trimmed) {
			continue
		}
		// A continuation line (Go flag style): indented prose describing the
		// previous option. Guarded so separators, extra flags and headers
		// are never glued onto an unrelated argument.
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) &&
			!strings.HasPrefix(trimmed, "-") && len(arguments) > 0 &&
			arguments[len(arguments)-1].Description == "" {
			arguments[len(arguments)-1].Description = continuationDescription(trimmed)
			continue
		}
		// Either a classic option-definition line or a compact flag-summary
		// row (`I/O: -pipe (auto) -file <path> -clip`); anything else is
		// prose and yields no candidates.
		var candidates []rawOption
		if raw, ok := parseOptionLine(line); ok {
			candidates = []rawOption{raw}
		} else {
			candidates = parseSummaryFlags(trimmed)
		}
		done := false
		for _, raw := range candidates {
			if len(raw.names) == 0 {
				continue
			}
			// Help and version are the host's own keys; they are never
			// suggested.
			skip := false
			for _, name := range raw.names {
				switch name {
				case "-h", "--help", "-V", "--version":
					skip = true
				}
			}
			if skip {
				continue
			}
			// Dedupe by the canonical long name (or the first flag when no
			// long form exists), so the same option reached through a
			// wrapped listing — or through both an options section and a
			// summary line — is only suggested once.
			key := raw.names[0]
			for _, name := range raw.names {
				if strings.HasPrefix(name, "--") {
					key = name
					break
				}
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			arguments = append(arguments, DerivedArgument{
				Names:       raw.names,
				Kind:        argumentKind(raw.valueHint, raw.valueHint != ""),
				Description: raw.description,
				TakesValue:  raw.valueHint != "",
				ValueHint:   raw.valueHint,
			})
			if len(arguments) >= maxDerivedArguments {
				done = true
				break
			}
		}
		if done {
			break
		}
	}
	return arguments
}

// rawOption is one parsed option-definition line.
type rawOption struct {
	names       []string
	valueHint   string
	description string
}

// Lines that are never option definitions even though they may sit inside
// the options section: usage banners (English and Chinese) and URLs.
func skippedLine(trimmed string) bool {
	lowered := strings.ToLower(trimmed)
	return strings.HasPrefix(lowered, "usage:") ||
		strings.HasPrefix(trimmed, "用法：") || strings.HasPrefix(trimmed, "用法:") ||
		strings.Contains(trimmed, "http://") || strings.Contains(trimmed, "https://") ||
		isSectionHeader(trimmed)
}

// isSectionHeader reports whether a line is one (`Options:`, `Flags:`,
// `Available Commands:`): it ends in a colon and carries nothing but
// letters, whitespace and that colon.
func isSectionHeader(trimmed string) bool {
	if !strings.HasSuffix(trimmed, ":") {
		return false
	}
	for _, char := range trimmed {
		if !isAlphanumeric(char) && char != ' ' && char != '\t' && char != ':' {
			return false
		}
	}
	return true
}

// isAlphanumeric covers the letters beyond ASCII too — the CJK and
// accented characters of localized help — but not the emoji and box-drawing
// glyphs decoration is made of: those are above the letter blocks and below
// the private-use ones.
func isAlphanumeric(char rune) bool {
	switch {
	case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		return true
	// Latin-1 letters and accents.
	case char >= 0xc0 && char <= 0x24f:
		return true
	// Greek and Cyrillic.
	case char >= 0x370 && char <= 0x4ff:
		return true
	// CJK, kana, Hangul.
	case char >= 0x2e80 && char <= 0xd7af:
		return true
	// CJK compatibility.
	case char >= 0xf900 && char <= 0xfaff:
		return true
	}
	return false
}

// parseOptionLine reads one candidate line into flags plus an optional
// value placeholder and description. It reports false for anything that
// does not begin with a syntactically plausible flag, which structurally
// excludes usage lines, section headers, subcommand lists and free-form
// examples.
func parseOptionLine(line string) (rawOption, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "-") {
		return rawOption{}, false
	}
	// Tokenize with the whitespace gap preceding each token; a gap of two
	// or more columns separates the definition from its description in
	// virtually every help formatter.
	type token struct {
		text string
		gap  int
	}
	var tokens []token
	rest := trimmed
	gap := 0
	for rest != "" {
		trimmedRest := strings.TrimLeft(rest, " \t")
		gap = len(rest) - len(trimmedRest)
		rest = trimmedRest
		if rest == "" {
			break
		}
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			tokens = append(tokens, token{rest, gap})
			break
		}
		tokens = append(tokens, token{rest[:end], gap})
		rest = rest[end:]
	}

	var out rawOption
	descriptionStart := ""
	for _, tok := range tokens {
		if len(out.names) == 0 {
			// The leading cluster: every token must be a flag (possibly
			// with an inline `=` value); anything else disqualifies the
			// line.
			flag, inline, ok := flagToken(tok.text)
			if !ok {
				return rawOption{}, false
			}
			out.names = append(out.names, flag)
			if inline != "" {
				if hint := inlineValue(inline); hint != "" && out.valueHint == "" {
					out.valueHint = hint
				}
			}
			continue
		}
		wideGap := tok.gap >= 2
		if flag, inline, ok := flagToken(tok.text); ok {
			if wideGap {
				// The next row of a multi-column layout, not an alias.
				descriptionStart = tok.text
				break
			}
			out.names = append(out.names, flag)
			if inline != "" {
				if hint := inlineValue(inline); hint != "" && out.valueHint == "" {
					out.valueHint = hint
				}
			}
			continue
		}
		// Placeholders may carry an alias-list comma too (`-c COUNT, …`).
		bare := strings.TrimSuffix(tok.text, ",")
		if hint := placeholderValue(bare); hint != "" {
			if out.valueHint == "" {
				out.valueHint = hint
			}
			continue
		}
		// An unknown token: the description begins here, whether or not the
		// formatter used a wide gap.
		descriptionStart = tok.text
		break
	}
	if len(out.names) == 0 {
		return rawOption{}, false
	}
	// The description is everything from its first token to the line's end.
	if descriptionStart != "" {
		if index := strings.Index(trimmed, descriptionStart); index >= 0 {
			out.description = strings.TrimSpace(trimmed[index:])
		}
	}
	return out, true
}

// flagToken recognizes a flag token (`-o,` `--output` `--format=json`) and
// returns its canonical spelling plus any inline value text.
func flagToken(token string) (string, string, bool) {
	raw, inline := token, ""
	if index := strings.IndexByte(token, '='); index >= 0 {
		raw, inline = token[:index], token[index+1:]
	}
	long := strings.HasPrefix(raw, "--")
	var body string
	if long {
		body = raw[2:]
	} else {
		if !strings.HasPrefix(raw, "-") || len(raw) < 2 {
			return "", "", false
		}
		body = raw[1:]
	}
	// Alias lists separate flags with commas (`-o, --output`).
	body = strings.TrimSuffix(body, ",")
	if body == "" {
		return "", "", false
	}
	for index, char := range body {
		if index == 0 {
			// The first character must be a letter.
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
				return "", "", false
			}
			continue
		}
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_') {
			return "", "", false
		}
	}
	dashes := "-"
	if long {
		dashes = "--"
	}
	return dashes + body, inline, true
}

// placeholderValue detects a value placeholder and returns its hint text:
// bracketed forms (`<FILE>`, `[N]`, `(DIR)`) or a bare uppercase/type token
// (`N`, `COUNT`, `int`) following the flags.
func placeholderValue(token string) string {
	if token == "" || len(token) > 24 {
		return ""
	}
	for _, pair := range [][2]byte{{'<', '>'}, {'[', ']'}, {'(', ')'}} {
		if strings.HasPrefix(token, string(pair[0])) && strings.HasSuffix(token, string(pair[1])) {
			inner := token[1 : len(token)-1]
			if inner == "" || strings.ContainsAny(inner, " \t") {
				return ""
			}
			return inner
		}
	}
	upper := true
	hasLetter := false
	for _, char := range token {
		if char >= 'a' && char <= 'z' {
			upper = false
			break
		}
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			if char >= 'A' && char <= 'Z' {
				hasLetter = true
			}
			continue
		}
		upper = false
		break
	}
	if upper && hasLetter {
		return token
	}
	// Lowercase type words emitted by Go `flag` and similar formatters.
	switch token {
	case "int", "integer", "uint", "string", "float", "number", "duration", "bool":
		return token
	}
	return ""
}

// inlineValue reads the value after `=`: bracketed forms or any non-empty
// remainder.
func inlineValue(text string) string {
	if hint := placeholderValue(text); hint != "" {
		return hint
	}
	return text
}

// argumentKind maps a value hint onto a descriptor kind. Flags stay flag;
// hints naming files or directories upgrade to the richer kinds so
// completions can offer paths, everything else degrades to plain strings.
func argumentKind(valueHint string, takesValue bool) string {
	if !takesValue {
		return "flag"
	}
	hint := strings.ToLower(valueHint)
	switch {
	case strings.Contains(hint, "dir"):
		return "directory"
	case strings.Contains(hint, "file"), strings.Contains(hint, "path"):
		return "path"
	default:
		return "string"
	}
}

// continuationDescription cleans a continuation line: a Go-style
// `(default …)` trailer describes a default rather than a purpose, and is
// dropped.
func continuationDescription(trimmed string) string {
	if index := strings.Index(trimmed, "(default"); index >= 0 && strings.HasSuffix(trimmed, ")") {
		return strings.TrimRight(trimmed[:index], " ")
	}
	return trimmed
}

// minSummaryFlags and maxSummaryFlags bound the compact one-line flag
// summary recognition.
const (
	minSummaryFlags = 3
	maxSummaryFlags = 16
)

// parseSummaryFlags reads a compact one-line flag summary into option
// definitions.
//
// Shape: a short label ending in `:` (at most eight characters, e.g.
// `I/O:` or `Flags:`) followed by at least three distinct `-word` tokens
// separated by middle dots, commas, bare whitespace, or parenthesized
// annotations (`(auto)`); a placeholder token directly after a flag
// attaches as its value hint (`-out <path>`). Every remaining token must be
// explainable, so prose lines (`Note: see -a and -b below`) are rejected
// wholesale instead of mining stray dashes.
func parseSummaryFlags(trimmed string) []rawOption {
	label, rest, found := strings.Cut(trimmed, ":")
	if !found {
		return nil
	}
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 8 {
		return nil
	}
	for _, char := range label {
		if !isAlphanumeric(char) && char != '/' {
			return nil
		}
	}
	var names []string
	var hints []string
	seen := map[string]bool{}
	for _, token := range strings.Fields(rest) {
		// Separator debris and annotation groups: middle dot, comma,
		// semicolon, empty parens, `(auto)`.
		if strings.Trim(token, "\u00b7,;") == "" ||
			(strings.HasPrefix(token, "(") && strings.HasSuffix(token, ")") &&
				len(token) >= 2 && !strings.ContainsAny(token[1:len(token)-1], " \t")) {
			continue
		}
		if flag, inline, ok := flagToken(token); ok {
			if !seen[flag] {
				seen[flag] = true
				names = append(names, flag)
				hints = append(hints, inlineValue(inline))
			}
			continue
		}
		// A placeholder belonging to the preceding flag (`-out <path>`).
		if hint := placeholderValue(token); hint != "" && len(hints) > 0 && hints[len(hints)-1] == "" {
			hints[len(hints)-1] = hint
			continue
		}
		// An unexplainable token: prose, not a summary.
		return nil
	}
	if len(names) < minSummaryFlags || len(names) > maxSummaryFlags {
		return nil
	}
	out := make([]rawOption, 0, len(names))
	for index, name := range names {
		hint := ""
		if index < len(hints) {
			hint = hints[index]
		}
		out = append(out, rawOption{names: []string{name}, valueHint: hint})
	}
	return out
}

// stripANSI removes ANSI escape sequences (CSI runs ending in a final byte,
// OSC strings terminated by BEL or ST, and bare two-byte escapes). Tools
// that colorize their help regardless of whether stdout is a TTY would
// otherwise poison every token with escape-sequence fragments.
func stripANSI(text string) string {
	var cleaned strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != 0x1b {
			cleaned.WriteRune(runes[i])
			continue
		}
		if i+1 >= len(runes) {
			break
		}
		switch runes[i+1] {
		case '[':
			i += 2
			// Parameter and intermediate bytes, then one final byte.
			for ; i < len(runes); i++ {
				if runes[i] >= 0x40 && runes[i] <= 0x7e {
					break
				}
			}
		case ']':
			i += 2
			for ; i < len(runes); i++ {
				switch runes[i] {
				case 0x07:
				case 0x1b:
					if i+1 < len(runes) {
						i++
					}
				default:
					continue
				}
				break
			}
		default:
			// Two-byte escape (charset selection etc.) — drop one follower.
			i++
		}
	}
	return cleaned.String()
}

// nonCommands never become subcommands, however command-like their row
// looks.
var nonCommands = map[string]bool{
	"help": true, "version": true, "completion": true, "man": true,
}

// DeriveSubcommands extracts subcommand entries from a listing-style help
// output.
//
// Two best-effort shapes are recognized:
//   - v-style plugin rows anywhere in the output: leading non-word glyphs
//     (emoji, punctuation) are stripped, the first token becomes the
//     candidate name, a following version-looking token is ignored, and
//     `(aliases: a, b)` groups are captured; the description comes from the
//     next indented line.
//   - cobra/go-style rows inside `Commands:` / `Available Commands:` /
//     `Subcommands:` sections: `name` + wide gap + description on the same
//     line.
//
// Flag lines, usage banners, URLs and obvious non-commands never become
// entries; names failing the generated-command charset are skipped; results
// dedupe by name, keep first-seen order, and cap at maxDerivedSubcommands.
func DeriveSubcommands(helpOutput string) []DerivedSubcommand {
	helpOutput = stripANSI(helpOutput)
	var subcommands []DerivedSubcommand
	seen := map[string]bool{}
	// The index of the most recent v-style row still awaiting its indented
	// description line.
	pending := -1
	// True while inside a Commands/Available Commands/Subcommands section,
	// enabling same-line row recognition there.
	inCommandSection := false

	for _, line := range strings.Split(helpOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Decorative rules, glyph-only rows, and obvious meta lines are
		// never entries. A pending row survives them so a description
		// following a rule line still lands.
		if isPunctuationRun(trimmed) || isMetaLine(trimmed) {
			continue
		}
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if isSectionHeader(trimmed) {
			lowered := strings.ToLower(trimmed)
			inCommandSection = strings.Contains(lowered, "command") || strings.Contains(lowered, "plugin")
			pending = -1
			continue
		}
		// Usage banners and URLs never introduce an entry.
		if strings.HasPrefix(strings.ToLower(trimmed), "usage:") || strings.HasPrefix(trimmed, "用法") ||
			strings.Contains(trimmed, "http://") || strings.Contains(trimmed, "https://") {
			continue
		}
		// Flag definitions are never subcommands.
		if strings.HasPrefix(trimmed, "-") {
			continue
		}
		// An indented follow-up line describing the previous v-style row.
		if indented && !inCommandSection {
			if pending >= 0 && subcommands[pending].Description == "" {
				subcommands[pending].Description = trimmed
			}
			pending = -1
			continue
		}
		if len(subcommands) >= maxDerivedSubcommands {
			break
		}
		remainder, aliases := extractAliases(trimmed)
		var name, description string
		var ok bool
		if inCommandSection && indented {
			name, description, ok = commandSectionRow(remainder)
		} else if !indented {
			name, description, ok = listingRow(remainder)
		}
		if !ok || nonCommands[name] || seen[name] {
			continue
		}
		seen[name] = true
		pending = len(subcommands)
		subcommands = append(subcommands, DerivedSubcommand{
			Name:        name,
			Aliases:     aliases,
			Description: description,
		})
	}
	return subcommands
}

// stripLeadingDecoration strips leading decoration from a token:
// emoji/symbol codepoints, variation selectors and other non-word glyphs. A
// meaningful leading `-` (an option marker) is never stripped.
func stripLeadingDecoration(token string) string {
	return strings.TrimLeftFunc(token, func(char rune) bool {
		return !isAlphanumeric(char) && char != '-' && char != '_'
	})
}

// isPunctuationRun reports whether a line is decorative filler: non-empty
// and carrying no alphanumeric character at all (`====`, `----`,
// box-drawing rules, lone emoji). Never a definition.
func isPunctuationRun(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	for _, char := range trimmed {
		if isAlphanumeric(char) {
			return false
		}
	}
	return true
}

// isMetaLine reports whether a line is an obvious meta banner or footer:
// `Version: …` headers and `Run … -h/--help for …` style pointers. Their
// first word is prose, never a command name or option.
func isMetaLine(trimmed string) bool {
	lowered := strings.ToLower(trimmed)
	if strings.HasPrefix(lowered, "version:") {
		return true
	}
	return strings.HasPrefix(lowered, "run ") &&
		(strings.Contains(lowered, "-h") || strings.Contains(lowered, "--help"))
}

// listingRow parses one v-style listing row into `(name, description)`. The
// description is always empty here — v-style rows take theirs from the next
// indented line, and any same-line remainder is metadata (author, homepage).
func listingRow(remainder string) (string, string, bool) {
	rest := stripLeadingDecoration(remainder)
	if rest == "" {
		return "", "", false
	}
	tokenEnd := strings.IndexAny(rest, " \t")
	if tokenEnd < 0 {
		tokenEnd = len(rest)
	}
	// Strict charset: unscoped prose lines must not slip through as
	// entries, so mixed-case tokens (`Run …`, `Version: …`) are rejected
	// outright.
	name, ok := sanitizeSubcommandName(rest[:tokenEnd], true)
	if !ok {
		return "", "", false
	}
	after := strings.TrimLeft(rest[tokenEnd:], " \t")
	// A version-looking token directly after the name (`jv 1.0.0`) is
	// ignored.
	if fields := strings.Fields(after); len(fields) > 0 && isVersionToken(fields[0]) {
		after = strings.TrimLeft(after[len(fields[0]):], " \t")
	}
	// Banner/usage shapes (`demo - Gadgets…`, `demo [options]`) are not
	// entries even though their first token looks like a valid name.
	if strings.HasPrefix(after, "-") || strings.HasPrefix(after, "[") || strings.HasPrefix(after, "<") {
		return "", "", false
	}
	return name, "", true
}

// commandSectionRow parses one cobra/go-style row (`  get    Get something`)
// into `(name, same-line description)`. It requires the wide gap that
// separates the definition column from the description column.
func commandSectionRow(line string) (string, string, bool) {
	rest := stripLeadingDecoration(line)
	if rest == "" {
		return "", "", false
	}
	tokenEnd := strings.IndexAny(rest, " \t")
	if tokenEnd < 0 {
		tokenEnd = len(rest)
	}
	name, ok := sanitizeSubcommandName(rest[:tokenEnd], false)
	if !ok {
		return "", "", false
	}
	after := rest[tokenEnd:]
	gap := len(after) - len(strings.TrimLeft(after, " \t"))
	if gap < 2 {
		return "", "", false
	}
	return name, strings.TrimSpace(after), true
}

// sanitizeSubcommandName validates and canonicalizes a candidate name
// against the generated-command charset: lowercase ASCII alphanumerics plus
// `-` and `_`, starting with a letter or digit. Trailing list punctuation
// is stripped first; when strict is set, uppercase letters are rejected
// instead of folded (used for unscoped lines where mixed case usually means
// prose rather than a command name).
func sanitizeSubcommandName(raw string, strict bool) (string, bool) {
	raw = strings.TrimRight(raw, ",:.;")
	if raw == "" {
		return "", false
	}
	var name strings.Builder
	for _, char := range raw {
		switch {
		case char >= 'A' && char <= 'Z':
			if strict {
				return "", false
			}
			name.WriteRune(char + ('a' - 'A'))
		case (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_':
			name.WriteRune(char)
		default:
			return "", false
		}
	}
	out := name.String()
	if out == "" {
		return "", false
	}
	first := rune(out[0])
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return "", false
	}
	return out, true
}

// isVersionToken reports whether a token looks like a version: digits and
// dots with at least one dot (`1.0.0`, `0.2`). Single numbers stay
// untouched so numeric subcommand names work.
func isVersionToken(token string) bool {
	if token == "" || !strings.Contains(token, ".") {
		return false
	}
	for _, char := range token {
		if (char < '0' || char > '9') && char != '.' {
			return false
		}
	}
	return true
}

// extractAliases splits a trailing `(aliases: a, b)` group off a listing
// row. It returns the remaining text plus the collected, validated alias
// list.
func extractAliases(trimmed string) (string, []string) {
	marker := strings.Index(strings.ToLower(trimmed), "(aliases")
	if marker < 0 {
		return trimmed, nil
	}
	rest := trimmed[marker:]
	offset := strings.IndexByte(rest, ')')
	if offset < 0 {
		return trimmed, nil
	}
	inner := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(rest[len("(aliases"):offset]), ":"))
	var aliases []string
	seen := map[string]bool{}
	for _, alias := range strings.Split(inner, ",") {
		alias = strings.TrimSpace(alias)
		if alias == "" || strings.ContainsAny(alias, " \t()") || seen[alias] {
			continue
		}
		seen[alias] = true
		aliases = append(aliases, alias)
	}
	return trimmed[:marker] + trimmed[marker+offset+1:], aliases
}
