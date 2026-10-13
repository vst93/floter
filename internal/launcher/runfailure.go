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
// from that path — plain prose — is returned verbatim, which is strictly
// better than swallowing it.
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

// paramRunMessage words a `run_param_*:{id}` refusal — the run-time parameter
// form's own keys, which are not the `key:{json}` run-failure shape and so are
// not claimed by `ParseRunFailure`. The declaration is not in the launcher's
// hands (an entry carries argv, not the manifest's parameters), so the id is
// what the sentence interpolates — the old `paramRunErrorMessage`'s own
// fallback when no declared parameter matched the id.
func paramRunMessage(copy i18n.Settings, failure *extensions.ParamFailure) string {
	switch failure.Key {
	case extensions.RunErrorParamRequired:
		return copy.RunParamRequired(failure.Param)
	case extensions.RunErrorParamInvalid:
		return copy.RunParamInvalid(failure.Param)
	case extensions.RunErrorParamUnknown:
		return copy.RunParamUnknown(failure.Param)
	case extensions.RunErrorParamWindowsUnsafe:
		return copy.RunParamWindowsUnsafe(failure.Param)
	default:
		return failure.Error()
	}
}
