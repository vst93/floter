//! One-level, allow-listed tool discovery.
//!
//! Discovery never executes a candidate or recursively walks arbitrary PATH
//! entries. Symlink targets are validated, while the discovered link path is
//! retained because shim names such as `python` and `node` are meaningful.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
#[cfg(target_os = "linux")]
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

const DEFAULT_TTL: Duration = Duration::from_secs(5 * 60);

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "kebab-case")]
pub enum DiscoverySource {
    Path,
    Desktop,
    Dpkg,
    Rpm,
    Pacman,
    Flatpak,
    Snap,
    Nix,
    Brew,
    LaunchServices,
    Registry,
    Scoop,
    Chocolatey,
    WinGet,
    Wsl,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase", tag = "kind")]
pub enum ToolLocator {
    Executable {
        path: String,
    },
    DockerImage {
        reference: String,
        digest: Option<String>,
    },
    Flatpak {
        app_id: String,
    },
    Snap {
        name: String,
    },
}

impl ToolLocator {
    pub fn normalized(&self) -> String {
        match self {
            Self::Executable { path } => normalized_locator_path(path),
            Self::DockerImage { reference, digest } => format!(
                "docker:{}@{}",
                reference.trim().to_ascii_lowercase(),
                digest.as_deref().unwrap_or("").trim().to_ascii_lowercase()
            ),
            Self::Flatpak { app_id } => {
                format!("flatpak:{}", app_id.trim().to_ascii_lowercase())
            }
            Self::Snap { name } => format!("snap:{}", name.trim().to_ascii_lowercase()),
        }
    }

    pub fn executable_path(&self) -> Option<&Path> {
        match self {
            Self::Executable { path } => Some(Path::new(path)),
            _ => None,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolCandidate {
    pub id: String,
    pub name: String,
    pub locator: ToolLocator,
    pub version: Option<String>,
    pub sources: Vec<DiscoverySource>,
    pub quality: DiscoveryQuality,
    pub available: bool,
    pub fingerprint: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "kebab-case")]
pub enum DiscoveryQuality {
    OfficialAdapter,
    NativeSupport,
    AutoDetected,
    UserDefined,
    Inferred,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolInventorySnapshot {
    pub generated_at: u64,
    pub platform: String,
    pub candidates: Vec<ToolCandidate>,
}

#[derive(Debug)]
pub struct ToolInventory {
    snapshot: ToolInventorySnapshot,
    ttl: Duration,
    last_environment: String,
    refreshed_at: Instant,
}

impl ToolInventory {
    pub fn new() -> Self {
        Self {
            snapshot: ToolInventorySnapshot {
                generated_at: 0,
                platform: current_platform(),
                candidates: Vec::new(),
            },
            ttl: DEFAULT_TTL,
            last_environment: environment_signature(),
            refreshed_at: Instant::now() - DEFAULT_TTL,
        }
    }

    #[cfg(test)]
    pub fn with_ttl(ttl: Duration) -> Self {
        let mut inventory = Self::new();
        inventory.ttl = ttl;
        inventory
    }

    pub fn needs_refresh(&self) -> bool {
        self.snapshot.generated_at == 0
            || self.refreshed_at.elapsed() >= self.ttl
            || self.last_environment != environment_signature()
    }

    pub fn refresh(&mut self) -> &ToolInventorySnapshot {
        self.snapshot = discover_snapshot();
        self.last_environment = environment_signature();
        self.refreshed_at = Instant::now();
        &self.snapshot
    }

    pub fn refresh_if_needed(&mut self) -> &ToolInventorySnapshot {
        if self.needs_refresh() {
            self.refresh();
        }
        &self.snapshot
    }

    pub fn search(&mut self, query: &str) -> Vec<ToolCandidate> {
        let query = query.trim().to_ascii_lowercase();
        let mut matches: Vec<(u8, ToolCandidate)> = self
            .refresh_if_needed()
            .candidates
            .iter()
            .filter_map(|candidate| {
                candidate_match_score(candidate, &query).map(|score| (score, candidate.clone()))
            })
            .collect();
        matches.sort_by(|left, right| {
            left.0
                .cmp(&right.0)
                .then_with(|| left.1.quality.cmp(&right.1.quality))
                .then_with(|| left.1.name.len().cmp(&right.1.name.len()))
                .then_with(|| left.1.name.cmp(&right.1.name))
                .then_with(|| left.1.id.cmp(&right.1.id))
        });
        matches
            .into_iter()
            .map(|(_, candidate)| candidate)
            .collect()
    }
}

impl Default for ToolInventory {
    fn default() -> Self {
        Self::new()
    }
}

pub fn discover_snapshot() -> ToolInventorySnapshot {
    let mut candidates = BTreeMap::new();
    discover_path(&mut candidates);
    #[cfg(target_os = "linux")]
    discover_linux(&mut candidates);
    #[cfg(target_os = "macos")]
    discover_macos(&mut candidates);
    #[cfg(target_os = "windows")]
    discover_windows(&mut candidates);
    ToolInventorySnapshot {
        generated_at: unix_now(),
        platform: current_platform(),
        candidates: candidates.into_values().collect(),
    }
}

fn discover_path(candidates: &mut BTreeMap<String, ToolCandidate>) {
    for directory in path_directories() {
        discover_executable_directory(
            candidates,
            &directory,
            DiscoverySource::Path,
            DiscoveryQuality::AutoDetected,
        );
    }
}

fn discover_executable_directory(
    candidates: &mut BTreeMap<String, ToolCandidate>,
    directory: &Path,
    source: DiscoverySource,
    quality: DiscoveryQuality,
) {
    let Ok(entries) = fs::read_dir(directory) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if !is_executable(&path) {
            continue;
        }
        let Some(name) = path
            .file_name()
            .and_then(|value| value.to_str())
            .map(str::to_string)
        else {
            continue;
        };
        insert_executable(candidates, path, name, source.clone(), quality);
    }
}

#[cfg(target_os = "linux")]
fn discover_linux(candidates: &mut BTreeMap<String, ToolCandidate>) {
    discover_linux_desktop_entries(candidates, &linux_desktop_roots());

    if let Some(home) = dirs::home_dir() {
        discover_executable_directory(
            candidates,
            &home.join(".local/share/flatpak/exports/bin"),
            DiscoverySource::Flatpak,
            DiscoveryQuality::NativeSupport,
        );
        discover_executable_directory(
            candidates,
            &home.join(".nix-profile/bin"),
            DiscoverySource::Nix,
            DiscoveryQuality::NativeSupport,
        );
    }
    for (directory, source) in [
        ("/var/lib/flatpak/exports/bin", DiscoverySource::Flatpak),
        ("/snap/bin", DiscoverySource::Snap),
        ("/nix/var/nix/profiles/default/bin", DiscoverySource::Nix),
    ] {
        discover_executable_directory(
            candidates,
            Path::new(directory),
            source,
            DiscoveryQuality::NativeSupport,
        );
    }
}

#[cfg(target_os = "linux")]
fn linux_desktop_roots() -> Vec<PathBuf> {
    let data_home = std::env::var_os("XDG_DATA_HOME")
        .map(PathBuf::from)
        .filter(|path| path.is_absolute())
        .or_else(|| dirs::home_dir().map(|home| home.join(".local/share")));
    let mut roots = Vec::new();
    if let Some(data_home) = data_home {
        roots.push(data_home.join("applications"));
        roots.push(data_home.join("flatpak/exports/share/applications"));
    }
    for root in [
        "/usr/local/share/applications",
        "/usr/share/applications",
        "/var/lib/flatpak/exports/share/applications",
        "/var/lib/snapd/desktop/applications",
    ] {
        push_unique(&mut roots, PathBuf::from(root));
    }
    let data_dirs = std::env::var_os("XDG_DATA_DIRS")
        .map(|value| std::env::split_paths(&value).collect::<Vec<_>>())
        .unwrap_or_else(|| {
            vec![
                PathBuf::from("/usr/local/share"),
                PathBuf::from("/usr/share"),
            ]
        });
    for directory in data_dirs {
        if directory.is_absolute() {
            push_unique(&mut roots, directory.join("applications"));
        }
    }
    roots
}

#[cfg(target_os = "linux")]
fn push_unique(paths: &mut Vec<PathBuf>, path: PathBuf) {
    if !paths.contains(&path) {
        paths.push(path);
    }
}

#[cfg(target_os = "linux")]
fn discover_linux_desktop_entries(
    candidates: &mut BTreeMap<String, ToolCandidate>,
    roots: &[PathBuf],
) {
    let mut seen_ids = HashSet::new();
    for root in roots {
        collect_desktop_entries(candidates, root, root, 0, &mut seen_ids);
    }
}

#[cfg(target_os = "linux")]
fn collect_desktop_entries(
    candidates: &mut BTreeMap<String, ToolCandidate>,
    root: &Path,
    directory: &Path,
    depth: usize,
    seen_ids: &mut HashSet<String>,
) {
    if depth > 3 {
        return;
    }
    let Ok(entries) = fs::read_dir(directory) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let Ok(metadata) = fs::symlink_metadata(&path) else {
            continue;
        };
        if metadata.is_dir() && !metadata.file_type().is_symlink() {
            collect_desktop_entries(candidates, root, &path, depth + 1, seen_ids);
            continue;
        }
        if path.extension().and_then(|extension| extension.to_str()) != Some("desktop") {
            continue;
        }
        if metadata.file_type().is_symlink()
            && !fs::metadata(&path)
                .ok()
                .is_some_and(|target| target.is_file())
        {
            continue;
        }
        let id = desktop_file_id(root, &path);
        if !seen_ids.insert(id) {
            continue;
        }
        let Some(entry) = DesktopEntry::parse(&path) else {
            continue;
        };
        let Some(command) = entry.command() else {
            continue;
        };
        match command {
            DesktopCommand::Executable { path, name } => insert_executable(
                candidates,
                path,
                entry.name.unwrap_or(name),
                DiscoverySource::Desktop,
                DiscoveryQuality::NativeSupport,
            ),
            DesktopCommand::Flatpak { app_id } => insert(
                candidates,
                ToolLocator::Flatpak {
                    app_id: app_id.clone(),
                },
                entry.name.unwrap_or(app_id),
                DiscoverySource::Desktop,
                DiscoveryQuality::NativeSupport,
            ),
            DesktopCommand::Snap { name } => insert(
                candidates,
                ToolLocator::Snap { name: name.clone() },
                entry.name.unwrap_or(name),
                DiscoverySource::Desktop,
                DiscoveryQuality::NativeSupport,
            ),
        }
    }
}

#[cfg(target_os = "linux")]
fn desktop_file_id(root: &Path, path: &Path) -> String {
    path.strip_prefix(root)
        .unwrap_or(path)
        .to_string_lossy()
        .replace(['/', '\\'], "-")
}

#[cfg(target_os = "linux")]
struct DesktopEntry {
    name: Option<String>,
    exec: Option<String>,
    try_exec: Option<String>,
    entry_type: Option<String>,
    hidden: bool,
    no_display: bool,
}

#[cfg(target_os = "linux")]
impl DesktopEntry {
    fn parse(path: &Path) -> Option<Self> {
        let content = fs::read_to_string(path).ok()?;
        let mut entry = Self {
            name: None,
            exec: None,
            try_exec: None,
            entry_type: None,
            hidden: false,
            no_display: false,
        };
        let mut in_desktop_entry = false;
        for line in content.lines() {
            let line = line.trim().trim_start_matches('\u{feff}');
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if line.starts_with('[') {
                in_desktop_entry = line == "[Desktop Entry]";
                continue;
            }
            if !in_desktop_entry {
                continue;
            }
            let Some((key, value)) = line.split_once('=') else {
                continue;
            };
            let key = key.trim();
            let value = value.trim();
            match key {
                "Name" => entry.name = Some(desktop_unescape(value)),
                "Exec" => entry.exec = Some(desktop_unescape(value)),
                "TryExec" => entry.try_exec = Some(desktop_unescape(value)),
                "Type" => entry.entry_type = Some(value.to_string()),
                "Hidden" => entry.hidden = value.eq_ignore_ascii_case("true"),
                "NoDisplay" => entry.no_display = value.eq_ignore_ascii_case("true"),
                _ => {}
            }
        }
        Some(entry)
    }

    fn command(&self) -> Option<DesktopCommand> {
        if self.hidden
            || self.no_display
            || self
                .entry_type
                .as_deref()
                .is_some_and(|entry_type| entry_type != "Application")
        {
            return None;
        }
        if self
            .try_exec
            .as_deref()
            .is_some_and(|program| resolve_program(program).is_none())
        {
            return None;
        }
        let argv = parse_desktop_exec(self.exec.as_deref()?)?;
        desktop_command(&argv)
    }
}

#[cfg(target_os = "linux")]
enum DesktopCommand {
    Executable { path: PathBuf, name: String },
    Flatpak { app_id: String },
    Snap { name: String },
}

#[cfg(target_os = "linux")]
fn desktop_command(argv: &[String]) -> Option<DesktopCommand> {
    let argv = unwrap_env(argv)?;
    let program_name = Path::new(argv.first()?).file_name()?.to_str()?;
    if program_name == "flatpak" {
        let run = argv.iter().position(|argument| argument == "run")?;
        let app_id = argv[run + 1..]
            .iter()
            .find(|argument| !argument.starts_with('-'))?
            .clone();
        return Some(DesktopCommand::Flatpak { app_id });
    }
    if program_name == "snap" && argv.get(1).is_some_and(|argument| argument == "run") {
        return argv
            .get(2)
            .cloned()
            .map(|name| DesktopCommand::Snap { name });
    }
    // A shell command string cannot be converted back into a structured target.
    if matches!(program_name, "sh" | "bash" | "zsh" | "fish")
        && argv.get(1).is_some_and(|argument| argument == "-c")
    {
        return None;
    }
    let path = resolve_program(argv.first()?)?;
    let name = Path::new(argv.first()?)
        .file_name()?
        .to_string_lossy()
        .into_owned();
    Some(DesktopCommand::Executable { path, name })
}

#[cfg(target_os = "linux")]
fn unwrap_env(argv: &[String]) -> Option<&[String]> {
    let program = Path::new(argv.first()?).file_name()?.to_str()?;
    if program != "env" {
        return Some(argv);
    }
    let mut index = 1;
    while index < argv.len() {
        let argument = &argv[index];
        if matches!(argument.as_str(), "-u" | "--unset") {
            index += 2;
        } else if argument.starts_with('-') || argument.contains('=') {
            index += 1;
        } else {
            break;
        }
    }
    (index < argv.len()).then_some(&argv[index..])
}

#[cfg(target_os = "linux")]
fn parse_desktop_exec(exec: &str) -> Option<Vec<String>> {
    let mut arguments = Vec::new();
    let mut current = String::new();
    let mut chars = exec.chars();
    let mut quoted = false;
    while let Some(character) = chars.next() {
        match character {
            '"' => quoted = !quoted,
            '\\' => current.push(chars.next()?),
            character if character.is_whitespace() && !quoted => {
                push_desktop_argument(&mut arguments, &mut current)?;
            }
            _ => current.push(character),
        }
    }
    if quoted {
        return None;
    }
    push_desktop_argument(&mut arguments, &mut current)?;
    (!arguments.is_empty()).then_some(arguments)
}

#[cfg(target_os = "linux")]
fn push_desktop_argument(arguments: &mut Vec<String>, current: &mut String) -> Option<()> {
    if current.is_empty() {
        return Some(());
    }
    let stripped = strip_desktop_field_codes(current)?;
    if !stripped.is_empty() {
        arguments.push(stripped);
    }
    current.clear();
    Some(())
}

#[cfg(target_os = "linux")]
fn strip_desktop_field_codes(value: &str) -> Option<String> {
    let mut output = String::with_capacity(value.len());
    let mut chars = value.chars();
    while let Some(character) = chars.next() {
        if character != '%' {
            output.push(character);
            continue;
        }
        match chars.next()? {
            '%' => output.push('%'),
            'f' | 'F' | 'u' | 'U' | 'i' | 'c' | 'k' => {}
            _ => return None,
        }
    }
    Some(output)
}

#[cfg(target_os = "linux")]
fn desktop_unescape(value: &str) -> String {
    let mut output = String::with_capacity(value.len());
    let mut chars = value.chars();
    while let Some(character) = chars.next() {
        if character != '\\' {
            output.push(character);
            continue;
        }
        match chars.next() {
            Some('s') => output.push(' '),
            Some('n') => output.push('\n'),
            Some('t') => output.push('\t'),
            Some('r') => output.push('\r'),
            Some('\\') => output.push('\\'),
            Some(other) => output.push(other),
            None => output.push('\\'),
        }
    }
    output
}

#[cfg(target_os = "linux")]
fn resolve_program(program: &str) -> Option<PathBuf> {
    let program = Path::new(program.trim());
    if program.as_os_str().is_empty() {
        return None;
    }
    if program.components().count() > 1 {
        let path = absolute_path(program);
        return is_executable(&path).then_some(path);
    }
    path_directories()
        .into_iter()
        .map(|directory| directory.join(program))
        .find(|candidate| is_executable(candidate))
}

#[cfg(target_os = "macos")]
fn discover_macos(candidates: &mut BTreeMap<String, ToolCandidate>) {
    for directory in [
        PathBuf::from("/usr/local/bin"),
        PathBuf::from("/opt/homebrew/bin"),
    ] {
        discover_executable_directory(
            candidates,
            &directory,
            DiscoverySource::Brew,
            DiscoveryQuality::NativeSupport,
        );
    }
    for directory in [
        PathBuf::from("/Applications"),
        PathBuf::from("/System/Applications"),
        dirs::home_dir()
            .map(|home| home.join("Applications"))
            .unwrap_or_default(),
    ] {
        discover_macos_apps(candidates, &directory);
    }
}

#[cfg(target_os = "macos")]
fn discover_macos_apps(candidates: &mut BTreeMap<String, ToolCandidate>, directory: &Path) {
    let Ok(entries) = fs::read_dir(directory) else {
        return;
    };
    for entry in entries.flatten() {
        let bundle = entry.path();
        if bundle
            .extension()
            .and_then(|value| value.to_str())
            .is_none_or(|value| !value.eq_ignore_ascii_case("app"))
        {
            continue;
        }
        let executable = plist::Value::from_file(bundle.join("Contents/Info.plist"))
            .ok()
            .and_then(|value| value.as_dictionary().cloned())
            .and_then(|dict| {
                dict.get("CFBundleExecutable")
                    .and_then(|value| value.as_string())
                    .map(str::to_string)
            })
            .map(|name| bundle.join("Contents/MacOS").join(name))
            .filter(|path| is_executable(path));
        if let Some(path) = executable {
            let name = bundle
                .file_stem()
                .and_then(|value| value.to_str())
                .unwrap_or("Application")
                .to_string();
            insert_executable(
                candidates,
                path,
                name,
                DiscoverySource::LaunchServices,
                DiscoveryQuality::NativeSupport,
            );
        }
    }
}

#[cfg(target_os = "windows")]
fn discover_windows(candidates: &mut BTreeMap<String, ToolCandidate>) {
    discover_executable_directory(
        candidates,
        Path::new("C:\\ProgramData\\chocolatey\\bin"),
        DiscoverySource::Chocolatey,
        DiscoveryQuality::NativeSupport,
    );
    if let Some(home) = dirs::home_dir() {
        discover_executable_directory(
            candidates,
            &home.join("scoop/shims"),
            DiscoverySource::Scoop,
            DiscoveryQuality::NativeSupport,
        );
    }
    if let Some(local_app_data) = std::env::var_os("LOCALAPPDATA") {
        discover_executable_directory(
            candidates,
            &PathBuf::from(local_app_data).join("Microsoft/WinGet/Links"),
            DiscoverySource::WinGet,
            DiscoveryQuality::NativeSupport,
        );
    }
    if let Some(program_data) = std::env::var_os("ProgramData") {
        discover_executable_directory(
            candidates,
            &PathBuf::from(program_data).join("chocolatey/bin"),
            DiscoverySource::Chocolatey,
            DiscoveryQuality::NativeSupport,
        );
    }
}

fn insert_executable(
    candidates: &mut BTreeMap<String, ToolCandidate>,
    path: PathBuf,
    name: String,
    source: DiscoverySource,
    quality: DiscoveryQuality,
) {
    let path = absolute_path(&path);
    if !is_executable(&path) {
        return;
    }
    insert(
        candidates,
        ToolLocator::Executable {
            path: path.to_string_lossy().into_owned(),
        },
        name,
        source,
        quality,
    );
}

fn insert(
    candidates: &mut BTreeMap<String, ToolCandidate>,
    locator: ToolLocator,
    name: String,
    source: DiscoverySource,
    quality: DiscoveryQuality,
) {
    let key = locator.normalized();
    let fingerprint = locator.executable_path().and_then(fingerprint);
    let entry = candidates
        .entry(key.clone())
        .or_insert_with(|| ToolCandidate {
            id: key,
            name,
            locator,
            version: None,
            sources: Vec::new(),
            quality,
            available: true,
            fingerprint: fingerprint.clone(),
        });
    if !entry.sources.contains(&source) {
        entry.sources.push(source);
        entry.sources.sort();
    }
    if quality < entry.quality {
        entry.quality = quality;
    }
    entry.fingerprint = fingerprint;
}

fn candidate_match_score(candidate: &ToolCandidate, query: &str) -> Option<u8> {
    let name = candidate.name.to_ascii_lowercase();
    if query.is_empty() || name == query {
        return Some(0);
    }
    if name.starts_with(query) {
        return Some(1);
    }
    if name.contains(query) {
        return Some(2);
    }
    if is_subsequence(&name, query) {
        return Some(3);
    }
    (candidate.id.to_ascii_lowercase().contains(query)
        || candidate.locator.normalized().contains(query))
    .then_some(4)
}

fn is_subsequence(candidate: &str, query: &str) -> bool {
    let mut query = query.chars();
    let Some(mut expected) = query.next() else {
        return true;
    };
    for character in candidate.chars() {
        if character == expected {
            let Some(next) = query.next() else {
                return true;
            };
            expected = next;
        }
    }
    false
}

fn path_directories() -> Vec<PathBuf> {
    std::env::var_os("PATH")
        .map(|path| std::env::split_paths(&path).collect())
        .unwrap_or_default()
}

fn absolute_path(path: &Path) -> PathBuf {
    if path.is_absolute() {
        path.to_path_buf()
    } else {
        std::env::current_dir()
            .map(|current| current.join(path))
            .unwrap_or_else(|_| path.to_path_buf())
    }
}

fn normalized_locator_path(path: &str) -> String {
    let path = absolute_path(Path::new(path))
        .to_string_lossy()
        .into_owned();
    #[cfg(target_os = "windows")]
    {
        path.to_ascii_lowercase()
    }
    #[cfg(not(target_os = "windows"))]
    {
        path
    }
}

fn fingerprint(path: &Path) -> Option<String> {
    let target = fs::canonicalize(path).ok()?;
    let metadata = fs::metadata(&target).ok()?;
    let modified = metadata
        .modified()
        .ok()?
        .duration_since(UNIX_EPOCH)
        .ok()?
        .as_nanos();
    Some(format!(
        "{}:{}:{}",
        target.to_string_lossy(),
        metadata.len(),
        modified
    ))
}

fn is_executable(path: &Path) -> bool {
    let Ok(link_metadata) = fs::symlink_metadata(path) else {
        return false;
    };
    if !link_metadata.is_file() && !link_metadata.file_type().is_symlink() {
        return false;
    }
    let Ok(target_metadata) = fs::metadata(path) else {
        return false;
    };
    if !target_metadata.is_file() {
        return false;
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        target_metadata.permissions().mode() & 0o111 != 0
    }
    #[cfg(windows)]
    {
        let executable_extension = |candidate: &Path| {
            candidate
                .extension()
                .and_then(|extension| extension.to_str())
                .is_some_and(|extension| {
                    matches!(
                        extension.to_ascii_lowercase().as_str(),
                        "exe" | "com" | "bat" | "cmd"
                    )
                })
        };
        executable_extension(path)
            || fs::canonicalize(path)
                .ok()
                .as_deref()
                .is_some_and(executable_extension)
    }
    #[cfg(not(any(unix, windows)))]
    {
        true
    }
}

fn current_platform() -> String {
    super::platform::PlatformTarget::current()
        .map(|target| target.identifier())
        .unwrap_or_else(|_| std::env::consts::OS.to_string())
}

fn environment_signature() -> String {
    ["PATH", "SHELL", "XDG_DATA_HOME", "XDG_DATA_DIRS"]
        .into_iter()
        .map(|name| std::env::var(name).unwrap_or_default())
        .collect::<Vec<_>>()
        .join("|")
}

fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn locator_normalization_separates_docker_from_executable() {
        assert_ne!(
            ToolLocator::DockerImage {
                reference: "Acme/Tool".into(),
                digest: None
            }
            .normalized(),
            "acme/tool"
        );
    }

    #[test]
    fn candidate_serialization_matches_the_frontend_contract() {
        let candidate = ToolCandidate {
            id: "/usr/bin/floter-tool".into(),
            name: "floter-tool".into(),
            locator: ToolLocator::Executable {
                path: "/usr/bin/floter-tool".into(),
            },
            version: Some("1.2.3".into()),
            sources: vec![DiscoverySource::LaunchServices],
            quality: DiscoveryQuality::NativeSupport,
            available: true,
            fingerprint: Some("fingerprint".into()),
        };

        assert_eq!(
            serde_json::to_value(candidate).unwrap(),
            serde_json::json!({
                "id": "/usr/bin/floter-tool",
                "name": "floter-tool",
                "locator": {
                    "kind": "executable",
                    "path": "/usr/bin/floter-tool"
                },
                "version": "1.2.3",
                "sources": ["launch-services"],
                "quality": "native-support",
                "available": true,
                "fingerprint": "fingerprint"
            })
        );
    }

    #[test]
    fn empty_inventory_refreshes() {
        assert!(ToolInventory::with_ttl(Duration::from_secs(60)).needs_refresh());
    }

    #[test]
    fn fuzzy_search_scores_exact_prefix_contains_and_subsequence() {
        let candidate = |name: &str| ToolCandidate {
            id: format!("/bin/{name}"),
            name: name.into(),
            locator: ToolLocator::Executable {
                path: format!("/bin/{name}"),
            },
            version: None,
            sources: vec![DiscoverySource::Path],
            quality: DiscoveryQuality::AutoDetected,
            available: true,
            fingerprint: None,
        };
        assert_eq!(candidate_match_score(&candidate("cargo"), "cargo"), Some(0));
        assert_eq!(
            candidate_match_score(&candidate("cargo-clippy"), "cargo"),
            Some(1)
        );
        assert_eq!(
            candidate_match_score(&candidate("my-cargo-tool"), "cargo"),
            Some(2)
        );
        assert_eq!(candidate_match_score(&candidate("cargo"), "cgo"), Some(3));
        assert_eq!(candidate_match_score(&candidate("cargo"), "xyz"), None);
    }

    #[cfg(unix)]
    #[test]
    fn executable_symlink_keeps_its_discovered_name_and_path() {
        use std::os::unix::fs::{symlink, PermissionsExt};

        let temporary = tempfile::tempdir().unwrap();
        let target = temporary.path().join("runtime-v1");
        fs::write(&target, "#!/bin/sh\n").unwrap();
        fs::set_permissions(&target, fs::Permissions::from_mode(0o755)).unwrap();
        let shim = temporary.path().join("runtime");
        symlink(&target, &shim).unwrap();

        let mut candidates = BTreeMap::new();
        discover_executable_directory(
            &mut candidates,
            temporary.path(),
            DiscoverySource::Path,
            DiscoveryQuality::AutoDetected,
        );
        let candidate = candidates
            .values()
            .find(|candidate| candidate.name == "runtime")
            .unwrap();
        assert_eq!(candidate.locator.executable_path(), Some(shim.as_path()));
    }

    #[cfg(unix)]
    #[test]
    fn broken_and_directory_symlinks_are_not_candidates() {
        use std::os::unix::fs::symlink;

        let temporary = tempfile::tempdir().unwrap();
        symlink(
            temporary.path().join("missing"),
            temporary.path().join("broken"),
        )
        .unwrap();
        symlink(temporary.path(), temporary.path().join("directory-link")).unwrap();
        let mut candidates = BTreeMap::new();
        discover_executable_directory(
            &mut candidates,
            temporary.path(),
            DiscoverySource::Path,
            DiscoveryQuality::AutoDetected,
        );
        assert!(candidates.is_empty());
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn desktop_exec_handles_env_quotes_and_field_codes() {
        let argv =
            parse_desktop_exec("env MODE=test \"/opt/My Tool/bin/tool\" --open=%U %%").unwrap();
        assert_eq!(
            argv,
            ["env", "MODE=test", "/opt/My Tool/bin/tool", "--open=", "%"]
        );
        assert_eq!(unwrap_env(&argv).unwrap()[0], "/opt/My Tool/bin/tool");
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn desktop_parser_ignores_actions_and_hidden_entries() {
        let temporary = tempfile::tempdir().unwrap();
        let path = temporary.path().join("hidden.desktop");
        fs::write(
            &path,
            "[Desktop Entry]\nType=Application\nName=Hidden tool\nExec=/bin/sh\nHidden=true\n[Desktop Action Other]\nExec=/bin/true\n",
        )
        .unwrap();
        let entry = DesktopEntry::parse(&path).unwrap();
        assert_eq!(entry.name.as_deref(), Some("Hidden tool"));
        assert!(entry.command().is_none());
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn desktop_parser_accepts_a_file_symlink() {
        use std::os::unix::fs::symlink;

        let temporary = tempfile::tempdir().unwrap();
        let target = temporary.path().join("target.desktop");
        fs::write(
            &target,
            "[Desktop Entry]\nType=Application\nName=Linked tool\nExec=/bin/true\n",
        )
        .unwrap();
        let link = temporary.path().join("linked.desktop");
        symlink(&target, &link).unwrap();
        assert_eq!(
            DesktopEntry::parse(&link).and_then(|entry| entry.name),
            Some("Linked tool".to_string())
        );
    }
}
