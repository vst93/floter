// The clipboard history page, as a plugin-page citizen.
//
// This is the extracted, framework-free successor of the old in-app React
// panel: same markup classes (the shared stylesheet is imported below), same
// history/favorites/tabs/files/images behavior, and floter's Spotlight
// keyboard discipline — typing anywhere routes to the filter field, Backspace
// from the list routes a character deletion into the filter (never a row),
// Tab toggles the all/favorites view, single-key row commands (F/* favorite,
// D/Del delete) only fire once the user deliberately clicked into the list,
// and Cmd/Ctrl+Backspace deletes the selected row from any focus.
//
// The surface's transparency level follows the terminal page's configured
// opacity: `page.css` colors the panel with the same `--terminal-opacity`
// custom property the host window sets, so moving one slider moves both.
//
// It runs inside a sandboxed iframe served by the generic plugin-page
// pipeline and reaches the host ONLY through the postMessage bridge — every
// command here goes over that bridge, dogfooding the mechanism end to end.

import "./page.css";
import { createTranslator, normalizeLanguage, type Translate } from "../../i18n";
import {
  clipboardPreview,
  filterClipboardEntries,
  formatClipboardAge,
  formatFilesPreview,
  imageFileMime,
  isFilesPreviewCandidate,
  looksLikeDirectoryPath,
  normalizeEntries,
  normalizeClipboardSession,
  sameClipboardSnapshot,
  shouldActivateClipboardEntry,
  type ClipboardEntry,
} from "../../clipboard-history";
import { BRIDGE_TAG, isBridgeOpacity, isBridgeTheme, isBridgeResultForSession, isBridgeReload, isBridgeVisibility } from "../../plugin-pages";

// ---- bridge client -------------------------------------------------------

type PendingCall = {
  resolve: (value: unknown) => void;
  reject: (error: string) => void;
  timer: number;
};

const pending = new Map<number, PendingCall>();
let nextCallId = 1;
const bridgeSession = crypto.randomUUID();

/** How long to wait for the host's reply before giving up on a call. Without
 * this a dropped or ignored message leaves the promise pending forever, and
 * the awaiting UI (a reload, a favorite toggle) hangs with no way back. */
const BRIDGE_TIMEOUT_MS = 10_000;

window.addEventListener("message", (event: MessageEvent) => {
  // Only the host window may talk to us.
  if (event.source !== window.parent) return;
  const data: unknown = event.data;
  if (isBridgeVisibility(data)) {
    if (data.visible) void handleReload();
    else handleHidden();
    return;
  }
  if (isBridgeOpacity(data)) {
    // Opacity sliders moved host-side; restyle in place.
    applyOpacity(data.mainOpacity, data.terminalOpacity);
    return;
  }
  if (isBridgeTheme(data)) {
    // Theme changed host-side; update the page's data-theme attribute and its
    // opaque page background without relying on rgba() variable alpha syntax.
    activeTheme = data.theme;
    document.documentElement.setAttribute("data-theme", data.theme);
    const rawOpacity = Number(rootStyle.getPropertyValue("--terminal-opacity"));
    applyPageBackground(Number.isFinite(rawOpacity) ? rawOpacity : 0.92);
    return;
  }
  if (isBridgeReload(data)) {
    // Page just became visible after being hidden; reload data.
    void handleReload();
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

/** Run one allowlisted host command through the postMessage bridge. */
const invokeCommand = <T>(
  command: string,
  args?: Record<string, unknown>,
): Promise<T> =>
  new Promise<T>((resolve, reject) => {
    if (pageDisposed) { reject("Clipboard page closed"); return; }
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
    window.parent.postMessage({ [BRIDGE_TAG]: "invoke", id, session: bridgeSession, command, args: args ?? {} }, "*");
  });

const requestClose = () => {
  window.parent.postMessage({ [BRIDGE_TAG]: "close" }, "*");
};

// ---- bootstrap ------------------------------------------------------------

const params = new URLSearchParams(window.location.search);
const t: Translate = createTranslator(normalizeLanguage(params.get("lang") ?? "en"));
const theme: "dark" | "light" = params.get("theme") === "light" ? "light" : "dark";
let activeTheme: "dark" | "light" = theme;
const rootStyle = document.documentElement.style;
const pageRgb = {
  dark: "17, 18, 20",
  light: "250, 250, 252",
} as const;

function applyPageBackground(terminal: number) {
  // WebKit rejects rgba() when its alpha argument is a CSS variable. Keep the
  // complete color as one custom property instead of composing it in CSS.
  rootStyle.setProperty("--page-bg", `rgba(${pageRgb[activeTheme]}, ${terminal})`);
}

function applyOpacity(main: number, terminal: number) {
  rootStyle.setProperty("--main-opacity", String(main));
  rootStyle.setProperty("--terminal-opacity", String(terminal));
  applyPageBackground(terminal);
}

/** Parse one opacity param, keeping a deliberate `0` (fully transparent) —
 * `Number(x) || fallback` silently promoted it to the default. */
const opacityParam = (name: string, fallback: number): number => {
  const raw = params.get(name);
  if (raw === null || raw.trim() === "") return fallback;
  const value = Number(raw);
  return Number.isFinite(value) ? value : fallback;
};

applyOpacity(opacityParam("main-opacity", 0.94), opacityParam("terminal-opacity", 0.92));
document.documentElement.setAttribute("data-theme", theme);

// ---- state ----------------------------------------------------------------

type ClipboardView = "all" | "favorites";
const SESSION_KEY = "floter.clipboard.session";
const savedSession = (() => {
  try { return normalizeClipboardSession(JSON.parse(sessionStorage.getItem(SESSION_KEY) ?? "null")); }
  catch { return normalizeClipboardSession(null); }
})();

let entries: ClipboardEntry[] = [];
let filterText = savedSession.filterText;
let view: ClipboardView = savedSession.view;
let selected = 0;
let hydrated = false;
let busy = false;
let clearArmed = false;
let clearTimer: number | null = null;
let noticeTimer: number | null = null;
let pageVisible = true;
let pageDisposed = false;
const thumbnails = new Map<string, string>();
let statuses: Record<string, boolean> = {};
/** True when the last entry fetch failed or timed out — an empty list then
 * means "we could not ask", not "nothing copied yet", so the page offers a
 * retry instead of a misleading empty state. */
let loadFailed = false;
/** Interval ID for periodic refresh while visible. */
let refreshInterval: number | null = null;

const scopedEntries = (): ClipboardEntry[] =>
  view === "favorites" ? entries.filter((entry) => entry.favorite) : entries;
const filteredEntries = (): ClipboardEntry[] =>
  filterClipboardEntries(scopedEntries(), filterText);

// ---- DOM scaffold ---------------------------------------------------------

const root = document.getElementById("root") ?? document.body;

root.innerHTML = `
  <div class="clipboard-panel">
    <div class="clipboard-panel__topbar">
      <span class="clipboard-panel__prompt" aria-hidden="true"></span>
      <input class="clipboard-panel__search" maxlength="512" spellcheck="false" autocapitalize="off" autocorrect="off" />
      <button type="button" class="clipboard-panel__filter-clear" hidden>×</button>
      <div class="clipboard-panel__tabs" role="tablist">
        <button type="button" role="tab" class="clipboard-panel__tab" data-view="all"></button>
        <button type="button" role="tab" class="clipboard-panel__tab" data-view="favorites"></button>
      </div>
    </div>
    <div class="clipboard-panel__content"></div>
    <div class="clipboard-panel__notice" role="alert" hidden></div>
    <div class="clipboard-panel__footer">
      <span class="clipboard-panel__hints"></span>
      <button type="button" class="clipboard-panel__clear"></button>
    </div>
  </div>
`;

const promptLabel = root.querySelector<HTMLElement>(".clipboard-panel__prompt")!;
const searchInput = root.querySelector<HTMLInputElement>(".clipboard-panel__search")!;
searchInput.value = filterText;
const notice = root.querySelector<HTMLElement>(".clipboard-panel__notice")!;

const showError = () => {
  notice.textContent = t("clipboard.actionFailed");
  notice.hidden = false;
  if (noticeTimer !== null) window.clearTimeout(noticeTimer);
  noticeTimer = window.setTimeout(() => { notice.hidden = true; }, 5000);
};

const saveSession = () => {
  if (!hydrated) return;
  try {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify({
      filterText, view, selectedId: filteredEntries()[selected]?.id ?? null, scrollTop: content.scrollTop,
    }));
  } catch { /* Session storage may be unavailable in a sandbox. */ }
};
const filterClear = root.querySelector<HTMLButtonElement>(".clipboard-panel__filter-clear")!;
const tabAll = root.querySelector<HTMLButtonElement>('[data-view="all"]')!;
const tabFavorites = root.querySelector<HTMLButtonElement>('[data-view="favorites"]')!;
const content = root.querySelector<HTMLElement>(".clipboard-panel__content")!;
const hints = root.querySelector<HTMLElement>(".clipboard-panel__hints")!;
const clearButton = root.querySelector<HTMLButtonElement>(".clipboard-panel__clear")!;
const panel = root.querySelector<HTMLElement>(".clipboard-panel")!;

// ---- rendering ------------------------------------------------------------

const countBadge = (count: number) => {
  const badge = document.createElement("span");
  badge.className = "clipboard-panel__tab-count";
  badge.textContent = String(count);
  return badge;
};

/** Text-entry marker: a plain chevron prefix, the same glyph the list has
 * used since the in-app React panel. */
const renderHistoryEntry = (marker: HTMLElement) => {
  marker.textContent = "›";
};

/** Image-entry marker: the thumbnail when the bytes have arrived, a "[?]"
 * placeholder while they are still in flight or have failed. */
const renderImageEntry = (
  marker: HTMLElement,
  entry: ClipboardEntry,
  hasThumbnail: boolean,
) => {
  if (hasThumbnail) {
    const img = document.createElement("img");
    img.src = thumbnails.get(entry.id)!;
    img.alt = "";
    img.draggable = false;
    marker.append(img);
  } else {
    marker.textContent = "[?]";
  }
};

/** Files-entry marker: thumbnail if the first path is an image file, a
 * triangle for directory paths, a square for individual files. */
const renderFilesEntry = (
  marker: HTMLElement,
  entry: ClipboardEntry,
  hasThumbnail: boolean,
) => {
  if (isFilesPreviewCandidate(entry.paths)) {
    if (hasThumbnail) {
      const img = document.createElement("img");
      img.src = thumbnails.get(entry.id)!;
      img.alt = "";
      img.draggable = false;
      marker.append(img);
    } else {
      marker.textContent = "[?]";
    }
  } else {
    marker.textContent =
      (entry.paths?.length ?? 0) > 0 && looksLikeDirectoryPath(entry.paths![0])
        ? "▸"
        : "▪";
  }
};

/** Empty-state markup: the localized "nothing here" message and the privacy
 * note underneath. The failure-with-retry case is a separate path in render()
 * because it carries a button and a reload promise. */
const renderEmpty = (): DocumentFragment => {
  const fragment = document.createDocumentFragment();
  const empty = document.createElement("div");
  empty.className = "clipboard-panel__empty";
  empty.textContent = t(
    scopedEntries().length
      ? "clipboard.emptyFilter"
      : view === "favorites"
        ? "clipboard.emptyFavorites"
        : "clipboard.empty",
  );
  fragment.append(empty);

  const privacy = document.createElement("div");
  privacy.className = "clipboard-panel__empty-privacy";
  privacy.textContent = t("settings.clipboardPrivacy");
  fragment.append(privacy);

  return fragment;
};

type InputMode = "pointer" | "keyboard";
let inputMode: InputMode = "pointer";

const setInputMode = (mode: InputMode) => {
  if (inputMode === mode) return;
  inputMode = mode;
  panel.classList.toggle("clipboard-panel--keyboard-mode", mode === "keyboard");
};

// Follow the platform convention used by command palettes: keyboard navigation
// owns the highlight until the pointer actually moves again.
document.addEventListener("pointermove", () => setInputMode("pointer"), {
  capture: true,
  passive: true,
});
document.addEventListener("pointerdown", () => setInputMode("pointer"), true);
document.addEventListener("keydown", () => setInputMode("keyboard"), true);

const focusSelectedRow = () => {
  content.querySelector<HTMLElement>(`[data-row-index="${selected}"]`)?.focus();
};

const syncRowSelection = (index: number) => {
  if (selected === index) return;
  const previous = content.querySelector<HTMLElement>(".clipboard-row--selected");
  previous?.classList.remove("clipboard-row--selected");
  previous?.setAttribute("aria-selected", "false");
  const next = content.querySelector<HTMLElement>(`[data-row-index="${index}"]`);
  next?.classList.add("clipboard-row--selected");
  next?.setAttribute("aria-selected", "true");
  selected = index;
  saveSession();
};

/** Build one list item with a separate favorite control. */
const renderRow = (
  entry: ClipboardEntry,
  index: number,
  selected: number,
  now: number,
): HTMLLIElement => {
  const missing = statuses[entry.id] === false;
  const filesPreview =
    entry.kind === "files" ? formatFilesPreview(entry.paths) : null;
  const hasThumbnail = thumbnails.has(entry.id) && !missing;

  const row = document.createElement("li");
  const button = document.createElement("div");
  button.setAttribute("role", "option");
  button.setAttribute("aria-selected", String(index === selected));
  button.tabIndex = 0;
  button.dataset.rowIndex = String(index);
  button.dataset.rowId = entry.id;
  button.className =
    `clipboard-row${index === selected ? " clipboard-row--selected" : ""}` +
    `${missing ? " clipboard-row--missing" : ""}`;
  if (entry.kind === "files") button.title = (entry.paths ?? []).slice(0, 20).join("\n");

  const marker = document.createElement("span");
  marker.className = [
    "clipboard-row__marker",
    entry.kind === "image" ? "clipboard-row__marker--image" : "",
    entry.kind === "files" ? "clipboard-row__marker--files" : "",
  ]
    .filter(Boolean)
    .join(" ");
  marker.setAttribute("aria-hidden", "true");
  if (entry.kind === "image") {
    renderImageEntry(marker, entry, hasThumbnail);
  } else if (entry.kind === "files") {
    renderFilesEntry(marker, entry, hasThumbnail);
  } else {
    renderHistoryEntry(marker);
  }

  const preview = document.createElement("span");
  preview.className = "clipboard-row__preview";
  if (filesPreview) {
    if (filesPreview.dirname) {
      const dir = document.createElement("span");
      dir.className = "clipboard-row__preview-dir";
      dir.textContent = filesPreview.dirname;
      preview.append(dir);
    }
    const base = document.createElement("span");
    base.textContent = filesPreview.basename;
    preview.append(base);
    if (filesPreview.extra > 0) {
      const extra = document.createElement("span");
      extra.className = "clipboard-row__preview-extra";
      extra.textContent = ` +${filesPreview.extra}`;
      preview.append(extra);
    }
  } else {
    preview.textContent = clipboardPreview(entry);
  }

  const meta = document.createElement("span");
  meta.className = "clipboard-row__meta";
  if (entry.kind === "text" && entry.text) {
    const chars = document.createElement("span");
    chars.className = "clipboard-row__chars";
    chars.textContent = t("clipboard.chars", { n: entry.text.length });
    meta.append(chars);
  }
  const age = document.createElement("span");
  age.className = `clipboard-row__age${missing ? " clipboard-row__age--missing" : ""}`;
  age.textContent = missing
    ? t("clipboard.missing")
    : formatClipboardAge(entry.created_at, now);
  meta.append(age);

  const star = document.createElement("button");
  star.type = "button";
  star.tabIndex = -1;
  star.className = `clipboard-row__star${entry.favorite ? " clipboard-row__star--on" : ""}`;
  star.setAttribute("aria-label", t("clipboard.favorite"));
  star.setAttribute("aria-pressed", String(entry.favorite));
  star.disabled = busy;
  star.title = t("clipboard.favorite");
  star.textContent = entry.favorite ? "★" : "☆";

  button.append(marker, preview, meta, star);
  button.addEventListener("pointerdown", () => {
    // Clicking selects the row before the action runs; hovering alone never
    // changes the keyboard selection.
    setInputMode("pointer");
    syncRowSelection(index);
    button.focus();
  });
  // The CSS :hover state is intentionally visual-only. Keeping pointer
  // movement out of `selected` prevents the mouse from hijacking arrow-key
  // navigation while the user scans the list.
  button.addEventListener("click", () => void activate(entry));
  star.addEventListener("click", (event) => {
    event.stopPropagation();
    void toggleFavorite(entry);
  });

  row.append(button);
  return row;
};

/** Rebuild the whole page's dynamic bits. The filter input is never rebuilt,
 * so its focus and caret survive every render — focus stays pinned there by
 * construction. */
const render = () => {
  const focused = document.activeElement;
  const rowFocused = focused instanceof HTMLElement && Boolean(focused.closest(".clipboard-row"));
  const starFocused = focused instanceof HTMLElement && focused.classList.contains("clipboard-row__star");
  const scrollTop = hydrated ? content.scrollTop : savedSession.scrollTop;
  const finish = () => {
    if (rowFocused) {
      const row = content.querySelector<HTMLElement>(`[data-row-index="${selected}"]`);
      const target = starFocused ? row?.querySelector<HTMLElement>(".clipboard-row__star") : row;
      (target ?? searchInput).focus({ preventScroll: true });
    }
    content.scrollTop = scrollTop;
    saveSession();
  };
  promptLabel.textContent = `${t("clipboard.prompt")}❯`;
  searchInput.placeholder = t("clipboard.filter");
  searchInput.setAttribute("aria-label", t("clipboard.title"));
  filterClear.setAttribute("aria-label", t("clipboard.filterClear"));
  filterClear.title = t("clipboard.filterClear");
  clearButton.textContent = t(clearArmed ? "clipboard.clearConfirm" : "clipboard.clear");
  clearButton.dataset.destructiveConfirm = String(clearArmed);
  clearButton.disabled = busy || !entries.some((entry) => !entry.favorite);
  panel.setAttribute("aria-busy", String(busy));
  clearButton.title = t("clipboard.clearTitle");
  clearButton.setAttribute("aria-label", t("clipboard.clearTitle"));
  hints.textContent = [
    t("clipboard.hintPaste"),
    t("clipboard.hintStar"),
    t("clipboard.hintDelete"),
  ].join(" · ");

  const favoritesCount = entries.reduce((total, entry) => total + (entry.favorite ? 1 : 0), 0);
  tabAll.replaceChildren(document.createTextNode(t("clipboard.tabAll")), countBadge(entries.length));
  tabFavorites.replaceChildren(
    document.createTextNode(t("clipboard.tabFavorites")),
    countBadge(favoritesCount),
  );
  const tabsGroup = root.querySelector<HTMLElement>(".clipboard-panel__tabs")!;
  tabsGroup.setAttribute("aria-label", t("clipboard.title"));
  for (const [tab, active] of [
    [tabAll, view === "all"],
    [tabFavorites, view === "favorites"],
  ] as const) {
    tab.classList.toggle("clipboard-panel__tab--active", active);
    tab.setAttribute("aria-selected", String(active));
  }

  filterClear.hidden = !filterText;

  const filtered = filteredEntries();
  selected = filtered.length ? Math.min(selected, filtered.length - 1) : 0;

  content.replaceChildren();
  if (loadFailed && entries.length === 0) {
    const failure = document.createElement("div");
    failure.className = "clipboard-panel__empty";
    failure.setAttribute("role", "alert");
    const label = document.createElement("span");
    label.textContent = t("plugin.pageError");
    const retry = document.createElement("button");
    retry.type = "button";
    retry.className = "clipboard-panel__clear";
    retry.textContent = t("settings.retry");
    retry.addEventListener("mousedown", (event) => event.preventDefault());
    retry.addEventListener("click", () => {
      retry.disabled = true;
      void reload().then(() => {
        render();
        searchInput.focus();
      });
    });
    failure.append(label, retry);
    content.append(failure);
    finish();
    return;
  }
  if (filtered.length === 0) {
    content.append(renderEmpty());
    finish();
    return;
  }

  const list = document.createElement("ul");
  list.className = "clipboard-panel__list";
  list.setAttribute("role", "listbox");
  list.setAttribute("aria-label", t("clipboard.title"));

  const now = Date.now();
  filtered.forEach((entry, index) => {
    list.append(renderRow(entry, index, selected, now));
  });
  content.append(list);
  finish();
};

// ---- data -----------------------------------------------------------------

const setThumbnail = (id: string, bytes: number[], mime: string) => {
  // Each blob URL pins its bytes in memory until revoked; overwriting an
  // entry's thumbnail (a reload re-fetches every image) would otherwise leak
  // the previous one for the life of the page.
  const previous = thumbnails.get(id);
  if (previous) URL.revokeObjectURL(previous);
  const url = URL.createObjectURL(new Blob([new Uint8Array(bytes)], { type: mime }));
  thumbnails.set(id, url);
  const marker = content.querySelector<HTMLElement>(`[data-row-id="${CSS.escape(id)}"] .clipboard-row__marker`);
  if (marker) {
    const img = document.createElement("img");
    img.src = url;
    img.alt = "";
    img.draggable = false;
    marker.replaceChildren(img);
  }
};

window.addEventListener("pagehide", () => {
  pageVisible = false;
  pageDisposed = true;
  saveSession();
  stopPeriodicRefresh();
  if (clearTimer !== null) window.clearTimeout(clearTimer);
  if (noticeTimer !== null) window.clearTimeout(noticeTimer);
  reloadGen += 1;
  for (const call of pending.values()) {
    window.clearTimeout(call.timer);
    call.reject("Clipboard page closed");
  }
  pending.clear();
  for (const url of thumbnails.values()) URL.revokeObjectURL(url);
  thumbnails.clear();
});

/** Bumped on every reload so a slower earlier pass cannot overwrite the newer
 * one's entries, statuses or thumbnails with stale data. */
let reloadGen = 0;
let reloadPending: Promise<void> | null = null;
const thumbnailPending = new Set<string>();

/**
 * Reload entries, statuses, and thumbnails. Preserves user state: filter text,
 * current tab, scroll position, and selected row (by anchoring to the entry id;
 * if that entry vanished, reset to row 0).
 */
const reloadData = async () => {
  const gen = ++reloadGen;

  try {
    const rows = await invokeCommand<unknown[]>("clipboard_get_entries", { filter: null });
    if (gen !== reloadGen) return;
    const nextEntries = normalizeEntries(rows);
    const entriesChanged = !sameClipboardSnapshot(nextEntries, entries);
    const wasFailed = loadFailed;
    loadFailed = false;
    // Read the selection at completion: the user can move it during a fetch.
    const anchorId = hydrated ? filteredEntries()[selected]?.id : savedSession.selectedId;
    if (entriesChanged) entries = nextEntries;
    if (entriesChanged || !hydrated) {
      const index = filteredEntries().findIndex((entry) => entry.id === anchorId);
      selected = Math.max(0, index);
    }

    // Drop object URLs for records that no longer exist before repainting.
    const liveThumbnailIds = new Set(
      entries
        .filter((entry) => entry.kind === "image" || isFilesPreviewCandidate(entry.paths))
        .map((entry) => entry.id),
    );
    for (const [id, url] of thumbnails) {
      if (!liveThumbnailIds.has(id)) {
        URL.revokeObjectURL(url);
        thumbnails.delete(id);
      }
    }
    if (entriesChanged || wasFailed || !hydrated) render();
    hydrated = true;
    saveSession();
  } catch {
    if (gen !== reloadGen) return;
    loadFailed = true;
    if (entries.length) showError();
    else render();
    return;
  }

  // File statuses: batch fetch for all file entries.
  const fileIds = entries
    .filter((entry) => entry.kind === "files")
    .map((entry) => entry.id);
  if (fileIds.length) {
    invokeCommand<Record<string, boolean>>("clipboard_entry_statuses", {
      ids: fileIds,
    })
      .then((nextStatuses) => {
        if (gen !== reloadGen) return;
        const changed = Object.keys(statuses).length !== Object.keys(nextStatuses).length
          || Object.entries(nextStatuses).some(([id, value]) => statuses[id] !== value);
        statuses = nextStatuses;
        if (changed) render();
      })
      .catch(() => undefined);
  }

  // Bound parallel binary transfers. Failed optional previews retry next poll,
  // including when the entry metadata has not changed.
  const queue = entries.filter((entry) => (entry.kind === "image" || isFilesPreviewCandidate(entry.paths))
    && !thumbnails.has(entry.id) && !thumbnailPending.has(entry.id));
  const worker = async () => {
    for (let entry = queue.shift(); entry && gen === reloadGen; entry = queue.shift()) {
      thumbnailPending.add(entry.id);
      try {
        const bytes = await invokeCommand<number[]>(entry.kind === "image" ? "clipboard_read_image" : "clipboard_read_file_preview", { id: entry.id });
        if (gen === reloadGen && entries.some((current) => current.id === entry.id)) {
          setThumbnail(entry.id, bytes, entry.kind === "image" ? "image/png" : imageFileMime(entry.paths![0]));
        }
      } catch { /* Optional preview; the next refresh retries it. */ }
      finally { thumbnailPending.delete(entry.id); }
    }
  };
  const slots = Math.max(0, 4 - thumbnailPending.size);
  for (let index = 0; index < slots; index += 1) void worker();
};

const reload = (): Promise<void> => {
  if (pageDisposed || !pageVisible) return Promise.resolve();
  if (reloadPending) return reloadPending;
  const request = reloadData().finally(() => { reloadPending = null; });
  reloadPending = request;
  return request;
};

/**
 * Start periodic refresh: poll every 2s while visible. 2s is fresh enough given
 * the backend monitor polls the system clipboard every ~900ms, so the page lags
 * behind the monitor by at most one poll cycle.
 */
const startPeriodicRefresh = () => {
  if (refreshInterval !== null || !pageVisible || pageDisposed) return;
  refreshInterval = window.setInterval(() => {
    if (!busy) void reload();
  }, 2000);
};

/** Stop periodic refresh when the page is hidden. */
const stopPeriodicRefresh = () => {
  if (refreshInterval !== null) {
    window.clearInterval(refreshInterval);
    refreshInterval = null;
  }
};

/**
 * Handle the reload message from the host: page just became visible. Refresh
 * data immediately, retain session state, and start the periodic refresh.
 */
const handleReload = async () => {
  if (pageDisposed) return;
  const wasHidden = !pageVisible;
  pageVisible = true;
  if (wasHidden) await reloadPending;
  if (!pageVisible || pageDisposed) return;
  await reload();
  if (!pageVisible || pageDisposed) return;
  searchInput.focus({ preventScroll: true });
  startPeriodicRefresh();
};

/** Handle the page becoming hidden: stop the periodic refresh. */
const handleHidden = () => {
  pageVisible = false;
  reloadGen += 1;
  saveSession();
  disarmClear();
  stopPeriodicRefresh();
};

const activate = async (entry: ClipboardEntry | undefined) => {
  if (!entry || busy || statuses[entry.id] === false) return;
  busy = true;
  render();
  try {
    await invokeCommand<void>("clipboard_copy_entry", { id: entry.id });
  } catch {
    // The clipboard may be held by another app; keep the page open so the
    // user can retry instead of silently losing the action.
    showError();
    return;
  } finally {
    busy = false;
    render();
  }
  if (pageVisible && !pageDisposed) requestClose();
};

const toggleFavorite = async (entry: ClipboardEntry | undefined) => {
  if (!entry || busy) return;
  busy = true;
  reloadGen += 1;
  const nextFavorite = !entry.favorite;
  render();
  searchInput.focus();
  try {
    await invokeCommand<void>("clipboard_set_favorite", {
      id: entry.id,
      favorite: nextFavorite,
    });
    entries = entries.map((current) => current.id === entry.id ? { ...current, favorite: nextFavorite } : current);
  } catch {
    showError();
  } finally {
    await reloadPending;
    await reload();
    busy = false;
    render();
  }
};

const removeEntry = async (entry: ClipboardEntry | undefined) => {
  if (!entry || busy) return;
  busy = true;
  reloadGen += 1;
  render();
  searchInput.focus();
  try {
    await invokeCommand<void>("clipboard_delete", { id: entry.id });
    entries = entries.filter((candidate) => candidate.id !== entry.id);
  } catch {
    showError();
  } finally {
    await reloadPending;
    await reload();
    busy = false;
    render();
  }
};

const clearHistory = async () => {
  if (busy) return;
  if (!clearArmed) {
    clearArmed = true;
    clearTimer = window.setTimeout(disarmClear, 3000);
    render();
    return;
  }
  disarmClear();
  busy = true;
  reloadGen += 1;
  render();
  try {
    await invokeCommand<void>("clipboard_clear_history");
    entries = entries.filter((entry) => entry.favorite);
  } catch {
    showError();
  } finally {
    selected = 0;
    await reloadPending;
    await reload();
    busy = false;
    render();
    searchInput.focus();
  }
};

const disarmClear = () => {
  clearArmed = false;
  if (clearTimer !== null) window.clearTimeout(clearTimer);
  clearTimer = null;
  render();
};

// ---- keyboard model -------------------------------------------------------

/** Route one character into the filter: append, redraw, own the field. */
const sendCharToFilter = (char: string) => {
  filterText = (filterText + char).slice(0, 512);
  searchInput.value = filterText;
  selected = 0;
  render();
  searchInput.focus();
};

/** Delete the filter's last character and take focus — what Backspace means
 * whenever the input itself is not focused, so an edit key can never reach a
 * row through accident. */
const backspaceIntoFilter = () => {
  filterText = filterText.slice(0, -1);
  searchInput.value = filterText;
  selected = 0;
  render();
  searchInput.focus();
};

window.addEventListener("keydown", (event) => {
  if (event.isComposing || event.keyCode === 229 || composing) return;
  const focusedControl = document.activeElement instanceof HTMLButtonElement;
  if (event.key === "Escape" && clearArmed) {
    event.preventDefault();
    event.stopPropagation();
    disarmClear();
    searchInput.focus();
    return;
  }
  if (event.repeat && ["Enter", "Delete", "Backspace", "d", "D", "f", "F", "*"].includes(event.key) && (document.activeElement !== searchInput || event.ctrlKey || event.metaKey || event.key === "Enter")) {
    event.preventDefault();
    return;
  }
  if (focusedControl && (event.key === "Enter" || event.key === " ")) {
    if (event.key === "Enter" && document.activeElement === clearButton) event.preventDefault();
    return;
  }
  // Capture phase: this handler decides before anything (default traversal
  // included) can act on the press.

  // Cmd+W (macOS) / Ctrl+W (other platforms) dismisses the page — the same
  // convention every overlay surface in floter follows.
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
    event.preventDefault();
    event.stopPropagation();
    requestClose();
    return;
  }
  // Cmd/Ctrl+Backspace deletes the selected row from ANY focus — the explicit,
  // collision-free escape hatch that survives even while the filter is held.
  if ((event.metaKey || event.ctrlKey) && event.key === "Backspace") {
    event.preventDefault();
    event.stopPropagation();
    void removeEntry(filteredEntries()[selected]);
    searchInput.focus();
    return;
  }
  if (event.key === "Escape") {
    event.preventDefault();
    event.stopPropagation();
    requestClose();
    return;
  }
  if (event.key === "Tab") {
    // Tab TOGGLES the view rather than walking focus away from the input —
    // intercepted before default focus traversal from any focus, filter
    // included.
    event.preventDefault();
    event.stopPropagation();
    view = view === "all" ? "favorites" : "all";
    selected = 0;
    render();
    searchInput.focus();
    return;
  }
  if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
    // Tabs also switch directly — but only while the list owns attention; the
    // same keys are ordinary caret moves inside the filter.
    if (document.activeElement === searchInput) return;
    event.preventDefault();
    event.stopPropagation();
    view = view === "all" ? "favorites" : "all";
    selected = 0;
    render();
    focusSelectedRow();
    return;
  }
  if (event.key === "ArrowDown") {
    event.preventDefault();
    event.stopPropagation();
    const length = filteredEntries().length;
    selected = length ? (selected + 1) % length : 0;
    render();
    focusSelectedRow();
    return;
  }
  if (event.key === "ArrowUp") {
    event.preventDefault();
    event.stopPropagation();
    const length = filteredEntries().length;
    selected = length ? (selected - 1 + length) % length : 0;
    render();
    focusSelectedRow();
    return;
  }
  if (shouldActivateClipboardEntry(event, document.activeElement === searchInput ? "search" : "row")) {
    event.preventDefault();
    event.stopPropagation();
    void activate(filteredEntries()[selected]);
    return;
  }

  if (document.activeElement === searchInput) {
    // While the filter holds focus every remaining key — characters,
    // Backspace, Delete — is ordinary text editing inside it. Row commands
    // are unreachable here; press-wise the two worlds cannot collide.
    return;
  }
  if (event.metaKey || event.ctrlKey) {
    // Platform copy/paste/cut shortcuts keep working.
    return;
  }
  if (event.altKey && event.key.length === 1) {
    // Alt+letter combinations belong to global shortcuts; ignore.
    return;
  }

  // List-focus territory: the user deliberately stepped off the filter
  // (clicked into the list). Single-key row commands live here, and focus
  // returns to the filter after each one.
  if (event.key === "f" || event.key === "F" || event.key === "*") {
    event.preventDefault();
    event.stopPropagation();
    void toggleFavorite(filteredEntries()[selected]);
    searchInput.focus();
    return;
  }
  if (event.key === "d" || event.key === "D" || event.key === "Delete") {
    event.preventDefault();
    event.stopPropagation();
    void removeEntry(filteredEntries()[selected]);
    searchInput.focus();
    return;
  }
  if (event.key === "1" || event.key === "2") {
    event.preventDefault();
    event.stopPropagation();
    view = event.key === "1" ? "all" : "favorites";
    selected = 0;
    render();
    focusSelectedRow();
    return;
  }
  if (event.key === "Backspace") {
    // Editing keys route into the filter, never to a row: Backspace erases
    // the filter's last character and takes focus back.
    event.preventDefault();
    event.stopPropagation();
    backspaceIntoFilter();
    return;
  }
  // Any other printable character typed anywhere lands in the filter —
  // inserted by hand because the field was not focused when the press
  // happened, and nothing else would insert it.
  if (event.key.length === 1) {
    event.preventDefault();
    event.stopPropagation();
    sendCharToFilter(event.key);
  }
}, { capture: true });

// ---- wiring ---------------------------------------------------------------

let composing = false;
searchInput.addEventListener("compositionstart", () => { composing = true; });
searchInput.addEventListener("compositionend", () => { composing = false; });
searchInput.addEventListener("input", () => {
  if (clearArmed) disarmClear();
  filterText = searchInput.value;
  selected = 0;
  render();
});
filterClear.addEventListener("mousedown", (event) => event.preventDefault());
filterClear.addEventListener("click", () => {
  filterText = "";
  searchInput.value = "";
  selected = 0;
  searchInput.focus();
  render();
});
for (const [tab, next] of [
  [tabAll, "all"],
  [tabFavorites, "favorites"],
] as const) {
  tab.addEventListener("mousedown", (event) => event.preventDefault());
  tab.addEventListener("click", () => {
    view = next;
    selected = 0;
    render();
    searchInput.focus();
  });
}
clearButton.addEventListener("mousedown", (event) => event.preventDefault());
clearButton.addEventListener("click", () => void clearHistory());
content.addEventListener("scroll", saveSession, { passive: true });

window.addEventListener("paste", (event) => {
  if (document.activeElement === searchInput) return;
  const text = event.clipboardData?.getData("text");
  if (!text) return;
  event.preventDefault();
  filterText = (filterText + text).slice(0, 512);
  searchInput.value = filterText;
  selected = 0;
  render();
  searchInput.focus();
});

// Focus the filter on load — the page owns the keyboard from the first frame.
searchInput.focus();

// Render immediately so the page is never blank (falls back to page.css
// styles even if the @import chain is slow), then hydrate with data.
render();
void reload().then(() => {
  render();
  // Start periodic refresh after initial load.
  startPeriodicRefresh();
}).catch(() => {
  // Only mark failed on a real bridge error, not on empty results.
  if (entries.length === 0) {
    loadFailed = true;
    render();
  }
});

// Stop periodic refresh when the page is about to be hidden (not unloaded,
// since the iframe persists across toggles).
document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    handleHidden();
  } else {
    void handleReload();
  }
});
