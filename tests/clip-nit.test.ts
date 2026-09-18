// CLIP-NIT · two UI nits on the clipboard page, from a mac screenshot.
//
//   1. 「筛选」 stacked vertically. The prompt label (`clipboard.prompt` +
//      the `❯` caret) was pinned to an 18px basis — fine for the terminal-era
//      Latin word, fatal for a two-glyph CJK pair, because CJK breaks between
//      ANY two characters. The label now sizes to its text and refuses to
//      wrap, so neither 「筛选」 nor "filter" can become a vertical column.
//
//   2. 「全部 303」 twice. The top-right scope control was a 全部/置顶 tab pair
//      whose first tab duplicated the type bar's own 全部 slot. The scope is
//      now ONE 置顶 toggle (`aria-pressed`), and 全部 exists only on the type
//      axis — where it belongs. The keyboard resolver is untouched: `Tab`,
//      `1` and `2` still drive the same `view` state.
//
// Like the other plugin-page suites this reads the source (the page owns a DOM
// and imports CSS the node runner cannot instantiate). The last test replays
// the exact re-additions the round removed, so the suite cannot pass while
// either regression is live.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import { resolveClipboardKey } from "../src/clipboard-list.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};
const rule = (css: string, selector: string) =>
  rules(css).find((r) => r.selector === selector);
const decl = (body: string, prop: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${prop}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;

const page = async () => stripComments(await read("src/plugins/clipboard/page.css"));

/** The one predicate both the fix test and its mutation lock read: a prompt
 * label is safe when it sizes to its text and cannot wrap. */
const promptIsOneLine = (body: string) =>
  decl(body, "white-space") === "nowrap"
  && decl(body, "width") === null
  && !/^0\s+0\s+\d/.test(decl(body, "flex") ?? "");

// ── 1 · the prompt label cannot stack ─────────────────────────────────────

test("the filter prompt label is one line and sizes to its text", async () => {
  const css = await page();
  const prompt = rule(css, ".clipboard-panel__prompt");
  assert.ok(prompt, "page.css must still define the prompt label");
  // The fix, as one predicate: no wrapping, and no fixed-basis squeeze.
  assert.ok(
    promptIsOneLine(prompt!.body),
    `the prompt label must size to its text and never wrap, got ${JSON.stringify({
      whiteSpace: decl(prompt!.body, "white-space"),
      width: decl(prompt!.body, "width"),
      flex: decl(prompt!.body, "flex"),
    })}`,
  );
  assert.equal(decl(prompt!.body, "white-space"), "nowrap", "the prompt label must not wrap");
  assert.equal(
    decl(prompt!.body, "width"),
    null,
    "the prompt label must size to its content, not to a hardcoded width",
  );
  // The layering the aura depends on survives the fix.
  assert.equal(decl(prompt!.body, "position"), "relative");
  assert.equal(decl(prompt!.body, "z-index"), "1");
});

test("both languages' prompt text is a single short token plus the caret", () => {
  for (const lang of ["en", "zh"] as const) {
    const t = createTranslator(lang);
    const label = `${t("clipboard.prompt")}❯`;
    assert.ok(label.length > 1, `${lang} must render a prompt label`);
    assert.ok(!/\s/.test(t("clipboard.prompt")), `${lang} prompt must be one token, got "${t("clipboard.prompt")}"`);
  }
  // The two-glyph CJK pair is exactly the string that used to stack; pin it so
  // a future "shortening" of the zh label cannot quietly reintroduce a wrap.
  assert.equal(createTranslator("zh")("clipboard.prompt"), "筛选");
});

test("mutation lock: re-pinning the prompt to an 18px basis goes red", async () => {
  const css = await page();
  // The exact regression: drop the nowrap and restore the 18px basis/width the
  // pre-fix rule carried.
  const mutated = css.replace(
    /(\.clipboard-panel__prompt \{[^}]*?)flex: 0 0 auto;\n  align-items: center;\n  white-space: nowrap;/,
    "$1width: 18px;\n  flex: 0 0 18px;\n  align-items: center;",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  const safe = promptIsOneLine;
  const shipped = rule(css, ".clipboard-panel__prompt")!.body;
  const broken = rule(mutated, ".clipboard-panel__prompt")!.body;
  assert.ok(safe(shipped), "the shipped prompt label must pass the one-line predicate");
  assert.ok(!safe(broken), "the 18px-basis mutation must fail the one-line predicate");
});

// ── 2 · the scope control is one toggle, not a 全部/置顶 pair ──────────────

test("the markup carries one scope toggle and no 全部 tab", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  // One button on the scope axis, and it is the 置顶 toggle.
  assert.match(page, /data-axis="scope"/, "the scope axis must exist");
  assert.match(page, /clipboard-panel__scope-toggle[^>]*data-view="favorites"/, "the toggle targets the favorites view");
  assert.match(page, /aria-pressed="false"/, "the toggle must advertise a pressed state");
  // The old pair — a `data-axis="view"` tablist with a 全部 tab — is gone.
  assert.ok(!page.includes('data-axis="view"'), "the 全部/置顶 tab strip must be gone");
  assert.ok(!page.includes('data-view="all"'), "the duplicate 全部 tab must be gone");
  assert.ok(!page.includes("clipboard.tabAll"), "the 全部 scope label must not be rendered");
  assert.ok(!page.includes("clipboard.tabFavorites"), "the old scope label key must not be rendered");
  // …and the new label is.
  assert.ok(page.includes('t("clipboard.scopePinned")'), "the toggle must render the pinned label");
});

test("the toggle's pressed state is the view, repainted every render", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.match(page, /const pinnedView = view === "favorites";/, "the toggle reads the view state");
  assert.match(
    page,
    /scopeToggle\.setAttribute\("aria-pressed",\s*String\(pinnedView\)\)/,
    "aria-pressed must follow the view",
  );
  assert.match(
    page,
    /scopeToggle\.classList\.toggle\("clipboard-panel__type--active",\s*pinnedView\)/,
    "the lit face must follow the view",
  );
  // The click flips the scope exactly like Tab, and keeps focus on the field.
  assert.match(page, /view = view === "all" \? "favorites" : "all";/, "a click must flip the scope");
  assert.match(page, /scopeToggle\.addEventListener\("mousedown"/, "the click must not steal focus first");
});

test("the i18n scope keys are declared in both languages, and 全部 is not among them", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of ["clipboard.scopePinned", "clipboard.scopePinnedLabel"]) {
    const occurrences = i18n.split(`"${key}":`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared for en and zh`);
  }
  for (const gone of ["clipboard.tabAll", "clipboard.tabFavorites"]) {
    assert.ok(!i18n.includes(`"${gone}"`), `${gone} must be retired with the tab pair`);
  }
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  assert.equal(en("clipboard.scopePinned"), "Pinned");
  assert.equal(zh("clipboard.scopePinned"), "置顶");
  assert.notEqual(zh("clipboard.scopePinnedLabel"), en("clipboard.scopePinnedLabel"));
});

test("the scope chip never squeezes — the label is nowrap and the chip cannot shrink", async () => {
  const css = await page();
  const scope = rule(css, ".clipboard-panel__scope");
  const toggle = rule(css, ".clipboard-panel__type.clipboard-panel__scope-toggle");
  assert.ok(scope, "page.css must define the scope chip group");
  assert.ok(
    toggle,
    "the scope toggle must be styled with a compound selector so the shared `.clipboard-panel__type` face cannot outrank it on source order",
  );
  assert.equal(decl(scope!.body, "flex"), "0 0 auto", "the scope chip must not be shrunk by the tabs");
  assert.equal(decl(toggle!.body, "flex"), "0 0 auto", "the scope toggle itself must not shrink");
  // The shared `.clipboard-panel__type` face already carries `white-space:
  // nowrap`; assert it here too, because the scope chip reuses that face.
  const face = rule(css, ".clipboard-panel__type");
  assert.equal(decl(face!.body, "white-space"), "nowrap", "the tab face must stay one line");
});

// ── 3 · the keyboard semantics do not regress ─────────────────────────────

test("Tab and 1/2 still drive the scope through the unchanged resolver", () => {
  assert.deepEqual(
    resolveClipboardKey({ key: "Tab", focus: "row", clearArmed: false }),
    { kind: "toggle-view" },
  );
  assert.deepEqual(
    resolveClipboardKey({ key: "1", focus: "row", clearArmed: false }),
    { kind: "set-view", view: "all" },
  );
  assert.deepEqual(
    resolveClipboardKey({ key: "2", focus: "row", clearArmed: false }),
    { kind: "set-view", view: "favorites" },
  );
  // The keys the round promised not to move, checked in the same breath.
  assert.deepEqual(resolveClipboardKey({ key: "f", focus: "row", clearArmed: false }), { kind: "toggle-pin" });
  assert.deepEqual(resolveClipboardKey({ key: "*", focus: "row", clearArmed: false }), { kind: "toggle-pin" });
  assert.deepEqual(resolveClipboardKey({ key: "p", focus: "row", clearArmed: false }), { kind: "toggle-pin" });
  assert.deepEqual(resolveClipboardKey({ key: "ArrowRight", focus: "row", clearArmed: false }), { kind: "cycle-type", step: 1 });
});

test("mutation lock: re-adding the 全部 scope tab goes red", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  const mutated = page.replace(
    /(<div class="clipboard-panel__tabs clipboard-panel__scope" role="group" data-axis="scope">)/,
    `$1\n        <button type="button" class="clipboard-panel__type" data-view="all"></button>`,
  );
  assert.notEqual(mutated, page, "the mutation must land");
  assert.ok(
    mutated.includes('data-view="all"'),
    "the duplicate 全部 tab predicate must reject a re-added tab",
  );
  assert.ok(!page.includes('data-view="all"'), "the shipped source must not carry one");
});
