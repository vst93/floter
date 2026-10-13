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

// ValidatePaths applies the relative-path rule to every path a manifest
// declares: the icon, a script runtime's source, a static provider's
// descriptor, a completion's source file, a configuration template's source
// and target, and a launch command's program. It is the old host's
// `validate_paths` (`manifest.rs:565`) minus the lifecycle count rules, which
// are validated separately.
func (m Manifest) ValidatePaths() error {
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
	return m.Lifecycle.ValidatePaths()
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
