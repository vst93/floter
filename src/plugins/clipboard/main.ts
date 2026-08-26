// The clipboard history page, as a plugin-page citizen.
//
// This is the extracted, framework-free successor of the old in-app React
// panel: same markup classes (the shared stylesheet is imported below), same
// history/favorites/tabs/files/images behavior, and floter's Spotlight
// keyboard discipline — typing always routes to the filter field, Tab toggles
// the all/favorites view, Cmd/Ctrl+Backspace deletes the selected row, focus
// stays pinned to the input.
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
import { BRIDGE_TAG, isBridgeResult } from "../../plugin-pages";

// ---- bridge client -------------------------------------------------------

type PendingCall = { resolve: (value: unknown) => void; reject: (error: string) => void };

const pending = new Map<number, PendingCall>();
let nextCallId = 1;

window.addEventListener("message", (event: MessageEvent) => {
  // Only the host window may talk to us.
  if (event.source !== window.parent) return;
  const data: unknown = event.data;
  if (!isBridgeResult(data)) return;
  const call = pending.get(data.id);
  if (!call) return;
  pending.delete(data.id);
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
    pending.set(id, {
      resolve: (value) => resolve(value as T),
      reject: (error) => reject(error),
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
rootStyle.setProperty(
  "--main-opacity",
  String(Number(params.get("main-opacity") ?? "0.94") || 0.94),
);
rootStyle.setProperty(
  "--terminal-opacity",
  String(Number(params.get("terminal-opacity") ?? "0.92") || 0.92),
);
document.documentElement.setAttribute("data-theme", theme);

// ---- state ----------------------------------------------------------------

type ClipboardView = "all" | "favorites";

let entries: ClipboardEntry[] = [];
let filterText = "";
let view: ClipboardView = "all";
let selected = 0;
const thumbnails = new Map<string, string>();
let statuses: Record<string, boolean> = {};

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
  if (filtered.length === 0) {
    const empty = document.createElement("div");
    empty.className = "clipboard-panel__empty";
    empty.textContent = t(
      scopedEntries().length
        ? "clipboard.emptyFilter"
        : view === "favorites"
          ? "clipboard.emptyFavorites"
          : "clipboard.empty",
    );
    content.append(empty);
    return;
  }

  const list = document.createElement("ul");
  list.className = "clipboard-panel__list";
  list.setAttribute("role", "listbox");
  list.setAttribute("aria-label", t("clipboard.title"));

  const now = Date.now();
  filtered.forEach((entry, index) => {
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
    if (entry.kind === "image" || (entry.kind === "files" && isFilesPreviewCandidate(entry.paths))) {
      if (hasThumbnail) {
        const img = document.createElement("img");
        img.src = thumbnails.get(entry.id)!;
        img.alt = "";
        img.draggable = false;
        marker.append(img);
      } else {
        marker.textContent = "[?]";
      }
    } else if (entry.kind === "files") {
      marker.textContent =
        (entry.paths?.length ?? 0) > 0 && looksLikeDirectoryPath(entry.paths![0])
          ? "▸"
          : "▪";
    } else {
      marker.textContent = "›";
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
    button.addEventListener("mousedown", (event) => event.preventDefault());
    button.addEventListener("click", () => void activate(entry));
    star.addEventListener("click", (event) => {
      event.stopPropagation();
      void toggleFavorite(entry);
    });

    row.append(button);
    list.append(row);
  });
  content.append(list);
};

// ---- data -----------------------------------------------------------------

const setThumbnail = (id: string, bytes: number[], mime: string) => {
  const url = URL.createObjectURL(new Blob([new Uint8Array(bytes)], { type: mime }));
  thumbnails.set(id, url);
  render();
};

const reload = async () => {
  try {
    const rows = await invokeCommand<unknown[]>("clipboard_get_entries", { filter: null });
    entries = normalizeEntries(rows);
  } catch {
    entries = [];
  }
  // Thumbnails for image entries and single-image files entries, fetched once
  // per load; bytes cross the bridge as plain arrays and become blob URLs.
  for (const entry of entries.filter((candidate) => candidate.kind === "image")) {
    invokeCommand<number[]>("clipboard_read_image", { id: entry.id })
      .then((bytes) => setThumbnail(entry.id, bytes, "image/png"))
      .catch(() => undefined);
  }
  for (const entry of entries.filter(
    (candidate) => candidate.kind === "files" && isFilesPreviewCandidate(candidate.paths),
  )) {
    const mime = imageFileMime(entry.paths![0]);
    invokeCommand<number[]>("clipboard_read_file_preview", { id: entry.id })
      .then((bytes) => setThumbnail(entry.id, bytes, mime))
      .catch(() => undefined);
  }
  const fileIds = entries
    .filter((entry) => entry.kind === "files")
    .map((entry) => entry.id);
  statuses = fileIds.length
    ? await invokeCommand<Record<string, boolean>>("clipboard_entry_statuses", {
        ids: fileIds,
      }).catch(() => ({}))
    : {};
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
  try {
    await invokeCommand<void>("clipboard_set_favorite", {
      id: entry.id,
      favorite: nextFavorite,
    });
  } catch {
    await reload().then(render);
  }
};

const removeEntry = async (entry: ClipboardEntry | undefined) => {
  if (!entry) return;
  entries = entries.filter((candidate) => candidate.id !== entry.id);
  render();
  try {
    await invokeCommand<void>("clipboard_delete", { id: entry.id });
  } catch {
    await reload().then(render);
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
};

// ---- keyboard model -------------------------------------------------------

/**
 * Spotlight focus discipline, owned entirely here (the host window never sees
 * a key while the iframe has focus): typing always lands in the filter field,
 * which keeps focus for the page's whole visit; Tab toggles the view;
 * Cmd/Ctrl+Backspace deletes the selected row; Enter puts the entry back on
 * the system clipboard and closes; Esc / Cmd+W close.
 */
window.addEventListener("keydown", (event: KeyboardEvent) => {
  // Cmd+W (macOS) / Ctrl+W (other platforms) dismisses the page — the same
  // convention every overlay surface in floter follows.
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
    event.preventDefault();
    event.stopPropagation();
    requestClose();
    return;
  }
  if ((event.metaKey || event.ctrlKey) && event.key === "Backspace") {
    event.preventDefault();
    event.stopPropagation();
    void removeEntry(filteredEntries()[selected]);
    searchInput.focus();
    return;
  }
  if (event.metaKey || event.ctrlKey) {
    // Platform copy/paste/cut shortcuts keep working inside the filter field.
    return;
  }
  if (event.altKey && event.key.length === 1 && document.activeElement !== searchInput) {
    // Alt+letter combinations belong to global shortcuts; ignore.
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
    // the caret belongs to the filter for as long as this page is up.
    event.preventDefault();
    event.stopPropagation();
    view = view === "all" ? "favorites" : "all";
    selected = 0;
    render();
    searchInput.focus();
    return;
  }
  if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
    // Tabs also switch directly on the keypress — no focus dance between the
    // tab buttons first.
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
  const inSearch = document.activeElement === searchInput;
  if (inSearch) {
    // Content-editing keys stay in the filter field; everything below only
    // fires when the press happened outside it.
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
  if (event.key === "f" || event.key === "F" || event.key === "*") {
    event.preventDefault();
    event.stopPropagation();
    void toggleFavorite(filteredEntries()[selected]);
    return;
  }
  if (event.key === "Delete" || event.key === "d" || event.key === "D") {
    event.preventDefault();
    event.stopPropagation();
    void removeEntry(filteredEntries()[selected]);
  }
});

/**
 * A printable key typed anywhere outside the field is inserted by hand — the
 * field was not focused when the press happened (a click landed on a row, say)
 * and nothing else will insert it. This is what keeps "typing routes to the
 * filter" literally true.
 */
window.addEventListener("keypress", (event) => {
  if (document.activeElement === searchInput) return;
  if (event.metaKey || event.ctrlKey || event.altKey) return;
  if (event.key.length !== 1) return;
  event.preventDefault();
  filterText = filterText + event.key;
  searchInput.value = filterText;
  selected = 0;
  render();
  searchInput.focus();
});

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

void reload().then(render);
render();
