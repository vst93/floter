package extensions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// InstallLocal installs (or updates) an integration from a package
// directory: the distribution type every "connect a tool" flow uses, where
// the package is already on the machine.
//
// It stages the package beside the installed ones and renames it into place,
// writes the repository atomically, and applies the manifest's configuration
// templates. An existing entry keeps its audit trail (approvals, error
// state) while its version and paths move forward.
func InstallLocal(paths Paths, packageDir string) (Entry, error) {
	info, err := os.Stat(packageDir)
	if err != nil {
		return Entry{}, err
	}
	if !info.IsDir() {
		return Entry{}, fmt.Errorf("extensions: %s is not a directory", packageDir)
	}
	manifestPath := filepath.Join(packageDir, manifestFileName)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return Entry{}, err
	}
	if err := validID(manifest.ID); err != nil {
		return Entry{}, err
	}
	if err := paths.Ensure(); err != nil {
		return Entry{}, err
	}

	// Resolve the runtime before touching the disk: a package whose program
	// cannot be found must not replace a working install.
	probe := Integration{Entry: Entry{ID: manifest.ID}, Paths: paths, Manifest: manifest}
	binding, err := ResolveRuntime(probe)
	if err != nil {
		return Entry{}, err
	}

	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return Entry{}, err
	}
	previous, existed := repo.Extensions[manifest.ID]

	target := filepath.Join(paths.Extensions, manifest.ID)
	staging, err := os.MkdirTemp(paths.Extensions, ".staging-"+manifest.ID+"-")
	if err != nil {
		return Entry{}, err
	}
	defer os.RemoveAll(staging)

	if err := copyTree(packageDir, staging); err != nil {
		return Entry{}, fmt.Errorf("extensions: staging %s: %w", packageDir, err)
	}

	// Move the old package aside, put the new one in place, and only then
	// drop the backup: a failure leaves the old install usable.
	backup := ""
	if _, err := os.Stat(target); err == nil {
		backup = target + ".backup"
		os.RemoveAll(backup)
		if err := os.Rename(target, backup); err != nil {
			return Entry{}, err
		}
	}
	if err := os.Rename(staging, target); err != nil {
		if backup != "" {
			os.Rename(backup, target)
		}
		return Entry{}, err
	}

	entry := buildEntry(manifest, target, binding, previous, existed)
	repo.Extensions[manifest.ID] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		// Put the previous install back: the repository is the source of
		// truth and it did not change.
		os.RemoveAll(target)
		if backup != "" {
			os.Rename(backup, target)
		}
		return Entry{}, err
	}
	if backup != "" {
		os.RemoveAll(backup)
	}

	// Configuration templates: copy each source into the integration's data
	// directory, without overwriting what a previous install left.
	if err := applyConfigTemplates(paths, manifest, target); err != nil {
		return entry, err
	}
	return entry, nil
}

// buildEntry is the repository record for a freshly installed package,
// carrying over what an existing entry must keep.
func buildEntry(manifest Manifest, packageDir string, binding RuntimeBinding, previous Entry, existed bool) Entry {
	now := uint64(time.Now().Unix())
	runtimeOwnership := "system"
	var runtimeRoot *string
	if manifest.Runtime.Type == "script" {
		runtimeOwnership = "bundled"
		root := packageDir
		runtimeRoot = &root
	}
	providerKind := "executable"
	switch manifest.Provider.Type {
	case "static-descriptor":
		providerKind = "static-descriptor"
	case "bundled-static":
		providerKind = "bundled-static"
	}
	version := manifestPackageVersion(manifest)

	entry := Entry{
		ID:                 manifest.ID,
		Name:               manifest.Name,
		PublisherID:        manifest.Publisher.ID,
		PublisherName:      manifest.Publisher.Name,
		DistributionSource: "local",
		RuntimeOwnership:   runtimeOwnership,
		ProviderKind:       providerKind,
		State:              "enabled",
		Enabled:            true,
		PackageVersion:     version,
		CurrentVersion:     version,
		ManifestPath:       filepath.Join(packageDir, manifestFileName),
		ExecutablePath:     binding.Program,
		RuntimeRoot:        runtimeRoot,
		InstalledAt:        now,
		UpdatedAt:          now,
		Channel:            "stable",
		extra:              map[string]any{},
	}
	if existed {
		entry.InstalledAt = previous.InstalledAt
		entry.Channel = previous.Channel
		entry.Pinned = previous.Pinned
		// The audit trail survives; a changed manifest is what would
		// require a new approval, and that is the caller's decision.
		entry.ApprovedPermissions = previous.ApprovedPermissions
		entry.ApprovedAt = previous.ApprovedAt
		entry.ApprovedManifestDigest = previous.ApprovedManifestDigest
		entry.ConfigGeneration = previous.ConfigGeneration
		if previous.State == "broken" {
			entry.State = "broken"
			entry.Enabled = previous.Enabled
			entry.LastErrorCode = previous.LastErrorCode
			entry.LastErrorDetail = previous.LastErrorDetail
			entry.LastErrorAt = previous.LastErrorAt
			entry.BrokenReason = previous.BrokenReason
		}
		if previous.CurrentVersion != "" && previous.CurrentVersion != version {
			entry.PreviousVersion = &previous.CurrentVersion
		}
	}
	return entry
}

// manifestPackageVersion is a package's version: a manifest may carry one
// outside the schema's required set, and "local" stands in for a package
// that does not.
func manifestPackageVersion(manifest Manifest) string {
	if manifest.Version != "" {
		return manifest.Version
	}
	return "local"
}

// Uninstall removes an integration: its package, its record, and its data
// when asked. The data goes last, so a failure never leaves a record
// pointing at data that is gone.
func Uninstall(paths Paths, id string, removeData bool) error {
	if err := validID(id); err != nil {
		return err
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return err
	}
	if _, ok := repo.Extensions[id]; !ok {
		return ErrNoIntegration
	}

	packageDir := filepath.Join(paths.Extensions, id)
	trash := ""
	if _, err := os.Stat(packageDir); err == nil {
		trash = packageDir + ".removing"
		os.RemoveAll(trash)
		if err := os.Rename(packageDir, trash); err != nil {
			return err
		}
	}

	delete(repo.Extensions, id)
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		if trash != "" {
			os.Rename(trash, packageDir)
		}
		return err
	}
	if trash != "" {
		if err := os.RemoveAll(trash); err != nil {
			return err
		}
	}
	if removeData {
		if err := os.RemoveAll(filepath.Join(paths.Data, id)); err != nil {
			return err
		}
	}
	return nil
}

// SetToolVersion records the version a provider reported, which is what the
// details list shows beside the package version.
func SetToolVersion(paths Paths, id, version string) error {
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return err
	}
	entry, ok := repo.Extensions[id]
	if !ok {
		return ErrNoIntegration
	}
	if version == "" {
		entry.ToolVersion = nil
	} else {
		entry.ToolVersion = &version
	}
	repo.Extensions[id] = entry
	return SaveRepository(paths.RepositoryFile, repo)
}

// ManifestDigest is the SHA-256 of a manifest's bytes, the form the
// approval record stores.
func ManifestDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256-" + hex.EncodeToString(sum[:]), nil
}

// applyConfigTemplates copies the manifest's configuration templates into the
// integration's data directory, skipping targets that already exist.
func applyConfigTemplates(paths Paths, manifest Manifest, packageDir string) error {
	if len(manifest.Lifecycle.ConfigurationTemplates) == 0 {
		return nil
	}
	dataDir, err := safeJoin(paths.Data, manifest.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	for _, template := range manifest.Lifecycle.ConfigurationTemplates {
		source, ok := joinWithin(packageDir, template.Source)
		if !ok {
			return fmt.Errorf("extensions: template source %q leaves the package", template.Source)
		}
		target, ok := joinWithin(dataDir, template.Target)
		if !ok {
			return fmt.Errorf("extensions: template target %q leaves the data directory", template.Target)
		}
		if _, err := os.Stat(target); err == nil {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies a directory tree, following no links out of it.
func copyTree(source, destination string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(source, entry.Name())
		to := filepath.Join(destination, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// A package that ships links could point anywhere: skip them.
			continue
		}
		if entry.IsDir() {
			if err := os.MkdirAll(to, info.Mode().Perm()); err != nil {
				return err
			}
			if err := copyTree(from, to); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// validID checks an extension id the way the old lock did: dot-separated
// lowercase segments, which also keeps it a safe directory name.
func validID(id string) error {
	if id == "" {
		return errors.New("extensions: an empty id")
	}
	if strings.ContainsAny(id, `/\`) || id == "." || id == ".." || strings.HasPrefix(id, ".") {
		return fmt.Errorf("extensions: unsafe id %q", id)
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return fmt.Errorf("extensions: invalid id %q", id)
		}
	}
	return nil
}
