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
// Both end the session, so both ask first: a native dialog with the platform's
// own words, and only then the system's command. Nothing here runs on arrival —
// the row is offered, the user confirms, and the command goes to the system.

// power confirms a power action and runs it. The dialog blocks, so the whole
// thing runs off the main thread.
func (a *App) power(action string) {
	go func() {
		copy := a.Launcher.Copy()
		name := copy.PowerRestart
		if action == launcher.PowerShutdown {
			name = copy.PowerShutdown
		}
		confirmed, err := a.confirmPower(copy.PowerConfirmTitle(name))
		if err != nil {
			log.Printf("floter: could not ask about %s: %v", action, err)
			return
		}
		if !confirmed {
			return
		}
		if err := a.runPower(action); err != nil {
			log.Printf("floter: could not %s: %v", action, err)
			a.onMain(func() { a.Launcher.Feedback(copy.PowerFailed) })
		}
	}()
}

// confirmPower asks the user, through the framework's dialog unless a test
// answered for itself.
func (a *App) confirmPower(title string) (bool, error) {
	copy := a.Launcher.Copy()
	if a.confirmPowerDialog != nil {
		return a.confirmPowerDialog(title), nil
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageWarning,
		Message: title,
		Detail:  copy.PowerConfirmDetail,
		Buttons: []string{copy.PowerConfirmButton, copy.PowerCancel},
	})
	if err != nil {
		return false, err
	}
	return result.Button == 0, nil
}

// runPower runs the platform's own command, through the injected runner when
// there is one.
func (a *App) runPower(action string) error {
	if a.runPowerCommand != nil {
		return a.runPowerCommand(action)
	}
	program, args, err := powerCommand(action, runtime.GOOS)
	if err != nil {
		return err
	}
	return spawn.Program(program, args...)
}

// powerCommand is the platform's way to restart or shut down.
func powerCommand(action, goos string) (string, []string, error) {
	restart := action == launcher.PowerRestart
	switch goos {
	case "darwin":
		// AppleScript asks the system, which is what the menu's own items do.
		verb := "restart"
		if !restart {
			verb = "shut down"
		}
		return "osascript", []string{"-e", `tell application "System Events" to ` + verb}, nil
	case "windows":
		// `shutdown` wants a timeout: 0 means "now", and the flag is required
		// or the call refuses to run without one.
		if restart {
			return "shutdown", []string{"/r", "/t", "0"}, nil
		}
		return "shutdown", []string{"/s", "/t", "0"}, nil
	default:
		if restart {
			return "systemctl", []string{"reboot"}, nil
		}
		return "systemctl", []string{"poweroff"}, nil
	}
}

// errNoPowerAction is what an unknown action reports.
var errNoPowerAction = errors.New("shell: no such power action")
