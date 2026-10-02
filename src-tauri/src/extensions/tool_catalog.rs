//! R67 · the static install catalog for external CLI tools.
//!
//! Floter deliberately does **not** become a package manager and never touches
//! the network. What it can do is know *where* a tool comes from and *which*
//! package manager this machine already has, then hand the user a single
//! command to type into their own shell — where their proxy, mirror and
//! environment variables are already in effect.
//!
//! Three facts shaped this module:
//!
//! * **A tool that is not installed cannot describe itself.** A provider
//!   descriptor is the tool's own output, and `provider.rs` refuses to accept
//!   one whose executable is not a file, so install metadata cannot live in a
//!   descriptor. It is a compile-time Rust table instead — no schema change,
//!   no generated data.
//! * **Detection must not spawn.** "Is `brew` installed?" is a `stat` of a
//!   candidate name inside the host's search path, never `Command::new`. The
//!   one path source is [`crate::extensions::runtime_path::search_directories`]
//!   (the same answer the run path and the interpreter check use), not the bare
//!   process `PATH` a Finder launch hands a macOS app.
//! * **A missing tool is not an error.** Detection is a best-effort boolean.
//!   No version is claimed — a `stat` cannot produce one, and pretending it
//!   could would be a lie the UI would repeat.
//!
//! The module is data plus pure functions. The IPC command
//! ([`crate::commands::extensions::extensions_tool_catalog`]) is the only
//! caller; the command string the user ultimately sees is rendered on the
//! frontend (`src/extensions/tool-install.ts`), which owns the per-manager
//! syntax while this table owns the manager/package data. The two sides are
//! pinned together by a bidirectional guard test.

use crate::extensions::install;
use serde::Serialize;
use std::path::PathBuf;

/// The platform keys every per-platform table is addressed by. Kept as a
/// constant so the frontend guard and the tables cannot disagree about the
/// vocabulary (the frontend's twin is `RECIPE_PLATFORMS`).
pub const PLATFORM_KEYS: &[&str] = &["macos", "linux", "windows"];

/// The platform this build runs on, as a [`PLATFORM_KEYS`] member.
pub fn current_platform() -> &'static str {
    if cfg!(target_os = "macos") {
        "macos"
    } else if cfg!(target_os = "windows") {
        "windows"
    } else {
        "linux"
    }
}

/// One install recipe: a package manager Floter knows how to phrase a command
/// for, and the package name it installs. The *syntax* (`brew install …`) is
/// deliberately not stored here — it belongs to the frontend's
/// `MANAGER_INSTALL_COMMANDS` table, so this table carries only data.
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolRecipe {
    pub manager: &'static str,
    pub package: &'static str,
}

/// Candidate executable names to `stat` on each platform, in probe order. The
/// name is not always the tool's name: Debian ships `fd` as `fdfind` and `bat`
/// as `batcat`, so a single-name probe would report both as missing.
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProbeCandidates {
    pub macos: &'static [&'static str],
    pub linux: &'static [&'static str],
    pub windows: &'static [&'static str],
}

impl ProbeCandidates {
    /// The candidate names for `platform`; an unknown key falls back to the
    /// Linux list, which is the loosest (most candidates) of the three.
    pub fn for_platform(&self, platform: &str) -> &'static [&'static str] {
        match platform {
            "macos" => self.macos,
            "windows" => self.windows,
            _ => self.linux,
        }
    }
}

/// The install recipes for each platform, ordered by priority. The frontend
/// picks the first one whose manager is actually present on the machine, so
/// the order is the preference order, not a set.
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RecipeTable {
    pub macos: &'static [ToolRecipe],
    pub linux: &'static [ToolRecipe],
    pub windows: &'static [ToolRecipe],
}

impl RecipeTable {
    /// The ordered recipes for `platform`; an unknown key falls back to Linux.
    pub fn for_platform(&self, platform: &str) -> &'static [ToolRecipe] {
        match platform {
            "macos" => self.macos,
            "windows" => self.windows,
            _ => self.linux,
        }
    }
}

/// How R69 may call the tool out: the argv of the GUI/TUI program to spawn,
/// and a human-readable description of what the call does. The launcher's
/// invoke row renders the argv as its subtitle and hands it to the detached
/// spawn ([`crate::commands::actions::system_spawn_detached`]) — no shell, no
/// terminal, no provider run path.
///
/// Only tools with a GUI or TUI action carry a hint. A pure CLI filter — `jq`,
/// `fd`, `rg` — has nothing sensible to start detached (it reads standard
/// input), and its honest home is the user's own shell, so it stays `None` and
/// earns no invoke row.
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LaunchHint {
    pub argv: &'static [&'static str],
    pub description: &'static str,
}

/// One catalog entry. Every string is a proper noun and stays untranslated, so
/// no i18n key exists for the catalog.
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolCatalogEntry {
    /// Stable id; the frontend's `TOOL_CATALOG_IDS` mirror is guarded against it.
    pub id: &'static str,
    pub display_name: &'static str,
    pub homepage: &'static str,
    /// R68 · words a launcher query may be matched against, beyond the id and
    /// the display name. They are search vocabulary, not metadata: the
    /// frontend's `tool-rows.ts` does a case-insensitive substring test over
    /// them, so a Chinese speaker can reach `flameshot` by typing 截图 and a
    /// Latin keyboard can reach it through the pinyin initials (the R51
    /// precedent for names the frontend owns). Two to five per tool, all
    /// untranslated data like the rest of the table.
    pub keywords: &'static [&'static str],
    pub probe_candidates: ProbeCandidates,
    pub recipes: RecipeTable,
    pub launch: Option<LaunchHint>,
}

/// The package managers Floter can phrase an install for. Detection is a pure
/// `stat` of `candidates` inside the host search path; `platforms` records
/// where the manager is meaningful (a recipe never names a manager outside its
/// own platform).
#[derive(Debug, Clone, Copy)]
pub struct PackageManager {
    pub id: &'static str,
    pub display_name: &'static str,
    pub candidates: &'static [&'static str],
    pub platforms: &'static [&'static str],
}

/// Compact constructors for the tables below. They keep the data readable as
/// data — rustfmt would otherwise expand every recipe literal to four lines,
/// turning a curated table into a wall of braces. `const fn` so the tables stay
/// compile-time constants.
const fn recipe(manager: &'static str, package: &'static str) -> ToolRecipe {
    ToolRecipe { manager, package }
}

const fn launch(argv: &'static [&'static str], description: &'static str) -> LaunchHint {
    LaunchHint { argv, description }
}

/// The one package-manager table. Order is presentation order, not preference.
pub const PACKAGE_MANAGERS: &[PackageManager] = &[
    PackageManager {
        id: "brew",
        display_name: "Homebrew",
        candidates: &["brew"],
        platforms: &["macos", "linux"],
    },
    PackageManager {
        id: "winget",
        display_name: "winget",
        candidates: &["winget"],
        platforms: &["windows"],
    },
    PackageManager {
        id: "pacman",
        display_name: "pacman",
        candidates: &["pacman"],
        platforms: &["linux"],
    },
    PackageManager {
        id: "apt",
        display_name: "apt",
        candidates: &["apt", "apt-get"],
        platforms: &["linux"],
    },
    PackageManager {
        id: "dnf",
        display_name: "dnf",
        candidates: &["dnf"],
        platforms: &["linux"],
    },
    PackageManager {
        id: "npm",
        display_name: "npm",
        candidates: &["npm"],
        platforms: &["macos", "linux", "windows"],
    },
    PackageManager {
        id: "cargo",
        display_name: "Cargo",
        candidates: &["cargo"],
        platforms: &["macos", "linux", "windows"],
    },
    PackageManager {
        id: "pipx",
        display_name: "pipx",
        candidates: &["pipx"],
        platforms: &["macos", "linux", "windows"],
    },
];

/// The tool table. Twelve tools that a developer or ops user plausibly reaches
/// for, each with at least one recipe on every platform. Homebrew is the macOS
/// answer; on Linux the system manager (pacman/apt/dnf) comes first and a
/// language package manager (npm/cargo/pipx) is the fallback; Windows uses
/// winget, falling back to a language manager where winget has no package.
///
/// Recipes are omitted rather than guessed: a manager whose package name is not
/// known for a tool simply does not appear, and the frontend falls through to
/// the next recipe.
pub const TOOL_CATALOG: &[ToolCatalogEntry] = &[
    ToolCatalogEntry {
        id: "flameshot",
        display_name: "Flameshot",
        homepage: "https://flameshot.org",
        keywords: &["截图", "screenshot", "screen", "jietu"],
        probe_candidates: ProbeCandidates {
            macos: &["flameshot"],
            linux: &["flameshot"],
            windows: &["flameshot"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "flameshot")],
            linux: &[
                recipe("pacman", "flameshot"),
                recipe("apt", "flameshot"),
                recipe("dnf", "flameshot"),
            ],
            windows: &[recipe("winget", "Flameshot.Flameshot")],
        },
        launch: Some(launch(&["flameshot", "gui"], "Take a screenshot")),
    },
    ToolCatalogEntry {
        id: "yt-dlp",
        display_name: "yt-dlp",
        homepage: "https://github.com/yt-dlp/yt-dlp",
        keywords: &["下载", "download", "video", "youtube", "xiazai"],
        probe_candidates: ProbeCandidates {
            macos: &["yt-dlp"],
            linux: &["yt-dlp"],
            windows: &["yt-dlp"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "yt-dlp")],
            linux: &[
                recipe("pacman", "yt-dlp"),
                recipe("apt", "yt-dlp"),
                recipe("dnf", "yt-dlp"),
                recipe("pipx", "yt-dlp"),
            ],
            windows: &[recipe("winget", "yt-dlp.yt-dlp"), recipe("pipx", "yt-dlp")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "jq",
        display_name: "jq",
        homepage: "https://jqlang.github.io/jq/",
        keywords: &["json", "解析", "parse", "jiexi"],
        probe_candidates: ProbeCandidates {
            macos: &["jq"],
            linux: &["jq"],
            windows: &["jq"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "jq")],
            linux: &[
                recipe("pacman", "jq"),
                recipe("apt", "jq"),
                recipe("dnf", "jq"),
            ],
            windows: &[recipe("winget", "jqlang.jq")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "fd",
        display_name: "fd",
        homepage: "https://github.com/sharkdp/fd",
        keywords: &["查找", "search", "files", "chazhao"],
        probe_candidates: ProbeCandidates {
            macos: &["fd"],
            // Debian/Ubuntu install the binary as `fdfind` to avoid a clash.
            linux: &["fd", "fdfind"],
            windows: &["fd"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "fd")],
            linux: &[
                recipe("pacman", "fd"),
                recipe("apt", "fd-find"),
                recipe("dnf", "fd-find"),
                recipe("cargo", "fd-find"),
            ],
            windows: &[recipe("winget", "sharkdp.fd"), recipe("cargo", "fd-find")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "ripgrep",
        display_name: "ripgrep",
        homepage: "https://github.com/BurntSushi/ripgrep",
        keywords: &["搜索", "search", "grep", "sousuo"],
        probe_candidates: ProbeCandidates {
            macos: &["rg"],
            linux: &["rg"],
            windows: &["rg"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "ripgrep")],
            linux: &[
                recipe("pacman", "ripgrep"),
                recipe("apt", "ripgrep"),
                recipe("dnf", "ripgrep"),
                recipe("cargo", "ripgrep"),
            ],
            windows: &[
                recipe("winget", "BurntSushi.ripgrep.MSVC"),
                recipe("cargo", "ripgrep"),
            ],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "fzf",
        display_name: "fzf",
        homepage: "https://github.com/junegunn/fzf",
        keywords: &["模糊查找", "fuzzy", "search", "mohuchazhao"],
        probe_candidates: ProbeCandidates {
            macos: &["fzf"],
            linux: &["fzf"],
            windows: &["fzf"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "fzf")],
            linux: &[
                recipe("pacman", "fzf"),
                recipe("apt", "fzf"),
                recipe("dnf", "fzf"),
            ],
            windows: &[recipe("winget", "junegunn.fzf")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "bat",
        display_name: "bat",
        homepage: "https://github.com/sharkdp/bat",
        keywords: &["高亮", "highlight", "cat", "gaoliang"],
        probe_candidates: ProbeCandidates {
            macos: &["bat"],
            // Debian/Ubuntu install the binary as `batcat`.
            linux: &["bat", "batcat"],
            windows: &["bat"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "bat")],
            linux: &[
                recipe("pacman", "bat"),
                recipe("apt", "bat"),
                recipe("dnf", "bat"),
                recipe("cargo", "bat"),
            ],
            windows: &[recipe("winget", "sharkdp.bat"), recipe("cargo", "bat")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "eza",
        display_name: "eza",
        homepage: "https://github.com/eza-community/eza",
        keywords: &["列表", "list", "ls", "liebiao"],
        probe_candidates: ProbeCandidates {
            macos: &["eza"],
            linux: &["eza"],
            windows: &["eza"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "eza")],
            linux: &[
                recipe("pacman", "eza"),
                recipe("apt", "eza"),
                recipe("dnf", "eza"),
                recipe("cargo", "eza"),
            ],
            windows: &[
                recipe("winget", "eza-community.eza"),
                recipe("cargo", "eza"),
            ],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "tldr",
        display_name: "tldr",
        homepage: "https://github.com/tldr-pages/tldr",
        keywords: &["帮助", "examples", "man", "bangzhu"],
        probe_candidates: ProbeCandidates {
            macos: &["tldr"],
            linux: &["tldr"],
            windows: &["tldr"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "tldr")],
            linux: &[
                recipe("pacman", "tldr"),
                recipe("apt", "tldr"),
                recipe("dnf", "tldr"),
                recipe("npm", "tldr"),
            ],
            windows: &[recipe("npm", "tldr")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "httpie",
        display_name: "HTTPie",
        homepage: "https://httpie.io",
        keywords: &["http", "api", "请求", "qingqiu"],
        probe_candidates: ProbeCandidates {
            // The CLI installs as `http` (and `https`), not `httpie`.
            macos: &["http", "https"],
            linux: &["http", "https"],
            windows: &["http", "https"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "httpie"), recipe("pipx", "httpie")],
            linux: &[
                recipe("pacman", "httpie"),
                recipe("apt", "httpie"),
                recipe("dnf", "httpie"),
                recipe("pipx", "httpie"),
            ],
            windows: &[recipe("pipx", "httpie")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "gh",
        display_name: "GitHub CLI",
        homepage: "https://cli.github.com",
        keywords: &["github", "pull request", "pr", "issues"],
        probe_candidates: ProbeCandidates {
            macos: &["gh"],
            linux: &["gh"],
            windows: &["gh"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "gh")],
            linux: &[
                // Arch ships the GitHub CLI under its full name.
                recipe("pacman", "github-cli"),
                recipe("apt", "gh"),
                recipe("dnf", "gh"),
            ],
            windows: &[recipe("winget", "GitHub.cli")],
        },
        launch: None,
    },
    ToolCatalogEntry {
        id: "lazygit",
        display_name: "lazygit",
        homepage: "https://github.com/jesseduffield/lazygit",
        keywords: &["git", "tui", "版本控制", "banbenkongzhi"],
        probe_candidates: ProbeCandidates {
            macos: &["lazygit"],
            linux: &["lazygit"],
            windows: &["lazygit"],
        },
        recipes: RecipeTable {
            macos: &[recipe("brew", "lazygit")],
            // No apt recipe: the package only reached Debian/Ubuntu recently,
            // and a wrong package name is worse than a missing row.
            linux: &[recipe("pacman", "lazygit"), recipe("dnf", "lazygit")],
            windows: &[recipe("winget", "JesseDuffield.lazygit")],
        },
        launch: Some(launch(&["lazygit"], "Open the lazygit TUI")),
    },
];

/// The first candidate name that resolves to a linked executable inside
/// `directories`. Name-outer, directory-inner (the same preference order the
/// interpreter scan uses): a `fdfind` anywhere on the path beats a `fd` later
/// in it.
fn find_first(directories: &[PathBuf], names: &[&str]) -> Option<PathBuf> {
    for name in names {
        for candidate_name in install::linked_candidate_names(name) {
            for directory in directories {
                let candidate: PathBuf = directory.join(&candidate_name);
                if install::is_linked_executable_public(&candidate) {
                    return Some(candidate);
                }
            }
        }
    }
    None
}

/// Which package managers this machine has, as ids. A pure function of the
/// directory list so a test never depends on what the machine actually has
/// installed.
pub fn detect_package_managers(directories: &[PathBuf]) -> Vec<&'static str> {
    PACKAGE_MANAGERS
        .iter()
        .filter(|manager| find_first(directories, manager.candidates).is_some())
        .map(|manager| manager.id)
        .collect()
}

/// Whether `entry`'s executable is present, by `stat` of its candidate names
/// for `platform`. No version is probed and no process is spawned.
pub fn detect_tool(entry: &ToolCatalogEntry, platform: &str, directories: &[PathBuf]) -> bool {
    find_first(directories, entry.probe_candidates.for_platform(platform)).is_some()
}

/// One package manager and whether it was found on this machine.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PackageManagerStatus {
    pub id: &'static str,
    pub display_name: &'static str,
    pub detected: bool,
}

/// A catalog entry plus this machine's detection result for it.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolCatalogStatus {
    #[serde(flatten)]
    pub entry: ToolCatalogEntry,
    pub detected: bool,
}

/// The IPC payload: the whole catalog, the platform it was resolved for, and
/// the detection results. `detected` is a `stat` hit on a candidate name —
/// never a version claim.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolCatalogReport {
    pub platform: &'static str,
    pub managers: Vec<PackageManagerStatus>,
    pub tools: Vec<ToolCatalogStatus>,
}

/// Build the report from an injected directory list. Production passes
/// [`crate::extensions::runtime_path::search_directories`]; tests pass a
/// temp directory so the answer is the test's, not the machine's.
pub fn build_report(directories: &[PathBuf]) -> ToolCatalogReport {
    let platform = current_platform();
    let detected = detect_package_managers(directories);
    let managers = PACKAGE_MANAGERS
        .iter()
        .map(|manager| PackageManagerStatus {
            id: manager.id,
            display_name: manager.display_name,
            detected: detected.contains(&manager.id),
        })
        .collect();
    let tools = TOOL_CATALOG
        .iter()
        .map(|entry| ToolCatalogStatus {
            entry: *entry,
            detected: detect_tool(entry, platform, directories),
        })
        .collect();
    ToolCatalogReport {
        platform,
        managers,
        tools,
    }
}

/// A manager's declared platforms. Exposed so the recipe table can be checked
/// against it without duplicating the mapping.
pub fn manager_platforms(id: &str) -> Option<&'static [&'static str]> {
    PACKAGE_MANAGERS
        .iter()
        .find(|manager| manager.id == id)
        .map(|manager| manager.platforms)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    /// A temp directory with an executable file for each name.
    fn directory_with(names: &[&str]) -> tempfile::TempDir {
        let directory = tempfile::tempdir().unwrap();
        for name in names {
            let path = directory.path().join(name);
            fs::write(&path, b"#!/bin/sh\n").unwrap();
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                fs::set_permissions(&path, fs::Permissions::from_mode(0o755)).unwrap();
            }
        }
        directory
    }

    fn directories(temp: &tempfile::TempDir) -> Vec<PathBuf> {
        vec![temp.path().to_path_buf()]
    }

    #[test]
    fn r67_package_manager_detection_is_injected_not_the_machine() {
        let empty = tempfile::tempdir().unwrap();
        assert!(detect_package_managers(&directories(&empty)).is_empty());

        let temp = directory_with(&["brew", "cargo"]);
        assert_eq!(
            detect_package_managers(&directories(&temp)),
            vec!["brew", "cargo"],
            "detection follows the injected directories, in table order",
        );
        // An executable named `pacman` is not in this directory, and the
        // process PATH is never consulted.
        assert!(!detect_package_managers(&directories(&temp)).contains(&"pacman"));
    }

    #[test]
    fn r67_apt_detects_through_either_candidate_name() {
        for name in ["apt", "apt-get"] {
            let temp = directory_with(&[name]);
            assert_eq!(detect_package_managers(&directories(&temp)), vec!["apt"]);
        }
    }

    #[test]
    fn r67_detection_ignores_a_non_executable_file() {
        let temp = tempfile::tempdir().unwrap();
        fs::write(temp.path().join("brew"), b"not a program").unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(temp.path().join("brew"), fs::Permissions::from_mode(0o644))
                .unwrap();
        }
        // On Windows the extension decides and the fixture has none; on unix
        // the execute bit does. Either way it is not an install.
        assert!(detect_package_managers(&directories(&temp)).is_empty());
    }

    #[test]
    fn r67_tool_detection_uses_the_platform_candidates() {
        // `fdfind` is the Linux binary name; the macos list has only `fd`.
        let temp = directory_with(&["fdfind"]);
        let fd = TOOL_CATALOG.iter().find(|entry| entry.id == "fd").unwrap();
        assert!(detect_tool(fd, "linux", &directories(&temp)));
        assert!(!detect_tool(fd, "macos", &directories(&temp)));

        let temp = directory_with(&["batcat"]);
        let bat = TOOL_CATALOG.iter().find(|entry| entry.id == "bat").unwrap();
        assert!(detect_tool(bat, "linux", &directories(&temp)));

        // ripgrep's binary is `rg`, not the id.
        let temp = directory_with(&["rg"]);
        let rg = TOOL_CATALOG
            .iter()
            .find(|entry| entry.id == "ripgrep")
            .unwrap();
        assert!(detect_tool(rg, "linux", &directories(&temp)));
    }

    #[test]
    fn r67_every_tool_has_a_recipe_on_every_platform() {
        for entry in TOOL_CATALOG {
            for platform in PLATFORM_KEYS {
                let recipes = entry.recipes.for_platform(platform);
                assert!(!recipes.is_empty(), "{} has no {platform} recipe", entry.id);
            }
        }
    }

    #[test]
    fn r67_every_recipe_names_a_manager_valid_on_that_platform() {
        for entry in TOOL_CATALOG {
            for platform in PLATFORM_KEYS {
                for recipe in entry.recipes.for_platform(platform) {
                    let platforms = manager_platforms(recipe.manager).unwrap_or_else(|| {
                        panic!("{} names an unknown manager {}", entry.id, recipe.manager)
                    });
                    assert!(
                        platforms.contains(platform),
                        "{} uses {} on {platform}, where it is not available",
                        entry.id,
                        recipe.manager,
                    );
                }
            }
        }
    }

    #[test]
    fn r67_ids_are_unique_and_candidates_are_non_empty() {
        let mut seen = std::collections::BTreeSet::new();
        for entry in TOOL_CATALOG {
            assert!(seen.insert(entry.id), "duplicate tool id {}", entry.id);
            assert!(!entry.display_name.is_empty());
            assert!(entry.homepage.starts_with("https://"), "{}", entry.id);
            for platform in PLATFORM_KEYS {
                assert!(
                    !entry.probe_candidates.for_platform(platform).is_empty(),
                    "{} has no {platform} probe candidate",
                    entry.id,
                );
            }
        }
        assert!(
            (10..=14).contains(&TOOL_CATALOG.len()),
            "the catalog stays a curated dozen, not a directory dump",
        );
    }

    #[test]
    fn r67_report_carries_the_platform_and_the_detection() {
        let temp = directory_with(&["brew", "jq"]);
        let report = build_report(&directories(&temp));
        assert_eq!(report.platform, current_platform());
        assert_eq!(report.managers.len(), PACKAGE_MANAGERS.len());
        assert_eq!(report.tools.len(), TOOL_CATALOG.len());
        let brew = report
            .managers
            .iter()
            .find(|manager| manager.id == "brew")
            .unwrap();
        assert!(brew.detected);
        let jq = report
            .tools
            .iter()
            .find(|tool| tool.entry.id == "jq")
            .unwrap();
        assert!(jq.detected, "the injected `jq` must be reported detected");
        let gh = report
            .tools
            .iter()
            .find(|tool| tool.entry.id == "gh")
            .unwrap();
        assert!(!gh.detected);
    }

    #[test]
    fn r67_report_serializes_camel_case_with_the_flattened_entry() {
        let temp = tempfile::tempdir().unwrap();
        let value = serde_json::to_value(build_report(&directories(&temp))).unwrap();
        assert_eq!(value["platform"], current_platform());
        let tool = &value["tools"][0];
        // Flattened: the entry's fields sit at the top level next to `detected`.
        assert!(tool["id"].is_string(), "the entry must be flattened");
        assert!(tool["displayName"].is_string());
        assert_eq!(tool["detected"], false);
        assert!(tool["recipes"]["macos"].is_array());
        assert!(tool["probeCandidates"]["linux"].is_array());
        // R68 · the search vocabulary rides the same flattened entry.
        assert!(tool["keywords"].is_array());
        let manager = &value["managers"][0];
        assert!(manager["id"].is_string());
        assert!(manager["displayName"].is_string());
        assert!(manager["detected"].is_boolean());
    }

    #[test]
    fn r67_platform_keys_are_exactly_the_frontend_vocabulary() {
        assert_eq!(PLATFORM_KEYS, &["macos", "linux", "windows"]);
        assert!(PLATFORM_KEYS.contains(&current_platform()));
    }

    #[test]
    fn r68_keywords_are_two_to_five_unique_and_non_empty() {
        for entry in TOOL_CATALOG {
            assert!(
                (2..=5).contains(&entry.keywords.len()),
                "{} has {} keywords, outside the 2..=5 budget",
                entry.id,
                entry.keywords.len(),
            );
            let unique: std::collections::BTreeSet<&str> = entry.keywords.iter().copied().collect();
            assert_eq!(
                unique.len(),
                entry.keywords.len(),
                "{} repeats a keyword",
                entry.id,
            );
            for keyword in entry.keywords {
                assert!(
                    !keyword.trim().is_empty(),
                    "{} has a blank keyword",
                    entry.id,
                );
            }
        }
    }

    #[test]
    fn r67_launch_hints_are_optional_but_well_formed() {
        for entry in TOOL_CATALOG {
            if let Some(hint) = entry.launch {
                assert!(!hint.argv.is_empty(), "{} has an empty argv", entry.id);
                assert!(!hint.description.is_empty(), "{}", entry.id);
            }
        }
    }

    /// R69 · the launch hints are the GUI/TUI subset, not every tool. A pure CLI
    /// filter (`jq`, `fd`, `rg`, …) reads standard input and has nothing to
    /// start detached, so it carries no hint and earns no launcher invoke row.
    /// Only `flameshot` (a GUI) and `lazygit` (a TUI) qualify, and their argv is
    /// the exact program the invoke row hands to the detached spawn.
    #[test]
    fn r69_only_gui_and_tui_tools_carry_a_launch_hint() {
        let launched: Vec<&str> = TOOL_CATALOG
            .iter()
            .filter(|entry| entry.launch.is_some())
            .map(|entry| entry.id)
            .collect();
        assert_eq!(launched, vec!["flameshot", "lazygit"]);

        let argv = |id: &str| -> &'static [&'static str] {
            TOOL_CATALOG
                .iter()
                .find(|entry| entry.id == id)
                .and_then(|entry| entry.launch)
                .unwrap_or_else(|| panic!("{id} must carry a launch hint"))
                .argv
        };
        assert_eq!(argv("flameshot"), &["flameshot", "gui"]);
        assert_eq!(argv("lazygit"), &["lazygit"]);
    }

    /// R69 · the launch payload serializes as camelCase with the argv as a JSON
    /// array, which is what the frontend's invoke row reads.
    #[test]
    fn r69_the_launch_hint_serializes_with_its_argv() {
        let temp = tempfile::tempdir().unwrap();
        let value = serde_json::to_value(build_report(&directories(&temp))).unwrap();
        let tools = value["tools"].as_array().unwrap();
        let find = |id: &str| {
            tools
                .iter()
                .find(|tool| tool["id"] == id)
                .unwrap_or_else(|| panic!("{id} must be in the report"))
        };
        assert_eq!(find("flameshot")["launch"]["argv"][0], "flameshot");
        assert_eq!(find("flameshot")["launch"]["argv"][1], "gui");
        assert!(find("lazygit")["launch"]["argv"].is_array());
        // A CLI tool's hint is `null`, which is what makes the invoke row absent.
        assert!(find("jq")["launch"].is_null());
        assert!(find("fd")["launch"].is_null());
    }
}
