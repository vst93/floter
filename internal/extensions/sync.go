package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Export and import of the installed integrations: which ones are enabled and
// how they are configured, so a second machine can be brought to the same
// state. The document is the one the old build wrote (`version: 2`), so its
// exports can be read here and this build's can be read there.
//
// What travels:
//
//   - the identity, version, enabled flag and configuration values;
//   - the package's *essentials* — the manifest, a script runtime's script,
//     a static provider's descriptor — so an integration can be installed on
//     a machine that does not have it;
//   - the classification of every configuration field, so an importer knows
//     what was left out and why.
//
// What never travels: secrets. A configuration field that looks like a
// credential (or a device-specific path) is excluded and marked, so a
// password cannot be smuggled into a file the user shares. The receiving side
// keeps its own secrets.

// SyncFormatVersion is the document version this build writes and reads.
const SyncFormatVersion = 2

// maxSyncBytes bounds an import: a document is a text file a user picked, not
// an unbounded stream.
const maxSyncBytes = 4 << 20

// The field categories the export classifies a configuration value into.
const (
	CategoryNormal            = "normal"
	CategorySecret            = "secret"
	CategoryDevicePath        = "device_path"
	CategoryVersionConstraint = "version_constraint"
)

// FieldMetadata says what an exporter decided about one configuration field.
type FieldMetadata struct {
	Key string `json:"key"`
	// Category is one of the categories above.
	Category string `json:"category"`
	// Excluded is true when the field was left out of the document.
	Excluded bool `json:"excluded"`
}

// SyncEntry is one integration in a document.
type SyncEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	// Config is the configuration that was exported (secrets and
	// device-specific paths left out).
	Config map[string]any `json:"config,omitempty"`
	// DistributionSource and RuntimeOwnership are the repository's record.
	DistributionSource string `json:"distributionSource"`
	RuntimeOwnership   string `json:"runtimeOwnership"`
	// FieldMetadata classifies every field the configuration held.
	FieldMetadata []FieldMetadata `json:"fieldMetadata,omitempty"`

	// The package's essentials, present when the exporter could read them.
	Manifest           json.RawMessage `json:"manifest,omitempty"`
	ScriptContent      string          `json:"scriptContent,omitempty"`
	ProviderDescriptor json.RawMessage `json:"providerDescriptor,omitempty"`
	// Package is a field the old build wrote (the package archive); this
	// build neither writes nor reads it, but keeps it so a document survives
	// a round trip through here.
	Package json.RawMessage `json:"package,omitempty"`
}

// SyncDocument is an export file.
type SyncDocument struct {
	Version    int         `json:"version"`
	ExportedAt string      `json:"exportedAt"`
	Extensions []SyncEntry `json:"extensions"`
}

// SyncReport is what an import did, entry by entry.
type SyncReport struct {
	Succeeded []string
	Failed    []SyncFailure
	Skipped   []SyncFailure
}

// SyncFailure is one entry an import could not apply.
type SyncFailure struct {
	ID     string
	Reason string
}

// ExportSync reads the installed integrations into a document.
func ExportSync(paths Paths) (SyncDocument, error) {
	inventory := LoadInventory(paths)
	if inventory.RepositoryErr != nil {
		return SyncDocument{}, inventory.RepositoryErr
	}
	document := SyncDocument{
		Version:    SyncFormatVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, integration := range inventory.Integrations {
		entry, err := exportIntegration(paths, integration)
		if err != nil {
			return SyncDocument{}, fmt.Errorf("extensions: exporting %s: %w", integration.Entry.ID, err)
		}
		document.Extensions = append(document.Extensions, entry)
	}
	return document, nil
}

// exportIntegration reads one integration into a document entry.
func exportIntegration(paths Paths, integration Integration) (SyncEntry, error) {
	entry := SyncEntry{
		ID:                 integration.Entry.ID,
		Version:            integration.Version,
		Enabled:            integration.Entry.Enabled,
		DistributionSource: integration.Entry.DistributionSource,
		RuntimeOwnership:   integration.Entry.RuntimeOwnership,
	}
	if integration.ManifestErr == nil && integration.Manifest.Version != "" {
		entry.Version = integration.Manifest.Version
	}

	// The configuration: the raw file, without the secrets merged in, so a
	// credential is never in the document.
	raw, err := loadRawConfiguration(paths.Data, integration.Entry.ID)
	if err != nil {
		return SyncEntry{}, err
	}
	config, metadata := classifyConfiguration(raw.Values)
	entry.Config, entry.FieldMetadata = config, metadata

	// The package's essentials, when they can be read.
	packageDir := integration.PackageDir()
	if packageDir != "" && integration.ManifestErr == nil {
		if data, err := os.ReadFile(filepath.Join(packageDir, manifestFileName)); err == nil {
			entry.Manifest = json.RawMessage(data)
		}
		// A script runtime carries its script, a static provider its
		// descriptor: those two are what a package cannot be rebuilt without.
		if integration.Manifest.Runtime.Type == "script" {
			if script := integration.Manifest.Runtime.Path; script != "" {
				if data, err := os.ReadFile(filepath.Join(packageDir, filepath.FromSlash(script))); err == nil {
					entry.ScriptContent = string(data)
				}
			}
		}
		if descriptor := integration.Manifest.Provider.Descriptor; descriptor != "" {
			if data, err := os.ReadFile(filepath.Join(packageDir, filepath.FromSlash(descriptor))); err == nil {
				entry.ProviderDescriptor = json.RawMessage(data)
			}
		}
	}
	return entry, nil
}

// classifyConfiguration splits a configuration into what may be exported and
// what may not, and says what it decided about every field.
func classifyConfiguration(values map[string]any) (map[string]any, []FieldMetadata) {
	if len(values) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	exported := map[string]any{}
	var metadata []FieldMetadata
	for _, key := range keys {
		value := values[key]
		category := ClassifyConfigField(key, value)
		excluded := category != CategoryNormal
		metadata = append(metadata, FieldMetadata{Key: key, Category: category, Excluded: excluded})
		if !excluded {
			exported[key] = value
		}
	}
	if len(exported) == 0 {
		exported = nil
	}
	return exported, metadata
}

// ClassifyConfigField decides whether a configuration value may be exported.
//
// The rules are the old build's: a key that names a credential is a secret, a
// value that is an absolute path is device-specific, and a version constraint
// may not apply on the target machine. Everything else travels.
func ClassifyConfigField(key string, value any) string {
	lower := strings.ToLower(key)
	for _, word := range []string{"secret", "password", "passwd", "token", "apikey", "api_key", "credential", "private_key", "privatekey"} {
		if strings.Contains(lower, word) {
			return CategorySecret
		}
	}
	if lower == "key" || strings.HasSuffix(lower, "_key") || strings.HasSuffix(lower, "-key") {
		// A bare "key" is as likely a credential as an identifier; the old
		// build treated it as one.
		return CategorySecret
	}
	if text, ok := value.(string); ok {
		// The placeholder stands for a secret this machine holds in its
		// secrets file: it is not a value to export either.
		if text == PasswordPlaceholder {
			return CategorySecret
		}
		if strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~") ||
			(len(text) > 2 && text[1] == ':' && (text[2] == '\\' || text[2] == '/')) {
			return CategoryDevicePath
		}
	}
	if strings.Contains(lower, "version") && strings.Contains(lower, "constraint") {
		return CategoryVersionConstraint
	}
	return CategoryNormal
}

// Marshal encodes a document the way the old build did: indented JSON.
func (d SyncDocument) Marshal() ([]byte, error) {
	if d.Version == 0 {
		d.Version = SyncFormatVersion
	}
	if d.ExportedAt == "" {
		d.ExportedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return json.MarshalIndent(d, "", "  ")
}

// ReadSync parses a document, refusing one that is not a document this build
// understands.
func ReadSync(data []byte) (SyncDocument, error) {
	if len(data) > maxSyncBytes {
		return SyncDocument{}, fmt.Errorf("extensions: the import is larger than %d bytes", maxSyncBytes)
	}
	var document SyncDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return SyncDocument{}, fmt.Errorf("extensions: invalid import document: %w", err)
	}
	if document.Version == 0 || document.Version > SyncFormatVersion {
		return SyncDocument{}, fmt.Errorf("extensions: import version %d is not supported", document.Version)
	}
	for _, entry := range document.Extensions {
		if err := validID(entry.ID); err != nil {
			return SyncDocument{}, fmt.Errorf("extensions: invalid id %q: %w", entry.ID, err)
		}
	}
	return document, nil
}

// ImportSync applies a document.
//
// An integration that is installed keeps its secrets and its package: only
// its enabled flag and its exported configuration values change. One that is
// not installed is installed from the manifest the document carries, when it
// carries one — which needs the user's approval for whatever permissions it
// declares, so the caller passes the answer in.
//
// Every entry is reported: what was applied, what was refused and why, and
// what was skipped because the machine has neither the integration nor a
// package for it.
func ImportSync(paths Paths, document SyncDocument, approve func(PermissionApproval) bool) (SyncReport, error) {
	report := SyncReport{}
	inventory := LoadInventory(paths)
	installed := map[string]Integration{}
	for _, integration := range inventory.Integrations {
		installed[integration.Entry.ID] = integration
	}

	for _, entry := range document.Extensions {
		if integration, ok := installed[entry.ID]; ok {
			if err := applySyncEntry(paths, integration, entry); err != nil {
				report.Failed = append(report.Failed, SyncFailure{ID: entry.ID, Reason: err.Error()})
				continue
			}
			report.Succeeded = append(report.Succeeded, entry.ID)
			continue
		}
		if len(entry.Manifest) == 0 {
			report.Skipped = append(report.Skipped, SyncFailure{
				ID:     entry.ID,
				Reason: "not installed here, and the export carries no package",
			})
			continue
		}
		if err := installFromSyncEntry(paths, entry, approve); err != nil {
			report.Failed = append(report.Failed, SyncFailure{ID: entry.ID, Reason: err.Error()})
			continue
		}
		report.Succeeded = append(report.Succeeded, entry.ID)
	}
	return report, nil
}

// applySyncEntry updates an installed integration: its enabled flag and the
// configuration values the document carries. A value the document excluded
// (a secret, a path) is left as this machine has it.
func applySyncEntry(paths Paths, integration Integration, entry SyncEntry) error {
	if integration.Entry.Enabled != entry.Enabled {
		repo, err := LoadRepository(paths.RepositoryFile)
		if err != nil {
			return err
		}
		if _, ok := repo.SetEnabled(entry.ID, entry.Enabled); !ok {
			return ErrNoIntegration
		}
		if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
			return err
		}
	}
	if len(entry.Config) == 0 {
		return nil
	}
	// The raw values, not the ones with the secrets merged in: writing those
	// back would copy a secret out of the secrets file into the values file.
	stored, err := loadRawConfiguration(paths.Data, entry.ID)
	if err != nil {
		return err
	}
	values := map[string]any{}
	for key, value := range stored.Values {
		values[key] = value
	}
	for key, value := range entry.Config {
		values[key] = value
	}
	return SaveStoredConfiguration(paths.Data, entry.ID, StoredConfiguration{
		ConfigVersion:    stored.ConfigVersion,
		SecretGeneration: stored.SecretGeneration,
		Values:           values,
		Schema:           stored.Schema,
	})
}

// installFromSyncEntry installs an integration the document carries.
func installFromSyncEntry(paths Paths, entry SyncEntry, approve func(PermissionApproval) bool) error {
	staging, err := os.MkdirTemp("", "floter-sync-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := os.WriteFile(filepath.Join(staging, manifestFileName), entry.Manifest, 0o644); err != nil {
		return err
	}
	manifest, err := LoadManifest(filepath.Join(staging, manifestFileName))
	if err != nil {
		return err
	}
	if manifest.ID != entry.ID {
		return fmt.Errorf("extensions: the package declares %s, not %s", manifest.ID, entry.ID)
	}
	if manifest.Runtime.Type == "script" {
		if entry.ScriptContent == "" {
			return errors.New("extensions: the export carries no script for this integration")
		}
		script := filepath.Join(staging, filepath.FromSlash(manifest.Runtime.Path))
		if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(script, []byte(entry.ScriptContent), 0o755); err != nil {
			return err
		}
	}
	if descriptor := manifest.Provider.Descriptor; descriptor != "" {
		if len(entry.ProviderDescriptor) == 0 {
			return errors.New("extensions: the export carries no descriptor for this integration")
		}
		path := filepath.Join(staging, filepath.FromSlash(descriptor))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, entry.ProviderDescriptor, 0o644); err != nil {
			return err
		}
	}

	prepared, err := PrepareLocal(paths, staging)
	if err != nil {
		return err
	}
	approved := true
	if prepared.Approval.NeedsApproval() && approve != nil {
		approved = approve(prepared.Approval)
	}
	installed, err := prepared.Commit(approved)
	if err != nil {
		return err
	}
	// The document's configuration goes in after the package, so the
	// integration arrives configured.
	return applySyncEntry(paths, Integration{Entry: installed, Paths: paths}, entry)
}

// loadRawConfiguration reads an integration's stored configuration *without*
// merging the secrets file: what the export may carry is what the file holds,
// and a secret lives in the secrets file.
func loadRawConfiguration(dataRoot, id string) (StoredConfiguration, error) {
	dir, err := safeJoin(dataRoot, id)
	if err != nil {
		return StoredConfiguration{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StoredConfiguration{Values: map[string]any{}}, nil
		}
		return StoredConfiguration{}, err
	}
	stored, err := parseStoredConfiguration(data)
	if err != nil {
		return StoredConfiguration{}, err
	}
	if stored.Values == nil {
		stored.Values = map[string]any{}
	}
	// The placeholders are left in place: the classification sees the field
	// (so it can report it as excluded) and an import writes the file back
	// with the placeholder intact, which is what keeps the secret in the
	// secrets file rather than in the values.
	return stored, nil
}
