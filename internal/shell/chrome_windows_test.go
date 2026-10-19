//go:build windows

package shell

import "testing"

// The frame attributes are DWM's own numbers and a wrong one is silently
// ignored, so the values are pinned here: DWMWA_WINDOW_CORNER_PREFERENCE (33)
// with DWMWCP_DONOTROUND (1), and DWMWA_BORDER_COLOR (34) with
// DWMWA_COLOR_NONE (0xFFFFFFFE, the documented "no border" colour rather than
// black, which would draw a black line).
func TestTheWindowsFrameAttributesAreDwmsOwn(t *testing.T) {
	if dwmwaWindowCornerPreference != 33 {
		t.Errorf("DWMWA_WINDOW_CORNER_PREFERENCE = %d, want 33", dwmwaWindowCornerPreference)
	}
	if dwmcpDoNotRound != 1 {
		t.Errorf("DWMWCP_DONOTROUND = %d, want 1", dwmcpDoNotRound)
	}
	if dwmwaBorderColor != 34 {
		t.Errorf("DWMWA_BORDER_COLOR = %d, want 34", dwmwaBorderColor)
	}
	if dwmwaColorNone != 0xFFFFFFFE {
		t.Errorf("DWMWA_COLOR_NONE = %#x, want 0xFFFFFFFE", dwmwaColorNone)
	}
	// A zero handle is the "no window yet" case and must not reach DWM.
	dressWindowsFrame(0)
	stripSystemChrome(0)
}

// The caption bits are the ones the style is built from: WS_CAPTION is
// WS_BORDER|WS_DLGFRAME, and the three window-menu bits sit beside it.
func TestTheStrippedStyleBitsAreTheCaptionsOwn(t *testing.T) {
	if wsCaption != 0x00C00000 {
		t.Errorf("WS_CAPTION = %#x, want 0x00C00000", wsCaption)
	}
	if wsSysMenu != 0x00080000 || wsMinimizeBox != 0x00020000 || wsMaximizeBox != 0x00010000 {
		t.Errorf("window-menu bits = %#x %#x %#x, want 0x80000 0x20000 0x10000",
			wsSysMenu, wsMinimizeBox, wsMaximizeBox)
	}
}
