// The clipboard history page, as a plugin-page citizen.
//
// GLASS-CLIP-2 rewrote this page's list around two user reports — 「剪切板还是
// 没有跟随透明度调整」 and 「剪贴板的 ui 和 ux 也要重写」 — and the rewrite
// changed *how the page behaves*, so the summary below is the current truth:
//
//   · **Material** — the page *sheet* (`--page-fill`) is the frame it replaces
//     and follows the terminal transparency, as before. The *list field* and
//     each *row card* now follow the same slider through the page's own, steeper
//     band (`GLASS_PAGE_CONTENT_BAND` in `src/glass-material.ts`), which is what
//     makes the follow legible: the host recess band the page used to borrow
//     moved by 0.15 across the whole slider and was invisible at page scale.
//     Body copy sits on the row card (one ladder rung above the field), so the
//     field can travel all the way down without costing legibility.
//   · **Keyboard** — the model is [`resolveClipboardKey`] in
//     `src/clipboard-list.ts`, a pure function this file only executes: ↑/↓
//     move, Enter copies (and dismisses), ⌫/Delete delete, P pins (F/* kept as
//     the pre-rewrite spellings), Tab toggles the pinned scope, ←/→ walk the
//     type bar, 1/2 set the scope, 3–7 jump to a type, and the filter field
//     keeps its own typing. A printable key with the list focused is routed into
//     the filter.
//   · **Rows** — one line at rest (icon / source / relative age / preview),
//     expanding under hover or focus, with the copy·pin·delete trio revealed on
//     hover/focus/selection. The list is still reconciled in place.
//
// It runs inside a sandboxed iframe served by the generic plugin-page pipeline
// and reaches the host ONLY through the postMessage bridge — every command here
// goes over that bridge, dogfooding the mechanism end to end. The bridge
// protocol and the command allowlist are unchanged by the rewrite.

import "./page.css";
import { createTranslator, normalizeLanguage, type Translate } from "../../i18n";
import {
  clipboardAge,
  clipboardEntryType,
  clipboardPreview,
  filterClipboardEntries,
  formatClipboardDateTime,
  formatFilesPreview,
  imageFileMime,
  isFilesPreviewCandidate,
  normalizeClipboardTypeFilter,
  normalizeEntries,
  normalizeClipboardSession,
  sameClipboardSnapshot,
  splitFilePath,
  urlHost,
  type ClipboardEntry,
  type ClipboardEntryType,
} from "../../clipboard-history";
import { clipboardIcon, type ClipboardIconName } from "../../clipboard-icons";
import {
  applyClipboardFilters,
  clipboardChipFace,
  clipboardTypeCounts,
  cycleClipboardTypeFilter,
  moveClipboardSelection,
  resolveClipboardKey,
  CLIPBOARD_TYPE_ICON,
  CLIPBOARD_TYPE_LABEL,
  type ClipboardChipFace,
  type ClipboardView,
} from "../../clipboard-list";
import { BRIDGE_TAG, createFailureDeduper, createRetryRegistry, isBridgeGlass, isBridgeNotifyRetry, isBridgeOpacity, isBridgeTheme, isBridgeResultForSession, isBridgeReload, isBridgeVisibility } from "../../plugin-pages";
import { GLASS_STEP_TOKENS, GLASS_SOLID_TOP, GLASS_FRAME_FLOOR, glassPageContentAlpha, glassPageRowAlpha, normalizeGlassStep, type GlassStep } from "../../glass-material";

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
  if (isBridgeGlass(data)) {
    // The material step changed host-side; swap the tokens this document's
    // `--page-fill` derives from and repaint. An older host never sends this
    // message — the page then keeps the `glass-step` bootstrap param (or mid).
    applyGlassStep(data.glassStep);
    return;
  }
  if (isBridgeTheme(data)) {
    // Theme changed host-side; update the page's data-theme attribute and its
    // opaque page background without relying on rgba() variable alpha syntax.
    activeTheme = data.theme;
    document.documentElement.setAttribute("data-theme", data.theme);
    const rawOpacity = Number(rootStyle.getPropertyValue("--terminal-opacity"));
    applyPageBackground(Number.isFinite(rawOpacity) ? rawOpacity : 0.46);
    return;
  }
  if (isBridgeReload(data)) {
    // Page just became visible after being hidden; reload data.
    void handleReload();
    return;
  }
  if (isBridgeNotifyRetry(data)) {
    // The user pressed the retry action on a toast this page raised. Only the
    // page knows what failed, so the host hands the intent back instead of
    // trying to replay the command itself. The id the host echoes decides
    // *which* failure re-runs; an id already run, or one evicted because
    // newer failures pushed it out, is dropped rather than falling back to
    // whatever action happens to be newest.
    retryRegistry.run(data.id);
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

// ---- host feedback --------------------------------------------------------

/**
 * The retry the page promised for a toast it raised, keyed by that toast's id.
 * Feedback is host-owned chrome, so the page never paints its own notice: it
 * names a dictionary key, the host shows it on the app's one toast stack
 * (same position, same lifetime, same visual as every other surface), and if
 * the action can be retried the host sends `notify-retry` back with the id it
 * was given. The page keeps the thunk because only the page can re-run its
 * own action against its own state.
 *
 * Keyed, not a single slot: up to `MAX_TOASTS` toasts coexist and each carries
 * its own Retry, so the oldest one must fire the action that raised *it*.
 */
const retryRegistry = createRetryRegistry();

/** Id stamped on the next `host-notify` message; the host echoes it back. */
let nextNotifyId = 1;

/**
 * One background-load failure per window: the 2s poll re-runs `reload()` while
 * the backend is down, and each failure used to raise a fresh toast, so three
 * toasts churned forever and none was readable. The window (30s) is long enough
 * to be quiet and short enough that a failure that comes back later is
 * announced again. Only the *poll* is coalesced — a failure caused by a user
 * gesture (copy, delete, clear) still reports every time the user asks, or the
 * second click would fail silently. A successful reload re-arms the key so
 * failure → recovery → failure shows a fresh toast.
 */
const loadFailureDeduper = createFailureDeduper();

/** Raise host feedback for a failed action. `onRetry` is omitted when retrying
 * cannot help (backend off, page not loaded) — the toast then offers only a
 * dismissal instead of a dead-end button. */
const notifyFailure = (messageKey: string, onRetry?: () => void) => {
  const id = nextNotifyId++;
  if (onRetry) retryRegistry.add(id, onRetry);
  window.parent.postMessage(
    { [BRIDGE_TAG]: "host-notify", id, kind: "error", messageKey, retryable: Boolean(onRetry) },
    "*",
  );
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

function applyPageBackground(transparency: number) {
  // WebKit rejects rgba() when its alpha argument is a CSS variable. Keep the
  // complete color as one custom property instead of composing it in CSS.
  //
  // The alpha is the *frame alpha plus the step's haze*: GLASS-3STOP made the
  // transparency slider the only frame-alpha truth, clamped to the near-solid
  // top and lifted only by the accessibility floor, with the material step's
  // haze composited underneath (a 50% veil on the thin frosted end keeps it
  // readable over bright content). The arithmetic mirrors base.css's
  // `--glass-frame-alpha` and `--glass-tint-alpha`.
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

/**
 * Adopt a material step. The values come from the shared `GLASS_STEP_TOKENS`
 * table in `src/glass-material.ts`, not from literals here, so the host and
 * the page can never disagree about what `frosted`/`regular`/`liquid` mean.
 * Called once from the bootstrap param and again whenever the host pushes a
 * new step.
 */
function applyGlassStep(step: GlassStep) {
  const tokens = GLASS_STEP_TOKENS[step];
  rootStyle.setProperty("--glass-step-dim", String(tokens.dim));
  rootStyle.setProperty("--glass-solid-top", String(GLASS_SOLID_TOP));
  rootStyle.setProperty("--glass-frame-floor", String(GLASS_FRAME_FLOOR));
  // Re-derive the fill for the transparency already in force: a step change
  // moves the material without touching the slider.
  const rawTerminal = Number.parseFloat(rootStyle.getPropertyValue("--terminal-opacity"));
  applyPageBackground(Number.isFinite(rawTerminal) ? rawTerminal : 0.46);
}

function applyOpacity(main: number, terminal: number) {
  rootStyle.setProperty("--main-opacity", String(main));
  rootStyle.setProperty("--terminal-opacity", String(terminal));
  // GLASS-CLIP-2: the page's field and row cards follow the transparency
  // slider through the page's OWN band (steeper than the host recess band — see
  // `GLASS_PAGE_CONTENT_BAND`), evaluated at the *terminal* transparency, which
  // is the frame this page replaces. `--glass-content-alpha` is the field the
  // list sits on and `--glass-row-alpha` the card each row is; the rung between
  // them is what keeps the copy legible at the thin end while the field still
  // travels the whole way. Both are re-derived here so a slider move repaints
  // the list material, not just the sheet.
  rootStyle.setProperty("--glass-content-alpha", String(glassPageContentAlpha(terminal)));
  rootStyle.setProperty("--glass-row-alpha", String(glassPageRowAlpha(terminal)));
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

applyOpacity(opacityParam("main-opacity", 0.47), opacityParam("terminal-opacity", 0.46));
// The step arrives as a bootstrap param too. An older host that does not send
// it (or a hand-typed URL) falls back to Regular via `normalizeGlassStep`,
// which is the same "unknown value rests on mid" rule the Rust loader uses.
applyGlassStep(normalizeGlassStep(params.get("glass-step")));
document.documentElement.setAttribute("data-theme", theme);

// ---- state ----------------------------------------------------------------

// GLASS-CLIP-2 · the list's two filter axes are independent:
//
//   * `view`      — 全部 / 收藏, the scope the data comes from (favorites are
//                   exempt from pruning, so this is a *retention* view);
//   * `typeFilter`— 全部 / 文本 / 链接 / 颜色 / 文件, the *kind* of capture.
//
// They compose (收藏 + 链接 is a legal state) and each has its own control: a
// Tab / ←→ / 1-2 toggle for the scope, a click / 3-7 for the type. Keeping them
// as two variables rather than one five-way enum is what makes that
// composition free. [`ClipboardView`] and the filter machinery live in
// `clipboard-list.ts`.
const SESSION_KEY = "floter.clipboard.session";
const savedSession = (() => {
  try { return normalizeClipboardSession(JSON.parse(sessionStorage.getItem(SESSION_KEY) ?? "null")); }
  catch { return normalizeClipboardSession(null); }
})();
/** The saved type filter, read through its own normalizer so the stored
 * session's shape stays exactly what it always was (see
 * `normalizeClipboardTypeFilter`). */
const savedTypeFilter = (() => {
  try {
    const raw = JSON.parse(sessionStorage.getItem(SESSION_KEY) ?? "null") as { typeFilter?: unknown } | null;
    return normalizeClipboardTypeFilter(raw?.typeFilter);
  } catch { return null; }
})();

let entries: ClipboardEntry[] = [];
let filterText = savedSession.filterText;
let view: ClipboardView = savedSession.view;
let typeFilter: ClipboardEntryType | null = savedTypeFilter;
let selected = 0;
let hydrated = false;
let busy = false;
let clearArmed = false;
let clearTimer: number | null = null;
let pageVisible = true;
let pageDisposed = false;
/** The entry id whose copy action is showing its "copied" confirmation, and
 * the timer that clears it. Kept as state (not a CSS animation) so the tick is
 * driven by the same render pass as everything else and cannot outlive the row
 * it belongs to. */
let copiedId: string | null = null;
let copiedTimer: number | null = null;
/** Whether the first fetch has landed (successfully or not). Drives the
 * loading state: only a page with nothing on screen yet shows the inline
 * spinner, so a background poll never replaces a populated list. */
let loaded = false;
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
/** The list, after both filters: scope first (it is the cheaper cut and the one
 * the counts are reported against), then the type, then the text query. The
 * cuts and their order live in `clipboard-list.ts` so the node suite pins what
 * each tab's count means; this is only the page's binding of its own state.
 */
const filteredEntries = (): ClipboardEntry[] =>
  applyClipboardFilters(entries, view, typeFilter, filterText, filterClipboardEntries);

/** How many entries each type tab should show, computed from the *scoped* set
 * (so the counts agree with what the tab would actually reveal, not with the
 * whole history while a favorites view is active). */
const typeCounts = (): Record<ClipboardEntryType, number> => clipboardTypeCounts(entries, view);

// ---- DOM scaffold ---------------------------------------------------------

const root = document.getElementById("root") ?? document.body;

root.innerHTML = `
  <div class="clipboard-panel">
    <div class="clipboard-panel__topbar">
      <span class="clipboard-panel__prompt" aria-hidden="true"></span>
      <input class="clipboard-panel__search" maxlength="512" spellcheck="false" autocapitalize="off" autocorrect="off" />
      <button type="button" class="clipboard-panel__filter-clear" hidden></button>
      <div class="clipboard-panel__tabs" role="tablist" data-axis="view">
        <button type="button" role="tab" class="clipboard-panel__type" data-view="all"></button>
        <button type="button" role="tab" class="clipboard-panel__type" data-view="favorites"></button>
      </div>
    </div>
    <div class="clipboard-panel__filterbar">
      <div class="clipboard-panel__tabs" role="tablist" data-axis="type">
        <button type="button" role="tab" class="clipboard-panel__type" data-type=""></button>
        <button type="button" role="tab" class="clipboard-panel__type" data-type="text"></button>
        <button type="button" role="tab" class="clipboard-panel__type" data-type="link"></button>
        <button type="button" role="tab" class="clipboard-panel__type" data-type="color"></button>
        <button type="button" role="tab" class="clipboard-panel__type" data-type="image"></button>
        <button type="button" role="tab" class="clipboard-panel__type" data-type="files"></button>
      </div>
      <span class="clipboard-panel__tally" role="status" aria-live="polite"></span>
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
searchInput.value = filterText;

const saveSession = () => {
  if (!hydrated) return;
  try {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify({
      filterText, view, selectedId: filteredEntries()[selected]?.id ?? null, scrollTop: content.scrollTop,
      // GLASS-CLIP-2 · the type filter rides the same session record. It is an
      // *extra* field, not a change to `ClipboardSession`: an older build reads
      // the record it always did and ignores this, and this build reads the
      // four it always did plus the one it added.
      typeFilter,
    }));
  } catch { /* Session storage may be unavailable in a sandbox. */ }
};
const filterClear = root.querySelector<HTMLButtonElement>(".clipboard-panel__filter-clear")!;
const content = root.querySelector<HTMLElement>(".clipboard-panel__content")!;
const hints = root.querySelector<HTMLElement>(".clipboard-panel__hints")!;
const clearButton = root.querySelector<HTMLButtonElement>(".clipboard-panel__clear")!;
const tally = root.querySelector<HTMLElement>(".clipboard-panel__tally")!;
const panel = root.querySelector<HTMLElement>(".clipboard-panel")!;
const viewTabs = Array.from(root.querySelectorAll<HTMLButtonElement>('[data-axis="view"] .clipboard-panel__type'));
const typeTabs = Array.from(root.querySelectorAll<HTMLButtonElement>('[data-axis="type"] .clipboard-panel__type'));
// ---- rendering ------------------------------------------------------------

const countBadge = (count: number) => {
  const badge = document.createElement("span");
  badge.className = "clipboard-panel__type-count";
  badge.textContent = String(count);
  return badge;
};

/**
 * The type chip: one 32×32 slot with five faces.
 *
 *   · image / image-previewable files → the real thumbnail once its bytes have
 *     arrived, the type glyph until then;
 *   · a colour entry → the colour itself, painted as a swatch (the point of the
 *     row is the value, and a swatch reads it faster than any glyph);
 *   · a directory → the folder glyph;
 *   · a file → its uppercase extension badge (`PNG`), or the file glyph when the
 *     name has no extension;
 *   · text / link → the Lucide type / link glyph.
 *
 * The *which face* decision lives in `clipboard-list.ts` ([`clipboardChipFace`])
 * so the node suite can exercise the precedence without a DOM; this function is
 * only the paint. The thumbnail path is the one face that changes after creation
 * (bytes arrive asynchronously), so the chip is rebuilt only when the pixels it
 * should be showing actually differ — the same "do not churn the DOM" rule the
 * list reconcile follows, driven by the face's own signature.
 */
const renderTypeChip = (
  chip: HTMLElement,
  entry: ClipboardEntry,
  hasThumbnail: boolean,
) => {
  const face: ClipboardChipFace = clipboardChipFace(entry, hasThumbnail);
  const thumbnailUrl = thumbnails.get(entry.id);
  // The model decides the *kind* of face; the page owns the blob handle, so it
  // folds the URL into the comparison key. A face that is a thumbnail only wins
  // if the handle has actually arrived (`hasThumbnail` is the map's membership,
  // so the URL is present whenever the face says `thumbnail`).
  const signature = face.kind === "thumbnail" ? `thumb:${thumbnailUrl}` : face.signature;
  if (chip.dataset.faceSig === signature) return;
  chip.dataset.faceSig = signature;
  chip.replaceChildren();

  switch (face.kind) {
    case "thumbnail": {
      const img = document.createElement("img");
      img.src = thumbnailUrl!;
      img.alt = "";
      img.draggable = false;
      chip.append(img);
      return;
    }
    case "swatch": {
      const swatch = document.createElement("span");
      swatch.className = "clipboard-row__swatch";
      // The literal is painted straight from the entry's own text — it is
      // validated as a colour by `clipboardEntryType` before this path is
      // reached, so nothing untrusted is interpolated into the style. A value
      // the engine refuses (an exotic-but-valid literal) simply leaves the
      // swatch transparent with its hairline, which still reads as "a colour".
      swatch.style.background = (entry.text ?? "").trim();
      chip.append(swatch);
      return;
    }
    case "badge":
      chip.textContent = face.text;
      return;
    default:
      chip.append(clipboardIcon(document, face.icon, 16));
  }
};

/**
 * The row's secondary line: the one concrete fact about the entry that its
 * preview does not already say.
 *
 *   · a link → the host it points at (`example.com`);
 *   · an image → its pixel dimensions, or its caption's source when it has one;
 *   · a file → the directory the first path lives in;
 *   · text / colour → nothing (the preview is the whole fact).
 */
const rowSource = (entry: ClipboardEntry): string => {
  const type = clipboardEntryType(entry);
  if (type === "link") return urlHost(entry.text);
  if (entry.kind === "image") {
    return Number.isFinite(entry.width) && Number.isFinite(entry.height)
      ? `${entry.width} × ${entry.height}`
      : "";
  }
  if (entry.kind === "files") {
    const first = entry.paths?.[0];
    if (!first) return "";
    const { dirname } = splitFilePath(first);
    const extra = Math.max(0, (entry.paths?.length ?? 1) - 1);
    const suffix = extra > 0 ? ` +${extra}` : "";
    return `${dirname || "/"}${suffix}`;
  }
  return "";
};

/** The human age sentence: the structured [`clipboardAge`] fact rendered
 * through the dictionary, so the wording (and its pluralization) lives in
 * `i18n.ts` rather than in a string builder here. */
const formatAgeSentence = (entry: ClipboardEntry, now: number): string => {
  const age = clipboardAge(entry.created_at, now);
  switch (age.unit) {
    case "now": return t("clipboard.ageNow");
    case "minute": return t("clipboard.ageMinute", { n: age.value });
    case "hour": return t("clipboard.ageHour", { n: age.value });
    case "day": return t("clipboard.ageDay", { n: age.value });
    default: return formatClipboardDateTime(entry.created_at);
  }
};

/** Empty-state markup: title + one-line explanation + the privacy note, the
 * shared shape of every empty state in the app (a title, a why, and — where
 * one exists — the action that changes the situation). The failure case is a
 * separate path in `render()` because it carries retry/dismiss controls. The
 * empty state is not one of those: there is nothing to retry here — the user
 * simply has not copied anything (or filtered it away) yet. */
const renderEmpty = (): DocumentFragment => {
  const fragment = document.createDocumentFragment();
  const scoped = scopedEntries().length;
  // Four empty situations, most specific first: a type tab that matched
  // nothing, a query that matched nothing, an empty favorites view, and the
  // first-run state. The order matters — "no links" is a different sentence
  // from "no matches", and telling a user to shorten their search when the
  // problem is the type tab they are standing on would be a lie.
  const title = scoped === 0
    ? view === "favorites"
      ? t("clipboard.emptyFavorites")
      : t("clipboard.empty")
    : filterText.trim()
      ? t("clipboard.emptyFilter")
      : typeFilter
        ? t("clipboard.emptyType")
        : t("clipboard.emptyFilter");
  const hint = scoped === 0
    ? view === "favorites"
      ? t("clipboard.emptyFavoritesHint")
      : t("clipboard.emptyHint")
    : filterText.trim()
      ? t("clipboard.emptyFilterHint")
      : typeFilter
        ? t("clipboard.emptyTypeHint")
        : t("clipboard.emptyFilterHint");

  const block = document.createElement("div");
  block.className = "clipboard-panel__empty";
  const heading = document.createElement("div");
  heading.className = "clipboard-panel__empty-title";
  heading.textContent = title;
  const detail = document.createElement("div");
  detail.className = "clipboard-panel__empty-hint";
  detail.textContent = hint;
  block.append(heading, detail);

  // GLASS-CLIP-2 · a first-run empty state teaches instead of being blank: the
  // three keys that produce a capture and the one that dismisses the page. It
  // is only shown when there is genuinely nothing to filter (a type/query empty
  // state is a *search* result, not an onboarding moment).
  if (scoped === 0 && view === "all") {
    const keys = document.createElement("div");
    keys.className = "clipboard-panel__empty-keys";
    for (const [combo, label] of [
      ["⌘C", t("clipboard.emptyHint")],
      ["⌘⇧V", t("system.clipboardHistorySubtitle")],
    ] as const) {
      const chip = document.createElement("span");
      chip.className = "clipboard-panel__key";
      chip.textContent = combo;
      chip.title = label;
      keys.append(chip);
    }
    block.append(keys);
  }
  fragment.append(block);

  const privacy = document.createElement("div");
  privacy.className = "clipboard-panel__empty-privacy";
  privacy.textContent = t("settings.clipboardPrivacy");
  fragment.append(privacy);

  return fragment;
};

/**
 * Loading state: one inline row (spinner + label), never a full-block
 * replacement of the list. It is only reached while `loaded` is false — a
 * refresh that runs on top of existing rows repaints them in place, so the
 * layout never collapses and never flashes.
 */
const renderLoading = (): DocumentFragment => {
  const fragment = document.createDocumentFragment();
  const block = document.createElement("div");
  block.className = "clipboard-panel__loading";
  block.setAttribute("role", "status");
  block.setAttribute("aria-busy", "true");
  const spinner = document.createElement("span");
  spinner.className = "clipboard-panel__spinner";
  spinner.setAttribute("aria-hidden", "true");
  const label = document.createElement("span");
  label.textContent = t("settings.loading");
  block.append(spinner, label);
  fragment.append(block);
  return fragment;
};

/**
 * Failure state (the third of the three): a title, a why, and controls. The
 * retry slot is filled only when retrying can help — with the backend off or
 * unmanaged the button would be a dead end, so the state explains the switch
 * instead and offers only a dismissal. Dismissal is always present, so the
 * user is never trapped in the state, and it clears `loadFailed` exactly the
 * way a successful load would.
 */
const renderLoadFailure = (): DocumentFragment => {
  const fragment = document.createDocumentFragment();
  const failure = document.createElement("div");
  failure.className = "clipboard-panel__empty";
  failure.setAttribute("role", "alert");
  failure.dataset.failureState = String(backendUnavailable);

  const heading = document.createElement("div");
  heading.className = "clipboard-panel__empty-title";
  heading.textContent = t(
    backendUnavailable ? "clipboard.pageUnavailable" : "clipboard.loadFailed",
  );
  failure.append(heading);

  if (backendUnavailable) {
    const hint = document.createElement("div");
    hint.className = "clipboard-panel__empty-hint";
    hint.textContent = t("clipboard.pageUnavailableHint");
    failure.append(hint);
  }

  const actions = document.createElement("div");
  actions.className = "clipboard-panel__empty-actions";
  if (!backendUnavailable) {
    const retry = document.createElement("button");
    retry.type = "button";
    retry.className = "clipboard-panel__clear";
    retry.textContent = t("clipboard.retry");
    retry.addEventListener("mousedown", (event) => event.preventDefault());
    retry.addEventListener("click", () => {
      // Deliberately never `retry.disabled = true`: the failure node is reused
      // across repaints ([`reconcileList`]), so a flag set here would outlive
      // the click and lock the button out forever when the retry fails too.
      // Re-entrancy is already handled by `reload()`'s in-flight dedupe.
      void reload().then(() => {
        render();
        searchInput.focus();
      });
    });
    actions.append(retry);
  }
  const dismiss = document.createElement("button");
  dismiss.type = "button";
  dismiss.className = "clipboard-panel__clear";
  dismiss.textContent = t("clipboard.dismiss");
  dismiss.addEventListener("mousedown", (event) => event.preventDefault());
  dismiss.addEventListener("click", () => {
    // The next successful reload clears the flag for real; dismissing only
    // stops the state from being painted, it does not fake a successful load.
    loadFailed = false;
    render();
    searchInput.focus();
  });
  actions.append(dismiss);
  failure.append(actions);
  fragment.append(failure);
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
 * within one pass; rows whose age has not changed keep a zero-DOM-write pass.
 *
 * GLASS-CLIP-2 adds the two states the rewritten row paints that the old one
 * did not: `copied` (the in-place copy confirmation, which swaps one glyph) and
 * the derived *type* (a colour row's swatch and a link row's host are functions
 * of the entry's text, so a caption edit on an immutable id/hash is impossible
 * — but the type is cheap to include and keeps the key honest about what the
 * row shows). */
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
    clipboardEntryType(entry),
    copiedId === entry.id ? 1 : 0,
    missing ? t("clipboard.missing") : formatAgeSentence(entry, now),
  ].join("\u0001");

/** Per-row record of the inputs each node was last painted from, so an
 * unchanged row can skip all DOM work. Weak so discarded rows collect. */
const rowPaintKeys = new WeakMap<HTMLLIElement, string>();
/** The `busy` value the actions were last synced to. A busy flip only toggles
 * `disabled` on every action button, so it is kept out of the row paint key and
 * synced with one cheap pass instead of repainting 200+ rows of content. */
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

  const chip = button.children.item(0) as HTMLElement;
  renderTypeChip(chip, entry, hasThumbnail);

  // The body: a preview line and the sub line (source · age). Both are direct
  // children of `.clipboard-row__body`, which is the grid's middle column.
  const body = button.children.item(1) as HTMLElement;
  const preview = body.children.item(0) as HTMLElement;
  const filesPreview =
    entry.kind === "files" ? formatFilesPreview(entry.paths) : null;
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
  // The full value on the node, so a screen reader and the rare over-long
  // capture still get everything the clamp hides.
  const fullText = entry.kind === "files"
    ? (entry.paths ?? []).join("\n")
    : clipboardPreview(entry, 400);
  if (preview.title !== fullText) preview.title = fullText;

  const sub = body.children.item(1) as HTMLElement;
  const source = sub.querySelector<HTMLElement>(".clipboard-row__source");
  const sourceText = rowSource(entry);
  if (sourceText) {
    if (source) {
      if (source.textContent !== sourceText) source.textContent = sourceText;
    } else {
      const next = document.createElement("span");
      next.className = "clipboard-row__source";
      next.textContent = sourceText;
      sub.prepend(next);
    }
  } else if (source) {
    source.remove();
  }
  const age = sub.querySelector<HTMLElement>(".clipboard-row__age")!;
  const ageClasses = `clipboard-row__age${missing ? " clipboard-row__age--missing" : ""}`;
  if (age.className !== ageClasses) age.className = ageClasses;
  const ageText = missing ? t("clipboard.missing") : formatAgeSentence(entry, now);
  if (age.textContent !== ageText) age.textContent = ageText;

  // The inline actions. The copy action swaps its glyph for a check while its
  // confirmation is showing; the pin action is lit for good when the entry is a
  // favorite. Both are read from state, never from a transition.
  const actions = button.children.item(2) as HTMLElement;
  const copyAction = actions.querySelector<HTMLButtonElement>('[data-action="copy"]')!;
  const pinAction = actions.querySelector<HTMLButtonElement>('[data-action="pin"]')!;
  const deleteAction = actions.querySelector<HTMLButtonElement>('[data-action="delete"]')!;
  const copied = copiedId === entry.id;
  if (copyAction.classList.contains("clipboard-row__action--done") !== copied) {
    copyAction.classList.toggle("clipboard-row__action--done", copied);
  }
  const copyLabel = t(copied ? "clipboard.copied" : "clipboard.actionCopy");
  if (copyAction.getAttribute("aria-label") !== copyLabel) copyAction.setAttribute("aria-label", copyLabel);
  if (copyAction.title !== copyLabel) copyAction.title = copyLabel;
  // The confirmation glyph is a different icon, so the swap is a rebuild — but
  // only on the one row that flipped, which is why it is safe to do here.
  if (copyAction.dataset.icon !== (copied ? "check" : "copy")) {
    copyAction.dataset.icon = copied ? "check" : "copy";
    copyAction.replaceChildren(clipboardIcon(document, copied ? "check" : "copy", 15));
  }
  const pinLabel = t(entry.favorite ? "clipboard.pinOn" : "clipboard.pin");
  const pinClasses = `clipboard-row__action${entry.favorite ? " clipboard-row__action--on" : ""}`;
  if (pinAction.className !== pinClasses) pinAction.className = pinClasses;
  if (pinAction.getAttribute("aria-pressed") !== String(entry.favorite)) {
    pinAction.setAttribute("aria-pressed", String(entry.favorite));
  }
  if (pinAction.getAttribute("aria-label") !== pinLabel) pinAction.setAttribute("aria-label", pinLabel);
  if (pinAction.title !== pinLabel) pinAction.title = pinLabel;
  if (deleteAction.getAttribute("aria-label") !== t("clipboard.actionDelete")) {
    deleteAction.setAttribute("aria-label", t("clipboard.actionDelete"));
  }
  if (deleteAction.title !== t("clipboard.actionDelete")) deleteAction.title = t("clipboard.actionDelete");
  for (const action of [copyAction, pinAction, deleteAction]) {
    if (action.disabled !== busy) action.disabled = busy;
  }

  rowPaintKeys.set(row, rowPaintKey(entry, index, selected, missing, hasThumbnail, now));
};

/** Build one list item with its type chip, body and inline actions. The
 * skeleton is made once here; [`applyRowState`] fills it, so the create and
 * patch paths stay identical. Click/action handlers resolve the live entry by
 * `data-row-id` at event time, so a reused node never acts on a stale object
 * after a reload. */
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

  const chip = document.createElement("span");
  chip.className = "clipboard-row__type";
  chip.setAttribute("aria-hidden", "true");

  const body = document.createElement("span");
  body.className = "clipboard-row__body";
  const preview = document.createElement("span");
  preview.className = "clipboard-row__preview";
  const sub = document.createElement("span");
  sub.className = "clipboard-row__sub";
  const age = document.createElement("span");
  age.className = "clipboard-row__age";
  sub.append(age);
  body.append(preview, sub);

  // The inline action trio. `tabIndex = -1` keeps the row's own Tab behaviour
  // (Tab toggles the favorites view) intact — the actions are reached with the
  // pointer or with the row's own single-key commands, never by tabbing
  // through 200 rows of buttons. They are `aria-hidden` at rest and revealed on
  // hover/focus/selection, so a screen reader never sees a hidden button.
  const actions = document.createElement("span");
  actions.className = "clipboard-row__actions";
  const actionButton = (action: "copy" | "pin" | "delete", icon: ClipboardIconName, label: string) => {
    const control = document.createElement("button");
    control.type = "button";
    control.tabIndex = -1;
    control.dataset.action = action;
    control.dataset.icon = icon;
    control.className = "clipboard-row__action";
    control.setAttribute("aria-label", label);
    control.title = label;
    control.append(clipboardIcon(document, icon, 15));
    return control;
  };
  const copyAction = actionButton("copy", "copy", t("clipboard.actionCopy"));
  const pinAction = actionButton("pin", "pin", t("clipboard.pin"));
  const deleteAction = actionButton("delete", "trash", t("clipboard.actionDelete"));
  actions.append(copyAction, pinAction, deleteAction);

  button.append(chip, body, actions);
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
  copyAction.addEventListener("click", (event) => {
    event.stopPropagation();
    const live = entries.find((current) => current.id === button.dataset.rowId);
    // The inline button is the *stay-open* copy: the row beside it is the
    // act-and-dismiss one, and two affordances for one outcome would be noise.
    void copyEntry(live, false);
  });
  pinAction.addEventListener("click", (event) => {
    event.stopPropagation();
    const live = entries.find((current) => current.id === button.dataset.rowId);
    void toggleFavorite(live);
  });
  deleteAction.addEventListener("click", (event) => {
    event.stopPropagation();
    const live = entries.find((current) => current.id === button.dataset.rowId);
    void removeEntry(live);
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
    // Keep an already-painted failure state in place across repaints (a poll, a
    // keypress): recreating it would drop focus and re-run the entry
    // animation. Only a change of *which* failure (backend off vs transient)
    // rebuilds it.
    const painted = content.querySelector<HTMLElement>("[data-failure-state]");
    if (painted?.dataset.failureState !== String(backendUnavailable)) {
      content.replaceChildren(renderLoadFailure());
    }
    return;
  }

  if (!loaded) {
    // First fetch still in flight: one inline spinner row instead of a blank
    // canvas. Both outcomes below repaint this node, so it never survives past
    // the first answer, and the guard keeps a repaint from restarting the spin.
    if (!content.querySelector(".clipboard-panel__loading")) {
      content.replaceChildren(renderLoading());
    }
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
  const actionFocused = focused instanceof HTMLElement && Boolean(focused.closest(".clipboard-row__actions"));
  const scrollTop = hydrated ? content.scrollTop : savedSession.scrollTop;
  const finish = () => {
    if (rowFocused) {
      const row = content.querySelector<HTMLElement>(`[data-row-index="${selected}"]`);
      // Focus returns to the same *kind* of control it was on: an inline action
      // keeps the keyboard on the action, a bare row keeps it on the row. A
      // blanket focus() on the row would move a keyboard user off the button
      // they were about to press.
      const target = actionFocused
        ? row?.querySelector<HTMLElement>(`.clipboard-row__action[data-action="${focused instanceof HTMLElement ? focused.dataset.action ?? "copy" : "copy"}"]`)
        : row;
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
  filterClear.replaceChildren(clipboardIcon(document, "close", 14));
  clearButton.textContent = t(clearArmed ? "clipboard.clearConfirm" : "clipboard.clear");
  clearButton.dataset.destructiveConfirm = String(clearArmed);
  clearButton.disabled = busy || !entries.some((entry) => !entry.favorite);
  panel.setAttribute("aria-busy", String(busy));
  clearButton.title = t("clipboard.clearTitle");
  clearButton.setAttribute("aria-label", t("clipboard.clearTitle"));
  hints.textContent = [
    t("clipboard.hintNavigate"),
    t("clipboard.hintCopy"),
    t("clipboard.hintPin"),
    t("clipboard.hintDelete"),
  ].join(" · ");

  // ── The scope tabs (全部 / 收藏) ──────────────────────────────────────────
  const favoritesCount = entries.reduce((total, entry) => total + (entry.favorite ? 1 : 0), 0);
  for (const [tab, scope, active, count] of [
    [viewTabs[0], "all", view === "all", entries.length],
    [viewTabs[1], "favorites", view === "favorites", favoritesCount],
  ] as const) {
    if (!tab) continue;
    tab.replaceChildren(
      document.createTextNode(t(scope === "all" ? "clipboard.tabAll" : "clipboard.tabFavorites")),
      countBadge(count),
    );
    tab.classList.toggle("clipboard-panel__type--active", active);
    tab.setAttribute("aria-selected", String(active));
  }
  viewTabs[0]?.parentElement?.setAttribute("aria-label", t("clipboard.title"));

  // ── The type tabs ──────────────────────────────────────────────────────
  // Six tabs, each with its own glyph and count. The counts are of the scoped
  // set, so switching scope re-labels them instead of leaving a count that
  // describes a list the tab would not show.
  const counts = typeCounts();
  const scopedTotal = scopedEntries().length;
  for (const tab of typeTabs) {
    const raw = tab.dataset.type ?? "";
    const type = raw === "" ? null : (raw as ClipboardEntryType);
    const active = typeFilter === type;
    tab.replaceChildren(
      ...(type ? [clipboardIcon(document, CLIPBOARD_TYPE_ICON[type], 13)] : []),
      document.createTextNode(t(CLIPBOARD_TYPE_LABEL[type ?? "all"] as "clipboard.typeAll")),
      countBadge(type ? counts[type] : scopedTotal),
    );
    tab.classList.toggle("clipboard-panel__type--active", active);
    tab.setAttribute("aria-selected", String(active));
  }
  typeTabs[0]?.parentElement?.setAttribute("aria-label", t("clipboard.typeLabel"));

  filterClear.hidden = !filterText;

  const filtered = filteredEntries();
  selected = filtered.length ? Math.min(selected, filtered.length - 1) : 0;

  // One `now` for the whole pass: the paint key's age comparison and every
  // row's age text read the same clock, so they cannot disagree.
  const now = Date.now();
  reconcileList(filtered, now);
  // The tally is the only text that needs to know the filtered count, and it
  // is a live region so a keyboard filter change is announced. It stays empty
  // when nothing is being filtered away — a "200/200" would be noise.
  tally.textContent = filtered.length === entries.length ? "" : `${filtered.length}/${entries.length}`;
  // `busy` only disables the per-row action buttons; sync them in one pass when
  // it flips rather than folding it into every row's paint key.
  if (renderedBusy !== busy) {
    renderedBusy = busy;
    for (const action of content.querySelectorAll<HTMLButtonElement>(".clipboard-row__action")) {
      action.disabled = busy;
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
  // Repaint only this row's chip through the shared row patcher, so the
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
  if (copiedTimer !== null) window.clearTimeout(copiedTimer);
  reloadGen += 1;
  for (const call of pending.values()) {
    window.clearTimeout(call.timer);
    call.reject("Clipboard page closed");
  }
  pending.clear();
  for (const url of thumbnails.values()) URL.revokeObjectURL(url);
  thumbnails.clear();
  // Forget the busy-sync state: a hidden-then-reopened page (the iframe is
  // kept alive, so this page is not torn down) must re-sync every action from
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

/** Whether an entry can render pixels in its type chip at all. */
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

/** Watch the type chip of every candidate row currently in the list; rows created
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
    // Recovery re-arms the failure toast: a load failure that comes back
    // after a good poll is news again, not a repeat of the one already shown.
    loadFailureDeduper.clear("clipboard.loadFailed");
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
    if (entriesChanged || wasFailed || !hydrated || !loaded) render();
    hydrated = true;
    loaded = true;
    saveSession();
  } catch (error) {
    if (gen !== reloadGen) return;
    loadFailed = true;
    loaded = true;
    if (isBackendUnavailable(error)) backendUnavailable = true;
    if (entries.length) {
      // The list already shows content; a background poll failing must not
      // blank it. Feedback goes to the host's toast stack: a dismissed-page
      // retry re-runs this reload. The deduper keeps a persistent outage to
      // one toast per window instead of one per 2s poll.
      if (loadFailureDeduper.allow("clipboard.loadFailed")) {
        // The retry is a user gesture, so it re-arms the window: if the retry
        // itself fails, that failure is an answer to the user's click and is
        // reported, not swallowed by the toast they already dismissed.
        notifyFailure("clipboard.loadFailed", () => {
          loadFailureDeduper.clear("clipboard.loadFailed");
          void reload();
        });
      }
    } else {
      render();
    }
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

// ── The copy action ──────────────────────────────────────────────────────
//
// Two entry points, one implementation, and they differ in exactly one thing:
// whether the page closes afterwards.
//
//   · **row click / Enter** — the fast path. Copy and get out of the way, the
//     same "act and dismiss" the launcher's results use. This is what the
//     page has always done and what `activate` below still does.
//   · **the row's inline copy button** — the deliberate path. Copy and stay, so
//     the user can copy two things in a row, or check the row they picked. A
//     button that did the same thing as clicking the row beside it would be a
//     second affordance for one action; this is a second *outcome* for it.
//
// The deliberate path confirms in place: the copy glyph swaps to a check for a
// beat (`copiedId`). No toast for a one-key action that already happened.

/** How long the in-place "copied" confirmation stays up before the row's copy
 * glyph returns. Long enough to register, short enough not to linger. */
const COPIED_CONFIRM_MS = 1200;

const markCopied = (id: string) => {
  if (copiedTimer !== null) window.clearTimeout(copiedTimer);
  copiedId = id;
  render();
  copiedTimer = window.setTimeout(() => {
    copiedTimer = null;
    copiedId = null;
    render();
  }, COPIED_CONFIRM_MS);
};

const copyEntry = async (entry: ClipboardEntry | undefined, closeAfter: boolean) => {
  if (!entry || busy || statuses[entry.id] === false) return;
  const target = entry;
  busy = true;
  render();
  try {
    await invokeCommand<void>("clipboard_copy_entry", { id: target.id });
  } catch {
    // The clipboard may be held by another app; keep the page open so the
    // user can retry instead of silently losing the action. The toast's retry
    // action re-enters this same function with the same entry.
    notifyFailure("clipboard.copyFailed", () => { void copyEntry(target, closeAfter); });
    return;
  } finally {
    busy = false;
    render();
  }
  if (closeAfter) {
    if (pageVisible && !pageDisposed) requestClose();
    return;
  }
  markCopied(target.id);
};

/** The row-click / Enter path: copy and dismiss. */
const activate = (entry: ClipboardEntry | undefined) => copyEntry(entry, true);

const toggleFavorite = async (entry: ClipboardEntry | undefined) => {
  if (!entry || busy) return;
  const target = entry;
  busy = true;
  reloadGen += 1;
  const nextFavorite = !target.favorite;
  render();
  searchInput.focus();
  try {
    await invokeCommand<void>("clipboard_set_favorite", {
      id: target.id,
      favorite: nextFavorite,
    });
    entries = entries.map((current) => current.id === target.id ? { ...current, favorite: nextFavorite } : current);
  } catch {
    notifyFailure("clipboard.favoriteFailed", () => { void toggleFavorite(target); });
  } finally {
    await reloadPending;
    await reload();
    busy = false;
    render();
  }
};

const removeEntry = async (entry: ClipboardEntry | undefined) => {
  if (!entry || busy) return;
  const target = entry;
  busy = true;
  reloadGen += 1;
  render();
  searchInput.focus();
  try {
    await invokeCommand<void>("clipboard_delete", { id: target.id });
    entries = entries.filter((candidate) => candidate.id !== target.id);
  } catch {
    notifyFailure("clipboard.deleteFailed", () => { void removeEntry(target); });
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
    // Retrying must re-arm the confirmation: the first click only arms, the
    // second click is what actually clears.
    notifyFailure("clipboard.clearFailed", () => { void clearHistory(); });
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

/** Apply a type filter from a keypress, keeping the keyboard where it was. The
 * five-way order itself lives in `clipboard-list.ts`
 * ([`CLIPBOARD_TYPE_FILTER_ORDER`]) so the ←/→ walk and the 3–7 keys share
 * one table with the painted bar. */
const setTypeFilter = (next: ClipboardEntryType | null) => {
  typeFilter = next;
  selected = 0;
  render();
  focusSelectedRow();
};

window.addEventListener("keydown", (event) => {
  // The whole decision is a pure function (see `resolveClipboardKey` in
  // `clipboard-list.ts`); this handler is only the *executor*. Keeping the
  // policy out of the DOM is what lets the node suite drive the rewritten
  // keyboard surface — every branch below has a matching test that calls the
  // resolver directly.
  const action = resolveClipboardKey({
    key: event.key,
    metaKey: event.metaKey,
    ctrlKey: event.ctrlKey,
    altKey: event.altKey,
    repeat: event.repeat,
    isComposing: event.isComposing || composing,
    keyCode: event.keyCode,
    // The page's three focus worlds. `search` and `row` are the two the
    // keyboard model distinguishes; any *other* focused `<button>` (a tab, the
    // clear button) is a control, whose own Enter/Space activation the page
    // must leave alone.
    focus: document.activeElement === searchInput
      ? "search"
      : document.activeElement instanceof HTMLButtonElement
        ? "control"
        : "row",
    clearArmed,
    clearFocused: document.activeElement === clearButton,
  });

  // The catch-all claim: any key the resolver answered with an action (or an
  // explicit `preventDefault`) is stopped in the capture phase, before default
  // focus traversal or caret movement can act on it. A key the resolver let
  // through (`ignore` with no preventDefault) reaches the filter untouched.
  const claimed = action.kind !== "ignore" || action.preventDefault;
  if (claimed) {
    event.preventDefault();
    event.stopPropagation();
  }

  switch (action.kind) {
    case "ignore":
      return;
    case "close":
      requestClose();
      return;
    case "disarm-clear":
      disarmClear();
      searchInput.focus();
      return;
    case "toggle-view":
      view = view === "all" ? "favorites" : "all";
      selected = 0;
      render();
      searchInput.focus();
      return;
    case "cycle-type":
      setTypeFilter(cycleClipboardTypeFilter(typeFilter, action.step));
      return;
    case "set-type":
      setTypeFilter(action.type);
      return;
    case "set-view":
      view = action.view;
      selected = 0;
      render();
      focusSelectedRow();
      return;
    case "move": {
      const length = filteredEntries().length;
      selected = moveClipboardSelection(selected, action.delta, length);
      render();
      focusSelectedRow();
      return;
    }
    case "activate":
      void activate(filteredEntries()[selected]);
      return;
    case "toggle-pin":
      void toggleFavorite(filteredEntries()[selected]);
      searchInput.focus();
      return;
    case "delete":
      void removeEntry(filteredEntries()[selected]);
      searchInput.focus();
      return;
    case "append-char":
      sendCharToFilter(action.char);
      return;
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
// ── The two tab groups ───────────────────────────────────────────────────
//
// Both are wired the same way and both keep focus on the filter field after a
// change: the field is the page's keyboard home, and a click on a tab is a
// *filter* action, not a focus destination. `mousedown` is prevented so the
// click does not first blur the field and flash the caret away.
const wireTabs = <T,>(tabs: HTMLButtonElement[], read: (tab: HTMLButtonElement) => T, apply: (value: T) => void) => {
  for (const tab of tabs) {
    tab.addEventListener("mousedown", (event) => event.preventDefault());
    tab.addEventListener("click", () => {
      apply(read(tab));
      selected = 0;
      render();
      searchInput.focus();
    });
  }
};
wireTabs(viewTabs, (tab) => tab.dataset.view === "favorites" ? "favorites" : "all", (next) => { view = next; });
wireTabs(
  typeTabs,
  (tab) => normalizeClipboardTypeFilter(tab.dataset.type),
  (next) => { typeFilter = next; },
);

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
  // `reloadData` absorbs its own failures (it paints the failure state and
  // raises the host toast), so this is the belt-and-braces path for a future
  // implementation that rethrows instead.
  loaded = true;
  loadFailed = true;
  render();
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
