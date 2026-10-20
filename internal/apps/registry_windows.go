//go:build windows

package apps

import (
	"os"
	"path/filepath"
)

// uninstallKeys is the reference's three uninstall keys (`windows.rs:32-36`):
// per-user installs, machine-wide 64-bit installs, and the WOW6432Node view the
// 32-bit installers write into.
var uninstallKeys = []string{
	`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
	`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
	`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
}

// queryRegistry reads all three uninstall keys through `reg query /s`.
//
// The dump is written by PowerShell with `Out-File -Encoding Unicode` rather
// than read from `reg`'s own stdout: `reg` writes in the console's OEM code
// page, so a program named in Chinese would come back as bytes no decoder here
// can recover (`windows.rs:369-380`). The helper process is hidden, so a scan
// never flashes a console window (`CREATE_NO_WINDOW`, see hiddenPowerShell).
func queryRegistry() []string {
	dir, err := os.MkdirTemp("", "floter-registry")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)
	var out []string
	for i, key := range uninstallKeys {
		target := filepath.Join(dir, "key"+itoa(i)+".txt")
		script := "& reg.exe query " + psQuote(key) + " /s | Out-File -LiteralPath " +
			psQuote(target) + " -Encoding Unicode"
		if err := runPowerShell(script); err != nil {
			continue
		}
		data, err := os.ReadFile(target)
		if err != nil {
			continue
		}
		if text := DecodeRegistryOutput(data); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// itoa is a tiny decimal for the temp file names above (three of them, so the
// dependency on strconv is not worth carrying into this file's signature).
func itoa(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return itoa(value/10) + string(rune('0'+value%10))
}
