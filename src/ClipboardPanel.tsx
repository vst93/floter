import { useCallback, useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  clipboardPreview,
  filterClipboardEntries,
  formatClipboardAge,
  formatFilesPreview,
  imageFileMime,
  isFilesPreviewCandidate,
  looksLikeDirectoryPath,
  normalizeEntries,
  shellQuotePath,
  type ClipboardEntry,
} from "./clipboard-history";
import type { Translate } from "./i18n";

type ClipboardPanelProps = {
  t: Translate;
  /** Whether an embedded terminal session is live; Enter then also pastes. */
  terminalActive: boolean;
  onPasteText: (text: string) => void;
  onClose: () => void;
};

type ClipboardView = "all" | "favorites";

/**
 * The clipboard history page — a terminal application surface.
 *
 * It renders INSIDE the terminal card (the same shell and window geometry as
 * terminal mode; see App.tsx's clipboard branch), full-bleed and monospace:
 * one prompt-like search line with an 全部/收藏 segmented control, then plain
 * terminal rows — type marker, one-line preview, character count and relative
 * age for text, favorite star. Keyboard-first: arrows move, Enter puts the
 * entry back on the system clipboard (pasting it into the embedded terminal
 * when one is live), F stars, Delete removes, Esc closes.
 *
 * Files entries hold path references only: the row shows a glyph or — for a
 * single image file — the file's own pixels read on demand. Existence of
 * every referenced path is checked once per load; rows whose files have
 * vanished render dimmed with a missing label and refuse to activate.
 */
export function ClipboardPanel({
  t,
  terminalActive,
  onPasteText,
  onClose,
}: ClipboardPanelProps) {
  const [entries, setEntries] = useState<ClipboardEntry[]>([]);
  const [filter, setFilter] = useState("");
  const [view, setView] = useState<ClipboardView>("all");
  const [selected, setSelected] = useState(0);
  const [thumbnails, setThumbnails] = useState<Record<string, string>>({});
  /** Per-files-entry existence, fetched once per load; `false` = missing. */
  const [statuses, setStatuses] = useState<Record<string, boolean>>({});
  /** Every object URL ever handed out, so unmount can revoke them all — the
   * state map alone cannot, since its snapshot at cleanup time is empty. */
  const liveThumbnailUrls = useRef<Set<string>>(new Set());
  const searchRef = useRef<HTMLInputElement>(null);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const reload = useCallback((): Promise<ClipboardEntry[]> => {
    return invoke<unknown[]>("clipboard_get_entries", { filter: null })
      .then((rows) => {
        const next = normalizeEntries(rows);
        setEntries(next);
        return next;
      })
      .catch(() => {
        setEntries([]);
        return [];
      });
  }, []);

  useEffect(() => {
    void reload().then((loaded) => {
      // Thumbnails for image entries, fetched once per load; the panel holds
      // at most a few hundred entries and images dominate the payload.
      for (const entry of loaded.filter((candidate) => candidate.kind === "image")) {
        invoke<number[]>("clipboard_read_image", { id: entry.id })
          .then((bytes) => {
            const url = URL.createObjectURL(new Blob([new Uint8Array(bytes)], { type: "image/png" }));
            liveThumbnailUrls.current.add(url);
            setThumbnails((current) => ({ ...current, [entry.id]: url }));
          })
          .catch(() => undefined);
      }
      // Single-path image files preview straight from disk through the same
      // blob-URL machinery.
      for (const entry of loaded.filter((candidate) =>
        candidate.kind === "files" && isFilesPreviewCandidate(candidate.paths),
      )) {
        const mime = imageFileMime(entry.paths![0]);
        invoke<number[]>("clipboard_read_file_preview", { id: entry.id })
          .then((bytes) => {
            const url = URL.createObjectURL(new Blob([new Uint8Array(bytes)], { type: mime }));
            liveThumbnailUrls.current.add(url);
            setThumbnails((current) => ({ ...current, [entry.id]: url }));
          })
          .catch(() => undefined);
      }
      // Existence check for every files row: pure metadata on our own stored
      // paths, so one round trip covers the whole history.
      const fileIds = loaded.filter((entry) => entry.kind === "files").map((entry) => entry.id);
      if (fileIds.length === 0) {
        setStatuses({});
        return;
      }
      invoke<Record<string, boolean>>("clipboard_entry_statuses", { ids: fileIds })
        .then(setStatuses)
        .catch(() => setStatuses({}));
    });
  }, [reload]);

  useEffect(
    () => () => {
      for (const url of liveThumbnailUrls.current) URL.revokeObjectURL(url);
    },
    [],
  );

  useEffect(() => {
    searchRef.current?.focus({ preventScroll: true });
  }, []);

  useEffect(() => {
    setSelected((current) =>
      entries.length ? Math.min(current, entries.length - 1) : 0,
    );
  }, [entries.length]);

  const scoped = view === "favorites"
    ? entries.filter((entry) => entry.favorite)
    : entries;
  const filtered = filterClipboardEntries(scoped, filter);

  const activate = async (entry: ClipboardEntry | undefined) => {
    if (!entry || statuses[entry.id] === false) return;
    try {
      await invoke("clipboard_copy_entry", { id: entry.id });
    } catch {
      // The clipboard may be held by another app; keep the panel open so the
      // user can retry instead of silently losing the action.
      return;
    }
    if (entry.kind === "text" && terminalActive) {
      onPasteText(entry.text ?? "");
    } else if (entry.kind === "files" && terminalActive) {
      // Shell-quoted so spaces and quotes in paths survive the shell.
      onPasteText((entry.paths ?? []).map(shellQuotePath).join(" "));
    }
    onClose();
  };

  const toggleFavorite = async (entry: ClipboardEntry | undefined) => {
    if (!entry) return;
    const nextFavorite = !entry.favorite;
    setEntries((current) =>
      current.map((candidate) =>
        candidate.id === entry.id ? { ...candidate, favorite: nextFavorite } : candidate,
      ),
    );
    try {
      await invoke("clipboard_set_favorite", { id: entry.id, favorite: nextFavorite });
    } catch {
      await reload();
    }
  };

  const removeEntry = async (entry: ClipboardEntry | undefined) => {
    if (!entry) return;
    setEntries((current) => current.filter((candidate) => candidate.id !== entry.id));
    try {
      await invoke("clipboard_delete", { id: entry.id });
    } catch {
      await reload();
    }
  };

  /** Wipe every non-favorite entry — the backend's contract guarantees
   * favorites survive, so one click needs no confirmation dialog. */
  const clearHistory = async () => {
    try {
      await invoke("clipboard_clear_history");
    } catch {
      // A failed clear leaves the history as it was; reload shows the truth.
    }
    setSelected(0);
    await reload();
  };

  useEffect(() => {
    // Capture phase so the launcher's window-level handlers never see a key
    // this panel has claimed.
    const onKeyDown = (event: KeyboardEvent) => {
      // Cmd+W (macOS) / Ctrl+W (other platforms) dismisses the panel — a
      // convention every overlay surface in floter follows.
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
        event.preventDefault();
        event.stopPropagation();
        onClose();
        return;
      }
      if (event.metaKey || event.ctrlKey) return;
      const inSearch = event.target === searchRef.current;
      if (event.altKey && !inSearch && event.key.length === 1) {
        // Alt+letter combinations belong to global shortcuts; ignore.
        return;
      }

      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onClose();
        return;
      }
      if (event.key === "ArrowDown") {
        event.preventDefault();
        event.stopPropagation();
        setSelected((current) => (filtered.length ? (current + 1) % filtered.length : 0));
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        event.stopPropagation();
        setSelected((current) =>
          filtered.length ? (current - 1 + filtered.length) % filtered.length : 0,
        );
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        event.stopPropagation();
        void activate(filtered[selected]);
        return;
      }
      // Content-editing keys stay in the search field; everything below is
      // only reachable once focus has moved off it (Tab into the list).
      if (inSearch) return;
      if (event.key === "1" || event.key === "2") {
        // Direct tab jump — only reachable once focus has left the filter
        // field, so typing "12" still filters instead of switching tabs.
        event.preventDefault();
        event.stopPropagation();
        setView(event.key === "1" ? "all" : "favorites");
        setSelected(0);
        return;
      }
      if (event.key === "f" || event.key === "F" || event.key === "*") {
        event.preventDefault();
        event.stopPropagation();
        void toggleFavorite(filtered[selected]);
        return;
      }
      if (event.key === "Delete" || event.key === "d" || event.key === "D") {
        event.preventDefault();
        event.stopPropagation();
        void removeEntry(filtered[selected]);
      }
    };

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filtered, selected, onClose]);

  /** Shared switch logic for click and keyboard activation: a tab change
   * always lands on row 0, so the selection never points across lists. */
  const switchView = (next: ClipboardView) => {
    setView(next);
    setSelected(0);
  };

  const favoritesCount = entries.reduce((total, entry) => total + (entry.favorite ? 1 : 0), 0);
  const now = Date.now();

  // Terminal-page anatomy: a prompt-like search line (`filter❯`) with the
  // 全部/收藏 segmented control on the same row, the history as plain lines
  // under it, and a one-line hint footer. Everything is monospace via the
  // panel class; the page fills the terminal card it is mounted into (see
  // App.css, `.clipboard-panel`).
  return (
    <div className="clipboard-panel">
      <div className="clipboard-panel__topbar">
        <span className="clipboard-panel__prompt" aria-hidden="true">
          {t("clipboard.prompt")}❯
        </span>
        <input
          ref={searchRef}
          className="clipboard-panel__search"
          value={filter}
          placeholder={t("clipboard.filter")}
          aria-label={t("clipboard.title")}
          spellCheck={false}
          autoCapitalize="off"
          autoCorrect="off"
          onChange={(event) => {
            setFilter(event.target.value);
            setSelected(0);
          }}
        />
        {filter && (
          <button
            type="button"
            className="clipboard-panel__filter-clear"
            aria-label={t("clipboard.filterClear")}
            title={t("clipboard.filterClear")}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              setFilter("");
              setSelected(0);
              searchRef.current?.focus();
            }}
          >
            ×
          </button>
        )}
        <div
          className="clipboard-panel__tabs"
          role="tablist"
          aria-label={t("clipboard.title")}
          onKeyDown={(event) => {
            // Two tabs, so either arrow moves to the other one; the moved-to
            // tab takes focus so repeated arrows keep working.
            if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
            event.preventDefault();
            const next: ClipboardView = view === "all" ? "favorites" : "all";
            switchView(next);
            tabRefs.current[next === "all" ? 0 : 1]?.focus();
          }}
        >
          <button
            type="button"
            role="tab"
            ref={(node) => { tabRefs.current[0] = node; }}
            tabIndex={view === "all" ? 0 : -1}
            aria-selected={view === "all"}
            className={`clipboard-panel__tab${view === "all" ? " clipboard-panel__tab--active" : ""}`}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => switchView("all")}
          >
            {t("clipboard.tabAll")}
            <span className="clipboard-panel__tab-count">{entries.length}</span>
          </button>
          <button
            type="button"
            role="tab"
            ref={(node) => { tabRefs.current[1] = node; }}
            tabIndex={view === "favorites" ? 0 : -1}
            aria-selected={view === "favorites"}
            className={`clipboard-panel__tab${view === "favorites" ? " clipboard-panel__tab--active" : ""}`}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => switchView("favorites")}
          >
            {t("clipboard.tabFavorites")}
            <span className="clipboard-panel__tab-count">{favoritesCount}</span>
          </button>
        </div>
      </div>
      {filtered.length === 0 ? (
        <div className="clipboard-panel__empty">
          {t(
            scoped.length
              ? "clipboard.emptyFilter"
              : view === "favorites"
                ? "clipboard.emptyFavorites"
                : "clipboard.empty",
          )}
        </div>
      ) : (
        <ul className="clipboard-panel__list" role="listbox" aria-label={t("clipboard.title")}>
          {filtered.map((entry, index) => {
            const missing = statuses[entry.id] === false;
            const filesPreview = entry.kind === "files" ? formatFilesPreview(entry.paths) : null;
            const hasThumbnail = Boolean(thumbnails[entry.id]) && !missing;
            const markerClass = [
              "clipboard-row__marker",
              entry.kind === "image" ? "clipboard-row__marker--image" : "",
              entry.kind === "files" ? "clipboard-row__marker--files" : "",
            ]
              .filter(Boolean)
              .join(" ");
            return (
            <li key={entry.id}>
              <button
                type="button"
                role="option"
                aria-selected={index === selected}
                tabIndex={-1}
                className={`clipboard-row${index === selected ? " clipboard-row--selected" : ""}${missing ? " clipboard-row--missing" : ""}`}
                title={entry.kind === "files" ? (entry.paths ?? []).join("\n") : undefined}
                onMouseMove={() => setSelected(index)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => void activate(entry)}
              >
                <span className={markerClass} aria-hidden="true">
                  {(entry.kind === "image" || (entry.kind === "files" && isFilesPreviewCandidate(entry.paths))) ? (
                    hasThumbnail
                      ? <img src={thumbnails[entry.id]} alt="" draggable={false} />
                      : "[?]"
                  ) : entry.kind === "files" ? (
                    (entry.paths?.length ?? 0) > 0 && looksLikeDirectoryPath(entry.paths![0]) ? "▸" : "▪"
                  ) : (
                    "›"
                  )}
                </span>
                <span className="clipboard-row__preview">
                  {filesPreview ? (
                    <>
                      {filesPreview.dirname && (
                        <span className="clipboard-row__preview-dir">{filesPreview.dirname}</span>
                      )}
                      <span>{filesPreview.basename}</span>
                      {filesPreview.extra > 0 && (
                        <span className="clipboard-row__preview-extra"> +{filesPreview.extra}</span>
                      )}
                    </>
                  ) : (
                    clipboardPreview(entry)
                  )}
                </span>
                {entry.kind === "text" && entry.text ? (
                  <span className="clipboard-row__chars">
                    {t("clipboard.chars", { n: entry.text.length })}
                  </span>
                ) : null}
                <span className={`clipboard-row__age${missing ? " clipboard-row__age--missing" : ""}`}>
                  {missing
                    ? t("clipboard.missing")
                    : formatClipboardAge(entry.created_at, now)}
                </span>
                <button
                  type="button"
                  tabIndex={-1}
                  className={`clipboard-row__star${entry.favorite ? " clipboard-row__star--on" : ""}`}
                  aria-label={t("clipboard.favorite")}
                  title={t("clipboard.favorite")}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={(event) => {
                    event.stopPropagation();
                    void toggleFavorite(entry);
                  }}
                >
                  {entry.favorite ? "★" : "☆"}
                </button>
              </button>
            </li>
            );
          })}
        </ul>
      )}
      <div className="clipboard-panel__footer">
        <span className="clipboard-panel__hints">
          {t("clipboard.hintPaste")} · {t("clipboard.hintStar")} · {t("clipboard.hintDelete")}
        </span>
        <button
          type="button"
          className="clipboard-panel__clear"
          title={t("clipboard.clearTitle")}
          aria-label={t("clipboard.clearTitle")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => void clearHistory()}
        >
          {t("clipboard.clear")}
        </button>
      </div>
    </div>
  );
}
