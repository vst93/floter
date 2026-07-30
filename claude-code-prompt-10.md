# floter: Launcher action bar (Raycast-style) + URL/path smart actions

You are working on **floter**, a Tauri v2 floating terminal/launcher app. Do NOT modify any code under `~/.hermes/`.

## Overview

Replace the current "shell command always in slot 2" launcher layout with a Raycast-style **action bar** at the bottom of the results. App results fill the top, the action bar is a separate fixed row below them.

## Current behavior (to replace)

In `src/App.tsx`, `launcherItems` useMemo currently builds:
```
[bestApp, commandItem, ...remainingApps].slice(0, MAX_RESULTS)
```

The shell command is always inserted at index 1, taking up a numbered result slot. This is awkward — the command isn't really a "search result", it's an action.

## New behavior

### Layout

```
┌─────────────────────────────┐
│  [icon] App Name            │  ← app results (up to 5, Ctrl+1-5)
│        subtitle             │
├─────────────────────────────┤  ← divider
│  $     git status           │  ← action bar (not numbered, not in Ctrl+N)
│        Run in terminal      │
└─────────────────────────────┘
```

The action bar is a separate element below `.launcher-results`, not part of the results list. It has its own selection state.

### Data model

Split `launcherItems` into two separate values:

```typescript
// App + system results (the numbered list)
const launcherResults = useMemo<LauncherItem[]>(() => {
    const command = query.trim();
    if (!command) return [];

    const needle = normalizeSearch(command);
    if (!needle) return [];

    const matches: { item: LauncherItem; score: number }[] = [];

    // ... existing app scoring ...
    // ... existing system command scoring ...

    return matches
        .sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title))
        .slice(0, MAX_RESULTS - 1)  // Leave room for the action bar
        .map((match) => match.item);
}, [query, searchableApps, t]);

// The bottom action bar
const actionBar = useMemo<{ type: "shell" | "url" | "path"; label: string; value: string } | null>(() => {
    const trimmed = query.trim();
    if (!trimmed) return null;

    // URL detection: starts with http://, https://, or ftp://
    if (/^https?:\/\//i.test(trimmed) || /^ftp:\/\//i.test(trimmed)) {
        return { type: "url", label: t("launcher.openInBrowser"), value: trimmed };
    }

    // Path detection: starts with / or ~ or ./ or ../, or is a Windows drive path like C:\
    if (/^[/~.]/.test(trimmed) || /^[A-Za-z]:[\\/]/.test(trimmed)) {
        return { type: "path", label: t("launcher.openInFiles"), value: trimmed };
    }

    // Default: shell command
    return { type: "shell", label: t("launcher.runInShell"), value: trimmed };
}, [query, t]);
```

### Selection model

Two indices: `selectedResultIndex` for the app list, `selectedActionBar` (boolean) for the action bar.

Default selection logic (on query change):
- If query contains spaces, pipes, redirects, `&&`, `||`, or starts with common command names (`cd`, `git`, `npm`, `ls`, `cat`, `echo`, `curl`, `wget`, `ssh`, `cp`, `mv`, `rm`, `mkdir`, `touch`, `chmod`, `grep`, `find`, `sed`, `awk`, `make`, `docker`, `kubectl`, `python`, `node`, `go`, `cargo`, `brew`, `apt`, `yum`, `pip`, `yarn`, `pnpm`): default to action bar
- If action bar type is "url" or "path": default to action bar
- Otherwise: default to first app result

```typescript
useEffect(() => {
    setSelectedResultIndex(0);
    const trimmed = query.trim();
    if (!trimmed) {
        setSelectedActionBar(false);
        return;
    }
    // Smart default: commands with arguments or known CLI tools go to action bar
    const looksLikeCommand = /\s/.test(trimmed) || /[|>&]/.test(trimmed) ||
        COMMAND_PREFIXES.some(prefix => trimmed.toLowerCase().startsWith(prefix));
    const isSpecialAction = actionBar?.type === "url" || actionBar?.type === "path";
    setSelectedActionBar(looksLikeCommand || isSpecialAction || launcherResults.length === 0);
}, [query, launcherResults, actionBar]);
```

### Keyboard navigation

```
ArrowUp/ArrowDown: navigate within app results, then wrap to/from action bar
  - ArrowDown from last app result -> action bar
  - ArrowDown from action bar -> first app result (wrap)
  - ArrowUp from action bar -> last app result
  - ArrowUp from first app result -> action bar (wrap)

Tab: jump to action bar (from app list) or back to first result (from action bar)

Enter: execute selected item (app result or action bar)

Ctrl+1-5: select app result by number (does NOT include action bar)

Escape: hide window
```

Update `onInputKeyDown`:
```typescript
if (event.key === "Enter") {
    event.preventDefault();
    if (selectedActionBar && actionBar) {
        executeActionBar(actionBar);
    } else if (launcherResults[selectedResultIndex]) {
        runLauncherItem(launcherResults[selectedResultIndex]);
    }
    return;
}

if (event.key === "ArrowDown") {
    event.preventDefault();
    if (selectedActionBar) {
        // Wrap to first result
        setSelectedActionBar(false);
        setSelectedResultIndex(0);
    } else if (selectedResultIndex < launcherResults.length - 1) {
        setSelectedResultIndex(i => i + 1);
    } else {
        // Last result -> action bar
        setSelectedActionBar(true);
    }
    return;
}

if (event.key === "ArrowUp") {
    event.preventDefault();
    if (selectedActionBar) {
        // Action bar -> last result
        setSelectedActionBar(false);
        setSelectedResultIndex(launcherResults.length - 1);
    } else if (selectedResultIndex > 0) {
        setSelectedResultIndex(i => i - 1);
    } else {
        // First result -> action bar (wrap)
        setSelectedActionBar(true);
    }
    return;
}

if (event.key === "Tab") {
    event.preventDefault();
    setSelectedActionBar(prev => !prev);
    if (selectedActionBar) {
        setSelectedResultIndex(0);
    }
    return;
}
```

### Execute action bar

```typescript
const executeActionBar = (action: { type: "shell" | "url" | "path"; label: string; value: string }) => {
    if (action.type === "shell") {
        void runCommand();
    } else if (action.type === "url") {
        invoke("open_url", { url: action.value }).catch(() => undefined);
        setQuery("");
        invoke("hide_window");
    } else if (action.type === "path") {
        invoke("open_path", { path: action.value }).catch(() => undefined);
        setQuery("");
        invoke("hide_window");
    }
};
```

### Rendering

The action bar renders BELOW `.launcher-results` as a separate element:

```tsx
{(launcherResults.length > 0 || actionBar) && (
    <div className="launcher-bottom">
        {launcherResults.length > 0 && (
            <div className="launcher-results" role="listbox" aria-label="Launcher results">
                {launcherResults.map((item, index) => (
                    // ... existing result rendering, use selectedResultIndex ...
                ))}
            </div>
        )}
        {actionBar && (
            <button
                type="button"
                className={`launcher-action-bar${selectedActionBar ? " launcher-action-bar--selected" : ""}`}
                onMouseEnter={() => setSelectedActionBar(true)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => executeActionBar(actionBar)}
            >
                <span className="launcher-action-bar__icon">
                    {actionBar.type === "url" ? (
                        // Lucide external-link icon
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none"
                             stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M15 3h6v6" />
                            <path d="M10 14 21 3" />
                            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                        </svg>
                    ) : actionBar.type === "path" ? (
                        // Lucide folder icon
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none"
                             stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
                        </svg>
                    ) : (
                        <span>$</span>
                    )}
                </span>
                <span className="launcher-action-bar__main">
                    <span className="launcher-action-bar__title">{actionBar.value}</span>
                    <span className="launcher-action-bar__subtitle">{actionBar.label}</span>
                </span>
            </button>
        )}
    </div>
)}
```

### CSS for action bar

```css
.launcher-bottom {
    display: grid;
    gap: 0;
}

.launcher-action-bar {
    width: 100%;
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    align-items: center;
    gap: 10px;
    padding: 0 9px 0 11px;
    border-radius: 7px;
    background: transparent;
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
    border-top: 1px solid var(--hairline);
    margin-top: 2px;
    padding-top: 2px;
}

.launcher-action-bar--selected {
    background: var(--accent-tint);
}

/* Different accent for shell command vs URL/path — green tint to distinguish
   from the blue app selection */
.launcher-action-bar--selected.launcher-action-bar--shell {
    background: rgba(72, 187, 120, 0.12);
}

.launcher-action-bar__icon {
    width: 28px;
    height: 28px;
    display: grid;
    place-items: center;
    border-radius: 6px;
    background: var(--icon-surface);
    color: var(--text-secondary);
    font-size: 14px;
    font-family: "SF Mono", Menlo, Monaco, Consolas, monospace;
}

.launcher-action-bar__main {
    min-width: 0;
    display: grid;
    gap: 2px;
}

.launcher-action-bar__title {
    font-size: 13px;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.launcher-action-bar__subtitle {
    font-size: 11px;
    color: var(--text-tertiary);
}
```

### Backend: open_url and open_path commands

Add to `src-tauri/src/commands/mod.rs`: `pub mod system;` already exists. Add the new commands there or in a new `actions.rs`:

```rust
#[tauri::command]
pub fn open_url(url: String) -> Result<(), String> {
    // open 是跨平台的: macOS 用 open, Linux 用 xdg-open, Windows 用 start
    // 但直接用 opener crate 或 std/sysinfo 更简单
    // 最简单: 用 tauri 的 shell open
    // Actually, Tauri v2 has opener plugin, but let's just use std::process::Command
    
    // Validate URL is actually a URL (basic check)
    if !url.starts_with("http://") && !url.starts_with("https://") && !url.starts_with("ftp://") {
        return Err("Invalid URL".to_string());
    }
    
    #[cfg(target_os = "macos")]
    let opener = "open";
    #[cfg(target_os = "linux")]
    let opener = "xdg-open";
    #[cfg(target_os = "windows")]
    let opener = "cmd";
    
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new(opener)
            .args(["/c", "start", "", &url])
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    #[cfg(not(target_os = "windows"))]
    {
        std::process::Command::new(opener)
            .arg(&url)
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub fn open_path(path: String) -> Result<(), String> {
    // Expand ~ on Unix
    #[cfg(unix)]
    let path = shellexpand::tilde(&path).to_string();
    
    let path = std::path::PathBuf::from(&path);
    if !path.exists() {
        return Err(format!("Path does not exist: {}", path.display()));
    }
    
    #[cfg(target_os = "macos")]
    {
        // open opens Finder for directories, default app for files
        std::process::Command::new("open")
            .arg(&path)
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    #[cfg(target_os = "linux")]
    {
        std::process::Command::new("xdg-open")
            .arg(&path)
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new("explorer")
            .arg(&path)
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    Ok(())
}
```

For `~` expansion, either add `shellexpand` crate or do it manually:
```rust
fn expand_tilde(path: &str) -> String {
    if let Some(stripped) = path.strip_prefix("~/") {
        if let Some(home) = dirs::home_dir() {
            return home.join(stripped).to_string_lossy().to_string();
        }
    }
    if path == "~" {
        if let Some(home) = dirs::home_dir() {
            return home.to_string_lossy().to_string();
        }
    }
    path.to_string()
}
```

Use the manual expansion to avoid adding a dependency.

### i18n

Add to both `en` and `zh` in `src/i18n.ts`:

```
English:
"launcher.openInBrowser": "Open in browser",
"launcher.openInFiles": "Open in files",

Chinese:
"launcher.openInBrowser": "在浏览器中打开",
"launcher.openInFiles": "在文件管理器中打开",
```

### Action bar type modifier class

Add the `--shell` modifier when `actionBar.type === "shell"`:
```tsx
className={`launcher-action-bar${selectedActionBar ? " launcher-action-bar--selected" : ""} launcher-action-bar--${actionBar.type}`}
```

So the green tint only applies to shell commands, while URL/path use the blue accent tint.

### Remove old command item

Remove the `commandItem` from `launcherItems`/`launcherResults`. The shell command is now ONLY in the action bar. The `LauncherItem` type's `"command"` variant is no longer used in the results list, but keep the type for now (it doesn't hurt).

Actually, keep `"command"` in the type — it's still used by `runLauncherItem`. But it won't appear in `launcherResults`.

### Ctrl+1-5 shortcut

`matchesResultShortcut` now maps to `launcherResults` (not `launcherItems`). Update the handler:

```typescript
const resultNumber = matchesResultShortcut(native, shortcuts.select_result);
if (resultNumber !== null) {
    if (launcherResults[resultNumber - 1]) {
        event.preventDefault();
        runLauncherItem(launcherResults[resultNumber - 1]);
    }
    return;
}
```

### Important: remove old launcherItems

The old `launcherItems` useMemo and all references to it should be replaced with `launcherResults` + `actionBar`. Search for ALL references to `launcherItems` and update them:
- `launcherItems.length` -> `launcherResults.length` or `launcherResults.length || actionBar`
- `launcherItems[index]` -> `launcherResults[index]`
- The global keydown handler that checks `launcherItems.length`
- The window height calculation that uses `launcherItems.length`

### Window height

The window height calculation needs to account for the action bar:
```typescript
const resultCount = launcherResults.length + (actionBar ? 1 : 0);
const resultHeight = resultCount > 0
    ? resultCount * RESULT_ROW_HEIGHT + Math.max(0, resultCount - 1) * RESULT_ROW_GAP
    : 0;
```

But the action bar has a border-top, so add a few pixels for the divider.

### COMMAND_PREFIXES constant

```typescript
const COMMAND_PREFIXES = [
    "cd", "git", "npm", "ls", "cat", "echo", "curl", "wget", "ssh",
    "cp", "mv", "rm", "mkdir", "touch", "chmod", "grep", "find",
    "sed", "awk", "make", "docker", "kubectl", "python", "python3",
    "node", "go", "cargo", "brew", "apt", "yum", "pip", "yarn",
    "pnpm", "tar", "gzip", "unzip", "head", "tail", "wc", "sort",
    "uniq", "diff", "kill", "ps", "top", "df", "du", "free", "uname",
    "whoami", "hostname", "ping", "ifconfig", "ip", "netstat", "lsof",
    "systemctl", "journalctl", "man", "which", "whereis", "export",
    "source", "alias", "history", "sudo",
];
```

## Verification

```bash
cd src-tauri && cargo check --all-targets 2>&1
cd .. && npx tsc --noEmit 2>&1
```

Both must pass with zero errors.

## Code Style
- Keep existing code style: detailed doc comments explaining WHY
- All comments in English
- Lucide SVG icons (24×24 viewBox, stroke-width=2)
- CSS variables for all colors
- i18n keys in both en and zh
- No new dependencies (manual ~ expansion, std::process::Command for open)
