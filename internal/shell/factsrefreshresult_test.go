package shell

import (
	"context"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func checkFactsRefreshResult(t *testing.T, digest bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			const id = "dev.floter.configured"
			repo, err := extensions.LoadRepository(paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			entry := repo.Extensions[id]
			current, err := extensions.ManifestDigest(entry.ManifestPath)
			if err != nil {
				t.Fatal(err)
			}
			entry.ApprovedAt = 1
			entry.ApprovedManifestDigest = &current
			repo.Extensions[id] = entry
			if err := extensions.SaveRepository(paths.RepositoryFile, repo); err != nil {
				t.Fatal(err)
			}
			a.RefreshIntegrations(context.Background())
			a.Settings = settingsui.New(a.Store, settingsui.Actions{Freshness: a.integrationFreshness})
			a.Settings.Integrations = a.integrationList
			tt := transferCauseFrame(t, a, dark)
			tt = ui.NewTester(a.View, 960, 1400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			items := a.Integrations.Inventory().Integrations
			old := computeIntegrationFacts(items)
			fact := old[id]
			// Inject the result of an earlier failed runtime probe or an earlier
			// manifest read, then exercise the real refresh and settings projection.
			name := "runtime-late-result"
			if digest {
				fact.digest = "obsolete-manifest-digest"
				name = "approval-late-result"
			} else {
				fact.available = false
			}
			old[id] = fact
			entered, release, landed := make(chan struct{}), make(chan struct{}), make(chan struct{})
			released := false
			defer func() {
				if !released {
					close(release)
					refreshResultWait(t, landed)
				}
			}()
			_, computed, _ := integrationFactsMemo.snapshot()
			refreshIntegrationFacts(&integrationFactsMemo, items, func([]extensions.Integration) map[string]integrationFact { close(entered); <-release; return old }, func() { close(landed) }, computed.Add(integrationFactsTTL+time.Millisecond))
			refreshResultWait(t, entered)
			a.RefreshIntegrations(context.Background())
			tt.Frame()
			tt.Frame()
			close(release)
			released = true
			refreshResultWait(t, landed)
			tt.Frame()
			tt.Frame()
			refreshResultDump(t, tt, name, dark)
			in := a.integrationList()[0]
			if digest {
				if in.Approval == nil || in.Approval.CurrentDigest != current {
					t.Errorf("retired digest revived approval warning: %+v", in.Approval)
				}
			} else if !in.RuntimeAvailable {
				t.Error("retired runtime failure disabled a repaired integration")
			}
		})
	}
}

func TestFactsRefreshOldRuntimeCannotUndoRepair(t *testing.T)  { checkFactsRefreshResult(t, false) }
func TestFactsRefreshOldDigestCannotUndoApproval(t *testing.T) { checkFactsRefreshResult(t, true) }
