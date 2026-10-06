#[cfg(target_os = "windows")]
fn apply_launch_at_startup(enabled: bool) -> Result<(), String> {
    use std::os::windows::ffi::OsStrExt;
    use windows::core::w;
    use windows::Win32::Foundation::{ERROR_FILE_NOT_FOUND, ERROR_SUCCESS};
    use windows::Win32::System::Registry::{
        RegCloseKey, RegCreateKeyW, RegDeleteValueW, RegOpenKeyExW, RegSetValueExW, HKEY,
        HKEY_CURRENT_USER, KEY_SET_VALUE, REG_SZ,
    };

    const ACTION: &str = "update Windows startup registration";
    let mut key = HKEY::default();

    if enabled {
        let executable = std::env::current_exe().map_err(|error| error.to_string())?;
        let mut command = Vec::new();
        command.push(u16::from(b'"'));
        command.extend(executable.as_os_str().encode_wide());
        command.extend("\" --background".encode_utf16());
        command.push(0);

        let status = unsafe {
            RegCreateKeyW(
                HKEY_CURRENT_USER,
                w!("Software\\Microsoft\\Windows\\CurrentVersion\\Run"),
                &mut key,
            )
        };
        if status != ERROR_SUCCESS {
            return Err(format!("Failed to {ACTION} (error {})", status.0));
        }

        let data =
            unsafe { std::slice::from_raw_parts(command.as_ptr().cast::<u8>(), command.len() * 2) };
        // R123 · read-compare-write is a file rule: this branch goes through
        // the registry API, where `RegSetValueExW` is the only handle on the
        // existing value. Left as it is (registered in the R123 report).
        let status = unsafe { RegSetValueExW(key, w!("floter"), None, REG_SZ, Some(data)) };
        unsafe {
            let _ = RegCloseKey(key);
        }
        if status != ERROR_SUCCESS {
            return Err(format!("Failed to {ACTION} (error {})", status.0));
        }
        return Ok(());
    }

    let status = unsafe {
        RegOpenKeyExW(
            HKEY_CURRENT_USER,
            w!("Software\\Microsoft\\Windows\\CurrentVersion\\Run"),
            None,
            KEY_SET_VALUE,
            &mut key,
        )
    };
    if status == ERROR_FILE_NOT_FOUND {
        return Ok(());
    }
    if status != ERROR_SUCCESS {
        return Err(format!("Failed to {ACTION} (error {})", status.0));
    }

    let status = unsafe { RegDeleteValueW(key, w!("floter")) };
    unsafe {
        let _ = RegCloseKey(key);
    }
    if status == ERROR_SUCCESS || status == ERROR_FILE_NOT_FOUND {
        Ok(())
    } else {
        Err(format!("Failed to {ACTION} (error {})", status.0))
    }
}

#[cfg(target_os = "macos")]
fn apply_launch_at_startup(enabled: bool) -> Result<(), String> {
    use plist::{Dictionary, Value};

    let home = dirs::home_dir().ok_or("Cannot find home directory")?;
    let launch_agents = home.join("Library").join("LaunchAgents");
    let path = launch_agents.join("com.v.floter.plist");
    if !enabled {
        return remove_file_if_present(&path);
    }

    let executable = std::env::current_exe().map_err(|error| error.to_string())?;
    let executable = executable
        .to_str()
        .ok_or("Application path is not valid Unicode")?;

    // A per-user LaunchAgent avoids the helper bundle required by
    // SMLoginItemSetEnabled while retaining normal non-sandboxed app behavior.
    let mut document = Dictionary::new();
    document.insert(
        "Label".to_string(),
        Value::String("com.v.floter".to_string()),
    );
    document.insert(
        "ProgramArguments".to_string(),
        Value::Array(vec![
            Value::String(executable.to_string()),
            Value::String("--background".to_string()),
        ]),
    );
    document.insert("RunAtLoad".to_string(), Value::Boolean(true));
    document.insert(
        "LimitLoadToSessionType".to_string(),
        Value::String("Aqua".to_string()),
    );
    let document = Value::Dictionary(document);
    // R123 · steady state writes nothing, the same read-compare-write as the
    // Linux branch. The entry is a plist, so the comparison is on the parsed
    // value rather than on raw bytes (the serializer's own whitespace is not
    // part of the entry) — a file whose parsed value already equals this one
    // returns before `create_dir_all` and the write. An absent, unreadable, or
    // foreign file falls through and is overwritten, which is also what creates
    // the LaunchAgents directory the first time.
    if let Ok(existing) = Value::from_file(&path) {
        if existing == document {
            return Ok(());
        }
    }
    std::fs::create_dir_all(&launch_agents).map_err(|error| error.to_string())?;
    plist::to_file_xml(path, &document).map_err(|error| error.to_string())
}

#[cfg(target_os = "linux")]
fn apply_launch_at_startup(enabled: bool) -> Result<(), String> {
    let config = dirs::config_dir().ok_or("Cannot find config directory")?;
    let autostart_dir = config.join("autostart");
    let path = autostart_dir.join("floter.desktop");
    if !enabled {
        return remove_file_if_present(&path);
    }

    // AppImage's current_exe() is inside a temporary mount; APPIMAGE names the
    // stable file that remains valid at the next login.
    let executable = std::env::var_os("APPIMAGE")
        .map(std::path::PathBuf::from)
        .map(Ok)
        .unwrap_or_else(std::env::current_exe)
        .map_err(|error| error.to_string())?;
    let executable = executable
        .to_str()
        .ok_or("Application path is not valid Unicode")?;
    let escaped = executable
        .replace('%', "%%")
        .replace('\\', "\\\\")
        .replace('"', "\\\"")
        .replace('`', "\\`")
        .replace('$', "\\$");
    let entry = format!(
        "[Desktop Entry]\nType=Application\nName=floter\nExec=\"{escaped}\" --background\nTerminal=false\nHidden=false\nX-GNOME-Autostart-enabled=true\n"
    );
    // R123 · steady state writes nothing: the entry is read back first and a
    // byte-identical file ends the call before the directory is touched. A
    // missing file reads as `Err(NotFound)`, and any other read error falls
    // through too — both take the write below, which is also what creates the
    // `autostart` directory on the first enable.
    if let Ok(existing) = std::fs::read(&path) {
        if existing == entry.as_bytes() {
            return Ok(());
        }
    }
    std::fs::create_dir_all(&autostart_dir).map_err(|error| error.to_string())?;
    std::fs::write(path, entry).map_err(|error| error.to_string())
}

#[cfg(not(any(target_os = "windows", target_os = "macos", target_os = "linux")))]
fn apply_launch_at_startup(_enabled: bool) -> Result<(), String> {
    Err("Launch at startup is unsupported on this platform".to_string())
}

#[cfg(any(target_os = "macos", target_os = "linux"))]
fn remove_file_if_present(path: &std::path::Path) -> Result<(), String> {
    match std::fs::remove_file(path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(error.to_string()),
    }
}

pub fn ensure_launch_at_startup(enabled: bool) -> Result<(), String> {
    apply_launch_at_startup(enabled)
}

#[tauri::command]
pub fn set_launch_at_startup(enabled: bool) -> Result<(), String> {
    apply_launch_at_startup(enabled)
}
