package launcher

import (
	"fmt"
	"slices"
	"testing"

	"floter/internal/apps"
	"github.com/egoist/mygo/ui"
)

func TestModifierReusesSearchAndKeepsOneTerminalAction(t *testing.T) {
	for _, query := range []string{"", "application"} {
		a := testApp()
		a.SetApps([]apps.App{{Name: "Application", Path: "/Applications/example.app"}})
		a.SetRecent([]string{"/Applications/example.app"}, true)
		a.Query = query
		a.Results()
		builds := a.ordinarySearch.builds
		if builds != 1 {
			t.Fatalf("initial query built %d times, want 1", builds)
		}
		for i := range 30 {
			a.modifierHeld = i%2 == 0
			rows := a.Results()
			bars := 0
			for _, row := range rows {
				if len(row.ID) >= 4 && row.ID[:4] == "bar:" {
					bars++
				}
			}
			want := 1
			if query == "" && !a.modifierHeld {
				want = 0
			}
			if bars != want {
				t.Fatalf("query=%q held=%v: %d bars, want %d", query, a.modifierHeld, bars, want)
			}
		}
		if a.ordinarySearch.builds != builds {
			t.Fatal("modifier-only frames repeated search work")
		}
	}
}

func TestSearchMemoInvalidatesForQueryCatalogAndRecentOrder(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{{Name: "Alpha", Path: "alpha"}, {Name: "Beta", Path: "beta"}})
	a.SetRecent([]string{"alpha", "beta"}, true)
	first := ids(a.Results())
	a.Recent[0], a.Recent[1] = a.Recent[1], a.Recent[0]
	if slices.Equal(ids(a.Results()), first) {
		t.Fatal("recent reorder served stale results")
	}
	a.ShowRecent = false
	if len(a.Results()) != 0 {
		t.Fatal("disabled recents remain visible")
	}
	a.Query = "alpha"
	if len(a.Results()) == 0 {
		t.Fatal("query did not invalidate empty result")
	}
	a.SetApps([]apps.App{{Name: "Gamma", Path: "gamma"}})
	if len(a.Results()) != 0 {
		t.Fatal("catalog refresh retained a removed application")
	}
	a.Query = "gamma"
	if len(a.Results()) == 0 {
		t.Fatal("new catalog result missing")
	}
}

func BenchmarkModifierSwitch(b *testing.B) {
	a := testApp()
	var found []apps.App
	for i := range 1000 {
		found = append(found, apps.App{Name: fmt.Sprintf("Application %d", i), Path: fmt.Sprintf("/Applications/%d.app", i)})
	}
	a.SetApps(found)
	a.Query = "application"
	tt := ui.NewTester(themedView(a), 720, 420)
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		mods := ui.Modifiers(0)
		if i%2 == 0 {
			mods = ui.Cmd
		}
		tt.HoldModifiers(mods)
		tt.Frame()
	}
}
