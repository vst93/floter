package launcher

import (
	"strconv"
	"strings"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// runErrorMessage is the readable half of a failed run: the keyed failure the
// backend recorded, turned into a sentence in the stored language.
//
// The split is the old host's: the backend owns the facts (a stable key and a
// JSON payload) and the surface owns the words. A message that did not come
// from that path — plain prose, or a key-plus-id such as
// `run_param_required:note` — is returned verbatim, which is strictly better
// than swallowing it.
func runErrorMessage(copy i18n.Settings, failure *extensions.RunFailure) string {
	switch failure.Key {
	case extensions.RunErrorScriptMissing:
		return copy.RunErrorScriptMissing(failure.String("path"))
	case extensions.RunErrorProgramMissing:
		return copy.RunErrorProgramMissing(failure.String("path"))
	case extensions.RunErrorProgramNotExecutable:
		return copy.RunErrorNotExecutable(failure.String("path"))
	case extensions.RunErrorInterpreterMissing:
		// The searched directories are a vertical list, as the old formatter
		// joined them, and the candidate names a horizontal one.
		return copy.RunErrorInterpreterMissing(
			failure.String("language"),
			strings.Join(failure.Strings("names"), " / "),
			strings.Join(failure.Strings("searched"), "\n"),
		)
	case extensions.RunErrorSpawnFailed:
		return copy.RunErrorSpawnFailed(failure.String("path"), failure.String("detail"))
	case extensions.RunErrorTimeout:
		return copy.RunErrorTimeout(strconv.Itoa(failure.Int("seconds")))
	case extensions.RunErrorKilled:
		return copy.RunErrorKilled(failure.String("extension_id"))
	case extensions.RunErrorIntegrationDisabled:
		return copy.RunErrorIntegrationDisabled(failure.String("extension_id"))
	case extensions.RunErrorIntegrationBroken:
		return copy.RunErrorIntegrationBroken(failure.String("extension_id"), failure.String("reason"))
	case extensions.RunErrorTaskFailed:
		return copy.RunErrorTaskFailed(failure.String("error"))
	default:
		return failure.Error()
	}
}
