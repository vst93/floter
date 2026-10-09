package extensions

import (
	"encoding/json"
	"errors"
	"os"
)

// Manifest is an extension package manifest (floter.extension.json), the
// subset this build reads. Unknown keys are ignored: the manifest is input,
// never written back.
type Manifest struct {
	SchemaVersion string `json:"schemaVersion"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Homepage      string `json:"homepage"`
	Icon          string `json:"icon"`

	Publisher     Publisher     `json:"publisher"`
	Compatibility Compatibility `json:"compatibility"`
	Distribution  Distribution  `json:"distribution"`
	Runtime       Runtime       `json:"runtime"`
	Provider      Provider      `json:"provider"`

	Platforms   []string  `json:"platforms"`
	Permissions []string  `json:"permissions"`
	Output      string    `json:"output"`
	Lifecycle   Lifecycle `json:"lifecycle"`
}

// Publisher identifies who ships the extension.
type Publisher struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Compatibility is the range of hosts and provider protocols the extension
// supports.
type Compatibility struct {
	Floter           string `json:"floter"`
	ProviderProtocol string `json:"providerProtocol"`
}

// Distribution says where the package came from.
type Distribution struct {
	Type string `json:"type"` // local | built-in
}

// Runtime is how the extension's program is found: an executable on the
// machine, or a script the host runs with an interpreter.
type Runtime struct {
	// Type is "system" or "script".
	Type string `json:"type"`
	// ExecutableNames are the program names to look for (system).
	ExecutableNames []string `json:"executableNames"`
	// Language and Path describe a script runtime.
	Language string `json:"language"`
	Path     string `json:"path"`
	// VersionArgs is how the runtime reports its version.
	VersionArgs []string `json:"versionArgs"`
}

// Provider is how the host asks the extension for its commands.
type Provider struct {
	// Type is "executable" or "static-descriptor".
	Type string `json:"type"`
	// Descriptor is a file, relative to the package, holding a static
	// provider description.
	Descriptor string `json:"descriptor"`
	// ArgsPrefix is passed before the protocol arguments.
	ArgsPrefix []string `json:"argsPrefix"`
	// DescribeTimeoutMs and CompleteTimeoutMs bound the protocol calls;
	// zero means the shipped defaults (5000 and 800).
	DescribeTimeoutMs int               `json:"describeTimeoutMs"`
	CompleteTimeoutMs int               `json:"completeTimeoutMs"`
	Environment       map[string]string `json:"environment"`
}

// Lifecycle is what the host does around an install: completions,
// configuration templates, health probes and how a command launches.
type Lifecycle struct {
	Completions            []Completion `json:"completions"`
	ConfigurationTemplates []Template   `json:"configurationTemplates"`
	Probes                 []Probe      `json:"probes"`
	Launch                 Launch       `json:"launch"`
}

// Completion installs a shell completion definition.
type Completion struct {
	Shell     string   `json:"shell"`
	Source    string   `json:"source"`
	Args      []string `json:"args"`
	FileName  string   `json:"fileName"`
	TimeoutMs int      `json:"timeoutMs"`
}

// Template copies a configuration file into the integration's data
// directory on install.
type Template struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Probe is a health check the host runs.
type Probe struct {
	ID        string   `json:"id"`
	Args      []string `json:"args"`
	TimeoutMs int      `json:"timeoutMs"`
	Required  bool     `json:"required"`
}

// Launch is how a command of the extension runs.
type Launch struct {
	Command   LaunchCommand `json:"command"`
	CWDPolicy string        `json:"cwdPolicy"`
}

// LaunchCommand is the program and leading arguments of a launch.
type LaunchCommand struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
}

// The shipped protocol defaults, from the schema.
const (
	DefaultDescribeTimeoutMs = 5000
	DefaultCompleteTimeoutMs = 800
)

// DescribeTimeout is the manifest's describe timeout, or the shipped default.
func (m Manifest) DescribeTimeout() int {
	if m.Provider.DescribeTimeoutMs >= 100 {
		return m.Provider.DescribeTimeoutMs
	}
	return DefaultDescribeTimeoutMs
}

// CompleteTimeout is the manifest's completion timeout, or the shipped
// default.
func (m Manifest) CompleteTimeout() int {
	if m.Provider.CompleteTimeoutMs >= 50 {
		return m.Provider.CompleteTimeoutMs
	}
	return DefaultCompleteTimeoutMs
}

// ErrNoManifest is what LoadManifest reports for a file that is not there.
var ErrNoManifest = errors.New("extensions: no manifest")

// LoadManifest reads and parses a manifest file.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, ErrNoManifest
		}
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	if m.ID == "" || m.Name == "" {
		return Manifest{}, errors.New("extensions: the manifest names no extension")
	}
	return m, nil
}
