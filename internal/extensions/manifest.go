package extensions

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

// Manifest is an extension package manifest (floter.extension.json). It is
// parsed strictly: a key this struct does not declare is a load error, the way
// the old host's `deny_unknown_fields` (and the bundled schema's
// `additionalProperties: false`) refused one. Fields this build does not act on
// are still declared, so a manifest the schema allows parses rather than being
// rejected for a field the host merely ignores.
type Manifest struct {
	SchemaVersion string `json:"schemaVersion"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Homepage      string `json:"homepage"`
	Icon          string `json:"icon"`
	// Version is the package version when the manifest carries one; the
	// schema does not require it, and a local package may not.
	Version string `json:"version"`

	Publisher     Publisher     `json:"publisher"`
	Compatibility Compatibility `json:"compatibility"`
	Distribution  Distribution  `json:"distribution"`
	Runtime       Runtime       `json:"runtime"`
	Provider      Provider      `json:"provider"`
	// Signatures is a package signature declaration. This build parses it so a
	// signed manifest loads, but it does not verify signatures.
	Signatures *SignatureConfig `json:"signatures,omitempty"`

	Platforms []string `json:"platforms"`
	// PlatformOverrides change the provider on one platform target: the
	// schema's `platformOverrides`, keyed by an identifier such as
	// `linux-x86_64-gnu` or `darwin-any` (see Platform).
	PlatformOverrides map[string]PlatformOverride `json:"platformOverrides"`
	Permissions       []string                    `json:"permissions"`
	// Output is where a manual run sends its output. The schema's default is
	// `background`, so it is omitted from a manifest that keeps the default —
	// which is exactly the spelling every manifest written before the field
	// existed already has.
	Output string `json:"output,omitempty"`
	// Params are the manifest's declared inputs: the values a run needs,
	// which the host renders a form for and turns into argv. They travel with
	// the manifest — the digest, the export, the approval — so editing them
	// invalidates an approval on purpose.
	Params    []ParamDefinition `json:"params"`
	Lifecycle Lifecycle         `json:"lifecycle"`
}

// The two routes a command's output can take: the terminal surface, or a
// headless run whose output the launcher shows.
const (
	RouteTerminal   = "terminal"
	RouteBackground = "background"
)

// Route is where a command's output goes: the manifest's `output` mode. An
// undeclared mode (or one this build does not recognise) keeps the schema's
// default, `background` — the compatibility rule the field was added under, so
// every manifest written before `output` existed runs silently and reports on
// completion exactly as it always did. Only an explicit `terminal` streams the
// run through a PTY.
func (m Manifest) Route() string {
	if strings.EqualFold(strings.TrimSpace(m.Output), RouteTerminal) {
		return RouteTerminal
	}
	return RouteBackground
}

// SignatureConfig is a manifest's package signature declaration.
type SignatureConfig struct {
	URL       string `json:"url"`
	PublicKey string `json:"publicKey"`
	Algorithm string `json:"algorithm"`
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
	// PlatformPackages and Executable belong to the schema's `managed` and
	// `linked` runtimes. They are parsed so such a manifest reaches the
	// combination rule (which refuses them) instead of failing as an unknown
	// field.
	PlatformPackages map[string]string `json:"platformPackages,omitempty"`
	Executable       string            `json:"executable,omitempty"`
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
	Completions            []ShellCompletion `json:"completions"`
	ConfigurationTemplates []Template        `json:"configurationTemplates"`
	Probes                 []Probe           `json:"probes"`
	Launch                 Launch            `json:"launch"`
}

// ShellCompletion installs a shell completion definition.
type ShellCompletion struct {
	Shell     string   `json:"shell"`
	Source    string   `json:"source"`
	Args      []string `json:"args"`
	FileName  string   `json:"fileName"`
	TimeoutMs int      `json:"timeoutMs"`
}

// The serde defaults the old host filled in for an omitted timeout: a
// completion gets 3s and a probe 2s (`lifecycle.rs`
// `default_completion_timeout` / `default_probe_timeout`). A zero here is an
// omitted field, not a request for an immediate timeout.
const (
	DefaultCompletionTimeoutMs = 3000
	DefaultProbeTimeoutMs      = 2000
)

// EffectiveTimeoutMs is the completion's timeout, or the schema default when
// the manifest left it out.
func (c ShellCompletion) EffectiveTimeoutMs() int {
	if c.TimeoutMs <= 0 {
		return DefaultCompletionTimeoutMs
	}
	return c.TimeoutMs
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

// EffectiveTimeoutMs is the probe's timeout, or the schema default when the
// manifest left it out.
func (p Probe) EffectiveTimeoutMs() int {
	if p.TimeoutMs <= 0 {
		return DefaultProbeTimeoutMs
	}
	return p.TimeoutMs
}

// Launch is how a command of the extension runs.
type Launch struct {
	Command LaunchCommand `json:"command"`
	// CWDPolicy is where the command starts. It parses the schema's string
	// and object spellings (see CWDPolicy), rather than a bare string that
	// makes a legal object fail the whole manifest.
	CWDPolicy CWDPolicy `json:"cwdPolicy"`
	// Terminal is what the launch asks of the terminal it runs in. Only the
	// color changes this host: it decides what TERM and COLORTERM say.
	Terminal Terminal `json:"terminal"`
	// RestorePolicy is what the host does with a launch's session when the
	// window reopens: `reattach`, `restart`, or `none`. The schema's default
	// is `reattach`, so an absent field means that.
	RestorePolicy string `json:"restorePolicy"`
}

// EffectiveRestorePolicy is the launch's restore policy, or the schema's
// default (`reattach`) when the manifest left it out.
func (l Launch) EffectiveRestorePolicy() string {
	if strings.TrimSpace(l.RestorePolicy) == "" {
		return "reattach"
	}
	return l.RestorePolicy
}

// Terminal is the launch's terminal requirement. Only the color changes what
// this host does; the schema's other fields (required, unicode, mouse,
// bracketed paste, synchronized output, keyboard protocol) describe the pty the
// framework always provides. They are parsed so a manifest declaring them is
// not refused as unknown, but they are not acted on.
type Terminal struct {
	Color              string `json:"color"`
	Required           bool   `json:"required,omitempty"`
	Unicode            bool   `json:"unicode,omitempty"`
	Mouse              string `json:"mouse,omitempty"`
	BracketedPaste     bool   `json:"bracketedPaste,omitempty"`
	SynchronizedOutput bool   `json:"synchronizedOutput,omitempty"`
	KeyboardProtocol   string `json:"keyboardProtocol,omitempty"`
}

// LaunchCommand is the program and leading arguments of a launch.
type LaunchCommand struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
}

// terminalEnvironment is the environment the old host gave a launch's terminal
// (launch.rs `terminal_environment`): the declared color names TERM and
// COLORTERM, and every launch is named as floter's. The shipped default is the
// 256-color pair, which is what an absent block means.
func terminalEnvironment(color string) []string {
	term, colorterm := "floter-256color", "truecolor"
	switch strings.ToLower(strings.TrimSpace(color)) {
	case "none":
		term, colorterm = "dumb", ""
	case "8":
		term, colorterm = "xterm", ""
	case "256":
		term, colorterm = "floter-256color", ""
	}
	return []string{"TERM=" + term, "COLORTERM=" + colorterm, "TERM_PROGRAM=floter"}
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

// decodeManifest parses manifest bytes strictly: an unknown field is an error
// (`json.Decoder.DisallowUnknownFields`, the old host's `deny_unknown_fields`),
// and trailing data after the object is refused. Nested structs are held to the
// same rule by the decoder.
func decodeManifest(data []byte) (Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var m Manifest
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, err
	}
	// Reject a second JSON value (or any trailing non-whitespace): a manifest
	// is exactly one object, and silently ignoring the tail would let a file
	// carry bytes no validator ever saw.
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return Manifest{}, errors.New("extensions: the manifest has trailing data")
	}
	return m, nil
}

// LoadManifest reads and parses a manifest file.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, ErrNoManifest
		}
		return Manifest{}, err
	}
	m, err := decodeManifest(data)
	if err != nil {
		return Manifest{}, err
	}
	if m.ID == "" || m.Name == "" {
		return Manifest{}, errors.New("extensions: the manifest names no extension")
	}
	// Every structural rule is applied while parsing, as the old host did, so
	// nothing downstream (an export's staging write, a descriptor read) has to
	// defend against a package pointing outside itself or declaring a
	// lifecycle the host cannot run.
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
