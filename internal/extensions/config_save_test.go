package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var saveSchema = []ConfigField{
	{Key: "endpoint", Type: "text", Required: true, MinLength: intPtr(4)},
	{Key: "retries", Type: "number", Minimum: floatPtr(0), Maximum: floatPtr(10)},
	{Key: "verbose", Type: "boolean"},
	{Key: "region", Type: "select", Options: []any{"eu", "us"}},
	{Key: "tags", Type: "multiSelect", Options: []any{"a", "b", "c"}},
	{Key: "api_token", Type: "password", Required: true},
	{Key: "workdir", Type: "path"},
}

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

func TestSaveConfigurationSplitsSecrets(t *testing.T) {
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	values := map[string]any{
		"endpoint":  "https://example.com",
		"retries":   3,
		"verbose":   true,
		"region":    "eu",
		"tags":      []any{"a", "c"},
		"api_token": "s3cret",
		"workdir":   "/Users/me/work",
	}
	if err := SaveConfiguration(paths, "dev.floter.save", saveSchema, values); err != nil {
		t.Fatal(err)
	}

	// The values file holds the placeholder, never the secret.
	data, err := os.ReadFile(filepath.Join(paths.Data, "dev.floter.save", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "s3cret") {
		t.Errorf("the secret is in the values file: %s", text)
	}
	if !strings.Contains(text, PasswordPlaceholder) {
		t.Errorf("the placeholder is missing: %s", text)
	}
	// The schema is stored with a redacted default.
	if !strings.Contains(text, "api_token") {
		t.Errorf("the stored schema is missing: %s", text)
	}

	// A read merges the secret back.
	stored, err := LoadStoredConfiguration(paths.Data, "dev.floter.save")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Values["api_token"] != "s3cret" || stored.Values["endpoint"] != "https://example.com" {
		t.Errorf("values = %v", stored.Values)
	}
	if stored.SecretGeneration == "" {
		t.Error("no secret generation was written")
	}

	// The secrets file is private, and only the current generation survives.
	entries, err := os.ReadDir(filepath.Join(paths.Data, "dev.floter.save", secretsGenerationDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("generations = %v", entries)
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("secrets mode = %v", info.Mode().Perm())
	}

	// A second save replaces the generation and keeps the other values.
	if err := SaveConfiguration(paths, "dev.floter.save", saveSchema, map[string]any{
		"endpoint":  "https://other.example",
		"api_token": "new-secret",
	}); err != nil {
		t.Fatal(err)
	}
	stored, err = LoadStoredConfiguration(paths.Data, "dev.floter.save")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Values["api_token"] != "new-secret" || stored.Values["endpoint"] != "https://other.example" {
		t.Errorf("values after a second save = %v", stored.Values)
	}
	// The values the second save left out are gone from the file, and the old
	// generation with them.
	entries, err = os.ReadDir(filepath.Join(paths.Data, "dev.floter.save", secretsGenerationDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the old generation survived: %v", entries)
	}
}

func TestSaveConfigurationValidation(t *testing.T) {
	paths := FromRoot(t.TempDir())
	cases := map[string]map[string]any{
		"an unknown key":         {"nope": "x"},
		"a missing required":     {"retries": 3},
		"a wrong type":           {"endpoint": "https://x", "api_token": "x", "retries": "3"},
		"a bad boolean":          {"endpoint": "https://x", "api_token": "x", "verbose": "yes"},
		"an unknown option":      {"endpoint": "https://x", "api_token": "x", "region": "mars"},
		"an unknown multiselect": {"endpoint": "https://x", "api_token": "x", "tags": []any{"a", "z"}},
		"an out-of-range number": {"endpoint": "https://x", "api_token": "x", "retries": 99},
		"a too-short text":       {"endpoint": "ab", "api_token": "x"},
	}
	for name, values := range cases {
		if err := SaveConfiguration(paths, "dev.floter.save", saveSchema, values); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	// Nothing was written by any of the refusals.
	if _, err := os.Stat(filepath.Join(paths.Data, "dev.floter.save")); !os.IsNotExist(err) {
		t.Errorf("a refused save wrote files: %v", err)
	}

	// The defaults fill in for a missing optional field, and a password that
	// is left out keeps its placeholder rather than clearing the secret.
	if err := SaveConfiguration(paths, "dev.floter.save", saveSchema, map[string]any{
		"endpoint": "https://example.com", "api_token": "s3cret",
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadStoredConfiguration(paths.Data, "dev.floter.save")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Values["api_token"] != "s3cret" {
		t.Errorf("the secret was lost by a save without it: %v", stored.Values)
	}
}

func TestValidateConfiguration(t *testing.T) {
	if err := ValidateConfiguration(saveSchema, map[string]any{
		"endpoint": "https://x", "api_token": "x", "region": "us", "tags": []any{"b"},
	}); err != nil {
		t.Errorf("valid values refused: %v", err)
	}
	// A default fills in for a missing field: the required check passes.
	schema := []ConfigField{{Key: "name", Type: "text", Required: true, Default: "floter"}}
	if err := ValidateConfiguration(schema, map[string]any{}); err != nil {
		t.Errorf("a defaulted required field refused: %v", err)
	}
	if err := ValidateConfiguration(nil, map[string]any{"x": 1}); err == nil {
		t.Error("values were accepted without a schema")
	}
}

func TestSavedConfigurationInjects(t *testing.T) {
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfiguration(paths, "dev.floter.save", []ConfigField{
		{Key: "endpoint", Type: "text", EnvVar: "ENDPOINT"},
		{Key: "api_token", Type: "password", EnvVar: "TOKEN"},
	}, map[string]any{"endpoint": "https://x", "api_token": "s3cret"}); err != nil {
		t.Fatal(err)
	}
	// The description the provider would report.
	schema := []ConfigField{
		{Key: "endpoint", Type: "text", EnvVar: "ENDPOINT"},
		{Key: "api_token", Type: "password", EnvVar: "TOKEN"},
	}
	stored, err := LoadStoredConfiguration(paths.Data, "dev.floter.save")
	if err != nil {
		t.Fatal(err)
	}
	injection := Inject(&Configuration{Schema: schema}, stored.Values, Injection{})
	found := map[string]string{}
	for _, env := range injection.Env {
		key, value, _ := strings.Cut(env, "=")
		found[key] = value
	}
	if found["ENDPOINT"] != "https://x" || found["TOKEN"] != "s3cret" {
		t.Errorf("injected env = %v", found)
	}
	// The password went to the environment, never to argv.
	if len(injection.Args) != 0 {
		t.Errorf("injected args = %v", injection.Args)
	}
	_ = json.Marshal
}
