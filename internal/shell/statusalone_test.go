package shell

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
)

// P13-bu · 插件窗口列表里的状态行：「列表的从属注」还是「这一页的结论」
// （`docs/rounds/P13-bu-spec.md`、`docs/liquid-glass.md` §6.39）。
//
// 先量的数（720×460 静息、窗口自己的地面 `tokens.Opaque` 浅 246 / 深 20，
// 与本包 `stateladder_test.go` 同一把尺）：
//
//	列表里**只有**一条状态行      Caption + TextMuted   浅 131（对地面 115） / 深 130（110）
//	状态行旁边有一条行盘           Caption + TextMuted   同上（这一档是对的）
//	这一页的陈述（`empty` 臂）     Emphasis + TextStrong 浅 216 / 深 214（P13-be 实测）
//	bar 标题                      Caption + TextMuted   浅 131 / 深 130
//
// 第一行与第四行逐级相同：一条「没有匹配」的插件列表，窗口上最轻的东西就是它唯一的那句话。
// 这一轮把**判据**从行搬到列表：旁边有没有别的行，决定这句话是哪一档。

const (
	// statusAloneTitle is the status line every arm below carries. It is the
	// text `bodyArmText`'s default branch looks for, so `bodyProbe` (the
	// measurement harness this file reuses from `stateladder_test.go`) can find
	// it in any of the three arms.
	statusAloneTitle = "3 of 4 shown"
	// statusNeighbour is a plain list row — a door, not a note. Its presence is
	// the whole input to the predicate under test.
	statusNeighbour = "row one"
)

// statusAloneArm is the shape the defect is about: a plugin list whose only
// content is its own status line.
func statusAloneArm() detachedArm {
	return detachedArm{
		name: "statusalone",
		ran:  true,
		view: launcher.OutputView{Title: "jv", IsList: true,
			Rows: []extensions.Row{{Status: true, Title: statusAloneTitle}}},
	}
}

// statusNeighbourArm is the same status line with one ordinary row beside it.
// `statusFirst` puts the note above the row instead of under it, so the
// predicate cannot be satisfied by looking at the first row only (mutation M3).
func statusNeighbourArm(statusFirst bool) detachedArm {
	rows := []extensions.Row{{Title: statusNeighbour}, {Status: true, Title: statusAloneTitle}}
	if statusFirst {
		rows = []extensions.Row{{Status: true, Title: statusAloneTitle}, {Title: statusNeighbour}}
	}
	return detachedArm{
		name: "statusneighbour",
		ran:  true,
		view: launcher.OutputView{Title: "jv", IsList: true, Rows: rows},
	}
}

// statementArm is the page's own statement line (`empty`), the rung a lone
// status row has to reach.
func statementArm(en i18n.Launcher) detachedArm {
	return bodyArms(en)[2]
}

// TestALoneStatusRowIsThePagesStatement pins the first half of §6.39: when the
// list has nothing but status rows, that line **is** the page's conclusion and
// takes the statement register.
//
// Measured before this round (see the header): the lone line sat at the bar
// title's own level — 115 / 110 against the ground, identical to the chrome.
// Mutation M1 (`alone` pinned false) turns the first assertion red.
func TestALoneStatusRowIsThePagesStatement(t *testing.T) {
	en := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		alone := bodyProbe(t, dark, statusAloneArm())
		stmt := bodyProbe(t, dark, statementArm(en))
		weight := bodyWeight(alone.ground, alone.ink)
		if weight < 100 {
			t.Errorf("dark=%v: the lone status row is not on the statement register (weight %d, want ≥ 100; ink %v on ground %v)",
				dark, weight, alone.ink, alone.ground)
		}
		// The chrome stays where it was: the sentence must clear the bar title.
		if clear := weight - bodyWeight(alone.ground, alone.titleInk); clear < 30 {
			t.Errorf("dark=%v: the lone status row does not outrank the bar title (by %d, want ≥ 30; line %v, title %v)",
				dark, clear, alone.ink, alone.titleInk)
		}
		// Same register as the page's own statement: one step, not a new one.
		if alone.box.H != stmt.box.H {
			t.Errorf("dark=%v: the lone status row is not the statement's size (%.1f vs %.1f)",
				dark, alone.box.H, stmt.box.H)
		}
		// And it really did move off the note register, not onto a third one.
		note := bodyProbe(t, dark, statusNeighbourArm(false))
		if gap := weight - bodyWeight(note.ground, note.ink); gap < 30 {
			t.Errorf("dark=%v: the lone status row did not leave the note register (gap %d, want ≥ 30; lone %v, note %v)",
				dark, gap, alone.ink, note.ink)
		}
		// The round's own before/after pair, printed so a report can quote a
		// measured number instead of re-deriving it: ink and weight, both
		// appearances, next to the note it left and the statement it joined.
		t.Logf("dark=%v lone ink %v (weight %d) box %.1f | note ink %v (weight %d) | statement ink %v (weight %d) box %.1f | bar title %v (weight %d)",
			dark, alone.ink, weight, alone.box.H,
			note.ink, bodyWeight(note.ground, note.ink),
			stmt.ink, bodyWeight(stmt.ground, stmt.ink), stmt.box.H,
			alone.titleInk, bodyWeight(alone.ground, alone.titleInk))
	}
}

// TestTheStatusRowIsANoteWhenTheListHasRowsBesideIt pins the second half: with
// an ordinary row beside it the same line stays the reference's muted caption
// (`launcher.css:1783-1789`) — the list's own subject is the plate, not this
// sentence. Both orders are measured, because the predicate reads the whole
// list (mutation M3 makes only the first one pass).
func TestTheStatusRowIsANoteWhenTheListHasRowsBesideIt(t *testing.T) {
	en := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		stmt := bodyProbe(t, dark, statementArm(en))
		for _, statusFirst := range []bool{false, true} {
			note := bodyProbe(t, dark, statusNeighbourArm(statusFirst))
			gap := bodyWeight(stmt.ground, stmt.ink) - bodyWeight(note.ground, note.ink)
			if gap < 30 {
				t.Errorf("dark=%v statusFirst=%v: the list's note rose to the statement register (gap %d, want ≥ 30; note %v, statement %v)",
					dark, statusFirst, gap, note.ink, stmt.ink)
			}
			if note.box.H >= stmt.box.H {
				t.Errorf("dark=%v statusFirst=%v: the list's note grew to the statement's size (%.1f vs %.1f)",
					dark, statusFirst, note.box.H, stmt.box.H)
			}
		}
	}
}

// TestDumpP13BU writes the two arms in both appearances, for the round's own
// before/after pair. Opt-in: it writes files and asserts nothing.
func TestDumpP13BU(t *testing.T) {
	if os.Getenv("FLOTER_P13BU_DUMP") == "" {
		t.Skip("opt-in: set FLOTER_P13BU_DUMP=1 to dump the plugin window's list arms")
	}
	dir := os.Getenv("FLOTER_P13BU_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	arms := []struct {
		name string
		arm  detachedArm
	}{{"alone", statusAloneArm()}, {"note", statusNeighbourArm(false)}}
	for _, dark := range []bool{false, true} {
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		for _, arm := range arms {
			a := barApp(t, dark)
			detached := &detachedOutput{title: "jv", ran: true, view: arm.arm.view}
			tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
			tt.SetDark(dark)
			tt.Frame()
			name := filepath.Join(dir, "floter-P13-bu-window-"+arm.name+"-"+suffix+".png")
			f, err := os.Create(name)
			if err != nil {
				t.Fatalf("create %s: %v", name, err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatalf("encode %s: %v", name, err)
			}
			f.Close()
			t.Logf("wrote %s", name)
		}
	}
}
