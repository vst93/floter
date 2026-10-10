package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// installSpec is what an install needs to know about the package: where it
// is, which manifest describes it, and what the repository should record.
type installSpec struct {
	// PackageDir is the package's directory, already extracted or local.
	PackageDir string
	// ManifestPath is the manifest inside it.
	ManifestPath string
	// Distribution is "local" or "npm".
	Distribution string
	// PackageName, Version and Integrity come from a package.json and its
	// registry entry; a plain local directory has none.
	PackageName string
	Version     string
	Integrity   string
	// Approved says the user approved the manifest's permissions for this
	// exact manifest.
	Approved bool
	// ExecutablePath is the program the caller resolved, when the manifest
	// names a system runtime by name and the program is not on the search
	// path (the connect flow's own file).
	ExecutablePath string
	// InPlace installs the package where it already is instead of copying it
	// into the extensions directory: the connect flow writes a generated
	// package into the *data* directory (the old build's
	// `<data>/<id>/integration`), and that directory is its home.
	InPlace bool
}

// Prepared is an install that has been staged but not committed: the
// package is ready, and the caller can show the user what it declares before
// anything is grafted into place.
type Prepared struct {
	paths   Paths
	spec    installSpec
	cleanup func()
	// Manifest is the package's manifest.
	Manifest Manifest
	// Approval is what the user must approve (its Added list is empty when
	// nothing is needed).
	Approval PermissionApproval
	// Digest is the manifest digest the approval would be bound to.
	Digest string
	// ExecutablePath is the program the caller already resolved, for a
	// generated package whose manifest names a system runtime by name: the
	// connect flow knows exactly which file the user pointed at, and the
	// search path may not carry it.
	ExecutablePath string
	// InPlace installs the package where it already is, for the connect flow's
	// generated packages (see installSpec.InPlace).
	InPlace bool
}

// PrepareLocal stages a package directory for install.
func PrepareLocal(paths Paths, packageDir string) (Prepared, error) {
	spec := installSpec{
		PackageDir:   packageDir,
		ManifestPath: filepath.Join(packageDir, manifestFileName),
		Distribution: "local",
	}
	if pkg, manifestPath, err := LoadPackageManifest(packageDir); err == nil {
		spec.ManifestPath = manifestPath
		spec.PackageName = pkg.Name
		spec.Version = pkg.Version
	}
	return prepare(paths, spec, nil)
}

// PrepareRegistry downloads a package and stages it for install.
func PrepareRegistry(ctx context.Context, paths Paths, registry *Registry, name, constraint string) (Prepared, error) {
	if registry == nil {
		registry = NewRegistry()
	}
	packument, err := registry.Packument(ctx, name)
	if err != nil {
		return Prepared{}, err
	}
	info, err := packument.Resolve(constraint)
	if err != nil {
		return Prepared{}, err
	}
	data, err := registry.Download(ctx, info)
	if err != nil {
		return Prepared{}, err
	}
	if err := paths.Ensure(); err != nil {
		return Prepared{}, err
	}
	download, err := os.MkdirTemp(paths.Root, ".download-*")
	if err != nil {
		return Prepared{}, err
	}
	cleanup := func() { os.RemoveAll(download) }
	if err := ExtractTarball(data, download); err != nil {
		cleanup()
		return Prepared{}, err
	}
	pkg, manifestPath, err := LoadPackageManifest(download)
	if err != nil {
		cleanup()
		return Prepared{}, err
	}
	spec := installSpec{
		PackageDir:   download,
		ManifestPath: manifestPath,
		Distribution: "npm",
		PackageName:  name,
		Version:      pkg.Version,
		Integrity:    info.Dist.Integrity,
	}
	return prepare(paths, spec, cleanup)
}

// prepare loads and validates a staged package and works out what the user
// must approve.
func prepare(paths Paths, spec installSpec, cleanup func()) (Prepared, error) {
	manifest, err := LoadManifest(spec.ManifestPath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return Prepared{}, err
	}
	if err := validID(manifest.ID); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return Prepared{}, err
	}
	digest, err := ManifestDigest(spec.ManifestPath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return Prepared{}, err
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return Prepared{}, err
	}
	var previous *Entry
	if entry, ok := repo.Extensions[manifest.ID]; ok {
		previous = &entry
	}
	return Prepared{
		paths:    paths,
		spec:     spec,
		cleanup:  cleanup,
		Manifest: manifest,
		Approval: RequiresApproval(previous, manifest, digest),
		Digest:   digest,
	}, nil
}

// Name and Version describe the prepared package.
func (p Prepared) Name() string    { return p.Manifest.Name }
func (p Prepared) Version() string { return p.spec.Version }

// Commit installs the prepared package. approved says whether the user
// approved the permissions RequiresApproval asked about; without it, a
// package that declares permissions is refused.
func (p Prepared) Commit(approved bool) (Entry, error) {
	if p.cleanup != nil {
		defer p.cleanup()
	}
	spec := p.spec
	spec.Approved = approved
	if p.ExecutablePath != "" {
		spec.ExecutablePath = p.ExecutablePath
	}
	if p.InPlace {
		spec.InPlace = true
	}
	return install(p.paths, spec)
}

// InstallLocal installs (or updates) an integration from a package
// directory: the distribution type every "connect a tool" flow uses, where
// the package is already on the machine. A package.json that names a floter
// manifest is respected, and its version is the integration's.
//
// It is PrepareLocal and Commit in one call, for a caller that has already
// approved what the package declares: a package that needs approval is
// refused with ErrPermissionApprovalRequired, and the caller shows the user
// what RequiresApproval listed.
func InstallLocal(paths Paths, packageDir string) (Entry, error) {
	prepared, err := PrepareLocal(paths, packageDir)
	if err != nil {
		return Entry{}, err
	}
	return prepared.Commit(!prepared.Approval.NeedsApproval())
}

// install grafts a package into the extension directory: it stages the
// package beside the installed ones and renames it into place, writes the
// repository atomically, and applies the manifest's configuration templates.
// An existing entry keeps its audit trail (approvals, error state) while its
// version and paths move forward.
func install(paths Paths, spec installSpec) (Entry, error) {
	packageDir := spec.PackageDir
	info, err := os.Stat(packageDir)
	if err != nil {
		return Entry{}, err
	}
	if !info.IsDir() {
		return Entry{}, fmt.Errorf("extensions: %s is not a directory", packageDir)
	}
	manifest, err := LoadManifest(spec.ManifestPath)
	if err != nil {
		return Entry{}, err
	}
	if err := validID(manifest.ID); err != nil {
		return Entry{}, err
	}
	// A declared input is checked wherever a manifest is installed, so a
	// hand-written one and one the connect form produced meet the same rule.
	if err := ValidateParams(manifest.Params); err != nil {
		return Entry{}, err
	}
	if err := paths.Ensure(); err != nil {
		return Entry{}, err
	}
	digest, err := ManifestDigest(spec.ManifestPath)
	if err != nil {
		return Entry{}, err
	}

	// Resolve the runtime before touching the disk: a package whose program
	// cannot be found must not replace a working install. A caller that
	// already resolved the program (the connect flow) hands it in.
	probe := Integration{
		Entry:    Entry{ID: manifest.ID, ExecutablePath: spec.ExecutablePath},
		Paths:    paths,
		Manifest: manifest,
	}
	binding, err := ResolveRuntime(probe)
	if err != nil {
		return Entry{}, err
	}

	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return Entry{}, err
	}
	previous, existed := repo.Extensions[manifest.ID]

	// A package that declares permissions is installed only with an
	// approval bound to this manifest's bytes.
	var existing *Entry
	if existed {
		existing = &previous
	}
	approval := RequiresApproval(existing, manifest, digest)
	if approval.NeedsApproval() && !spec.Approved {
		return Entry{}, fmt.Errorf("%w: %s declares %s", ErrPermissionApprovalRequired, manifest.Name, strings.Join(approval.Added, ", "))
	}

	target := filepath.Join(paths.Extensions, manifest.ID)
	backup := ""
	if spec.InPlace {
		// The package is already where it belongs (the connect flow writes it
		// into the data directory): there is nothing to copy and nothing to
		// move, and the entry points at it as it stands.
		target = packageDir
	} else {
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
	}

	entry := buildEntry(manifest, target, binding, spec, previous, existed)
	if len(manifest.Permissions) > 0 {
		entry.ApprovedPermissions = filterKnownPermissions(manifest.Permissions)
		entry.ApprovedAt = uint64(time.Now().Unix())
		entry.ApprovedManifestDigest = optionalString(digest)
	}
	repo.Extensions[manifest.ID] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		// Put the previous install back: the repository is the source of
		// truth and it did not change.
		if !spec.InPlace {
			os.RemoveAll(target)
		}
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
func buildEntry(manifest Manifest, packageDir string, binding RuntimeBinding, spec installSpec, previous Entry, existed bool) Entry {
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
	version := spec.Version
	if version == "" {
		version = manifestPackageVersion(manifest)
	}
	distribution := spec.Distribution
	if distribution == "" {
		distribution = "local"
	}

	entry := Entry{
		ID:                 manifest.ID,
		Name:               manifest.Name,
		PublisherID:        manifest.Publisher.ID,
		PublisherName:      manifest.Publisher.Name,
		DistributionSource: distribution,
		RuntimeOwnership:   runtimeOwnership,
		ProviderKind:       providerKind,
		State:              "enabled",
		Enabled:            true,
		PackageVersion:     version,
		PackageName:        optionalString(spec.PackageName),
		Integrity:          optionalString(spec.Integrity),
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

// optionalString is a pointer to a non-empty string, nil otherwise: the
// repository's optional fields.
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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
