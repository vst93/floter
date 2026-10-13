package extensions

import (
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// The run-time half of the parameter feature: the declaration plus the
// caller's answers become argv, and a combination the manifest would not allow
// is refused with a stable key rather than handed on.
//
// This is the only argv builder. The old host kept it in one function
// (`param_arguments`, `src-tauri/src/extensions/run.rs:295`) precisely so no
// second builder could drift from it; the same rule holds here — `ParamArgv`
// used to be a second, silent builder, and it is gone.
//
// The refusal keys carry the parameter id instead of a JSON payload
// (`run_param_required:note`), the shape the old host emitted and the frontend
// parsed with its own regexp. `ParseRunFailure` deliberately does not claim
// them: they are not the `key:{json}` run-failure shape.

const (
	// RunErrorParamUnknown: a value was keyed by an id no parameter declares.
	RunErrorParamUnknown RunErrorKey = "run_param_unknown"
	// RunErrorParamRequired: a required parameter has neither a value nor a
	// default.
	RunErrorParamRequired RunErrorKey = "run_param_required"
	// RunErrorParamInvalid: the value does not match its kind, or the flag is
	// not one argv token.
	RunErrorParamInvalid RunErrorKey = "run_param_invalid"
	// RunErrorParamWindowsUnsafe: the value carries a character cmd.exe would
	// re-parse, and the program is a `.cmd`/`.bat` script launched through it.
	RunErrorParamWindowsUnsafe RunErrorKey = "run_param_windows_unsafe"
)

// ParamFailure is a refused run, naming the parameter that caused it. The
// surface resolves the label (and the wording) from the id.
type ParamFailure struct {
	Key   RunErrorKey
	Param string
}

// Error formats the refusal the way the old host did: `key:{id}`.
func (f *ParamFailure) Error() string { return string(f.Key) + ":" + f.Param }

// ParseParamFailure reads a `run_param_*:{id}` message back. It mirrors the
// old frontend's regexp: the key must be one of the four and the id must be
// non-empty. A JSON-payload run failure is not claimed here.
func ParseParamFailure(message string) (*ParamFailure, bool) {
	trimmed := strings.TrimSpace(message)
	separator := strings.Index(trimmed, ":")
	if separator < 0 {
		return nil, false
	}
	key := RunErrorKey(trimmed[:separator])
	switch key {
	case RunErrorParamUnknown, RunErrorParamRequired, RunErrorParamInvalid, RunErrorParamWindowsUnsafe:
	default:
		return nil, false
	}
	id := trimmed[separator+1:]
	if id == "" {
		return nil, false
	}
	return &ParamFailure{Key: key, Param: id}, true
}

// windowsCmdUnsafeChars are the characters cmd.exe re-parses. A `.cmd`/`.bat`
// program is launched through `cmd.exe /D /S /C`, so a value containing any of
// these can still change the command line even though it arrived as its own
// argv entry. The structured-argv guarantee does not extend across cmd.exe, so
// the run refuses such a value rather than pretending it is safe.
var windowsCmdUnsafeChars = []rune{'&', '|', '<', '>', '^', '%'}

// WindowsCmdUnsafeChar is the first character in value cmd.exe would re-parse.
func WindowsCmdUnsafeChar(value string) (rune, bool) {
	for _, char := range value {
		for _, unsafe := range windowsCmdUnsafeChars {
			if char == unsafe {
				return char, true
			}
		}
	}
	return 0, false
}

// IsCmdScriptName reports whether a program's own file name ends in the
// `.cmd`/`.bat` extension cmd.exe re-parses. It is a pure spelling check, kept
// separate from the platform gate so it stays testable on every platform.
func IsCmdScriptName(program string) bool {
	name := program
	if index := strings.LastIndexAny(name, `/\`); index >= 0 {
		name = name[index+1:]
	}
	lowered := strings.ToLower(name)
	return strings.HasSuffix(lowered, ".cmd") || strings.HasSuffix(lowered, ".bat")
}

// ProgramIsCmdScript reports whether a program is a Windows command script,
// which the execution host launches through cmd.exe. Always false off Windows:
// the wrap only happens there, so only there can a value be re-parsed.
func ProgramIsCmdScript(program string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return IsCmdScriptName(program)
}

// finiteDecimalPattern is the number kind's value grammar, the same regexp the
// old frontend used (`FINITE_DECIMAL_PATTERN`, `src/extensions/script-params.ts`).
// It is deliberately not `strconv.ParseFloat`: that also accepts the
// radix-prefixed and non-finite spellings (`0x1p-2`, `inf`, `NaN`) and
// underscores, none of which the old host's `parse::<f64>` accepted as a plain
// decimal.
var finiteDecimalPattern = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// IsFiniteDecimal reports whether a value is a finite decimal literal,
// whitespace aside.
func IsFiniteDecimal(value string) bool {
	return finiteDecimalPattern.MatchString(strings.TrimSpace(value))
}

// ParamArguments turns the declared parameters plus the caller's answers into
// argv items, or refuses with a `run_param_*` key.
//
// The order is declaration order, and the shape per parameter is: `flag` then
// `value` as two separate items when a flag is declared, `value` alone when it
// is not, `flag` alone for a boolean that is true, and nothing for a boolean
// that is false or absent. An absent non-boolean uses its default, and an
// absent required parameter without a default is refused.
//
// Values are converted per kind: `number` must be a finite decimal, `select`
// must be one of its options, `path` and `text` pass through untouched (no
// `$HOME`/`~` expansion). Every value is its own element; nothing here ever
// joins an argument into a shell string, which is the injection defence.
func ParamArguments(params []ParamDefinition, values ParamValues, cmdScript bool) ([]string, error) {
	// A value keyed by an id that is not declared is refused: silently
	// dropping it would hide a caller bug, and treating it as free-form would
	// be an injection path. The ids are visited in sorted order so the refusal
	// is deterministic.
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !declaresParam(params, key) {
			return nil, &ParamFailure{Key: RunErrorParamUnknown, Param: key}
		}
	}

	var args []string
	for _, param := range params {
		raw, supplied := values[param.ID]
		if !supplied || raw == "" {
			if param.Default != "" {
				raw = param.Default
			} else if param.Required {
				return nil, &ParamFailure{Key: RunErrorParamRequired, Param: param.ID}
			} else {
				continue
			}
		}
		if param.Flag != "" && validateParamFlag(param.Flag) != nil {
			return nil, &ParamFailure{Key: RunErrorParamInvalid, Param: param.ID}
		}
		if cmdScript {
			if _, unsafe := WindowsCmdUnsafeChar(raw); unsafe {
				return nil, &ParamFailure{Key: RunErrorParamWindowsUnsafe, Param: param.ID}
			}
		}
		switch param.Kind {
		case ParamBoolean:
			switch strings.ToLower(strings.TrimSpace(raw)) {
			case "true":
				if param.Flag != "" {
					args = append(args, param.Flag)
				}
			case "false":
			default:
				return nil, &ParamFailure{Key: RunErrorParamInvalid, Param: param.ID}
			}
		case ParamNumber:
			trimmed := strings.TrimSpace(raw)
			if !IsFiniteDecimal(trimmed) {
				return nil, &ParamFailure{Key: RunErrorParamInvalid, Param: param.ID}
			}
			args = pushFlagged(args, param.Flag, trimmed)
		case ParamSelect:
			if !containsString(param.Options, raw) {
				return nil, &ParamFailure{Key: RunErrorParamInvalid, Param: param.ID}
			}
			args = pushFlagged(args, param.Flag, raw)
		default:
			args = pushFlagged(args, param.Flag, raw)
		}
	}
	return args, nil
}

// pushFlagged appends the flag (when present) and the value as two separate
// argv items — never one joined string.
func pushFlagged(args []string, flag, value string) []string {
	if flag != "" {
		args = append(args, flag)
	}
	return append(args, value)
}

// declaresParam reports whether the declaration names the id.
func declaresParam(params []ParamDefinition, id string) bool {
	for _, param := range params {
		if param.ID == id {
			return true
		}
	}
	return false
}

// containsString reports whether the list holds the value.
func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
