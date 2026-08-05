//! Linux application discovery: freedesktop `.desktop` entries.
//!
//! Entries are read from the XDG application directories (plus the flatpak
//! exports) in precedence order, so a user override shadows the system copy of
//! the same desktop-file id. Icons are resolved by name through the icon theme
//! directories and copied into the app cache, which is the only location the
//! asset protocol is allowed to serve from.

use std::collections::{HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{Mutex, OnceLock};
use std::time::Duration;
use tauri::AppHandle;

use super::{
    cached_icon_is_fresh, icon_cache_path, mark_icon_cached, paths_signature, LocalApplication,
};

/// Icon sizes to try, largest first, in both the `48x48` and the `48` spelling
/// (hicolor/Adwaita use the former, breeze-style themes the latter).
const ICON_SIZES: [&str; 13] = [
    "512x512", "256x256", "192x192", "128x128", "96x96", "72x72", "64x64", "48x48", "scalable",
    "36x36", "32x32", "24x24", "16x16",
];

const ICON_EXTENSIONS: [&str; 3] = ["png", "svg", "xpm"];

/// Themes searched before falling back to whatever else is installed.
const PREFERRED_THEMES: [&str; 6] = [
    "hicolor", "Adwaita", "breeze", "Papirus", "gnome", "default",
];

pub fn roots() -> Vec<PathBuf> {
    let mut roots = Vec::new();

    if let Some(data_home) = data_home() {
        roots.push(data_home.join("applications"));
        roots.push(data_home.join("flatpak/exports/share/applications"));
    }
    roots.push(PathBuf::from("/usr/local/share/applications"));
    roots.push(PathBuf::from("/usr/share/applications"));
    roots.push(PathBuf::from("/var/lib/flatpak/exports/share/applications"));
    roots.push(PathBuf::from("/var/lib/snapd/desktop/applications"));

    // Anything else the distribution advertises through XDG_DATA_DIRS.
    for dir in data_dirs() {
        let candidate = dir.join("applications");
        if !roots.contains(&candidate) {
            roots.push(candidate);
        }
    }

    roots
}

pub fn scan(roots: &[PathBuf]) -> Vec<LocalApplication> {
    let mut apps = Vec::new();
    // Desktop-file ids are unique across the whole search path; the first root
    // that provides one wins, which is what gives user entries precedence.
    let mut seen_ids = HashSet::new();

    for dir in roots {
        collect_entries(dir, dir, 0, &mut seen_ids, &mut apps);
    }

    apps
}

pub fn source_signature(roots: &[PathBuf]) -> u64 {
    let mut sources = roots.to_vec();
    for root in roots {
        collect_desktop_paths(root, 0, &mut sources);
    }
    paths_signature(sources)
}

pub fn signature_check_interval() -> Duration {
    Duration::from_secs(30)
}

pub fn max_cache_age() -> Option<Duration> {
    None
}

pub fn open(path: &Path) -> Result<(), String> {
    if path.extension().and_then(|ext| ext.to_str()) != Some("desktop") {
        return Command::new("xdg-open")
            .arg(path)
            .spawn()
            .map(|_| ())
            .map_err(|e| e.to_string());
    }

    // `gio launch` is the only launcher that honours the full entry semantics
    // (field codes, `Terminal=`, D-Bus activation), so it is the happy path.
    if Command::new("gio")
        .arg("launch")
        .arg(path)
        .status()
        .is_ok_and(|status| status.success())
    {
        return Ok(());
    }

    // No glib tooling installed: parse the `Exec=` line according to the
    // desktop-entry rules. It is an argv declaration, not shell syntax.
    // `Terminal=true` entries are not wrapped here — that is the fallback of a
    // fallback, and the command still runs, just without a visible terminal.
    let entry = DesktopEntry::parse(path).ok_or("Cannot read desktop entry")?;
    let exec = entry.exec.ok_or("Desktop entry has no Exec line")?;
    let argv = parse_exec_argv(&exec).ok_or("Invalid desktop entry Exec line")?;
    let Some((program, arguments)) = argv.split_first() else {
        return Err("Desktop entry has an empty Exec line".to_string());
    };

    Command::new(program)
        .args(arguments)
        .spawn()
        .map(|_| ())
        .map_err(|e| e.to_string())
}

pub fn icon_path(app: &AppHandle, path: &Path, source_hint: Option<&str>) -> Option<String> {
    let icon = match source_hint {
        Some(icon) => icon.to_string(),
        None => DesktopEntry::parse(path)?.icon?,
    };
    let source = resolve_icon(&icon)?;
    let extension = source
        .extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| ext.to_ascii_lowercase())?;
    // XPM is still common in /usr/share/pixmaps but no browser renders it.
    if extension == "xpm" {
        return None;
    }

    let target = icon_cache_path(app, path, &extension)?;
    if !cached_icon_is_fresh(&target, &source) {
        fs::copy(&source, &target).ok()?;
        mark_icon_cached(&target, &source);
    }
    Some(target.to_string_lossy().to_string())
}

fn data_home() -> Option<PathBuf> {
    std::env::var_os("XDG_DATA_HOME")
        .map(PathBuf::from)
        .filter(|path| path.is_absolute())
        .or_else(|| dirs::home_dir().map(|home| home.join(".local/share")))
}

fn data_dirs() -> Vec<PathBuf> {
    std::env::var_os("XDG_DATA_DIRS")
        .map(|value| std::env::split_paths(&value).collect())
        .unwrap_or_default()
}

fn collect_entries(
    root: &Path,
    dir: &Path,
    depth: usize,
    seen_ids: &mut HashSet<String>,
    apps: &mut Vec<LocalApplication>,
) {
    // Distributions nest entries a couple of levels deep (`kde4/`, `screensavers/`).
    if depth > 3 || !dir.is_dir() {
        return;
    }

    let entries = match fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(_) => return,
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect_entries(root, &path, depth + 1, seen_ids, apps);
            continue;
        }

        if path.extension().and_then(|ext| ext.to_str()) != Some("desktop") {
            continue;
        }

        let id = desktop_file_id(root, &path);
        if !seen_ids.insert(id) {
            continue;
        }

        let Some(entry) = DesktopEntry::parse(&path) else {
            continue;
        };
        let Some(app) = entry.into_application(&path) else {
            continue;
        };
        apps.push(app);
    }
}

fn collect_desktop_paths(dir: &Path, depth: usize, paths: &mut Vec<PathBuf>) {
    if depth > 3 || !dir.is_dir() {
        return;
    }
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect_desktop_paths(&path, depth + 1, paths);
        } else if path.extension().and_then(|ext| ext.to_str()) == Some("desktop") {
            paths.push(path);
        }
    }
}

/// Desktop-file id: the path below the applications directory with separators
/// turned into dashes (`kde4/konsole.desktop` -> `kde4-konsole.desktop`).
fn desktop_file_id(root: &Path, path: &Path) -> String {
    path.strip_prefix(root)
        .unwrap_or(path)
        .to_string_lossy()
        .replace('/', "-")
}

/// The fields of a `[Desktop Entry]` group that the launcher cares about.
struct DesktopEntry {
    name: Option<String>,
    localized_name: Option<String>,
    generic_name: Option<String>,
    comment: Option<String>,
    exec: Option<String>,
    icon: Option<String>,
    try_exec: Option<String>,
    keywords: Option<String>,
    startup_wm_class: Option<String>,
    entry_type: Option<String>,
    hidden: bool,
    no_display: bool,
}

impl DesktopEntry {
    fn parse(path: &Path) -> Option<Self> {
        let content = fs::read_to_string(path).ok()?;
        let mut entry = Self {
            name: None,
            localized_name: None,
            generic_name: None,
            comment: None,
            exec: None,
            icon: None,
            try_exec: None,
            keywords: None,
            startup_wm_class: None,
            entry_type: None,
            hidden: false,
            no_display: false,
        };
        // Chinese locales first so the launcher can show the same
        // localized/original name pair it shows on macOS.
        let locales = ["zh_CN", "zh_SG", "zh", "zh_TW", "zh_HK"];
        let mut best_name_locale = usize::MAX;
        let mut best_comment_locale = usize::MAX;

        let mut in_entry_group = false;
        for line in content.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if line.starts_with('[') {
                // Only the main group describes the application itself; the
                // trailing `[Desktop Action ...]` groups are separate entries.
                in_entry_group = line == "[Desktop Entry]";
                continue;
            }
            if !in_entry_group {
                continue;
            }

            let Some((raw_key, value)) = line.split_once('=') else {
                continue;
            };
            let raw_key = raw_key.trim();
            let value = value.trim();
            let (key, locale) = match raw_key.split_once('[') {
                Some((key, rest)) => (key.trim(), rest.strip_suffix(']')),
                None => (raw_key, None),
            };

            match (key, locale) {
                ("Name", None) => entry.name = Some(unescape(value)),
                ("Name", Some(locale)) => {
                    if let Some(rank) = locales.iter().position(|candidate| *candidate == locale) {
                        if rank < best_name_locale {
                            best_name_locale = rank;
                            entry.localized_name = Some(unescape(value));
                        }
                    }
                }
                ("Comment", None) => {
                    if entry.comment.is_none() {
                        entry.comment = Some(unescape(value));
                    }
                }
                ("Comment", Some(locale)) => {
                    // Same ranking as the name, so an entry that ships both
                    // zh_CN and zh_TW text does not end up mixing the two.
                    if let Some(rank) = locales.iter().position(|candidate| *candidate == locale) {
                        if rank < best_comment_locale {
                            best_comment_locale = rank;
                            entry.comment = Some(unescape(value));
                        }
                    }
                }
                ("Exec", None) => entry.exec = Some(unescape(value)),
                ("Icon", None) => entry.icon = Some(value.to_string()),
                ("TryExec", None) => entry.try_exec = Some(unescape(value)),
                // The untranslated spellings only: these are search keys for a
                // Latin keyboard, and the localized variants are already covered
                // by the name and its pinyin initials.
                ("GenericName", None) => entry.generic_name = Some(unescape(value)),
                ("Keywords", None) => entry.keywords = Some(unescape(value)),
                ("StartupWMClass", None) => entry.startup_wm_class = Some(unescape(value)),
                ("Type", None) => entry.entry_type = Some(value.to_string()),
                ("Hidden", None) => entry.hidden = value == "true",
                ("NoDisplay", None) => entry.no_display = value == "true",
                _ => {}
            }
        }

        Some(entry)
    }

    fn into_application(self, path: &Path) -> Option<LocalApplication> {
        if self.hidden || self.no_display {
            return None;
        }
        if self
            .entry_type
            .as_deref()
            .is_some_and(|t| t != "Application")
        {
            return None;
        }
        self.exec.as_ref()?;
        // `TryExec` is the entry's own "is this installed" probe.
        if let Some(try_exec) = self.try_exec.as_deref() {
            if !executable_available(try_exec) {
                return None;
            }
        }

        let name = self.name.unwrap_or_else(|| {
            path.file_stem()
                .and_then(|stem| stem.to_str())
                .unwrap_or("Application")
                .to_string()
        });
        let localized_name = self.localized_name.filter(|value| value != &name);

        // What a query can reach the entry by besides its name: the program it
        // runs, the class its window reports, the words the entry advertises
        // itself with, and the application half of its desktop-file id. All are
        // Latin by convention, which is what a Chinese-named entry needs.
        let mut candidates = Vec::new();
        candidates.extend(self.generic_name);
        candidates.extend(self.startup_wm_class);
        candidates.extend(executable_name(self.exec.as_deref()));
        candidates.extend(executable_name(self.try_exec.as_deref()));
        candidates.extend(
            self.keywords
                .iter()
                .flat_map(|keywords| keywords.split(';'))
                .map(str::to_string),
        );
        candidates.extend(
            path.file_stem()
                .and_then(|stem| stem.to_str())
                .map(super::identifier_aliases)
                .unwrap_or_default(),
        );
        let aliases = super::build_aliases(&name, localized_name.as_deref(), candidates);

        Some(LocalApplication {
            name,
            localized_name,
            path: path.to_string_lossy().to_string(),
            icon_path: self.icon,
            comment: self.comment.filter(|value| !value.is_empty()),
            // Filled in by `list_applications` once the scan is done, so the
            // pinyin lookup lives in one place rather than in every scanner.
            initials: String::new(),
            aliases,
        })
    }
}

/// The program an `Exec=`/`TryExec=` line runs, without its path or arguments:
/// `/usr/bin/google-chrome-stable %U` is searchable as `google-chrome-stable`.
fn executable_name(exec: Option<&str>) -> Option<String> {
    let command = parse_exec_argv(exec?)?;
    // Wrappers take the real program as an argument, so the first word is not
    // always the interesting one — but it is the one the entry is identified by
    // everywhere else, and guessing further would be worse than not guessing.
    let program = command.first()?;
    let stem = Path::new(program).file_name()?.to_str()?;
    (!stem.is_empty()).then(|| stem.to_string())
}

fn executable_available(program: &str) -> bool {
    use std::os::unix::fs::PermissionsExt;

    let is_executable = |path: &Path| {
        fs::metadata(path).ok().is_some_and(|metadata| {
            metadata.is_file() && metadata.permissions().mode() & 0o111 != 0
        })
    };
    if program.contains(std::path::is_separator) {
        return is_executable(Path::new(program));
    }
    std::env::var_os("PATH")
        .map(|paths| std::env::split_paths(&paths).any(|dir| is_executable(&dir.join(program))))
        .unwrap_or(false)
}

fn parse_exec_argv(exec: &str) -> Option<Vec<String>> {
    let mut arguments = Vec::new();
    let mut current = String::new();
    let mut chars = exec.chars();
    let mut quoted = false;
    while let Some(character) = chars.next() {
        match character {
            '"' => quoted = !quoted,
            '\\' => current.push(chars.next()?),
            character if character.is_whitespace() && !quoted => {
                if !current.is_empty() {
                    let value = strip_field_codes(&current);
                    if !value.is_empty() {
                        arguments.push(value);
                    }
                    current.clear();
                }
            }
            _ => current.push(character),
        }
    }
    if quoted {
        return None;
    }
    if !current.is_empty() {
        let value = strip_field_codes(&current);
        if !value.is_empty() {
            arguments.push(value);
        }
    }
    Some(arguments)
}

/// Desktop-entry string escapes (`\s`, `\n`, `\t`, `\r`, `\\`).
fn unescape(value: &str) -> String {
    if !value.contains('\\') {
        return value.to_string();
    }
    let mut output = String::with_capacity(value.len());
    let mut chars = value.chars();
    while let Some(ch) = chars.next() {
        if ch != '\\' {
            output.push(ch);
            continue;
        }
        match chars.next() {
            Some('s') => output.push(' '),
            Some('n') => output.push('\n'),
            Some('t') => output.push('\t'),
            Some('r') => output.push('\r'),
            Some('\\') => output.push('\\'),
            Some(other) => {
                output.push('\\');
                output.push(other);
            }
            None => output.push('\\'),
        }
    }
    output
}

/// Drop the `%f`/`%U`/... placeholders: the launcher opens applications with no
/// arguments, and leaving the codes in would pass them through literally.
fn strip_field_codes(exec: &str) -> String {
    let mut output = String::with_capacity(exec.len());
    let mut chars = exec.chars().peekable();
    while let Some(ch) = chars.next() {
        if ch != '%' {
            output.push(ch);
            continue;
        }
        match chars.next() {
            // `%%` is a literal percent sign.
            Some('%') => output.push('%'),
            Some('f' | 'F' | 'u' | 'U' | 'd' | 'D' | 'n' | 'N' | 'i' | 'c' | 'k' | 'v' | 'm') => {}
            Some(other) => {
                output.push('%');
                output.push(other);
            }
            None => {}
        }
    }
    output
}

/// Memo table for [`resolve_icon`], keyed by the raw `Icon=` value.
///
/// Resolving one name walks the pixmap directories and then every theme
/// directory installed on the machine, and the launcher asks for the same
/// handful of names again and again as the user retypes a query. Misses are
/// cached alongside hits, because a name that resolves to nothing is the most
/// expensive lookup there is: it exhausts the entire search path, including the
/// recursive walk at the end.
///
/// Icon themes change about as often as applications are installed, and the
/// process is a long-lived tray app, so the table is never invalidated — a
/// newly themed icon shows up on the next start.
fn icon_cache() -> &'static Mutex<HashMap<String, Option<PathBuf>>> {
    static CACHE: OnceLock<Mutex<HashMap<String, Option<PathBuf>>>> = OnceLock::new();
    CACHE.get_or_init(|| Mutex::new(HashMap::new()))
}

/// [`resolve_icon_uncached`], memoized. A poisoned lock degrades to an uncached
/// lookup rather than taking the launcher down with it.
fn resolve_icon(icon: &str) -> Option<PathBuf> {
    if let Ok(cache) = icon_cache().lock() {
        if let Some(cached) = cache.get(icon) {
            return cached.clone();
        }
    }

    let resolved = resolve_icon_uncached(icon);

    if let Ok(mut cache) = icon_cache().lock() {
        cache.insert(icon.to_string(), resolved.clone());
    }
    resolved
}

fn resolve_icon_uncached(icon: &str) -> Option<PathBuf> {
    // An absolute path is used verbatim, which also covers flatpak entries that
    // point straight into their export directory.
    if icon.starts_with('/') {
        let path = PathBuf::from(icon);
        return path.exists().then_some(path);
    }
    // Some entries carry `Icon=name.png`; the stem is what the theme indexes.
    let name = Path::new(icon)
        .file_stem()
        .and_then(|stem| stem.to_str())
        .unwrap_or(icon);

    for dir in pixmap_dirs() {
        for extension in ICON_EXTENSIONS {
            let candidate = dir.join(format!("{name}.{extension}"));
            if candidate.is_file() {
                return Some(candidate);
            }
        }
    }

    let bases = icon_base_dirs();
    for base in &bases {
        for theme in themes_in(base) {
            if let Some(found) = find_in_theme(&base.join(theme), name) {
                return Some(found);
            }
        }
    }

    None
}

fn icon_base_dirs() -> Vec<PathBuf> {
    let mut dirs = Vec::new();
    if let Some(home) = dirs::home_dir() {
        dirs.push(home.join(".icons"));
    }
    if let Some(data_home) = data_home() {
        dirs.push(data_home.join("icons"));
        dirs.push(data_home.join("flatpak/exports/share/icons"));
    }
    dirs.push(PathBuf::from("/usr/local/share/icons"));
    dirs.push(PathBuf::from("/usr/share/icons"));
    dirs.push(PathBuf::from("/var/lib/flatpak/exports/share/icons"));
    for dir in data_dirs() {
        let candidate = dir.join("icons");
        if !dirs.contains(&candidate) {
            dirs.push(candidate);
        }
    }
    dirs
}

fn pixmap_dirs() -> Vec<PathBuf> {
    let mut dirs = vec![PathBuf::from("/usr/share/pixmaps")];
    if let Some(data_home) = data_home() {
        dirs.insert(0, data_home.join("pixmaps"));
    }
    dirs
}

/// Theme directory names inside `base`, preferred themes first.
fn themes_in(base: &Path) -> Vec<String> {
    let mut themes: Vec<String> = PREFERRED_THEMES
        .iter()
        .filter(|theme| base.join(theme).is_dir())
        .map(|theme| (*theme).to_string())
        .collect();

    if let Ok(entries) = fs::read_dir(base) {
        for entry in entries.flatten() {
            if !entry.path().is_dir() {
                continue;
            }
            let name = entry.file_name().to_string_lossy().to_string();
            if !themes.contains(&name) {
                themes.push(name);
            }
        }
    }

    themes
}

fn find_in_theme(theme_dir: &Path, name: &str) -> Option<PathBuf> {
    if !theme_dir.is_dir() {
        return None;
    }

    // Fast path: the layouts every common theme actually uses.
    for size in ICON_SIZES {
        let numeric = size.split_once('x').map(|(value, _)| value).unwrap_or(size);
        for relative in [
            theme_dir.join(size).join("apps"),
            theme_dir.join("apps").join(size),
            theme_dir.join("apps").join(numeric),
        ] {
            for extension in ICON_EXTENSIONS {
                let candidate = relative.join(format!("{name}.{extension}"));
                if candidate.is_file() {
                    return Some(candidate);
                }
            }
        }
    }

    for extension in ICON_EXTENSIONS {
        for candidate in [
            theme_dir.join("apps").join(format!("{name}.{extension}")),
            theme_dir.join(format!("{name}.{extension}")),
        ] {
            if candidate.is_file() {
                return Some(candidate);
            }
        }
    }

    // Themes with an unusual layout: a bounded walk before giving up.
    search_recursive(theme_dir, name, 0)
}

fn search_recursive(dir: &Path, name: &str, depth: usize) -> Option<PathBuf> {
    if depth > 3 {
        return None;
    }
    let entries = fs::read_dir(dir).ok()?;
    let mut subdirs = Vec::new();

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            subdirs.push(path);
            continue;
        }
        let matches_name = path.file_stem().and_then(|stem| stem.to_str()) == Some(name);
        let renderable = path
            .extension()
            .and_then(|ext| ext.to_str())
            .is_some_and(|ext| ext == "png" || ext == "svg");
        if matches_name && renderable {
            return Some(path);
        }
    }

    subdirs
        .into_iter()
        .find_map(|subdir| search_recursive(&subdir, name, depth + 1))
}
