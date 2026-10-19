package shell

import (
	"strings"
	"testing"
)

// The geometry line has to answer the two reports it was written for.
//
// 「窗口周边外圈还有一圈额外的底色」 and 「搜索页面左右两边各有一大块空白」 have
// been answered by inference for eight rounds because the numbers never left
// the machine. The card's width is only one of the three that separate the
// candidates, so the line carries all of them: the frame the View drew into,
// the card it derived, the window's own client, the inset band, and whether the
// window has a material behind it at all (where it does not, the card is
// painted opaque over the whole window — see windowShowsMaterial, whose
// decision is the difference between 「no transparency」 and 「a ring of
// background」).
func TestTheGeometryLineCarriesWhatTheReportsNeed(t *testing.T) {
	line := geometryLine("windows", "launcher", 720, 700, 720, 480, false, 1, 2.5)
	for _, want := range []string{
		"surface=launcher", "os=windows",
		"frame=720.0", "card=700.0", "window=720x480",
		"inset.l=1.0", "inset.r=2.5", "material=false",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the geometry line is missing %q: %s", want, line)
		}
	}
	// The two cases a report has to tell apart read differently: a card inset
	// inside the frame, and a card that is the frame with no material at all.
	full := geometryLine("windows", "launcher", 720, 720, 720, 480, false, 0, 0)
	if strings.Contains(full, "card=700.0") || !strings.Contains(full, "material=false") {
		t.Errorf("the opaque full-window case is not distinguishable: %s", full)
	}
}
