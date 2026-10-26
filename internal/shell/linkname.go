package shell

import "strings"

// maxCommandNameChars bounds a `floter://register` name the way the reference
// bounded its `cmd` parameter (`MAX_COMMAND_CHARS`,
// main:src-tauri/src/deep_link.rs:116). 64 characters is longer than any real
// command name and short enough that a hostile link cannot make the search path
// walk do work for a value that could never resolve.
const maxCommandNameChars = 64

// bareCommandName is the one answer to "what may a register link name": a bare
// command name, never a path.
//
// It is the reference's `validate_command` (main:src-tauri/src/deep_link.rs:364-390),
// rule for rule: trim, non-empty, at most `maxCommandNameChars`, not `..` (so it
// is not a traversal), no leading `-` (a leading dash is an option to whatever
// consumes the value, not a name), and only `[A-Za-z0-9._+-]`. Its test refuses
// exactly these spellings (`deep_link.rs:1465-1487`): `/usr/bin/rg`, `../rg`,
// `rg;rm -rf /`, `rg --exec=ls`, `rg\nx`, `-rg`, `rg$(x)`.
//
// The reference spells its "one path component" rule as a separate
// `contains('/') || contains('\\')` check *and* the character set; here the set
// is the one answer, because this build's refusal carries no reason string. The
// reference needed the split only to name the refusal (`CommandNotBasename` vs
// `CommandInsecure`, `deep_link.rs:219-252`), and both are one sentence here
// (`settings.deepLinkRejected`). Two checks for one verdict would be two places
// to keep in step, and the set already refuses both separators.
//
// Two consequences worth stating, because they are the point of the rule:
//
//   - A link cannot name a file by path at all. The reference's register
//     resolves the name through the discovery inventory and its resolver
//     (`deep_link.rs:1136-1180` → `resolver.rs:63-77`), so a path that is not on
//     the search path has no candidate and never had a branch of its own. This
//     build used to take an absolute path as it was (`system.go:252-260`, no
//     "is this a program" question at all): `floter://register?cmd=C:\notes.txt`
//     connected a text file. Refusing at the router is the reference's answer,
//     and it needs no new sentence — the refusal is the one the router already
//     raises (`Reject::is_silent` is false for `CommandNotBasename`,
//     `deep_link.rs:225-228`).
//   - The rule is structural, not a lookup: a name that passes may still resolve
//     to nothing, and that is the "not found" path (`registerFromLink`), not a
//     refusal. The two answers stay two answers.
//
// The reference's `value.split('.').any(|segment| segment == "..")` half is
// dead code — a split on `.` can never yield a segment containing `.` — so only
// its `value == ".."` clause can fire, and that is what is ported here.
func bareCommandName(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > maxCommandNameChars {
		return "", false
	}
	if value == ".." {
		return "", false
	}
	if strings.HasPrefix(value, "-") {
		return "", false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' {
			continue
		}
		switch character {
		case '.', '_', '+', '-':
			continue
		}
		return "", false
	}
	return value, true
}
