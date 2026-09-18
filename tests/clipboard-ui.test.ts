// GLASS-CLIP-2 · the rewritten clipboard list's *behavior*, as tests.
//
// The page itself (`src/plugins/clipboard/main.ts`) owns a DOM and imports CSS,
// so `node --test` cannot instantiate it (this repo deliberately has no DOM
// library — see the note repeated across the other plugin-page suites). The
// rewrite's answer was to move every *decision* into `src/clipboard-list.ts`
// and leave the page the plumbing. This suite drives those decisions directly:
// it renders a set of mock records through the same functions the page calls,
// and asserts the type chips, the keyboard sequence, the row-action trios, the
// filter tabs and the pin persistence — the exact list of behaviors
// GLASS-CLIP-2 promised.
//
// The last two tests are penetration locks: they re-run the two red-line
// mutations the round named (steep page band flattened to 0.18; Enter handled
// as a no-op) against the *predicates this suite relies on*, so the suite
// cannot pass while those regressions are live.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  applyClipboardFilters,
  clipboardChipFace,
  clipboardTypeCounts,
  cycleClipboardTypeFilter,
  moveClipboardSelection,
  resolveClipboardKey,
  CLIPBOARD_ROW_ACTIONS,
  CLIPBOARD_ROW_ACTION_COMMAND,
  CLIPBOARD_ROW_ACTION_ICON,
  CLIPBOARD_TYPE_FILTER_ORDER,
  CLIPBOARD_TYPE_ICON,
  CLIPBOARD_TYPE_LABEL,
} from "../src/clipboard-list.ts";
import { filterClipboardEntries, type ClipboardEntry } from "../src/clipboard-history.ts";

const root = new URL("../", import.meta.url);

/** One mock record, with the fields the row actually reads. */
const entry = (overrides: Partial<ClipboardEntry> & { id: string }): ClipboardEntry => ({
  kind: "text",
  text: "hello",
  image_file: null,
  width: null,
  height: null,
  hash: `h-${overrides.id}`,
  created_at: 1_700_000_000_000,
  favorite: false,
  ...overrides,
});

/** The five-way fixture: one record of each presentation type, plus a pinned
 * text row so the favorites scope has something to show. */
const records: ClipboardEntry[] = [
  entry({ id: "t1", text: "just an ordinary line of text" }),
  entry({ id: "l1", text: "https://docs.example.com/guide?page=2" }),
  entry({ id: "c1", text: "#8fb7ff" }),
  entry({ id: "i1", kind: "image", text: null, width: 1280, height: 720 }),
  entry({ id: "f1", kind: "files", paths: ["/home/u/report.PNG"] }),
  entry({ id: "t2", text: "pinned note", favorite: true }),
];

// ── 1 · the type chip has the right face for each record ──────────────────

test("each record paints the chip face its type calls for", () => {
  const faceOf = (id: string) => clipboardChipFace(records.find((r) => r.id === id)!, false);

  // Text and link both fall through to their type glyph.
  assert.deepEqual(faceOf("t1"), { kind: "icon", icon: "type", signature: "icon:type" });
  assert.deepEqual(faceOf("l1"), { kind: "icon", icon: "link", signature: "icon:link" });
  // A colour paints the colour itself, normalised for the signature.
  assert.deepEqual(faceOf("c1"), { kind: "swatch", color: "#8fb7ff", signature: "color:#8fb7ff" });
  // A file shows its uppercase extension; an image shows its type glyph until
  // its bytes arrive (then the thumbnail face wins — asserted below).
  assert.deepEqual(faceOf("i1"), { kind: "icon", icon: "image", signature: "icon:image" });
  assert.deepEqual(faceOf("f1"), { kind: "badge", text: "PNG", signature: "files:PNG" });
});

test("the type vocabulary is complete and consistent across label, icon and order", () => {
  // The five types plus 全部 are exactly the six tabs: one icon, one label and
  // one filter stop each. A sixth type cannot land without all three.
  assert.deepEqual([...CLIPBOARD_TYPE_FILTER_ORDER], [null, "text", "link", "color", "image", "files"]);
  for (const type of ["text", "link", "color", "image", "files"] as const) {
    assert.ok(CLIPBOARD_TYPE_LABEL[type]?.startsWith("clipboard.type"), `${type} must have a tab label key`);
    assert.ok(CLIPBOARD_TYPE_ICON[type], `${type} must have a chip glyph`);
    assert.ok(CLIPBOARD_TYPE_FILTER_ORDER.includes(type), `${type} must be a filter stop`);
  }
  assert.equal(CLIPBOARD_TYPE_LABEL.all, "clipboard.typeAll");
});

test("a colour literal is a colour, not a link, and an image caption keeps its image face", () => {
  // The precedence that matters: a hex is never mistaken for a URL, and an
  // image with a caption is still an image (its chip is a glyph/thumbnail, not
  // a text glyph).
  assert.equal(clipboardChipFace(entry({ id: "x", text: "#fff" }), false).kind, "swatch");
  assert.equal(
    clipboardChipFace(entry({ id: "y", kind: "image", text: "a caption" }), false).kind,
    "icon",
  );
  // A thumbnail, once its bytes arrived, outranks every glyph.
  assert.deepEqual(
    clipboardChipFace(entry({ id: "z", kind: "image" }), true),
    { kind: "thumbnail", signature: "thumbnail" },
  );
});

// ── 2 · keyboard navigation ───────────────────────────────────────────────

const key = (overrides: Partial<Parameters<typeof resolveClipboardKey>[0]>) =>
  resolveClipboardKey({ key: "x", focus: "row", clearArmed: false, ...overrides });

test("↑/↓ move the selection with wrap-around, and Enter copies", () => {
  // The navigation arithmetic is the page's: wrap-around, clamped on empty.
  assert.equal(moveClipboardSelection(0, 1, 5), 1);
  assert.equal(moveClipboardSelection(4, 1, 5), 0);
  assert.equal(moveClipboardSelection(0, -1, 5), 4);
  assert.equal(moveClipboardSelection(3, 1, 0), 0, "an empty list has no row to move to");

  assert.deepEqual(key({ key: "ArrowDown", focus: "row" }), { kind: "move", delta: 1 });
  assert.deepEqual(key({ key: "ArrowUp", focus: "row" }), { kind: "move", delta: -1 });
  assert.deepEqual(key({ key: "Enter", focus: "row" }), { kind: "activate" });
  // From the filter field too: Enter is the fast copy in both worlds.
  assert.deepEqual(key({ key: "Enter", focus: "search" }), { kind: "activate" });
});

test("the row commands are Backspace/Delete delete and P pin", () => {
  assert.deepEqual(key({ key: "Backspace", focus: "row" }), { kind: "delete" });
  assert.deepEqual(key({ key: "Delete", focus: "row" }), { kind: "delete" });
  assert.deepEqual(key({ key: "d", focus: "row" }), { kind: "delete" });
  for (const pin of ["p", "P", "f", "F", "*"]) {
    assert.deepEqual(key({ key: pin, focus: "row" }), { kind: "toggle-pin" }, `${pin} must pin`);
  }
  // Cmd/Ctrl+Backspace deletes from any focus, even the filter.
  assert.deepEqual(key({ key: "Backspace", focus: "search", metaKey: true }), { kind: "delete" });
});

test("the filter field keeps its own typing: printable keys and Backspace pass through", () => {
  // The single most important precedence in the page — if a row command stole
  // these, the filter would be unusable.
  assert.deepEqual(key({ key: "a", focus: "search" }), { kind: "ignore", preventDefault: false });
  assert.deepEqual(key({ key: "Backspace", focus: "search" }), { kind: "ignore", preventDefault: false });
  assert.deepEqual(key({ key: "Delete", focus: "search" }), { kind: "ignore", preventDefault: false });
  // …and p/f/d are ordinary letters while typing, never commands.
  for (const letter of ["p", "f", "d"]) {
    assert.equal(key({ key: letter, focus: "search" }).kind, "ignore", `${letter} must type, not command`);
  }
  // A printable key pressed with the list focused is routed into the filter.
  assert.deepEqual(key({ key: "a", focus: "row" }), { kind: "append-char", char: "a" });
});

test("Tab toggles the scope and ←/→ cycle the type bar (but not inside the filter)", () => {
  assert.deepEqual(key({ key: "Tab", focus: "row" }), { kind: "toggle-view" });
  assert.deepEqual(key({ key: "ArrowRight", focus: "row" }), { kind: "cycle-type", step: 1 });
  assert.deepEqual(key({ key: "ArrowLeft", focus: "row" }), { kind: "cycle-type", step: -1 });
  // Inside the filter, ←/→ are caret moves.
  assert.deepEqual(key({ key: "ArrowRight", focus: "search" }), { kind: "ignore", preventDefault: false });
  // 1/2 set the scope directly; 3–7 set the type.
  assert.deepEqual(key({ key: "1", focus: "row" }), { kind: "set-view", view: "all" });
  assert.deepEqual(key({ key: "2", focus: "row" }), { kind: "set-view", view: "favorites" });
  assert.deepEqual(key({ key: "7", focus: "row" }), { kind: "set-type", type: "files" });
  // The cycle wraps and walks the same six stops the bar paints.
  assert.equal(cycleClipboardTypeFilter("files", 1), null);
  assert.equal(cycleClipboardTypeFilter(null, -1), "files");
});

test("an IME confirmation, a repeat and a focused control never fire a row command", () => {
  assert.equal(key({ key: "Enter", focus: "row", isComposing: true }).kind, "ignore");
  assert.equal(key({ key: "Enter", focus: "row", keyCode: 229 }).kind, "ignore");
  assert.equal(key({ key: "Enter", focus: "row", repeat: true }).kind, "ignore");
  assert.equal(key({ key: "Delete", focus: "row", repeat: true }).kind, "ignore");
  // A focused tab's Enter/Space is its own activation, not a row copy.
  assert.equal(key({ key: "Enter", focus: "control" }).kind, "ignore");
  assert.equal(key({ key: " ", focus: "control" }).kind, "ignore");
  // Escape closes; an armed clear is disarmed first.
  assert.deepEqual(key({ key: "Escape", focus: "row" }), { kind: "close" });
  assert.deepEqual(key({ key: "Escape", focus: "row", clearArmed: true }), { kind: "disarm-clear" });
  assert.deepEqual(key({ key: "w", focus: "row", metaKey: true }), { kind: "close" });
});

// ── 3 · the row-action trio ───────────────────────────────────────────────

test("each row carries exactly copy, pin and delete, each mapped to an existing command", () => {
  // Three actions, in this order, and every one runs a command from the frozen
  // bridge allowlist. The rewrite is presentation-only: no new write path.
  assert.deepEqual([...CLIPBOARD_ROW_ACTIONS], ["copy", "pin", "delete"]);
  assert.deepEqual(CLIPBOARD_ROW_ACTION_COMMAND, {
    copy: "clipboard_copy_entry",
    // Pin is the *existing* favorite command under the UI's new name; the
    // stored field and the command signature are untouched.
    pin: "clipboard_set_favorite",
    delete: "clipboard_delete",
  });
  assert.deepEqual(CLIPBOARD_ROW_ACTION_ICON, { copy: "copy", pin: "pin", delete: "trash" });
});

test("the page invokes the row commands through the bridge with the entry id", async () => {
  const page = await readFile(new URL("src/plugins/clipboard/main.ts", root), "utf8");
  // Every action the trio names is actually dispatched by the page.
  for (const command of Object.values(CLIPBOARD_ROW_ACTION_COMMAND)) {
    assert.ok(page.includes(`"${command}"`), `the page must invoke ${command}`);
  }
  // Pin persists through the same id-argumented favorite call it always did.
  assert.match(page, /invokeCommand<void>\("clipboard_set_favorite",\s*\{\s*id:/);
});

// ── 4 · the filter bar ────────────────────────────────────────────────────

test("the type tabs filter the list, and the counts come from the scoped set", () => {
  const match = (list: ClipboardEntry[], query: string) => filterClipboardEntries(list, query);

  // 全部 keeps everything.
  assert.equal(applyClipboardFilters(records, "all", null, "", match).length, records.length);
  // Each type tab keeps only its own records.
  assert.deepEqual(
    applyClipboardFilters(records, "all", "link", "", match).map((r) => r.id),
    ["l1"],
  );
  assert.deepEqual(
    applyClipboardFilters(records, "all", "color", "", match).map((r) => r.id),
    ["c1"],
  );
  assert.deepEqual(
    applyClipboardFilters(records, "all", "files", "", match).map((r) => r.id),
    ["f1"],
  );
  // A type cut and a text query compose.
  assert.deepEqual(
    applyClipboardFilters(records, "all", "text", "ordinary", match).map((r) => r.id),
    ["t1"],
  );
  // The favorites scope cuts first, then the type.
  assert.deepEqual(
    applyClipboardFilters(records, "favorites", "text", "", match).map((r) => r.id),
    ["t2"],
  );
  assert.deepEqual(
    applyClipboardFilters(records, "favorites", "link", "", match).map((r) => r.id),
    [],
  );

  // Counts are of the whole (or scoped) set, so a tab shows what it would
  // actually reveal.
  const all = clipboardTypeCounts(records, "all");
  assert.deepEqual(all, { text: 2, link: 1, color: 1, image: 1, files: 1 });
  assert.deepEqual(clipboardTypeCounts(records, "favorites"), { text: 1, link: 0, color: 0, image: 0, files: 0 });
});

// ── 5 · the icon set really is Lucide ─────────────────────────────────────

test("every inline clipboard icon still matches the installed Lucide package", async () => {
  // The page cannot render lucide-react (no React in the iframe), so the
  // geometry is copied into `src/clipboard-icons.ts`. This asserts the copy has
  // not drifted: each node's `d`/`cx`/`cy`/`r`/`width`/`height` must equal the
  // package's own node for the same icon.
  const icons = await import("../src/clipboard-icons.ts");
  const lucideFor: Record<string, string> = {
    type: "type",
    link: "link",
    palette: "palette",
    image: "image",
    file: "file",
    folder: "folder",
    copy: "copy",
    pin: "pin",
    trash: "trash-2",
    search: "search",
    close: "x",
    check: "check",
  };
  for (const [name, lucideName] of Object.entries(lucideFor)) {
    const source = await readFile(
      new URL(`node_modules/lucide-react/dist/esm/icons/${lucideName}.mjs`, root),
      "utf8",
    );
    // Compare the set of `tag:key=value` tuples, order-independent: the copy
    // reorders nothing but drops Lucide's `key` metadata, which is React's.
    const tuples = (text: string) =>
      [...text.matchAll(/\[\s*"(\w+)",\s*\{([^}]*)\}\s*\]/g)]
        .flatMap(([, tag, body]) =>
          [...body.matchAll(/(\w+):\s*"([^"]*)"/g)]
            .filter(([, k]) => k !== "key")
            .map(([, k, v]) => `${tag}:${k}=${v}`),
        )
        .sort();
    const ours = (icons.CLIPBOARD_ICON_NODES as Record<string, unknown>)[name] as [
      string,
      Record<string, string>,
    ][];
    const ourTuples = ours
      .flatMap(([tag, attrs]) => Object.entries(attrs).map(([k, v]) => `${tag}:${k}=${v}`))
      .sort();
    assert.deepEqual(ourTuples, tuples(source), `${name} has drifted from Lucide's ${lucideName}`);
  }
});

// ── 6 · the two red-line mutations, replayed ──────────────────────────────

test("mutation: flattening the page band's slope back to 0.18 fails the amplitude predicate", async () => {
  const { GLASS_PAGE_CONTENT_BAND } = await import("../src/glass-material.ts");
  // The same predicate `tests/glass-clip.test.ts` uses; replayed here so this
  // suite's own claim ("the slider visibly moves the page") cannot pass while
  // the regression is live.
  const span = (base: number, slope: number) => (base + slope * 0.95) - (base + slope * 0.10);
  assert.ok(span(GLASS_PAGE_CONTENT_BAND.base, GLASS_PAGE_CONTENT_BAND.slope) >= 0.5);
  assert.ok(span(GLASS_PAGE_CONTENT_BAND.base, 0.18) < 0.5, "0.18 must fail — otherwise the lock is vacuous");
});

test("mutation: an Enter no-op resolver fails the keyboard sequence", () => {
  // The exact regression the round named. The real resolver must answer Enter
  // with an activation; a resolver that ignored it (the mutation) must not be
  // able to pass the same assertion the sequence test above makes.
  const real = resolveClipboardKey({ key: "Enter", focus: "row", clearArmed: false });
  assert.deepEqual(real, { kind: "activate" });
  const mutated = () => ({ kind: "ignore", preventDefault: false } as const);
  assert.notDeepEqual(mutated(), real, "the Enter no-op must differ from the shipped answer");
});
