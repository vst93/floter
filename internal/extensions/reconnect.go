package extensions

import (
	"errors"
	"fmt"
	"time"
)

// ErrNotSystemOwned is what a reconnect reports for an integration whose
// runtime floter does not own. Only a system row's binding is floter's to
// re-derive: a bundled package's runtime ships with the package, so there is
// nothing on the search path to look for (the old host's `reconnect_system`
// rejected every other ownership the same way).
var ErrNotSystemOwned = errors.New("extensions: only a system-owned integration can be reconnected")

// ReconnectReport is what a reconnect did: the program the search path
// resolved to, and whether the record had to be rewritten. It is the old
// host's `extensions_reconnect_system` answer, which the row's Reconnect
// button turns into a notice.
type ReconnectReport struct {
	// Reconnected is true when the record changed: the program moved, or a
	// recorded binding failure is no longer true.
	Reconnected bool
	// Path is the program the reconnect resolved, whether or not it moved.
	Path string
}

// Reconnect re-derives a system integration's runtime binding from the search
// path — the old host's `extensions_reconnect_system`, the one repair a row
// whose program moved can offer.
//
// The recorded executable is forgotten first, deliberately: a reconnect is
// exactly the case where the program was reinstalled elsewhere or vanished
// (the row's own "Tool unavailable" state), so the search path is asked again
// rather than a stale absolute path confirmed. That is also what makes an
// *unbound* system row recoverable: with no recorded path the resolver looks
// the manifest's executable names up from PATH, which is precisely what the
// reference's comment says the old row could not do
// (`src/ExtensionsPanel.tsx`, the `runtimeSource === "system"` arm of
// `src/extensions/ExtensionRow.tsx:345-370`).
//
// A success rewrites `executablePath` and clears a *binding* failure; a
// failure records the reason on the entry the way a check does, so the row
// can word it in the user's language.
func Reconnect(paths Paths, id string) (ReconnectReport, error) {
	return ReconnectAt(paths, id, "")
}

// ReconnectAt is Reconnect with an explicit program, for the flow that asked
// the user which of several matching executables to bind: the reference's
// `extensions_reconnect_system` took the chosen `executablePath` the same way
// (`src-tauri/src/commands/extensions.rs:1754-1759`), and an empty path keeps
// the discover-from-the-search-path behavior.
func ReconnectAt(paths Paths, id, executablePath string) (ReconnectReport, error) {
	inventory := LoadInventory(paths)
	integration, ok := inventory.WithID(id)
	if !ok {
		return ReconnectReport{}, ErrNoIntegration
	}
	if integration.Entry.RuntimeOwnership != RuntimeOwnershipSystem {
		return ReconnectReport{}, fmt.Errorf("%w: %s", ErrNotSystemOwned, id)
	}

	var binding RuntimeBinding
	if executablePath != "" {
		if !isExecutableFile(executablePath) {
			return ReconnectReport{}, NewBindingFailure(BindingCodeMissing, BindingMissingDetail(executablePath))
		}
		binding = RuntimeBinding{Program: executablePath}
	} else {
		unbound := integration
		unbound.Entry.ExecutablePath = ""
		resolved, err := ResolveRuntime(unbound)
		if err != nil {
			var failure *BindingFailure
			if errors.As(err, &failure) {
				if recordErr := recordBindingFailure(paths, id, failure); recordErr != nil {
					return ReconnectReport{}, recordErr
				}
			}
			return ReconnectReport{}, err
		}
		binding = resolved
	}

	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return ReconnectReport{}, err
	}
	entry, ok := repo.Extensions[id]
	if !ok {
		return ReconnectReport{}, ErrNoIntegration
	}
	moved := entry.ExecutablePath != binding.Program
	cleared := clearBindingFailure(&entry)
	if !moved && !cleared {
		return ReconnectReport{Path: binding.Program}, nil
	}
	entry.ExecutablePath = binding.Program
	entry.UpdatedAt = uint64(time.Now().Unix())
	repo.Extensions[id] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		return ReconnectReport{}, err
	}
	return ReconnectReport{Reconnected: true, Path: binding.Program}, nil
}

// clearBindingFailure drops a recorded binding failure from the entry and
// hands the row back the state its own switch says, reporting whether there
// was one to drop. Only a *binding* code is cleared: a failed lifecycle
// probe's reason is a different fact about the same entry, and overwriting it
// here would erase a real diagnosis the row is drawing.
func clearBindingFailure(entry *Entry) bool {
	if entry.LastErrorCode == nil || !isBindingCode(*entry.LastErrorCode) {
		return false
	}
	entry.LastErrorCode = nil
	entry.LastErrorDetail = nil
	entry.LastErrorAt = nil
	entry.BrokenReason = nil
	if entry.State == "broken" {
		if entry.Enabled {
			entry.State = "enabled"
		} else {
			entry.State = "disabled"
		}
	}
	return true
}

// isBindingCode reports whether a recorded code is one of the runtime-binding
// failures rather than a probe's.
func isBindingCode(code string) bool {
	switch code {
	case BindingCodeChanged, BindingCodeMissing, BindingCodeRuntimeUnavailable, BindingCodeCheckFailed:
		return true
	}
	return false
}
