//go:build windows

package shell

import "syscall"

// The style bits a frameless window must not carry on Windows.
//
// mygo's Windows backend builds every window's style from `WS_OVERLAPPEDWINDOW`
// (`internal/windows/window.go`, `styles()`) and never strips the caption for a
// frameless one: `WindowOptions.Frameless` only reaches the menu-bar and sizing
// paths there. A launcher created that way keeps `WS_CAPTION` (border + dialog
// frame), `WS_SYSMENU` and the minimize/maximize boxes, so Windows draws a
// non-client band around the card (the report 「窗口周边外圈还有一圈额外的
// 底色」) and, where the acrylic material extends the frame over the client
// area, the system's three caption buttons over the top-right corner of the
// content (「右上角还显示了系统自带的三大金刚按钮」). The app's own card is the
// whole of the window's chrome, so the bits come off here, once the window
// exists, and again whenever a surface change makes mygo re-add them.
const (
	wsCaption     = 0x00C00000 // WS_BORDER | WS_DLGFRAME
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
)

// gwlStyle is GetWindowLongPtr's GWL_STYLE index (-16): a variable, because a
// negative constant does not convert to uintptr.
var gwlStyle int32 = -16

var (
	user32DLL            = syscall.NewLazyDLL("user32.dll")
	procGetWindowLongPtr = user32DLL.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr = user32DLL.NewProc("SetWindowLongPtrW")
	procGetWindowLong    = user32DLL.NewProc("GetWindowLongW")
	procSetWindowLong    = user32DLL.NewProc("SetWindowLongW")
	procSetWindowPos     = user32DLL.NewProc("SetWindowPos")
)

// windowStyle reads the window's style, through the pointer-width entry point
// where the system has one (64-bit) and the plain one where it does not.
func windowStyle(hwnd uintptr) uintptr {
	if err := procGetWindowLongPtr.Find(); err == nil {
		v, _, _ := procGetWindowLongPtr.Call(hwnd, uintptr(int(gwlStyle)))
		return v
	}
	v, _, _ := procGetWindowLong.Call(hwnd, uintptr(int(gwlStyle)))
	return v
}

// setWindowStyle writes the window's style back, through the same pair.
func setWindowStyle(hwnd uintptr, style uintptr) {
	if err := procSetWindowLongPtr.Find(); err == nil {
		procSetWindowLongPtr.Call(hwnd, uintptr(int(gwlStyle)), style)
		return
	}
	procSetWindowLong.Call(hwnd, uintptr(int(gwlStyle)), style)
}

// stripSystemChrome takes the caption, the window menu and the minimize and
// maximize boxes off a frameless window, and asks Windows to recompute the
// frame so the non-client band goes with them.
func stripSystemChrome(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	style := windowStyle(hwnd)
	cleared := style &^ (wsCaption | wsSysMenu | wsMinimizeBox | wsMaximizeBox)
	if cleared == style {
		return
	}
	setWindowStyle(hwnd, cleared)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
}
