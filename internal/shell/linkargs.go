package shell

import (
	"net/url"
	"strings"
)

// linkActions is the command line's allow-list, the same three names the
// `floter://` router answers (HandleURL) and the same three the old build
// listed in `ACTIONS` (`src-tauri/src/deep_link.rs:104`): one list, two
// spellings. Anything else on the command line is a word for somebody else
// (a shell, a launcher, an editor) and stays quiet.
var linkActions = map[string]bool{"open": true, "connect": true, "register": true}

// LinkFromArguments turns a launch's command line into the URL the deep link
// router answers, or reports that the line names no action.
//
// args are the arguments after the program name: `os.Args[1:]` for the launch
// that started the app, and the arguments a second launch forwarded (mygo's
// `OnSecondInstance`, `single_instance.go:71`). The extension guide documents
// the terminal spelling next to the URL one — `floter register rg` beside
// `floter://register?cmd=rg` (`docs/extensions/README.zh-CN.md:100`) — and the
// old build normalized it in `canonical_argument` (`deep_link.rs:556`); this
// is that normalizer, and the URL it builds goes through the one router that
// already validates every field.
//
// Two deliberate differences from the old build, both because this build's
// router has no reader for the extra words:
//
//   - The rest of a `register` line (`floter register rg --hidden src`) is not
//     folded into an `args=` hint: registerFromLink takes the name and nothing
//     else (system.go:249), and the old build only carried `args` for display.
//   - `--yes` is a flag like any other, so `floter --yes register rg` and
//     `floter register rg` are the same line. The old build folded it into
//     `confirm=1` because its terminal spelling could *finish* a connection;
//     this build's register link already runs the one-press connect path
//     (system.go:236-238), so there is no second step for a confirmation to
//     skip.
//
// A value-less action (`floter connect`, `floter register`) is still a link:
// the router is the only place that decides what each action may carry, and it
// refuses those with one sentence (system.go:196-215).
func LinkFromArguments(args []string) (string, bool) {
	words := make([]string, 0, len(args))
	for _, argument := range args {
		argument = strings.TrimSpace(argument)
		if argument == "" {
			continue
		}
		// A URL of a scheme this app handles is the framework's to deliver:
		// mygo scans the launch's arguments and those a second instance
		// forwarded for them and fires OnOpenURL (deeplink.go:80,185). Handing
		// the same argument back here would route one launch twice — two
		// refusals, two connect attempts.
		if scheme, _, ok := strings.Cut(argument, "://"); ok && strings.EqualFold(scheme, "floter") {
			return "", false
		}
		// Flags are skipped rather than rejected: `--background`, `--yes` and
		// macOS's own `-psn_0_…` all ride a launch the user did not spell.
		if strings.HasPrefix(argument, "-") {
			continue
		}
		words = append(words, argument)
	}
	if len(words) == 0 {
		return "", false
	}
	action := strings.ToLower(words[0])
	if !linkActions[action] {
		return "", false
	}
	value := ""
	if len(words) > 1 {
		value = words[1]
	}
	switch action {
	case "connect":
		if value == "" {
			return scheme + "://connect", true
		}
		return scheme + "://connect?manifest=" + url.QueryEscape(value), true
	case "register":
		if value == "" {
			return scheme + "://register", true
		}
		return scheme + "://register?cmd=" + url.QueryEscape(value), true
	}
	return scheme + "://" + action, true
}
