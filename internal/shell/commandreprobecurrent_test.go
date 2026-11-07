package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/i18n"
)

func checkCommandReprobeCurrent(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := commandReprobeFixture(t, dark)
			tt := healthRefreshFrame(t, f, "unused", dark)
			clearNotices(f.a)
			settled := make(chan struct{}, 16)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			if change == "repository-start" {
				if err := os.WriteFile(f.paths.RepositoryFile, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			commandReprobeClick(t, f, tt)
			if change != "repository-start" {
				commandReprobeStarted(t, f)
			}
			switch change {
			case "manifest":
				f.manifest.Description = "当前工具声明"
				f.write(t)
			case "descriptor":
				commandReprobeDescriptor(t, f, "Current command")
			case "publisher":
				f.entry.PublisherID = "publisher"
				f.write(t)
			case "runtime":
				f.entry.ExecutablePath = filepath.Join(t.TempDir(), "replacement-tool")
				f.write(t)
			case "repository-completion":
				if err := os.WriteFile(f.paths.RepositoryFile, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(filepath.Dir(f.entry.ManifestPath), "provider-description.json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			commandReprobeRelease(t, f)
			for i := 0; i < 2; i++ {
				select {
				case <-settled:
				case <-time.After(5 * time.Second):
					t.Fatal("current command reprobe did not settle")
				}
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			toast := queuedToast(f.a, "floter-notice")
			tt.Frame()
			tt.Frame()
			dumpCommandReprobe(t, tt, change, dark)
			if !bytes.Equal(before, after) {
				t.Errorf("late command reprobe overwrote current %s descriptor", change)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), "help-probe.json")); !os.IsNotExist(err) {
				t.Errorf("obsolete %s probe wrote a freshness record", change)
			}
			if change == "repository-start" || change == "repository-completion" {
				want := i18n.NotificationsFor("zh").IntegrationCheckFailed(f.manifest.Name)
				if toast.Type != "error" || toast.Title != want || !tt.HasText(want) {
					t.Errorf("command reprobe %s read failure is silent: %+v", change, toast)
				}
				f.write(t)
				f.a.RefreshIntegrations(context.Background())
				tt.Frame()
				tt.Frame()
				commandReprobeClick(t, f, tt)
				for i := 0; i < 2; i++ {
					select {
					case <-settled:
					case <-time.After(5 * time.Second):
						t.Fatal("restored repository could not retry command reprobe")
					}
				}
				if len(f.a.launcherCommands()) != 2 {
					t.Error("restored repository did not refresh commands")
				}
			} else {
				if toast.Title != "" {
					t.Errorf("obsolete probe raised a notice: %+v", toast)
				}
				rows := f.a.integrationList()
				if len(rows) != 1 || rows[0].DiagnosisFailed {
					t.Errorf("obsolete probe left a failure on its replacement: %+v", rows)
				}
			}
		})
	}
}

func TestCommandReprobeKeepsCurrentManifest(t *testing.T) { checkCommandReprobeCurrent(t, "manifest") }
func TestCommandReprobeKeepsCurrentDescriptor(t *testing.T) {
	checkCommandReprobeCurrent(t, "descriptor")
}
