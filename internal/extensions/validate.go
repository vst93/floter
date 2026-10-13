package extensions

import (
	"errors"
	"fmt"
	"strings"
)

// The manifest's own structural rules, the half the old host enforced while
// *parsing* a manifest rather than while using it.
//
// The old host validated every declared path the moment a manifest was read
// (`ExtensionManifest::parse` → `validate_paths` →
// `validate_relative_path`, `src-tauri/src/extensions/manifest.rs:492,565,606`),
// so a package that pointed at `/etc/passwd`, `../../secret` or a Windows
// drive root could not even be loaded — let alone exported, installed or run.
// This build parsed the same fields without asking, and the gap was not
// theoretical: `sync.go` wrote `runtime.path` and `provider.descriptor`
// straight into the staging directory with `filepath.Join`, so a package could
// escape it on import.
//
// The rule is the old host's `validate_relative_path`: non-empty, relative,
// and built only from ordinary components — no root, no drive prefix, no `.`,
// no `..`. It is deliberately stricter than "stays inside the package" (which
// `joinWithin` answers at use time): a declared path is *data from a package*,
// and a package has no business naming a parent directory even when it happens
// to resolve back inside.

// ErrBadManifest is what a structural manifest validator reports.
var ErrBadManifest = errors.New("extensions: the manifest is not usable")

// ValidateRelativePath checks a manifest-declared path is one that stays
// inside the package that declares it. `field` names the field for the error.
//
// An empty value is refused: every field this is used on is required when it
// is present at all, and a blank path is a bug rather than "the default". The
// caller that treats absence as meaningful (an omitted `icon`, say) skips the
// call instead.
func ValidateRelativePath(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s must be a non-empty relative path", ErrBadManifest, field)
	}
	if hasRootPrefix(value) {
		return fmt.Errorf("%w: %s must be a relative path, got %q", ErrBadManifest, field, value)
	}
	// Split on both separators, so a manifest authored on Windows and one
	// authored on Unix are held to the same rule on either host. Empty
	// components are dropped, exactly as `Path::components` collapses `a//b`.
	for _, component := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if component == "." || component == ".." {
			return fmt.Errorf("%w: %s contains an unsafe path component: %s", ErrBadManifest, field, value)
		}
	}
	return nil
}

// hasRootPrefix reports whether a path is rooted: a leading separator, or a
// Windows drive letter (`C:\…`, `C:/…`). `filepath.IsAbs` is host-specific, so
// the drive spelling is checked by hand and the rule is the same everywhere.
func hasRootPrefix(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return true
	}
	if len(value) >= 2 && value[1] == ':' {
		if c := value[0]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

// Validate applies every structural rule the old host enforced while parsing
// a manifest: the relative-path rule on every declared path (`validate_paths`,
// `manifest.rs:565`) and the lifecycle's count, combination and range rules
// (`ToolLifecycle::validate`, `lifecycle.rs:39`). A manifest that fails here
// never reaches an export, an install or a run.
func (m Manifest) Validate() error {
	if err := m.validateDeclaredPaths(); err != nil {
		return err
	}
	return m.Lifecycle.Validate()
}

// ValidatePaths applies the relative-path rule to every path a manifest
// declares: the icon, a script runtime's source, a static provider's
// descriptor, a completion's source file, a configuration template's source
// and target, and a launch command's program. It is the path half of
// Validate, kept for callers that want only that rule.
func (m Manifest) ValidatePaths() error {
	if err := m.validateDeclaredPaths(); err != nil {
		return err
	}
	return m.Lifecycle.ValidatePaths()
}

// validateDeclaredPaths is the manifest's own paths (everything outside the
// lifecycle).
func (m Manifest) validateDeclaredPaths() error {
	if m.Icon != "" {
		if err := ValidateRelativePath(m.Icon, "icon"); err != nil {
			return err
		}
	}
	if m.Runtime.Type == "script" {
		if err := ValidateRelativePath(m.Runtime.Path, "runtime script"); err != nil {
			return err
		}
	}
	if m.Provider.Descriptor != "" {
		if err := ValidateRelativePath(m.Provider.Descriptor, "provider descriptor"); err != nil {
			return err
		}
	}
	return nil
}

// The shells a completion may target, the old host's `Shell` enum
// (`lifecycle.rs:253`, `rename_all = "lowercase"`).
var validShells = map[string]bool{
	"bash": true, "zsh": true, "fish": true, "powershell": true,
}

// Validate applies the lifecycle rules the old host ran while parsing a
// manifest (`ToolLifecycle::validate`, `lifecycle.rs:39`): at most one
// completion per shell, a completion declaring exactly one of source or args,
// bounded argument counts and timeouts, unique template targets and probe ids,
// a safe completion file name, and a launch with a known restore policy and a
// NUL-free program and arguments. The declared paths are checked first.
func (l Lifecycle) Validate() error {
	if err := l.ValidatePaths(); err != nil {
		return err
	}

	shells := map[string]bool{}
	for _, completion := range l.Completions {
		if !validShells[completion.Shell] {
			return fmt.Errorf("%w: unknown completion shell %q", ErrBadManifest, completion.Shell)
		}
		if shells[completion.Shell] {
			return fmt.Errorf("%w: lifecycle declares more than one %s completion", ErrBadManifest, completion.Shell)
		}
		shells[completion.Shell] = true

		// Exactly one of source or args: the old host validated the source path
		// when it was the only input, and refused the pair when both or neither
		// were given.
		hasSource := completion.Source != ""
		hasArgs := len(completion.Args) > 0
		if hasSource == hasArgs {
			return fmt.Errorf("%w: %s completion must declare exactly one of source or args", ErrBadManifest, completion.Shell)
		}
		if len(completion.Args) > 16 {
			return fmt.Errorf("%w: completion generation accepts at most 16 arguments", ErrBadManifest)
		}
		if timeout := completion.EffectiveTimeoutMs(); timeout < 100 || timeout > 10000 {
			return fmt.Errorf("%w: completion timeout must be between 100 and 10000 ms", ErrBadManifest)
		}
		if completion.FileName != "" {
			if err := validateCompletionFileName(completion.FileName); err != nil {
				return err
			}
		}
	}

	targets := map[string]bool{}
	for _, template := range l.ConfigurationTemplates {
		if targets[template.Target] {
			return fmt.Errorf("%w: lifecycle declares configuration target %s more than once", ErrBadManifest, template.Target)
		}
		targets[template.Target] = true
	}

	probeIDs := map[string]bool{}
	for _, probe := range l.Probes {
		if probeIDs[probe.ID] {
			return fmt.Errorf("%w: lifecycle declares probe %q more than once", ErrBadManifest, probe.ID)
		}
		probeIDs[probe.ID] = true
		if timeout := probe.EffectiveTimeoutMs(); timeout < 100 || timeout > 30000 {
			return fmt.Errorf("%w: probe %q timeout must be between 100 and 30000 ms", ErrBadManifest, probe.ID)
		}
		if len(probe.Args) > 32 {
			return fmt.Errorf("%w: probe %q accepts at most 32 arguments", ErrBadManifest, probe.ID)
		}
	}

	return l.Launch.Validate()
}

// Validate applies the launch rules the old host ran (`LaunchConfig::validate`
// and `LaunchCommand::validate`, `lifecycle.rs:119,135`): a known restore
// policy, a NUL-free program and arguments, and a program that is either
// `self` or a relative path (already checked by ValidatePaths). An absent
// program or restore policy is the schema's default.
func (l Launch) Validate() error {
	switch l.EffectiveRestorePolicy() {
	case "reattach", "restart", "none":
	default:
		return fmt.Errorf("%w: lifecycle.launch.restorePolicy must be 'reattach', 'restart', or 'none', got %q", ErrBadManifest, l.RestorePolicy)
	}
	program := l.Command.Program
	if strings.TrimSpace(program) == "" {
		program = "self"
	}
	if program != "self" && strings.ContainsRune(program, '\x00') {
		return fmt.Errorf("%w: invalid lifecycle.launch.command.program", ErrBadManifest)
	}
	for _, arg := range l.Command.Args {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("%w: lifecycle.launch.command.args must not contain NUL", ErrBadManifest)
		}
	}
	return nil
}

// validateCompletionFileName is the old host's `validate_file_name`: a plain
// file name of at most 128 bytes, made only of ASCII letters, digits, dots,
// underscores and hyphens.
func validateCompletionFileName(value string) error {
	if value == "" || len(value) > 128 {
		return fmt.Errorf("%w: invalid completion file name: %s", ErrBadManifest, value)
	}
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-'
		if !ok {
			return fmt.Errorf("%w: invalid completion file name: %s", ErrBadManifest, value)
		}
	}
	return nil
}

// ValidatePaths applies the relative-path rule to the paths a lifecycle
// declares: a completion's `source` (when it has one), a configuration
// template's `source` and `target`, and a launch command's `program` (unless
// it is the special `self`).
func (l Lifecycle) ValidatePaths() error {
	for _, completion := range l.Completions {
		if completion.Source == "" {
			continue
		}
		if err := ValidateRelativePath(completion.Source, "completion source"); err != nil {
			return err
		}
	}
	for _, template := range l.ConfigurationTemplates {
		if err := ValidateRelativePath(template.Source, "configuration template source"); err != nil {
			return err
		}
		if err := ValidateRelativePath(template.Target, "configuration template target"); err != nil {
			return err
		}
	}
	if program := l.Launch.Command.Program; program != "" && program != "self" {
		if err := ValidateRelativePath(program, "lifecycle.launch.command.program"); err != nil {
			return err
		}
	}
	return nil
}
