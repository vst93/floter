package launcher

import (
	"testing"

	"floter/internal/apps"
	"floter/internal/extensions"
)

// The ordinary list announces a block only with a `heading` row: the title is a
// row of its own, and `windowRows` reports no section titles beside it. A
// plugin list's own group name (the old build's `list-sections.ts` pluginGroup)
// belongs to the captured-output view, whose rows carry a `group` and whose
// `outputRow` prints it — not to the launcher's result items, which is why
// `Item` has no section field left to set.
func TestTheOrdinaryListChargesNoSectionBands(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
	a.SetRecent([]string{"/Applications/Editor.app"}, true)
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID:   "io.github.vst93.zoo",
		IntegrationName: "Zoo Tools",
		Command: extensions.Command{
			ID:          "zebra",
			Name:        "Zebra",
			Description: "A striped animal",
		},
		Program: "/bin/echo",
		Mode:    "pty",
	}})
	view := render(t, a)
	view.Frame()

	results := a.Results()
	rows, held, sections := a.windowRows(results)
	if sections != 0 {
		t.Errorf("the ordinary list reported %d section bands", sections)
	}
	if held != len(results) || len(rows) != len(results) {
		t.Fatalf("rows = %d, held = %d, results = %d", len(rows), held, len(results))
	}
	// The "Recently launched" block is still announced — as a heading row that
	// the window charges as a heading rather than as a section band.
	found := false
	for i, item := range results {
		if item.heading {
			found = true
			if !rows[i].Heading {
				t.Errorf("the heading row at %d is not charged as a heading", i)
			}
		} else if rows[i].Heading {
			t.Errorf("the ordinary row at %d is charged as a heading", i)
		}
	}
	if !found {
		t.Fatalf("no heading row in the list: %v", ids(results))
	}
}
