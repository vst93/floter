//! Windows application discovery: Start Menu shortcuts, with installed
//! programs from the uninstall registry as a second source.
//!
//! `.lnk` files are parsed directly (the `LinkInfo` structure of
//! MS-SHLLINK) rather than through a crate: the launcher only needs the target
//! path, and a shortcut whose target cannot be read is still listed and still
//! launchable, because Windows resolves it itself when the file is opened.

use std::collections::HashSet;
use std::fs;
use std::os::windows::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;
use tauri::AppHandle;
use windows::core::{w, PCWSTR};
use windows::Win32::Foundation::FILETIME;
use windows::Win32::System::Registry::{
    RegCloseKey, RegOpenKeyExW, RegQueryInfoKeyW, HKEY, HKEY_CURRENT_USER, HKEY_LOCAL_MACHINE,
    KEY_READ,
};
use windows::Win32::UI::Shell::ShellExecuteW;
use windows::Win32::UI::WindowsAndMessaging::SW_SHOWNORMAL;

use super::{
    cached_icon_is_fresh, icon_cache_path, mark_icon_cached, paths_signature, LocalApplication,
};

/// Keeps the helper processes (`reg`, `powershell`) from flashing a console.
const CREATE_NO_WINDOW: u32 = 0x0800_0000;

const UNINSTALL_KEYS: [&str; 3] = [
    r"HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall",
    r"HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall",
    r"HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall",
];

pub fn roots() -> Vec<PathBuf> {
    let mut roots = Vec::new();
    let suffix = r"Microsoft\Windows\Start Menu\Programs";

    if let Some(app_data) = std::env::var_os("APPDATA") {
        roots.push(PathBuf::from(app_data).join(suffix));
    }
    if let Some(program_data) = std::env::var_os("PROGRAMDATA") {
        roots.push(PathBuf::from(program_data).join(suffix));
    }

    roots
}

pub fn scan(roots: &[PathBuf]) -> Vec<LocalApplication> {
    let mut apps = Vec::new();
    let mut seen_targets = HashSet::new();
    let mut seen_names = HashSet::new();

    for dir in roots {
        collect_shortcuts(dir, 0, &mut seen_targets, &mut seen_names, &mut apps);
    }

    collect_registry_programs(&mut seen_targets, &mut seen_names, &mut apps);
    apps
}

pub fn source_signature(roots: &[PathBuf]) -> u64 {
    let mut sources = roots.to_vec();
    for root in roots {
        collect_shortcut_paths(root, 0, &mut sources);
    }
    UNINSTALL_KEYS
        .iter()
        .map(|key| registry_key_last_write(key))
        .fold(paths_signature(sources), |signature, last_write| {
            signature.rotate_left(11) ^ last_write
        })
}

pub fn signature_check_interval() -> Duration {
    Duration::from_secs(60)
}

/// Parent-key timestamps catch normal installs and uninstalls. Keep a periodic
/// rebuild as a fallback for installers that only edit values in an existing
/// child key, which does not necessarily update the parent timestamp.
pub fn max_cache_age() -> Option<Duration> {
    Some(Duration::from_secs(30 * 60))
}

pub fn open(path: &Path) -> Result<(), String> {
    use std::os::windows::ffi::OsStrExt;

    let wide = path
        .as_os_str()
        .encode_wide()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();
    let result = unsafe {
        ShellExecuteW(
            None,
            w!("open"),
            PCWSTR(wide.as_ptr()),
            PCWSTR::null(),
            PCWSTR::null(),
            SW_SHOWNORMAL,
        )
    };
    let code = result.0 as isize;
    (code > 32)
        .then_some(())
        .ok_or_else(|| format!("ShellExecuteW failed with code {code}"))
}

pub fn icon_path(app: &AppHandle, path: &Path, source_hint: Option<&str>) -> Option<String> {
    let target = icon_cache_path(app, path, "png")?;
    // Old caches may not carry a target hint. Leave shortcut resolution to the
    // shell in that case: `WScript.Shell` expands environment variables too.
    let fallback_source = path.to_string_lossy();
    let source = source_hint.unwrap_or(fallback_source.as_ref());
    let source_path = Path::new(source);
    if cached_icon_is_fresh(&target, source_path) {
        return Some(target.to_string_lossy().to_string());
    }
    let script = format!(
        "$ErrorActionPreference='Stop';\
         $source='{source}';\
         if ($source.ToLower().EndsWith('.lnk')) {{\
           $shell = New-Object -ComObject WScript.Shell;\
           $source = $shell.CreateShortcut($source).TargetPath;\
         }}\
         if (-not $source) {{ exit 1 }};\
         Add-Type -AssemblyName System.Drawing;\
         $icon = [System.Drawing.Icon]::ExtractAssociatedIcon($source);\
         if (-not $icon) {{ exit 1 }};\
         $icon.ToBitmap().Save('{target}', [System.Drawing.Imaging.ImageFormat]::Png);",
        source = powershell_literal(source),
        target = powershell_literal(&target.to_string_lossy()),
    );

    let status = Command::new("powershell")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-WindowStyle",
            "Hidden",
            "-Command",
        ])
        .arg(&script)
        .creation_flags(CREATE_NO_WINDOW)
        .status()
        .ok()?;

    if status.success() && target.exists() {
        mark_icon_cached(&target, source_path);
        Some(target.to_string_lossy().to_string())
    } else {
        None
    }
}

/// Escape a value for a PowerShell single-quoted string literal.
fn powershell_literal(value: &str) -> String {
    value.replace('\'', "''")
}

fn collect_shortcuts(
    dir: &Path,
    depth: usize,
    seen_targets: &mut HashSet<String>,
    seen_names: &mut HashSet<String>,
    apps: &mut Vec<LocalApplication>,
) {
    // Vendors nest their folders a couple of levels below `Programs`.
    if depth > 4 || !dir.is_dir() {
        return;
    }

    let entries = match fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(_) => return,
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect_shortcuts(&path, depth + 1, seen_targets, seen_names, apps);
            continue;
        }

        let is_shortcut = path
            .extension()
            .and_then(|ext| ext.to_str())
            .is_some_and(|ext| ext.eq_ignore_ascii_case("lnk"));
        if !is_shortcut {
            continue;
        }

        let Some(name) = path.file_stem().and_then(|stem| stem.to_str()) else {
            continue;
        };

        let target = shortcut_target(&path);
        let target_key = target.as_deref().map(str::to_lowercase);
        if let Some(target) = target.as_deref() {
            if !is_launchable_target(target) {
                continue;
            }
            if target_key
                .as_ref()
                .is_some_and(|key| seen_targets.contains(key))
            {
                continue;
            }
        }
        let name_key = name.to_lowercase();
        if seen_names.contains(&name_key) {
            continue;
        }
        if let Some(target_key) = target_key {
            seen_targets.insert(target_key);
        }
        seen_names.insert(name_key);

        apps.push(LocalApplication {
            name: name.to_string(),
            localized_name: None,
            path: path.to_string_lossy().to_string(),
            icon_path: target.clone(),
            comment: None,
            // Filled in by `list_applications` once the scan is done, so the
            // pinyin lookup lives in one place rather than in every scanner.
            initials: String::new(),
            // A shortcut named in Chinese still points at a program with a Latin
            // name — "企业微信.lnk" runs `WXWork.exe` — so the target is what
            // keeps the entry reachable from a Latin keyboard.
            aliases: super::build_aliases(name, None, executable_name(target.as_deref())),
        });
    }
}

fn collect_shortcut_paths(dir: &Path, depth: usize, paths: &mut Vec<PathBuf>) {
    if depth > 4 || !dir.is_dir() {
        return;
    }
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect_shortcut_paths(&path, depth + 1, paths);
        } else if path
            .extension()
            .and_then(|ext| ext.to_str())
            .is_some_and(|ext| ext.eq_ignore_ascii_case("lnk"))
        {
            paths.push(path);
        }
    }
}

/// Uninstall entries fill the gaps left by programs that ship no Start Menu
/// shortcut. Only entries that resolve to a real executable are kept — the key
/// also holds updaters, runtimes and driver packages that cannot be launched.
fn collect_registry_programs(
    seen_targets: &mut HashSet<String>,
    seen_names: &mut HashSet<String>,
    apps: &mut Vec<LocalApplication>,
) {
    let outputs = std::thread::scope(|scope| {
        let queries = UNINSTALL_KEYS
            .iter()
            .map(|key| scope.spawn(move || query_registry(key)))
            .collect::<Vec<_>>();
        queries
            .into_iter()
            .filter_map(|query| query.join().ok().flatten())
            .collect::<Vec<_>>()
    });

    for output in outputs {
        let text = decode_registry_output(&output);
        for program in parse_registry_programs(&text) {
            let Some(executable) = program.executable() else {
                continue;
            };
            let path = executable.to_string_lossy().to_string();
            let target_key = path.to_lowercase();
            let name_key = program.display_name.to_lowercase();
            if seen_targets.contains(&target_key) {
                continue;
            }
            if seen_names.contains(&name_key) {
                continue;
            }
            seen_targets.insert(target_key);
            seen_names.insert(name_key);
            // The registry lists a program by its display name and its
            // executable; the second is the Latin key for the first.
            let aliases = super::build_aliases(
                &program.display_name,
                None,
                executable_name(Some(path.as_str())),
            );
            apps.push(LocalApplication {
                name: program.display_name,
                localized_name: None,
                path: path.clone(),
                icon_path: Some(path.clone()),
                comment: None,
                // Filled in by `list_applications` once the scan is done, so the
                // pinyin lookup lives in one place rather than in every scanner.
                initials: String::new(),
                aliases,
            });
        }
    }
}

/// The program a shortcut points at, without its path or extension:
/// `C:\Program Files\WXWork\WXWork.exe` is searchable as `WXWork`.
fn executable_name(target: Option<&str>) -> Option<String> {
    let stem = Path::new(target?).file_stem()?.to_str()?;
    (!stem.is_empty()).then(|| stem.to_string())
}

fn registry_key_last_write(key: &str) -> u64 {
    let (root, subkey) = if let Some(subkey) = key.strip_prefix("HKCU\\") {
        (HKEY_CURRENT_USER, subkey)
    } else if let Some(subkey) = key.strip_prefix("HKLM\\") {
        (HKEY_LOCAL_MACHINE, subkey)
    } else {
        return 0;
    };
    let wide = subkey
        .encode_utf16()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();
    let mut handle = HKEY(std::ptr::null_mut());
    let opened = unsafe { RegOpenKeyExW(root, PCWSTR(wide.as_ptr()), None, KEY_READ, &mut handle) };
    if opened.0 != 0 {
        return 0;
    }

    let mut last_write = FILETIME::default();
    let queried = unsafe {
        RegQueryInfoKeyW(
            handle,
            None,
            None,
            None,
            None,
            None,
            None,
            None,
            None,
            None,
            None,
            Some(&mut last_write),
        )
    };
    let _ = unsafe { RegCloseKey(handle) };
    if queried.0 != 0 {
        return 0;
    }
    (u64::from(last_write.dwHighDateTime) << 32) | u64::from(last_write.dwLowDateTime)
}

fn query_registry(key: &str) -> Option<Vec<u8>> {
    let temporary = tempfile::NamedTempFile::new().ok()?;
    let output_path = temporary.path().to_string_lossy();
    let script = format!(
        "& reg.exe query '{}' /s | Out-File -LiteralPath '{}' -Encoding Unicode",
        powershell_literal(key),
        powershell_literal(&output_path),
    );
    let status = Command::new("powershell")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-WindowStyle",
            "Hidden",
            "-Command",
        ])
        .arg(script)
        .creation_flags(CREATE_NO_WINDOW)
        .status()
        .ok()?;
    status
        .success()
        .then(|| fs::read(temporary.path()).ok())
        .flatten()
}

fn decode_registry_output(bytes: &[u8]) -> String {
    let (bytes, little_endian) = if bytes.starts_with(&[0xff, 0xfe]) {
        (&bytes[2..], true)
    } else if bytes.starts_with(&[0xfe, 0xff]) {
        (&bytes[2..], false)
    } else {
        let pairs = bytes.chunks_exact(2).take(64).collect::<Vec<_>>();
        let looks_utf16 =
            !pairs.is_empty() && pairs.iter().filter(|pair| pair[1] == 0).count() > pairs.len() / 2;
        if !looks_utf16 {
            return String::from_utf8_lossy(bytes).into_owned();
        }
        (bytes, true)
    };
    let units = bytes.chunks_exact(2).map(|chunk| {
        if little_endian {
            u16::from_le_bytes([chunk[0], chunk[1]])
        } else {
            u16::from_be_bytes([chunk[0], chunk[1]])
        }
    });
    String::from_utf16_lossy(&units.collect::<Vec<_>>())
}

#[derive(Default)]
struct RegistryProgram {
    display_name: String,
    display_icon: Option<String>,
    install_location: Option<String>,
    system_component: bool,
}

impl RegistryProgram {
    fn executable(&self) -> Option<PathBuf> {
        if self.system_component || self.display_name.is_empty() {
            return None;
        }

        // `DisplayIcon` is usually the main executable, optionally followed by
        // the icon index (`C:\app\app.exe,0`).
        if let Some(icon) = self.display_icon.as_deref() {
            let icon = icon.trim().trim_matches('"');
            let candidate = match icon.rsplit_once(',') {
                Some((path, index))
                    if index
                        .trim_start_matches('-')
                        .chars()
                        .all(|c| c.is_ascii_digit()) =>
                {
                    path
                }
                _ => icon,
            };
            let candidate = PathBuf::from(candidate.trim().trim_matches('"'));
            if is_existing_executable(&candidate) {
                return Some(candidate);
            }
        }

        // Otherwise look for an executable named after the program.
        let install_location = self.install_location.as_deref()?;
        let dir = PathBuf::from(install_location.trim().trim_matches('"'));
        if !dir.is_dir() {
            return None;
        }
        let needle = normalized_key(&self.display_name);
        fs::read_dir(&dir)
            .ok()?
            .flatten()
            .map(|entry| entry.path())
            .find(|path| {
                is_existing_executable(path)
                    && path
                        .file_stem()
                        .and_then(|stem| stem.to_str())
                        .map(normalized_key)
                        .is_some_and(|stem| !stem.is_empty() && needle.contains(&stem))
            })
    }
}

fn normalized_key(value: &str) -> String {
    value
        .chars()
        .filter(|c| c.is_alphanumeric())
        .flat_map(char::to_lowercase)
        .collect()
}

fn is_existing_executable(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .is_some_and(|ext| ext.eq_ignore_ascii_case("exe"))
        && path.is_file()
}

/// Parse the flat text `reg query ... /s` prints: a key line, then one indented
/// `name    REG_TYPE    value` line per value.
fn parse_registry_programs(text: &str) -> Vec<RegistryProgram> {
    let mut programs = Vec::new();
    let mut current = RegistryProgram::default();

    for line in text.lines() {
        if line.starts_with("HKEY_") {
            if !current.display_name.is_empty() {
                programs.push(std::mem::take(&mut current));
            } else {
                current = RegistryProgram::default();
            }
            continue;
        }

        let trimmed = line.trim_start();
        if trimmed.is_empty() {
            continue;
        }
        let mut parts = trimmed.splitn(3, "    ");
        let (Some(name), Some(_kind), Some(value)) = (parts.next(), parts.next(), parts.next())
        else {
            continue;
        };
        let value = value.trim();

        match name.trim() {
            "DisplayName" => current.display_name = value.to_string(),
            "DisplayIcon" => current.display_icon = Some(value.to_string()),
            "InstallLocation" if !value.is_empty() => {
                current.install_location = Some(value.to_string())
            }
            "SystemComponent" => {
                current.system_component = value.ends_with('1');
            }
            _ => {}
        }
    }

    if !current.display_name.is_empty() {
        programs.push(current);
    }

    programs
}

/// Filter out the help files, uninstallers and web links that share the Start
/// Menu with real applications.
fn is_launchable_target(target: &str) -> bool {
    let lower = target.to_lowercase();
    if !lower.ends_with(".exe") && !lower.ends_with(".bat") && !lower.ends_with(".cmd") {
        return false;
    }
    let stem = Path::new(&lower)
        .file_stem()
        .and_then(|stem| stem.to_str())
        .unwrap_or_default();
    !stem.contains("uninst") && !stem.contains("unins")
}

/// Read the target path out of a `.lnk` file.
///
/// Only the `LinkInfo` structure is decoded (MS-SHLLINK §2.3): it is the part
/// that carries a plain local path. Shortcuts that store their target only as
/// an item-id list or an environment-variable block return `None`, and the
/// caller keeps them anyway.
fn shortcut_target(path: &Path) -> Option<String> {
    const HEADER_SIZE: usize = 0x4C;
    const HAS_LINK_TARGET_ID_LIST: u32 = 0x1;
    const HAS_LINK_INFO: u32 = 0x2;
    const VOLUME_ID_AND_LOCAL_BASE_PATH: u32 = 0x1;

    let data = fs::read(path).ok()?;
    if data.len() < HEADER_SIZE || read_u32(&data, 0)? != HEADER_SIZE as u32 {
        return None;
    }

    let flags = read_u32(&data, 20)?;
    let mut cursor = HEADER_SIZE;

    if flags & HAS_LINK_TARGET_ID_LIST != 0 {
        let id_list_size = read_u16(&data, cursor)? as usize;
        cursor = cursor.checked_add(2)?.checked_add(id_list_size)?;
    }
    if flags & HAS_LINK_INFO == 0 || cursor >= data.len() {
        return None;
    }

    let info = data.get(cursor..)?;
    let info_size = read_u32(info, 0)? as usize;
    if info_size > info.len() {
        return None;
    }
    let info = info.get(..info_size)?;

    let header_size = read_u32(info, 4)? as usize;
    let info_flags = read_u32(info, 8)?;
    if info_flags & VOLUME_ID_AND_LOCAL_BASE_PATH == 0 {
        return None;
    }

    let ansi_base = read_u32(info, 16)? as usize;
    let ansi_suffix = read_u32(info, 24)? as usize;
    let unicode_base = if header_size >= 0x24 {
        read_u32(info, 28)? as usize
    } else {
        0
    };
    let unicode_suffix = if header_size >= 0x24 {
        read_u32(info, 32)? as usize
    } else {
        0
    };
    let base = if unicode_base == 0 {
        read_link_string(info, ansi_base, false)
    } else {
        read_link_string(info, unicode_base, true)
    }?;
    let suffix = if unicode_suffix == 0 {
        read_link_string(info, ansi_suffix, false)
    } else {
        read_link_string(info, unicode_suffix, true)
    }
    .unwrap_or_default();
    let target = format!("{base}{suffix}");
    (!target.is_empty()).then_some(target)
}

fn read_link_string(info: &[u8], offset: usize, unicode: bool) -> Option<String> {
    if offset == 0 || offset >= info.len() {
        return None;
    }
    let bytes = info.get(offset..)?;

    if unicode {
        let units: Vec<u16> = bytes
            .chunks_exact(2)
            .map(|chunk| u16::from_le_bytes([chunk[0], chunk[1]]))
            .take_while(|unit| *unit != 0)
            .collect();
        return Some(String::from_utf16_lossy(&units));
    }

    let end = bytes
        .iter()
        .position(|byte| *byte == 0)
        .unwrap_or(bytes.len());
    Some(String::from_utf8_lossy(&bytes[..end]).to_string())
}

fn read_u16(data: &[u8], offset: usize) -> Option<u16> {
    let bytes = data.get(offset..offset + 2)?;
    Some(u16::from_le_bytes([bytes[0], bytes[1]]))
}

fn read_u32(data: &[u8], offset: usize) -> Option<u32> {
    let bytes = data.get(offset..offset + 4)?;
    Some(u32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]))
}
