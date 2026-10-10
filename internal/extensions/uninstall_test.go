package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func uninstallComponentsFixture(t *testing.T) (Paths, string) {
	t.Helper()
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, "dev.floter.parts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.parts", "name": "Parts",
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.parts": {"id": "dev.floter.parts", "name": "Parts",
	    "state": "enabled", "enabled": true, "manifestPath": "`+filepath.Join(dir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(paths.Data, "dev.floter.parts")
	for _, sub := range []string{"sessions", "completions", "artifacts", "logs"} {
		if err := os.MkdirAll(filepath.Join(data, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(data, "config.json"), []byte(`{"values":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(data, "config-secrets", "1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "health.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths, data
}

func TestUninstallComponentizedKeepsData(t *testing.T) {
	paths, data := uninstallComponentsFixture(t)
	result, err := UninstallComponentized(paths, "dev.floter.parts", UninstallComponents{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovedProgram {
		t.Errorf("program = %+v", result)
	}
	if result.RemovedHostConfig || result.RemovedToolData || result.RemovedArtifacts {
		t.Errorf("data was removed: %+v", result)
	}
	if _, err := os.Stat(data); err != nil {
		t.Errorf("the data root was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "config.json")); err != nil {
		t.Errorf("the configuration was removed: %v", err)
	}
	// The repository entry is gone.
	if inventory := LoadInventory(paths); len(inventory.Integrations) != 0 {
		t.Errorf("the entry survived: %+v", inventory.Integrations)
	}
}

func TestUninstallComponentizedAllRemovesTheDataRoot(t *testing.T) {
	paths, data := uninstallComponentsFixture(t)
	result, err := UninstallComponentized(paths, "dev.floter.parts", UninstallComponents{
		RemoveHostConfig: true, RemoveToolData: true, RemoveArtifacts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovedHostConfig || !result.RemovedToolData || !result.RemovedArtifacts {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(data); !os.IsNotExist(err) {
		t.Errorf("the data root survived: %v", err)
	}
}

func TestUninstallComponentizedIndividual(t *testing.T) {
	paths, data := uninstallComponentsFixture(t)
	result, err := UninstallComponentized(paths, "dev.floter.parts", UninstallComponents{
		RemoveHostConfig: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovedHostConfig || result.RemovedToolData || result.RemovedArtifacts {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(data, "config.json")); !os.IsNotExist(err) {
		t.Errorf("the configuration survived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "sessions")); err != nil {
		t.Errorf("the tool data was removed: %v", err)
	}
	_ = strings.TrimSpace("")
}

func TestUninstallComponentizedRefusesBadIDs(t *testing.T) {
	paths := FromRoot(t.TempDir())
	if _, err := UninstallComponentized(paths, "../escape", UninstallComponents{}); err == nil {
		t.Error("an invalid id was accepted")
	}
	if _, err := UninstallComponentized(paths, "dev.floter.gone", UninstallComponents{}); err != ErrNoIntegration {
		t.Errorf("an unknown id = %v", err)
	}
}
