package settingsui

import "testing"

func TestCustomSaveFailureMode(t *testing.T)   { checkCustomSaveFailure(t, "mode") }
func TestCustomSaveFailureRemove(t *testing.T) { checkCustomSaveFailure(t, "remove") }
