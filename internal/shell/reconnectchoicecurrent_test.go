package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

func checkReconnectChoiceCurrent(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f, program := repairSearchFixture(t, dark)
			second := filepath.Join(f.dir, filepath.Base(program))
			if err := os.Link(program, second); err != nil {
				t.Fatal(err)
			}
			f.entry.ExecutablePath = filepath.Join(f.dir, "gone")
			f.write(t)
			f.a.RefreshIntegrations(context.Background())
			t.Cleanup(extensions.StubToolCandidates(func(...string) []extensions.ExecutableCandidate {
				return []extensions.ExecutableCandidate{{Name: "first", Path: program}, {Name: "second", Path: second}}
			}))
			started, release := make(chan struct{}), make(chan struct{})
			resume := sync.OnceFunc(func() { close(release) })
			defer resume()
			f.a.chooseTool = func(_ string, candidates []extensions.ExecutableCandidate) (string, bool) {
				close(started)
				<-release
				return candidates[1].Path, true
			}
			tt := healthRefreshFrame(t, f, "unused", dark)
			clearNotices(f.a)
			done := repairSearchClick(t, f, tt, false)
			select {
			case <-started:
			case <-time.After(6 * time.Second):
				t.Fatal("runtime chooser did not open")
			}
			switch change {
			case "manifest":
				f.manifest.Runtime.ExecutableNames = []string{"current-tool"}
			case "package":
				f.entry.ManifestPath = filepath.Join(f.dir, "replacement", "floter.extension.json")
			case "binding":
				f.entry.ExecutablePath = program
			case "root":
				root := filepath.Join(f.dir, "current-runtime")
				f.entry.RuntimeRoot = &root
			case "ownership":
				f.entry.RuntimeOwnership = "managed"
			}
			f.write(t)
			if change == "removed" {
				diagnosisWrite(t, f.paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]extensions.Entry{}})
			}
			if change == "package" {
				commandReprobeDescriptor(t, f, "Current package command")
			}
			before, err := os.ReadFile(f.paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			// Only disk changes: the chooser's old cached owner must not be
			// accepted just because no background refresh happened yet.
			resume()
			result := repairSearchWait(t, done)
			after, err := os.ReadFile(f.paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Frame()
			repairSearchDump(t, tt, "choice-"+change, dark)
			if !bytes.Equal(before, after) {
				t.Errorf("obsolete %s choice rewrote current repository", change)
			}
			if result != nil || len(titles) != 0 {
				t.Errorf("obsolete %s choice published a result: err=%v notices=%q", change, result, titles)
			}
			// The callback must release the original busy row. A current
			// action is reachable again when the same tool is still listed.
			if change != "removed" && change != "ownership" && change != "binding" {
				calls := 0
				f.a.Settings.Actions.ReconnectIntegration = func(string, func(error)) { calls++ }
				if err := tt.Click(i18n.For("zh").Settings.IntegrationsReconnect); err != nil {
					t.Error(err)
				}
				tt.Frame()
				if calls != 1 {
					t.Errorf("current %s row lost retry outlet: calls=%d", change, calls)
				}
			}
		})
	}
}

func TestReconnectChoiceCurrentManifest(t *testing.T) { checkReconnectChoiceCurrent(t, "manifest") }
func TestReconnectChoiceCurrentPackage(t *testing.T)  { checkReconnectChoiceCurrent(t, "package") }
func TestReconnectChoiceCurrentBinding(t *testing.T)  { checkReconnectChoiceCurrent(t, "binding") }
func TestReconnectChoiceCurrentRoot(t *testing.T)     { checkReconnectChoiceCurrent(t, "root") }
