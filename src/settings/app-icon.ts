// R150 · the application icon's light/dark variants, as values.
//
// The page (`GeneralPage.tsx`) is JSX around these decisions, and the Rust
// side mirrors them (`app_icon.rs` + `normalize_settings` in
// `src-tauri/src/commands/config.rs`). The switch is a *setting*, not a
// command: the frontend persists `app_icon` through the ordinary settings save
// and the backend installs it, so there is no second source of truth to keep
// in step.
//
// The icon the user sees is the menu bar / tray icon (the app is a
// menu-bar-resident, so macOS runs it as an `Accessory` with no Dock icon); on
// Windows/Linux the same image is the window's taskbar icon. The packaged
// bundle icon is fixed at build time, so only the runtime surfaces switch.
//
// Keeping the vocabulary here rather than inline in the JSX is what makes the
// round's red line testable: an unknown or hand-edited stored value resolving
// to the wrong variant fails a unit test instead of only being visible on a
// real menu bar.

import type { MessageKey } from "../i18n";

/** The two appearances, in the order the picker paints them. */
export const APP_ICON_APPEARANCES = ["dark", "light"] as const;

export type AppIconAppearance = (typeof APP_ICON_APPEARANCES)[number];

/** The shipped appearance. Must equal `DEFAULT_APP_ICON` in
 *  `src-tauri/src/app_icon.rs`: every build through R149 shipped the black
 *  mark, so a pre-hydration frame and an upgraded settings file both have to
 *  resolve to it. */
export const DEFAULT_APP_ICON: AppIconAppearance = "dark";

/** One label key per appearance, spelled out. A dynamic
 *  `settings.appIcon.${value}` would leave the keys with no static consumer
 *  (the R142 dictionary sweep) and turn a typo into a blank label; the record
 *  makes both a compile error instead. */
export const APP_ICON_LABEL_KEYS: Record<AppIconAppearance, MessageKey> = {
  dark: "settings.appIcon.dark",
  light: "settings.appIcon.light",
};

/** Map any stored value onto a shipped appearance. An unknown or hand-edited
 *  value lands on the default rather than on whichever variant happens to be
 *  first — the same policy `normalizeUiScale` applies to a size step. */
export function normalizeAppIconAppearance(value: unknown): AppIconAppearance {
  return (APP_ICON_APPEARANCES as readonly unknown[]).includes(value)
    ? (value as AppIconAppearance)
    : DEFAULT_APP_ICON;
}

/** The label key the picker paints for a stored value. The value is normalized
 *  first, so an unknown string can only ever paint the default's label. */
export function appIconLabelKey(value: unknown): MessageKey {
  return APP_ICON_LABEL_KEYS[normalizeAppIconAppearance(value)];
}
