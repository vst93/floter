package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProtocolVersion is the provider protocol this build speaks.
const ProtocolVersion = "1.0"

// Description is a provider's answer to `describe`: its identity and the
// commands the installed version offers (schemas/provider-description).
type Description struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Provider        ProviderIdentity `json:"provider"`
	Commands        []Command        `json:"commands"`
	Configuration   *Configuration   `json:"configuration,omitempty"`
}

// ProviderIdentity names the provider that answered.
type ProviderIdentity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// Command is one command the provider offers.
type Command struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Aliases     []string   `json:"aliases"`
	Keywords    []string   `json:"keywords"`
	Execution   Execution  `json:"execution"`
	Arguments   []Argument `json:"arguments"`
}

// Execution is how a command runs: the program (self means the provider
// itself), the argv prefix, the mode and the working directory.
type Execution struct {
	Program          string   `json:"program"`
	ArgsPrefix       []string `json:"argsPrefix"`
	Mode             string   `json:"mode"` // pty | capture | external
	WorkingDirectory string   `json:"workingDirectory"`
}

// NormalizedMode returns the execution mode with the legacy `capture` value
// normalized to `pty`, as the old host did.
func (e Execution) NormalizedMode() string {
	switch e.Mode {
	case "pty", "capture", "":
		return "pty"
	case "external":
		return "external"
	default:
		return "pty"
	}
}

// Argument is one command-line parameter the provider declares.
type Argument struct {
	Names       []string `json:"names"`
	Kind        string   `json:"kind"` // flag | string | integer | number | path | directory | url | enum | command
	Description string   `json:"description"`
	TakesValue  bool     `json:"takesValue"`
	Required    bool     `json:"required"`
	Repeatable  bool     `json:"repeatable"`
	Values      []string `json:"values"`
	ValueHint   string   `json:"valueHint"`
}

// Configuration is the integration's configuration contract: the host owns
// the values and renders them (a schema), or the tool owns them and the host
// only opens its editor (openCommand).
type Configuration struct {
	ConfigVersion      int               `json:"configVersion"`
	Owner              string            `json:"owner"` // host | tool
	OpenCommand        []string          `json:"openCommand"`
	EnvironmentMapping map[string]string `json:"environmentMapping"`
	Schema             []ConfigField     `json:"schema"`
}

// ConfigField is one field of a host-owned configuration.
type ConfigField struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Default     any      `json:"default"`
	Options     []any    `json:"options"`
	Minimum     *float64 `json:"minimum"`
	Maximum     *float64 `json:"maximum"`
	MinLength   *int     `json:"minLength"`
	MaxLength   *int     `json:"maxLength"`
	EnvVar      string   `json:"envVar"`
	Argument    string   `json:"argument"`
}

// ErrBadDescription is what a description parser reports for a payload that
// is not a provider description.
var ErrBadDescription = errors.New("extensions: invalid provider description")

// ParseDescription decodes and checks a provider description.
func ParseDescription(data []byte) (Description, error) {
	var d Description
	if err := json.Unmarshal(data, &d); err != nil {
		return Description{}, fmt.Errorf("%w: %v", ErrBadDescription, err)
	}
	if d.ProtocolVersion != ProtocolVersion {
		return Description{}, fmt.Errorf("%w: protocol %q, this build speaks %s", ErrBadDescription, d.ProtocolVersion, ProtocolVersion)
	}
	if d.Provider.ID == "" || d.Provider.Name == "" {
		return Description{}, fmt.Errorf("%w: the provider names no identity", ErrBadDescription)
	}
	for i, command := range d.Commands {
		if command.ID == "" || command.Name == "" {
			return Description{}, fmt.Errorf("%w: command %d names no identity", ErrBadDescription, i)
		}
	}
	return d, nil
}

// LoadStaticDescription reads a provider description a package ships, for
// the static-descriptor provider kind.
func LoadStaticDescription(path string) (Description, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Description{}, err
	}
	return ParseDescription(data)
}

// joinWithin joins a package-relative path, refusing absolute paths and
// escapes as the manifest schema does.
func joinWithin(dir, rel string) (string, bool) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	joined := filepath.Join(dir, rel)
	cleanDir := filepath.Clean(dir)
	cleanJoined := filepath.Clean(joined)
	if cleanJoined != cleanDir && !strings.HasPrefix(cleanJoined, cleanDir+string(filepath.Separator)) {
		return "", false
	}
	return joined, true
}

// StaticDescriptionPath returns the descriptor file for an integration whose
// manifest names one, relative to the package directory.
func StaticDescriptionPath(integration Integration) (string, bool) {
	descriptor := integration.Manifest.Provider.Descriptor
	if descriptor == "" {
		return "", false
	}
	dir := integration.PackageDir()
	if dir == "" {
		return "", false
	}
	return joinWithin(dir, descriptor)
}
