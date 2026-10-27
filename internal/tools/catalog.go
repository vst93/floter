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
	"path/filepath"
	"runtime"
	"strings"

	"floter/internal/progname"
	"floter/internal/progtype"
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

// ProbeTable is the executable names that mean "this tool is here", per
// platform key. It is a table rather than one list because the three platforms
// disagree about what a tool's binary is called, and a name a distribution
// invented must not light the tool up somewhere that distribution does not
// exist.
type ProbeTable map[string][]string

// ProbesEverywhere is the table for a tool whose binary has the same name on
// all three platforms, which is most of them.
func ProbesEverywhere(names ...string) ProbeTable {
	table := make(ProbeTable, 3)
	for _, platform := range []string{PlatformMacOS, PlatformLinux, PlatformWindows} {
		table[platform] = names
	}
	return table
}

// For is the candidate names to probe on `platform`. An empty key means the
// host, the same compatibility rule `candidateNames` has, and an unknown key
// falls back to the host for the same reason: the answer has to be a real
// platform's list, never an empty one.
func (t ProbeTable) For(platform string) []string {
	if platform == "" {
		platform = Platform()
	}
	if names := t[platform]; len(names) > 0 {
		return names
	}
	return t[Platform()]
}

// All is every name in the table, in platform order and deduplicated. It is
// what search reads: a user who knows the tool as `rg` or `batcat` must find it
// wherever they are, so the search vocabulary is the union, not one column.
func (t ProbeTable) All() []string {
	var names []string
	seen := map[string]bool{}
	for _, platform := range []string{PlatformMacOS, PlatformLinux, PlatformWindows} {
		for _, name := range t[platform] {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// Manager is a package manager floter can phrase a command for.
type Manager struct {
	ID string
	// Name is what the user calls it.
	Name string
	// Candidates are the executable names that mean "this manager is here".
	Candidates []string
	// Platforms are where the manager is meaningful, as the same three keys
	// every per-platform table is addressed by. A recipe never names a manager
	// outside its own platform (that is what
	// TestEveryRecipeNamesAManagerValidOnThatPlatform checks), and this field is
	// the only place that fact is written down: detection itself does not
	// filter by it, exactly as the old build's `detect_package_managers` did
	// not — a `brew` in the search path is a `brew` whatever the host is.
	Platforms []string
}

// Managers are the package managers, in the order a machine that has several
// prefers them: a native package manager before a language one.
var Managers = []Manager{
	{"brew", "Homebrew", []string{"brew"}, []string{PlatformMacOS, PlatformLinux}},
	{"winget", "winget", []string{"winget"}, []string{PlatformWindows}},
	{"pacman", "pacman", []string{"pacman"}, []string{PlatformLinux}},
	{"apt", "apt", []string{"apt", "apt-get"}, []string{PlatformLinux}},
	{"dnf", "dnf", []string{"dnf"}, []string{PlatformLinux}},
	{"npm", "npm", []string{"npm"}, []string{PlatformMacOS, PlatformLinux, PlatformWindows}},
	{"cargo", "Cargo", []string{"cargo"}, []string{PlatformMacOS, PlatformLinux, PlatformWindows}},
	{"pipx", "pipx", []string{"pipx"}, []string{PlatformMacOS, PlatformLinux, PlatformWindows}},
}

// ManagerPlatforms are the platforms the manager with this id is meaningful on,
// nil when there is no such manager. The recipe table is checked against this
// without a second copy of the mapping.
func ManagerPlatforms(id string) []string {
	for _, manager := range Managers {
		if manager.ID == id {
			return manager.Platforms
		}
	}
	return nil
}

// ManagerName is what the user calls the manager with this id, empty when
// there is no such manager. The install row's source slot names the manager
// the command will use, and it is the manager's own proper noun — "Homebrew",
// not the executable's "brew" — so the row reads like the old build's.
func ManagerName(id string) string {
	for _, manager := range Managers {
		if manager.ID == id {
			return manager.Name
		}
	}
	return ""
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
	// Probes are the executable names that mean "this tool is here", per
	// platform. The table is per platform because a name a distribution
	// invented is only a name on that distribution's platform: `fdfind` is
	// Debian's spelling of `fd` and means nothing on macOS or Windows.
	Probes ProbeTable
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
		Probes:   ProbesEverywhere("flameshot"),
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
		Probes:   ProbesEverywhere("yt-dlp"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "yt-dlp"}},
			PlatformLinux:   {{"pacman", "yt-dlp"}, {"apt", "yt-dlp"}, {"dnf", "yt-dlp"}, {"pipx", "yt-dlp"}},
			PlatformWindows: {{"winget", "yt-dlp.yt-dlp"}, {"pipx", "yt-dlp"}},
		},
	},
	{
		ID: "jq", Name: "jq",
		Keywords: []string{"json", "解析", "parse", "jiexi"},
		Probes:   ProbesEverywhere("jq"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "jq"}},
			PlatformLinux:   {{"pacman", "jq"}, {"apt", "jq"}, {"dnf", "jq"}},
			PlatformWindows: {{"winget", "jqlang.jq"}},
		},
	},
	{
		ID: "fd", Name: "fd",
		Keywords: []string{"查找", "search", "files", "chazhao"},
		// Debian and Ubuntu install the binary as `fdfind` to avoid a clash —
		// on Linux only, which is what makes this the one entry that needs a
		// spelled-out table.
		Probes: ProbeTable{
			PlatformMacOS:   {"fd"},
			PlatformLinux:   {"fd", "fdfind"},
			PlatformWindows: {"fd"},
		},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "fd"}},
			PlatformLinux:   {{"pacman", "fd"}, {"apt", "fd-find"}, {"dnf", "fd-find"}, {"cargo", "fd-find"}},
			PlatformWindows: {{"winget", "sharkdp.fd"}, {"cargo", "fd-find"}},
		},
	},
	{
		ID: "ripgrep", Name: "ripgrep",
		Keywords: []string{"搜索", "search", "grep", "sousuo"},
		Probes:   ProbesEverywhere("rg"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "ripgrep"}},
			PlatformLinux:   {{"pacman", "ripgrep"}, {"apt", "ripgrep"}, {"dnf", "ripgrep"}, {"cargo", "ripgrep"}},
			PlatformWindows: {{"winget", "BurntSushi.ripgrep.MSVC"}, {"cargo", "ripgrep"}},
		},
	},
	{
		ID: "fzf", Name: "fzf",
		Keywords: []string{"模糊查找", "fuzzy", "search", "mohuchazhao"},
		Probes:   ProbesEverywhere("fzf"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "fzf"}},
			PlatformLinux:   {{"pacman", "fzf"}, {"apt", "fzf"}, {"dnf", "fzf"}},
			PlatformWindows: {{"winget", "junegunn.fzf"}},
		},
	},
	{
		ID: "bat", Name: "bat",
		Keywords: []string{"高亮", "highlight", "cat", "gaoliang"},
		// Debian and Ubuntu install the binary as `batcat` — on Linux only.
		Probes: ProbeTable{
			PlatformMacOS:   {"bat"},
			PlatformLinux:   {"bat", "batcat"},
			PlatformWindows: {"bat"},
		},
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "bat"}},
			PlatformLinux:   {{"pacman", "bat"}, {"apt", "bat"}, {"dnf", "bat"}, {"cargo", "bat"}},
			PlatformWindows: {{"winget", "sharkdp.bat"}, {"cargo", "bat"}},
		},
	},
	{
		ID: "eza", Name: "eza",
		Keywords: []string{"列表", "list", "ls", "liebiao"},
		Probes:   ProbesEverywhere("eza"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "eza"}},
			PlatformLinux:   {{"pacman", "eza"}, {"apt", "eza"}, {"dnf", "eza"}, {"cargo", "eza"}},
			PlatformWindows: {{"winget", "eza-community.eza"}, {"cargo", "eza"}},
		},
	},
	{
		ID: "tldr", Name: "tldr",
		Keywords: []string{"帮助", "examples", "man", "bangzhu"},
		Probes:   ProbesEverywhere("tldr"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "tldr"}},
			PlatformLinux:   {{"pacman", "tldr"}, {"apt", "tldr"}, {"dnf", "tldr"}, {"npm", "tldr"}},
			PlatformWindows: {{"npm", "tldr"}},
		},
	},
	{
		ID: "httpie", Name: "HTTPie",
		Keywords: []string{"http", "api", "请求", "qingqiu"},
		// The CLI installs as `http` (and `https`), not `httpie`, on every
		// platform.
		Probes: ProbesEverywhere("http", "https"),
		Recipes: map[string][]Recipe{
			PlatformMacOS:   {{"brew", "httpie"}, {"pipx", "httpie"}},
			PlatformLinux:   {{"pacman", "httpie"}, {"apt", "httpie"}, {"dnf", "httpie"}, {"pipx", "httpie"}},
			PlatformWindows: {{"pipx", "httpie"}},
		},
	},
	{
		ID: "gh", Name: "GitHub CLI",
		Keywords: []string{"github", "pull request", "pr", "issues"},
		Probes:   ProbesEverywhere("gh"),
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
		Probes:   ProbesEverywhere("lazygit"),
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
	detected := DetectManagers(dirs, platform)
	states := make([]State, 0, len(Catalog))
	for _, entry := range Catalog {
		state := State{Entry: entry, Installed: Installed(entry, dirs, platform)}
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
func Installed(entry Entry, dirs []string, platform string) bool {
	return Probe(entry, dirs, platform) != ""
}

// Probe is the probe name that resolved, empty when none did.
func Probe(entry Entry, dirs []string, platform string) string {
	for _, name := range entry.Probes.For(platform) {
		for _, candidate := range candidateNames(platform, name) {
			for _, dir := range dirs {
				if dir == "" {
					continue
				}
				if progtype.For(platform, filepath.Join(dir, candidate)) {
					return candidate
				}
			}
		}
	}
	return ""
}

// DetectManagers lists the managers this machine has, as ids, in the table's
// order.
func DetectManagers(dirs []string, platform string) []string {
	var found []string
	for _, manager := range Managers {
		if managerPresent(manager, dirs, platform) {
			found = append(found, manager.ID)
		}
	}
	return found
}

// managerPresent reports whether any of a manager's candidate names resolves.
func managerPresent(manager Manager, dirs []string, platform string) bool {
	for _, name := range manager.Candidates {
		for _, candidate := range candidateNames(platform, name) {
			for _, dir := range dirs {
				if dir != "" && progtype.For(platform, filepath.Join(dir, candidate)) {
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
// program is named with its extension. `platform` is the same key `Look` was
// given — reading it (not `runtime.GOOS`) is what lets a Linux host pin the
// Windows ladder; an empty value falls back to the host.
//
// The ladder itself is `progname.Ladder`: the four launcher suffixes are one
// table shared with the rule that decides a file is a program and the rule
// that strips the suffix back off a program's name.
func candidateNames(platform, name string) []string {
	return progname.Ladder(platform, name)
}

// Matches reports whether a query's terms all appear in a tool's name, id,
// keywords or probe names: the executable a user knows the tool by (`rg`,
// `batcat`) is search vocabulary too, on every platform.
func (s State) Matches(terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	haystack := s.SearchHaystack()
	for _, term := range terms {
		if !strings.Contains(haystack, strings.ToLower(term)) {
			return false
		}
	}
	return true
}

// SearchHaystack is the one haystack a launcher query is matched against, and
// the one string an install or invoke row carries as its search text: the id,
// the display name, the keywords and every probe name on every platform.
//
// It is lowercase and lives here so the three places that used to spell it out
// (Matches and the launcher's two row builders) cannot drift apart.
func (s State) SearchHaystack() string {
	return strings.ToLower(strings.Join([]string{s.ID, s.Name,
		strings.Join(s.Keywords, " "), strings.Join(s.Probes.All(), " ")}, " "))
}
