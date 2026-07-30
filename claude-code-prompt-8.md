# floter: Complete theme UI, SVG icons, terminal theme colors, pinyin search, bottom padding

You are working on **floter**, a Tauri v2 floating terminal/launcher app. Do NOT modify any code under `~/.hermes/`.

A previous Claude Code session completed the backend and CSS work for dark/light theming, fullscreen window fix, and external terminal fix. But it ran out of turns before completing the frontend. Your job is to finish the remaining work.

## Already done (DO NOT redo):
- CSS variables for dark (`:root`) and light (`[data-theme="light"]`) themes in `src/App.css`
- `--terminal-bg`, `--terminal-fg`, `--terminal-cursor`, `--terminal-selection`, `--terminal-scrollbar` CSS variables defined for both themes
- Backend: theme default changed to "auto" in `config.rs`
- Backend: `raise_window_level` called before AND after show on macOS
- Backend: external terminal `clear` removed
- i18n keys: `settings.theme`, `settings.themeHint`, `settings.theme.dark`, `settings.theme.light`, `settings.theme.auto` added

## Task 1: Frontend theme switching UI + data-theme attribute

### `src/App.tsx`

1. **Set `data-theme` on `<html>`**: Resolve "auto" to "dark"/"light" using `prefers-color-scheme`, then set the attribute:

```typescript
// Resolve "auto" to a concrete theme and apply it to the document.
const resolvedTheme = useMemo(() => {
    if (settings.theme === "auto") {
        return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    }
    return settings.theme;
}, [settings.theme]);

useEffect(() => {
    document.documentElement.setAttribute("data-theme", resolvedTheme);
}, [resolvedTheme]);

// Re-check system theme when in auto mode
useEffect(() => {
    if (settings.theme !== "auto") return;
    const media = window.matchMedia("(prefers-color-scheme: light)");
    const handler = () => {
        document.documentElement.setAttribute(
            "data-theme",
            media.matches ? "light" : "dark",
        );
    };
    media.addEventListener("change", handler);
    return () => media.removeEventListener("change", handler);
}, [settings.theme]);
```

2. **Add theme options constant** (near the top of the file, after imports):

```typescript
const THEME_OPTIONS: { value: string; labelKey: MessageKey }[] = [
    { value: "auto", labelKey: "settings.theme.auto" },
    { value: "dark", labelKey: "settings.theme.dark" },
    { value: "light", labelKey: "settings.theme.light" },
];
```

3. **Add `changeTheme` function** (near `changeLanguage`):

```typescript
const changeTheme = (theme: string) => {
    if (theme === settings.theme) return;
    const updated: AppSettings = { ...settings, theme };
    setSettings(updated);
    suppressBlurUntil.current = Date.now() + 400;
    invoke("save_settings", { settings: updated }).catch(() => undefined);
};
```

4. **Add theme section in settings panel** — insert it BEFORE the language section:

```tsx
<section className="settings-section">
    <h2 className="settings-section__label">{t("settings.theme")}</h2>
    <div className="settings-options" role="radiogroup" aria-label={t("settings.theme")}>
        {THEME_OPTIONS.map((option) => {
            const active = option.value === settings.theme;
            return (
                <button
                    key={option.value}
                    type="button"
                    role="radio"
                    aria-checked={active}
                    className={`settings-option${active ? " settings-option--active" : ""}`}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => changeTheme(option.value)}
                >
                    <span className="settings-option__main">
                        <span className="settings-option__label">{t(option.labelKey)}</span>
                    </span>
                    <span className="settings-option__check" aria-hidden="true">
                        {active ? "✓" : ""}
                    </span>
                </button>
            );
        })}
    </div>
    <p className="settings-section__hint">{t("settings.themeHint")}</p>
</section>
```

## Task 2: SVG icons for system power commands

In the launcher result rendering, replace the text fallback for system items with inline Lucide-style SVG icons.

Current code (around line 1462):
```tsx
<span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
    {item.type === "app" && appIconUrls[item.app.path] ? (
        <img src={appIconUrls[item.app.path]} alt="" />
    ) : (
        <span>{item.type === "command" ? "$" : item.title.slice(0, 1)}</span>
    )}
</span>
```

Replace with:
```tsx
<span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
    {item.type === "app" && appIconUrls[item.app.path] ? (
        <img src={appIconUrls[item.app.path]} alt="" />
    ) : item.type === "system" ? (
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none"
             stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            {item.action === "restart" ? (
                <>
                    <path d="M21 12a9 9 0 1 1-2.64-6.36" />
                    <path d="M21 3v6h-6" />
                </>
            ) : (
                <>
                    <path d="M12 2v10" />
                    <path d="M18.4 6.6a9 9 0 1 1-12.77.04" />
                </>
            )}
        </svg>
    ) : (
        <span>$</span>
    )}
</span>
```

The restart icon is Lucide's `rotate-cw` (circular arrow). The shutdown icon is Lucide's `power` (power button).

## Task 3: Terminal renderer theme colors

In `src/terminal/render.ts`, the hardcoded colors need to read from CSS variables.

Current:
```typescript
const BG = 0x101216;
const FG = 0xd7dae0;
const CURSOR = 0x8bd5ca;
```

These are used throughout the class as constants. Make them instance properties that are read from CSS variables:

1. Add to `RendererOptions`:
```typescript
export interface RendererOptions {
    fontFamily: string;
    fontSize: number;
    lineHeight: number;
    paddingX: number;
    paddingY: number;
}
```

2. In the `TerminalCanvas` class, add a method to read theme colors from CSS:
```typescript
private readThemeColor(varName: string, fallback: number): number {
    const style = getComputedStyle(this.canvas);
    const value = style.getPropertyValue(varName).trim();
    if (!value) return fallback;
    // Parse hex (#rrggbb) or rgb(r, g, b)
    if (value.startsWith('#')) {
        const hex = value.slice(1);
        if (hex.length === 6) {
            return parseInt(hex, 16);
        }
    }
    const rgbMatch = value.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
    if (rgbMatch) {
        return (parseInt(rgbMatch[1]) << 16) | (parseInt(rgbMatch[2]) << 8) | parseInt(rgbMatch[3]);
    }
    return fallback;
}
```

Wait — `getComputedStyle(this.canvas)` won't work because CSS variables are defined on `:root`/`[data-theme]`, not on the canvas element. Use `getComputedStyle(document.documentElement)` instead.

3. Replace the hardcoded constants with instance properties:
```typescript
class TerminalCanvas {
    // ...
    private bg: number;
    private fg: number;
    private cursor: number;
    
    constructor(canvas, opts) {
        // ...
        this.bg = this.readThemeColor('--terminal-bg', 0x101216);
        this.fg = this.readThemeColor('--terminal-fg', 0xd7dae0);
        this.cursor = this.readThemeColor('--terminal-cursor', 0x8bd5ca);
    }
```

4. Replace ALL references to `BG`, `FG`, `CURSOR` with `this.bg`, `this.fg`, `this.cursor` throughout the class.

5. Add a `updateTheme()` method so the theme can be updated without recreating the renderer:
```typescript
updateTheme() {
    this.bg = this.readThemeColor('--terminal-bg', 0x101216);
    this.fg = this.readThemeColor('--terminal-fg', 0xd7dae0);
    this.cursor = this.readThemeColor('--terminal-cursor', 0x8bd5ca);
}
```

6. In `App.tsx`, when the theme changes, call `renderer.updateTheme()`:
```typescript
useEffect(() => {
    const renderer = rendererRef.current;
    if (renderer) {
        renderer.updateTheme();
    }
}, [resolvedTheme]);
```

## Task 4: Pinyin initial matching + multi-word initial matching

### Backend

Add `pinyin` crate to `Cargo.toml`:
```toml
pinyin = "0.4"
```

Add `initials` field to `LocalApplication` in `src-tauri/src/commands/apps/mod.rs`:
```rust
pub struct LocalApplication {
    pub name: String,
    pub localized_name: Option<String>,
    pub path: String,
    pub icon_path: Option<String>,
    pub comment: Option<String>,
    pub initials: String,
}
```

Compute initials in `list_applications` after scanning (before sorting, or after — doesn't matter), using the `pinyin` crate:

```rust
fn compute_initials(name: &str, localized_name: &Option<String>) -> String {
    use pinyin::ToPinyin;
    let combined = match localized_name {
        Some(l) if !l.is_empty() => format!("{name} {l}"),
        _ => name.to_string(),
    };
    let mut result = String::with_capacity(combined.len());
    for c in combined.chars() {
        if c.is_ascii_alphanumeric() {
            result.push(c.to_ascii_lowercase());
        } else if c.is_ascii_whitespace() {
            // skip — word boundary, next alnum will be the next initial
        } else {
            // CJK: get pinyin first letter
            for item in c.to_string().to_pinyin() {
                if let Some(py) = item {
                    if let Some(first) = py.first_letter().chars().next() {
                        result.push(first.to_ascii_lowercase());
                    }
                }
            }
        }
    }
    result
}
```

Check the `pinyin` crate's actual API — `to_pinyin()` returns an iterator of `Option<Pinyin>`. For a single CJK char, it yields one `Some(Pinyin)`. For non-CJK chars, it yields `None`. `Pinyin::first_letter()` returns `&str`.

**Important**: The `into_application` methods in the platform files (`linux.rs`, `macos.rs`, `windows.rs`) construct `LocalApplication` without `initials`. Add `initials: String::new()` to those constructors, and compute the real value in `list_applications` after the scan. This avoids changing every platform file's scan logic.

### Frontend

1. Add `initials` to `LocalApplication` type in `App.tsx`:
```typescript
type LocalApplication = {
    name: string;
    localizedName?: string | null;
    path: string;
    iconPath?: string | null;
    comment?: string | null;
    initials: string;
};
```

2. Add `initials` to `SearchableApp`:
```typescript
type SearchableApp = { app: LocalApplication; names: string[]; initials: string };
```

3. In `searchableApps` useMemo, include initials:
```typescript
const searchableApps = useMemo<SearchableApp[]>(
    () =>
        applications.map((app) => ({
            app,
            names: [...new Set(
                [app.name, app.localizedName, `${app.localizedName ?? ""} ${app.name}`]
                    .filter((name): name is string => Boolean(name))
                    .map(normalizeSearch)
                    .filter(Boolean),
            )],
            initials: app.initials || "",
        })),
    [applications],
);
```

4. Update `scoreApp` to match against initials:
```typescript
const scoreApp = (needle: string, names: string[], initials: string) => {
    let best = 0;
    for (const name of names) {
        const score = scoreNormalized(needle, name);
        if (score > best) best = score;
    }
    if (initials) {
        const initialsScore = scoreNormalized(needle, initials);
        // Strong, but an exact full-name match should still win
        const adjusted = initialsScore >= 1000 ? 950 : initialsScore;
        if (adjusted > best) best = adjusted;
    }
    return best;
};
```

5. Update the call site where `scoreApp` is called to pass `entry.initials`.

## Task 5: Result list bottom padding

In `src/App.css`, change `.launcher-results` bottom padding from `4px` to `8px`:

```css
.launcher-results {
    padding: 3px 4px 8px;
    /* ... rest unchanged */
}
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
- Lucide SVG icons (24×24 viewBox, stroke-width=2) for system commands
- CSS variables for theming, not hardcoded colors
- i18n keys in both en and zh (already added, just use them)
- The `pinyin` crate is the only new Cargo dependency
