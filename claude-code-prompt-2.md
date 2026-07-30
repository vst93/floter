# floter Bug Fixes Round 2

You are working on **floter**, a Tauri v2 floating terminal/launcher app. The project is at the current working directory. Do NOT modify any code under `~/.hermes/`.

## Context

In the previous round, a `prefer_portable_terminfo()` function was added to `src-tauri/src/terminal/session.rs` that overrides `TERM=alacritty` to `TERM=xterm-256color` after `tty::setup_env()`. This was supposed to fix TUI programs hanging on startup. But the user reports it still doesn't work on macOS.

## Two Bugs to Fix

### Bug 1: TUI programs still slow to start on macOS

**Current code** in `src-tauri/src/terminal/session.rs`:
```rust
pub fn new(id: String, app: AppHandle, shell: Option<String>, cols: u16, rows: u16) -> Result<Self, String> {
    tty::setup_env();
    prefer_portable_terminfo();
    // ...
    let mut tty_options = TtyOptions::default();
    tty_options.working_directory = dirs::home_dir();
    // ...
    let pty = tty::new(&tty_options, window_size, 0).map_err(|e| e.to_string())?;
}
```

**Root cause analysis**: `prefer_portable_terminfo()` sets `TERM` in the *process environment* via `std::env::set_var("TERM", "xterm-256color")`. The child shell inherits this through normal environment propagation. BUT on macOS, `alacritty_terminal`'s `default_shell_command()` uses `login -flp` which runs the shell through macOS's `login` command. While `-p` preserves the environment, the `login` command on macOS may reset `TERM` or the login shell's profile scripts (`.zshrc`, `.zprofile`) may override `TERM`.

More importantly: `TtyOptions` has an `env: HashMap<String, String>` field that gets set explicitly on the child `Command` via `builder.env(key, value)`. This is a MORE RELIABLE way to set environment variables on the child process than relying on process-level `set_var` + inheritance. The `tty::new()` function iterates `config.env` and calls `builder.env(key, value)` for each entry, which directly sets it on the child process.

**Fix**: Set `TERM` through `tty_options.env` explicitly, so it goes through `builder.env("TERM", ...)` which directly sets it on the child Command. Keep `prefer_portable_terminfo()` for the process env (in case anything else reads it), but ALSO add it to `tty_options.env`.

Replace the `prefer_portable_terminfo()` approach. Instead:

```rust
pub fn new(id: String, app: AppHandle, shell: Option<String>, cols: u16, rows: u16) -> Result<Self, String> {
    tty::setup_env();

    // alacritty_terminal sets TERM=alacritty (unconditionally via setup_env),
    // but the alacritty terminfo entry is missing on most macOS/Linux installs.
    // TUI programs that can't resolve TERM hang for seconds searching terminfo
    // fallback paths before degrading to dumb-terminal behavior.
    //
    // Override to xterm-256color which ships with ncurses everywhere. Set it
    // both in the process environment AND in the PTY's explicit env map: on
    // macOS the shell is launched through `login -flp`, and the login command
    // or the shell's profile scripts may reset TERM from the inherited env.
    // The explicit env map goes through `builder.env()` which is set directly
    // on the child Command and takes precedence.
    let term = std::env::var("TERM").unwrap_or_default();
    if term.is_empty() || term == "alacritty" {
        std::env::set_var("TERM", "xterm-256color");
    }

    // ...
    let mut tty_options = TtyOptions::default();
    tty_options.working_directory = dirs::home_dir();
    // Explicitly set TERM on the child process, overriding anything login or
    // profile scripts might do.
    tty_options.env.insert("TERM".to_string(), "xterm-256color".to_string());
    // ...
```

Wait, actually check: does `builder.env(key, value)` on a `Command` that then execs `login -flp` actually override what `login` sets? The `login` command receives the environment from the parent process. With `-p`, it preserves the environment. But `login` itself may set `TERM`. Let me think...

Actually, `builder.env("TERM", "xterm-256color")` sets the env var on the `login` process. `login -p` preserves environment for the child shell. But `login` itself might override `TERM`. However, looking at the macOS `login` source, with `-p` it preserves the environment including `TERM`. The issue is more likely that the shell's `.zshrc` sets `TERM`.

Actually, the real fix should be simpler and more robust: **Don't use `login` at all.** The `login -flp` command is used by alacritty to create a "login shell" on macOS. But floter doesn't need a login shell — it needs an interactive shell. The `login` command adds the "Last login" banner and may interfere with environment variables.

But we can't change alacritty_terminal's `default_shell_command` — it's in the library. What we CAN do is provide our own shell in `tty_options.shell`:

```rust
let shell_program = std::env::var("SHELL").unwrap_or_else(|_| "/bin/zsh".to_string());
tty_options.shell = Some(Shell::new(shell_program, Vec::new()));
```

When `config.shell` is `Some`, `tty::new()` uses `Command::new(&shell.program)` directly instead of `default_shell_command()` (which uses `login`). This avoids the `login` command entirely, which means:
1. No "Last login" banner
2. TERM is inherited directly without `login` interference
3. The shell runs as a non-login interactive shell

BUT — this changes the shell from a login shell to a non-login shell. A login shell sources `.zprofile` then `.zshrc`. A non-login interactive shell sources `.zshrc`. Most user customizations are in `.zshrc`, so this should be fine. The PATH set in `.zprofile` would be missing, but on modern macOS, PATH is set by `/etc/paths` and `/etc/paths.d/` via `path_helper`, which is called from `/etc/zprofile` — which only runs for login shells.

Hmm, this could break PATH for some users. A safer approach: use `login` but with `-q` flag to suppress the banner, and set TERM explicitly.

Actually, the safest and most correct fix is:

1. Set TERM through `tty_options.env` (explicit child env, most reliable)
2. Also set it via `std::env::set_var` (for anything that reads process env before spawn)
3. Remove the separate `prefer_portable_terminfo()` function — fold its logic inline

If after setting TERM through `tty_options.env` it still doesn't work (because login or .zshrc overrides it), we have a deeper problem. But `builder.env()` sets it directly on the child process, and with `login -p`, the environment is preserved. The key insight is that `tty_options.env` goes through `builder.env()` which is MORE reliable than `std::env::set_var()`.

Actually wait — I need to check the order in `tty::new()`. The env vars from `config.env` are set AFTER `builder.env("USER", ...)`, `builder.env("HOME", ...)`, etc. But they're all set via `builder.env()`. The last one wins if there are duplicates. And `TERM` is NOT set by `builder.env()` in the current code — it relies on inheritance. So adding `TERM` to `tty_options.env` should work.

Let me also consider: on macOS, when `login -flp` runs, it receives the environment from the `Command` (which includes our explicit `TERM=xterm-256color`). The `-p` flag means "preserve environment." So `login` should pass `TERM=xterm-256color` through to the shell. The shell's `.zshrc` MIGHT override it, but that's a user configuration issue, not our bug.

**The fix**: In `TerminalSession::new()`, after `tty::setup_env()`, set TERM in the process env (for good measure) AND add it to `tty_options.env` (the reliable path). Remove the separate `prefer_portable_terminfo()` function and inline the logic.

### Bug 2: "Open in external terminal" shows login banner and .command path instead of clean shell

**Current code** in `src-tauri/src/terminal/session.rs` (macOS `open_terminal_at`):
```rust
fn open_terminal_at(dir: &Path) -> Result<(), String> {
    // writes a .command shim to cache dir, then: open -a Terminal shim.command
    let shim = cache_dir.join("open-here.command");
    let mut content = String::from("#!/bin/sh\n");
    content.push_str(&format!("cd {}\n", sh_quote(&dir.to_string_lossy())));
    content.push_str(&format!("exec {}\n", sh_quote(&shell)));
    // ...
    Command::new("open").args(["-a", "Terminal"]).arg(&shim).spawn()
}
```

**Problem**: When Terminal.app opens a `.command` file, it:
1. Prints "Last login: ..." (login banner)
2. Prints the command path: `/Users/v/.../open-here.command ; exit;`
3. Shows a prompt at the home directory (because login shell cd's to $HOME before running the script)

The user sees:
```
Last login: Thu Jul 30 10:54:52 on ttys013
/Users/v/Library/Caches/floter/open-here.command ; exit;
➜ ~ /Users/v/Library/Caches/floter/open-here.command ; exit;
➜ ~
```

**Fix**: Use AppleScript (`osascript`) to control Terminal.app directly. This gives a clean window with no banner and no .command path:

```rust
fn open_terminal_at(dir: &Path) -> Result<(), String> {
    let dir_str = sh_quote(&dir.to_string_lossy());
    let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/zsh".to_string());
    let shell_str = sh_quote(&shell);

    // osascript opens a Terminal window and runs the command directly — no
    // .command file, no login banner, no "script path ; exit;" line.
    let script = format!(
        "tell application \"Terminal\"\n\
         activate\n\
         do script \"cd {dir_str} && exec {shell_str}\"\n\
         end tell"
    );

    Command::new("osascript")
        .args(["-e", &script])
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}
```

The `do script` command opens a new Terminal window and runs the given command string. `cd DIR && exec $SHELL` lands in the right directory and replaces the temporary shell with the user's shell. No `.command` file, no login banner, no path display.

Note: `do script` runs in a login shell context by default in Terminal.app, so the user's PATH and profile are loaded. The "Last login" banner is NOT shown because `do script` doesn't go through `login` — it opens a new window and types the command.

Remove the `.command` shim approach entirely (the `cache_dir`, `shim`, `std::fs::write`, `std::fs::set_permissions` code). The `sh_quote` function is still needed for the osascript approach.

## Verification

After all changes:
```bash
cd src-tauri && cargo check --all-targets 2>&1
cd .. && npx tsc --noEmit 2>&1
```

Both must pass with zero errors.

## Code Style Requirements

- Keep existing code style: detailed doc comments explaining WHY
- `#[cfg(target_os = "macos")]` guards on the macOS-specific code
- Remove the old `prefer_portable_terminfo()` function and inline its logic
- Remove the `.command` shim code and replace with osascript
- Keep `sh_quote()` — it's still used
- No new dependencies
- All comments in English
