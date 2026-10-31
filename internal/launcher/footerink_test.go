package launcher

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// P13-bj（`docs/liquid-glass.md` §6.29）：分页脚注的两个状态此前**一档墨**。
//
// 这一组 pin 量的是**页面画出来的像素**（`ui.Tester`，装上 app 主题，与
// `shell.View` 的生产路径一致），不是 token，也不是测试自己拼的颜色——
// P13-bh 的教训：pin 必须量页面画的，否则产品改错也绿。
//
// 实测（本轮，对地面峰值差 / 墨质量）：
//
//	              浅                    深
//	在飞（加载词） 139 / 21.14          139 / 21.14
//	到底了（结尾词）118 / 15.81         118 / 15.81
//	环的活动象限   139                  139
//
// 改动前两者都是 118 / 15.81，差为 0。

const (
	// footerLiveFloor and footerEndCeiling are the two rungs as measured on
	// this machine: the live word peaks at 139 over the plane, the end word at
	// 118. The floor/ceiling are the values the ladder has to keep, not the
	// exact pixels: a 12-level gap is the contract's own "one visible step"
	// (§1), and the end word's ceiling is its unchanged rest rung plus the
	// tester's own jitter.
	footerLiveFloor   = 130
	footerEndCeiling  = 122
	footerMassRatio   = 1.25
	footerRingSlack   = 4
	footerProbeMargin = 4
)

// footerFrame draws the captured-output page on a self-paging list and leaves
// the footer in the asked-for state: `loading` (the fetch never comes back) or
// `end` (the plugin says there is nothing more).
func footerFrame(t *testing.T, state string, dark bool) (*ui.Tester, *App) {
	t.Helper()
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "paged", Name: "Paged"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	first := `{"output":` + listOutput(12) + `,"page":{"cursor":"c1","hasMore":true}}`
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: first}, nil)
	}
	if state == footerLoading {
		// The fetch never comes back: the footer stays in its live state.
		a.Actions.LoadOutputPage = func(extensions.CommandEntry, []string, string, func(extensions.CapturedRun, error)) {}
	} else {
		a.Actions.LoadOutputPage = func(_ extensions.CommandEntry, _ []string, _ string, done func(extensions.CapturedRun, error)) {
			done(extensions.CapturedRun{Command: []string{"tool"}, Success: true,
				Stdout: `{"output":[],"page":{"cursor":"","hasMore":false}}`}, nil)
		}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		// The app's own palette, installed the way `shell.View` installs it:
		// the launcher reads `t.TextMuted` off the context theme, so a frame
		// drawn without this is drawn in the framework's stock inks.
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	tt.SetDark(dark)
	tt.Frame()
	a.Query = "paged"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	a.loadMoreOutputPage()
	tt.Frame()
	if got := outputFooterState(a.output); got != state {
		t.Fatalf("the footer is in state %q, want %q", got, state)
	}
	return tt, a
}

// footerLuma is one pixel's brightness, the unit every "N 级" in the contract
// is counted in.
func footerLuma(r, g, b uint8) int {
	return (299*int(r) + 587*int(g) + 114*int(b)) / 1000
}

// footerBand reads a box's plane (the pixel just above it) and its peak ink.
func footerBand(tt *ui.Tester, box ui.Rect) (plane, peak int) {
	img := tt.Image()
	x0, y0 := int(box.X), int(box.Y)
	i := (y0-footerProbeMargin)*img.Stride + x0*4
	plane = footerLuma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
	for y := y0; y < int(box.Y+box.H); y++ {
		for x := x0; x < int(box.X+box.W); x++ {
			j := y*img.Stride + x*4
			d := footerLuma(img.Pix[j], img.Pix[j+1], img.Pix[j+2]) - plane
			if d < 0 {
				d = -d
			}
			if d > peak {
				peak = d
			}
		}
	}
	return plane, peak
}

// footerMass is a box's ink mass per unit area, in luma levels — how heavy the
// mark is rather than how dark its darkest pixel is. That is the only way a
// single word's weight shows up in pixels.
func footerMass(tt *ui.Tester, box ui.Rect) float64 {
	img := tt.Image()
	x0, y0 := int(box.X), int(box.Y)
	i := (y0-footerProbeMargin)*img.Stride + x0*4
	plane := footerLuma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
	sum := 0
	for y := y0; y < int(box.Y+box.H); y++ {
		for x := x0; x < int(box.X+box.W); x++ {
			j := y*img.Stride + x*4
			d := footerLuma(img.Pix[j], img.Pix[j+1], img.Pix[j+2]) - plane
			if d < 0 {
				d = -d
			}
			sum += d
		}
	}
	return float64(sum) / float64(box.W*box.H)
}

// footerWordBox is one footer word's box, failing the test when the word is not
// on the page.
func footerWordBox(t *testing.T, tt *ui.Tester, a *App, state string) ui.Rect {
	t.Helper()
	word := a.Copy().OutputLoading
	if state == footerEnd {
		word = a.Copy().OutputEnd
	}
	box, ok := tt.Find(word)
	if !ok {
		t.Fatalf("the %s footer word %q is not on the page: %q", state, word, tt.Texts())
	}
	return box
}

// The two words are two rungs: the live one carries the page's activity, the
// end one is the list's floor. Before P13-bj both peaked at 118 over the plane
// in both appearances — a byte-for-byte tie.
func TestThePagingFooterSeparatesTheLiveStateFromTheEnd(t *testing.T) {
	for _, dark := range []bool{false, true} {
		live, liveApp := footerFrame(t, footerLoading, dark)
		_, livePeak := footerBand(live, footerWordBox(t, live, liveApp, footerLoading))
		end, endApp := footerFrame(t, footerEnd, dark)
		_, endPeak := footerBand(end, footerWordBox(t, end, endApp, footerEnd))
		if livePeak < footerLiveFloor {
			t.Errorf("dark=%v: the live word peaks at %d over the plane, want ≥ %d — it is not in the active register",
				dark, livePeak, footerLiveFloor)
		}
		if endPeak > footerEndCeiling {
			t.Errorf("dark=%v: the end word peaks at %d over the plane, want ≤ %d — the list's floor left its rest rung",
				dark, endPeak, footerEndCeiling)
		}
		if gap := livePeak - endPeak; gap < 12 {
			t.Errorf("dark=%v: the live word and the end word are one ink again (gap %d, want ≥ 12)", dark, gap)
		}
	}
}

// The ring's moving quarter and the word are one object: a label darker than
// the thing it labels reads as two voices in one row.
func TestThePagingFooterSpinnerSharesTheLiveWordInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, a := footerFrame(t, footerLoading, dark)
		word := footerWordBox(t, tt, a, footerLoading)
		_, wordPeak := footerBand(tt, word)
		// The ring sits to the left of the word inside the row's own gap.
		ring := ui.Rect{X: word.X - 30, Y: word.Y, W: 24, H: word.H}
		_, ringPeak := footerBand(tt, ring)
		if ringPeak < wordPeak-footerRingSlack || ringPeak > wordPeak+footerRingSlack {
			t.Errorf("dark=%v: the ring peaks at %d, its word at %d — the pair is not one ink",
				dark, ringPeak, wordPeak)
		}
	}
}

// The live word also wears one step of weight: at the same font size the ink
// alone would be a colour difference, and this row is a label for a turning
// ring, not a statement. Weight is only visible as ink mass.
func TestThePagingFooterLiveWordIsTheHeavierOne(t *testing.T) {
	for _, dark := range []bool{false, true} {
		live, liveApp := footerFrame(t, footerLoading, dark)
		liveMass := footerMass(live, footerWordBox(t, live, liveApp, footerLoading))
		end, endApp := footerFrame(t, footerEnd, dark)
		endMass := footerMass(end, footerWordBox(t, end, endApp, footerEnd))
		if ratio := liveMass / endMass; ratio < footerMassRatio {
			t.Errorf("dark=%v: the live word is not the heavier one (mass ratio %.2f, want ≥ %.2f)",
				dark, ratio, footerMassRatio)
		}
	}
}

// TestDumpPagingFooter writes the two footer states, in both appearances, to
// /tmp for the round's own visual check. Opt-in: it is the only test here that
// needs a file system and a name the report quotes.
func TestDumpPagingFooter(t *testing.T) {
	if os.Getenv("FLOTER_P13BJ_DUMP") == "" {
		t.Skip("set FLOTER_P13BJ_DUMP=1 to dump the paging footer's two states")
	}
	tag := os.Getenv("FLOTER_P13BJ_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, state := range []string{footerLoading, footerEnd} {
			tt, _ := footerFrame(t, state, dark)
			name := fmt.Sprintf("/tmp/floter-P13-bj-footer-%s-%s-%s.png", appearance, state, tag)
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
