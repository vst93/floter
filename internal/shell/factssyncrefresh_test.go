package shell

import (
	"context"
	"strings"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func checkFactsSyncRefresh(t *testing.T, scenario string) {
	t.Helper()
	digest := strings.Contains(scenario, "approval")
	missing := strings.HasPrefix(scenario, "added-")
	neighbour := scenario == "added-neighbour-runtime"
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			if missing {
				syncRefreshAddNeighbour(t, a, paths)
			}
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
			entry.ApprovedAt, entry.ApprovedManifestDigest = 1, &current
			repo.Extensions[id] = entry
			if err := extensions.SaveRepository(paths.RepositoryFile, repo); err != nil {
				t.Fatal(err)
			}
			a.RefreshIntegrations(context.Background())
			a.Settings = settingsui.New(a.Store, settingsui.Actions{Freshness: a.integrationFreshness})
			a.Settings.Integrations = a.integrationList
			transferCauseFrame(t, a, dark)
			tt := ui.NewTester(a.View, 960, 1400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			items := a.Integrations.Inventory().Integrations
			invalidateIntegrationFacts()
			if missing {
				seed := computeIntegrationFacts(items)
				if neighbour {
					fact := seed[id]
					fact.available = false
					seed[id] = fact
					delete(seed, "dev.floter.second")
				} else {
					delete(seed, id)
				}
				integrationFactsMemo.publish(seed, time.Now())
			}
			reads := 0
			got := refreshIntegrationFacts(&integrationFactsMemo, items, func(given []extensions.Integration) map[string]integrationFact {
				old := computeIntegrationFacts(given)
				reads++
				if reads != 1 {
					return old
				}
				if !neighbour {
					fact := old[id]
					if digest {
						fact.digest = "retired-manifest-digest"
					} else {
						fact.available = false
					}
					old[id] = fact
				}
				// A real inventory refresh and its successful probe complete while
				// the earlier synchronous read is returning its result.
				a.RefreshIntegrations(context.Background())
				a.integrationFacts(a.Integrations.Inventory().Integrations)
				return old
			}, nil, time.Now())
			tt.Frame()
			tt.Frame()
			syncRefreshDump(t, tt, scenario, dark)
			var visible settingsui.Integration
			for _, in := range a.integrationList() {
				if in.ID == id {
					visible = in
				}
			}
			if digest {
				if got[id].digest != current {
					t.Errorf("%s: synchronous probe returned retired approval digest", scenario)
				}
				if visible.Approval == nil || visible.Approval.CurrentDigest != current {
					t.Errorf("%s: synchronous probe revived approval warning: %+v", scenario, visible.Approval)
				}
			} else {
				if !got[id].available {
					t.Errorf("%s: synchronous probe returned retired unavailable state", scenario)
				}
				if !visible.RuntimeAvailable || tt.HasText("Tool unavailable") {
					t.Errorf("%s: synchronous probe disabled a repaired tool", scenario)
				}
			}
		})
	}
}

func TestFactsSyncFirstProbeCannotUndoRepair(t *testing.T) { checkFactsSyncRefresh(t, "cold-runtime") }
func TestFactsSyncFirstDigestCannotUndoApproval(t *testing.T) {
	checkFactsSyncRefresh(t, "cold-approval")
}

func TestFactsSyncAddedProbeCannotUndoNeighbourRepair(t *testing.T) {
	checkFactsSyncRefresh(t, "added-neighbour-runtime")
}
func TestFactsSyncAddedDigestCannotUndoOwnApproval(t *testing.T) {
	checkFactsSyncRefresh(t, "added-own-approval")
}
