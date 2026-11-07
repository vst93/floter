package shell

import "testing"

func TestShortcutExitBuiltinHide(t *testing.T) { checkShortcutExit(t, false, "hide") }
func TestShortcutExitCustomHide(t *testing.T)  { checkShortcutExit(t, true, "hide") }
