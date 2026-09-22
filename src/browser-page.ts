// Pure logic behind the browser plugin's own page (`src/plugins/browser/`).
//
// The page itself owns a DOM and imports CSS, so nothing in the node suite can
// import it. Everything that is a *decision* rather than a paint lives here
// instead: the shape the Rust commands actually send, the choice of which
// profile a search runs against, and the two small normalizers the settings
// card writes back. `src/plugins/browser/main.ts` only executes what this file
// decides — the same split `src/clipboard-list.ts` gives the clipboard page.

/** One browser/profile pair, mirroring `BrowserProfileInfo` in Rust. */
export type BrowserProfile = {
  browser_id: string;
  browser_name: string;
  profile_key: string;
  profile_dir_name: string;
  profile_name: string;
  base_dir: string;
  has_bookmarks: boolean;
  has_history: boolean;
};

/**
 * One bookmark or history row, mirroring `BrowserItem` in Rust.
 *
 * The two kinds share a shape and the fields one kind does not use are absent
 * rather than null (`#[serde(skip_serializing_if)]` on the Rust side), so the
 * optional members are genuinely optional here.
 */
export type BrowserSearchRow = {
  id: string;
  title: string;
  url: string;
  profile_key: string;
  folder_path?: string;
  date_added?: number;
  visit_count?: number;
  last_visit?: number;
};

/** One open tab, mirroring `BrowserTab` in Rust. */
export type BrowserTab = {
  browser_id: string;
  window_index: number;
  tab_index: number;
  title: string;
  url: string;
  active: boolean;
};

/** The plugin's settings block, mirroring `BrowserPluginSettings` in Rust. */
export type BrowserPluginSettings = {
  /** R26-D · the plugin's on/off switch. The page's own settings card does not
   *  draw it (the switch lives on the host's base-plugins row), but the value
   *  rides the block so the page and the host read one shape. The backend
   *  preserves the stored flag on write regardless of what the page sends. */
  enabled: boolean;
  target: string;
  custom_base_dir: string | null;
  history_days: number;
  cdp_enabled: boolean;
  cdp_port: number;
};

/** The port Chromium's debug endpoint uses unless the user picks another.
 * Mirrors `DEFAULT_CDP_PORT` in `browser_data::tabs`. */
export const DEFAULT_CDP_PORT = 9222;

/** The largest history window the backend honours (ten years). */
export const MAX_HISTORY_DAYS = 3650;

/** The shipped settings, used until the backend answers and when an answer is
 * unreadable. Identical to `BrowserPluginSettings::default()` in Rust. */
export const defaultBrowserSettings = (): BrowserPluginSettings => ({
  enabled: true,
  target: "auto",
  custom_base_dir: null,
  history_days: 30,
  cdp_enabled: false,
  cdp_port: DEFAULT_CDP_PORT,
});

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;

const asString = (value: unknown, fallback = ""): string =>
  typeof value === "string" ? value : fallback;

const asNumber = (value: unknown, fallback: number): number =>
  typeof value === "number" && Number.isFinite(value) ? value : fallback;

const asBoolean = (value: unknown, fallback: boolean): boolean =>
  typeof value === "boolean" ? value : fallback;

/** Every discovered profile, in the order the backend returned them.
 *
 * A malformed entry is dropped rather than repaired: a profile with no
 * `profile_key` cannot be searched, and inventing one would point the reader at
 * a directory that does not exist. */
export const normalizeProfiles = (value: unknown): BrowserProfile[] => {
  if (!Array.isArray(value)) return [];
  const profiles: BrowserProfile[] = [];
  for (const entry of value) {
    const record = asRecord(entry);
    if (!record) continue;
    const profileKey = asString(record.profile_key);
    const browserId = asString(record.browser_id);
    if (!profileKey || !browserId) continue;
    profiles.push({
      browser_id: browserId,
      browser_name: asString(record.browser_name, browserId),
      profile_key: profileKey,
      profile_dir_name: asString(record.profile_dir_name),
      profile_name: asString(record.profile_name, asString(record.profile_dir_name)),
      base_dir: asString(record.base_dir),
      has_bookmarks: asBoolean(record.has_bookmarks, false),
      has_history: asBoolean(record.has_history, false),
    });
  }
  return profiles;
};

/** Bookmark/history rows that have somewhere to go.
 *
 * A row with no URL is not a result — pressing it would open nothing — so it is
 * dropped, and an empty title falls back to the URL so no row renders blank. */
export const normalizeSearchRows = (value: unknown): BrowserSearchRow[] => {
  if (!Array.isArray(value)) return [];
  const rows: BrowserSearchRow[] = [];
  for (const entry of value) {
    const record = asRecord(entry);
    if (!record) continue;
    const url = asString(record.url);
    if (!url) continue;
    const row: BrowserSearchRow = {
      id: asString(record.id, url),
      title: asString(record.title) || url,
      url,
      profile_key: asString(record.profile_key),
    };
    if (typeof record.folder_path === "string") row.folder_path = record.folder_path;
    if (typeof record.date_added === "number") row.date_added = record.date_added;
    if (typeof record.visit_count === "number") row.visit_count = record.visit_count;
    if (typeof record.last_visit === "number") row.last_visit = record.last_visit;
    rows.push(row);
  }
  return rows;
};

/** Open tabs, keeping every row the backend sent (a tab may legitimately have
 * an empty URL — a new-tab page — and its title still identifies it). */
export const normalizeTabs = (value: unknown): BrowserTab[] => {
  if (!Array.isArray(value)) return [];
  const tabs: BrowserTab[] = [];
  for (const entry of value) {
    const record = asRecord(entry);
    if (!record) continue;
    const url = asString(record.url);
    const title = asString(record.title);
    if (!url && !title) continue;
    tabs.push({
      browser_id: asString(record.browser_id),
      window_index: Math.max(0, Math.trunc(asNumber(record.window_index, 0))),
      tab_index: Math.max(0, Math.trunc(asNumber(record.tab_index, 0))),
      title: title || url,
      url,
      active: asBoolean(record.active, false),
    });
  }
  return tabs;
};

/** A stable React-less key for one tab row. Position, not URL: the same page
 * can be open in two windows and the two rows must stay distinct. */
export const tabRowKey = (tab: BrowserTab): string =>
  `tab:${tab.browser_id}:${tab.window_index}:${tab.tab_index}`;

/** The one-line description of a tab: which window it lives in, plus the
 * active marker. The CDP path has no window concept, so a single-window list
 * says nothing rather than "window 0". */
export const tabSubtitle = (tab: BrowserTab, windows: number): string => {
  const parts: string[] = [];
  if (windows > 1) parts.push(`W${tab.window_index}`);
  if (tab.active) parts.push("●");
  parts.push(tab.url);
  return parts.join("  ");
};

/** How many distinct windows a tab list spans — used to decide whether the
 * window label carries any information. */
export const tabWindowCount = (tabs: BrowserTab[]): number =>
  new Set(tabs.map((tab) => tab.window_index)).size;

/** The settings the backend will actually store, given whatever the card holds.
 *
 * The card edits three free-form fields (a directory, a day count, a port), so
 * the normalization the backend applies is applied here too — the user sees the
 * value that was saved instead of the one they typed. */
export const normalizeBrowserSettings = (value: unknown): BrowserPluginSettings => {
  const record = asRecord(value) ?? {};
  const custom = record.custom_base_dir;
  return {
    enabled: asBoolean(record.enabled, true),
    target: asString(record.target, "auto") || "auto",
    custom_base_dir:
      typeof custom === "string" && custom.trim() ? custom.trim() : null,
    history_days: clampHistoryDays(asNumber(record.history_days, 30)),
    cdp_enabled: asBoolean(record.cdp_enabled, false),
    cdp_port: normalizeCdpPort(record.cdp_port),
  };
};

/** Clamp a history window to the range the backend honours. `0` is meaningful
 * (it disables the recency filter), so it is kept rather than floored to 1. */
export const clampHistoryDays = (value: number): number => {
  if (!Number.isFinite(value)) return 30;
  return Math.min(MAX_HISTORY_DAYS, Math.max(0, Math.trunc(value)));
};

/** A port that a TCP connect can actually use. Zero and out-of-range values
 * fall back to the browser's own default rather than writing a dead port. */
export const normalizeCdpPort = (value: unknown): number => {
  const port = typeof value === "number" ? Math.trunc(value) : Number.NaN;
  if (!Number.isFinite(port) || port < 1 || port > 65535) return DEFAULT_CDP_PORT;
  return port;
};

/**
 * Which profile the page searches, given the user's target setting.
 *
 * `target` names a browser id or `"auto"`. Auto prefers a profile that actually
 * has data — an installed-but-never-opened browser has an empty `Default` and
 * would otherwise shadow the one the user really browses with. `null` means
 * there is nothing to search and the page shows its "no browser" state.
 */
export const pickProfileKey = (
  profiles: BrowserProfile[],
  target: string,
): string | null => {
  if (!profiles.length) return null;
  if (target && target !== "auto") {
    const match = profiles.find((profile) => profile.browser_id === target);
    if (match) return match.profile_key;
  }
  const withHistory = profiles.find((profile) => profile.has_history);
  const withBookmarks = profiles.find((profile) => profile.has_bookmarks);
  return (withHistory ?? withBookmarks ?? profiles[0]).profile_key;
};

/**
 * The browser ids the target dropdown offers, one entry per browser, in
 * discovery order and named the way the platform names them.
 */
export const browserTargets = (
  profiles: BrowserProfile[],
): { id: string; name: string }[] => {
  const seen = new Map<string, string>();
  for (const profile of profiles) {
    if (!seen.has(profile.browser_id)) seen.set(profile.browser_id, profile.browser_name);
  }
  return [...seen].map(([id, name]) => ({ id, name }));
};

/** True when a target value names a browser the machine actually has. Used to
 * keep the dropdown's "auto" honest after a browser is uninstalled. */
export const isKnownTarget = (profiles: BrowserProfile[], target: string): boolean =>
  target === "auto" || browserTargets(profiles).some((entry) => entry.id === target);

/**
 * Whether this is a Mac, judged from the webview's user agent.
 *
 * The page cannot ask the backend for its platform (there is no such command,
 * and adding one for a display decision would widen the allowlist for nothing),
 * and the answer only decides whether the settings card *shows* the debug-port
 * block: on macOS tabs come from AppleScript and the port is irrelevant.
 */
export const isMacUserAgent = (userAgent: string): boolean => /Mac|iPhone|iPad/i.test(userAgent);

/** The arguments `browser_activate_tab` takes for one tab row. */
export const tabActivationArgs = (tab: BrowserTab): Record<string, unknown> => ({
  browserId: tab.browser_id,
  windowIndex: tab.window_index,
  tabIndex: tab.tab_index,
  url: tab.url,
});

/** The arguments `browser_open_url` takes for one bookmark/history row. */
export const openRowArgs = (row: BrowserSearchRow): Record<string, unknown> => ({
  profileKey: row.profile_key,
  url: row.url,
});
