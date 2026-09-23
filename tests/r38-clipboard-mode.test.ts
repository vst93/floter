// R38 · the clipboard mode's filter chips, its favorite toggle, the row
// thumbnails and the destructive action's move into the config overlay.
//
// The mode's decisions are pure and live in `plugins/clipboard/mode.ts` and
// `launcher.ts`, so most of this suite drives them directly. The pieces that
// are wiring rather than decisions (the star's markup, the thumbnail request's
// visibility gate, the overlay's invoke) are pinned at the source, the same way
// the other plugin suites pin a page that owns a DOM the node runner cannot
// instantiate.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import type { ClipboardEntry } from "../src/clipboard-history.ts";
import { createTranslator } from "../src/i18n.ts";
import {
  CLIPBOARD_FAVORITE_SHORTCUT,
  CLIPBOARD_FILTERS,
  cycleClipboardFilter,
  type ClipboardModeFilter,
} from "../src/launcher.ts";
import { IS_MAC, matchesShortcut } from "../src/shortcuts.ts";
import {
  clipboardEntryMatchesFilter,
  clipboardKindChip,
  clipboardModeRows,
} from "../src/plugins/clipboard/mode.ts";
import {
  CLIPBOARD_CONFIG_SCHEMA,
  configDefaults,
  configValues,
  normalizeConfigValue,
} from "../src/plugins/config-schema.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

const entry = (
  id: string,
  overrides: Partial<ClipboardEntry> = {},
): ClipboardEntry => ({
  id,
  kind: "text",
  text: `note ${id}`,
  hash: `hash-${id}`,
  created_at: 1_700_000_000_000,
  favorite: false,
  ...overrides,
});

// ── A · the six chips ─────────────────────────────────────────────────────

test("the six chips are 全部 / 收藏 / 文字 / 图片 / 链接 / 文件, in that order", () => {
  assert.deepEqual(
    [...CLIPBOARD_FILTERS],
    ["all", "favorites", "text", "image", "link", "files"],
  );
});

test("cycleClipboardFilter wraps at both ends, like the browser filter", () => {
  assert.equal(cycleClipboardFilter("all", 1), "favorites");
  assert.equal(cycleClipboardFilter("files", 1), "all");
  assert.equal(cycleClipboardFilter("all", -1), "files");
  assert.equal(cycleClipboardFilter("image", -1), "text");
  // A full cycle returns to where it started.
  let filter: ClipboardModeFilter = "all";
  for (let step = 0; step < CLIPBOARD_FILTERS.length; step += 1) {
    filter = cycleClipboardFilter(filter, 1);
  }
  assert.equal(filter, "all");
});

// ── B · the kind classification ───────────────────────────────────────────

test("text and link are mutually exclusive, and the link wins", () => {
  const url = entry("url", { text: "https://example.com/a" });
  const bare = entry("bare", { text: "example.com/page" });
  const prose = entry("prose", { text: "see https://example.com for details" });
  const plain = entry("plain", { text: "just some words" });

  assert.equal(clipboardKindChip(url), "link");
  assert.equal(clipboardKindChip(bare), "link", "a bare host is a link too");
  assert.equal(
    clipboardKindChip(prose),
    "text",
    "a sentence that contains a URL is prose, not a link",
  );
  assert.equal(clipboardKindChip(plain), "text");

  // One entry, one kind chip — never both.
  assert.equal(clipboardEntryMatchesFilter(url, "link"), true);
  assert.equal(clipboardEntryMatchesFilter(url, "text"), false);
  assert.equal(clipboardEntryMatchesFilter(plain, "text"), true);
  assert.equal(clipboardEntryMatchesFilter(plain, "link"), false);
});

test("images and file lists keep their stored kind; colours stay 文字", () => {
  const image = entry("img", { kind: "image", text: null, image_file: "a.png" });
  const files = entry("fs", { kind: "files", text: null, paths: ["/a/b"] });
  // A hex literal is `clipboardEntryType`'s `color`, but the chips have no
  // colour face — it is a text entry and belongs under 文字.
  const color = entry("hex", { text: "#ff8800" });

  assert.equal(clipboardKindChip(image), "image");
  assert.equal(clipboardKindChip(files), "files");
  assert.equal(clipboardKindChip(color), "text");

  assert.equal(clipboardEntryMatchesFilter(image, "image"), true);
  assert.equal(clipboardEntryMatchesFilter(image, "files"), false);
  assert.equal(clipboardEntryMatchesFilter(files, "files"), true);
  assert.equal(clipboardEntryMatchesFilter(color, "text"), true);
  assert.equal(clipboardEntryMatchesFilter(color, "link"), false);
});

test("favorites is an orthogonal axis: any kind can be a favorite", () => {
  const favImage = entry("fi", { kind: "image", text: null, favorite: true });
  assert.equal(clipboardEntryMatchesFilter(favImage, "favorites"), true);
  assert.equal(clipboardEntryMatchesFilter(favImage, "image"), true);
  assert.equal(clipboardEntryMatchesFilter(entry("plain"), "favorites"), false);
  assert.equal(clipboardEntryMatchesFilter(entry("plain"), "all"), true);
});

test("the four kind chips partition the history; favorites overlap", () => {
  const entries = [
    entry("t1"),
    entry("t2"),
    entry("l1", { text: "https://a.example" }),
    entry("i1", { kind: "image", text: null, favorite: true }),
    entry("f1", { kind: "files", text: null, paths: ["/a"] }),
  ];
  const count = (filter: ClipboardModeFilter) =>
    entries.filter((item) => clipboardEntryMatchesFilter(item, filter)).length;

  assert.equal(count("all"), 5);
  assert.equal(count("favorites"), 1);
  // Every entry belongs to exactly one kind chip, so the four counts partition
  // the history; favorites is an orthogonal axis and deliberately overlaps.
  assert.equal(count("text") + count("image") + count("link") + count("files"), 5);
  assert.deepEqual(
    { text: count("text"), link: count("link"), image: count("image"), files: count("files") },
    { text: 2, link: 1, image: 1, files: 1 },
  );
  // The rows each chip produces agree with the count above it.
  const rows = (filter: ClipboardModeFilter) =>
    clipboardModeRows(entries, { needle: "", filter }, en, 1_700_000_000_000).length;
  assert.equal(rows("link"), count("link"));
  assert.equal(rows("image"), count("image"));
  assert.equal(rows("favorites"), count("favorites"));
});

// ── C · the rows each chip produces ───────────────────────────────────────

test("clipboardModeRows applies the chip after the needle, in memory", () => {
  const entries = [
    entry("t1", { text: "rust book" }),
    entry("l1", { text: "https://rust-lang.org" }),
    entry("i1", { kind: "image", text: null, favorite: true }),
    entry("f1", { kind: "files", text: null, paths: ["/a/rust"] }),
  ];
  const ids = (filter: ClipboardModeFilter, needle = "") =>
    clipboardModeRows(entries, { needle, filter }, en, 1_700_000_000_000).map((row) => row.id);

  assert.deepEqual(ids("all"), ["t1", "l1", "i1", "f1"]);
  assert.deepEqual(ids("favorites"), ["i1"]);
  assert.deepEqual(ids("text"), ["t1"]);
  assert.deepEqual(ids("link"), ["l1"]);
  assert.deepEqual(ids("image"), ["i1"]);
  assert.deepEqual(ids("files"), ["f1"]);
  // Both cuts compose: the needle narrows, the chip selects.
  assert.deepEqual(ids("link", "rust"), ["l1"]);
  assert.deepEqual(ids("text", "rust"), ["t1"]);
});

test("an empty favorites chip is its own sentence, not 'nothing copied yet'", () => {
  const rows = clipboardModeRows(
    [entry("a")],
    { needle: "", filter: "favorites" },
    en,
    1_700_000_000_000,
  );
  assert.equal(rows.length, 1);
  assert.equal(rows[0].kind, "status");
  assert.equal(rows[0].disabled, true);
  assert.equal(rows[0].title, en("clipboard.emptyFavorites"));
  assert.notEqual(rows[0].title, en("clipboard.empty"));
  // A needle that matches nothing still reports the search, not the chip.
  const searched = clipboardModeRows(
    [entry("a")],
    { needle: "zzz", filter: "favorites" },
    en,
    1_700_000_000_000,
  );
  assert.equal(searched[0].title, en("clipboard.emptyFilter"));
});

test("a colour literal's row says 文字, matching its chip and its icon", () => {
  // The chips have no colour face, so a colour literal is a `text` entry; the
  // row's own type word folds the same way, so the word, the icon and the chip
  // that reveals the row are one vocabulary (the retired page's 颜色 label is
  // not resurrected on the launcher).
  const rows = clipboardModeRows(
    [entry("hex", { text: "#ff8800" })],
    { needle: "", filter: "all" },
    en,
    1_700_000_000_000,
  );
  const row = rows[0];
  assert.equal(row.family, "clipboard");
  assert.ok(
    row.family === "clipboard" && row.subtitle?.includes(en("clipboard.typeText")),
    "a colour's row type word is 文字, not 颜色",
  );
  assert.equal(clipboardKindChip(entry("hex", { text: "#ff8800" })), "text");
});

// ── D · the favorite key ──────────────────────────────────────────────────

const keyEvent = (init: Partial<KeyboardEvent> & { key: string }) =>
  ({
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    shiftKey: false,
    code: `Key${init.key.toUpperCase()}`,
    ...init,
  }) as KeyboardEvent;

test("the favorite key is ⌘D / Ctrl+D, and nothing else", () => {
  assert.equal(CLIPBOARD_FAVORITE_SHORTCUT, "CmdOrCtrl+D");
  // `CmdOrCtrl` resolves to ⌘ on macOS and Ctrl everywhere else, exactly as a
  // stored shortcut would; the *other* modifier must not match on either.
  const primary = IS_MAC ? { metaKey: true } : { ctrlKey: true };
  const other = IS_MAC ? { ctrlKey: true } : { metaKey: true };
  assert.ok(matchesShortcut(keyEvent({ key: "d", ...primary }), CLIPBOARD_FAVORITE_SHORTCUT));
  assert.equal(matchesShortcut(keyEvent({ key: "d", ...other }), CLIPBOARD_FAVORITE_SHORTCUT), false);
  // Bare `d` is typing; Shift+⌘D is a different binding.
  assert.equal(matchesShortcut(keyEvent({ key: "d" }), CLIPBOARD_FAVORITE_SHORTCUT), false);
  assert.equal(
    matchesShortcut(keyEvent({ key: "d", ...primary, shiftKey: true }), CLIPBOARD_FAVORITE_SHORTCUT),
    false,
  );
});

// ── E · the config overlay's destructive action ───────────────────────────

test("the clipboard schema offers 'clear history' as an action, not a value", () => {
  const action = CLIPBOARD_CONFIG_SCHEMA.fields.find((field) => field.type === "action");
  assert.ok(action, "the clipboard schema must carry the destructive action");
  assert.equal(action!.key, "clear_history");
  assert.equal(action!.type === "action" ? action!.command : null, "clipboard_clear_history");
  // The label/confirm/cancel/failure wording all exist in both dictionaries.
  const keys = [action!.labelKey, action!.confirmKey];
  if (action!.type === "action") keys.push(action!.cancelKey, action!.failedKey);
  for (const key of keys) {
    assert.equal(typeof en(key), "string");
    assert.equal(typeof zh(key), "string");
    assert.ok(en(key).length > 0 && zh(key).length > 0, `${key} must be translated`);
  }
});

test("an action field holds no value and never round-trips", () => {
  const action = CLIPBOARD_CONFIG_SCHEMA.fields.find((field) => field.type === "action")!;
  assert.equal(normalizeConfigValue(action, "anything"), null);
  const values = configValues(CLIPBOARD_CONFIG_SCHEMA, { enabled: true, max_items: 100 });
  assert.equal(values[action.key], null);
  assert.equal(configDefaults(CLIPBOARD_CONFIG_SCHEMA)[action.key], null);
  // The capacity field is untouched by the action's arrival.
  assert.equal(values.max_items, 100);
});

test("the overlay invokes the action's command and tells the launcher it ran", async () => {
  const overlay = stripJsComments(await read("src/plugins/PluginConfigOverlay.tsx"));
  assert.match(overlay, /field\.type !== "action"\) return false/);
  assert.match(overlay, /await invoke\(field\.command\)/);
  assert.match(overlay, /onActionComplete\?\.\(key\)/);

  const controls = stripJsComments(await read("src/plugins/controls.tsx"));
  // Two-step, in the overlay: the first press arms, the second runs. No dialog.
  assert.match(controls, /setArmed\(true\)/);
  assert.match(controls, /onRun\?\.\(field\.key\)/);
  assert.doesNotMatch(controls, /confirm\(/, "the confirmation is in the overlay, not a system dialog");

  // The App refetches the mode's entries after the action, so the list behind
  // the overlay cannot keep showing rows the user just deleted.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /onActionComplete=\{reloadClipboardEntries\}/);
});

// ── F · the wiring, pinned at the source ──────────────────────────────────

test("the clipboard mode's chips reuse the R32 launcher-filter row", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /filterRowVisible && launcherScope === "clipboard" && \(\s*<div className="launcher-filter">/);
  assert.match(app, /\{CLIPBOARD_FILTERS\.map\(\(filter\) => \(/);
  // The active chip is the mode's own `filter` — one source of truth.
  assert.match(app, /aria-selected=\{clipboardFilter === filter\}/);
  assert.match(app, /launcher-filter__chip--active/);
  // The subline is fixed chrome, so the window height charges it for the
  // clipboard scope exactly as R32 does for the browser — six chips can never
  // resize the window as the list under them filters. R41 · the charge follows
  // the row's visibility, so the overlay no longer reserves the chips' band.
  assert.match(app, /filterRowVisible,/);
});

test("the favorite toggle is one path: star click and ⌘D call the same handler", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(catalog, /invoke\("clipboard_set_favorite", \{ id, favorite: next \}\)/);
  // Optimistic: the paint happens before the write, and the write's failure
  // rolls the one row back.
  assert.match(catalog, /paint\(next\);/);
  assert.match(catalog, /paint\(current\.favorite\);/);
  assert.match(catalog, /showLauncherFeedback\("clipboard\.favoriteFailed"\)/);
  // Un-favoriting hands the entry back to the retention policy, which can prune
  // it in the same write; the hook refetches then so the row cannot outlive the
  // entry. Favoriting never prunes, so it does not.
  assert.match(catalog, /if \(!next\) reloadClipboardEntries\(\);/);

  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /className=\{`launcher-result__favorite/);
  assert.match(results, /onToggleClipboardFavorite\(favoriteEntry\.id\)/);
  assert.match(results, /favoriteEntry\.favorite \? "currentColor" : "none"/);

  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(actions, /matchesShortcut\(event, CLIPBOARD_FAVORITE_SHORTCUT\)/);
  assert.match(actions, /toggleClipboardFavorite\(selected\.entry\.id\)/);
  // Tab cycles the chips before the numbered family, the same order the browser
  // mode established in R32.
  assert.match(actions, /clipboardScope && event\.key === "Tab"/);
  assert.match(actions, /cycleClipboardFilter\(event\.shiftKey \? -1 : 1\)/);
});

test("thumbnails are asked for only for visible image rows, and only once", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  // The R34 viewport is the gate: a row outside it carries a null slot.
  assert.match(
    results,
    /if \(resultShortcutSlots\[index\] === null\) return;/,
    "off-viewport rows must not request a thumbnail",
  );
  assert.match(results, /invoke<string>\("clipboard_thumbnail", \{ id, size: 32 \}\)/);
  // The session memo: an id already fetched or in flight is skipped, and the
  // batch merges in one write.
  assert.match(results, /thumbnailPending\.current\.has\(id\)/);
  assert.match(results, /setClipboardThumbnails\(\(previous\) => \(\{ \.\.\.previous, \.\.\.next \}\)\)/);
  // The payload is a data URL, never the full image: the row reads the small
  // string, and the full-resolution read command is not called from the list.
  assert.doesNotMatch(results, /clipboard_read_image/);

  const rust = await read("src-tauri/src/clipboard_history/mod.rs");
  assert.match(rust, /data:image\/png;base64,/);
  assert.match(rust, /monitor::downscale_rgba/);
});

test("each clipboard kind gets its own glyph; an image gets its thumbnail", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /const ClipboardRowIcon = \(/);
  assert.match(results, /chip === "link"\) return <LinkIcon/);
  assert.match(results, /chip === "files"\) return <FolderIcon/);
  assert.match(results, /<TypeIcon size=\{16\} strokeWidth=\{2\}/);
  assert.match(results, /<ImageIcon size=\{16\} strokeWidth=\{2\}/);
  // A thumbnail replaces the image glyph once the bytes arrive.
  assert.match(results, /return thumbnail \? \(\s*<img src=\{thumbnail\}/);
});

test("the chips row is six, and its hint names both mode keys in both languages", () => {
  for (const t of [en, zh]) {
    for (const filter of CLIPBOARD_FILTERS) {
      const key = filter === "all"
        ? "launcher.clipboardFilterAll"
        : filter === "favorites"
          ? "launcher.clipboardFilterFavorites"
          : filter === "text"
            ? "clipboard.typeText"
            : filter === "image"
              ? "clipboard.typeImage"
              : filter === "link"
                ? "clipboard.typeLink"
                : "clipboard.typeFiles";
      assert.ok(t(key).length > 0, `${key} must be translated`);
    }
    assert.ok(t("launcher.clipboardFilter").length > 0);
    assert.ok(t("launcher.clipboardFavorite").length > 0);
    assert.ok(t("launcher.clipboardUnfavorite").length > 0);
    const hint = t("launcher.clipboardFilterHint", { shortcut: "⌘D" });
    assert.ok(hint.includes("⌘D"), "the hint names the favorite key");
    assert.ok(!hint.includes("{shortcut}"), "the placeholder is interpolated");
  }
});
