package shell

import (
	"errors"
	"log"
	"runtime"

	"github.com/egoist/mygo"

	"floter/internal/launcher"
	"floter/internal/spawn"
)

// Restarting and shutting down, from the launcher's power rows.
//
// Both end the session, so both ask first — but the panel asks, in its own
// docked confirmation (the launcher's `.launcher-system-confirm`: the action's
// words, the action's own button, and the way back), so nothing here asks a
// second time. A system dialog after the user has already answered the panel's
// own question would be asking twice.

// power runs a power action the panel has confirmed. The system's command is
// the system's, so it runs off the main thread.
func (a *App) power(action string) {
	go func() {
		copy := a.Launcher.Copy()
		if err := a.runPower(action); err != nil {
			log.Printf("floter: could not %s: %v", action, err)
			a.onMain(func() { a.Launcher.WarnFeedback(copy.PowerFailed) })
		}
	}()
}

// confirmClearClipboard asks the user, through the framework's dialog unless a
// test answered for itself.
func (a *App) confirmClearClipboard(title string) bool {
	copy := a.SettingsCopy()
	if a.confirmPowerDialog != nil {
		return a.confirmPowerDialog(title)
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageWarning,
		Message: title,
		Detail:  copy.ClipboardClearDetail,
		Buttons: []string{copy.ClipboardClearButton, copy.ClipboardClearCancel},
	})
	return err == nil && result.Button == 0
}

// runPower runs the platform's own command, through the injected runner when
// there is one. The candidates are tried in order: the first that **starts**
// wins, since a failure after that (polkit refusing, the dialog cancelled)
// would only be doubled by the next.
func (a *App) runPower(action string) error {
	if a.runPowerCommand != nil {
		return a.runPowerCommand(action)
	}
	candidates, err := powerCommands(action, runtime.GOOS)
	if err != nil {
		return err
	}
	var lastErr error
	for _, candidate := range candidates {
		if err := spawn.Program(candidate[0], candidate[1:]...); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// powerCommands is the platform's way to restart or shut down, most likely
// first: `systemctl` is accepted from an unprivileged seat session through
// polkit on every systemd distribution, and the SysV binaries follow it for
// the rare init that is not systemd.
func powerCommands(action, goos string) ([][]string, error) {
	if action != launcher.PowerRestart && action != launcher.PowerShutdown {
		return nil, errNoPowerAction
	}
	switch goos {
	case "darwin":
		// The Apple event is what GUI applications are expected to use: it
		// goes through loginwindow, which asks the user to confirm exactly
		// as the Apple menu does; the `shutdown` binary would need root.
		if action == launcher.PowerRestart {
			return [][]string{{"osascript", "-e", `tell application "System Events" to restart`}}, nil
		}
		return [][]string{{"osascript", "-e", `tell application "System Events" to shut down`}}, nil
	case "windows":
		// `shutdown.exe` needs no elevation to end the calling user's own
		// session, and `/t 0` skips the default grace period.
		if action == launcher.PowerRestart {
			return [][]string{{"shutdown", "/r", "/t", "0"}}, nil
		}
		return [][]string{{"shutdown", "/s", "/t", "0"}}, nil
	default:
		if action == launcher.PowerRestart {
			return [][]string{{"systemctl", "reboot"}, {"reboot"}}, nil
		}
		return [][]string{{"systemctl", "poweroff"}, {"poweroff"}}, nil
	}
}

// errNoPowerAction is what an unknown action reports.
var errNoPowerAction = errors.New("shell: no such power action")
