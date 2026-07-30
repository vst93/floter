# floter: App list refresh on summon + system power commands

You are working on **floter**, a Tauri v2 floating terminal/launcher app. Do NOT modify any code under `~/.hermes/`.

## Feature 1: App list refresh on summon + loading placeholder

### Current behavior

- `list_applications` is called once on component mount with `forceRefresh: false`
- Backend has a signature-based cache: computes `roots_signature` (mtime + entry count of app directories), returns cache if unchanged
- No refresh trigger after initial load — newly installed apps don't appear until restart
- During initial scan, `applications` is empty, user sees only shell command option with no indication apps are loading

### Desired behavior

1. **First launch (no cache)**: Show "Scanning applications…" as the input placeholder while scanning. Switch to normal placeholder when done.
2. **Every summon (window reveal)**: Check if app directories changed. If unchanged, do nothing (user is unaware). If changed, silently refresh in background — user continues using old data, seamlessly swaps when ready.
3. **Performance**: The signature check must be fast and non-blocking. Only trigger a full scan when the signature actually changes.

### Implementation

#### Backend (`src-tauri/src/commands/apps/mod.rs`)

Add a new lightweight command `check_applications`:

```rust
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ApplicationsStatus {
    pub up_to_date: bool,
    pub count: usize,
}

#[tauri::command]
pub fn check_applications(state: State<'_, ApplicationState>) -> Result<ApplicationsStatus, String> {
    let roots = platform::roots();
    let signature = roots_signature(&roots);
    let cache = state.cache.lock().map_err(|e| e.to_string())?;
    let up_to_date = !cache.apps.is_empty() && cache.signature == signature;
    Ok(ApplicationsStatus { up_to_date, count: cache.apps.len() })
}
```

Make `list_applications` async so the scan runs on a thread pool, not the main thread:

```rust
#[tauri::command]
pub async fn list_applications(
    state: State<'_, ApplicationState>,
    force_refresh: Option<bool>,
) -> Result<Vec<LocalApplication>, String> {
    let force_refresh = force_refresh.unwrap_or(false);

    // Fast path: check signature under the lock, return cache if valid.
    {
        let roots = platform::roots();
        let signature = roots_signature(&roots);
        let cache = state.cache.lock().map_err(|e| e.to_string())?;
        if !force_refresh && !cache.apps.is_empty() && cache.signature == signature {
            return Ok(cache.apps.clone());
        }
    }

    // Slow path: scan in a blocking thread.
    let apps = tauri::async_runtime::spawn_blocking(move || {
        let roots = platform::roots();
        let mut apps = platform::scan(&roots);
        apps.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
        apps
    })
    .await
    .map_err(|e| e.to_string())?;

    // Update cache.
    let roots = platform::roots();
    let signature = roots_signature(&roots);
    let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
    cache.signature = signature;
    cache.apps = apps.clone();
    Ok(apps)
}
```

**Important**: `platform::roots()` and `roots_signature()` are called multiple times — that's fine, they're cheap (just path construction and stat calls). The `spawn_blocking` closure captures nothing from `state`, so there's no lock held during the scan.

Register `check_applications` in the invoke handler in `lib.rs`.

#### Frontend (`src/App.tsx`)

1. Add state: `const [appsLoading, setAppsLoading] = useState(true);`

2. Initial load (existing useEffect, modify):
```typescript
useEffect(() => {
    setAppsLoading(true);
    invoke<LocalApplication[]>("list_applications", { forceRefresh: false })
      .then((apps) => {
        setApplications(apps);
        setAppsLoading(false);
      })
      .catch(() => setAppsLoading(false));
  }, []);
```

3. On every reveal (modify the existing `floter://revealed` listener):
```typescript
const unlistenRevealPromise = listen<string>("floter://revealed", (event) => {
      // ... existing mode restore logic ...

      // Check if app directories changed since last scan.
      invoke<{ upToDate: boolean; count: number }>("check_applications")
        .then((status) => {
          if (!status.upToDate) {
            invoke<LocalApplication[]>("list_applications", { forceRefresh: true })
              .then(setApplications)
              .catch(() => undefined);
          }
        })
        .catch(() => undefined);
    });
```

4. Dynamic placeholder:
```typescript
const placeholder = appsLoading && applications.length === 0
  ? t("input.scanning")
  : t("input.placeholder");
```

Use `placeholder` instead of `t("input.placeholder")` in the input element.

#### i18n (`src/i18n.ts`)

Add new keys:
- English: `"input.scanning": "Scanning applications…"`
- Chinese: `"input.scanning": "正在扫描应用…"`

#### Platform compatibility notes

- `list_applications` being `async` works on all platforms — Tauri v2 runs async commands on a thread pool.
- `spawn_blocking` is available via `tauri::async_runtime` — no new dependency.
- `check_applications` stays synchronous (fast, < 1ms) — it only stats directories.
- The `roots_signature` function already handles all platforms (macOS `.app` dirs, Linux `.desktop` dirs, Windows Start Menu).
- On Windows, `std::fs::metadata` works the same way. No platform-specific changes needed.

---

## Feature 2: System power commands (restart, shutdown)

Add two built-in launcher items: "Restart" and "Shut Down". These appear in search results alongside apps and shell commands.

### Implementation

#### New LauncherItem type

In `src/App.tsx`, extend the `LauncherItem` type:

```typescript
type LauncherItem =
  | { type: "app"; id: string; title: string; subtitle: string; app: LocalApplication }
  | { type: "command"; id: string; title: string; subtitle: string }
  | { type: "system"; id: string; title: string; subtitle: string; action: "restart" | "shutdown" };
```

#### i18n keys (`src/i18n.ts`)

English:
```
"system.restart": "Restart",
"system.restartSubtitle": "Restart the computer",
"system.shutdown": "Shut Down",
"system.shutdownSubtitle": "Turn off the computer",
```

Chinese:
```
"system.restart": "重启",
"system.restartSubtitle": "重启电脑",
"system.shutdown": "关机",
"system.shutdownSubtitle": "关闭电脑",
```

#### System commands in launcher

In the `launcherItems` useMemo, add system commands that match the query:

```typescript
const SYSTEM_ITEMS: Array<{ id: string; action: "restart" | "shutdown"; titleKey: MessageKey; subtitleKey: MessageKey }> = [
    { id: "system-restart", action: "restart", titleKey: "system.restart", subtitleKey: "system.restartSubtitle" },
    { id: "system-shutdown", action: "shutdown", titleKey: "system.shutdown", subtitleKey: "system.shutdownSubtitle" },
];
```

These should be matched against the query using the same fuzzy scoring as apps. When the query is empty, they don't appear (same as apps). When the query matches (e.g., "restart", "reboot", "shutdown", "关机", "重启"), they appear in the results.

The system items should be scored and mixed into the results alongside apps. Insert them into the results list, sort by score, then slice to MAX_RESULTS.

Actually, a simpler approach: compute system item scores alongside app scores, merge them, sort, then build the final list.

```typescript
const systemItems = SYSTEM_ITEMS.map((item) => ({
    ...item,
    score: Math.max(
      scoreNormalized(needle, normalizeSearch(t(item.titleKey))),
      // Also match common synonyms
      item.action === "restart" ? scoreNormalized(needle, normalizeSearch("reboot")) : 0,
    ),
})).filter((item) => item.score > 0);
```

Wait, but `t()` is a function and we're inside a `useMemo` that already depends on `t`. Let me think about this more carefully.

The system items need i18n. The `t` function is available in the component. So in the `launcherItems` useMemo:

```typescript
const launcherItems = useMemo<LauncherItem[]>(() => {
    const command = query.trim();
    if (!command) return [];

    const commandItem: LauncherItem = {
      type: "command",
      id: "command",
      title: command,
      subtitle: t("launcher.runInShell"),
    };

    const needle = normalizeSearch(command);

    // System power commands.
    const systemItems: LauncherItem[] = [];
    if (needle) {
      const systemCommands = [
        { action: "restart" as const, names: [t("system.restart"), "reboot", "restart"] },
        { action: "shutdown" as const, names: [t("system.shutdown"), "power off", "shutdown"] },
      ];
      for (const cmd of systemCommands) {
        const scores = cmd.names.map((name) => scoreNormalized(needle, normalizeSearch(name)));
        const bestScore = Math.max(...scores);
        if (bestScore > 0) {
          systemItems.push({
            type: "system",
            id: `system-${cmd.action}`,
            title: cmd.action === "restart" ? t("system.restart") : t("system.shutdown"),
            subtitle: cmd.action === "restart" ? t("system.restartSubtitle") : t("system.shutdownSubtitle"),
            action: cmd.action,
          });
        }
      }
    }

    // App items (existing code).
    const appItems = ...; // existing

    // Merge: apps + system + command, sort by score, take top MAX_RESULTS.
    // Actually, keep it simple: system items get a high score so they appear
    // near the top when matched. Build the combined list.
    ...
}, [query, searchableApps, t]);
```

Hmm, this is getting complex because the current code separates app items and the command item. Let me think of a cleaner approach.

**Cleaner approach**: Score everything uniformly, then build the result list.

```typescript
const launcherItems = useMemo<LauncherItem[]>(() => {
    const command = query.trim();
    if (!command) return [];

    const needle = normalizeSearch(command);

    // Collect all scored candidates.
    type Scored = { item: LauncherItem; score: number };
    const candidates: Scored[] = [];

    // Shell command — always present, moderate score so it doesn't outrank
    // an exact app name match but always appears.
    candidates.push({
      item: { type: "command", id: "command", title: command, subtitle: t("launcher.runInShell") },
      score: 500,
    });

    if (needle) {
      // Apps.
      for (const entry of searchableApps) {
        const score = scoreApp(needle, entry.names);
        if (score > 0) {
          candidates.push({
            item: {
              type: "app",
              id: entry.app.path,
              title: entry.app.localizedName || entry.app.name,
              subtitle: (entry.app.localizedName && entry.app.name) || entry.app.comment || t(appSubtitleKey(entry.app.path)),
              app: entry.app,
            },
            score,
          });
        }
      }

      // System power commands.
      const systemCommands = [
        { action: "restart" as const, titleKey: "system.restart" as const, subtitleKey: "system.restartSubtitle" as const, aliases: ["reboot"] },
        { action: "shutdown" as const, titleKey: "system.shutdown" as const, subtitleKey: "system.shutdownSubtitle" as const, aliases: ["power off"] },
      ];
      for (const cmd of systemCommands) {
        const title = t(cmd.titleKey);
        const names = [title, ...cmd.aliases].map(normalizeSearch);
        const score = Math.max(...names.map((n) => scoreNormalized(needle, n)));
        if (score > 0) {
          candidates.push({
            item: {
              type: "system",
              id: `system-${cmd.action}`,
              title,
              subtitle: t(cmd.subtitleKey),
              action: cmd.action,
            },
            score,
          });
        }
      }
    }

    // Sort by score descending, then alphabetically for ties.
    candidates.sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title));

    // Ensure the shell command is always in the results.
    const result = candidates.slice(0, MAX_RESULTS);
    if (!result.some((c) => c.item.type === "command")) {
      // Command was pushed out — replace last slot.
      result[result.length - 1] = { item: candidates.find((c) => c.item.type === "command")!.item, score: 0 };
    }

    return result.map((c) => c.item);
  }, [query, searchableApps, t]);
```

Wait, the existing code has a specific ordering: app first, then command, then more apps. Let me preserve that pattern but generalize it.

Actually, looking at the existing code more carefully:
```typescript
if (!appItems.length) return [commandItem];
return [appItems[0], commandItem, ...appItems.slice(1)].slice(0, MAX_RESULTS);
```

The pattern is: best app, then shell command, then remaining apps. The shell command is always in slot 2 (or slot 1 if no apps). This is a good UX — the user can always press Enter to run the command, or select the app above it.

For system commands, they should appear based on score, just like apps. The shell command should still always be present.

Let me keep it simpler — don't restructure the whole thing. Just add system items alongside app items:

```typescript
const launcherItems = useMemo<LauncherItem[]>(() => {
    const command = query.trim();
    if (!command) return [];

    const commandItem: LauncherItem = {
      type: "command",
      id: "command",
      title: command,
      subtitle: t("launcher.runInShell"),
    };

    const needle = normalizeSearch(command);

    if (!needle) return [commandItem];

    // Score both apps and system commands uniformly.
    type ScoredItem = { item: LauncherItem; score: number };

    const appItems: ScoredItem[] = searchableApps
      .map((entry) => {
        const score = scoreApp(needle, entry.names);
        if (score === 0) return null;
        const app = entry.app;
        return {
          item: {
            type: "app" as const,
            id: app.path,
            title: app.localizedName || app.name,
            subtitle: (app.localizedName && app.name) || app.comment || t(appSubtitleKey(app.path)),
            app,
          } as LauncherItem,
          score,
        };
      })
      .filter((entry): entry is ScoredItem => entry !== null);

    // System power commands — matched against localized name + English aliases.
    const systemDefs = [
      { action: "restart" as const, titleKey: "system.restart" as MessageKey, subtitleKey: "system.restartSubtitle" as MessageKey, aliases: ["reboot"] },
      { action: "shutdown" as const, titleKey: "system.shutdown" as MessageKey, subtitleKey: "system.shutdownSubtitle" as MessageKey, aliases: ["power off"] },
    ];
    const systemItems: ScoredItem[] = systemDefs
      .map((def) => {
        const title = t(def.titleKey);
        const names = [title, ...def.aliases].map(normalizeSearch);
        const score = Math.max(...names.map((n) => scoreNormalized(needle, n)));
        if (score === 0) return null;
        return {
          item: {
            type: "system" as const,
            id: `system-${def.action}`,
            title,
            subtitle: t(def.subtitleKey),
            action: def.action,
          } as LauncherItem,
          score,
        };
      })
      .filter((entry): entry is ScoredItem => entry !== null);

    // Merge and sort by score.
    const allItems = [...appItems, ...systemItems]
      .sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title))
      .slice(0, MAX_RESULTS - 1)
      .map((entry) => entry.item);

    if (!allItems.length) return [commandItem];
    return [allItems[0], commandItem, ...allItems.slice(1)].slice(0, MAX_RESULTS);
  }, [query, searchableApps, t]);
```

This preserves the existing pattern: best match first, shell command second, then more matches.

#### Running system commands

In `runLauncherItem`, handle the new type:

```typescript
const runLauncherItem = (item: LauncherItem | undefined) => {
    if (!item) return;
    if (item.type === "app") {
      void launchApplication(item.app);
      return;
    }
    if (item.type === "system") {
      invoke("system_power", { action: item.action });
      invoke("hide_window");
      return;
    }
    void runCommand();
  };
```

#### Backend system power command

In `src-tauri/src/commands/mod.rs`, add `pub mod system;` and create `src-tauri/src/commands/system.rs`:

```rust
use tauri::State;
use std::process::Command;

#[tauri::command]
pub fn system_power(action: String) -> Result<(), String> {
    match action.as_str() {
        "restart" => {
            #[cfg(target_os = "macos")]
            {
                Command::new("osascript")
                    .args(["-e", "tell application \"System Events\" to restart"])
                    .spawn()
                    .map_err(|e| e.to_string())?;
            }
            #[cfg(target_os = "linux")]
            {
                Command::new("systemctl")
                    .args(["reboot"])
                    .spawn()
                    .map_err(|e| e.to_string())?;
            }
            #[cfg(target_os = "windows")]
            {
                Command::new("shutdown")
                    .args(["/r", "/t", "0"])
                    .spawn()
                    .map_err(|e| e.to_string())?;
            }
        }
        "shutdown" => {
            #[cfg(target_os = "macos")]
            {
                Command::new("osascript")
                    .args(["-e", "tell application \"System Events\" to shut down"])
                    .spawn()
                    .map_err(|e| e.to_string())?;
            }
            #[cfg(target_os = "linux")]
            {
                Command::new("systemctl")
                    .args(["poweroff"])
                    .spawn()
                    .map_err(|e| e.to_string())?;
            }
            #[cfg(target_os = "windows")]
            {
                Command::new("shutdown")
                    .args(["/s", "/t", "0"])
                    .spawn()
                    .map_err(|e| e.to_string())?;
            }
        }
        _ => return Err(format!("Unknown system action: {action}")),
    }
    Ok(())
}
```

**Platform notes**:
- **macOS**: Uses `osascript` with `System Events` which prompts for permission on first use (standard macOS behavior). Alternative: `shutdown -r now` / `shutdown -h now` but those require root. The `osascript` approach is what most macOS apps use.
- **Linux**: Uses `systemctl` which works on systemd-based distros (all modern ones). Fallback to `reboot`/`poweroff` commands for non-systemd (rare).
- **Windows**: Uses `shutdown /r /t 0` (restart) and `shutdown /s /t 0` (shutdown) — no elevation needed.

Register `system_power` in the invoke handler in `lib.rs`. Add `mod system;` to `commands/mod.rs`.

#### Icon for system items

In the launcher result rendering, the system items should have a distinct icon. Currently:
```tsx
<span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
  {item.type === "app" && appIconUrls[item.app.path] ? (
    <img src={appIconUrls[item.app.path]} alt="" />
  ) : (
    <span>{item.type === "app" ? item.title.slice(0, 1) : "$"}</span>
  )}
</span>
```

For system items, show a power icon character. Add to the fallback:
```tsx
<span>{item.type === "app" ? item.title.slice(0, 1) : item.type === "system" ? "⏻" : "$"}</span>
```

Or use the first letter of the title ("R" for restart, "S" for shutdown). The power symbol `⏻` is cleaner but may not render in all fonts. Use the first letter to be safe:
```tsx
<span>{item.type === "app" ? item.title.slice(0, 1) : item.type === "system" ? item.title.slice(0, 1) : "$"}</span>
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
- `#[cfg(target_os = "...")]` guards for platform-specific code
- No new dependencies (use `std::process::Command` for system power, `tauri::async_runtime` for async)
- i18n keys must be added to BOTH `en` and `zh` tables in `src/i18n.ts`
- The `MessageKey` type is derived from the `en` object's keys — new keys there are automatically typed
