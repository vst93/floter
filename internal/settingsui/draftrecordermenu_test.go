package settingsui

import "testing"

func TestDraftRecorderMenuSelection(t *testing.T) { checkRecorderMenu(t, true, false) }
func TestDraftRecorderMenuEscape(t *testing.T)    { checkRecorderMenu(t, true, true) }
