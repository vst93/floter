// Package tools knows where a CLI tool comes from, and whether this machine
// already has it.
//
// Floter is deliberately not a package manager and never touches the network:
// what it does is know which package manager this machine already has and hand
// the user **one command to type into their own shell**, where their proxy,
// mirror and environment variables are already in effect. Nothing here runs an
// install, and nothing claims a version: detection is a `stat` of a candidate
// name in the host's search path, never a spawn.
package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The platform keys every per-platform table is addressed by.
const (
	PlatformMacOS   = "macos"
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
)

// Platform is the key for the platform the app is running on.
func Platform() string {
	switch runtime.GOOS {
	case "darwin":
		return PlatformMacOS
	case "windows":
		return PlatformWindows
	default:
		return PlatformLinux
	}
}

// Manager is a package manager floter can phrase a command for.
type Manager struct {
	ID string
	// Name is what the user calls it.
	Name string
	// Candidates are the executable names that mean "this manager is here".
	Candidates []string
}

// Managers are the package managers, in the order a machine that has several
// prefers them: a native package manager before a language one.
var Managers = []Manager{
	{"brew", "Homebrew", []string{"brew"}},
	{"winget", "winget", []string{"winget"}},
	{"pacman", "pacman", []string{"pacman"}},
	{"apt", "apt", []string{"apt", "apt-get"}},
	{"dnf", "dnf", []string{"dnf"}},
	{"npm", "npm", []string{"npm"}},
	{"cargo", "Cargo", []string{"cargo"}},
	{"pipx", "pipx", []string{"pipx"}},
}

// Recipe is one manager and the package that installs a tool with it.
type Recipe struct {
	Manager string
	Package string
}

// Entry is one tool floter knows where to get.
type Entry struct {
	ID   string
	Name string
	// Keywords are extra search vocabulary (the tool's job in both languages,
	// so a Chinese query finds it).
	Keywords []string
	// Probes are the executable names that mean "this tool is here".
	Probes []string
	// Recipes are per platform, most preferred first.
	Recipes map[string][]Recipe
	// NeedsTerminal marks a full-screen tool, which has to run in a terminal.
	NeedsTerminal bool
	// Launch is how the tool may be called out from the launcher: the argv of
	// the program to start, for a tool with a GUI or TUI action of its own. A
	// pure CLI filter — jq, fd, rg — carries none: it reads standard input,
	// and its honest home is the user's own shell.
	Launch []string
}

// Catalog is the tool table. Recipes are omitted rather than guessed: a
// manager whose package name is not known for a tool simply does not appear,
// and the next recipe is tried.
var Catalog = []Entry{
	{
		ID: "flameshot", Name: "Flameshot",
		Keywords: []string{"截图", "screenshot", "screen", "jietu"},
		Probes:   []string{"flameshot"},
		// Its own GUI action, started detached: `flameshot gui`.
		Launch: []string{"flameshot", "gui"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "flameshot"}},
			PlatformLinux:   {{"pacman", "flameshot"}, {"apt", "flameshot"}, {"dnf", "flameshot"}},
			PlatformWindows: {{"winget", "Flameshot.Flameshot"}},
		},
	},
	{
		ID: "yt-dlp", Name: "yt-dlp",
		Keywords: []string{"下载", "download", "video", "youtube", "xiazai"},
		Probes:   []string{"yt-dlp"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "yt-dlp"}},
			PlatformLinux:   {{"pacman", "yt-dlp"}, {"apt", "yt-dlp"}, {"dnf", "yt-dlp"}, {"pipx", "yt-dlp"}},
			PlatformWindows: {{"winget", "yt-dlp.yt-dlp"}, {"pipx", "yt-dlp"}},
		},
	},
	{
		ID: "jq", Name: "jq",
		Keywords: []string{"json", "解析", "parse", "jiexi"},
		Probes:   []string{"jq"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "jq"}},
			PlatformLinux:   {{"pacman", "jq"}, {"apt", "jq"}, {"dnf", "jq"}},
			PlatformWindows: {{"winget", "jqlang.jq"}},
		},
	},
	{
		ID: "fd", Name: "fd",
		Keywords: []string{"查找", "search", "files", "chazhao"},
		// Debian and Ubuntu install the binary as `fdfind` to avoid a clash.
		Probes: []string{"fd", "fdfind"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "fd"}},
			PlatformLinux:   {{"pacman", "fd"}, {"apt", "fd-find"}, {"dnf", "fd-find"}, {"cargo", "fd-find"}},
			PlatformWindows: {{"winget", "sharkdp.fd"}, {"cargo", "fd-find"}},
		},
	},
	{
		ID: "ripgrep", Name: "ripgrep",
		Keywords: []string{"搜索", "search", "grep", "sousuo"},
		Probes:   []string{"rg"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "ripgrep"}},
			PlatformLinux:   {{"pacman", "ripgrep"}, {"apt", "ripgrep"}, {"dnf", "ripgrep"}, {"cargo", "ripgrep"}},
			PlatformWindows: {{"winget", "BurntSushi.ripgrep.MSVC"}, {"cargo", "ripgrep"}},
		},
	},
	{
		ID: "fzf", Name: "fzf",
		Keywords: []string{"模糊查找", "fuzzy", "search", "mohuchazhao"},
		Probes:   []string{"fzf"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "fzf"}},
			PlatformLinux:   {{"pacman", "fzf"}, {"apt", "fzf"}, {"dnf", "fzf"}},
			PlatformWindows: {{"winget", "junegunn.fzf"}},
		},
	},
	{
		ID: "bat", Name: "bat",
		Keywords: []string{"高亮", "highlight", "cat", "gaoliang"},
		// Debian and Ubuntu install the binary as `batcat`.
		Probes: []string{"bat", "batcat"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "bat"}},
			PlatformLinux:   {{"pacman", "bat"}, {"apt", "bat"}, {"dnf", "bat"}, {"cargo", "bat"}},
			PlatformWindows: {{"winget", "sharkdp.bat"}, {"cargo", "bat"}},
		},
	},
	{
		ID: "eza", Name: "eza",
		Keywords: []string{"列表", "list", "ls", "liebiao"},
		Probes:   []string{"eza"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "eza"}},
			PlatformLinux:   {{"pacman", "eza"}, {"apt", "eza"}, {"dnf", "eza"}, {"cargo", "eza"}},
			PlatformWindows: {{"winget", "eza-community.eza"}, {"cargo", "eza"}},
		},
	},
	{
		ID: "tldr", Name: "tldr",
		Keywords: []string{"帮助", "examples", "man", "bangzhu"},
		Probes:   []string{"tldr"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "tldr"}},
			PlatformLinux:   {{"pacman", "tldr"}, {"apt", "tldr"}, {"dnf", "tldr"}, {"npm", "tldr"}},
			PlatformWindows: {{"npm", "tldr"}},
		},
	},
	{
		ID: "httpie", Name: "HTTPie",
		Keywords: []string{"http", "api", "请求", "qingqiu"},
		// The CLI installs as `http` (and `https`), not `httpie`.
		Probes: []string{"http", "https"},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "httpie"}, {"pipx", "httpie"}},
			PlatformLinux:   {{"pacman", "httpie"}, {"apt", "httpie"}, {"dnf", "httpie"}, {"pipx", "httpie"}},
			PlatformWindows: {{"pipx", "httpie"}},
		},
	},
	{
		ID: "gh", Name: "GitHub CLI",
		Keywords: []string{"github", "pull request", "pr", "issues"},
		Probes:   []string{"gh"},
		Recipes: map[string][]Recipe{
			PlatformMacOS: {{"brew", "gh"}},
			// Arch ships the GitHub CLI under its full name.
			PlatformLinux:   {{"pacman", "github-cli"}, {"apt", "gh"}, {"dnf", "gh"}},
			PlatformWindows: {{"winget", "GitHub.cli"}},
		},
	},
	{
		ID: "lazygit", Name: "lazygit",
		Keywords: []string{"git", "tui", "版本控制", "banbenkongzhi"},
		Probes:   []string{"lazygit"},
		Recipes: map[string][]Recipe{
			PlatformMacOS: {{"brew", "lazygit"}},
			// No apt recipe: the package only reached Debian and Ubuntu
			// recently, and a wrong package name is worse than a missing row.
			PlatformLinux:   {{"pacman", "lazygit"}, {"dnf", "lazygit"}},
			PlatformWindows: {{"winget", "JesseDuffield.lazygit"}},
		},
		// A full-screen TUI: it needs a real terminal or it exits at once.
		NeedsTerminal: true,
		Launch:        []string{"lazygit"},
	},
}

// InstallCommand is the command line that installs a package with a manager:
// the syntax is floter's, the data is the recipe's.
func InstallCommand(manager, packageName string) string {
	switch manager {
	case "brew":
		return "brew install " + packageName
	case "winget":
		return "winget install --id " + packageName + " -e"
	case "pacman":
		return "sudo pacman -S " + packageName
	case "apt":
		return "sudo apt install " + packageName
	case "dnf":
		return "sudo dnf install " + packageName
	case "npm":
		return "npm install -g " + packageName
	case "cargo":
		return "cargo install " + packageName
	case "pipx":
		return "pipx install " + packageName
	default:
		return ""
	}
}

// State is one tool as this machine sees it.
type State struct {
	Entry
	// Installed reports whether a probe name resolved in the search path.
	Installed bool
	// Manager and Package are the recipe that was chosen: the first whose
	// manager this machine has, else the platform's first.
	Manager string
	Package string
	// Command is the install command for that recipe, empty when the tool has
	// no recipe for this platform.
	Command string
}

// Look resolves the catalog against a search path. The path is a parameter so
// a test never depends on what the machine actually has installed.
func Look(dirs []string, platform string) []State {
	detected := DetectManagers(dirs)
	states := make([]State, 0, len(Catalog))
	for _, entry := range Catalog {
		state := State{Entry: entry, Installed: Installed(entry, dirs)}
		if recipe, ok := ChosenRecipe(entry, platform, detected); ok {
			state.Manager, state.Package = recipe.Manager, recipe.Package
			state.Command = InstallCommand(recipe.Manager, recipe.Package)
		}
		states = append(states, state)
	}
	return states
}

// Installed reports whether any of a tool's probe names resolves in the search
// path. Detection is a `stat`, never a spawn: no version is claimed.
func Installed(entry Entry, dirs []string) bool {
	return Probe(entry, dirs) != ""
}

// Probe is the probe name that resolved, empty when none did.
func Probe(entry Entry, dirs []string) string {
	for _, name := range entry.Probes {
		for _, candidate := range candidateNames(name) {
			for _, dir := range dirs {
				if dir == "" {
					continue
				}
				if isExecutable(filepath.Join(dir, candidate)) {
					return candidate
				}
			}
		}
	}
	return ""
}

// DetectManagers lists the managers this machine has, as ids, in the table's
// order.
func DetectManagers(dirs []string) []string {
	var found []string
	for _, manager := range Managers {
		if managerPresent(manager, dirs) {
			found = append(found, manager.ID)
		}
	}
	return found
}

// managerPresent reports whether any of a manager's candidate names resolves.
func managerPresent(manager Manager, dirs []string) bool {
	for _, name := range manager.Candidates {
		for _, candidate := range candidateNames(name) {
			for _, dir := range dirs {
				if dir != "" && isExecutable(filepath.Join(dir, candidate)) {
					return true
				}
			}
		}
	}
	return false
}

// ChosenRecipe picks the recipe to show: the first whose manager this machine
// has, else the platform's first (so a machine with no manager still gets the
// command for the one it would install).
func ChosenRecipe(entry Entry, platform string, detected []string) (Recipe, bool) {
	recipes := entry.Recipes[platform]
	if len(recipes) == 0 {
		return Recipe{}, false
	}
	for _, manager := range detected {
		for _, recipe := range recipes {
			if recipe.Manager == manager {
				return recipe, true
			}
		}
	}
	return recipes[0], true
}

// candidateNames are the file names one probe may resolve to: on Windows a
// program is named with its extension.
func candidateNames(name string) []string {
	if runtime.GOOS != "windows" {
		return []string{name}
	}
	return []string{name + ".exe", name + ".cmd", name + ".bat", name + ".com", name}
}

// isExecutable reports whether path is a regular file the user can run.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

// Matches reports whether a query's terms all appear in a tool's name, id,
// keywords or probe names: the executable a user knows the tool by (`rg`,
// `batcat`) is search vocabulary too.
func (s State) Matches(terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{s.ID, s.Name,
		strings.Join(s.Keywords, " "), strings.Join(s.Probes, " ")}, " "))
	for _, term := range terms {
		if !strings.Contains(haystack, strings.ToLower(term)) {
			return false
		}
	}
	return true
}
