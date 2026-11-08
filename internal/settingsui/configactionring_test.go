package settingsui

import "testing"

func TestFormActionConfigDefaultsRing(t *testing.T) { checkFormActionRing(t, "config-defaults") }
func TestFormActionConfigSaveRing(t *testing.T)     { checkFormActionRing(t, "config-save") }
