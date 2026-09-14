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
/** True when the fetch failed because the clipboard backend refused the call
 * (feature off, unmanaged state, command unavailable) rather than a transient
 * network/bridge hiccup. A retry cannot fix that, so the page explains how to
 * turn the feature on instead of offering a useless "Retry". Kept monotonic:
 * once the backend is known to be unavailable, only a successful fetch clears
 * it again. */
let backendUnavailable = false;
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

/** Identity of everything a row paints. A row whose key is unchanged is left
 * alone by [`reconcileList`] — its DOM node is reused verbatim — which is what
 * keeps a 200+ row list from being torn down and rebuilt on every keystroke,
 * selection move or favorite toggle. Selection and list position are in the
 * key, so the two rows whose highlight moves repaint while the rest sit still.
 *
 * The rendered age string — not merely `created_at` — is in the key, so a row
 * whose visible "x m" ticked over repaints on the next 2s poll instead of
 * freezing at its first-painted value. `now` is the same value
 * [`applyRowState`] paints from, so the key and the paint can never disagree
 * within one pass; rows whose age has not changed keep a zero-DOM-write pass. */
const rowPaintKey = (
  entry: ClipboardEntry,
  index: number,
  selected: number,
  missing: boolean,
  hasThumbnail: boolean,
  now: number,
): string =>
  [
    entry.id,
    entry.kind,
    entry.hash,
    entry.created_at,
    entry.favorite ? 1 : 0,
    missing ? 1 : 0,
    hasThumbnail ? 1 : 0,
    index,
    index === selected ? 1 : 0,
    missing ? t("clipboard.missing") : formatClipboardAge(entry.created_at, now),
  ].join("\u0001");

/** Per-row record of the inputs each node was last painted from, so an
 * unchanged row can skip all DOM work. Weak so discarded rows collect. */
const rowPaintKeys = new WeakMap<HTMLLIElement, string>();
/** The `busy` value the stars were last synced to. A busy flip only toggles
 * `disabled` on every star, so it is kept out of the row paint key and synced
 * with one cheap pass instead of repainting 200+ rows of content. */
let renderedBusy: boolean | null = null;

/** Repaint a row's data-driven bits (icon, preview, meta, selection, favorite,
 * index) in place from its entry, without creating or replacing the row or its
 * button node. Used both when a row is created and when an existing one is
 * patched. */
const applyRowState = (
  row: HTMLLIElement,
  entry: ClipboardEntry,
  index: number,
  selected: number,
  now: number,
) => {
  const missing = statuses[entry.id] === false;
  const hasThumbnail = thumbnails.has(entry.id) && !missing;
  const button = row.firstElementChild as HTMLElement;
  const isSelected = index === selected;
  const classes =
    `clipboard-row${isSelected ? " clipboard-row--selected" : ""}` +
    `${missing ? " clipboard-row--missing" : ""}`;
  // Guard every write: a row reindexed by an insert/removal above it has the
  // same class/attrs as before, and re-writing identical values would still
  // dirty the node and force layout for no visible change.
  if (button.dataset.rowIndex !== String(index)) button.dataset.rowIndex = String(index);
  if (button.getAttribute("aria-selected") !== String(isSelected)) button.setAttribute("aria-selected", String(isSelected));
  if (button.className !== classes) button.className = classes;
  if (entry.kind === "files") {
    const title = (entry.paths ?? []).slice(0, 20).join("\n");
    if (button.title !== title) button.title = title;
  } else if (button.hasAttribute("title")) {
    button.removeAttribute("title");
  }

  const marker = button.children.item(0) as HTMLElement;
  const markerClasses = [
    "clipboard-row__marker",
    entry.kind === "image" ? "clipboard-row__marker--image" : "",
    entry.kind === "files" ? "clipboard-row__marker--files" : "",
  ]
    .filter(Boolean)
    .join(" ");
  if (marker.className !== markerClasses) marker.className = markerClasses;
  const thumbnailUrl = thumbnails.get(entry.id);
  const markerImage = marker.querySelector("img");
  if (entry.kind === "image" || entry.kind === "files") {
    // Rebuild the marker only when it is not already showing exactly the right
    // pixels; a plain `[?]` glyph and a stale/duplicate img both get replaced.
    if (!hasThumbnail || !markerImage || markerImage.getAttribute("src") !== thumbnailUrl) {
      marker.replaceChildren();
      if (entry.kind === "image") renderImageEntry(marker, entry, hasThumbnail);
      else renderFilesEntry(marker, entry, hasThumbnail);
    }
  } else if (markerImage || marker.textContent !== "›") {
    renderHistoryEntry(marker);
  }

  const filesPreview =
    entry.kind === "files" ? formatFilesPreview(entry.paths) : null;
  const preview = button.children.item(1) as HTMLElement;
  if (filesPreview) {
    const signature = `${filesPreview.dirname}\u0001${filesPreview.basename}\u0001${filesPreview.extra}`;
    if (preview.dataset.previewSig !== signature) {
      preview.replaceChildren();
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
      preview.dataset.previewSig = signature;
    }
  } else {
    delete preview.dataset.previewSig;
    const text = clipboardPreview(entry);
    if (preview.textContent !== text) preview.textContent = text;
  }

  const meta = button.children.item(2) as HTMLElement;
  const chars = meta.querySelector<HTMLElement>(".clipboard-row__chars");
  if (entry.kind === "text" && entry.text) {
    const label = t("clipboard.chars", { n: entry.text.length });
    if (chars) {
      if (chars.textContent !== label) chars.textContent = label;
    } else {
      const next = document.createElement("span");
      next.className = "clipboard-row__chars";
      next.textContent = label;
      meta.prepend(next);
    }
  } else if (chars) {
    chars.remove();
  }
  const age = meta.querySelector<HTMLElement>(".clipboard-row__age")!;
  const ageClasses = `clipboard-row__age${missing ? " clipboard-row__age--missing" : ""}`;
  if (age.className !== ageClasses) age.className = ageClasses;
  const ageText = missing ? t("clipboard.missing") : formatClipboardAge(entry.created_at, now);
  if (age.textContent !== ageText) age.textContent = ageText;

  const star = button.children.item(3) as HTMLButtonElement;
  const starClasses = `clipboard-row__star${entry.favorite ? " clipboard-row__star--on" : ""}`;
  if (star.className !== starClasses) star.className = starClasses;
  const pressed = String(entry.favorite);
  if (star.getAttribute("aria-pressed") !== pressed) star.setAttribute("aria-pressed", pressed);
  if (star.disabled !== busy) star.disabled = busy;
  const starText = entry.favorite ? "★" : "☆";
  if (star.textContent !== starText) star.textContent = starText;
  if (star.title !== t("clipboard.favorite")) star.title = t("clipboard.favorite");

  rowPaintKeys.set(row, rowPaintKey(entry, index, selected, missing, hasThumbnail, now));
};

/** Build one list item with a separate favorite control. The skeleton is made
 * once here; [`applyRowState`] fills it, so the create and patch paths stay
 * identical. Click/star handlers resolve the live entry by `data-row-id` at
 * event time, so a reused node never acts on a stale object after a reload. */
const renderRow = (
  entry: ClipboardEntry,
  index: number,
  selected: number,
  now: number,
): HTMLLIElement => {
  const row = document.createElement("li");
  const button = document.createElement("div");
  button.setAttribute("role", "option");
  button.tabIndex = 0;
  button.dataset.rowId = entry.id;
  button.className = "clipboard-row";

  const marker = document.createElement("span");
  marker.className = "clipboard-row__marker";
  marker.setAttribute("aria-hidden", "true");

  const preview = document.createElement("span");
  preview.className = "clipboard-row__preview";

  const meta = document.createElement("span");
  meta.className = "clipboard-row__meta";
  const age = document.createElement("span");
  age.className = "clipboard-row__age";
  meta.append(age);

  const star = document.createElement("button");
  star.type = "button";
  star.tabIndex = -1;
  star.className = "clipboard-row__star";
  star.setAttribute("aria-label", t("clipboard.favorite"));

  button.append(marker, preview, meta, star);
  button.addEventListener("pointerdown", () => {
    // Clicking selects the row before the action runs; hovering alone never
    // changes the keyboard selection.
    setInputMode("pointer");
    syncRowSelection(Number(button.dataset.rowIndex ?? "0"));
    button.focus();
  });
  // The CSS :hover state is intentionally visual-only. Keeping pointer
  // movement out of `selected` prevents the mouse from hijacking arrow-key
  // navigation while the user scans the list.
  button.addEventListener("click", () => {
    const live = entries.find((current) => current.id === button.dataset.rowId);
    void activate(live);
  });
  star.addEventListener("click", (event) => {
    event.stopPropagation();
    const live = entries.find((current) => current.id === button.dataset.rowId);
    void toggleFavorite(live);
  });

  row.append(button);
  applyRowState(row, entry, index, selected, now);
  return row;
};

/** The list element exists only while there is at least one row to show; the
 * empty / loading / failure states fill `content` instead. */
const currentList = (): HTMLElement | null =>
  content.querySelector<HTMLElement>(".clipboard-panel__list");

/** Reconcile `content` with `filtered`, reusing the DOM of every row whose
 * paint key is unchanged. Rows are keyed on `data-id`: existing nodes are
 * patched and glued back into order, new ones are built once, and anything the
 * filter dropped is removed. No `replaceChildren()` on the list means the
 * browser keeps the layout of the hundreds of rows that did not change. */
const reconcileList = (filtered: ClipboardEntry[], now: number) => {
  if (loadFailed && entries.length === 0) {
    const failure = document.createElement("div");
    failure.className = "clipboard-panel__empty";
    failure.setAttribute("role", "alert");
    const label = document.createElement("span");
    label.textContent = t(backendUnavailable ? "clipboard.pageUnavailable" : "clipboard.loadFailed");
    failure.append(label);
    if (backendUnavailable) {
      // Retrying cannot help when the backend is off/unmanaged; say how to
      // turn it on instead of offering a dead-end button.
      const hint = document.createElement("span");
      hint.className = "clipboard-panel__empty-hint";
      hint.textContent = t("clipboard.pageUnavailableHint");
      failure.append(hint);
    } else {
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
      failure.append(retry);
    }
    content.replaceChildren(failure);
    return;
  }

  if (filtered.length === 0) {
    content.replaceChildren(renderEmpty());
    return;
  }

  let list = currentList();
  if (!list) {
    list = document.createElement("ul");
    list.className = "clipboard-panel__list";
    list.setAttribute("role", "listbox");
    list.setAttribute("aria-label", t("clipboard.title"));
    content.replaceChildren(list);
  }

  const byId = new Map<string, HTMLLIElement>();
  for (const child of list.children) {
    const element = child as HTMLLIElement;
    const id = element.firstElementChild instanceof HTMLElement
      ? element.firstElementChild.dataset.rowId
      : undefined;
    if (id) byId.set(id, element);
  }

  const seen = new Set<string>();
  const desired: HTMLLIElement[] = [];
  filtered.forEach((entry, index) => {
    if (seen.has(entry.id)) return;
    seen.add(entry.id);
    const missing = statuses[entry.id] === false;
    const hasThumbnail = thumbnails.has(entry.id) && !missing;
    let row = byId.get(entry.id);
    if (!row) {
      row = renderRow(entry, index, selected, now);
      byId.set(entry.id, row);
    } else if (
      rowPaintKeys.get(row)
      !== rowPaintKey(entry, index, selected, missing, hasThumbnail, now)
    ) {
      applyRowState(row, entry, index, selected, now);
    }
    desired.push(row);
  });

  // Place only the nodes that are out of position: a leading run already in
  // order is left untouched, and each later row is inserted before the next
  // in-order node. Rows already in order keep their DOM slots even when a new
  // entry shifted their index on paper, so a reload that prepends one capture
  // touches one node instead of the whole list.
  let cursor: ChildNode | null = list.firstChild;
  for (const row of desired) {
    if (row === cursor) {
      cursor = cursor.nextSibling;
      continue;
    }
    list.insertBefore(row, cursor);
  }
  for (const [id, row] of byId) {
    if (!seen.has(id)) row.remove();
  }
};

/** Rebuild the whole page's dynamic bits. The filter input is never rebuilt,
 * so its focus and caret survive every render — focus stays pinned there by
 * construction. The list is reconciled in place (see [`reconcileList`]), so
 * unchanged rows keep their DOM nodes. */
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

  // One `now` for the whole pass: the paint key's age comparison and every
  // row's age text read the same clock, so they cannot disagree.
  const now = Date.now();
  reconcileList(filtered, now);
  // `busy` only disables the per-row star buttons; sync them in one pass when
  // it flips rather than folding it into every row's paint key.
  if (renderedBusy !== busy) {
    renderedBusy = busy;
    for (const star of content.querySelectorAll<HTMLButtonElement>(".clipboard-row__star")) {
      star.disabled = busy;
    }
  }
  // Arm the viewport watcher for any candidate row this pass introduced; the
  // observer queues a reload the moment one scrolls into range.
  observeThumbnailCandidates();
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
  // Repaint only this row's marker through the shared row patcher, so the
  // node and the paint-key cache stay truthful as the bytes arrive.
  const row = content.querySelector<HTMLElement>(`[data-row-id="${CSS.escape(id)}"]`)?.parentElement as HTMLLIElement | null;
  const entry = entries.find((current) => current.id === id);
  if (!row || !entry) return;
  const index = filteredEntries().findIndex((current) => current.id === id);
  if (index >= 0) applyRowState(row, entry, index, selected, Date.now());
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
  // Forget the busy-sync state: a hidden-then-reopened page (the iframe is
  // kept alive, so this page is not torn down) must re-sync every star from
  // scratch rather than rely on the fallback pass below.
  renderedBusy = null;
  thumbnailObserver.disconnect();
});

/** Bumped on every reload so a slower earlier pass cannot overwrite the newer
 * one's entries, statuses or thumbnails with stale data. */
let reloadGen = 0;
let reloadPending: Promise<void> | null = null;
const thumbnailPending = new Set<string>();
/** Entry ids whose candidate rows have actually been scrolled into view. Only
 * these (plus their warmed neighbors below) request bytes, so a long history
 * of images doesn't batch-decode its whole first screen. */
const thumbnailVisible = new Set<string>();

/** Whether an entry can render pixels in its marker at all. */
const isThumbnailCandidate = (entry: ClipboardEntry): boolean =>
  entry.kind === "image" || isFilesPreviewCandidate(entry.paths);

/** A row is measured against the scroll container once it has a node, and
 * queued the moment it intersects. There is one observer for the whole list;
 * re-observing an already-observed node is a no-op, so this stays cheap on
 * every reconcile. */
const thumbnailObserver = new IntersectionObserver(
  (records) => {
    let queued = false;
    for (const record of records) {
      if (!record.isIntersecting) continue;
      const id = (record.target as HTMLElement).dataset.rowId;
      if (id) {
        thumbnailVisible.add(id);
        queued = true;
      }
    }
    if (queued) void reload();
  },
  { root: content, rootMargin: "240px" },
);

/** Watch the marker of every candidate row currently in the list; rows created
 * once are observed once. */
const observeThumbnailCandidates = () => {
  const candidates = new Set(
    entries.filter(isThumbnailCandidate).map((entry) => entry.id),
  );
  for (const row of content.querySelectorAll<HTMLElement>(".clipboard-row")) {
    const id = row.dataset.rowId;
    if (!id || !candidates.has(id) || thumbnailVisible.has(id)) continue;
    thumbnailObserver.observe(row);
  }
};

/**
 * Whether a failed bridge call means the clipboard backend itself is
 * unavailable (the `clipboard-history` feature is off / the command is not
 * registered / the state is unmanaged) rather than a transient hiccup. Those
 * are the errors the host returns for an unknown command, and retrying cannot
 * fix them — the page points at the Settings switch instead.
 */
const isBackendUnavailable = (error: unknown): boolean => {
  const message = typeof error === "string" ? error : String(error ?? "");
  return /not allowed for this plugin page|feature is disabled|unknown command|not found/i.test(message);
};

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
    backendUnavailable = false;
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
  } catch (error) {
    if (gen !== reloadGen) return;
    loadFailed = true;
    if (isBackendUnavailable(error)) backendUnavailable = true;
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

  // Bound parallel binary transfers, and only fetch what can be seen. Failed
  // optional previews retry next poll until they either succeed or change.
  const wanted: ClipboardEntry[] = [];
  const visibleRows = filteredEntries();
  visibleRows.forEach((entry, index) => {
    if (!isThumbnailCandidate(entry) || thumbnails.has(entry.id) || thumbnailPending.has(entry.id)) return;
    // Load rows that scrolled into view, plus the next couple below them, so
    // scrolling on stays seamless without decoding images the user never sees.
    if (thumbnailVisible.has(entry.id)) wanted.push(entry);
    else if (thumbnailVisible.has(visibleRows[index + 1]?.id) || thumbnailVisible.has(visibleRows[index + 2]?.id)) wanted.push(entry);
  });
  const worker = async () => {
    for (let entry = wanted.shift(); entry && gen === reloadGen; entry = wanted.shift()) {
      thumbnailPending.add(entry.id);
      try {
        const bytes = await invokeCommand<number[]>(
          entry.kind === "image" ? "clipboard_read_image" : "clipboard_read_file_preview",
          { id: entry.id },
        );
        if (gen === reloadGen && entries.some((current) => current.id === entry.id)) {
          setThumbnail(entry.id, bytes, entry.kind === "image" ? "image/png" : imageFileMime(entry.paths![0]));
        }
      } catch {
        // Optional preview; the next poll retries it.
      }
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
}).catch((error) => {
  // Only mark failed on a real bridge error, not on empty results.
  if (entries.length === 0) {
    loadFailed = true;
    if (isBackendUnavailable(error)) backendUnavailable = true;
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
