//go:build !windows

package shell

// stripSystemChrome is a no-op off Windows: macOS keeps the window server's
// own chrome (the launcher's panel has none of its own) and Linux's frameless
// toplevels carry no caption to take off.
func stripSystemChrome(hwnd uintptr) {}

// clearWindowsBackdrop is a no-op off Windows: the `off` glass stop is spelled
// as "no material" there (see windowMaterialCall), because macOS's and Linux's
// windows do not lose their transparency with the material.
func clearWindowsBackdrop(hwnd uintptr) {}
