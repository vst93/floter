package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/tools"

	clipboardpkg "floter/internal/clipboard"
	"floter/internal/i18n"
)

// The axis cycle wraps at both ends and lands on the first value for a choice
// off the axis.
func TestFilterAxisCycle(t *testing.T) {
	axis := FilterAxis{Values: []string{"a", "b", "c"}}
	cases := []struct {
		current   string
		direction int
		want      string
	}{
		{"a", 1, "b"}, {"b", 1, "c"}, {"c", 1, "a"},
		{"a", -1, "c"}, {"c", -1, "b"}, {"b", -1, "a"},
		{"off", 1, "a"},
	}
	for _, c := range cases {
		if got := axis.Cycle(c.current, c.direction); got != c.want {
			t.Errorf("Cycle(%q, %d) = %q, want %q", c.current, c.direction, got, c.want)
		}
	}
}

// The three axes are the ones the old build declared, in order.
func TestFilterAxesMatchTheOldBuild(t *testing.T) {
	want := map[string][]string{
		"browser":   {filterAll, filterBookmarks, filterHistory, filterTabs},
		"clipboard": {filterAll, filterFavorites, filterText, filterImage, filterLink, filterFiles},
	}
	if got := BrowserFilterAxis.Values; len(got) != len(want["browser"]) {
		t.Errorf("browser = %v", got)
	} else {
		for i, v := range want["browser"] {
			if got[i] != v {
				t.Errorf("browser[%d] = %q, want %q", i, got[i], v)
			}
		}
	}
	if got := ClipboardFilterAxis.Values; len(got) != len(want["clipboard"]) {
		t.Errorf("clipboard = %v", got)
	} else {
		for i, v := range want["clipboard"] {
			if got[i] != v {
				t.Errorf("clipboard[%d] = %q, want %q", i, got[i], v)
			}
		}
	}
	if got := CalculatorFilterAxis.Values; len(got) != 2 || got[0] != filterAll || got[1] != filterFavorites {
		t.Errorf("calculator = %v", got)
	}
}

// Every value of every axis has a word, in both languages: a chip that prints
// nothing is the drift the axis exists to prevent.
func TestEveryChipHasAWord(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		copy := i18n.For(language).Launcher
		for _, axis := range []FilterAxis{BrowserFilterAxis, ClipboardFilterAxis, CalculatorFilterAxis} {
			for _, value := range axis.Values {
				word := axis.LabelOf(copy, value)
				if word == "" || word == value {
					t.Errorf("%s: %q has no word", language, value)
				}
			}
		}
	}
}

// The clipboard kind split: a link is a text entry whose whole value is a
// page address, and the stored kinds pass through.
func TestClipboardEntryKind(t *testing.T) {
	cases := []struct {
		entry clipboardpkg.Entry
		want  string
	}{
		{clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "hello world"}, filterText},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "https://example.com/a"}, filterLink},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "see https://example.com"}, filterText},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindImage}, filterImage},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindFiles, Paths: []string{"/tmp/a"}}, filterFiles},
	}
	for _, c := range cases {
		if got := clipboardEntryKind(c.entry); got != c.want {
			t.Errorf("kind(%q, %q) = %q, want %q", c.entry.Kind, c.entry.Text, got, c.want)
		}
	}
}

// The list's own section headings: a heading row is a label the keyboard steps
// over and never numbers, and a block's first row prints its title above
// itself.
func TestSectionHeadings(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
	a.SetRecent([]string{"/Applications/Editor.app"}, true)
	// A taller window: the recents and their heading sit below the built-ins.
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) },
		InputWindowWidth, int(WindowHeight("small"))+200)
	tt.Frame()
	if !tt.HasText("Recently launched") {
		t.Errorf("the recents' heading is missing: %q", tt.Texts())
	}
	results := a.Results()
	headingAt := -1
	for i, item := range results {
		if item.heading {
			headingAt = i
		}
	}
	if headingAt < 0 {
		t.Fatal("no heading row in the list")
	}
	// The keyboard never lands on a heading.
	a.Selected = 0
	for i := 0; i < len(results)+2; i++ {
		a.Selected = a.nextSelectable(a.Selected, 1)
		if results[a.Selected].heading {
			t.Fatalf("the arrows landed on the heading at %d", a.Selected)
		}
	}
	// A heading has no action: Enter on it does nothing.
	if results[headingAt].Run != nil {
		t.Error("a heading is runnable")
	}
	// The dropped-file block prints its own heading.
	a.SetDropped([]drops.File{{Name: "a.txt", Path: "/tmp/a.txt", Directory: "/tmp"}})
	a.files = true
	files := a.filesItems()
	if len(files) == 0 || !files[0].heading || files[0].Title != a.copy().FilesSection {
		t.Errorf("the dropped block's heading is missing: %+v", files)
	}
}

// The docked status row: a message shows under the list with the warning tone
// when it reports a failure, it is charged to the window's height, and it
// expires on its own clock.
func TestFeedbackRow(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	if a.FeedbackVisible() {
		t.Fatal("the row shows before any message")
	}
	base := Geometry{Font: a.Font, Spacing: a.Spacing, RowLines: a.RowLines, Held: a.HeldRows}.Height()
	withRow := Geometry{Font: a.Font, Spacing: a.Spacing, RowLines: a.RowLines, Held: a.HeldRows, Feedback: true}.Height()
	if withRow <= base {
		t.Errorf("the feedback band is not charged: %d vs %d", withRow, base)
	}
	a.Feedback("copied")
	if !a.FeedbackVisible() {
		t.Fatal("the message did not show")
	}
	tt.Frame()
	if !tt.HasText("copied") {
		t.Errorf("the row did not draw: %q", tt.Texts())
	}
	if a.toastWarning {
		t.Error("a plain message took the warning tone")
	}
	// A failure message takes the warning tone.
	a.WarnFeedback("it failed")
	tt.Frame()
	if !tt.HasText("it failed") || !a.toastWarning {
		t.Errorf("the warning row: %q warning=%v", tt.Texts(), a.toastWarning)
	}
}

// A command word two integrations share is marked on the rows: the launcher
// enters the first, so the shadowed one says the word is shared rather than
// looking like the only answer.
func TestSharedCommandWordIsMarked(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{
		{IntegrationID: "one", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
		{IntegrationID: "two", Command: extensions.Command{ID: "jv", Name: "JSON Viewer (other)"}},
		{IntegrationID: "three", Command: extensions.Command{ID: "diff", Name: "Diff"}},
	})
	if !a.commandConflicts["jv"] || a.commandConflicts["diff"] {
		t.Errorf("conflicts = %v", a.commandConflicts)
	}
	warned := map[string]bool{}
	for _, item := range a.commandItems() {
		warned[item.Title] = item.warning
	}
	if !warned["JSON Viewer"] || !warned["JSON Viewer (other)"] {
		t.Errorf("the shared word's rows are not marked: %v", warned)
	}
	if warned["Diff"] {
		t.Error("an unshared word was marked")
	}
}

// The invoke row: a tool the catalog knows, that *is* installed, and that
// carries a launch action of its own offers the argv; a pure CLI filter and a
// missing tool offer nothing; and the two rows are mutually exclusive.
func TestToolInvokeRows(t *testing.T) {
	a := testApp()
	spawned := [][]string{}
	terminal := [][]string{}
	a.Actions.SpawnDetached = func(argv []string) { spawned = append(spawned, argv) }
	a.Actions.RunInTerminal = func(argv []string) { terminal = append(terminal, argv) }
	a.SetToolCatalog([]tools.State{
		{Entry: tools.Entry{ID: "flameshot", Name: "Flameshot", Probes: []string{"flameshot"},
			Keywords: []string{"screenshot"}, Launch: []string{"flameshot", "gui"}}, Installed: true},
		{Entry: tools.Entry{ID: "lazygit", Name: "lazygit", Probes: []string{"lazygit"},
			Keywords: []string{"git"}, Launch: []string{"lazygit"}, NeedsTerminal: true}, Installed: true},
		{Entry: tools.Entry{ID: "jq", Name: "jq", Probes: []string{"jq"}, Keywords: []string{"json"}}, Installed: true},
		{Entry: tools.Entry{ID: "fd", Name: "fd", Probes: []string{"fd"}, Keywords: []string{"find"},
			Recipes: map[string][]tools.Recipe{"linux": {{Manager: "pacman", Package: "fd"}}}},
			Command: "pacman -S fd", Installed: false},
	})
	// A detected GUI tool offers its argv, and the row starts it detached.
	a.Query = "flameshot"
	rows := a.toolInvokeItems()
	if len(rows) != 1 || rows[0].Title != "Flameshot" || rows[0].Detail != "flameshot gui" {
		t.Fatalf("invoke rows = %+v", rows)
	}
	rows[0].Run()
	if len(spawned) != 1 || spawned[0][0] != "flameshot" {
		t.Errorf("spawned = %v", spawned)
	}
	// A full-screen TUI goes to the terminal instead.
	a.Query = "lazygit"
	rows = a.toolInvokeItems()
	if len(rows) != 1 {
		t.Fatalf("lazygit rows = %+v", rows)
	}
	rows[0].Run()
	if len(terminal) != 1 || terminal[0][0] != "lazygit" {
		t.Errorf("terminal = %v", terminal)
	}
	// A pure CLI filter carries no launch action: no row.
	a.Query = "jq"
	if rows := a.toolInvokeItems(); len(rows) != 0 {
		t.Errorf("jq produced an invoke row: %+v", rows)
	}
	// A missing tool is the install row's business, not this one.
	a.Query = "fd"
	if rows := a.toolInvokeItems(); len(rows) != 0 {
		t.Errorf("an uninstalled tool produced an invoke row: %+v", rows)
	}
	if rows := a.toolInstallItems(); len(rows) != 1 {
		t.Errorf("install rows = %+v", rows)
	}
}

// The plugin modes name their own empty state rather than claiming a search
// found nothing: nothing copied yet, no favorites, no matches.
func TestModeEmptyMessages(t *testing.T) {
	a, _ := clipboardApp(t)
	copy := a.copy()
	// The clipboard, with nothing copied.
	a.clipboard = true
	if got := a.emptyMessage(copy); got != copy.ClipboardEmpty {
		t.Errorf("clipboard empty = %q", got)
	}
	// A needle that matched nothing.
	a.Query = "clipboard zzz"
	if got := a.emptyMessage(copy); got != copy.ClipboardEmptyFilter {
		t.Errorf("clipboard filter = %q", got)
	}
	// The favorites chip.
	a.Query = "clipboard "
	a.clipboardFilter = filterFavorites
	if got := a.emptyMessage(copy); got != copy.ClipboardEmptyFavorites {
		t.Errorf("clipboard favorites = %q", got)
	}
	// The calculator's two.
	a.clipboard, a.calculatorMode = false, true
	a.calculatorFilter = filterAll
	if got := a.emptyMessage(copy); got != copy.CalculatorEmpty {
		t.Errorf("calculator empty = %q", got)
	}
	a.calculatorFilter = filterFavorites
	if got := a.emptyMessage(copy); got != copy.CalculatorEmptyFavorites {
		t.Errorf("calculator favorites = %q", got)
	}
	// The ordinary page says plainly that a search found nothing.
	a.calculatorMode = false
	if got := a.emptyMessage(copy); got != copy.NoResults {
		t.Errorf("ordinary = %q", got)
	}
}

// The held modifier's row: while the app modifier is down on an empty field,
// the list ends with a bare terminal session and the selection is on it, so
// Enter reaches it without a second key. A query, or a mode, shows nothing.
func TestHeldModifierBareTerminalRow(t *testing.T) {
	a := testApp()
	opened := 0
	a.Actions.OpenTerminal = func() { opened++ }
	tt := render(t, a)
	tt.Frame()
	if got := a.Results(); hasItem(got, "system-terminal-bare") {
		t.Errorf("the row shows with the modifier up: %+v", got)
	}
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	results := a.Results()
	if !hasItem(results, "system-terminal-bare") {
		t.Fatalf("the row is missing while the modifier is held: %+v", results)
	}
	// It is the list's last line, and the selection is on it.
	if results[len(results)-1].ID != "system-terminal-bare" {
		t.Errorf("the row is not last: %+v", results)
	}
	if a.Selected != len(results)-1 {
		t.Errorf("the selection is on %d, want the held row %d", a.Selected, len(results)-1)
	}
	// The chord that put the row there opens it: the modifier is still held
	// when Enter is pressed.
	tt.Key(ui.Cmd, ui.KeyEnter)
	tt.Frame()
	if opened != 1 {
		t.Errorf("Enter opened %d sessions", opened)
	}
	// Releasing the modifier gives the selection back and drops the row.
	tt.HoldModifiers(0)
	tt.Frame()
	if hasItem(a.Results(), "system-terminal-bare") {
		t.Errorf("the row survived the release: %+v", a.Results())
	}
	if a.Selected != 0 {
		t.Errorf("the selection did not come back: %d", a.Selected)
	}
	// A typed query is a search: the row stays away.
	tt.HoldModifiers(ui.Cmd)
	tt.Type("zzz")
	tt.Frame()
	if hasItem(a.Results(), "system-terminal-bare") {
		t.Errorf("the row showed over a query: %+v", a.Results())
	}
}

func hasItem(items []Item, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

// A built-in mode's chips row carries the door to its own configuration: the
// gear opens the plugin's schema in the launcher's own band (the old build's
// R29 overlay), and the gear flips to an ✕ while it shows, so the control that
// opened the sheet is the control that closes it. An extension command's list
// has no such door (its options are the manifest's).
func TestModeOptionsGear(t *testing.T) {
	a, _ := clipboardApp(t)
	asked := []string{}
	a.Actions.ConfigFor = func(plugin string) *Config {
		asked = append(asked, plugin)
		return &Config{
			Schema: &plugincfg.Schema{Plugin: plugin, Title: "Clipboard history", Fields: []plugincfg.Field{
				{Key: "enabled", Kind: plugincfg.Toggle, Label: "Keep a clipboard history"},
			}},
			Values: plugincfg.Values{"enabled": true},
		}
	}
	a.Actions.DrawConfig = func(c *ui.Context, config *Config) {
		ui.Text(c, config.Schema.Title)
	}
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	if !tt.HasText("Options") {
		t.Fatalf("the gear is missing: %q", tt.Texts())
	}
	if err := tt.Click("Options"); err != nil {
		t.Fatalf("the gear: %v", err)
	}
	tt.Frame()
	if len(asked) != 1 || asked[0] != settings.CustomPluginClipboard {
		t.Errorf("asked for %v", asked)
	}
	if !a.configOpen() {
		t.Fatal("the sheet did not open")
	}
	// The sheet took the list's place and the gear became its own close
	// control.
	if !tt.HasText("Clipboard history") {
		t.Errorf("the sheet is not drawn: %q", tt.Texts())
	}
	if !tt.HasText("Close options") || tt.HasText("Options") {
		t.Errorf("the gear did not flip: %q", tt.Texts())
	}
	// Escape closes it and the gear is a gear again.
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.configOpen() {
		t.Error("Escape left the sheet open")
	}
	if !tt.HasText("Options") {
		t.Errorf("the gear did not come back: %q", tt.Texts())
	}
	// An extension command's own list has no gear: its options are its
	// manifest's.
	a.clipboard = false
	a.mode = &extensions.CommandEntry{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}}
	a.Query = "jv "
	tt.Frame()
	if tt.HasText("Options") {
		t.Errorf("an extension command's list offered options: %q", tt.Texts())
	}
}

// Leaving a plugin's mode leaves its sheet: the sheet is that mode's own face.
func TestConfigClosesWithItsMode(t *testing.T) {
	a, _ := clipboardApp(t)
	a.Actions.ConfigFor = func(plugin string) *Config {
		return &Config{
			Schema: &plugincfg.Schema{Plugin: plugin, Title: "Clipboard history"},
			Values: plugincfg.Values{},
		}
	}
	a.Actions.DrawConfig = func(c *ui.Context, config *Config) { ui.Text(c, config.Schema.Title) }
	a.openConfig(settings.CustomPluginClipboard)
	if !a.configOpen() {
		t.Fatal("the sheet did not open")
	}
	a.clipboard = false
	a.Query = ""
	a.syncConfig()
	if a.configOpen() {
		t.Error("the sheet outlived its mode")
	}
}

// A clipboard entry's whole text can be shown in a window of its own, from the
// selected row's own control: the old build's "pin one entry" (R90), whose Tab
// key the filter chips took over.
func TestPinningAClipboardEntry(t *testing.T) {
	a, _ := clipboardApp(t)
	pinned := [][2]string{}
	a.Actions.PinText = func(title, text string) { pinned = append(pinned, [2]string{title, text}) }
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	// The control rides the selected row.
	if !tt.HasText("Pin as a window") {
		t.Fatalf("the pin control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Pin as a window"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	tt.Frame()
	if len(pinned) != 1 {
		t.Fatalf("pinned = %v", pinned)
	}
	if pinned[0][1] == "" {
		t.Errorf("the pinned text is empty: %v", pinned)
	}
	// The pin does not run the row (it is a control, not the row's action).
	if testRuns["copy"] != 0 {
		t.Errorf("the pin also ran the row: %v", testRuns)
	}
}

// A captured run's text can be shown in a window of its own: the pin rides the
// output view's own title row, on the surface that holds the text.
func TestOutputViewPin(t *testing.T) {
	a := testApp()
	pinned := [][2]string{}
	a.Actions.PinText = func(title, text string) { pinned = append(pinned, [2]string{title, text}) }
	a.output = &OutputView{
		Title:  "jv --floter",
		Text:   "{\"a\": 1}",
		Status: "ok",
	}
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText("jv --floter") || !tt.HasText("{\"a\": 1}") {
		t.Fatalf("the output view is not showing: %q", tt.Texts())
	}
	if err := tt.Click("Pin as a window"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	tt.Frame()
	if len(pinned) != 1 || pinned[0][1] != "{\"a\": 1}" {
		t.Fatalf("pinned = %v", pinned)
	}
	if pinned[0][0] != "jv --floter" {
		t.Errorf("the window's title = %q", pinned[0][0])
	}
}
