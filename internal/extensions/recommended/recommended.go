package recommended

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"floter/internal/extensions"
)

// The tools floter ships a package for: a manifest and a provider descriptor,
// which connecting materializes into the integrations directory and installs
// through the same pipeline as any other local tool.
//
// They are ordinary packages, not a special kind: the embedded bytes are
// written out and then handed to PrepareLocal, so a recommended tool is
// validated, permission-gated and recorded exactly like one the user built
// themselves.

//go:embed v-tools/floter.extension.json v-tools/provider-description.json
var packages embed.FS

// Tool is one shipped package.
type Tool struct {
	// ID is the manifest's id.
	ID string
	// Name is the manifest's name.
	Name string
	// Description is the manifest's one line.
	Description string
	// Dir is where the package's files live inside this package, for
	// materializing it.
	Dir string
}

// Tools are the shipped packages, in the order a settings page lists them.
var Tools = []Tool{{
	ID:          "io.github.vst93.v",
	Name:        "V Tools",
	Description: "Developer tools running in the terminal",
	Dir:         "v-tools",
}}

// WithID finds a shipped package.
func WithID(id string) (Tool, bool) {
	for _, tool := range Tools {
		if tool.ID == id {
			return tool, true
		}
	}
	return Tool{}, false
}

// Materialize writes a shipped package into a directory, so the ordinary
// install pipeline can take it from there.
func (t Tool) Materialize(dir string) error {
	entries, err := packages.ReadDir(t.Dir)
	if err != nil {
		return fmt.Errorf("extensions: reading the %s package: %w", t.Name, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := packages.ReadFile(filepath.Join(t.Dir, entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Prepare stages a shipped package for install: its files are written into a
// temporary directory, and the caller commits the result with the user's
// answer about its permissions.
func Prepare(paths extensions.Paths, id string) (extensions.Prepared, func(), error) {
	tool, ok := WithID(id)
	if !ok {
		return extensions.Prepared{}, nil, errors.New("extensions: no such recommended tool")
	}
	dir, err := os.MkdirTemp("", "floter-recommended-")
	if err != nil {
		return extensions.Prepared{}, nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	if err := tool.Materialize(dir); err != nil {
		cleanup()
		return extensions.Prepared{}, nil, err
	}
	prepared, err := extensions.PrepareLocal(paths, dir)
	if err != nil {
		cleanup()
		return extensions.Prepared{}, nil, err
	}
	return prepared, cleanup, nil
}

// Installed reports which shipped packages the inventory already has.
func Installed(inventory extensions.Inventory) map[string]bool {
	installed := map[string]bool{}
	for _, integration := range inventory.Integrations {
		installed[integration.Entry.ID] = true
	}
	return installed
}
