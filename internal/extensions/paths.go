// Package extensions reads floter's extension state: the installed
// integrations, their manifests, and the repository file that records them.
//
// The disk format is the one every earlier build wrote, so an existing
// user's integrations are found without any migration:
//
//	<config dir>/floter/extensions/               installed packages
//	<config dir>/floter/extension-data/           per-integration data
//	<config dir>/floter/extension-cache/          per-integration cache
//	<config dir>/floter/extension-repository.json the authoritative state
//	<config dir>/floter/tool-lock.json            a projection (read-only)
//	<config dir>/floter/extensions.lock.json      legacy (read-only)
//
// The repository is authoritative: the filesystem supplies manifests, the
// repository decides what is installed, enabled and broken. Every entry keeps
// the keys this package does not model, so a write (enable, disable) cannot
// drop the audit trail an older build recorded.
package extensions

import (
	"os"
	"path/filepath"
)

// The directory and file names, mirroring ExtensionPaths::from_root.
const (
	extensionsDir = "extensions"
	dataDir       = "extension-data"
	cacheDir      = "extension-cache"

	repositoryFile = "extension-repository.json"
	toolLockFile   = "tool-lock.json"
	legacyLockFile = "extensions.lock.json"
)

// Paths are the extension directories and state files under the app's config
// directory.
type Paths struct {
	// Root is the app's config directory (<config dir>/floter).
	Root string
	// Extensions holds the installed packages, one directory each.
	Extensions string
	// Data is per-integration writable state.
	Data string
	// Cache is per-integration cache.
	Cache string
	// RepositoryFile is the authoritative state file.
	RepositoryFile string
	// ToolLockFile is the older projection; read-only here.
	ToolLockFile string
	// LegacyLockFile is the first-generation state file; read-only here.
	LegacyLockFile string
}

// DiscoverPaths returns the paths under the platform's config directory,
// exactly as ExtensionPaths::discover did. It does not create anything.
func DiscoverPaths() (Paths, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	return FromRoot(filepath.Join(config, "floter")), nil
}

// FromRoot builds the paths under a config root.
func FromRoot(root string) Paths {
	return Paths{
		Root:           root,
		Extensions:     filepath.Join(root, extensionsDir),
		Data:           filepath.Join(root, dataDir),
		Cache:          filepath.Join(root, cacheDir),
		RepositoryFile: filepath.Join(root, repositoryFile),
		ToolLockFile:   filepath.Join(root, toolLockFile),
		LegacyLockFile: filepath.Join(root, legacyLockFile),
	}
}

// Ensure creates the directories the extension system owns, as
// ExtensionPaths::ensure did. It never creates the state files.
func (p Paths) Ensure() error {
	for _, dir := range []string{p.Root, p.Extensions, p.Data, p.Cache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
