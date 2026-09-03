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
  type ClipboardEntry,
} from "../../clipboard-history";
import { BRIDGE_TAG, isBridgeOpacity, isBridgeTheme, isBridgeResult } from "../../plugin-pages";

// ---- bridge client -------------------------------------------------------

type PendingCall = {
  resolve: (value: unknown) => void;
  reject: (error: string) => void;
  timer: number;
};

const pending = new Map<number, PendingCall>();
let nextCallId = 1;

/** How long to wait for the host's reply before giving up on a call. Without
 * this a dropped or ignored message leaves the promise pending forever, and
 * the awaiting UI (a reload, a favorite toggle) hangs with no way back. */
const BRIDGE_TIMEOUT_MS = 10_000;

window.addEventListener("message", (event: MessageEvent) => {
  // Only the host window may talk to us.
  if (event.source !== window.parent) return;
  const data: unknown = event.data;
  if (isBridgeOpacity(data)) {
    // Opacity sliders moved host-side; restyle in place.
    applyOpacity(data.mainOpacity, data.terminalOpacity);
    return;
  }
  if (isBridgeTheme(data)) {
    // Theme changed host-side; update the page's data-theme attribute.
    document.documentElement.setAttribute("data-theme", data.theme);
    return;
  }
  if (!isBridgeResult(data)) return;
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
    window.parent.postMessage({ [BRIDGE_TAG]: "invoke", id, command, args: args ?? {} }, "*");
  });

const requestClose = () => {
  window.parent.postMessage({ [BRIDGE_TAG]: "close" }, "*");
};

// ---- bootstrap ------------------------------------------------------------

const params = new URLSearchParams(window.location.search);
const t: Translate = createTranslator(normalizeLanguage(params.get("lang") ?? "en"));
const theme = params.get("theme") === "light" ? "light" : "dark";
const rootStyle = document.documentElement.style;

function applyOpacity(main: number, terminal: number) {
  rootStyle.setProperty("--main-opacity", String(main));
  rootStyle.setProperty("--terminal-opacity", String(terminal));
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

let entries: ClipboardEntry[] = [];
let filterText = "";
let view: ClipboardView = "all";
let selected = 0;
const thumbnails = new Map<string, string>();
let statuses: Record<string, boolean> = {};
/** True when the last entry fetch failed or timed out — an empty list then
 * means "we could not ask", not "nothing copied yet", so the page offers a
 * retry instead of a misleading empty state. */
let loadFailed = false;

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
      <input class="clipboard-panel__search" spellcheck="false" autocapitalize="off" autocorrect="off" />
      <button type="button" class="clipboard-panel__filter-clear" hidden>×</button>
      <div class="clipboard-panel__tabs" role="tablist">
        <button type="button" role="tab" class="clipboard-panel__tab" data-view="all"></button>
        <button type="button" role="tab" class="clipboard-panel__tab" data-view="favorites"></button>
      </div>
    </div>
    <div class="clipboard-panel__content"></div>
    <div class="clipboard-panel__footer">
      <span class="clipboard-panel__hints"></span>
      <button type="button" class="clipboard-panel__clear"></button>
    </div>
  </div>
`;

const promptLabel = root.querySelector<HTMLElement>(".clipboard-panel__prompt")!;
const searchInput = root.querySelector<HTMLInputElement>(".clipboard-panel__search")!;
const filterClear = root.querySelector<HTMLButtonElement>(".clipboard-panel__filter-clear")!;
const tabAll = root.querySelector<HTMLButtonElement>('[data-view="all"]')!;
const tabFavorites = root.querySelector<HTMLButtonElement>('[data-view="favorites"]')!;
const content = root.querySelector<HTMLElement>(".clipboard-panel__content")!;
const hints = root.querySelector<HTMLElement>(".clipboard-panel__hints")!;
const clearButton = root.querySelector<HTMLButtonElement>(".clipboard-panel__clear")!;

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

/** Build one row's <li>: a button with marker, preview, meta, and a star,
 * plus the hover/click wiring that makes it a real list option. `now` is
 * passed in so every row's age is relative to the same render moment, not to
 * the millisecond this row happened to be built. */
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
  const button = document.createElement("button");
  button.type = "button";
  button.setAttribute("role", "option");
  button.setAttribute("aria-selected", String(index === selected));
  button.tabIndex = -1;
  button.className =
    `clipboard-row${index === selected ? " clipboard-row--selected" : ""}` +
    `${missing ? " clipboard-row--missing" : ""}`;
  if (entry.kind === "files") button.title = (entry.paths ?? []).join("\n");

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
  star.title = t("clipboard.favorite");
  star.textContent = entry.favorite ? "★" : "☆";

  button.append(marker, preview, meta, star);
  button.addEventListener("mousemove", () => {
    if (selected === index) return;
    selected = index;
    render();
  });
  // No mousedown hijack: a click genuinely moves focus into the list — that
  // deliberate step away from the filter is what arms the single-key row
  // commands. Hovering alone never steals focus.
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
  promptLabel.textContent = `${t("clipboard.prompt")}❯`;
  searchInput.placeholder = t("clipboard.filter");
  searchInput.setAttribute("aria-label", t("clipboard.title"));
  filterClear.setAttribute("aria-label", t("clipboard.filterClear"));
  filterClear.title = t("clipboard.filterClear");
  clearButton.textContent = t("clipboard.clear");
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
      void reload().then(() => {
        render();
        searchInput.focus();
      });
    });
    failure.append(label, retry);
    content.append(failure);
    return;
  }
  if (filtered.length === 0) {
    content.append(renderEmpty());
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
  render();
};

window.addEventListener("pagehide", () => {
  for (const url of thumbnails.values()) URL.revokeObjectURL(url);
  thumbnails.clear();
});

/** Bumped on every reload so a slower earlier pass cannot overwrite the newer
 * one's entries, statuses or thumbnails with stale data. */
let reloadGen = 0;

const reload = async () => {
  const gen = ++reloadGen;
  try {
    const rows = await invokeCommand<unknown[]>("clipboard_get_entries", { filter: null });
    if (gen !== reloadGen) return;
    entries = normalizeEntries(rows);
    loadFailed = false;
  } catch {
    if (gen !== reloadGen) return;
    entries = [];
    loadFailed = true;
    return;
  }

  // Render immediately with entry data, before fetching thumbnails or statuses.
  // First paint shows the text content; images populate progressively.
  render();

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
        statuses = nextStatuses;
        render();
      })
      .catch(() => undefined);
  }

  // Thumbnails: fetched progressively after the list is already visible.
  // Each thumbnail arrival triggers a single-row repaint through setThumbnail.
  for (const entry of entries.filter((candidate) => candidate.kind === "image")) {
    invokeCommand<number[]>("clipboard_read_image", { id: entry.id })
      .then((bytes) => {
        if (gen !== reloadGen) return;
        setThumbnail(entry.id, bytes, "image/png");
      })
      .catch(() => undefined);
  }
  for (const entry of entries.filter(
    (candidate) => candidate.kind === "files" && isFilesPreviewCandidate(candidate.paths),
  )) {
    const mime = imageFileMime(entry.paths![0]);
    invokeCommand<number[]>("clipboard_read_file_preview", { id: entry.id })
      .then((bytes) => {
        if (gen !== reloadGen) return;
        setThumbnail(entry.id, bytes, mime);
      })
      .catch(() => undefined);
  }
};

const activate = async (entry: ClipboardEntry | undefined) => {
  if (!entry || statuses[entry.id] === false) return;
  try {
    await invokeCommand<void>("clipboard_copy_entry", { id: entry.id });
  } catch {
    // The clipboard may be held by another app; keep the page open so the
    // user can retry instead of silently losing the action.
    return;
  }
  requestClose();
};

const toggleFavorite = async (entry: ClipboardEntry | undefined) => {
  if (!entry) return;
  const nextFavorite = !entry.favorite;
  entry.favorite = nextFavorite;
  render();
  searchInput.focus();
  try {
    await invokeCommand<void>("clipboard_set_favorite", {
      id: entry.id,
      favorite: nextFavorite,
    });
  } catch {
    await reload().then(render);
    searchInput.focus();
  }
};

const removeEntry = async (entry: ClipboardEntry | undefined) => {
  if (!entry) return;
  entries = entries.filter((candidate) => candidate.id !== entry.id);
  render();
  searchInput.focus();
  try {
    await invokeCommand<void>("clipboard_delete", { id: entry.id });
  } catch {
    await reload().then(render);
    searchInput.focus();
  }
};

const clearHistory = async () => {
  try {
    await invokeCommand<void>("clipboard_clear_history");
  } catch {
    // A failed clear leaves the history as it was; the reload shows the truth.
  }
  selected = 0;
  await reload().then(render);
  searchInput.focus();
};

// ---- keyboard model -------------------------------------------------------

/** Route one character into the filter: append, redraw, own the field. */
const sendCharToFilter = (char: string) => {
  filterText += char;
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
    return;
  }
  if (event.key === "ArrowDown") {
    event.preventDefault();
    event.stopPropagation();
    const length = filteredEntries().length;
    selected = length ? (selected + 1) % length : 0;
    render();
    return;
  }
  if (event.key === "ArrowUp") {
    event.preventDefault();
    event.stopPropagation();
    const length = filteredEntries().length;
    selected = length ? (selected - 1 + length) % length : 0;
    render();
    return;
  }
  if (event.key === "Enter") {
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

searchInput.addEventListener("input", () => {
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

// Focus the filter on load — the page owns the keyboard from the first frame.
searchInput.focus();

// Render immediately so the page is never blank (falls back to page.css
// styles even if the @import chain is slow), then hydrate with data.
render();
void reload().then(render).catch(() => {
  // Only mark failed on a real bridge error, not on empty results.
  if (entries.length === 0) {
    loadFailed = true;
    render();
  }
});
