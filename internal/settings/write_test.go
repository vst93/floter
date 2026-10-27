package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Save keeps the old build's two-file contract: the canonical settings file
// and a recovery copy beside it with the same bytes, both 0600, and neither
// left half-written.
func TestSaveWritesTheRecoveryCopyBesideThePrimary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "floter", "settings.json")

	s, err := Parse([]byte(`{"theme":"dark","language":"zh","custom_key":"kept"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	primary, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read primary: %v", err)
	}
	backup, err := os.ReadFile(path + backupSuffix)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(primary, backup) {
		t.Errorf("backup differs from the primary:\nprimary: %s\nbackup:  %s", primary, backup)
	}
	if !bytes.Contains(primary, []byte(`"custom_key": "kept"`)) {
		t.Errorf("unknown key missing from the saved file: %s", primary)
	}

	if runtime.GOOS != "windows" {
		for _, name := range []string{path, path + backupSuffix} {
			info, err := os.Stat(name)
			if err != nil {
				t.Fatalf("stat %s: %v", name, err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Errorf("%s mode = %o, want 600", filepath.Base(name), got)
			}
		}
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	// P11-k 重定基线（S：结构变了）：临时文件前缀从 `.settings-*.tmp` 换成
	// `statefile.Write` 的 `.floter-state-*.tmp` —— 同一个写入器服务所有状态文件，
	// 名字不再按包分。断言的内容不变：写完之后目录里不许留下写入中的文件。
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".floter-state-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

// Every save refreshes the recovery copy. A primary that is later truncated
// recovers the *latest* settings, not the first save's.
func TestSaveRefreshesTheRecoveryCopyOnEveryWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	first, err := Parse([]byte(`{"theme":"dark"}`))
	if err != nil {
		t.Fatalf("Parse first: %v", err)
	}
	if err := Save(path, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	second, err := Parse([]byte(`{"theme":"light"}`))
	if err != nil {
		t.Fatalf("Parse second: %v", err)
	}
	if err := Save(path, second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	if err := os.WriteFile(path, []byte("{truncated"), 0o644); err != nil {
		t.Fatalf("corrupt primary: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != "light" {
		t.Errorf("recovered theme = %q, want the latest write's light", got.Theme)
	}
}
