package shell

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func adoptionFixture(t *testing.T, dark bool) *diagnosisFixture {
	t.Helper()
	f, program := repairSearchFixture(t, dark)
	f.manifest.Permissions = []string{extensions.PermissionEnvironment}
	f.manifest.Runtime.ExecutableNames = []string{program}
	f.write(t)
	diagnosisWrite(t, f.paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]extensions.Entry{}})
	t.Cleanup(extensions.StubToolLookup(func(...string) (string, bool) { return program, true }))
	f.a.RefreshIntegrations(context.Background())
	f.a.Settings.Recommended = nil
	clearNotices(f.a)
	return f
}

func adoptionWait(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(6 * time.Second):
		t.Fatal("orphan operation did not reach its lifecycle boundary")
	}
}

func adoptionDump(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JV_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jv-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func adoptionClick(t *testing.T, f *diagnosisFixture, tt *ui.Tester, deleting bool) {
	t.Helper()
	run, label := f.a.Settings.Actions.AdoptIntegration, i18n.For("zh").Settings.IntegrationsAdopt
	if deleting {
		run, label = f.a.Settings.Actions.DeleteOrphan, i18n.For("zh").Settings.IntegrationsDeleteOrphan
	}
	target := ""
	if deleting {
		f.a.Settings.Actions.DeleteOrphan = func(id string) { target = id }
	} else {
		f.a.Settings.Actions.AdoptIntegration = func(id string) { target = id }
	}
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if deleting {
		f.a.Settings.Actions.DeleteOrphan = run
	} else {
		f.a.Settings.Actions.AdoptIntegration = run
	}
	if target != f.entry.ID {
		adoptionDump(t, tt, "click-debug", false)
		t.Fatalf("orphan click dispatched to %q; texts=%q", target, tt.Texts())
	}
	run(target)
}

func checkAdoptionCurrent(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := adoptionFixture(t, dark)
			started, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{}, 8)
			resume := sync.OnceFunc(func() { close(release) })
			defer resume()
			var confirmations atomic.Int32
			f.a.confirmPermissions = func(extensions.PermissionApproval) bool {
				if confirmations.Add(1) == 1 {
					close(started)
				}
				<-release
				return change != "cancel"
			}
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			tt := healthRefreshFrame(t, f, "unused", dark)
			adoptionClick(t, f, tt, false)
			adoptionWait(t, started)
			adoptionWait(t, settled)
			// A duplicate host dispatch must not open another permission dialog.
			f.a.Settings.Actions.AdoptIntegration(f.entry.ID)
			switch change {
			case "permissions":
				f.manifest.Permissions = append(f.manifest.Permissions, extensions.PermissionClipboardRead)
				diagnosisWrite(t, f.entry.ManifestPath, f.manifest)
			case "package":
				current := f.manifest
				current.Name = "当前工具"
				diagnosisWrite(t, filepath.Join(filepath.Dir(f.entry.ManifestPath), "current.json"), current)
				diagnosisWrite(t, filepath.Join(filepath.Dir(f.entry.ManifestPath), "package.json"), map[string]any{"name": "current-package", "version": "2.0.0", "floter": map[string]string{"manifest": "current.json"}})
			case "record":
				f.entry.Name, f.entry.Enabled, f.entry.State = "已连接工具", false, "disabled"
				f.write(t)
			case "removed":
				if err := os.RemoveAll(filepath.Dir(f.entry.ManifestPath)); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(f.paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			resume()
			adoptionWait(t, settled)
			f.a.Settings.Actions.Invalidate = nil
			after, err := os.ReadFile(f.paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			adoptionDump(t, tt, change, dark)
			if !bytes.Equal(before, after) {
				t.Errorf("obsolete %s adoption rewrote current repository", change)
			}
			if len(titles) != 0 {
				t.Errorf("obsolete %s adoption published a result: %q", change, titles)
			}
			if confirmations.Load() != 1 {
				t.Errorf("adoption opened %d permission dialogs", confirmations.Load())
			}
			rows := f.a.integrationList()
			if change == "removed" {
				if len(rows) != 0 {
					t.Errorf("removed package remains on screen: %+v", rows)
				}
			} else if change == "record" {
				if len(rows) != 1 || rows[0].Orphan || rows[0].Enabled {
					t.Errorf("current installed row was not refreshed: %+v", rows)
				}
			} else {
				calls := 0
				f.a.Settings.Actions.AdoptIntegration = func(string) { calls++ }
				if err := tt.Click(i18n.For("zh").Settings.IntegrationsAdopt); err != nil {
					t.Error(err)
				}
				tt.Frame()
				if calls != 1 {
					t.Errorf("current %s orphan lost retry outlet: calls=%d", change, calls)
				}
			}
		})
	}
}

func TestAdoptionCurrentPermissions(t *testing.T) { checkAdoptionCurrent(t, "permissions") }
func TestAdoptionCurrentPackage(t *testing.T)     { checkAdoptionCurrent(t, "package") }
func TestAdoptionCurrentRecord(t *testing.T)      { checkAdoptionCurrent(t, "record") }
func TestAdoptionCurrentRemoved(t *testing.T)     { checkAdoptionCurrent(t, "removed") }
func TestAdoptionCancel(t *testing.T)             { checkAdoptionCurrent(t, "cancel") }

func TestAdoptionDeleteFailure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := adoptionFixture(t, dark)
			tt := healthRefreshFrame(t, f, "unused", dark)
			settled := make(chan struct{}, 8)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			if err := os.WriteFile(f.paths.RepositoryFile, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
			adoptionClick(t, f, tt, true)
			adoptionWait(t, settled)
			adoptionWait(t, settled)
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			adoptionDump(t, tt, "delete-failure", dark)
			want := i18n.NotificationsFor("zh").IntegrationRemoveFailed(f.entry.ID)
			if len(titles) != 1 || titles[0] != want {
				t.Errorf("orphan deletion failure lost result: got=%q want=%q", titles, want)
			}
			if _, err := os.Stat(filepath.Dir(f.entry.ManifestPath)); err != nil {
				t.Fatal("failed deletion lost package:", err)
			}
			diagnosisWrite(t, f.paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]extensions.Entry{}})
			adoptionClick(t, f, tt, true)
			adoptionWait(t, settled)
			adoptionWait(t, settled)
			f.a.Settings.Actions.Invalidate = nil
			if _, err := os.Stat(filepath.Dir(f.entry.ManifestPath)); !os.IsNotExist(err) {
				t.Errorf("retry did not delete package: %v", err)
			}
			if len(f.a.integrationList()) != 0 {
				t.Error("successful retry left an orphan row")
			}
		})
	}
}
