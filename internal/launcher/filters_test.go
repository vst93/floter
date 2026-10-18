package launcher

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/ansi"
	"floter/internal/apps"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/tools"

	calculatorpkg "floter/internal/calculator"
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
// over and never numbers, and it is charged as its own band.
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
	base := Geometry{Font: a.Font, Spacing: a.Spacing, Rows: a.Rows, Held: a.HeldRows, TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine}.Height()
	withRow := Geometry{Font: a.Font, Spacing: a.Spacing, Rows: a.Rows, Held: a.HeldRows, Feedback: true,
		TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine}.Height()
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

// The install row's right-hand source names the package manager the command
// will use — the same "where did this row come from" answer a command row gives
// with its integration name, and the manager's own proper noun ("Homebrew"),
// not the executable's. The shortcut slot stays empty: the manager is not a
// key hint.
func TestInstallRowNamesTheManagerAsItsSource(t *testing.T) {
	a := testApp()
	a.SetToolCatalog([]tools.State{{
		Entry: tools.Entry{ID: "fd", Name: "fd", Probes: []string{"fd"}, Keywords: []string{"find"},
			Recipes: map[string][]tools.Recipe{"darwin": {{Manager: "brew", Package: "fd"}}}},
		Installed: false, Manager: "brew", Package: "fd", Command: "brew install fd",
	}})
	a.Query = "fd"
	rows := a.toolInstallItems()
	if len(rows) != 1 {
		t.Fatalf("install rows = %+v", rows)
	}
	if rows[0].Source != "Homebrew" {
		t.Errorf("source = %q, want Homebrew", rows[0].Source)
	}
	if rows[0].Shortcut != "" {
		t.Errorf("the manager is not a key hint: shortcut = %q", rows[0].Shortcut)
	}
	if rows[0].Detail != "brew install fd" {
		t.Errorf("detail = %q, want the command verbatim", rows[0].Detail)
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
	a.Query = "zzz"
	if got := a.emptyMessage(copy); got != copy.ClipboardEmptyFilter {
		t.Errorf("clipboard filter = %q", got)
	}
	// The favorites chip.
	a.Query = ""
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
	// Releasing the modifier drops the row and resets the selection to the
	// empty page's own default (its first runnable row, the reference's R64
	// release edge — unconditional, as the reference's is).
	tt.HoldModifiers(0)
	tt.Frame()
	if hasItem(a.Results(), "system-terminal-bare") {
		t.Errorf("the row survived the release: %+v", a.Results())
	}
	if a.Selected != a.firstSelectable() {
		t.Errorf("the selection did not come back: %d, want the page's default %d", a.Selected, a.firstSelectable())
	}
	// A typed query turns the row away: the reference's one placement is the
	// empty field (R64, `main:src/launcher/terminal-row.ts:77-85`). The port
	// had kept R60's input-state append, so the action word showed up beside
	// the matches — the report 「输入框中有输入内容时出现的「在终端中打开」项：
	// 现在固定在第一项（错误）」. Type first, then hold, the order the report
	// names.
	tt.Type("zzz")
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	got := a.Results()
	if hasItem(got, "system-terminal-bare") {
		t.Fatalf("a typed query kept the held row: %+v", got)
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

// The gear is a glyph, and it says which way it goes: while the sheet is
// closed it configures the plugin, while it is open the same control is the
// sheet's only close and its line says so (the old build's
// `plugins.config.openHint` and `plugins.config.close`).
func TestPluginGearCarriesItsHints(t *testing.T) {
	a, _ := clipboardApp(t)
	a.Actions.ConfigFor = func(plugin string) *Config {
		return &Config{
			Schema: &plugincfg.Schema{Plugin: plugin, Title: "Clipboard history"},
			Values: plugincfg.Values{},
		}
	}
	a.Actions.DrawConfig = func(c *ui.Context, config *Config) { ui.Text(c, config.Schema.Title) }
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	copy := a.copy()
	hover := func(label string) {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q control: %q", label, tt.Texts())
		}
		// Park the pointer away first, so this control's own rest timer
		// starts fresh rather than warming on the previous tip.
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H+120)
		tt.Frame()
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
		// The tip waits for the pointer to rest (Base UI's 600ms).
		time.Sleep(700 * time.Millisecond)
		tt.Frame()
	}
	hover(copy.ConfigurePlugin)
	if !tt.HasText(copy.ConfigurePluginHint) {
		t.Errorf("the gear has no tooltip %q: %q", copy.ConfigurePluginHint, tt.Texts())
	}

	if err := tt.Click(copy.ConfigurePlugin); err != nil {
		t.Fatalf("the gear: %v", err)
	}
	tt.Frame()
	if !a.configOpen() {
		t.Fatal("the sheet did not open")
	}
	hover(copy.ConfigurePluginClose)
	if !tt.HasText(copy.ConfigurePluginCloseHint) {
		t.Errorf("the close control has no tooltip %q: %q", copy.ConfigurePluginCloseHint, tt.Texts())
	}
}

// R41 · the chips row is the plugin *list's* own filter, so it hides with the
// list while the plugin's configuration sheet takes the list's place — the
// user read the chips as the list's chrome, not the search field's. And the
// sheet's own door (the gear) rides the field row's tail, where the reference
// put it, not the chips row that just left; in a plugin mode the field's tail
// is the plugin's, so the sessions/settings pair is not there either.
func TestConfigSheetHidesTheChipsRow(t *testing.T) {
	a, _ := clipboardApp(t)
	a.Actions.ConfigFor = func(plugin string) *Config {
		return &Config{
			Schema: &plugincfg.Schema{Plugin: plugin, Title: "Clipboard history"},
			Values: plugincfg.Values{},
		}
	}
	a.Actions.DrawConfig = func(c *ui.Context, config *Config) { ui.Text(c, config.Schema.Title) }
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	copy := a.copy()
	// The chips are on screen before the sheet, with the gear above them in
	// the field row's own band.
	if !tt.HasText(copy.ClipboardFilterFavorites) {
		t.Fatalf("the chips are missing: %q", tt.Texts())
	}
	gear, ok := tt.Find(copy.ConfigurePlugin)
	if !ok {
		t.Fatalf("the gear is missing: %q", tt.Texts())
	}
	chip, ok := tt.Find(copy.ClipboardFilterFavorites)
	if !ok {
		t.Fatalf("the chip's box is missing: %q", tt.Texts())
	}
	if gear.Y >= chip.Y {
		t.Errorf("the gear (%v) is not above the chips (%v)", gear, chip)
	}
	// In a plugin mode the field's tail is the plugin's: the sessions and
	// settings entries are not drawn.
	if tt.HasText(copy.FieldSessions) || tt.HasText(copy.FieldSettings) {
		t.Errorf("the field kept its own buttons in a plugin mode: %q", tt.Texts())
	}
	// Opening the sheet takes the chips with it — a filter for an invisible
	// list is chrome with no referent.
	if err := tt.Click(copy.ConfigurePlugin); err != nil {
		t.Fatalf("the gear: %v", err)
	}
	tt.Frame()
	if !a.configOpen() {
		t.Fatal("the sheet did not open")
	}
	if tt.HasText(copy.ClipboardFilterFavorites) {
		t.Errorf("the chips outlived the list: %q", tt.Texts())
	}
	if !tt.HasText("Clipboard history") {
		t.Errorf("the sheet is not drawn: %q", tt.Texts())
	}
	// The gear is still there, flipped to the sheet's only close control.
	if !tt.HasText(copy.ConfigurePluginClose) {
		t.Errorf("the sheet's close control is missing: %q", tt.Texts())
	}
	// Closing it brings the chips back.
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.configOpen() {
		t.Fatal("Escape left the sheet open")
	}
	if !tt.HasText(copy.ClipboardFilterFavorites) {
		t.Errorf("the chips did not come back: %q", tt.Texts())
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

// Only the kinds that carry one piece of text earn a pin: a text entry always,
// an image when it stored a caption, and a file list never — pinning several
// paths as one body would be a lie about what the window holds (R90). The
// window's title is the row's own text, not the collapsed one-line label.
func TestOnlySingleTextClipboardRowsArePinnable(t *testing.T) {
	cases := []struct {
		name      string
		entry     clipboardpkg.Entry
		wantPin   bool
		wantTitle string
		wantBody  string
	}{
		{
			name:      "text: the row preview names the window, the body is the capture",
			entry:     clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "git status\ngit diff"},
			wantPin:   true,
			wantTitle: "git status",
			wantBody:  "git status\ngit diff",
		},
		{
			name:      "image with a caption pins the caption",
			entry:     clipboardpkg.Entry{Kind: clipboardpkg.KindImage, Text: "quarterly report", Width: 20, Height: 10},
			wantPin:   true,
			wantTitle: "quarterly report",
			wantBody:  "quarterly report",
		},
		{
			name:    "a bare image has nothing to pin",
			entry:   clipboardpkg.Entry{Kind: clipboardpkg.KindImage, Width: 20, Height: 10},
			wantPin: false,
		},
		{
			name:    "a file list has no single body to pin",
			entry:   clipboardpkg.Entry{Kind: clipboardpkg.KindFiles, Paths: []string{"/tmp/a.png", "/tmp/b.png"}},
			wantPin: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clipboardPinApplies(tc.entry); got != tc.wantPin {
				t.Fatalf("clipboardPinApplies = %v, want %v", got, tc.wantPin)
			}
			a := testApp()
			var got [][2]string
			a.Actions.PinText = func(title, text string) { got = append(got, [2]string{title, text}) }
			a.pinClip(tc.entry)
			if !tc.wantPin {
				if len(got) != 0 {
					t.Fatalf("an unpinnable row pinned %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("pinned = %v", got)
			}
			if got[0][0] != tc.wantTitle || got[0][1] != tc.wantBody {
				t.Errorf("pinned %q / %q, want %q / %q", got[0][0], got[0][1], tc.wantTitle, tc.wantBody)
			}
		})
	}
}

// An empty text capture is still a capture: the pin is real, and the window
// takes its name from the type word the row would show.
func TestAnEmptyTextCaptureStillPins(t *testing.T) {
	a := testApp()
	var got [][2]string
	a.Actions.PinText = func(title, text string) { got = append(got, [2]string{title, text}) }
	a.pinClip(clipboardpkg.Entry{Kind: clipboardpkg.KindText})
	if len(got) != 1 || got[0][0] != a.copy().ClipboardTypeText || got[0][1] != "" {
		t.Fatalf("pinned = %v", got)
	}
}

// The control itself obeys the same rule: a selected file row shows no pin,
// because there is no text window to open.
func TestAFileRowShowsNoPinControl(t *testing.T) {
	store := clipboardpkg.NewStore(clipboardpkg.FromConfigRoot(t.TempDir()), 0)
	if _, _, err := store.AddFiles([]string{"/tmp/a.png", "/tmp/b.png"}); err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.Clipboard = store
	a.EnterClipboard()
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText("a.png +1") {
		t.Fatalf("the file row is missing: %q", tt.Texts())
	}
	if tt.HasText("Pin as a window") {
		t.Fatalf("a file row offered a pin: %q", tt.Texts())
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

// The field row's own buttons, as the old build kept them beside the field:
// the sessions page and the settings, each one click from the search row and
// each named for assistive technology. Neither needs a typed command.
func TestFieldRowButtons(t *testing.T) {
	a := testApp()
	settings, sessions := 0, 0
	a.Actions.OpenSettings = func() { settings++ }
	a.Actions.OpenSessions = func() { sessions++ }
	tt := render(t, a)
	for _, label := range []string{"Settings", "Sessions"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("the field row has no %q button: %q", label, tt.Texts())
		}
	}
	if err := tt.Click("Sessions"); err != nil {
		t.Fatalf("sessions: %v", err)
	}
	tt.Frame()
	if sessions != 1 || settings != 0 {
		t.Errorf("sessions = %d, settings = %d", sessions, settings)
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatalf("settings: %v", err)
	}
	tt.Frame()
	if settings != 1 {
		t.Errorf("settings = %d", settings)
	}
}

// A captured run that came from a command can be detached into a window of its
// own, which stays put and re-runs it (the old build's second window). The
// control rides the output view's title row, and it is offered only when the
// run came from a command — there is nothing to re-run otherwise.
func TestOutputViewDetach(t *testing.T) {
	a := testApp()
	detached := []OutputView{}
	a.Actions.DetachOutput = func(view OutputView) { detached = append(detached, view) }
	entry := &extensions.CommandEntry{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}}
	a.output = &OutputView{
		Title:  "jv --floter",
		Text:   "{\"a\": 1}",
		Status: "ok",
		Entry:  entry,
	}
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText("Open in a window") {
		t.Fatalf("the detach control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Open in a window"); err != nil {
		t.Fatalf("detach: %v", err)
	}
	tt.Frame()
	if len(detached) != 1 {
		t.Fatalf("detached = %+v", detached)
	}
	if detached[0].Entry == nil || detached[0].Entry.Command.ID != "jv" {
		t.Errorf("the detached view lost its command: %+v", detached[0].Entry)
	}
	if detached[0].Text != "{\"a\": 1}" {
		t.Errorf("the detached text = %q", detached[0].Text)
	}

	// A run with no command behind it has nothing to re-run.
	a.output = &OutputView{Title: "x", Text: "y", Status: "ok"}
	tt.Frame()
	if tt.HasText("Open in a window") {
		t.Errorf("a run with no command offered a window: %q", tt.Texts())
	}
}

// A command's output is read as a terminal reads it: the colours and weights
// it asked for are styles, the characters without the sequences are what shows
// and what a copy takes. Rendered verbatim the bytes would be visible garbage
// (`^[[31m`), which is what the old build's R65 set out to stop.
func TestOutputReadsAnsiStyles(t *testing.T) {
	a := testApp()
	run := extensions.CapturedRun{
		Command: []string{"git", "diff"},
		Stdout:  "\x1b[32m+added\x1b[0m\n\x1b[31m-removed\x1b[0m",
	}
	view := a.OutputFor(run, nil, nil)
	if strings.Contains(view.Text, "\x1b") {
		t.Fatalf("the text kept its escapes: %q", view.Text)
	}
	if view.Text != "+added\n-removed" {
		t.Errorf("text = %q", view.Text)
	}
	if len(view.Spans) != 3 {
		t.Fatalf("spans = %+v", view.Spans)
	}
	if view.Spans[0].Style.FG.Kind != ansi.Named || view.Spans[0].Style.FG.N != 2 {
		t.Errorf("the first run is not green: %+v", view.Spans[0].Style)
	}
	if view.Spans[2].Style.FG.N != 1 {
		t.Errorf("the second run is not red: %+v", view.Spans[2].Style)
	}

	// Drawn, the escape bytes are nowhere: the runs are their own text.
	a.output = &view
	tt := render(t, a)
	tt.Frame()
	for _, text := range tt.Texts() {
		if strings.Contains(text, "\x1b") || strings.Contains(text, "[32m") {
			t.Errorf("an escape reached the frame: %q", text)
		}
	}
	if !tt.HasText("+added") || !tt.HasText("-removed") {
		t.Errorf("the output is not showing: %q", tt.Texts())
	}
	// Enter copies the text without the sequences.
	if got := a.outputText(); got != "+added\n-removed" {
		t.Errorf("a copy would take %q", got)
	}
}

// A history row carries its own star: the favourite toggle the old build drew
// at the row's trailing edge, hidden until the row is hovered or chosen and
// shown whenever the entry is starred. Clicking it stars the entry without
// running the row.
func TestHistoryRowStar(t *testing.T) {
	a, store := clipboardApp(t)
	if len(store.Entries()) == 0 {
		t.Fatal("the fixture has no entries")
	}
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	// The star is on the chosen row and names itself for assistive
	// technology.
	if _, ok := tt.Find("Favorite"); !ok {
		t.Fatalf("the star is missing: %q", tt.Texts())
	}
	runs := testRuns["copy"]
	if err := tt.Click("Favorite"); err != nil {
		t.Fatalf("the star: %v", err)
	}
	tt.Frame()
	// The entry is starred, and the row did not run.
	entry, ok := a.selectedClip()
	if !ok {
		t.Fatal("no entry is selected")
	}
	if !entry.Favorite {
		t.Errorf("the star did not land: %+v", entry)
	}
	if testRuns["copy"] != runs {
		t.Errorf("the star ran the row: %v", testRuns)
	}
	// Clicking again unstars it: the control's own name follows the state, so
	// a starred row's star reads "Unfavorite".
	if err := tt.Click("Unfavorite"); err != nil {
		t.Fatalf("the star again: %v", err)
	}
	tt.Frame()
	if entry, _ := a.selectedClip(); entry.Favorite {
		t.Errorf("the star did not clear: %+v", entry)
	}
}

// The chips row carries the mode's own hint at its end, as the old build's
// `.launcher-filter__hint` did: what Tab does, and the favourite key spelled
// the way this platform spells it.
func TestFilterRowHint(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	copy := a.copy()
	accelerator := "Ctrl+D"
	if runtime.GOOS == "darwin" {
		accelerator = "Cmd+D"
	}
	want := copy.ClipboardFilterHint(shortcuts.Badge(accelerator, "D"))
	if !tt.HasText(want) {
		t.Fatalf("the chips hint is missing: %q (want %q)", tt.Texts(), want)
	}
	// The badge is the platform's own spelling of the app modifier.
	if runtime.GOOS == "darwin" {
		if badge := shortcuts.Badge("Cmd+D", "D"); badge != "\u2318D" {
			t.Errorf("macOS badge = %q", badge)
		}
	} else if badge := shortcuts.Badge("Ctrl+D", "D"); badge != "Ctrl+D" {
		t.Errorf("badge = %q", badge)
	}
	// The browser mode's own hint has no favourite key in it.
	a.clipboard = false
	a.EnterBrowser()
	tt.Frame()
	if !tt.HasText(copy.BrowserFilterHint) {
		t.Errorf("the browser hint is missing: %q", tt.Texts())
	}
	// A plain query has no chips row and so no hint.
	a.ResetQuery()
	tt.Frame()
	if tt.HasText(copy.BrowserFilterHint) {
		t.Errorf("the hint outlived its mode: %q", tt.Texts())
	}
}

// A numbered row's badge is the select_result binding's own modifiers with the
// digit: `⌘1` on macOS, `Ctrl+1` elsewhere, and the bare digit when the stored
// binding will not parse — the old build's `formatResultShortcut`.
func TestResultNumberBadge(t *testing.T) {
	a := testApp()
	accelerator := "Ctrl+1"
	if runtime.GOOS == "darwin" {
		accelerator = "Cmd+1"
	}
	if got := a.resultNumber(1); got != shortcuts.Badge(accelerator, "1") {
		t.Errorf("badge = %q, want %q", got, shortcuts.Badge(accelerator, "1"))
	}
	// The binding the user stored is the one the badge shows.
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutSelectResult, "Alt+7")
	}); err != nil {
		t.Fatal(err)
	}
	if got := a.resultNumber(1); got != shortcuts.Badge("Alt+7", "1") {
		t.Errorf("badge = %q, want Alt+7", got)
	}
	// A binding that will not parse is refused by the store's own normaliser,
	// which leaves the last good one in place: the badge never shows a key the
	// stored binding does not have.
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutSelectResult, "not a key")
	}); err != nil {
		t.Fatal(err)
	}
	if got := a.resultNumber(1); got != shortcuts.Badge("Alt+7", "1") {
		t.Errorf("badge = %q, want the stored binding's", got)
	}
}

// A command whose output the launcher shows itself carries the space-bar
// keycap on its row, as the old build's R65 put there; a terminal-route
// command does not, since its output goes to the terminal surface. The
// field's own nudge — the promise that one space enters the mode — is the
// entry rule's, not the route's, so it shows either way as one paragraph
// (`␣ {name} · enter`); the page's only *standalone* `␣` is the row's
// keycap, and its presence is what the route decides.
func TestDirectOutputNudge(t *testing.T) {
	direct := commandApp()
	for i := range direct.Commands {
		direct.Commands[i].Route = extensions.RouteBackground
	}
	direct.Query = "JSON"
	tt := render(t, direct)
	tt.Frame()
	if !tt.HasText("\u2423") {
		t.Errorf("the space-bar nudge is missing: %q", tt.Texts())
	}
	if n := countTexts(tt, "\u2423"); n != 1 {
		t.Errorf("the direct page shows %d standalone \u2423, want the row's keycap alone: %q", n, tt.Texts())
	}
	// A terminal-route command runs where a terminal runs it, and its row
	// carries no keycap: only the field nudge's paragraph, which names the
	// command, is left — no bare keycap anywhere.
	terminal := commandApp()
	terminal.Query = "JSON"
	tt = render(t, terminal)
	tt.Frame()
	if n := countTexts(tt, "\u2423"); n != 0 {
		t.Errorf("a terminal-route row carries the keycap: %d standalone \u2423: %q", n, tt.Texts())
	}
}

// countTexts counts the frame's labels that say exactly s.
func countTexts(tt *ui.Tester, s string) int {
	n := 0
	for _, text := range tt.Texts() {
		if text == s {
			n++
		}
	}
	return n
}

// A browser search with the plugin switched off says so, instead of claiming
// the machine has no browser: the mode's own answer names where to turn the
// plugin back on.
func TestBrowserModeDisabled(t *testing.T) {
	a := testApp()
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{Disabled: true})
	}
	copy := a.copy()
	a.EnterBrowser()
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText(copy.BrowserDisabled) {
		t.Errorf("the disabled answer is missing: %q", tt.Texts())
	}
	if tt.HasText(copy.BrowserNoProfile) {
		t.Errorf("a switched-off plugin claimed no profile: %q", tt.Texts())
	}
}

// A calculator row's answer is printed complete at the row's right edge —
// nowrap, strong, at 620 — and the expression is what gives up room, as the
// old build's R50 had it: the result can never be truncated or wrapped.
func TestCalculatorTrailingResult(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculatorpkg.Entry{
		{ID: "1", Expression: "sqrt(2)", Result: "1.4142135623730951", CreatedAt: 2},
	}}
	a.Calculator = fixture
	a.EnterCalculator()
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText("1.4142135623730951") {
		t.Fatalf("the answer is missing: %q", tt.Texts())
	}
	if rect, ok := tt.Find("1.4142135623730951"); !ok || rect.W < rect.H {
		t.Errorf("the answer is not drawn on its own line: %+v", rect)
	}
}

// The held row leads with the terminal glyph, the reference's own icon for the
// bare-terminal row (`.launcher-result__icon--system` → SystemActionIcon): the
// recents' clock the id used to fall through to was never drawn there.
func TestHeldModifierRowCarriesTheTerminalGlyph(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Frame()
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	results := a.Results()
	item := results[len(results)-1]
	if item.ID != "system-terminal-bare" {
		t.Fatalf("the held row is not last: %+v", results)
	}
	glyph, ok := rowGlyph(item)
	if !ok || glyph != glyphTerminal {
		t.Errorf("the held row's glyph = %v, want the terminal mark", ok)
	}
}

// The held row is the one row the window grows for: the reference's R67 rule
// (LauncherResults.tsx's isBareTerminalRow branch) never scrolls it into view,
// because scrolling it in against the old, shorter box makes the landing clamp
// the scroll back — the list slid up and back down on the press edge. The row
// is given the room; the list stays at its top.
func TestHeldModifierRowIsNotScrolledIntoView(t *testing.T) {
	a := testApp()
	// A window that holds only part of the list: the reference's own world —
	// the row lands below the fold, so a scroll is a scroll the user sees.
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, int(WindowHeight("small"))+120)
	tt.Frame()
	first, last := a.List.Visible()
	if last >= len(a.Results()) {
		t.Skipf("the test window already shows every row; the scroll rule is not exercised")
	}
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	now, lastNow := a.List.Visible()
	if now != first || lastNow < a.Selected {
		t.Errorf("the list scrolled for the held row: %d..%d (was %d..%d), selected %d",
			now, lastNow, first, last, a.Selected)
	}
}

// The modifier over a typed query hands the selection to the action bar (a
// typed query draws no held row, R64 — see heldTerminalItem), and the bar keeps
// it for as long as the key is down: every keystroke rebuilds the list, and the
// query's own default reset used to land on the first match, leaving the bar on
// screen and unlit — the report 「应该有选中样式（当前缺失）」. Releasing the key
// gives the page's own default back.
func TestTheHeldModifierKeepsTheBarSelectedWhileTyping(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	a.SetQuery("open settings")
	tt.Frame()
	results := a.Results()
	bar := actionBarIndex(results)
	if bar < 0 {
		t.Fatalf("the query drew no action bar: %+v", results)
	}
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if a.Selected != bar {
		t.Errorf("the modifier's press put the selection on %d, want the bar %d", a.Selected, bar)
	}
	// A keystroke with the key still down: the bar is still the row the
	// modifier acts on, so it keeps the selection.
	a.SetQuery("open setting")
	tt.Frame()
	results = a.Results()
	bar = actionBarIndex(results)
	if bar < 0 {
		t.Fatalf("the bar left the list after a keystroke: %+v", results)
	}
	if a.Selected != bar {
		t.Errorf("typing with the modifier down moved the selection to %d, want the bar %d", a.Selected, bar)
	}
	// Released: the page's own default comes back.
	tt.HoldModifiers(0)
	tt.Frame()
	if want := a.defaultSelection(a.Results()); a.Selected != want {
		t.Errorf("the release left the selection on %d, want the page's default %d", a.Selected, want)
	}
}
