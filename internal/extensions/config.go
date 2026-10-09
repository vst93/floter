package extensions

import (
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
