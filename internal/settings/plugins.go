package settings

import (
	"sort"
	"strings"
)

// The search-facing keys the app carries through rather than modelling as
// fields: the per-command switches of the extensions, the user's command
// aliases, and the three switches that decide what the launcher offers.
const (
	commandSwitchesKey  = "plugin_command_switches"
	commandAliasesKey   = "command_aliases"
	showCommandsKey     = "show_commands_in_search"
	showRecentKey       = "show_recent_in_launcher"
	showMenubarIconKey  = "show_menubar_icon"
	lastSettingsPageKey = "last_settings_page"
	launchCountsKey     = "launch_counts"
)

// CommandSwitches is the per-command on/off state of an extension's commands:
// extension id -> command id -> enabled. Absence means off — a command with no
// entry has never been enabled by the user, so it is not summonable.
type CommandSwitches map[string]map[string]bool

// CommandSwitchesOf reads the switches, dropping empty ids as the old build's
// normalizer did: a blank key is not a command.
func CommandSwitchesOf(s Settings) CommandSwitches {
	switches := CommandSwitches{}
	raw, ok := s.Extra()[commandSwitchesKey].(map[string]any)
	if !ok {
		return switches
	}
	for extension, value := range raw {
		extension = strings.TrimSpace(extension)
		if extension == "" {
			continue
		}
		commands, ok := value.(map[string]any)
		if !ok {
			continue
		}
		kept := map[string]bool{}
		for command, state := range commands {
			command = strings.TrimSpace(command)
			if command == "" {
				continue
			}
			enabled, ok := state.(bool)
			if !ok {
				continue
			}
			kept[command] = enabled
		}
		if len(kept) > 0 {
			switches[extension] = kept
		}
	}
	return switches
}

// Enabled reports whether one command is switched on.
func (c CommandSwitches) Enabled(extensionID, commandID string) bool {
	if commands, ok := c[extensionID]; ok {
		return commands[commandID]
	}
	return false
}

// AnyEnabled reports whether an extension has at least one command switched on.
func (c CommandSwitches) AnyEnabled(extensionID string) bool {
	for _, enabled := range c[extensionID] {
		if enabled {
			return true
		}
	}
	return false
}

// SetCommandSwitch records one command's switch, writing it back into the
// settings file's carried-through map.
func (s *Settings) SetCommandSwitch(extensionID, commandID string, enabled bool) {
	extensionID, commandID = strings.TrimSpace(extensionID), strings.TrimSpace(commandID)
	if extensionID == "" || commandID == "" {
		return
	}
	raw := map[string]any{}
	for key, value := range s.switchesRaw() {
		raw[key] = value
	}
	commands, _ := raw[extensionID].(map[string]any)
	next := map[string]any{}
	for key, value := range commands {
		next[key] = value
	}
	next[commandID] = enabled
	raw[extensionID] = next
	s.SetExtra(commandSwitchesKey, raw)
}

// switchesRaw is the stored switch map, as decoded.
func (s *Settings) switchesRaw() map[string]any {
	raw, _ := s.extra[commandSwitchesKey].(map[string]any)
	return raw
}

// CommandAliases maps a command name to the alias the user types for it
// (`git` -> `gfm`).
type CommandAliases map[string]string

// CommandAliasesOf reads the raw alias map.
func CommandAliasesOf(s Settings) CommandAliases {
	aliases := CommandAliases{}
	raw, ok := s.Extra()[commandAliasesKey].(map[string]any)
	if !ok {
		return aliases
	}
	for command, value := range raw {
		alias, ok := value.(string)
		if !ok {
			continue
		}
		aliases[command] = alias
	}
	return aliases
}

// SetCommandAlias records one alias; an empty alias removes the entry, as the
// old settings card's "clear the field" did.
func (s *Settings) SetCommandAlias(command, alias string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	raw := map[string]any{}
	for key, value := range s.aliasesRaw() {
		raw[key] = value
	}
	if strings.TrimSpace(alias) == "" {
		delete(raw, command)
	} else {
		raw[command] = strings.TrimSpace(alias)
	}
	s.SetExtra(commandAliasesKey, raw)
}

// aliasesRaw is the stored alias map, as decoded.
func (s *Settings) aliasesRaw() map[string]any {
	raw, _ := s.extra[commandAliasesKey].(map[string]any)
	return raw
}

// ResolveCommandAliases is the effective alias per command: two commands may
// be given the same alias, so the policy is **first command locks the alias** —
// commands are visited in ascending name order and the first one to claim an
// alias keeps it, a later command whose alias collides being inert (its command
// name still matches normally). Empty aliases are ignored, as is an alias that
// only differs in case from one already claimed.
func ResolveCommandAliases(aliases CommandAliases) CommandAliases {
	commands := make([]string, 0, len(aliases))
	for command := range aliases {
		if strings.TrimSpace(command) != "" {
			commands = append(commands, command)
		}
	}
	sort.Strings(commands)

	claimed := map[string]bool{}
	resolved := CommandAliases{}
	for _, command := range commands {
		alias := strings.TrimSpace(aliases[command])
		if alias == "" {
			continue
		}
		key := strings.ToLower(alias)
		if claimed[key] {
			continue
		}
		claimed[key] = true
		resolved[command] = alias
	}
	return resolved
}

// The launcher's three switches, with the values every earlier build shipped.
const (
	DefaultShowCommandsInSearch = false
	DefaultShowRecentInLauncher = true
	DefaultShowMenubarIcon      = true
)

// ShowCommandsInSearch is whether the launcher offers the executables found on
// the PATH.
func ShowCommandsInSearch(s Settings) bool {
	return boolOr(s, showCommandsKey, DefaultShowCommandsInSearch)
}

// SetShowCommandsInSearch records the PATH-commands switch.
func (s *Settings) SetShowCommandsInSearch(on bool) { s.SetExtra(showCommandsKey, on) }

// ShowRecentInLauncher is whether an empty query offers the most-launched
// applications.
func ShowRecentInLauncher(s Settings) bool {
	return boolOr(s, showRecentKey, DefaultShowRecentInLauncher)
}

// SetShowRecentInLauncher records the recent-applications switch.
func (s *Settings) SetShowRecentInLauncher(on bool) { s.SetExtra(showRecentKey, on) }

// ShowMenubarIcon is whether the tray icon is shown.
func ShowMenubarIcon(s Settings) bool { return boolOr(s, showMenubarIconKey, DefaultShowMenubarIcon) }

// SetShowMenubarIcon records the tray switch.
func (s *Settings) SetShowMenubarIcon(on bool) { s.SetExtra(showMenubarIconKey, on) }

// LastSettingsPage is the page id the settings surface last showed.
func LastSettingsPage(s Settings) string {
	value, _ := s.Extra()[lastSettingsPageKey].(string)
	return strings.TrimSpace(value)
}

// SetLastSettingsPage records the page id.
func (s *Settings) SetLastSettingsPage(name string) { s.SetExtra(lastSettingsPageKey, name) }

// LaunchCounts is what the user launches, by id (an application path or an
// extension command id), as every earlier build stored it.
type LaunchCounts map[string]int

// LaunchCountsOf reads the launch counts.
func LaunchCountsOf(s Settings) LaunchCounts {
	counts := LaunchCounts{}
	raw, ok := s.Extra()[launchCountsKey].(map[string]any)
	if !ok {
		return counts
	}
	for id, value := range raw {
		if count, ok := asInt(value); ok && count > 0 {
			counts[id] = count
		}
	}
	return counts
}

// SetLaunchCounts writes the launch counts back.
func (s *Settings) SetLaunchCounts(counts LaunchCounts) {
	raw := map[string]any{}
	for id, count := range counts {
		if id == "" || count <= 0 {
			continue
		}
		raw[id] = count
	}
	s.SetExtra(launchCountsKey, raw)
}

// boolOr reads a boolean carried-through key, falling back to the shipped
// value when it is missing or of the wrong shape.
func boolOr(s Settings, key string, fallback bool) bool {
	if value, ok := s.Extra()[key].(bool); ok {
		return value
	}
	return fallback
}
