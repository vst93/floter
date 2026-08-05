use std::path::PathBuf;

pub const PIPE_PREFIX: &str = r"\\.\pipe\qscreen-";
const NAMESPACE_ENV: &str = "QSCREEN_NAMESPACE";

/// IPC name: a Named Pipe on Windows, a Unix domain socket path on Unix.
pub fn pipe_name() -> String {
    let stem = endpoint_stem();
    #[cfg(windows)]
    {
        format!("{PIPE_PREFIX}{stem}")
    }
    #[cfg(unix)]
    {
        unix_runtime_dir()
            .join(format!("{stem}.sock"))
            .to_string_lossy()
            .into_owned()
    }
}

/// Daemon log path: uses %TEMP% on Windows, ${TMPDIR:-/tmp} on Unix.
pub fn daemon_log_path() -> PathBuf {
    let stem = endpoint_stem();
    #[cfg(windows)]
    {
        let temp = std::env::var("TEMP")
            .or_else(|_| std::env::var("TMP"))
            .unwrap_or_else(|_| "C:\\Temp".to_string());
        PathBuf::from(temp).join(format!("{stem}-daemon.log"))
    }
    #[cfg(unix)]
    {
        unix_runtime_dir().join(format!("{stem}-daemon.log"))
    }
}

pub fn client_log_path() -> PathBuf {
    let stem = endpoint_stem();
    #[cfg(windows)]
    {
        let temp = std::env::var("TEMP")
            .or_else(|_| std::env::var("TMP"))
            .unwrap_or_else(|_| "C:\\Temp".to_string());
        PathBuf::from(temp).join(format!("{stem}-client.log"))
    }
    #[cfg(unix)]
    {
        unix_runtime_dir().join(format!("{stem}-client.log"))
    }
}

#[cfg(unix)]
pub fn daemon_lock_path() -> PathBuf {
    unix_runtime_dir().join(format!("{}.lock", endpoint_stem()))
}

pub fn current_user() -> String {
    std::env::var("USERNAME")
        .or_else(|_| std::env::var("USER"))
        .unwrap_or_else(|_| "unknown".to_string())
}

pub fn sanitize_pipe_user(user: &str) -> String {
    user.chars()
        .map(|character| {
            if character.is_ascii_alphanumeric() || character == '-' || character == '_' {
                character
            } else {
                '_'
            }
        })
        .collect()
}

fn endpoint_stem() -> String {
    let user = sanitize_pipe_user(&current_user());
    match std::env::var(NAMESPACE_ENV)
        .ok()
        .map(|value| sanitize_pipe_user(&value))
        .filter(|value| !value.is_empty())
    {
        Some(namespace) => format!("qscreen-{namespace}-{user}"),
        None => format!("qscreen-{user}"),
    }
}

#[cfg(unix)]
fn unix_runtime_dir() -> PathBuf {
    std::env::var_os("TMPDIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/tmp"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sanitizes_endpoint_components() {
        assert_eq!(sanitize_pipe_user("DOMAIN\\user"), "DOMAIN_user");
    }

    #[test]
    fn endpoint_uses_safe_user_suffix() {
        assert!(pipe_name().contains(&sanitize_pipe_user(&current_user())));
    }
}
