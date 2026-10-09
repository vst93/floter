package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syncFixture installs two integrations: one with a script runtime and a
// configuration (including a secret and a device path), one static.
func syncFixture(t *testing.T) (Paths, string) {
	t.Helper()
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	scriptDir := filepath.Join(paths.Extensions, "dev.floter.scripted")
	writeFile(t, filepath.Join(scriptDir, manifestFileName), `{
	  "schemaVersion": "2.0", "id": "dev.floter.scripted", "name": "Scripted", "version": "1.2.0",
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "permissions": ["environment", "filesystem-read"]
	}`)
	writeFile(t, filepath.Join(scriptDir, "tool.sh"), "#!/bin/sh\necho hi\n")

	staticDir := filepath.Join(paths.Extensions, "dev.floter.static")
	writeFile(t, filepath.Join(staticDir, manifestFileName), `{
	  "schemaVersion": "2.0", "id": "dev.floter.static", "name": "Static", "version": "0.5.0",
	  "runtime": {"type": "system", "executableNames": ["sh"]},
	  "provider": {"type": "static-descriptor", "descriptor": "description.json"}
	}`)
	writeFile(t, filepath.Join(staticDir, "description.json"),
		`{"protocolVersion":"1.0","provider":{"id":"dev.floter.static","name":"Static"},"commands":[{"id":"go","name":"Go"}]}`)

	repo := `{"schemaVersion":1,"extensions":{
	  "dev.floter.scripted": {"id":"dev.floter.scripted","name":"Scripted","state":"enabled","enabled":true,
	    "packageVersion":"1.2.0","distributionSource":"local","runtimeOwnership":"bundled",
	    "manifestPath":"` + filepath.Join(scriptDir, manifestFileName) + `","installedAt":1,"updatedAt":1},
	  "dev.floter.static": {"id":"dev.floter.static","name":"Static","state":"disabled","enabled":false,
	    "packageVersion":"0.5.0","distributionSource":"npm","runtimeOwnership":"system",
	    "manifestPath":"` + filepath.Join(staticDir, manifestFileName) + `","installedAt":1,"updatedAt":1}
	}}`
	writeFile(t, paths.RepositoryFile, repo)

	// A configuration with a secret (excluded) and a device path (excluded).
	writeFile(t, filepath.Join(paths.Data, "dev.floter.scripted", "config.json"), `{
	  "configVersion": 3, "secretGeneration": "7",
	  "values": {"endpoint": "https://example.com", "api_token": "[REDACTED]", "workdir": "/Users/me/work", "retries": 3}
	}`)
	writeFile(t, filepath.Join(paths.Data, "dev.floter.scripted", "config-secrets", "7.json"),
		`{"generation":"7","values":{"api_token":"s3cret"}}`)
	return paths, scriptDir
}

func TestExportSyncCarriesConfigWithoutSecrets(t *testing.T) {
	paths, _ := syncFixture(t)
	document, err := ExportSync(paths)
	if err != nil {
		t.Fatal(err)
	}
	if document.Version != SyncFormatVersion || document.ExportedAt == "" {
		t.Errorf("document = %+v", document)
	}
	if len(document.Extensions) != 2 {
		t.Fatalf("extensions = %+v", document.Extensions)
	}
	byID := map[string]SyncEntry{}
	for _, entry := range document.Extensions {
		byID[entry.ID] = entry
	}

	scripted := byID["dev.floter.scripted"]
	if !scripted.Enabled || scripted.Version != "1.2.0" || scripted.DistributionSource != "local" ||
		scripted.RuntimeOwnership != "bundled" {
		t.Errorf("scripted = %+v", scripted)
	}
	// The secret and the device path are gone; the ordinary values are there.
	if _, ok := scripted.Config["api_token"]; ok {
		t.Errorf("a secret was exported: %v", scripted.Config)
	}
	if _, ok := scripted.Config["workdir"]; ok {
		t.Errorf("a device path was exported: %v", scripted.Config)
	}
	if scripted.Config["endpoint"] != "https://example.com" || scripted.Config["retries"] != float64(3) {
		t.Errorf("config = %v", scripted.Config)
	}
	categories := map[string]FieldMetadata{}
	for _, meta := range scripted.FieldMetadata {
		categories[meta.Key] = meta
	}
	if categories["api_token"].Category != CategorySecret || !categories["api_token"].Excluded {
		t.Errorf("token metadata = %+v", categories["api_token"])
	}
	if categories["workdir"].Category != CategoryDevicePath || !categories["workdir"].Excluded {
		t.Errorf("path metadata = %+v", categories["workdir"])
	}
	if categories["endpoint"].Category != CategoryNormal || categories["endpoint"].Excluded {
		t.Errorf("endpoint metadata = %+v", categories["endpoint"])
	}
	// The package's essentials travel: the manifest and the script.
	if len(scripted.Manifest) == 0 || scripted.ScriptContent == "" {
		t.Errorf("the package was not carried: %+v", scripted)
	}
	if !strings.Contains(string(scripted.Manifest), "dev.floter.scripted") {
		t.Errorf("manifest = %s", scripted.Manifest)
	}

	static := byID["dev.floter.static"]
	if static.Enabled || static.DistributionSource != "npm" || len(static.ProviderDescriptor) == 0 {
		t.Errorf("static = %+v", static)
	}
}

func TestImportSyncInstallsAndConfigures(t *testing.T) {
	paths, _ := syncFixture(t)
	document, err := ExportSync(paths)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := document.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadSync(encoded)
	if err != nil {
		t.Fatal(err)
	}

	// A fresh machine: neither integration is installed.
	fresh := FromRoot(t.TempDir())
	if err := fresh.Ensure(); err != nil {
		t.Fatal(err)
	}
	asked := 0
	report, err := ImportSync(fresh, read, func(approvals []PermissionApproval) bool {
		asked++
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Succeeded) != 2 || len(report.Failed) != 0 || len(report.Skipped) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if asked > 1 {
		t.Errorf("the user was asked %d times, want at most one", asked)
	}
	inventory := LoadInventory(fresh)
	if len(inventory.Integrations) != 2 {
		t.Fatalf("installed = %+v", inventory.Integrations)
	}
	byID := map[string]Integration{}
	for _, integration := range inventory.Integrations {
		byID[integration.Entry.ID] = integration
	}
	scripted, ok := byID["dev.floter.scripted"]
	if !ok || !scripted.Entry.Enabled {
		t.Fatalf("scripted = %+v", scripted)
	}
	// The script arrived, and the configuration with it.
	if _, err := os.Stat(filepath.Join(fresh.Extensions, "dev.floter.scripted", "tool.sh")); err != nil {
		t.Errorf("the script is missing: %v", err)
	}
	stored, err := LoadStoredConfiguration(fresh.Data, "dev.floter.scripted")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Values["endpoint"] != "https://example.com" {
		t.Errorf("config = %v", stored.Values)
	}
	if _, ok := stored.Values["api_token"]; ok {
		t.Errorf("a secret was imported: %v", stored.Values)
	}
	static := byID["dev.floter.static"]
	if static.Entry.Enabled {
		t.Errorf("the disabled integration arrived enabled: %+v", static.Entry)
	}
	if _, err := os.Stat(filepath.Join(fresh.Extensions, "dev.floter.static", "description.json")); err != nil {
		t.Errorf("the descriptor is missing: %v", err)
	}
}

// Importing onto a machine that already has the integration keeps that
// machine's secrets and only takes what the document carries.
func TestImportSyncMergesIntoAnInstalledIntegration(t *testing.T) {
	paths, _ := syncFixture(t)
	document, err := ExportSync(paths)
	if err != nil {
		t.Fatal(err)
	}
	// The machine's own secret stays where it is.
	if _, err := ImportSync(paths, document, nil); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadStoredConfiguration(paths.Data, "dev.floter.scripted")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Values["api_token"] != "s3cret" {
		t.Errorf("the local secret was lost: %v", stored.Values)
	}
	if stored.Values["endpoint"] != "https://example.com" {
		t.Errorf("config = %v", stored.Values)
	}
	// The secret generation is untouched, so the loader still finds it.
	if stored.SecretGeneration != "7" {
		t.Errorf("secret generation = %q", stored.SecretGeneration)
	}
}

func TestImportSyncSkipsWhatItCannotInstall(t *testing.T) {
	fresh := FromRoot(t.TempDir())
	if err := fresh.Ensure(); err != nil {
		t.Fatal(err)
	}
	// A document from the old build that carries no package.
	document := SyncDocument{Version: 2, Extensions: []SyncEntry{
		{ID: "dev.floter.gone", Version: "1.0.0", Enabled: true, DistributionSource: "local", RuntimeOwnership: "system"},
	}}
	report, err := ImportSync(fresh, document, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Skipped) != 1 || len(report.Succeeded) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(report.Skipped[0].Reason, "not installed here") {
		t.Errorf("reason = %q", report.Skipped[0].Reason)
	}

	// A package whose manifest names another id is a failure, not a skip.
	bad := SyncDocument{Version: 2, Extensions: []SyncEntry{{
		ID: "dev.floter.mismatch", Enabled: true,
		Manifest: json.RawMessage(`{"id":"dev.floter.other","name":"Other","runtime":{"type":"system","executableNames":["sh"]},"provider":{"type":"executable"}}`),
	}}}
	report, err = ImportSync(fresh, bad, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 1 || !strings.Contains(report.Failed[0].Reason, "declares") {
		t.Fatalf("report = %+v", report)
	}
}

func TestReadSyncRejectsBadDocuments(t *testing.T) {
	cases := map[string]string{
		"not json":   `{`,
		"no version": `{"extensions":[]}`,
		"future":     `{"version":99,"extensions":[]}`,
		"invalid id": `{"version":2,"extensions":[{"id":"../escape"}]}`,
		"huge":       `{"version":2,"extensions":[]}` + strings.Repeat(" ", maxSyncBytes),
	}
	for name, text := range cases {
		if _, err := ReadSync([]byte(text)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestClassifyConfigField(t *testing.T) {
	cases := map[string]string{
		"api_token":          CategorySecret,
		"PASSWORD":           CategorySecret,
		"private_key":        CategorySecret,
		"key":                CategorySecret,
		"signing_key":        CategorySecret,
		"endpoint":           CategoryNormal,
		"retries":            CategoryNormal,
		"workdir":            CategoryDevicePath,
		"home":               CategoryDevicePath,
		"version_constraint": CategoryVersionConstraint,
	}
	for key, want := range cases {
		value := any("value")
		if key == "workdir" || key == "home" {
			value = "/Users/me/work"
		}
		if got := ClassifyConfigField(key, value); got != want {
			t.Errorf("ClassifyConfigField(%q) = %q, want %q", key, got, want)
		}
	}
	// A Windows path is device-specific too.
	if got := ClassifyConfigField("dir", `C:\Users\me`); got != CategoryDevicePath {
		t.Errorf("windows path = %q", got)
	}
	if got := ClassifyConfigField("dir", "~/.config"); got != CategoryDevicePath {
		t.Errorf("home path = %q", got)
	}
}

// Everything the import would install is asked about once, and a refusal
// installs nothing.
func TestImportSyncAsksOnceAndRefusalInstallsNothing(t *testing.T) {
	paths, _ := syncFixture(t)
	document, err := ExportSync(paths)
	if err != nil {
		t.Fatal(err)
	}
	fresh := FromRoot(t.TempDir())
	if err := fresh.Ensure(); err != nil {
		t.Fatal(err)
	}
	asked := 0
	report, err := ImportSync(fresh, document, func(approvals []PermissionApproval) bool {
		asked++
		if len(approvals) == 0 {
			t.Error("a question was asked with nothing to approve")
		}
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if asked != 1 {
		t.Errorf("the user was asked %d times, want one", asked)
	}
	// The question carried the permissions the document's packages declare.
	if asked == 1 && len(report.Skipped) != 2 {
		t.Errorf("report = %+v", report)
	}
	if len(report.Succeeded) != 0 || len(report.Skipped) != 2 {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(report.Skipped[0].Reason, "not approved") {
		t.Errorf("reason = %q", report.Skipped[0].Reason)
	}
	// Nothing landed: neither package directory nor repository entry.
	if inventory := LoadInventory(fresh); len(inventory.Integrations) != 0 {
		t.Errorf("a refused import installed %+v", inventory.Integrations)
	}
	entries, err := os.ReadDir(fresh.Extensions)
	if err == nil {
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".") {
				t.Errorf("a package directory was left behind: %s", entry.Name())
			}
		}
	}
}
