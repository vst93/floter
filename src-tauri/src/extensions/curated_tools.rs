//! The **one** curated allow-list for discovery ranking.
//!
//! Discovery on a real machine finds thousands of PATH executables (this
//! project's reference box has 3174 unique names), and every `AutoDetected`
//! candidate carries the same `quality`. Sorting by quality then name therefore
//! degrades to plain alphabetical order, where the first twelve rows are
//! `7z 7za 7zr a52dec …` and `git` sits at position 895. The fix is a ranking
//! signal, and the only hand-authored input to it is this list.
//!
//! ## Why a static list, and why exactly one
//!
//! The catalog's `score_entry` (`catalog.rs`) ranks *search hits* against a
//! query; it cannot rank *discoveries*, because a discovery has no query. A
//! popularity signal needs data the device does not have (there is no
//! download-count feed, and `catalog-usage.json` is not written yet — see the
//! R8 research P6). A static list is the smallest honest source of "a person
//! would recognise this name".
//!
//! Selection criteria, applied together:
//!
//! * **Ubiquitous developer/ops CLIs** — the tools this launcher exists to
//!   surface (`git`, `rg`, `docker`, `kubectl`, …).
//! * **Cross-platform where possible**, so the same ranking helps on Linux,
//!   macOS and Windows. Windows spellings that differ (`python` vs `py`,
//!   `ffmpeg`) are listed explicitly; the `.exe`/`.cmd`/`.bat` suffix is
//!   stripped before the lookup.
//! * **Not a personal preference**: nothing here is a Floter-specific or
//!   niche tool. A name that only one person would recognise does not earn a
//!   place, because a wrong promotion is worse than a missing one — the list
//!   only reorders the first twelve rows, it never hides anything.
//!
//! This is deliberately a **data list, not an adapter table**. Membership
//! changes nothing about how a tool is connected or run; it only moves the row
//! up. Keeping it in one constant means there is a single place to review when
//! the list changes — a second allow-list anywhere (a frontend copy, a
//! per-platform table, a `match` arm) would be a second source of truth for the
//! same decision and is forbidden by the R8 red lines.

/// Names that earn the curated ranking bonus. Lowercase, no extension; the
/// lookup in [`crate::extensions::inventory::candidate_priority`] lowercases the
/// candidate name and strips a Windows executable suffix first.
pub const CURATED_TOOLS: &[&str] = &[
    // Version control & review
    "git",
    "gh",
    "glab",
    "svn",
    "hg",
    // Search & navigation
    "rg",
    "fd",
    "fzf",
    "bat",
    "jq",
    "yq",
    "grep",
    "sed",
    "awk",
    "find",
    "tree",
    "less",
    // Runtimes & language toolchains
    "node",
    "npm",
    "pnpm",
    "yarn",
    "bun",
    "deno",
    "python",
    "python3",
    "py",
    "pip",
    "pip3",
    "uv",
    "cargo",
    "rustc",
    "go",
    "java",
    "ruby",
    "gem",
    "php",
    "dotnet",
    // Containers & orchestration
    "docker",
    "podman",
    "kubectl",
    "helm",
    "kind",
    "minikube",
    "terraform",
    // Media & conversion
    "ffmpeg",
    "ffprobe",
    "magick",
    "convert",
    "pandoc",
    // Network & transfer
    "curl",
    "wget",
    "ssh",
    "scp",
    "rsync",
    "nc",
    "dig",
    "ping",
    // Data stores
    "psql",
    "mysql",
    "sqlite3",
    "redis-cli",
    "mongosh",
    // Editors & shells
    "code",
    "vim",
    "nvim",
    "nano",
    "emacs",
    "tmux",
    "zsh",
    "bash",
    "fish",
    // Build & system
    "make",
    "cmake",
    "ninja",
    "gcc",
    "clang",
    "pkg-config",
    "systemctl",
    "journalctl",
    "top",
    "htop",
    "ps",
    "du",
    "df",
];

/// Whether `name` (already lowercased, extension stripped) is curated.
pub fn is_curated(name: &str) -> bool {
    CURATED_TOOLS.contains(&name)
}
