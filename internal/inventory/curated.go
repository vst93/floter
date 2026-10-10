package inventory

import (
	"os"
	"path/filepath"
	"strings"
)

// The curated allow-list: the one hand-authored input to discovery's
// ranking. Discovery on a real machine finds thousands of PATH executables,
// every one of them the same quality; sorting by quality then name degrades
// to plain alphabetical order, where the first twelve rows are
// `7z 7za 7zr a52dec …` and `git` sits at 895. A static list is the
// smallest honest source of "a person would recognise this name" — no
// download-count feed exists, and the list only reorders the first twelve
// rows, it never hides anything.

// CuratedTools are the ubiquitous developer and ops CLIs. Cross-platform
// where possible; nothing personal, because a wrong promotion is worse
// than a missing one.
var CuratedTools = []string{
	// Version control & review
	"git", "gh", "glab", "svn", "hg",
	// Search & navigation
	"rg", "fd", "fzf", "bat", "jq", "yq", "grep", "sed", "awk", "find", "tree", "less",
	// Runtimes & language toolchains
	"node", "npm", "pnpm", "yarn", "bun", "deno",
	"python", "python3", "py", "pip", "pip3", "uv",
	"cargo", "rustc", "go", "java", "ruby", "gem", "php", "dotnet",
	// Containers & orchestration
	"docker", "podman", "kubectl", "helm", "kind", "minikube", "terraform",
	// Media & conversion
	"ffmpeg", "ffprobe", "magick", "convert", "pandoc",
	// Network & transfer
	"curl", "wget", "ssh", "scp", "rsync", "nc", "dig", "ping",
	// Data stores
	"psql", "mysql", "sqlite3", "redis-cli", "mongosh",
	// Editors & shells
	"code", "vim", "nvim", "nano", "emacs", "tmux", "zsh", "bash", "fish",
	// Build & system
	"make", "cmake", "ninja", "gcc", "clang", "pkg-config",
	"systemctl", "journalctl", "top", "htop", "ps", "du", "df",
}

// curated is the lookup set over the list.
var curated = func() map[string]bool {
	set := make(map[string]bool, len(CuratedTools))
	for _, name := range CuratedTools {
		set[name] = true
	}
	return set
}()

// CuratedStem lowercases a name and strips a Windows launcher suffix, so
// `rg.exe`, `rg.cmd` and `rg` all compare equal against the list. The one
// spelling rule: the ranking and every caller normalize through it, so a
// name cannot be curated under one spelling and not another.
func CuratedStem(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com"} {
		if trimmed, ok := strings.CutSuffix(name, suffix); ok {
			return trimmed
		}
	}
	return name
}

// IsCurated reports whether a name is on the list.
func IsCurated(name string) bool { return curated[CuratedStem(name)] }

// Priority is the ranking signal for a discovery suggestion: a signed sum
// of what discovery already collected, clamped at zero. Only the ordering
// it induces is meaningful.
//
//	being on the curated list   +300
//	an OS-published entry       +200  (desktop, LaunchServices)
//	a description                +60
//	a user-owned directory       +40  (~/.local/bin, ~/.cargo/bin, …)
//	a long name                  −4 a character past ten
//	a variant or wrapper name   −200
//	a one-character name        −100
func Priority(candidate Candidate) int {
	const (
		base               = 1000
		curatedBonus       = 300
		desktopBonus       = 200
		descriptionBonus   = 60
		userDirectoryBonus = 40
	)
	stem := CuratedStem(candidate.Name)
	score := base
	if IsCurated(stem) {
		score += curatedBonus
	}
	for _, source := range candidate.Sources {
		if source == SourceDesktop || source == SourceLaunchService {
			score += desktopBonus
			break
		}
	}
	if strings.TrimSpace(candidate.Description) != "" {
		score += descriptionBonus
	}
	if isUserOwnedDirectory(candidate.Path) {
		score += userDirectoryBonus
	}
	// A long name is far more often a variant or a wrapper than the
	// primary command (`python3.11-config`, `x86_64-linux-gnu-gcc-12`).
	length := len([]rune(stem))
	if length > 10 {
		score -= (length - 10) * 4
	}
	if isVariantName(stem) {
		score -= 200
	}
	if length <= 1 {
		score -= 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// isVariantName reports whether a name reads as a variant rather than the
// primary command: `aclocal-1.18`, `gcc-12`, `xml2-config`. Deliberately
// conservative — a false positive only costs 200 points, and the curated
// bonus still wins for anything on the list.
func isVariantName(name string) bool {
	if strings.HasSuffix(name, "-config") || strings.HasSuffix(name, "-test") {
		return true
	}
	// The last `-` group: a version-looking suffix marks a variant.
	index := strings.LastIndexByte(name, '-')
	if index < 0 {
		return false
	}
	suffix := name[index+1:]
	if suffix == "" {
		return false
	}
	hasDigit := false
	for _, char := range suffix {
		if char >= '0' && char <= '9' {
			hasDigit = true
		}
		if char != '.' && (char < '0' || char > '9') {
			return false
		}
	}
	return hasDigit
}

// isUserOwnedDirectory reports whether an executable lives under a
// directory the user owns and populated. Best-effort: an unreadable path
// simply earns no bonus.
func isUserOwnedDirectory(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, owned := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".nix-profile", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".local", "share", "flatpak", "exports", "bin"),
	} {
		if strings.HasPrefix(path, owned+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
