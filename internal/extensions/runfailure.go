package extensions

import (
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
)

// The keyed half of a failed run.
//
// A run failure is a *stable key plus its facts* — never a sentence — so the
// surface that shows it owns the words and both languages stay symmetric. The
// shape is the old host's (`src-tauri/src/extensions/run_error.rs`): the
// message is `key:{json}`, and the frontend maps the key to a localised
// sentence. In this build the same split holds: `runCaptured` emits a
// RunFailure, and the launcher (the only surface that renders one) turns it
// into words.

// RunErrorKey names one kind of run failure. The set is closed and every key
// has exactly one sentence per language; a key the UI does not know is a
// programming error, not a new message shape.
type RunErrorKey string

const (
	// RunErrorScriptMissing: the manifest's script file is gone.
	RunErrorScriptMissing RunErrorKey = "run_script_missing"
	// RunErrorProgramMissing: the program the plan would spawn does not exist.
	RunErrorProgramMissing RunErrorKey = "run_program_missing"
	// RunErrorProgramNotExecutable: the program exists but is not executable.
	RunErrorProgramNotExecutable RunErrorKey = "run_program_not_executable"
	// RunErrorInterpreterMissing: no interpreter for the script language was
	// found, with the candidate names and the directories that were searched.
	RunErrorInterpreterMissing RunErrorKey = "run_interpreter_missing"
	// RunErrorSpawnFailed: the OS refused the spawn for a reason that is not one
	// of the named cases; the OS's own text is kept verbatim.
	RunErrorSpawnFailed RunErrorKey = "run_spawn_failed"
	// RunErrorTimeout: the run exceeded its budget and was stopped.
	RunErrorTimeout RunErrorKey = "run_timeout"
	// RunErrorKilled: the run was stopped because its integration was disabled
	// or uninstalled, and names which one.
	RunErrorKilled RunErrorKey = "run_killed"
	// RunErrorIntegrationDisabled: a run was refused because the integration is
	// disabled.
	RunErrorIntegrationDisabled RunErrorKey = "run_integration_disabled"
	// RunErrorIntegrationBroken: a run was refused because the integration is
	// broken, carrying the recorded reason.
	RunErrorIntegrationBroken RunErrorKey = "run_integration_broken"
	// RunErrorTaskFailed: the background task failed to complete at all.
	RunErrorTaskFailed RunErrorKey = "run_task_failed"
)

// runErrorKeys is the closed set, in the old host's order. Only these keys are
// ever accepted from a message; a `run_…` key outside it is prose that happens
// to look like a key and is left alone.
var runErrorKeys = []RunErrorKey{
	RunErrorScriptMissing,
	RunErrorProgramMissing,
	RunErrorProgramNotExecutable,
	RunErrorInterpreterMissing,
	RunErrorSpawnFailed,
	RunErrorTimeout,
	RunErrorKilled,
	RunErrorIntegrationDisabled,
	RunErrorIntegrationBroken,
	RunErrorTaskFailed,
}

// KnownRunErrorKey reports whether the key is one of the closed set.
func KnownRunErrorKey(key RunErrorKey) bool {
	for _, candidate := range runErrorKeys {
		if candidate == key {
			return true
		}
	}
	return false
}

// RunFailure is one run's failure: the key the UI localises, and the facts its
// sentence interpolates.
type RunFailure struct {
	Key RunErrorKey
	// Payload holds the key's fields (path, language, seconds, …). A field the
	// key does not carry is absent rather than blank, so the renderer reads
	// what is there instead of a fabricated zero.
	Payload map[string]any
}

// NewRunFailure builds one. The payload is copied so a caller's map cannot
// drift under the failure it handed off.
func NewRunFailure(key RunErrorKey, payload map[string]any) *RunFailure {
	copied := make(map[string]any, len(payload))
	for name, value := range payload {
		copied[name] = value
	}
	return &RunFailure{Key: key, Payload: copied}
}

// PathFailure is the shape three keys share: a single `path` fact.
func PathFailure(key RunErrorKey, path string) *RunFailure {
	return NewRunFailure(key, map[string]any{"path": path})
}

// Error formats the failure the way the old host did: `key:{json}`. A payload
// that cannot be marshalled falls back to `key:{}` — the key is never lost.
func (f *RunFailure) Error() string {
	body, err := json.Marshal(f.Payload)
	if err != nil {
		body = []byte("{}")
	}
	return string(f.Key) + ":" + string(body)
}

// String is the payload's string field, blank when it is absent or not a
// string.
func (f *RunFailure) String(name string) string {
	value, _ := f.Payload[name].(string)
	return value
}

// Strings is a payload's list-of-strings field, nil when it is absent.
func (f *RunFailure) Strings(name string) []string {
	raw, ok := f.Payload[name].([]any)
	if !ok {
		if already, ok := f.Payload[name].([]string); ok {
			return already
		}
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			values = append(values, text)
		}
	}
	return values
}

// Int is a payload's integer field, 0 when it is absent. JSON numbers arrive
// as float64, so both spellings are accepted.
func (f *RunFailure) Int(name string) int {
	switch value := f.Payload[name].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}

// RunSpawnFailure maps an OS spawn error to the key the user can act on. A
// missing file and a permission refusal each have a distinct remedy, so they
// are named; everything else keeps the OS's own text rather than hiding it
// behind a generic sentence.
func RunSpawnFailure(program string, err error) *RunFailure {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return PathFailure(RunErrorProgramMissing, program)
	case errors.Is(err, fs.ErrPermission):
		return PathFailure(RunErrorProgramNotExecutable, program)
	default:
		detail := ""
		if err != nil {
			detail = err.Error()
		}
		return NewRunFailure(RunErrorSpawnFailed, map[string]any{"path": program, "detail": detail})
	}
}

// RunTimeoutFailure names how long the run was allowed, in whole seconds.
func RunTimeoutFailure(seconds int) *RunFailure {
	return NewRunFailure(RunErrorTimeout, map[string]any{"seconds": seconds})
}

// RunKilledFailure names the integration whose disable or uninstall stopped the
// run, the way the old host's run_killed did.
func RunKilledFailure(id string) *RunFailure {
	return NewRunFailure(RunErrorKilled, map[string]any{"extension_id": id})
}

// ParseRunFailure reads a keyed message back. It is deliberately conservative,
// exactly as the old frontend parser was: the key must start with `run_` and
// be one of the closed set, and the rest must parse as a JSON object. A plain
// prose error, or a key-plus-id message such as `run_param_required:note`, is
// not claimed here.
func ParseRunFailure(message string) (*RunFailure, bool) {
	trimmed := strings.TrimSpace(message)
	separator := strings.Index(trimmed, ":")
	if separator < 0 {
		return nil, false
	}
	key := RunErrorKey(trimmed[:separator])
	if !KnownRunErrorKey(key) {
		return nil, false
	}
	body := strings.TrimSpace(trimmed[separator+1:])
	if !strings.HasPrefix(body, "{") {
		return nil, false
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, false
	}
	return &RunFailure{Key: key, Payload: payload}, true
}
