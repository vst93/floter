package extensions

import (
	"fmt"
	"os"
	"path/filepath"
)

// The componentized uninstall: which parts of an installed integration to
// remove. The program is always required (an uninstall that keeps the program
// is nothing); the three data categories are the user's choice, and selecting
// all of them removes the data directory as a whole.

// UninstallComponents says which parts of an installed integration to remove.
type UninstallComponents struct {
	// RemoveHostConfig removes the host-owned configuration (config.json and
	// the secrets generations).
	RemoveHostConfig bool
	// RemoveToolData removes the tool's own data (sessions, completions,
	// health).
	RemoveToolData bool
	// RemoveArtifacts removes generated artifacts (logs, caches,
	// user-generated files).
	RemoveArtifacts bool
}

// UninstallResult is what was removed, by component.
type UninstallResult struct {
	RemovedProgram    bool
	RemovedHostConfig bool
	RemovedToolData   bool
	RemovedArtifacts  bool
}

// UninstallComponentized removes the requested parts of an installed
// integration. The program is removed through the same rename-aside-then-delete
// the plain uninstall uses; the data categories remove their own paths, and
// all three together remove the data root as a whole.
func UninstallComponentized(paths Paths, id string, components UninstallComponents) (UninstallResult, error) {
	if err := validID(id); err != nil {
		return UninstallResult{}, err
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return UninstallResult{}, err
	}
	entry, ok := repo.Extensions[id]
	if !ok {
		return UninstallResult{}, ErrNoIntegration
	}
	dataRoot := filepath.Join(paths.Data, id)

	var result UninstallResult

	// The program: the same transactional rename-aside the plain uninstall
	// uses, so a failure keeps the old install usable.
	if err := Uninstall(paths, id, false); err != nil {
		return UninstallResult{}, err
	}
	result.RemovedProgram = true

	// The data categories. All three together remove the data root, which is
	// the same outcome as removing each.
	all := components.RemoveHostConfig && components.RemoveToolData && components.RemoveArtifacts
	if all {
		if err := os.RemoveAll(dataRoot); err != nil {
			return result, fmt.Errorf("extensions: removing the data of %s: %w", id, err)
		}
		return UninstallResult{
			RemovedProgram:    true,
			RemovedHostConfig: true,
			RemovedToolData:   true,
			RemovedArtifacts:  true,
		}, nil
	}
	if components.RemoveHostConfig {
		for _, name := range []string{"config.json", "config-secrets"} {
			path := filepath.Join(dataRoot, name)
			if err := os.RemoveAll(path); err == nil {
				result.RemovedHostConfig = true
			}
		}
	}
	if components.RemoveToolData {
		for _, name := range []string{"sessions", "completions", "health.json", "user-data"} {
			path := filepath.Join(dataRoot, name)
			if err := os.RemoveAll(path); err == nil {
				result.RemovedToolData = true
			}
		}
	}
	if components.RemoveArtifacts {
		for _, name := range []string{"artifacts", "logs", "cache"} {
			path := filepath.Join(dataRoot, name)
			if err := os.RemoveAll(path); err == nil {
				result.RemovedArtifacts = true
			}
		}
	}
	_ = entry
	return result, nil
}
