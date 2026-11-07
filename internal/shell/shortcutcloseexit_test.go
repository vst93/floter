package shell

import "testing"

func TestShortcutExitBuiltinClose(t *testing.T) { checkShortcutExit(t, false, "close") }
func TestShortcutExitCustomClose(t *testing.T)  { checkShortcutExit(t, true, "close") }
