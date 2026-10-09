package extensions

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifestFileName is the package manifest inside an installed extension
// directory.
const manifestFileName = "floter.extension.json"

// Integration is one installed extension, as the app lists it: the
// repository's record joined with the package's manifest.
type Integration struct {
	Entry Entry
	// Paths is where the integration's package and data live.
	Paths Paths

	// Manifest is the package's manifest, zero when it could not be read.
	Manifest Manifest
	// ManifestErr is why the manifest is missing or malformed. An
	// integration whose package is gone still appears: the repository says
	// it is installed, and hiding it would hide the problem.
	ManifestErr error

	// Name and Description come from the manifest when there is one, and
	// from the repository otherwise.
	Name        string
	Description string
	Publisher   string
	// Version is the package version the repository recorded, with the
	// tool version beside it when there is one.
	Version     string
	ToolVersion string
}

// PackageDir is the installed package's directory: the manifest's own
// directory when the manifest was read, else the directory the entry's id
// names under extensions/.
func (i Integration) PackageDir() string {
	if i.ManifestErr == nil && i.Entry.ManifestPath != "" {
		if info, err := os.Stat(i.Entry.ManifestPath); err == nil {
			if info.IsDir() {
				return i.Entry.ManifestPath
			}
			return filepath.Dir(i.Entry.ManifestPath)
		}
	}
	if i.Paths.Extensions == "" || i.Entry.ID == "" {
		return ""
	}
	return filepath.Join(i.Paths.Extensions, i.Entry.ID)
}

// Broken reports whether the entry is in the broken state.
func (i Integration) Broken() bool { return i.Entry.State == "broken" }

// Running reports whether the integration contributes commands: enabled,
// not broken, and with a readable manifest.
func (i Integration) Running() bool {
	return i.Entry.IsRunning() && i.ManifestErr == nil
}

// Inventory is what the extension directories and the repository say.
type Inventory struct {
	// Integrations are the recorded extensions, ordered by name.
	Integrations []Integration
	// RepositoryErr is why the repository could not be read; the inventory
	// is then empty and the caller should report it.
	RepositoryErr error
	// Orphans are package directories under extensions/ that no repository
	// entry names: a package the old build did not finish installing, or one
	// dropped in by hand.
	Orphans []string
	// Paths is the root the inventory was read from.
	Paths Paths
}

// LoadInventory reads the repository and every entry's manifest. It never
// fails as a whole: a broken repository yields RepositoryErr, and a broken
// package yields a per-integration ManifestErr.
func LoadInventory(paths Paths) Inventory {
	inventory := Inventory{Paths: paths}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		inventory.RepositoryErr = err
		return inventory
	}

	for _, entry := range repo.Sorted() {
		integration := Integration{
			Entry:     entry,
			Paths:     paths,
			Name:      entry.Name,
			Publisher: entry.PublisherName,
			Version:   entry.PackageVersion,
		}
		if entry.ToolVersion != nil {
			integration.ToolVersion = *entry.ToolVersion
		}

		path := manifestPathFor(paths, entry)
		manifest, manifestErr := LoadManifest(path)
		integration.Manifest, integration.ManifestErr = manifest, manifestErr
		if manifestErr == nil {
			if manifest.Name != "" {
				integration.Name = manifest.Name
			}
			if manifest.Description != "" {
				integration.Description = manifest.Description
			}
			if manifest.Publisher.Name != "" {
				integration.Publisher = manifest.Publisher.Name
			}
		}
		inventory.Integrations = append(inventory.Integrations, integration)
	}

	inventory.Orphans = orphanPackages(paths, repo)
	return inventory
}

// manifestPathFor is where an entry's manifest is read from: the recorded
// path when it exists, else the package directory named by the id, which is
// where an install puts it.
func manifestPathFor(paths Paths, entry Entry) string {
	if entry.ManifestPath != "" {
		if info, err := os.Stat(entry.ManifestPath); err == nil {
			if info.IsDir() {
				return filepath.Join(entry.ManifestPath, manifestFileName)
			}
			return entry.ManifestPath
		}
	}
	return filepath.Join(paths.Extensions, entry.ID, manifestFileName)
}

// orphanPackages lists the package directories no repository entry names.
func orphanPackages(paths Paths, repo Repository) []string {
	entries, err := os.ReadDir(paths.Extensions)
	if err != nil {
		return nil
	}
	var orphans []string
	for _, dir := range entries {
		if !dir.IsDir() || strings.HasPrefix(dir.Name(), ".") {
			continue // .transactions and friends are the installer's own
		}
		if _, ok := repo.Extensions[dir.Name()]; ok {
			continue
		}
		orphans = append(orphans, dir.Name())
	}
	sort.Strings(orphans)
	return orphans
}

// WithID returns the integration with an id.
func (in Inventory) WithID(id string) (Integration, bool) {
	for _, integration := range in.Integrations {
		if integration.Entry.ID == id {
			return integration, true
		}
	}
	return Integration{}, false
}

// Running returns the integrations that contribute commands.
func (in Inventory) Running() []Integration {
	var out []Integration
	for _, integration := range in.Integrations {
		if integration.Running() {
			out = append(out, integration)
		}
	}
	return out
}
