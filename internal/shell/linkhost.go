package shell

import (
	"net/url"
	"strings"
)

// launcherPrefill is the one place that answers "what does this link put in the
// search box". The prefill is this build's own contract, not the reference's:
// the old build has no launcher action at all (`ACTIONS`, `deep_link.rs:104`),
// and it spells the launcher's two host forms out in
// `docs/mygo-rewrite-plan.md:226` — `floter://search?q=…` and `floter://?q=…`.
// Everything else that reaches the router's fallback opens the launcher and
// **leaves the query alone** (same line of the plan), which is also what the
// reference does with an action it does not know: `Reject::UnknownAction` is
// silent (`deep_link.rs:223-228`) and the query loop never runs (`:467-492`).
//
// The value's own order is the old spelling's: `q` first, `query` second, and
// an empty value counts as absent — a link that asks for nothing must not clear
// what the user is already typing.
func launcherPrefill(parsed *url.URL) string {
	if parsed == nil || !launcherHost(parsed) {
		return ""
	}
	query := parsed.Query()
	if value := query.Get("q"); value != "" {
		return value
	}
	return query.Get("query")
}

// launcherHost reports whether a link's host is one of this build's launcher
// spellings. Three shapes, and only these three:
//
//   - `search` — the documented prefill host.
//   - the bare launcher, written as an empty authority with nothing after it
//     (`floter://?q=…`, and the `floter:///?q=…` a shell hands over when it
//     adds the slash). A path makes it a different spelling, not this one:
//     `floter:///search?q=…` is neither documented nor the reference's.
//   - nothing at all — but `Opaque` says a URL like `floter:connect` never had
//     an authority to begin with, so it is the reference's `MalformedUrl`
//     (`deep_link.rs:453-456`) rather than the bare launcher, and it prefills
//     nothing. It still falls back to the launcher, which is this build's
//     documented answer for a link it cannot use (`mygo-rewrite-plan.md:226`);
//     the reference's is a refusal toast, and that divergence is left where it
//     is rather than widened here.
//
// The host is lower-cased here because `net/url` only folds the case of the
// schemes it treats as special: `FLOTER://SEARCH` parses with `Host = "SEARCH"`
// (measured), while the router's own action test already lower-cases
// (`system.go:198`). Two answers to "which host is this" would drift.
func launcherHost(parsed *url.URL) bool {
	if parsed.Opaque != "" {
		return false
	}
	switch strings.ToLower(parsed.Host) {
	case "search":
		return true
	case "":
		return parsed.Path == "" || parsed.Path == "/"
	}
	return false
}
