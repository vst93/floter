package launcher

import (
	"testing"

	"floter/internal/plugincfg"
)

func TestConfigLifetimeWriteAnswerBelongsToItsSheet(t *testing.T) {
	for _, switchSheet := range []bool{false, true} {
		for _, accept := range []bool{false, true} {
			name := "close"
			if switchSheet {
				name = "switch"
			}
			if accept {
				name += "-accepted"
			} else {
				name += "-refused"
			}
			t.Run(name, func(t *testing.T) {
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("write answer crashed after %s: %v", name, p)
					}
				}()
				a := configApp(func() bool { return accept })
				a.OpenConfig("clipboard")
				var replacement *Config
				calls := 0
				a.Actions.ConfigChange = func(plugin, key string, value any) plugincfg.Values {
					calls++
					if plugin != "clipboard" || key != "enabled" || value != false {
						t.Errorf("wrong write destination: %s %s %v", plugin, key, value)
					}
					if switchSheet {
						a.OpenConfig("browser")
						replacement = a.Config()
					} else {
						a.closeConfig()
					}
					if accept {
						return plugincfg.Values{"enabled": false}
					}
					return nil
				}
				a.ChangeConfig("enabled", false)
				if calls != 1 {
					t.Errorf("write dispatched %d times", calls)
				}
				if a.Config() != replacement {
					t.Error("write answer changed the current sheet")
				}
				if replacement != nil && (replacement.Failed || replacement.Values["enabled"] != true) {
					t.Errorf("old write answer contaminated new sheet: failed=%v enabled=%v", replacement.Failed, replacement.Values["enabled"])
				}
			})
		}
	}
}

func TestConfigLifetimeActionAnswerBelongsToItsSheet(t *testing.T) {
	for _, switchSheet := range []bool{false, true} {
		for _, accept := range []bool{false, true} {
			name := "close"
			if switchSheet {
				name = "switch"
			}
			if accept {
				name += "-accepted"
			} else {
				name += "-refused"
			}
			t.Run(name, func(t *testing.T) {
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("action answer crashed after %s: %v", name, p)
					}
				}()
				a := actionApp(func() bool { return accept })
				a.OpenConfig("clipboard")
				var replacement *Config
				calls := 0
				a.Actions.ConfigAction = func(plugin, key string) bool {
					calls++
					if plugin != "clipboard" || key != "clear_history" {
						t.Errorf("wrong action destination: %s %s", plugin, key)
					}
					if switchSheet {
						a.OpenConfig("browser")
						replacement = a.Config()
						a.ArmConfig("clear_history")
					} else {
						a.closeConfig()
					}
					return accept
				}
				a.ArmConfig("clear_history")
				a.RunConfig("clear_history")
				if calls != 1 {
					t.Errorf("action dispatched %d times", calls)
				}
				if a.Config() != replacement {
					t.Error("action answer changed the current sheet")
				}
				if replacement != nil && (replacement.ActionFailed != "" || replacement.Armed != "clear_history") {
					t.Errorf("old action answer contaminated new sheet: failed=%q armed=%q", replacement.ActionFailed, replacement.Armed)
				}
			})
		}
	}
}
