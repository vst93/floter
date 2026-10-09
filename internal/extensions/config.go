package extensions

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The configuration filenames under extension-data/<id>/.
const (
	configFile           = "config.json"
	legacySecretsFile    = "config.secrets.json"
	secretsGenerationDir = "config-secrets"
)

// PasswordPlaceholder is what the stored values file holds in place of a
// password whose secret lives in the secrets file.
const PasswordPlaceholder = "[REDACTED]"

// StoredConfiguration is an integration's host-owned configuration: the
// values the old app recorded in extension-data/<id>/config.json, with its
// secrets resolved from the secrets file.
type StoredConfiguration struct {
	// ConfigVersion is the schema generation the values were written
	// against; 0 for a legacy values-only file.
	ConfigVersion int
	// SecretGeneration names the secrets file the values were written with,
	// empty when the legacy secrets file applies.
	SecretGeneration string
	// Values are the configuration values, keyed by the schema's key.
	Values map[string]any
	// Schema is the schema the values were written against, when the file
	// recorded one.
	Schema []ConfigField
}

// Value returns a configuration value.
func (c StoredConfiguration) Value(key string) (any, bool) {
	value, ok := c.Values[key]
	return value, ok
}

// LoadStoredConfiguration reads an integration's configuration from the
// extension-data root. A missing file is an empty configuration, not an
// error: an integration that was never configured is normal.
func LoadStoredConfiguration(dataRoot, id string) (StoredConfiguration, error) {
	dir, err := safeJoin(dataRoot, id)
	if err != nil {
		return StoredConfiguration{}, err
	}
	path := filepath.Join(dir, configFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StoredConfiguration{Values: map[string]any{}}, nil
		}
		return StoredConfiguration{}, err
	}

	stored, err := parseStoredConfiguration(data)
	if err != nil {
		return StoredConfiguration{}, fmt.Errorf("extensions: invalid configuration %s: %w", path, err)
	}
	if stored.Values == nil {
		stored.Values = map[string]any{}
	}

	secrets, err := loadSecrets(dir, stored.SecretGeneration)
	if err != nil {
		return StoredConfiguration{}, err
	}
	if secrets != nil {
		for key, value := range secrets {
			stored.Values[key] = value
		}
		return stored, nil
	}
	// No secrets: a password placeholder is not a value.
	for key, value := range stored.Values {
		if text, ok := value.(string); ok && text == PasswordPlaceholder {
			delete(stored.Values, key)
		}
	}
	return stored, nil
}

// SaveStoredConfiguration writes an integration's configuration back, in the
// shape the file already had: the current envelope (a config generation and
// its secrets generation beside the values) when the file carried one, the
// legacy bare values map otherwise. The write is atomic — a temporary file in
// the same directory, flushed, then renamed — so a crash leaves the previous
// configuration intact.
//
// The secrets file is never touched: a value whose secret lives there stays
// there, and the loader keeps merging it by generation.
func SaveStoredConfiguration(dataRoot, id string, stored StoredConfiguration) error {
	dir, err := safeJoin(dataRoot, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	values := stored.Values
	if values == nil {
		values = map[string]any{}
	}

	var data []byte
	if stored.ConfigVersion > 0 || stored.SecretGeneration != "" || len(stored.Schema) > 0 {
		envelope := map[string]any{
			"configVersion":    stored.ConfigVersion,
			"secretGeneration": stored.SecretGeneration,
			"values":           values,
		}
		if len(stored.Schema) > 0 {
			envelope["schema"] = stored.Schema
		}
		data, err = json.MarshalIndent(envelope, "", "  ")
	} else {
		data, err = json.MarshalIndent(values, "", "  ")
	}
	if err != nil {
		return err
	}

	temporary, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, configFile))
}

// parseStoredConfiguration accepts both shapes the old app wrote: the
// current envelope, and the legacy bare values map.
func parseStoredConfiguration(data []byte) (StoredConfiguration, error) {
	var envelope struct {
		ConfigVersion    int            `json:"configVersion"`
		SecretGeneration string         `json:"secretGeneration"`
		Values           map[string]any `json:"values"`
		Schema           []ConfigField  `json:"schema"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return StoredConfiguration{}, err
	}
	if envelope.Values != nil {
		return StoredConfiguration{
			ConfigVersion:    envelope.ConfigVersion,
			SecretGeneration: envelope.SecretGeneration,
			Values:           envelope.Values,
			Schema:           envelope.Schema,
		}, nil
	}
	// Legacy: the file is the values map itself.
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return StoredConfiguration{}, err
	}
	return StoredConfiguration{Values: values}, nil
}

// loadSecrets reads the secrets for a configuration: the named generation
// file when the values recorded one, else the legacy secrets file. A missing
// file yields nil, which means "no secrets", not "empty secrets".
func loadSecrets(dir, generation string) (map[string]any, error) {
	if generation != "" {
		if !validGeneration(generation) {
			return nil, fmt.Errorf("extensions: invalid secret generation %q", generation)
		}
		path := filepath.Join(dir, secretsGenerationDir, generation+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, err
		}
		var secrets struct {
			Generation string         `json:"generation"`
			Values     map[string]any `json:"values"`
		}
		if err := json.Unmarshal(data, &secrets); err != nil || secrets.Generation != generation {
			return nil, nil
		}
		return secrets.Values, nil
	}

	data, err := os.ReadFile(filepath.Join(dir, legacySecretsFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("extensions: invalid secrets file: %w", err)
	}
	return values, nil
}

func validGeneration(generation string) bool {
	if generation == "" {
		return false
	}
	for i := 0; i < len(generation); i++ {
		c := generation[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	return true
}

// safeJoin joins an id under a root, refusing one that would escape it.
func safeJoin(root, id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\`) || id == "." || id == ".." {
		return "", fmt.Errorf("extensions: invalid id %q", id)
	}
	return filepath.Join(root, id), nil
}

// Injection is what a launch adds from the configuration: environment
// variables and leading arguments.
type Injection struct {
	Env  []string
	Args []string
}

// Inject adds a configuration to a launch: every schema field with a value
// contributes its environment variable (envVar, or the descriptor's
// environmentMapping) and its argument, as the old host did. Passwords go to
// the environment only: an argument is never a secret.
//
// The descriptor may be nil (a static provider that declares no
// configuration), in which case nothing is added.
func Inject(descriptor *Configuration, values map[string]any, base Injection) Injection {
	if descriptor == nil {
		return base
	}
	env := append([]string{}, base.Env...)
	args := append([]string{}, base.Args...)

	mapping := descriptor.EnvironmentMapping
	for _, field := range descriptor.Schema {
		value, ok := values[field.Key]
		if !ok || value == nil {
			continue
		}
		envVar := field.EnvVar
		if envVar == "" {
			envVar = mapping[field.Key]
		}
		if envVar != "" {
			if rendered, ok := renderValue(value); ok {
				env = append(env, envVar+"="+rendered)
			}
		}
		if field.Argument == "" || field.Type == "password" {
			continue
		}
		if value == false {
			continue
		}
		args = append(args, field.Argument)
		if value != true {
			if rendered, ok := renderValue(value); ok {
				args = append(args, rendered)
			}
		}
	}
	sort.Strings(env)
	return Injection{Env: env, Args: args}
}

// renderValue renders a configuration value as a string, as the old host
// did: strings, booleans and numbers as themselves, string lists joined by
// commas, and anything else nothing.
func renderValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case json.Number:
		return v.String(), true
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			text, ok := item.(string)
			if !ok {
				return "", false
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, ","), true
	default:
		return "", false
	}
}

// ConfigurationCommand is the tool-owned configuration's runnable command:
// the provider's own editor, opened in the terminal.
func ConfigurationCommand(integration Integration, description Description) (Command, bool) {
	config := description.Configuration
	if config == nil || config.Owner != "tool" || len(config.OpenCommand) == 0 {
		return Command{}, false
	}
	return Command{
		ID:          "configuration",
		Name:        "Configuration",
		Description: "Edit this integration's configuration",
		Execution: Execution{
			Program:    "self",
			ArgsPrefix: append([]string{}, config.OpenCommand...),
			Mode:       "pty",
		},
	}, true
}

// SaveConfiguration validates a configuration against its schema and writes
// it: the password fields go into a **new** secrets generation (0600, atomic),
// the values file holds a placeholder in their place, and the stored schema is
// written with them so a later read knows the shape. Generations older than
// the new one are removed.
//
// Validation is the old build's: no unknown keys, a required field must be
// present (its default filling in when it is not), each type must match, a
// select's value must be one of its options, a number must be inside its
// range, and a text must be inside its length.
func SaveConfiguration(paths Paths, id string, schema []ConfigField, values map[string]any) error {
	if err := validID(id); err != nil {
		return err
	}
	if err := ValidateConfiguration(schema, values); err != nil {
		return err
	}
	dir, err := safeJoin(paths.Data, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// The stored envelope: a config generation the file already carried, or
	// the schema's own, so a later read knows what the values were written
	// against.
	// The stored envelope's config generation is carried forward; the
	// description's own generation fills it in when the file has none.
	configVersion := 0
	if previous, err := parseStoredConfigurationFile(filepath.Join(dir, configFile)); err == nil {
		configVersion = previous.ConfigVersion
	}

	passwords := map[string]bool{}
	for _, field := range schema {
		if strings.EqualFold(field.Type, "password") {
			passwords[field.Key] = true
		}
	}
	public := map[string]any{}
	secrets := map[string]any{}
	for key, value := range values {
		if passwords[key] {
			secrets[key] = value
			public[key] = PasswordPlaceholder
			continue
		}
		public[key] = value
	}
	for _, field := range schema {
		if !passwords[field.Key] || public[field.Key] != nil {
			continue
		}
		// A password the caller left out keeps its placeholder, so the secret
		// this machine holds is not lost by a save that did not touch it.
		if field.Required {
			public[field.Key] = PasswordPlaceholder
		}
	}

	generation, err := newGeneration()
	if err != nil {
		return err
	}
	if err := writeSecrets(dir, generation, secrets); err != nil {
		return err
	}
	envelope := map[string]any{
		"configVersion":    configVersion,
		"secretGeneration": generation,
		"values":           public,
	}
	if len(schema) > 0 {
		envelope["schema"] = redactedSchema(schema)
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteFile(filepath.Join(dir, configFile), data); err != nil {
		return err
	}
	removeOldGenerations(dir, generation)
	return nil
}

// ValidateConfiguration checks values against a schema, the old build's rules.
func ValidateConfiguration(schema []ConfigField, values map[string]any) error {
	fields := map[string]ConfigField{}
	for _, field := range schema {
		fields[field.Key] = field
	}
	for key := range values {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("extensions: unknown configuration key %q", key)
		}
	}
	for _, field := range schema {
		value, ok := values[field.Key]
		if !ok || value == nil {
			if field.Default != nil {
				continue // the default fills in at read time
			}
			if field.Required {
				return fmt.Errorf("extensions: missing required configuration key %q", field.Key)
			}
			continue
		}
		if err := validateFieldValue(field, value); err != nil {
			return err
		}
	}
	return nil
}

// validateFieldValue checks one value against its field.
func validateFieldValue(field ConfigField, value any) error {
	kind := strings.ToLower(field.Type)
	switch kind {
	case "text", "password", "path", "select":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("extensions: the value of %q is not text", field.Key)
		}
	case "multiselect":
		values, ok := value.([]any)
		if !ok {
			return fmt.Errorf("extensions: the value of %q is not a list", field.Key)
		}
		for _, item := range values {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("extensions: the value of %q is not a list of text", field.Key)
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("extensions: the value of %q is not a boolean", field.Key)
		}
	case "number":
		number, ok := asNumber(value)
		if !ok {
			return fmt.Errorf("extensions: the value of %q is not a number", field.Key)
		}
		if field.Minimum != nil && number < *field.Minimum || field.Maximum != nil && number > *field.Maximum {
			return fmt.Errorf("extensions: the value of %q is outside its allowed range", field.Key)
		}
	default:
		return fmt.Errorf("extensions: unknown configuration type %q for %q", field.Type, field.Key)
	}
	if kind == "select" {
		if !optionAllowed(field.Options, value) {
			return fmt.Errorf("extensions: the value of %q is not an allowed option", field.Key)
		}
	}
	if kind == "multiselect" {
		for _, item := range value.([]any) {
			if !optionAllowed(field.Options, item) {
				return fmt.Errorf("extensions: the value of %q is not an allowed option", field.Key)
			}
		}
	}
	if kind == "text" {
		if text, ok := value.(string); ok {
			length := len([]rune(text))
			if field.MinLength != nil && length < *field.MinLength ||
				field.MaxLength != nil && length > *field.MaxLength {
				return fmt.Errorf("extensions: the value of %q is outside its allowed length", field.Key)
			}
		}
	}
	return nil
}

// optionAllowed reports whether a select's value is one of its options,
// compared as text: a JSON number in the options matches a string value.
func optionAllowed(options []any, value any) bool {
	for _, option := range options {
		if option == value {
			return true
		}
		if text, ok := option.(string); ok {
			if other, ok := value.(string); ok && text == other {
				return true
			}
		}
	}
	return false
}

// asNumber reads a JSON number of either shape.
func asNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// redactedSchema is the schema as it is stored: a password's default value
// becomes the placeholder, so a default credential is not written to disk.
func redactedSchema(schema []ConfigField) []ConfigField {
	out := make([]ConfigField, len(schema))
	copy(out, schema)
	for i := range out {
		if strings.EqualFold(out[i].Type, "password") && out[i].Default != nil {
			out[i].Default = PasswordPlaceholder
		}
	}
	return out
}

// newGeneration is a fresh secrets generation id.
func newGeneration() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// writeSecrets writes one secrets generation, atomically and privately.
func writeSecrets(dir, generation string, secrets map[string]any) error {
	if secrets == nil {
		secrets = map[string]any{}
	}
	secretsDir := filepath.Join(dir, secretsGenerationDir)
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return err
	}
	document, err := json.MarshalIndent(struct {
		Generation string         `json:"generation"`
		Values     map[string]any `json:"values"`
	}{generation, secrets}, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteFile(filepath.Join(secretsDir, generation+".json"), document); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(secretsDir, generation+".json"), 0o600)
}

// removeOldGenerations drops the secrets generations the new one replaced.
func removeOldGenerations(dir, current string) {
	entries, err := os.ReadDir(filepath.Join(dir, secretsGenerationDir))
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".json")
		if name == current || !validGeneration(name) {
			continue
		}
		os.Remove(filepath.Join(filepath.Join(dir, secretsGenerationDir), entry.Name()))
	}
}

// atomicWriteFile writes data to path: a temporary file in the same directory,
// flushed, then renamed over the target.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// parseStoredConfigurationFile reads a configuration file's envelope, for the
// config generation a save has to carry forward.
func parseStoredConfigurationFile(path string) (StoredConfiguration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredConfiguration{}, err
	}
	return parseStoredConfiguration(data)
}
