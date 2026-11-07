package shell

import (
	"strings"
	"testing"
)

func TestNoticeLongPathKeepsDismissReachable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a := newApp(t)
			message := "open /tmp/" + strings.Repeat("workspace", 12) + "/export.json: is a directory"
			a.queueNotice(message, refusalNotice)
			tt := transferFeedbackFrame(t, a, "long-path", dark)
			configTargetDump(t, tt, "long-path", map[bool]string{false: "light", true: "dark"}[dark])
			body, ok := tt.Find(message)
			if !ok {
				t.Fatal("full filesystem cause missing")
			}
			close, ok := tt.Find(a.SettingsCopy().DismissNotice)
			if !ok {
				t.Fatal("dismiss missing")
			}
			if close.X+close.W > 720 || body.X+body.W > close.X {
				t.Errorf("long path overlaps dismiss or viewport: body=%v dismiss=%v", body, close)
			}
			if err := tt.Click(a.SettingsCopy().DismissNotice); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if tt.HasText(message) {
				t.Error("long path notice could not be dismissed")
			}
		})
	}
}
