package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationExportTextIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		field  ConfigField
		value  any
		refuse bool
	}{
		{"text", ConfigField{Key: "title", Type: "text"}, "bad\xff", true},
		{"path", ConfigField{Key: "folder", Type: "path"}, "bad\xff", true},
		{"multiselect", ConfigField{Key: "items", Type: "multiselect", Options: []any{"bad\xff"}}, []any{"bad\xff"}, true},
		{"key", ConfigField{Key: "bad\xff", Type: "boolean"}, true, true},
		{"redacted", ConfigField{Key: "token", Type: "password"}, "bad\xff", false},
		{"unicode", ConfigField{Key: "title", Type: "text"}, "项目 � 😀", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := ExportConfiguration("dev.text", 1, []ConfigField{tc.field}, map[string]any{tc.field.Key: tc.value})
			if tc.refuse {
				if err == nil || !strings.Contains(err.Error(), "UTF-8") || len(data) != 0 {
					t.Fatalf("damaged public text was exported: %v", err)
				}
				return
			}
			var doc ConfigurationExportDocument
			if err != nil || json.Unmarshal(data, &doc) != nil {
				t.Fatalf("valid public payload refused: %v", err)
			}
			want := tc.value
			if tc.name == "redacted" {
				want = PasswordPlaceholder
			}
			if doc.Values[tc.field.Key] != want {
				t.Error("valid text or password redaction changed")
			}
		})
	}
}

func TestSyncExportLimitIncludesEscapingAndNewline(t *testing.T) {
	doc := SyncDocument{Version: 2, ExportedAt: "2026-11-08T00:00:00Z", Extensions: []SyncEntry{{ID: "dev.text", ScriptContent: "x"}}}
	one, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	doc.Extensions[0].ScriptContent = strings.Repeat("x", maxSyncBytes-len(one))
	data, err := doc.Marshal()
	if err != nil || len(data)+1 != maxSyncBytes {
		t.Fatalf("exactly readable backup refused: size=%d err=%v", len(data)+1, err)
	}
	if _, err := ReadSync(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
	doc.Extensions[0].ScriptContent += "x"
	if data, err := doc.Marshal(); err == nil || len(data) != 0 {
		t.Error("one-byte oversize backup was returned")
	}
	doc.Extensions[0].ScriptContent = strings.Repeat("\t", maxSyncBytes/2)
	if data, err := doc.Marshal(); err == nil || len(data) != 0 {
		t.Error("JSON escaping bypassed the backup limit")
	}
}

func TestBackupPackageCompatibility(t *testing.T) {
	for _, mode := range []string{"no-package", "other-platform", "custom-manifest", "missing-declared"} {
		t.Run(mode, func(t *testing.T) {
			paths := FromRoot(t.TempDir())
			if err := paths.Ensure(); err != nil {
				t.Fatal(err)
			}
			pkg := filepath.Join(paths.Extensions, "dev.text")
			if err := os.MkdirAll(pkg, 0o700); err != nil {
				t.Fatal(err)
			}
			manifestPath := filepath.Join(pkg, manifestFileName)
			if mode == "custom-manifest" {
				manifestPath = filepath.Join(pkg, "custom.json")
			}
			manifest := `{"schemaVersion":"2.0","id":"dev.text","name":"Text","distribution":{"type":"local"},"runtime":{"type":"script","language":"shell","path":"tool.sh"},"provider":{"type":"executable"}}`
			if mode == "other-platform" {
				manifest = strings.Replace(manifest, `"name":"Text"`, `"name":"Text","platforms":["windows-x86_64"]`, 1)
				paths.Platform = Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
			}
			if mode != "no-package" && mode != "missing-declared" {
				if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("echo hello\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			entry := map[string]any{"id": "dev.text", "name": "Text", "enabled": true, "state": "enabled"}
			if mode != "no-package" {
				entry["manifestPath"] = manifestPath
			}
			repo, err := json.Marshal(map[string]any{"schemaVersion": 1, "extensions": map[string]any{"dev.text": entry}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.RepositoryFile, repo, 0o600); err != nil {
				t.Fatal(err)
			}
			doc, err := ExportSync(paths)
			if mode == "missing-declared" {
				if err == nil || !strings.Contains(err.Error(), manifestFileName) {
					t.Fatal("missing declared package was silently omitted")
				}
				return
			}
			if err != nil || len(doc.Extensions) != 1 {
				t.Fatalf("compatible backup failed: %v", err)
			}
			if mode == "no-package" {
				if len(doc.Extensions[0].Manifest) != 0 {
					t.Error("unpackaged record invented a manifest")
				}
			} else if string(doc.Extensions[0].Manifest) != manifest || doc.Extensions[0].ScriptContent != "echo hello\n" {
				t.Error("compatible package was omitted or changed")
			}
		})
	}
}
