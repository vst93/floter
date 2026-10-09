package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// RepositorySchemaVersion is the extension-repository.json schema this build
// reads and writes (repository.rs: REPOSITORY_SCHEMA_VERSION).
const RepositorySchemaVersion = 1

// Entry is one extension's state in the repository: the fields this build
// uses, plus every key it does not, kept so a write cannot drop the audit
// trail an older build recorded (integrity, approvals, error codes, ...).
type Entry struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	PublisherID   string `json:"publisherId"`
	PublisherName string `json:"publisherName"`

	// DistributionSource is "npm", "local" or "built-in".
	DistributionSource string `json:"distributionSource"`
	// RuntimeOwnership is "bundled" or "system".
	RuntimeOwnership string `json:"runtimeOwnership"`
	// ProviderKind is "executable", "static-descriptor" or "bundled-static".
	ProviderKind string `json:"providerKind"`
	// State is "enabled", "disabled" or "broken".
	State   string `json:"state"`
	Enabled bool   `json:"enabled"`

	PackageName    *string `json:"packageName,omitempty"`
	PackageVersion string  `json:"packageVersion"`
	ToolVersion    *string `json:"toolVersion,omitempty"`
	Integrity      *string `json:"integrity,omitempty"`

	CurrentVersion  string  `json:"currentVersion"`
	PreviousVersion *string `json:"previousVersion,omitempty"`

	ManifestPath   string  `json:"manifestPath"`
	ExecutablePath string  `json:"executablePath"`
	RuntimeRoot    *string `json:"runtimeRoot,omitempty"`

	InstalledAt uint64 `json:"installedAt"`
	UpdatedAt   uint64 `json:"updatedAt"`
	Pinned      bool   `json:"pinned"`
	Channel     string `json:"channel"`

	ApprovedPermissions    []string `json:"approvedPermissions,omitempty"`
	ApprovedAt             uint64   `json:"approvedAt,omitempty"`
	ApprovedManifestDigest *string  `json:"approvedManifestDigest,omitempty"`

	LastErrorCode   *string `json:"lastErrorCode,omitempty"`
	LastErrorDetail *string `json:"lastErrorDetail,omitempty"`
	LastErrorAt     *uint64 `json:"lastErrorAt,omitempty"`
	BrokenReason    *string `json:"brokenReason,omitempty"`

	ConfigGeneration uint64 `json:"configGeneration,omitempty"`

	// extra holds the keys this struct does not model.
	extra map[string]any
}

// knownEntryKeys are the JSON names Entry owns. Everything else in an entry
// is carried through verbatim.
var knownEntryKeys = []string{
	"id", "name", "publisherId", "publisherName",
	"distributionSource", "runtimeOwnership", "providerKind", "state", "enabled",
	"packageName", "packageVersion", "toolVersion", "integrity",
	"currentVersion", "previousVersion",
	"manifestPath", "executablePath", "runtimeRoot",
	"installedAt", "updatedAt", "pinned", "channel",
	"approvedPermissions", "approvedAt", "approvedManifestDigest",
	"lastErrorCode", "lastErrorDetail", "lastErrorAt", "brokenReason",
	"configGeneration",
}

// UnmarshalJSON decodes an entry, keeping the unknown keys.
func (e *Entry) UnmarshalJSON(data []byte) error {
	type plain Entry
	var typed plain
	if err := json.Unmarshal(data, &typed); err != nil {
		return err
	}
	var all map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	*e = Entry(typed)
	e.extra = map[string]any{}
	for _, key := range knownEntryKeys {
		delete(all, key)
	}
	for key, value := range all {
		e.extra[key] = value
	}
	return nil
}

// MarshalJSON encodes an entry with every key it carries.
func (e Entry) MarshalJSON() ([]byte, error) {
	type plain Entry
	data, err := json.Marshal(plain(e))
	if err != nil {
		return nil, err
	}
	if len(e.extra) == 0 {
		return data, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	for key, value := range e.extra {
		if _, owned := out[key]; !owned {
			out[key] = value
		}
	}
	return json.Marshal(out)
}

// Extra returns a copy of the keys this build does not model.
func (e Entry) Extra() map[string]any {
	out := make(map[string]any, len(e.extra))
	for key, value := range e.extra {
		out[key] = value
	}
	return out
}

// IsRunning reports whether the entry should contribute commands: enabled
// and not broken.
func (e Entry) IsRunning() bool {
	return e.Enabled && e.State == "enabled"
}

// Repository is the authoritative extension state file.
type Repository struct {
	// SchemaVersion is the file's schema; only RepositorySchemaVersion is
	// accepted.
	SchemaVersion int
	// Extensions is keyed by the extension id.
	Extensions map[string]Entry
}

// NewRepository is an empty repository at the current schema.
func NewRepository() Repository {
	return Repository{SchemaVersion: RepositorySchemaVersion, Extensions: map[string]Entry{}}
}

// LoadRepository reads the repository. A missing file is an empty repository
// and no error, exactly as a fresh install has no integrations; any other
// error (unreadable, malformed, an unknown schema) is reported with the
// empty repository so the caller keeps working.
func LoadRepository(path string) (Repository, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewRepository(), nil
		}
		return NewRepository(), err
	}
	var raw struct {
		SchemaVersion int              `json:"schemaVersion"`
		Extensions    map[string]Entry `json:"extensions"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return NewRepository(), fmt.Errorf("extensions: invalid repository %s: %w", path, err)
	}
	if raw.SchemaVersion != RepositorySchemaVersion {
		return NewRepository(), fmt.Errorf("extensions: unsupported repository schema %d in %s", raw.SchemaVersion, path)
	}
	if raw.Extensions == nil {
		raw.Extensions = map[string]Entry{}
	}
	return Repository{SchemaVersion: raw.SchemaVersion, Extensions: raw.Extensions}, nil
}

// SaveRepository writes the repository atomically: a temporary file in the
// same directory, flushed, then renamed over the target, as repository.rs
// does. A reader therefore sees the old file or the new one, never a partial
// write.
func SaveRepository(path string, repo Repository) error {
	if repo.SchemaVersion != RepositorySchemaVersion {
		return fmt.Errorf("extensions: refusing to write schema %d", repo.SchemaVersion)
	}
	if repo.Extensions == nil {
		repo.Extensions = map[string]Entry{}
	}
	payload := struct {
		SchemaVersion int              `json:"schemaVersion"`
		Extensions    map[string]Entry `json:"extensions"`
	}{RepositorySchemaVersion, repo.Extensions}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".extension-repository-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		dirFile.Close()
	}
	return nil
}

// Sorted returns the entries ordered by name, then id: the order a list
// shows.
func (r Repository) Sorted() []Entry {
	out := make([]Entry, 0, len(r.Extensions))
	for _, entry := range r.Extensions {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i].Name, out[j].Name
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})
	return out
}

// SetEnabled turns an extension on or off in the repository, keeping the
// state consistent with the flag: disabling moves an enabled entry to
// "disabled", enabling moves a disabled one to "enabled". A broken entry
// keeps its state (its runtime is not usable) and is only remembered.
func (r *Repository) SetEnabled(id string, enabled bool) (Entry, bool) {
	entry, ok := r.Extensions[id]
	if !ok {
		return Entry{}, false
	}
	entry.Enabled = enabled
	if entry.State != "broken" {
		if enabled {
			entry.State = "enabled"
		} else {
			entry.State = "disabled"
		}
	}
	r.Extensions[id] = entry
	return entry, true
}
