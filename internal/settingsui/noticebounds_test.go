package settingsui

import (
	"fmt"
	"strings"
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNoticeBandKeepsBothInsets(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, danger := range []bool{false, true} {
			for _, width := range []int{420, 720} {
				t.Run(fmt.Sprintf("%s/danger=%v/%d", appearanceName(dark), danger, width), func(t *testing.T) {
					a := noticeApp(t, dark)
					var ground ui.Color
					var margin float32
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(a.tokens(c).Theme)
						ground, margin = c.Theme().Background, c.Theme().Space(3)
						c.Root().Background(ground)
						ui.Column(c).FillWidth().Padding(24).Children(func() {
							a.noticeBand(c, danger, "Permission denied; close the process and retry", func(*ui.Element) {})
						})
					}, width, 180)
					tt.SetDark(dark)
					tt.SetPreferences(ui.Preferences{ReduceMotion: true})
					tt.Frame()
					tt.Frame()
					name := fmt.Sprintf("band-%v-%d", danger, width)
					noticeDump(t, tt, name, dark)
					box := paintedBox(tt.Image(), ground)
					left, right := box.X-24, float32(width)-24-box.X-box.W
					t.Logf("band=%+v left=%.2f right=%.2f margin=%.2f", box, left, right, margin)
					if absFloat(left-margin) > 1 || absFloat(right-margin) > 1 {
						t.Errorf("notice escaped symmetric gutters: left=%.2f right=%.2f want=%.2f", left, right, margin)
					}
				})
			}
		}
	}
}

func TestNoticeLongSentencePreservesAnswers(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, width := range []int{420, 720} {
			t.Run(fmt.Sprintf("%s/%d", appearanceName(dark), width), func(t *testing.T) {
				a := noticeApp(t, dark)
				sentence := "Stop the integration?"
				var cancel, confirm ui.Handle
				var cancelBox, confirmBox ui.Rect
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					c.Root().Background(c.Theme().Background)
					ui.Column(c).FillWidth().Padding(24).Children(func() {
						a.noticeBand(c, true, sentence, func(_ *ui.Element) {
							a.barButton(c, "Cancel", false, false, nil, func() {}).Bind(&cancel)
							a.barButton(c, "Remove package", true, false, mustGlyph("trash"), func() {}).Bind(&confirm)
						})
					})
					cancelBox, confirmBox = c.Resolve(cancel).Bounds(), c.Resolve(confirm).Bounds()
				}, width, 580)
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Frame()
				shortCancel, shortConfirm := cancelBox, confirmBox
				sentence = strings.Repeat("The configuration directory is locked by another process. ", 5)
				tt.Frame()
				tt.Frame()
				noticeDump(t, tt, fmt.Sprintf("long-notice-%d", width), dark)
				t.Logf("short cancel/confirm=%+v / %+v long=%+v / %+v", shortCancel, shortConfirm, cancelBox, confirmBox)
				if cancelBox.W != shortCancel.W || confirmBox.W != shortConfirm.W {
					t.Errorf("long sentence squeezed answers: cancel %.2f→%.2f confirm %.2f→%.2f", shortCancel.W, cancelBox.W, shortConfirm.W, confirmBox.W)
				}
				text, ok := tt.Find(sentence)
				if !ok || text.X+text.W >= cancelBox.X || confirmBox.X+confirmBox.W > float32(width)-24 {
					t.Errorf("sentence or answers overlap/escape: text=%+v cancel=%+v confirm=%+v", text, cancelBox, confirmBox)
				}
			})
		}
	}
}

func TestNoticeDisableFailureUsesErrorInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			integration := Integration{ID: "local", Name: "Workspace Tools"}
			var ground ui.Color
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				ground = c.Theme().Background
				c.Root().Background(ground)
				ui.Column(c).FillWidth().Padding(24).Children(func() {
					a.disableConfirmation(c, i18n.For("en").Settings, integration)
				})
			}, 720, 220)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			warmth := func() int {
				img := tt.Image()
				box := paintedBox(img, ground)
				return lensWarmth(lensMean(img, int(box.X+box.W*0.25), int(box.Y)+1, int(box.X+box.W*0.75), int(box.Y)+1))
			}
			if got := warmth(); got > 12 {
				t.Errorf("ordinary disable question has warning ring: %d", got)
			}
			a.disableError = map[string]string{"local": "Permission denied; close the process and retry"}
			tt.Frame()
			tt.Frame()
			noticeDump(t, tt, "disable-error", dark)
			if got := warmth(); got < 25 {
				t.Errorf("failed disable still wears neutral question ink: ring warmth=%d", got)
			}
			delete(a.disableError, "local")
			tt.Frame()
			tt.Frame()
			if got := warmth(); got > 12 {
				t.Errorf("retry kept the stale warning ring: %d", got)
			}
		})
	}
}
