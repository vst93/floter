package shell

import "testing"

func TestShortcutExitBuiltinPage(t *testing.T) { checkShortcutExit(t, false, "page") }
func TestShortcutExitCustomPage(t *testing.T)  { checkShortcutExit(t, true, "page") }
