//go:build !windows

package shell

// stripSystemChrome is a no-op off Windows: macOS keeps the window server's
// own chrome (the launcher's panel has none of its own) and Linux's frameless
// toplevels carry no caption to take off.
func stripSystemChrome(hwnd uintptr) {}
