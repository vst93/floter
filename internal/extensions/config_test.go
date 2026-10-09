package extensions

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadStoredConfiguration(t *testing.T) {
	root := t.TempDir()
	id := "io.github.vst93.v"
	dir := filepath.Join(root, id)

	// A missing file is an empty configuration.
	stored, err := LoadStoredConfiguration(root, "nothing.here")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Values) != 0 || stored.ConfigVersion != 0 {
		t.Errorf("missing file = %+v", stored)
	}

	// The current envelope.
	writeFile(t, filepath.Join(dir, configFile), `{
  "configVersion": 2,
  "secretGeneration": "gen-1",
  "values": {"endpoint": "https://example.com", "mode": "pretty", "verbose": true},
  "schema": [{"key": "endpoint", "type": "text"}, {"key": "token", "type": "password"}]
}`)
	writeFile(t, filepath.Join(dir, secretsGenerationDir, "gen-1.json"),
		`{"generation": "gen-1", "values": {"token": "s3cret"}}`)

	stored, err = LoadStoredConfiguration(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ConfigVersion != 2 || stored.SecretGeneration != "gen-1" {
		t.Errorf("envelope = %+v", stored)
	}
	if value, _ := stored.Value("endpoint"); value != "https://example.com" {
		t.Errorf("endpoint = %v", value)
	}
	if value, _ := stored.Value("token"); value != "s3cret" {
		t.Errorf("the secret did not merge: %v", value)
	}
	if len(stored.Schema) != 2 {
		t.Errorf("schema = %+v", stored.Schema)
	}

	// A generation the secrets file does not match yields no secrets, and
	// the placeholder is not a value.
	writeFile(t, filepath.Join(dir, configFile), `{
  "configVersion": 2,
  "secretGeneration": "gen-2",
  "values": {"token": "[REDACTED]"},
  "schema": [{"key": "token", "type": "password"}]
}`)
	stored, err = LoadStoredConfiguration(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stored.Value("token"); ok {
		t.Errorf("a password placeholder was kept: %v", stored.Values)
	}

	// The legacy shapes: the file is the values map, and the secrets live
	// beside it.
	writeFile(t, filepath.Join(dir, configFile), `{"legacyKey": "legacyValue"}`)
	if err := os.Remove(filepath.Join(dir, secretsGenerationDir, "gen-1.json")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, legacySecretsFile), `{"token": "legacy"}`)
	stored, err = LoadStoredConfiguration(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := stored.Value("legacyKey"); value != "legacyValue" {
		t.Errorf("legacy values = %+v", stored.Values)
	}
	if value, _ := stored.Value("token"); value != "legacy" {
		t.Errorf("legacy secrets = %+v", stored.Values)
	}
	if stored.ConfigVersion != 0 || stored.SecretGeneration != "" {
		t.Errorf("legacy envelope = %+v", stored)
	}

	// A malformed file is an error, not silent emptiness.
	writeFile(t, filepath.Join(dir, configFile), "{not json")
	if _, err := LoadStoredConfiguration(root, id); err == nil {
		t.Error("a malformed configuration loaded")
	}

	// An id that would escape the data root is refused.
	if _, err := LoadStoredConfiguration(root, "../escape"); err == nil {
		t.Error("an escaping id loaded")
	}
}

func TestInject(t *testing.T) {
	descriptor := &Configuration{
		Owner:              "host",
		EnvironmentMapping: map[string]string{"endpoint": "TOOL_ENDPOINT"},
		Schema: []ConfigField{
			{Key: "endpoint", Type: "text"},
			{Key: "mode", Type: "select", Argument: "--mode"},
			{Key: "verbose", Type: "boolean", Argument: "--verbose", EnvVar: "TOOL_VERBOSE"},
			{Key: "quiet", Type: "boolean", Argument: "--quiet"},
			{Key: "dirs", Type: "multi-select", Argument: "--dir"},
			{Key: "token", Type: "password", Argument: "--token", EnvVar: "TOOL_TOKEN"},
			{Key: "unset", Type: "text", Argument: "--unset", EnvVar: "TOOL_UNSET"},
		},
	}
	values := map[string]any{
		"endpoint": "https://example.com",
		"mode":     "pretty",
		"verbose":  true,
		"quiet":    false,
		"dirs":     []any{"/a", "/b"},
		"token":    "s3cret",
	}

	got := Inject(descriptor, values, Injection{Env: []string{"PRESERVED=1"}, Args: []string{"base"}})

	wantEnv := []string{"PRESERVED=1", "TOOL_ENDPOINT=https://example.com", "TOOL_TOKEN=s3cret", "TOOL_VERBOSE=true"}
	if !reflect.DeepEqual(got.Env, wantEnv) {
		t.Errorf("env = %v, want %v", got.Env, wantEnv)
	}
	wantArgs := []string{"base", "--mode", "pretty", "--verbose", "--dir", "/a,/b"}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Errorf("args = %v, want %v", got.Args, wantArgs)
	}
	// A password never reaches argv, and a false flag is not passed.
	for i, arg := range got.Args {
		if arg == "--token" || arg == "s3cret" {
			t.Errorf("the password reached argv: %v", got.Args)
		}
		if i > 0 && got.Args[i-1] == "--quiet" {
			t.Errorf("a false flag was passed: %v", got.Args)
		}
	}

	// No descriptor: nothing is added.
	if got := Inject(nil, values, Injection{Env: []string{"A=1"}}); !reflect.DeepEqual(got.Env, []string{"A=1"}) {
		t.Errorf("nil descriptor added %v", got.Env)
	}
}

func TestConfigurationCommand(t *testing.T) {
	integration := Integration{Entry: Entry{ID: "a.tool"}}
	description := Description{
		Configuration: &Configuration{Owner: "tool", OpenCommand: []string{"config", "edit"}},
	}
	command, ok := ConfigurationCommand(integration, description)
	if !ok {
		t.Fatal("a tool-owned configuration yielded no command")
	}
	if command.ID != "configuration" || !reflect.DeepEqual(command.Execution.ArgsPrefix, []string{"config", "edit"}) {
		t.Errorf("command = %+v", command)
	}

	for _, config := range []*Configuration{
		nil,
		{Owner: "host"},
		{Owner: "tool"},
	} {
		if _, ok := ConfigurationCommand(integration, Description{Configuration: config}); ok {
			t.Errorf("configuration %+v yielded a command", config)
		}
	}
}

func TestRenderValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
		ok   bool
	}{
		{"text", "text", true},
		{true, "true", true},
		{float64(42), "42", true},
		{2.5, "2.5", true},
		{[]any{"a", "b"}, "a,b", true},
		{[]any{"a", 1}, "", false},
		{map[string]any{}, "", false},
	}
	for _, tc := range cases {
		got, ok := renderValue(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("renderValue(%v) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
