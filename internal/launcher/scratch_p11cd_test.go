package launcher

import (
	"testing"

	"floter/internal/apps"
)

func TestScratchPasteMatch(t *testing.T) {
	for _, text := range []string{"git\n", "  git  ", "sublime text\n", "gti\n"} {
		a := testApp()
		a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}, {Name: "Sublime Text", Path: "/usr/bin/subl"}}, true)
		tt := render(t, a)
		tt.SetClipboard(text)
		tt.Command("paste")
		tt.Frame()
		var titles []string
		for _, it := range a.Results() {
			titles = append(titles, it.ID)
		}
		t.Logf("paste %q -> query %q results=%v", text, a.Query, titles)
	}
}
