package extensions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Connecting a local tool: the flow behind "point floter at a program on this
// machine and let it become an integration". The package is written where the
// host keeps its own generated integrations (`<data>/<id>/integration`), a
// manifest and a static descriptor are generated from the tool's own `--help`,
// and the package then goes through the *same* local install every other tool
// does — so its permissions are still reviewed, its digest is still recorded,
// and a package that cannot run never replaces a working install.

// CustomRequest is what a caller asks to connect. The id is the host's own
// (minted below): a caller that names one is ignored, which keeps two
// connections from fighting over a name.
type CustomRequest struct {
	// Name is what the user calls the integration.
	Name string
	// Command is the word that summons it, its id inside the descriptor.
	Command string
	// Version is the package version; a tool that reports a real semver
	// replaces it (see the version probe).
	Version string
	// ExecutablePath is the program on this machine. Empty means a script.
	ExecutablePath string
	// Mode is "executable" or "script".
	Mode string
	// ScriptLanguage and ScriptContent describe a script integration: the
	// interpreter's name and the script the host writes.
	ScriptLanguage string
	ScriptContent  string
	// ArgsPrefix is passed before the tool's own arguments, VersionArgs asks
	// the tool for its version.
	ArgsPrefix  []string
	VersionArgs []string
	// Description is the user's own words, empty for none (never a generated
	// sentence: a field the user cleared must stay cleared).
	Description string
	// Permissions are the ids the package declares.
	Permissions []string
	// Platforms limits the integration to the operating systems it names;
	// empty means everywhere.
	Platforms []string
	// Output is where a run sends its output: "terminal" or "background".
	Output string
}

// ErrBadRequest is what a connection reports for a request that cannot be
// honoured (no name, no program, a script with no language).
var ErrBadRequest = errors.New("extensions: the connection request is not usable")

// CreateCustom writes a local tool's package and installs it. `approved` says
// the user has seen and accepted the permissions (see PermissionApproval for
// the flow); the returned entry is the repository's record.
func CreateCustom(ctx context.Context, paths Paths, request CustomRequest, approved bool) (Entry, error) {
	request, err := normalizeCustom(request)
	if err != nil {
		return Entry{}, err
	}
	if err := paths.Ensure(); err != nil {
		return Entry{}, err
	}
	id, packageDir, err := reserveCustomPackage(paths)
	if err != nil {
		return Entry{}, err
	}
	// A package that fails to write leaves nothing behind: the reserved
	// directory goes with the failure.
	cleanup := func() { _ = os.RemoveAll(filepath.Dir(packageDir)) }
	if err := writeCustomPackage(ctx, paths, packageDir, id, request); err != nil {
		cleanup()
		return Entry{}, err
	}
	prepared, err := PrepareLocal(paths, packageDir)
	if err != nil {
		cleanup()
		return Entry{}, err
	}
	if request.Mode == "executable" {
		prepared.ExecutablePath = request.ExecutablePath
	}
	entry, err := prepared.Commit(approved)
	if err != nil {
		// A package the user did not approve keeps its files: the refusal was
		// about *this* manifest, and the next attempt reviews the same one.
		if errors.Is(err, ErrPermissionApprovalRequired) {
			return Entry{}, err
		}
		cleanup()
		return Entry{}, err
	}
	// The tool's own version, when it reports one: the recorded version is
	// what the drift re-probe compares against.
	if version := probeVersion(ctx, paths, entry.ID); version != "" {
		_ = SetToolVersion(paths, entry.ID, version)
		entry.ToolVersion = &version
	}
	return entry, nil
}

// normalizeCustom checks a request and fills its defaults.
func normalizeCustom(request CustomRequest) (CustomRequest, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Command = strings.TrimSpace(request.Command)
	request.ExecutablePath = strings.TrimSpace(request.ExecutablePath)
	request.Mode = strings.ToLower(strings.TrimSpace(request.Mode))
	if request.Name == "" {
		return request, fmt.Errorf("%w: no name", ErrBadRequest)
	}
	if request.Mode == "" {
		request.Mode = "executable"
	}
	switch request.Mode {
	case "executable":
		if request.ExecutablePath == "" {
			return request, fmt.Errorf("%w: no program", ErrBadRequest)
		}
		if !isExecutableFile(request.ExecutablePath) {
			return request, fmt.Errorf("%w: %s cannot be run", ErrBadRequest, request.ExecutablePath)
		}
		if request.Command == "" {
			request.Command = executableCommand(request.ExecutablePath)
		}
	case "script":
		if request.ScriptLanguage == "" {
			return request, fmt.Errorf("%w: a script needs a language", ErrBadRequest)
		}
		if strings.TrimSpace(request.ScriptContent) == "" {
			return request, fmt.Errorf("%w: the script is empty", ErrBadRequest)
		}
		if _, ok := findInterpreter(request.ScriptLanguage); !ok {
			return request, fmt.Errorf("%w: no %s interpreter is available", ErrBadRequest, request.ScriptLanguage)
		}
		if request.Command == "" {
			request.Command = "script"
		}
	default:
		return request, fmt.Errorf("%w: unknown mode %q", ErrBadRequest, request.Mode)
	}
	if request.Command == "" {
		request.Command = "tool"
	}
	if request.Version == "" {
		request.Version = "0.0.1"
	}
	switch request.Output {
	case "", "terminal":
	case "background":
	default:
		return request, fmt.Errorf("%w: unknown output %q", ErrBadRequest, request.Output)
	}
	for _, permission := range request.Permissions {
		if !knownPermission(permission) {
			return request, fmt.Errorf("%w: unknown permission %q", ErrBadRequest, permission)
		}
	}
	return request, nil
}

// executableCommand is the word a program's own file name gives: `rg` from
// `/usr/bin/rg`, with a Windows launcher suffix dropped.
func executableCommand(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com"} {
		if trimmed, ok := strings.CutSuffix(strings.ToLower(name), suffix); ok {
			return trimmed
		}
	}
	return name
}

// reserveCustomPackage mints an id and creates its package directory. The
// directory is created (not merely named) so two connections cannot mint the
// same one.
func reserveCustomPackage(paths Paths) (string, string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		id, err := mintCustomID()
		if err != nil {
			return "", "", err
		}
		dataRoot := filepath.Join(paths.Data, id)
		packageDir := filepath.Join(dataRoot, "integration")
		if _, err := os.Stat(dataRoot); err == nil {
			continue // taken
		}
		if err := os.MkdirAll(packageDir, 0o755); err != nil {
			return "", "", err
		}
		return id, packageDir, nil
	}
	return "", "", errors.New("extensions: could not mint a free id")
}

// mintCustomID is a fresh `local.<8 hex>` id.
func mintCustomID() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "local." + hex.EncodeToString(buf[:]), nil
}

// writeCustomPackage writes the manifest, the descriptor and (for a script)
// the script itself into a reserved package directory.
func writeCustomPackage(ctx context.Context, paths Paths, packageDir, id string, request CustomRequest) error {
	manifest := customManifest(id, request)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(packageDir, manifestFileName), manifestBytes, 0o644); err != nil {
		return err
	}

	// package.json: the same marker every local package carries, so the
	// loader treats this one like any other.
	packageJSON := map[string]any{
		"name":     "floter-local-" + strings.NewReplacer(".", "-", "_", "-").Replace(id),
		"version":  request.Version,
		"private":  true,
		"keywords": []string{"floter-extension"},
		"floter":   map[string]any{"manifest": manifestFileName},
	}
	packageBytes, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return err
	}
	packageBytes = append(packageBytes, '\n')
	if err := os.WriteFile(filepath.Join(packageDir, "package.json"), packageBytes, 0o644); err != nil {
		return err
	}

	if request.Mode == "script" {
		scriptPath := filepath.Join(packageDir, "provider."+scriptExtension(request.ScriptLanguage))
		if err := os.WriteFile(scriptPath, []byte(request.ScriptContent), 0o755); err != nil {
			return err
		}
	}

	// The descriptor: one root command, plus the subcommands the tool's own
	// help lists (the same derivation the re-probe re-runs later).
	descriptor := customDescriptor(request)
	if request.Mode == "executable" {
		probe := Integration{
			Entry:    Entry{ID: id, ExecutablePath: request.ExecutablePath},
			Paths:    paths,
			Manifest: manifest,
		}
		if binding, resolveErr := ResolveRuntime(probe); resolveErr == nil {
			if derivation := probeDeriveWith(ctx, binding, probe); !derivation.Empty() {
				descriptor.Commands = derivedCommands(descriptor.Commands[0], derivation)
			}
		}
	}
	descriptorBytes, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return err
	}
	descriptorBytes = append(descriptorBytes, '\n')
	return os.WriteFile(filepath.Join(packageDir, "provider-description.json"), descriptorBytes, 0o644)
}

// customManifest builds the manifest the host writes for a connected tool.
func customManifest(id string, request CustomRequest) Manifest {
	description := strings.TrimSpace(request.Description)
	runtime := Runtime{Type: "system", ExecutableNames: []string{executableCommand(request.ExecutablePath)}}
	if request.Mode == "script" {
		runtime = Runtime{
			Type:        "script",
			Language:    request.ScriptLanguage,
			Path:        "provider." + scriptExtension(request.ScriptLanguage),
			VersionArgs: append([]string{}, request.VersionArgs...),
		}
	} else {
		runtime.VersionArgs = append([]string{}, request.VersionArgs...)
	}
	manifest := Manifest{
		SchemaVersion: "2.0",
		ID:            id,
		Name:          request.Name,
		Description:   description,
		Version:       request.Version,
		Publisher:     Publisher{ID: "local-user", Name: "Local user"},
		Compatibility: Compatibility{Floter: ">=0.3.0", ProviderProtocol: "^1.0"},
		Distribution:  Distribution{Type: "local"},
		Runtime:       runtime,
		Provider: Provider{
			Type:              "static-descriptor",
			Descriptor:        "provider-description.json",
			ArgsPrefix:        append([]string{}, request.ArgsPrefix...),
			DescribeTimeoutMs: 5000,
			CompleteTimeoutMs: 800,
		},
		Permissions: append([]string{}, request.Permissions...),
		Platforms:   append([]string{}, request.Platforms...),
	}
	if request.Output != "" {
		manifest.Output = request.Output
	}
	return manifest
}

// customDescriptor builds the root command of the descriptor the host writes.
func customDescriptor(request CustomRequest) Description {
	return Description{
		ProtocolVersion: ProtocolVersion,
		Provider: ProviderIdentity{
			ID:          "local",
			Name:        request.Name,
			Version:     request.Version,
			Description: strings.TrimSpace(request.Description),
		},
		Commands: []Command{{
			ID:          request.Command,
			Name:        request.Name,
			Description: strings.TrimSpace(request.Description),
			Execution: Execution{
				Program:          "self",
				ArgsPrefix:       append([]string{}, request.ArgsPrefix...),
				Mode:             "pty",
				WorkingDirectory: "current",
			},
		}},
	}
}

// scriptExtension is the interpreter language's file suffix.
func scriptExtension(language string) string {
	switch strings.ToLower(language) {
	case "shell", "sh", "bash":
		return "sh"
	case "python", "python3":
		return "py"
	case "node", "javascript", "js":
		return "js"
	case "ruby":
		return "rb"
	case "php":
		return "php"
	case "perl":
		return "pl"
	case "powershell", "pwsh":
		return "ps1"
	default:
		return "txt"
	}
}

// probeVersion runs the tool's own version probe and returns the semver it
// reported, or "" when it reported none.
func probeVersion(ctx context.Context, paths Paths, id string) string {
	integration, ok := LoadInventory(paths).WithID(id)
	if !ok {
		return ""
	}
	args := integration.Manifest.Runtime.VersionArgs
	if len(args) == 0 {
		args = []string{"--version"}
	}
	binding, err := ResolveRuntime(integration)
	if err != nil {
		return ""
	}
	return SemverFromVersionOutput(ProbeHelpText(ctx, binding, integration, args...))
}
