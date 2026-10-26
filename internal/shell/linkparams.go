package shell

import "net/url"

// linkParamSlots is the reference's parameter table, one row per action it
// routes: which query keys an action may carry, and which slot each key fills
// (`main:src-tauri/src/deep_link.rs:467-492`). The loop there accepts a key only
// when the action names it *and* its slot is still empty, and sends every other
// key to `Reject::UnknownParameter` (`:491`). That refusal is not one of the
// silent ones (`is_silent`, `:225-228`), so a user who clicked the link is told
// nothing happened — with the one sentence every refused link already gets
// (`settings.deepLinkRejected`, `refuseLink`).
//
// The table is the whole rule. Two keys that fill the same slot are the same
// value spelled twice, which the reference refuses like any other second
// occurrence (`:472` `command.is_none()`), so `?cmd=rg&tool=fd` is refused even
// though both keys exist. A key with an empty slot is a flag: it carries no
// value, so a repeat is inert (`confirm`, `:487`).
//
// Two entries have no reader in this build, on purpose:
//
//   - `args` is the optional argument *hint* the extension guide promises
//     (`docs/extensions/README.zh-CN.md:97-104`). The reference shows it on the
//     review surface and never runs it; this build has no surface for it, so it
//     is accepted and dropped. Refusing it would refuse a link the reference
//     accepts, and the guide already told users it is allowed. Being in this
//     table is not the whole rule though: the *value* has its own gate
//     (`linkArgsFit`, linkargshint.go), because the reference judges a hint it
//     never runs too.
//   - `confirm` is the terminal spelling's `--yes` folded into a query parameter
//     (`deep_link.rs:487`). This build's register link already runs the
//     one-press connect path, so there is no second step for a confirmation to
//     skip (`linkargs.go:36-42`) — the key is accepted so a link written for the
//     reference still routes, and read by nobody.
//
// The hosts this build adds are *not* rows here: `settings`, `terminal`, the
// bare launcher and `search` are not reference actions at all (`ACTIONS`,
// `deep_link.rs:104`), and their `q`/`query` prefill is this build's own
// contract (`docs/mygo-rewrite-plan.md:226`). The reference's `open` takes no
// parameter whatsoever; that divergence is pinned rather than applied, because
// applying it would delete a documented feature (`search?q=…`).
var linkParamSlots = map[string]map[string]string{
	"connect": {
		"manifest": "manifest",
	},
	"register": {
		// One name, three spellings (`system.go`), one slot.
		"cmd":     "name",
		"tool":    "name",
		"program": "name",
		"args":    "args",
		"confirm": "",
	},
}

// linkQueryFits reports whether a link's query is the shape its action's table
// describes: every key named by the row, every value-bearing slot filled at most
// once. It is the reference's `parse_url` loop reduced to its verdict, and it
// says nothing about the *values* — a bare command name is `bareCommandName`'s
// question (`linkname.go`), a manifest path is the connect flow's.
//
// An action with no row is not a reference action, and this function has no
// opinion about it: the caller's own branch decides what its query means.
func linkQueryFits(action string, query url.Values) bool {
	slots, routed := linkParamSlots[action]
	if !routed {
		return true
	}
	filled := make(map[string]bool, len(query))
	for key, values := range query {
		slot, known := slots[key]
		if !known {
			return false
		}
		if slot == "" {
			// A flag: nothing to fill, so a repeat is not a second value.
			continue
		}
		if len(values) > 1 || filled[slot] {
			return false
		}
		filled[slot] = true
	}
	return true
}
