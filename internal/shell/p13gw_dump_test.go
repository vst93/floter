package shell

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/ansi"
	"floter/internal/launcher"
)

// TestDumpP13GWWindows is the opt-in evidence dump for P13-gw: the captured
// output window carrying coloured runs and an empty answer, and a note, in both
// appearances. FLOTER_P13GW_DUMP=1 go test -run TestDumpP13GWWindows ./internal/shell/
func TestDumpP13GWWindows(t *testing.T) {
	if os.Getenv("FLOTER_P13GW_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13GW_DUMP=1")
	}
	const styled = "\x1b[32m+added\x1b[0m  \x1b[31m-removed\x1b[0m  plain\n" +
		"\x1b[1;34mbold blue\x1b[0m  \x1b[2mdim\x1b[0m  the rest of the line is body ink\n"
	sentence := "The command produced no output"
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		a := barApp(t, appearance.dark)
		out := &detachedOutput{
			title: "diff --floter",
			ran:   true,
			view: launcher.OutputView{
				Title: "diff --floter",
				Text:  "+added  -removed  plain\nbold blue  dim  the rest of the line is body ink\n",
				Spans: ansi.Parse(styled),
			},
		}
		empty := &detachedOutput{
			title: "make --floter",
			ran:   true,
			view:  launcher.OutputView{Title: "make --floter", Text: sentence, Empty: true},
		}
		note := &pinned{title: "clipboard", text: strings.Repeat("a note keeps what it was given\n", 12)}
		for name, draw := range map[string]func(c *ui.Context){
			"runs":  func(c *ui.Context) { a.detachedView(c, out) },
			"empty": func(c *ui.Context) { a.detachedView(c, empty) },
			"note":  func(c *ui.Context) { a.pinnedView(c, note) },
		} {
			tt := ui.NewTester(draw, 720, 460)
			tt.SetDark(appearance.dark)
			tt.Frame()
			tt.Frame()
			path := "/tmp/floter-P13-gw-" + name + "-" + appearance.name + ".png"
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s %s -> %s", name, appearance.name, path)
		}
	}
}
