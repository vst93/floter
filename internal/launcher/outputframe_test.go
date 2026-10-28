package launcher

import (
	"image/color"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P11-aq · **输出面板的标题带占自己的位置，正文排在它下面。**
//
// The captured-output panel draws a strip — the command's title, its two
// controls and the status line — above the text (or the rows) that scroll. The
// strip used to be an absolute layer (`Absolute().Top(edge)`) whose height the
// body only guessed at: it reserved `edge + t.Space(4)` = 76u for a strip that
// is 60u + 42u tall, so the status line was painted on top of the list's first
// group heading and on top of the text page's first line, and the pointer never
// reached the runs in that 26u band (a visible line nobody could select).
//
// P11-aj's audit (`auditLauncherFrame`) is what catches the first half: it is
// the same "no two runs on the same pixels, no run outside the window" rule the
// launcher's own pages are held to, applied to this surface. The second half is
// the pointer: the band the strip occupies must be the strip's, and everything
// below it must belong to the body.
func TestTheOutputPanelPaintsNoTextOverText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		// §1 the list page: a group heading sits in the box the strip used to
		// print through.
		a := testApp()
		lt := runListOutput(t, a, `[{"id":"r0","title":"Alpha","group":"Group A"},`+
			`{"id":"r1","title":"Beta","group":"Group A"},`+
			`{"id":"r2","title":"Gamma","group":"Group B"}]`)
		lt.SetDark(dark)
		lt.Frame()
		auditLauncherFrame(t, appearance+" output list", lt)

		// §2 the text page: the same strip over a block of text.
		b := testApp()
		txt := runListOutput(t, b, longOutput(80))
		txt.SetDark(dark)
		txt.Frame()
		auditLauncherFrame(t, appearance+" output text", txt)
	}
}

// The strip's own band is the strip's: the body's first box starts at or below
// the status line's bottom edge, the pixels just above it belong to nothing
// (the default cursor, not a text I-beam), and the first line of the body is
// selectable exactly where it is drawn.
func TestTheOutputBodyStartsBelowTheStrip(t *testing.T) {
	cases := []struct {
		name   string
		stdout func() string
		body   func(tt *ui.Tester, a *App) (ui.Rect, bool)
	}{
		{
			name:   "list",
			stdout: func() string { return `[{"id":"r0","title":"Alpha","group":"Group A"},{"id":"r1","title":"Beta"}]` },
			body: func(tt *ui.Tester, _ *App) (ui.Rect, bool) {
				return tt.Find("Alpha")
			},
		},
		{
			name:   "text",
			stdout: func() string { return longOutput(80) },
			// The body is the one run that is not the strip's: the title and the
			// status line are the two short boxes above it, the text block is the
			// tallest box in the frame.
			body: func(tt *ui.Tester, _ *App) (ui.Rect, bool) {
				var body ui.Rect
				for _, s := range tt.Texts() {
					if r, ok := tt.Find(s); ok && r.H > body.H {
						body = r
					}
				}
				return body, body.H > 0
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			tt := runListOutput(t, a, tc.stdout())
			title := mustFind(t, tt, a.output.Title)
			status := mustFind(t, tt, a.output.Status+"  \u00b7  "+a.outputHint(a.copy(), a.output))
			stripBottom := status.Y + status.H
			bodyRect, ok := tc.body(tt, a)
			if !ok || bodyRect.W <= 0 {
				t.Fatalf("the body's first box is not in the frame: %q", tt.Texts())
			}
			if bodyRect.Y < stripBottom-0.5 {
				t.Errorf("the body starts at y=%.1f, the strip ends at y=%.1f: "+
					"%.1fu of the body is painted under the title/status line "+
					"(strip %.1f..%.1f, body starts %.1f)",
					bodyRect.Y, stripBottom, stripBottom-bodyRect.Y,
					title.Y, stripBottom, bodyRect.Y)
			}

			// The band between the two is the strip's alone: nothing selectable
			// is under it, so the pointer is the default arrow there.
			bandY := title.Y + (stripBottom-title.Y)/2
			tt.Move(InputWindowWidth/6, bandY)
			tt.Frame()
			if got := tt.Cursor(); got != ui.CursorDefault {
				t.Errorf("y=%.1f is inside the strip but the pointer is %v, not the default arrow", bandY, got)
			}

			// And the body's first line is reachable where it is drawn.
			bodyY := bodyRect.Y + bodyRect.H/2
			tt.Move(InputWindowWidth/6, bodyY)
			tt.Frame()
			want := ui.CursorText
			if tc.name == "list" {
				want = ui.CursorDefault
			}
			if got := tt.Cursor(); got != want {
				t.Errorf("y=%.1f is the body's first line but the pointer is %v, want %v", bodyY, got, want)
			}

		})
	}
}

// The P11-aq visual evidence: the output panel's strip and the body below it,
// list page and text page, both appearances.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11AQOutputStripPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		backdrop := color.RGBA{R: 0xDF, G: 0xDF, B: 0xE4, A: 0xFF}
		if dark {
			appearance = "dark"
			backdrop = color.RGBA{R: 0x1C, G: 0x1C, B: 0x20, A: 0xFF}
		}

		a := testApp()
		lt := runListOutput(t, a, `[{"id":"r0","title":"Alpha","group":"Group A"},`+
			`{"id":"r1","title":"Beta","group":"Group A"},`+
			`{"id":"r2","title":"Gamma","group":"Group B"}]`)
		lt.SetDark(dark)
		lt.Frame()
		writeDump(t, "floter-P11-aq-output-list-"+appearance+".png", lt, backdrop)

		b := testApp()
		txt := runListOutput(t, b, longOutput(80))
		txt.SetDark(dark)
		txt.Frame()
		writeDump(t, "floter-P11-aq-output-text-"+appearance+".png", txt, backdrop)
	}
}
