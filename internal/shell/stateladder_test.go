package shell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
)

// P13-be · 插件窗口正文里的「一句话」此前与它头顶的 chrome 同级
// （`docs/rounds/P13-be-spec.md`、`docs/liquid-glass.md` §6.24）。
//
// 先量的数（720×460 静息、窗口自己的地面 `tokens.Opaque` 浅 246 / 深 20）：
//
//	bar 标题    Caption + TextMuted          浅 131（对地面 115） / 深 130（110）
//	正文 running Caption + TextMuted          浅 131（115） / 深 130（110）
//	正文 empty   Caption + TextMuted          浅 130（116） / 深 131（111）
//	正文 idle    Caption + TextMuted          浅 128（118） / 深 133（113）
//	列表 status  Caption + TextMuted          浅 131（115） / 深 130（110）
//
// 四臂与标题逐级相同 —— 窗口读成「一条 bar + 一条同色的灰线」，没有第二档。
// 这一轮把**正文的四条状态行**分三档，列表的注与 bar 标题都不动。

// detachedArm is one body arm: what the window says and what it says it with.
type detachedArm struct {
	name    string
	running bool
	failed  string
	ran     bool
	view    launcher.OutputView
	kind    detachedStateKind
}

// bodyArms are the four states plus the list's own status row. The status row
// is not a state line: it is a note about the list (see detachedListNote).
func bodyArms(en i18n.Launcher) []detachedArm {
	return []detachedArm{
		{"running", true, "", true, launcher.OutputView{Title: "jv"}, detachedInFlight},
		{"failed", false, "Could not start /opt/bin/node: no such file", true, launcher.OutputView{Title: "jv"}, detachedFault},
		{"empty", false, "", true, launcher.OutputView{Title: "jv"}, detachedStatement},
		{"idle", false, "", false, launcher.OutputView{}, detachedStatement},
		{"liststatus", false, "", true, launcher.OutputView{IsList: true,
			Rows: []extensions.Row{{Status: true, Title: "3 of 4 shown"}}}, detachedStatement},
	}
}

// bodyArmText is the line the arm draws, for `Tester.Find`.
func bodyArmText(arm detachedArm, en i18n.Launcher) string {
	switch arm.name {
	case "running":
		return en.OutputWindowRunning
	case "failed":
		return arm.failed
	case "empty":
		return en.OutputWindowEmpty
	case "idle":
		return en.OutputWindowIdle
	default:
		return "3 of 4 shown"
	}
}

// bodyReadings is one arm's frame: the window's ground, the line's ink, the
// line's box and the bar title's box.
type bodyReadings struct {
	ground, ink ui.Color
	titleInk    ui.Color
	box, title  ui.Rect
}

func bodyProbe(t *testing.T, dark bool, arm detachedArm) bodyReadings {
	t.Helper()
	a := barApp(t, dark)
	detached := &detachedOutput{
		title: "jv", running: arm.running, failed: arm.failed, ran: arm.ran, view: arm.view,
	}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	en := i18n.For("en").Launcher
	box, ok := tt.Find(bodyArmText(arm, en))
	if !ok {
		t.Fatalf("%s: the line is missing: %q", arm.name, tt.Texts())
	}
	title, ok := tt.Find("jv")
	if !ok {
		t.Fatalf("%s: the bar title is missing: %q", arm.name, tt.Texts())
	}
	return bodyReadings{
		ground:   shellPixelAt(tt.Image(), 5, 5),
		ink:      shellInkColor(tt, box),
		titleInk: shellInkColor(tt, title),
		box:      box,
		title:    title,
	}
}

// bodyWeight is how heavy a line reads on its ground: the distance between the
// two lumas. Both candidates sit on the same ground, so this is the only
// comparable number — it does not care which way round the appearance runs
// (light ink is dark on a light ground, dark ink is light on a dark one).
func bodyWeight(ground, ink ui.Color) int {
	d := barLuma(ground) - barLuma(ink)
	if d < 0 {
		d = -d
	}
	return d
}

// TestThePluginWindowBodyOutranksItsOwnChrome pins the first rung: the page's
// sentence must be heavier than the bar that names the page.
//
// The statement register is §4.7's (emphasis step, strong ink) — the same pair
// `settingsui.emptyState` draws — so the lower bound is 100 levels of contrast
// against the window's own ground, and the line must clear the bar title by at
// least 60.
func TestThePluginWindowBodyOutranksItsOwnChrome(t *testing.T) {
	en := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		for _, arm := range bodyArms(en) {
			if arm.name == "liststatus" {
				continue // the list's note is the counter-case, pinned below
			}
			r := bodyProbe(t, dark, arm)
			weight := bodyWeight(r.ground, r.ink)
			// The statement register is §4.7's contrast band; a fault is the
			// same register in the warning hue, whose luma is lower on purpose
			// (measured light 179 / dark 146), so it gets its own floor.
			floor := 100
			if arm.kind == detachedFault {
				floor = 130
			}
			if weight < floor {
				t.Errorf("dark=%v %s: the body line is not on the statement register (weight %d, want ≥ %d; ink %v on ground %v)",
					dark, arm.name, weight, floor, r.ink, r.ground)
			}
			if r.box.H < r.title.H {
				t.Errorf("dark=%v %s: the body line is smaller than the bar title (%.1f vs %.1f)",
					dark, arm.name, r.box.H, r.title.H)
			}
			// The chrome stays where it was: the title is still the muted
			// caption, and the sentence clears it. The in-flight line is the
			// middle rung and clears it by less — that is the ladder working.
			clear := weight - bodyWeight(r.ground, r.titleInk)
			floor = 60
			if arm.kind == detachedInFlight {
				floor = 10
			}
			if arm.kind == detachedFault {
				floor = 25
			}
			if clear < floor {
				t.Errorf("dark=%v %s: the body line does not outrank the bar title (by %d, want ≥ %d; body %v, title %v)",
					dark, arm.name, clear, floor, r.ink, r.titleInk)
			}
		}
	}
}

// TestTheInFlightLineIsOneRungBelowTheStatement pins the middle rung: "Running…"
// is work in progress, not the page's conclusion. It sits one step under the
// statement (secondary ink) and never outranks it.
func TestTheInFlightLineIsOneRungBelowTheStatement(t *testing.T) {
	en := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		running := bodyProbe(t, dark, bodyArms(en)[0])
		empty := bodyProbe(t, dark, bodyArms(en)[2])
		gap := bodyWeight(empty.ground, empty.ink) - bodyWeight(running.ground, running.ink)
		if gap < 40 {
			t.Errorf("dark=%v: the in-flight line is not one rung below the statement (gap %d, want ≥ 40; running %v, statement %v)",
				dark, gap, running.ink, empty.ink)
		}
		if gap > 140 {
			t.Errorf("dark=%v: the in-flight line fell off the ladder (gap %d, want ≤ 140)", dark, gap)
		}
		// Same register: the two lines are the same size.
		if running.box.H != empty.box.H {
			t.Errorf("dark=%v: the in-flight line is a different size than the statement (%.1f vs %.1f)",
				dark, running.box.H, empty.box.H)
		}
	}
}

// TestTheFaultLineKeepsTheWarningHue pins the third rung: a refused run is the
// page's conclusion too, so it takes the statement register — but its colour is
// the semantic warning hue, not the strong neutral ink (a fault is not "more
// important text", it is a different kind of sentence).
func TestTheFaultLineKeepsTheWarningHue(t *testing.T) {
	en := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		fault := bodyProbe(t, dark, bodyArms(en)[1])
		statement := bodyProbe(t, dark, bodyArms(en)[2])
		// Warmth: the warning ink's red channel runs ahead of its blue on both
		// appearances (measured light 156/31/24, dark 213/149/138).
		warmth := int(fault.ink.R) - int(fault.ink.B)
		if warmth < 25 {
			t.Errorf("dark=%v: the fault line lost the warning hue (r−b = %d, want ≥ 25; ink %v)",
				dark, warmth, fault.ink)
		}
		// The neutral statement is not warm (both appearances are within 8).
		if w := int(statement.ink.R) - int(statement.ink.B); w > 8 || w < -8 {
			t.Errorf("dark=%v: the statement line is not the neutral ink (r−b = %d)", dark, w)
		}
		// Same register: a fault is a conclusion, not a footnote.
		if fault.box.H != statement.box.H {
			t.Errorf("dark=%v: the fault line is not on the statement register (%.1f vs %.1f)",
				dark, fault.box.H, statement.box.H)
		}
	}
}

// TestTheListsOwnNoteStaysUnderTheStatement pins the counter-case: a status row
// **inside the list** is a note about the list, not this page's sentence. It
// keeps the reference's caption/muted register (`launcher.css:1783-1789`),
// so raising the body's statements must not drag it along.
func TestTheListsOwnNoteStaysUnderTheStatement(t *testing.T) {
	en := i18n.For("en").Launcher
	arms := bodyArms(en)
	for _, dark := range []bool{false, true} {
		note := bodyProbe(t, dark, arms[4])
		statement := bodyProbe(t, dark, arms[2])
		gap := bodyWeight(statement.ground, statement.ink) - bodyWeight(note.ground, note.ink)
		if gap < 60 {
			t.Errorf("dark=%v: the list's note rose to the statement register (gap %d, want ≥ 60; note %v, statement %v)",
				dark, gap, note.ink, statement.ink)
		}
		// And it is still the caption's box, not the statement's.
		if note.box.H >= statement.box.H {
			t.Errorf("dark=%v: the list's note grew to the statement's size (%.1f vs %.1f)",
				dark, note.box.H, statement.box.H)
		}
		// The bar title stays where it was: the chrome is the quietest thing.
		if note.box.H != note.title.H {
			t.Errorf("dark=%v: the list's note left the chrome's register (%.1f vs title %.1f)",
				dark, note.box.H, note.title.H)
		}
	}
}

// TestTheDetachedStateLadderIsThreeRungs is the structural half: the body's
// four lines go through one function that takes a kind, and the list's note
// through a different one. A rewrite that folds the two back into one helper
// would re-flatten the ladder without any pixel changing on the arms this
// round did not touch.
func TestTheDetachedStateLadderIsThreeRungs(t *testing.T) {
	data, err := os.ReadFile("detach.go")
	if err != nil {
		t.Fatalf("read detach.go: %v", err)
	}
	src := string(data)
	for _, want := range []string{"func detachedStateLine(", "func detachedListNote(", "detachedStatement", "detachedInFlight", "detachedFault"} {
		if !strings.Contains(src, want) {
			t.Errorf("the ladder's %q is missing from detach.go", want)
		}
	}
	// Every body arm names its rung; none of them calls the list's note.
	for _, want := range []string{
		"detachedStateLine(c, copy.OutputWindowRunning, detachedInFlight",
		"detachedStateLine(c, detached.failed, detachedFault",
		"detachedStateLine(c, copy.OutputWindowEmpty, detachedStatement",
		"detachedStateLine(c, copy.OutputWindowIdle, detachedStatement",
		"detachedListNote(c, row.Title",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the body does not draw %q", want)
		}
	}
}

// TestDumpThePluginWindowBodyArms writes both appearances × the five arms so a
// report can show them side by side. Opt-in.
func TestDumpThePluginWindowBodyArms(t *testing.T) {
	if os.Getenv("FLOTER_P13BE_DUMP") == "" {
		t.Skip("set FLOTER_P13BE_DUMP=1 to dump the plugin window's body arms")
	}
	tag := os.Getenv("FLOTER_P13BE_TAG")
	if tag == "" {
		tag = "after"
	}
	en := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, arm := range bodyArms(en) {
			a := barApp(t, dark)
			detached := &detachedOutput{
				title: "jv", running: arm.running, failed: arm.failed, ran: arm.ran, view: arm.view,
			}
			tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
			tt.SetDark(dark)
			tt.Frame()
			name := filepath.Join("/tmp", fmt.Sprintf("floter-P13-be-pluginwindow-%s-%s-%s.png", appearance, arm.name, tag))
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
			fmt.Printf("DUMP %s\n", name)
		}
	}
}
