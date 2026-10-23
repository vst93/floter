//go:build windows

package shell

import (
	"syscall"
	"unsafe"
)

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

// dwmwaSystemBackdropType is DWMWA_SYSTEMBACKDROP_TYPE (38), the attribute that
// picks which system backdrop DWM composites behind the client area, and
// dwmsbtNone (1) is "no backdrop": nothing is composited there, so what the page
// leaves transparent shows the desktop. The material's extended frame is left
// alone — that frame is what makes the client area the window's whole surface in
// the first place.
const (
	dwmwaSystemBackdropType = 38
	dwmsbtNone              = 1
)

// The window-frame attributes that take the *rest* of the system's chrome off a
// frameless panel: DWM draws a one-pixel border around every top-level window
// and, on Windows 11, rounds the window's rectangle to the system corner radius
// (DWMWA_WINDOW_CORNER_PREFERENCE, 33). Both are drawn *outside* the app's own
// card: the border is the 「窗口周边外圈还有一圈额外的底色」 line, and the
// system's rounding clips the transparent corners into opaque wedges against the
// card's own radius. The card is the whole of the window's chrome, so the border
// goes (DWMWA_COLOR_NONE, 0xFFFFFFFE — the documented "no border" value, not
// black) and the rectangle stays square (DWMWCP_DONOTROUND), which leaves the
// card's own 14px rounding as the only one on screen — the same shape, and the
// same figure, macOS and Linux give it.
const (
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34
	dwmcpDoNotRound             = 1
	dwmwaColorNone              = 0xFFFFFFFE
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

	dwmapiDLL                 = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapiDLL.NewProc("DwmSetWindowAttribute")
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
// frame so the non-client band goes with them. The frame DWM still draws on top
// of that — its border line and its corner rounding — comes off in
// dressWindowsFrame, which is applied on every call: mygo re-adds the style bits
// on a resize, and the frame attributes are cheap enough to restate each time.
//
// **Why this is still here after mygo v0.6.0 added
// WindowOptions.HideWindowButtons** (P7-lf, from reading
// `mygo@v0.6.0/internal/windows/window.go`): that option cannot reach a
// frameless window, which is the only kind floter creates. Upstream computes
// its hidden-title-bar flag as
//
//	w.hiddenTitleBar = !o.Frameless && (o.TitleBarStyle == "hidden" || o.TitleBarStyle == "hiddenInset")
//
// (`window.go:133`), so `Frameless: true` makes the flag false whatever
// `TitleBarStyle` or `HideWindowButtons` say, and the two places the option is
// read are both behind it: the caption bar is only built for a hidden title bar
// (`window.go:196`) and `shownButtons()` returns its empty list only from that
// bar's layout (`titlebar.go:151-158`). `styles()` (`window.go:235-258`) keeps
// building from `WS_OVERLAPPEDWINDOW` regardless — it strips `WS_SYSMENU` only
// for `Panel`, which is why floter's panel window lost the window menu from
// upstream while the launcher's caption, minimize and maximize boxes still had
// to come off here. The alternative — a hidden title bar instead of a frameless
// window — is not a swap: it keeps an upstream-drawn band and a shadow laid out
// around the window's rectangle, which is exactly the 「外圈额外的底色」 the
// reports brought us here to remove, and the app's card is the whole of the
// window's chrome. So the local strip stays, and this note is the ledger of
// which bit belongs to whom: the caption/menu/boxes are ours because upstream
// will not touch a frameless window's style; the DWM border and the corner
// rounding are ours because upstream's equivalents ride `TitleBarStyle` too.
//
// **P9-a 复核，mygo v0.6.3** (upstream #612, 「Windows 最大化 + 隐藏标题栏 + 材质
// 时 DWM 不再画标题按钮」): the fix is `glassMargins`/`extendGlass`
// (`window.go:1075-1102`), which stops extending the material's frame over the
// top of a *maximized* hidden-title-bar window — the strip DWM paints its three
// buttons in — and extends it from the bottom instead. Both of its inputs come
// from `w.hiddenTitleBar`, which is false for every window floter creates, so
// the new branch is unreachable here and the conclusion of P7-lf stands: the
// local strip is still the only thing that takes the caption off a frameless
// window. It also cannot fight upstream: the style bits this function clears are
// the ones `styles()` sets unconditionally from `WS_OVERLAPPEDWINDOW`.
func stripSystemChrome(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	dressWindowsFrame(hwnd)
	style := windowStyle(hwnd)
	cleared := style &^ (wsCaption | wsSysMenu | wsMinimizeBox | wsMaximizeBox)
	if cleared == style {
		return
	}
	setWindowStyle(hwnd, cleared)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
}

// dressWindowsFrame removes the border DWM draws around the window and keeps
// the window's rectangle square, so the card's own rounded, transparent edge is
// the only edge on screen. Both attributes are ignored by systems that do not
// know them (the border colour is Windows 11 21H2+, the corner preference
// 22000+), and a refusal is not an error worth reporting: the window is still a
// frameless panel, it just keeps the system's own border.
func dressWindowsFrame(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	corner := int32(dwmcpDoNotRound)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
	border := uint32(dwmwaColorNone)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(dwmwaBorderColor),
		uintptr(unsafe.Pointer(&border)), unsafe.Sizeof(border))
}

// clearWindowsBackdrop drops the system backdrop behind a window's content
// while keeping the window's own transparency: the `off` glass stop means no
// blur and no haze, not an opaque window (see windowVibrancyFor).
//
// The window's material bit stays set, and that bit is what lets the desktop
// through, so the frame's tint is read over the desktop exactly as it is on
// macOS and Linux with the glass switched off. Asking the framework for "no
// material" instead would take the bit off — and it is the window's for life,
// so the panel would stay opaque until the app restarted.
func clearWindowsBackdrop(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	backdrop := int32(dwmsbtNone)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(dwmwaSystemBackdropType),
		uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
}
