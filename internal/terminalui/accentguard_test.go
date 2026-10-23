package terminalui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

// P8-e §2 守卫 1：`internal/shell` 与 `internal/terminalui` 这两片的生产代码里，
// 两个曾经花过 accent 的文件不得再出现 `Accent`——注释除外（注释里记的是旧值
// 与合约条文，那正是要留下的证据）。
//
// 与 `TestTheTerminalViewsPaintFromTokensNotLiterals` 同族：那一条守「不要写
// 字面色」，这一条守「不要花 accent」。两片收口后 §5 的判据第一次全仓一致，
// 这条守卫防的是下一轮把它们悄悄改回去。
func TestTheTerminalAndShellViewsSpendNoAccent(t *testing.T) {
	files := []string{
		"terminalui.go", // internal/terminalui
		filepath.Join("..", "shell", "notices.go"), // internal/shell
	}
	checked := 0
	for _, name := range files {
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("could not read %s: %v", name, err)
		}
		checked++
		for i, line := range strings.Split(string(data), "\n") {
			// A comment may quote the old value and the contract article that
			// retired it: that is documentation, not a painted token.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, "Accent") {
				t.Errorf("%s:%d reads an accent token (%q); §5 lets the accent answer only "+
					"「哪个被选中 / 哪个是开的」, and neither of these files draws one",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
	if checked != len(files) {
		t.Fatalf("the guard read %d of %d files", checked, len(files))
	}
}

// P8-e §2 守卫 2（正面 + 反面成对）：终端状态灯的点**不带彩度**（通道差 ≤ 12），
// 而**呼吸仍在**——存活由「动」说，不由色相说。
//
// 正面：活着的点在两套外观下都是一个灰，通道差不超过 12 级（accent 是
// `#F5544E`，R 比 B 高 152 级，任何一处 accent 都会把这个数撑破）。
// 反面：它**不是** accent——把 `dot.Background` 改回 `t.Accent`（mutation M1）
// 这一条就红。
// 呼吸：同一个点在 0% 与 50% 关键帧之间的墨量必须变化（阈值 500，实测
// 14816 → 5732，两套外观同值）。点的大小只有 7px，呼吸若停了这个差是 0。
func TestTheStatusDotIsNeutralAndStillBreathes(t *testing.T) {
	for _, reduce := range []bool{true, false} {
		live := New(newStore(), Actions{}, nil)
		withSession(live, &terminal.Terminal{})
		tt, tok, _ := barFrame(t, live, reduce)
		minX, maxX, mass := barInkBox(tt, 0, 22, 40)
		if maxX < 0 {
			t.Fatalf("reduce=%v: the live bar drew no status dot: %q", reduce, tt.Texts())
		}
		centre := dotCenter(tt, minX, maxX)
		spread := maxInt(
			absInt(int(centre.R)-int(centre.G)),
			absInt(int(centre.G)-int(centre.B)),
			absInt(int(centre.R)-int(centre.B)),
		)
		if spread > 12 {
			t.Errorf("reduce=%v: the live dot's centre is %v, whose channels differ by %d levels — "+
				"the dot answers 「这个进程是活的」, which is not one of §5's four", reduce, centre, spread)
		}
		// The reverse half: it is not the accent, in either appearance.
		if sameRGB(centre, tok.Theme.Accent) {
			t.Errorf("reduce=%v: the live dot is the accent %v; a running process is not a chosen row",
				reduce, tok.Theme.Accent)
		}
		if reduce {
			// The 0% keyframe: full opacity, the tertiary step exactly.
			if want := flatColor(tok, tok.TextTertiary, 1); !sameRGB(centre, want) {
				t.Errorf("the resting dot is %v, want the tertiary step at full opacity (%v)", centre, want)
			}
			continue
		}
		// The breath: one mid-cycle frame later the same dot's ink has faded and
		// shrunk, so the region's ink mass moves. A dot that stopped breathing
		// reads the same number twice.
		tt.Advance(1200 * time.Millisecond) // the 50% keyframe of the 2.4 s loop
		tt.Frame()
		_, _, later := barInkBox(tt, 0, 22, 40)
		if diff := absInt(later - mass); diff < 500 {
			t.Errorf("the live dot's ink mass is %d at the 0%% keyframe and %d at the 50%% one "+
				"(a difference of %d): the breath is not running, so a neutral dot says nothing at all",
				mass, later, diff)
		}
	}
}

// P8-e §2 守卫 2 的另一半：exited 态仍是灰点，与活着的点同档墨、只差 0.7 的
// 静止不透明——「结束了」也不借色相说。
func TestTheExitedStatusDotStaysAGreyDot(t *testing.T) {
	held := New(newStore(), Actions{}, nil)
	withSession(held, &terminal.Terminal{})
	held.Current().Resident, held.Current().ExitCode = true, 0
	tt, tok, _ := barFrame(t, held, true)
	minX, maxX, _ := barInkBox(tt, 0, 22, 40)
	if maxX < 0 {
		t.Fatalf("the held session's bar drew no status dot: %q", tt.Texts())
	}
	centre := dotCenter(tt, minX, maxX)
	spread := maxInt(
		absInt(int(centre.R)-int(centre.G)),
		absInt(int(centre.G)-int(centre.B)),
		absInt(int(centre.R)-int(centre.B)),
	)
	if spread > 12 {
		t.Errorf("the exited dot's centre is %v, whose channels differ by %d levels: still a hue, not a grey dot",
			centre, spread)
	}
	if want := flatColor(tok, tok.TextTertiary, 0.7); !sameRGB(centre, want) {
		t.Errorf("the exited dot is %v, want --text-tertiary at 0.7 (%v)", centre, want)
	}
}

// maxInt is the largest of the arguments: the channel-spread ruler above.
func maxInt(vs ...int) int {
	out := vs[0]
	for _, v := range vs[1:] {
		if v > out {
			out = v
		}
	}
	return out
}
