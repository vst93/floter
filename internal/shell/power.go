package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"floter/internal/launcher"
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
// test answered for itself: the settings' own destructive action, which is a
// dialog because it is not the panel's own row.
func (a *App) confirmClearClipboard(title string) bool {
	copy := a.SettingsCopy()
	if a.confirmDialog != nil {
		return a.confirmDialog(title)
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageWarning,
		Message: title,
		Detail:  copy.ClipboardClearDetail,
		Buttons: []string{copy.ClipboardClearButton, copy.ClipboardClearCancel},
	})
	return err == nil && result.Button == 0
}

// powerActionTimeout bounds a power utility, so a broken one cannot leave the
// launcher hidden forever (the reference's `POWER_ACTION_TIMEOUT`,
// `src-tauri/src/commands/system.rs:10`).
const powerActionTimeout = 30 * time.Second

// runPower runs the platform's own command, through the injected runner when
// there is one. The candidates are tried in order, and the result is waited for:
// a successful spawn can still end in a polkit, loginwindow, or privilege
// refusal. Only a missing program falls through to the next candidate; once a
// program starts, its refusal is authoritative — trying a more privileged
// fallback after that would undermine the desktop's decision.
func (a *App) runPower(action string) error {
	if a.runPowerCommand != nil {
		return a.runPowerCommand(action)
	}
	candidates, err := powerCommands(action, runtime.GOOS)
	if err != nil {
		return err
	}
	return runPowerCandidates(candidates, powerActionTimeout)
}

// runPowerCandidates is the reference's `run_first`
// (`src-tauri/src/commands/system.rs:72-118`): run each candidate, and let the
// first that is installed decide. A missing program continues; any other start
// error, a timeout, or a non-zero exit returns at once, with the utility's own
// stderr in the message.
func runPowerCandidates(candidates [][]string, timeout time.Duration) error {
	var missing error
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		program, args := candidate[0], candidate[1:]
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, program, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		deadline := ctx.Err()
		cancel()
		switch {
		case err == nil:
			return nil
		case errors.Is(err, exec.ErrNotFound):
			missing = fmt.Errorf("%s: %w", program, err)
			continue
		case errors.Is(deadline, context.DeadlineExceeded):
			return fmt.Errorf("%s: timed out while requesting power action", program)
		case cmd.ProcessState == nil:
			return fmt.Errorf("%s: %w", program, err)
		}
		status := "terminated by signal"
		if code := cmd.ProcessState.ExitCode(); code >= 0 {
			status = fmt.Sprintf("exit code %d", code)
		}
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return fmt.Errorf("%s: %s: %s", program, status, detail)
		}
		return fmt.Errorf("%s: %s", program, status)
	}
	if missing == nil {
		missing = errNoPowerCandidate
	}
	return missing
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

// errNoPowerCandidate is what a platform with no power utility reports (the
// reference's "Power actions are not supported on this platform").
var errNoPowerCandidate = errors.New("Power actions are not supported on this platform")
