package shell

import (
	"errors"
	"strings"
)

// errNonWebURL is the generic opener's refusal for anything outside the web
// schemes, the old host's `settings.extensions.openNonWebUrl` case.
var errNonWebURL = errors.New("shell: refusing to open a non-web URL")

// webSchemes is the old `open_url` allow-list: the three schemes a link may be
// followed through. The action bar's own `URLQuery` already restricts what it
// offers, but the generic opener is also reached by an integration's homepage
// and a plugin output row, and those must not become a wider door
// (`src-tauri/src/commands/actions.rs:44-68`).
var webSchemes = []string{"http://", "https://", "ftp://"}

// isWebURL reports whether raw begins with one of the three web schemes,
// case-insensitively. The length guard is the Go spelling of the old
// `url.get(..scheme.len())`: a short string can never be sliced past its end.
func isWebURL(raw string) bool {
	for _, scheme := range webSchemes {
		if len(raw) >= len(scheme) && strings.EqualFold(raw[:len(scheme)], scheme) {
			return true
		}
	}
	return false
}
