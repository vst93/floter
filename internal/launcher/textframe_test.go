package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/frameaudit"
	"floter/internal/settings"
)

// P11-aj · **没有两段字压在同一块像素上，也没有一段字跑出窗口。**
//
// The launcher's frame is built from absolute bands (the field row floats over
// the list, the footer is pinned at the panel's bottom edge) and from text
// whose width is whatever the row had left. Both are the two ways a text
// surface goes wrong without any test noticing: a run that is one row too wide
// runs into the trailing keycap, and a band whose box the layout never asked
// for puts its words outside the window where nobody sees them. Neither is
// visible in the text tree — `Tester.Texts` reports every run the tree holds,
// on screen or not — so both are read here off the boxes the frame registered
// (`Tester.Find`), which is the geometry the framework hit-tests and paints.
//
// What counts as a violation:
//   - two runs whose boxes overlap by more than one DIP in both axes, unless
//     one box contains the other (a button's label inside the button is one
//     run on one face, not two words on top of each other), and
//   - a run whose box leaves the window it was painted into.
//
// P11-by：**整宽箱子不再跳过**。那句 `if r.W >= w-1 { continue }` 当初的理由是
// 「整宽箱子是祖先，按构造包含每一个 run」——但那正是 `Encloses` 已经豁免的关系：
// 实测三面（启动器 / 设置页 / 抽屉条带）今日的整宽箱子**全是祖先**（面板自己的
// label、列表的 label、条带的 label），所以删掉跳过对今日的帧是行为中性的；
// 它真正丢掉的是**整宽叶子**（横跨面板的一条横幅），而那种东西一旦出现就没人量。
// 规则本身只留一份，在 `internal/frameaudit`；这里只转发。
func auditLauncherFrame(t *testing.T, tag string, tt *ui.Tester) {
	t.Helper()
	frameaudit.Audit(t, tag, tt, nil)
}

// The three pages the launcher draws from the same card — the empty page with
// its recents, a query with matches and a trailing badge, and a query that
// matched nothing — in both appearances, and in both shipped languages. A row
// long enough to be ellipsized is in the fixture on purpose: that is where a
// text run and a trailing keycap meet.
//
// P11-ba: the audit used to run in English only. The user reads the Chinese
// surface, and the two languages put different-length words in the same boxes
// (a Chinese group label is four wide glyphs where the English one is two
// words), so the same rule has to hold on both. Measured clean on both before
// this change; the arms below keep it that way.
func TestTheLauncherPaintsNoTextOverText(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			t.Run(lang+" "+appearance, func(t *testing.T) {
				a := testApp()
				if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
					t.Fatal(err)
				}
				a.SetTools([]apps.App{
					{Name: "git", Path: "/usr/bin/git"},
					{Name: "Supercalifragilisticexpialidocious-Application-Launcher-With-A-Very-Long-Name", Path: "/usr/bin/long"},
				}, true)
				tt := render(t, a)
				tt.SetDark(dark)
				tt.Frame()
				auditLauncherFrame(t, appearance+" "+lang+" empty", tt)
				tt.Type("git")
				tt.Frame()
				auditLauncherFrame(t, appearance+" "+lang+" match", tt)
				tt.Type("zzzz")
				tt.Frame()
				auditLauncherFrame(t, appearance+" "+lang+" no results", tt)
			})
		}
	}
}
