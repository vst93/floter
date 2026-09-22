// The browser plugin's page, as a plugin-page citizen.
//
// Three lists — bookmarks, history and the tabs the browser has open right now
// — over one search field, plus the plugin's own settings card in the same
// document. The page is the second consumer of the generic plugin-page
// mechanism (`src-tauri/src/plugin_pages.rs` + `src/plugins/PluginPageHost.tsx`)
// and reaches the host ONLY through the postMessage bridge: no Tauri API, no
// native directory picker, and every command it runs is on its own allowlist.
//
// Why the settings live here and not in the host's Settings screen: the host
// screen edits the whole application (`save_settings` takes the entire settings
// object), and a page has no business rewriting fields it does not own. The
// narrow `browser_get_settings`/`browser_set_settings` pair touches exactly the
// plugin's block, so the card can be complete without the page holding a
// capability it should not have.
//
// Tab capture is deliberately the *third* list and never the gate: a browser
// that is not running, a debug port that is closed, an AppleScript timeout —
// each is caught here and turned into one quiet line under the tab heading.
// Bookmarks and history keep rendering whatever happens.
//
// The pure decisions (what the backend's payload means, which profile to
// search, what the card's numbers normalize to) live in `src/browser-page.ts`;
// this file only paints them.

import "./page.css";
import { createTranslator, normalizeLanguage, type Translate } from "../../i18n";
import {
  BRIDGE_TAG,
  PLUGIN_PAGE_PROTOCOL,
  isBridgeGlass,
  isBridgeOpacity,
  isBridgeReload,
  isBridgeResultForSession,
  isBridgeTheme,
  isBridgeVisibility,
} from "../../plugin-pages";
import {
  GLASS_FRAME_FLOOR,
  GLASS_SOLID_TOP,
  GLASS_STEP_TOKENS,
  glassPageContentAlpha,
  glassPageRowAlpha,
  normalizeGlassStep,
  type GlassStep,
} from "../../glass-material";
import {
  browserTargets,
  defaultBrowserSettings,
  isKnownTarget,
  isMacUserAgent,
  normalizeBrowserSettings,
  normalizeProfiles,
  normalizeSearchRows,
  normalizeTabs,
  openRowArgs,
  pickProfileKey,
  tabActivationArgs,
  tabSubtitle,
  tabWindowCount,
  type BrowserPluginSettings,
  type BrowserProfile,
  type BrowserSearchRow,
  type BrowserTab,
} from "../../browser-page";

// ---- bridge client --------------------------------------------------------

type PendingCall = {
  resolve: (value: unknown) => void;
  reject: (error: string) => void;
  timer: number;
};

const pending = new Map<number, PendingCall>();
let nextCallId = 1;
const bridgeSession = crypto.randomUUID();

/** How long to wait for the host's reply before giving up. Without this a
 * dropped message leaves the awaiting UI hung with no way back. */
const BRIDGE_TIMEOUT_MS = 10_000;

/** Ask the host to start a native window drag. The page is a sandboxed iframe,
 * so a mousedown in here cannot reach the host's `startDrag`; the press is
 * reported as a payload-free message instead. */
const requestWindowDrag = () => {
  window.parent.postMessage({ [BRIDGE_TAG]: "drag" }, "*");
};

const requestClose = () => {
  window.parent.postMessage({ [BRIDGE_TAG]: "close" }, "*");
};

let pageDisposed = false;
let pageVisible = true;

window.addEventListener("message", (event: MessageEvent) => {
  if (event.source !== window.parent) return;
  const data: unknown = event.data;
  if (isBridgeVisibility(data)) {
    pageVisible = data.visible;
    if (data.visible) void reloadAll();
    return;
  }
  if (isBridgeOpacity(data)) {
    applyOpacity(data.mainOpacity, data.terminalOpacity);
    return;
  }
  if (isBridgeGlass(data)) {
    applyGlassStep(data.glassStep);
    return;
  }
  if (isBridgeTheme(data)) {
    activeTheme = data.theme;
    document.documentElement.setAttribute("data-theme", data.theme);
    const raw = Number.parseFloat(rootStyle.getPropertyValue("--terminal-opacity"));
    applyPageBackground(Number.isFinite(raw) ? raw : 0.46);
    return;
  }
  if (isBridgeReload(data)) {
    void reloadAll();
    return;
  }
  if (!isBridgeResultForSession(data, bridgeSession)) return;
  const call = pending.get(data.id);
  if (!call) return;
  pending.delete(data.id);
  window.clearTimeout(call.timer);
  if (data.ok) call.resolve(data.value);
  else call.reject(data.error);
});

// The handshake, first thing the page says: which protocol it was written
// against. The host refuses a mismatch with a readable error instead of
// answering half a conversation.
window.parent.postMessage(
  { [BRIDGE_TAG]: "frame-ready", protocol: PLUGIN_PAGE_PROTOCOL },
  "*",
);

/** Run one allowlisted host command through the postMessage bridge. */
const invokeCommand = <T>(command: string, args?: Record<string, unknown>): Promise<T> =>
  new Promise<T>((resolve, reject) => {
    if (pageDisposed) {
      reject("Browser page closed");
      return;
    }
    const id = nextCallId++;
    const timer = window.setTimeout(() => {
      pending.delete(id);
      reject(`Bridge call timed out after ${BRIDGE_TIMEOUT_MS}ms: ${command}`);
    }, BRIDGE_TIMEOUT_MS);
    pending.set(id, {
      resolve: (value) => resolve(value as T),
      reject: (error) => reject(error),
      timer,
    });
    window.parent.postMessage(
      { [BRIDGE_TAG]: "invoke", id, session: bridgeSession, command, args: args ?? {} },
      "*",
    );
  });

// ---- bootstrap ------------------------------------------------------------

const params = new URLSearchParams(window.location.search);
const t: Translate = createTranslator(normalizeLanguage(params.get("lang") ?? "en"));
let activeTheme: "dark" | "light" = params.get("theme") === "light" ? "light" : "dark";
const rootStyle = document.documentElement.style;
const pageRgb = { dark: "17, 18, 20", light: "250, 250, 252" } as const;

/** Whether this is a Mac, which is what decides if the settings card shows the
 * debug-port block: macOS reads tabs through AppleScript and has no use for it. */
const isMac = isMacUserAgent(navigator.userAgent);

/** The page sheet's alpha: the frame alpha (the transparency slider, clamped to
 * the near-solid top) composited under the material step's haze. Mirrors
 * base.css's `--glass-frame-alpha` / `--glass-tint-alpha`, the same arithmetic
 * the clipboard page uses. */
function applyPageBackground(transparency: number) {
  const number = (name: string, fallback: number) => {
    const value = Number.parseFloat(rootStyle.getPropertyValue(name));
    return Number.isFinite(value) ? value : fallback;
  };
  const floor = number("--glass-frame-floor", GLASS_FRAME_FLOOR);
  const solidTop = number("--glass-solid-top", GLASS_SOLID_TOP);
  const haze = number("--glass-step-dim", 0);
  const frame = Math.min(solidTop, Math.max(floor, transparency));
  const alpha = 1 - (1 - haze * (1 - transparency)) * (1 - frame);
  rootStyle.setProperty("--page-fill", String(alpha));
  rootStyle.setProperty("--page-bg", `rgba(${pageRgb[activeTheme]}, ${alpha})`);
}

/** Adopt a material step from the shared token table, never from literals. */
function applyGlassStep(step: GlassStep) {
  const tokens = GLASS_STEP_TOKENS[step];
  rootStyle.setProperty("--glass-step-dim", String(tokens.dim));
  rootStyle.setProperty("--glass-solid-top", String(GLASS_SOLID_TOP));
  rootStyle.setProperty("--glass-frame-floor", String(GLASS_FRAME_FLOOR));
  const raw = Number.parseFloat(rootStyle.getPropertyValue("--terminal-opacity"));
  applyPageBackground(Number.isFinite(raw) ? raw : 0.46);
}

function applyOpacity(main: number, terminal: number) {
  rootStyle.setProperty("--main-opacity", String(main));
  rootStyle.setProperty("--terminal-opacity", String(terminal));
  rootStyle.setProperty("--glass-content-alpha", String(glassPageContentAlpha(terminal)));
  rootStyle.setProperty("--glass-row-alpha", String(glassPageRowAlpha(terminal)));
  applyPageBackground(terminal);
}

/** Parse one opacity param, keeping a deliberate `0`. */
const opacityParam = (name: string, fallback: number): number => {
  const raw = params.get(name);
  if (raw === null || raw.trim() === "") return fallback;
  const value = Number(raw);
  return Number.isFinite(value) ? value : fallback;
};

applyOpacity(opacityParam("main-opacity", 0.47), opacityParam("terminal-opacity", 0.46));
applyGlassStep(normalizeGlassStep(params.get("glass-step")));
document.documentElement.setAttribute("data-theme", activeTheme);

// ---- state ----------------------------------------------------------------

/** Rows fetched per list. The page scrolls, so it can afford more than the
 * launcher's eight — but still a page, not the whole history. */
const FETCH_LIMIT = 60;

let settings: BrowserPluginSettings = defaultBrowserSettings();
let profiles: BrowserProfile[] = [];
let query = "";
let bookmarks: BrowserSearchRow[] = [];
let history: BrowserSearchRow[] = [];
let tabs: BrowserTab[] = [];
/** The tab read's soft failure, rendered as one line under the tab heading. */
let tabsError: string | null = null;
let loading = true;
let loadFailed = false;
let settingsOpen = false;
/** Set after a save whose custom directory held no profile, so the card can say
 * so instead of silently claiming success. */
let directoryWarning = false;
let settingsNotice: "saved" | "failed" | null = null;

// ---- data -----------------------------------------------------------------

const searchProfileKey = (): string | null => pickProfileKey(profiles, settings.target);

/** Load everything the page shows. Each list absorbs its own failure: a tab
 * read that fails leaves the other two untouched. */
async function loadAll(): Promise<void> {
  loading = true;
  loadFailed = false;
  render();
  try {
    const [rawSettings, rawProfiles] = await Promise.all([
      invokeCommand<unknown>("browser_get_settings"),
      invokeCommand<unknown>("browser_discover"),
    ]);
    settings = normalizeBrowserSettings(rawSettings);
    profiles = normalizeProfiles(rawProfiles);
  } catch {
    loading = false;
    loadFailed = true;
    render();
    return;
  }

  const profileKey = searchProfileKey();
  const trimmed = query.trim();
  const [rawBookmarks, rawHistory, rawTabs] = await Promise.all([
    profileKey
      ? invokeCommand<unknown>("browser_search_bookmarks", {
          profileKey,
          query: trimmed,
          limit: FETCH_LIMIT,
        }).catch(() => [])
      : Promise.resolve([]),
    profileKey
      ? invokeCommand<unknown>("browser_search_history", {
          profileKey,
          query: trimmed,
          limit: FETCH_LIMIT,
          days: settings.history_days,
        }).catch(() => [])
      : Promise.resolve([]),
    // The tab read is the one that is *expected* to fail on a browser started
    // without the debug flag, so its rejection is a value, not an error.
    profileKey
      ? invokeCommand<unknown>("browser_list_tabs", {
          profileKeyOrBrowser: profileKey,
          limit: FETCH_LIMIT,
        }).then(
          (value) => ({ tabs: normalizeTabs(value), error: null as string | null }),
          (error: unknown) => ({ tabs: [] as BrowserTab[], error: String(error) }),
        )
      : Promise.resolve({ tabs: [] as BrowserTab[], error: null as string | null }),
  ]);

  bookmarks = normalizeSearchRows(rawBookmarks);
  history = normalizeSearchRows(rawHistory);
  tabs = rawTabs.tabs;
  tabsError = rawTabs.error;
  loading = false;
  render();
}

let searchTimer = 0;
const scheduleSearch = () => {
  window.clearTimeout(searchTimer);
  searchTimer = window.setTimeout(() => void loadAll(), 180);
};

// ---- actions --------------------------------------------------------------

/** Open one bookmark or history row in the browser it came from. */
async function openRow(row: BrowserSearchRow): Promise<void> {
  try {
    await invokeCommand("browser_open_url", openRowArgs(row));
    requestClose();
  } catch {
    render();
  }
}

/** Bring one open tab to the front. The backend degrades to opening the tab's
 * URL when the browser is not running, so this always does something useful. */
async function activateTab(tab: BrowserTab): Promise<void> {
  try {
    await invokeCommand("browser_activate_tab", tabActivationArgs(tab));
    requestClose();
  } catch {
    render();
  }
}

/** Write the card's state back and re-read, so the page shows what was stored
 * rather than what was typed. Saving clears the backend's discovery cache, so
 * a new custom directory is picked up by the very next `browser_discover`. */
async function saveSettings(next: BrowserPluginSettings): Promise<void> {
  try {
    const stored = await invokeCommand<unknown>("browser_set_settings", { settings: next });
    settings = normalizeBrowserSettings(stored);
    settingsNotice = "saved";
    const rawProfiles = await invokeCommand<unknown>("browser_discover");
    profiles = normalizeProfiles(rawProfiles);
    // A custom directory is only useful if discovery found a profile in it.
    directoryWarning = Boolean(settings.custom_base_dir) && !profiles.some((p) => p.browser_id === "custom");
    if (!isKnownTarget(profiles, settings.target)) settings.target = "auto";
  } catch {
    settingsNotice = "failed";
  }
  await loadAll();
}

// ---- rendering ------------------------------------------------------------

const root = document.getElementById("root") as HTMLElement;

const el = <K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
};

const searchInput = el("input", "browser-page__search");
searchInput.type = "text";
searchInput.autocomplete = "off";
searchInput.spellcheck = false;
searchInput.placeholder = t("browserPage.search");
searchInput.setAttribute("aria-label", t("browserPage.search"));

const settingsToggle = el("button", "browser-page__icon-button");
settingsToggle.type = "button";
settingsToggle.setAttribute("aria-pressed", "false");

const reloadButton = el("button", "browser-page__icon-button");
reloadButton.type = "button";
reloadButton.title = t("browserPage.reload");
reloadButton.setAttribute("aria-label", t("browserPage.reload"));

const body = el("div", "browser-page__body");

/** The page sheet: header (search + two icon buttons) over a scrollable body. */
const panel = el("div", "browser-page");
const topbar = el("div", "browser-page__topbar");
const prompt = el("span", "browser-page__prompt", "❯");
topbar.append(prompt, searchInput, settingsToggle, reloadButton);
panel.append(topbar, body);

/** One group: a heading and its rows, or one quiet line when it is empty. */
function group(
  title: string,
  rows: HTMLElement[],
  emptyText: string,
  emptyHint?: string,
): HTMLElement {
  const block = el("section", "browser-page__group");
  block.append(el("div", "browser-page__group-title", title));
  if (rows.length) {
    const list = el("div", "browser-page__list");
    list.append(...rows);
    block.append(list);
    return block;
  }
  const empty = el("div", "browser-page__empty", emptyText);
  block.append(empty);
  if (emptyHint) block.append(el("div", "browser-page__hint", emptyHint));
  return block;
}

/** One bookmark/history row. */
function rowForItem(row: BrowserSearchRow, kind: "bookmark" | "history"): HTMLElement {
  const button = el("button", `browser-row browser-row--${kind}`);
  button.type = "button";
  const title = el("span", "browser-row__title", row.title);
  title.title = row.title;
  const subtitle = el("span", "browser-row__subtitle", row.url);
  subtitle.title = row.url;
  button.append(el("span", `browser-row__glyph browser-row__glyph--${kind}`), title, subtitle);
  button.addEventListener("click", () => void openRow(row));
  return button;
}

/** One open-tab row. Enter/click activates the tab rather than opening the URL
 * in a new one — the tab already exists. */
function rowForTab(tab: BrowserTab, windows: number): HTMLElement {
  const button = el("button", "browser-row browser-row--tab");
  button.type = "button";
  if (tab.active) button.classList.add("browser-row--active");
  const title = el("span", "browser-row__title", tab.title);
  title.title = tab.title;
  const subtitle = el("span", "browser-row__subtitle", tabSubtitle(tab, windows));
  subtitle.title = tab.url;
  button.append(el("span", "browser-row__glyph browser-row__glyph--tab"), title, subtitle);
  button.setAttribute("aria-label", `${t("browserPage.activate")}: ${tab.title}`);
  button.addEventListener("click", () => void activateTab(tab));
  return button;
}

/** One labelled control row inside the settings card. */
function field(labelText: string, control: HTMLElement, hint?: string): HTMLElement {
  const wrapper = el("label", "browser-field");
  wrapper.append(el("span", "browser-field__label", labelText), control);
  if (hint) wrapper.append(el("span", "browser-field__hint", hint));
  return wrapper;
}

/** The plugin's own settings card. Every control writes the whole settings
 * object back through `browser_set_settings`; the backend normalizes. */
function settingsCard(): HTMLElement {
  const card = el("div", "browser-page__settings");
  card.append(el("div", "browser-page__settings-title", t("browserPage.settings")));

  const target = el("select", "browser-field__control");
  const auto = el("option", undefined, t("settings.browserTargetAuto"));
  auto.value = "auto";
  target.append(auto);
  for (const entry of browserTargets(profiles)) {
    const option = el("option", undefined, entry.name);
    option.value = entry.id;
    target.append(option);
  }
  target.value = settings.target;
  target.addEventListener("change", () => {
    void saveSettings({ ...settings, target: target.value });
  });
  card.append(field(t("settings.browserTarget"), target));

  const dirInput = el("input", "browser-field__control browser-field__control--text");
  dirInput.type = "text";
  dirInput.value = settings.custom_base_dir ?? "";
  dirInput.placeholder = t("browserPage.customDirPlaceholder");
  dirInput.spellcheck = false;
  const dirRow = el("div", "browser-field__row");
  const useDefault = el("button", "browser-page__text-button", t("browserPage.useDefaultDir"));
  useDefault.type = "button";
  const commitDir = () => {
    const value = dirInput.value.trim();
    void saveSettings({ ...settings, custom_base_dir: value ? value : null });
  };
  // Enter commits; blurring does not, so a half-typed path is never saved by
  // clicking elsewhere in the card.
  dirInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      commitDir();
    }
  });
  useDefault.addEventListener("click", () => {
    dirInput.value = "";
    void saveSettings({ ...settings, custom_base_dir: null });
  });
  dirRow.append(dirInput, useDefault);
  // A plain `<div>`, not a `<label>`: a label wrapping a button makes the
  // button's click focus the field as a side effect, and this row is a field
  // *and* an action. The field carries its own `aria-label` instead.
  const dirField = el("div", "browser-field");
  dirInput.setAttribute("aria-label", t("settings.browserCustomDir"));
  dirField.append(
    el("span", "browser-field__label", t("settings.browserCustomDir")),
    dirRow,
    el("span", "browser-field__hint", t("settings.browserCustomDirHint")),
  );
  card.append(dirField);

  const days = el("input", "browser-field__control browser-field__control--number");
  days.type = "number";
  days.min = "0";
  days.max = "3650";
  days.value = String(settings.history_days);
  days.addEventListener("change", () => {
    const value = Number(days.value);
    void saveSettings({ ...settings, history_days: Number.isFinite(value) ? value : 0 });
  });
  card.append(
    field(
      t("settings.browserHistoryDays"),
      days,
      t("settings.browserHistoryDaysHint"),
    ),
  );

  if (!isMac) {
    const cdp = el("input", "browser-field__checkbox");
    cdp.type = "checkbox";
    cdp.checked = settings.cdp_enabled;
    cdp.addEventListener("change", () => {
      void saveSettings({ ...settings, cdp_enabled: cdp.checked });
    });
    card.append(field(t("browserPage.cdp"), cdp, t("browserPage.cdpHint")));

    const port = el("input", "browser-field__control browser-field__control--number");
    port.type = "number";
    port.min = "1";
    port.max = "65535";
    port.value = String(settings.cdp_port);
    port.addEventListener("change", () => {
      void saveSettings({ ...settings, cdp_port: Number(port.value) });
    });
    card.append(field(t("browserPage.cdpPort"), port));
  }

  if (directoryWarning) {
    card.append(el("div", "browser-page__notice", t("browserPage.dirNoProfile")));
  }
  if (settingsNotice === "saved") {
    card.append(el("div", "browser-page__notice", t("browserPage.settingsSaved")));
  } else if (settingsNotice === "failed") {
    card.append(el("div", "browser-page__notice browser-page__notice--error", t("browserPage.settingsFailed")));
  }
  return card;
}

function render(): void {
  settingsToggle.setAttribute("aria-pressed", String(settingsOpen));
  settingsToggle.classList.toggle("browser-page__icon-button--on", settingsOpen);
  settingsToggle.title = t("browserPage.settings");
  settingsToggle.setAttribute("aria-label", t("browserPage.settings"));
  searchInput.placeholder = t("browserPage.search");
  reloadButton.title = t("browserPage.reload");

  body.replaceChildren();

  if (settingsOpen) {
    body.append(settingsCard());
    return;
  }

  if (loading) {
    const block = el("div", "browser-page__empty", t("settings.loading"));
    block.setAttribute("role", "status");
    block.setAttribute("aria-busy", "true");
    body.append(block);
    return;
  }
  if (loadFailed) {
    body.append(el("div", "browser-page__empty", t("browserPage.loadFailed")));
    return;
  }
  if (!profiles.length) {
    body.append(el("div", "browser-page__empty", t("launcher.browserNoProfile")));
    return;
  }

  const emptyText = t("launcher.browserEmpty");
  body.append(
    group(
      t("launcher.browserBookmarks"),
      bookmarks.map((row) => rowForItem(row, "bookmark")),
      emptyText,
    ),
    group(
      t("launcher.browserHistory"),
      history.map((row) => rowForItem(row, "history")),
      emptyText,
    ),
  );

  const windows = tabWindowCount(tabs);
  const tabRows = tabs.map((tab) => rowForTab(tab, windows));
  // A failed tab read reports the backend's own one-line reason, with the
  // localized how-to underneath. A successful read with no rows says so and
  // still explains where tabs come from.
  body.append(
    group(
      t("browserPage.tabs"),
      tabRows,
      tabsError ?? t("browserPage.noTabs"),
      tabRows.length ? undefined : t("browserPage.tabsHint"),
    ),
  );
}

// ---- wiring ---------------------------------------------------------------

searchInput.addEventListener("input", () => {
  query = searchInput.value;
  scheduleSearch();
});
searchInput.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && searchInput.value) {
    event.preventDefault();
    searchInput.value = "";
    query = "";
    void loadAll();
  }
});

settingsToggle.addEventListener("mousedown", (event) => event.preventDefault());
settingsToggle.addEventListener("click", () => {
  settingsOpen = !settingsOpen;
  settingsNotice = null;
  render();
});

reloadButton.addEventListener("mousedown", (event) => event.preventDefault());
reloadButton.addEventListener("click", () => void loadAll());

// A press on the page's blank chrome moves the window; the host runs the same
// drag every other shell's chrome uses. Interactive elements are exempt so a
// press on a row, a field or a button is never a drag.
panel.addEventListener("mousedown", (event) => {
  if (event.button !== 0) return;
  const target = event.target as Element | null;
  if (target?.closest("button, input, select, a, .browser-page__body")) return;
  event.preventDefault();
  requestWindowDrag();
});

// Esc closes the page, the same key the host's own chrome listens for. The
// search field handles its own Escape first (it clears the query).
window.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  if (document.activeElement === searchInput && searchInput.value) return;
  event.preventDefault();
  requestClose();
});

window.addEventListener("beforeunload", () => {
  pageDisposed = true;
});

root.append(panel);
searchInput.focus();
render();
void reloadAll();

/** Reload, unless the page is hidden — a hidden frame has nothing to paint. */
function reloadAll(): Promise<void> {
  if (!pageVisible) return Promise.resolve();
  return loadAll();
}
