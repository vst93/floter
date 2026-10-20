package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Store is the app's view of the extension system: the repository, the
// installed manifests, and each provider's command catalog, refreshed
// together.
//
// It is safe from any goroutine. Refresh does the (slow) provider calls
// outside the lock, so readers never block on a provider.
type Store struct {
	paths Paths

	mu           sync.Mutex
	inventory    Inventory
	descriptions map[string]Description
	describeErr  map[string]error
	loaded       bool
	listeners    []func()
}

// OpenStore builds a store over the extension paths. Nothing is read until
// Refresh.
func OpenStore(paths Paths) *Store {
	return &Store{
		paths:        paths,
		inventory:    Inventory{Paths: paths},
		descriptions: map[string]Description{},
		describeErr:  map[string]error{},
	}
}

// Paths is the root the store reads.
func (s *Store) Paths() Paths { return s.paths }

// Refresh reloads the repository and every manifest, then asks each running
// integration's provider for its commands. Providers run outside the lock;
// the result is swapped in and the listeners are told.
func (s *Store) Refresh(ctx context.Context) {
	inventory := LoadInventory(s.paths)

	descriptions := map[string]Description{}
	describeErr := map[string]error{}
	for _, integration := range inventory.Integrations {
		if !integration.Entry.IsRunning() {
			continue
		}
		description, err := Describe(ctx, integration)
		if err != nil {
			describeErr[integration.Entry.ID] = err
			continue
		}
		descriptions[integration.Entry.ID] = description
	}

	s.mu.Lock()
	s.inventory = inventory
	s.descriptions = descriptions
	s.describeErr = describeErr
	s.loaded = true
	listeners := append([]func(){}, s.listeners...)
	s.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn()
		}
	}
}

// Loaded reports whether a refresh has finished at least once.
func (s *Store) Loaded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loaded
}

// Inventory is the last refresh's result.
func (s *Store) Inventory() Inventory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inventory
}

// Description is an integration's cached provider description.
func (s *Store) Description(id string) (Description, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	description, ok := s.descriptions[id]
	return description, ok
}

// DescribeError is why an integration's provider could not be asked, or nil.
func (s *Store) DescribeError(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.describeErr[id]
}

// OnChange registers a listener for a completed refresh.
func (s *Store) OnChange(fn func()) func() {
	s.mu.Lock()
	s.listeners = append(s.listeners, fn)
	index := len(s.listeners) - 1
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if index < len(s.listeners) {
			s.listeners[index] = nil
		}
	}
}

// SetEnabled turns an integration on or off in the repository and writes the
// file back. The in-memory inventory is updated too, so the caller does not
// have to refresh to see the change.
func (s *Store) SetEnabled(id string, enabled bool) error {
	repo, err := LoadRepository(s.paths.RepositoryFile)
	if err != nil {
		return err
	}
	entry, ok := repo.SetEnabled(id, enabled)
	if !ok {
		return ErrNoIntegration
	}
	if err := SaveRepository(s.paths.RepositoryFile, repo); err != nil {
		return err
	}

	s.mu.Lock()
	for i := range s.inventory.Integrations {
		if s.inventory.Integrations[i].Entry.ID == entry.ID {
			s.inventory.Integrations[i].Entry = entry
		}
	}
	if !enabled {
		delete(s.descriptions, id)
	}
	listeners := append([]func(){}, s.listeners...)
	s.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn()
		}
	}
	return nil
}

// ErrNoIntegration is what an operation reports for an id the repository
// does not know.
var ErrNoIntegration = errors.New("extensions: no such integration")

// launchDefaults is what a manifest's lifecycle.launch gives every command of
// an integration: the program and leading arguments that replace a command's
// own execution, the directory its cwd policy resolves to, and the terminal
// environment it asks for. Declared says whether the manifest set any of them,
// so an absent block adds nothing.
type launchDefaults struct {
	declared bool
	program  string
	args     []string
	dir      string
	env      []string
}

// launchOverrides reads the manifest's lifecycle.launch: the program and
// leading arguments every command of this integration runs with (an empty
// program means the commands' own execution stands), the directory its cwd
// policy resolves to, and the terminal environment it asks for.
//
// The cwd policy is resolved by CWDPolicy.Resolve against the integration's
// own data directory. There is no active terminal session's cwd to inherit in
// this build, so `inheritActiveSession`, `toolData`, `projectRoot` with no
// session, and the default all land on that data directory — which is where
// the old host's fallback chain lands too (`run.rs` `run_cwd`). The data
// directory is created first: a cwd that is not there fails the spawn.
func launchOverrides(integration Integration) launchDefaults {
	config := integration.Manifest.Lifecycle.Launch
	defaults := launchDefaults{
		program: config.Command.Program,
		args:    append([]string{}, config.Command.Args...),
	}
	dataDir := filepath.Join(integration.Paths.Data, integration.Entry.ID)
	_ = os.MkdirAll(dataDir, 0o755)
	home, _ := os.UserHomeDir()
	defaults.dir = config.CWDPolicy.Resolve(CwdContext{DataDir: dataDir, Home: home})
	// The terminal environment: a tool that declares what its terminal must
	// support gets exactly that, and every launch is named as floter's.
	defaults.env = terminalEnvironment(config.Terminal.Color)
	defaults.declared = defaults.program != "" || len(defaults.args) > 0 ||
		config.CWDPolicy.Kind != "" || strings.TrimSpace(config.Terminal.Color) != ""
	return defaults
}

// CommandInfo is one command of one integration, as the settings panel's
// switch list shows it: the identity the switch map is keyed by, and whether
// the integration's runtime resolves right now. A command whose runtime is
// missing is still listed — the switch keeps its state either way.
type CommandInfo struct {
	ExtensionID string
	CommandID   string
	Name        string
	Description string
	Aliases     []string
	Available   bool
	// Declared is whether the *provider's descriptor* declared this command.
	// The host synthesizes one command of its own — the tool-owned
	// configuration editor (`ConfigurationCommand`) — and the reference's
	// registry (`catalog::plugin_command_registry`, `main:src-tauri/src/
	// extensions/catalog.rs:427-449`) lists declared commands only, so the
	// panel's "Summon with: …" line is printed for a declared command and not
	// for the synthesized one (`ExtensionsPanel.tsx:2824-2829`, which gates the
	// line on `externalCommands.find(...)`).
	Declared bool
}

// CommandRegistry lists every command of every integration that has a
// provider description, sorted by (extension id, command id) so the panel's
// rows are deterministic.
//
// The rows carry no runtime answer. `CommandInfo.Available` used to be filled
// here by calling ResolveRuntime per integration, and that call is a stat walk
// (a `system` runtime with no recorded path is looked up by name in every
// search directory) — paid again on every call of a function the settings page
// builds its whole list from, sixty times a second while the page is open. The
// answer is the same one the card's own rows already ask for, and the shell
// holds it in the integration-facts memo (internal/shell/integrationfacts.go),
// so it is filled from there and this function stays a pure projection of the
// store's inventory.
func (s *Store) CommandRegistry() []CommandInfo {
	s.mu.Lock()
	inventory, descriptions := s.inventory, s.descriptions
	s.mu.Unlock()

	var out []CommandInfo
	for _, integration := range inventory.Integrations {
		description, ok := descriptions[integration.Entry.ID]
		if !ok {
			continue
		}
		available := false
		commands := description.Commands
		declared := len(commands)
		if configuration, ok := ConfigurationCommand(integration, description); ok {
			commands = append(append([]Command{}, commands...), configuration)
		}
		for index, command := range commands {
			out = append(out, CommandInfo{
				ExtensionID: integration.Entry.ID,
				CommandID:   command.ID,
				Name:        command.Name,
				Description: command.Description,
				Aliases:     append([]string{}, command.Aliases...),
				Available:   available,
				Declared:    index < declared,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ExtensionID != out[j].ExtensionID {
			return out[i].ExtensionID < out[j].ExtensionID
		}
		return out[i].CommandID < out[j].CommandID
	})
	return out
}

// CommandEntry is one runnable command of one integration, ready for the
// launcher: the argv the command runs, without caller arguments.
type CommandEntry struct {
	IntegrationID   string
	IntegrationName string
	ProviderName    string

	Command Command

	// Program is the resolved executable (or interpreter), and Args the
	// argv before the caller's own: the script path, the execution prefix.
	Program string
	Args    []string

	// Mode is the normalized execution mode, and Dir the working directory
	// the provider asked for.
	Mode string
	Dir  string
	// Route is where the output goes: the terminal surface, or a headless
	// run the launcher shows (the manifest's `output` mode).
	Route string

	// Env is the integration's configured environment ("KEY=value"),
	// injected when the command runs.
	Env []string

	// Declared is whether the provider's descriptor declared this command, as
	// opposed to the host synthesizing it — the tool-owned configuration
	// editor (`ConfigurationCommand`). The reference's command registry lists
	// declared commands only (`catalog::plugin_command_registry`,
	// `main:src-tauri/src/extensions/catalog.rs:427-449`), so the launcher has
	// no `configuration` mode: the editor is a row in the settings panel and
	// nothing else (`ExtensionsPanel.tsx:2824-2829` gates its summon line on
	// the same registry).
	Declared bool
}

// CommandEntries flattens every running integration's commands.
func (s *Store) CommandEntries() []CommandEntry {
	s.mu.Lock()
	inventory, descriptions := s.inventory, s.descriptions
	s.mu.Unlock()

	var out []CommandEntry
	for _, integration := range inventory.Integrations {
		description, ok := descriptions[integration.Entry.ID]
		if !ok {
			continue
		}
		binding, err := ResolveRuntime(integration)
		if err != nil {
			continue
		}
		stored, err := LoadStoredConfiguration(s.paths.Data, integration.Entry.ID)
		if err != nil {
			stored = StoredConfiguration{Values: map[string]any{}}
		}
		injection := Inject(description.Configuration, stored.Values, Injection{})

		commands := description.Commands
		declared := len(commands)
		if configuration, ok := ConfigurationCommand(integration, description); ok {
			commands = append(append([]Command{}, commands...), configuration)
		}
		// The manifest's lifecycle.launch is how the integration as a whole
		// runs: its program and leading arguments replace each command's own
		// execution, and its cwd policy and terminal environment come with it.
		launch := launchOverrides(integration)
		// The user's home directory is what a descriptor's "home"
		// workingDirectory means; an unavailable home is the old host's
		// empty answer (inherit).
		home, _ := os.UserHomeDir()
		for index, command := range commands {
			// The runtime binding runs the integration's own executable (an
			// interpreter and its script, or the tool itself), so those leading
			// arguments come first.
			program, args := binding.Program, append([]string{}, binding.Args...)
			// The descriptor names where the process starts as a behaviour;
			// "current" means the launch's own cwd.
			dir := WorkingDirectory(command.Execution.WorkingDirectory, launch.dir, home)
			env := append([]string{}, injection.Env...)
			if launch.program != "" {
				// `program: "self"` — the schema's default — is what the
				// binding resolved. A declared program replaces it, and the
				// interpreter arguments with it.
				if launch.program != "self" {
					program = launch.program
					args = []string{}
				}
				args = append(args, launch.args...)
			}
			if launch.declared {
				env = append(env, launch.env...)
			}
			args = append(args, command.Execution.ArgsPrefix...)
			entry := CommandEntry{
				IntegrationID:   integration.Entry.ID,
				IntegrationName: integration.Name,
				ProviderName:    description.Provider.Name,
				Command:         command,
				Program:         program,
				Args:            args,
				Mode:            command.Execution.NormalizedMode(),
				Dir:             dir,
				// The manifest's declared `output` mode decides where a run of
				// this command sends its output: the terminal surface, or a
				// headless run the launcher shows (see the launcher's
				// `runCommandWith` and its direct-output row).
				Route: integration.Manifest.Route(),
				Env:   env,
				// Everything past the descriptor's own list is the host's
				// synthesized editor (see Declared).
				Declared: index < declared,
			}
			entry.Args = append(entry.Args, injection.Args...)
			out = append(out, entry)
		}
	}
	return out
}
