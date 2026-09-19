// R7-7b · the list row's freshness dot.
//
// The drawer (R7-7a) answers "how fresh is this command list" in three facts;
// the list row answers the same question in one mark, because a user scanning
// twenty rows cannot read twenty timelines. The two must never disagree, which
// is why the row projects through the *same* pure module the drawer uses
// (`freshnessOf` → `freshnessDotState`) rather than re-deriving a state from
// whatever fields the list happens to carry.
//
// The three states are the whole vocabulary and each one is a claim:
//
//   synced   "a probe compared two command lists and they matched"
//   changed  "a probe compared two command lists and they did not"
//   unknown  "nothing was compared" — never probed, probed once, in flight,
//            failed, degraded, or a publisher descriptor whose command table is
//            never probed at all
//
// `unknown` is the default and the majority case, so a projection that
// collapses it into `synced` would paint most of the list with confidence
// nobody has. That is the mutation this file exists to catch.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import { freshnessDotState, freshnessOf } from "../src/extensions/freshness.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The body of a declaration, taken by brace matching — the same discipline
 *  `extension-freshness.test.ts` and the R7-3a suites use, so a renamed
 *  neighbour cannot silently widen the slice. */
const declarationBody = (source: string, signature: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const open = source.indexOf("{", at);
  assert.notEqual(open, -1, `declaration without a body: ${signature}`);
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated declaration: ${signature}`);
};

/** The JSX of a component, from its signature to the declaration after it. */
const componentBody = (source: string, signature: string, next: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const end = source.indexOf(next, at + signature.length);
  assert.notEqual(end, -1, `missing the declaration after: ${next}`);
  const body = source.slice(at, end);
  assert.ok(body.includes("return ("), `${signature} must contain a JSX return`);
  return body;
};

/** A JSX fragment bound to a `const`, from its declaration to the next anchor.
 *  Unlike `componentBody` there is no `return` to anchor on — `rowContent` is
 *  a value, not a component — so the caller names the neighbour that ends it. */
const constJsxBody = (source: string, signature: string, next: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const end = source.indexOf(next, at + signature.length);
  assert.notEqual(end, -1, `missing the declaration after: ${next}`);
  const body = source.slice(at, end);
  assert.ok(body.includes("<>"), `${signature} must contain JSX`);
  return body;
};

const rules = (css: string) =>
  [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({
    selector: m[1].trim().replace(/\s+/g, " "),
    body: m[2],
  }));

const ruleFor = (css: string, selector: string) => {
  const found = rules(css).filter((r) => r.selector === selector);
  assert.equal(found.length, 1, `expected exactly one rule for ${selector}`);
  return found[0].body;
};

// The accent-budget vocabulary, mirrored from `tests/accent-budget.test.ts`:
// a *mark* is emphasis (a dot, a progress fill), not a filled action, and the
// budget counts only filled action faces. The dot has to be judged by that
// same vocabulary, not by a rule invented here — otherwise "8px muted dot" is
// just an assertion that it is small, which is not the property we care about.
const ACCENT_FILL =
  /var\(--(accent|accent-tint|accent-tint-hover|accent-wash|glass-raised|glass-raised-hover|ext-bg-selected)\)/;
const MARK = /[-_](?:dot|progress-(?:bar|fill|track))\b/;

const DOT_STATES = ["synced", "changed", "unknown"] as const;

// ── A. the projection: freshness → one of three states ──────────────────────

test("the dot has exactly three states and the default is `unknown`", () => {
  // Never probed at all: no timestamp, no counts. The row knows nothing.
  assert.equal(freshnessDotState(freshnessOf({})), "unknown");
  // A first probe: a timestamp and a count, but no earlier count to compare
  // with. "We have not compared anything" is not "in sync" — this is the same
  // rule `commandDelta` already enforces by returning `unknown`, and the dot
  // must inherit it rather than round it to the reassuring state.
  assert.equal(
    freshnessDotState(freshnessOf({ lastProbeAt: 1_700_000_000, commandCount: 3 })),
    "unknown",
    "a first probe has no comparison, so it cannot claim sync",
  );
  // Two counts that are both absent are also not a comparison.
  assert.equal(
    freshnessDotState(freshnessOf({ lastProbeAt: 1_700_000_000, commandCount: null, previousCommandCount: null })),
    "unknown",
  );
});

test("a completed comparison is the only source of `synced` or `changed`", () => {
  const probed = { lastProbeAt: 1_700_000_000 };
  // Equal counts, real comparison: in sync.
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, commandCount: 4, previousCommandCount: 4 })),
    "synced",
  );
  // A growth and a shrink are the same state: the count moved.
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, commandCount: 6, previousCommandCount: 4 })),
    "changed",
  );
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, commandCount: 4, previousCommandCount: 6 })),
    "changed",
    "a removal is a change, not a quieter kind of sync",
  );
  // Zero is a real count: `0 -> 0` is a comparison that happened.
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, commandCount: 0, previousCommandCount: 0 })),
    "synced",
    "zero is a number, not an absence",
  );
});

test("a state that is not a completed comparison can never read as synced", () => {
  const probed = { lastProbeAt: 1_700_000_000 };
  // In flight: the probe has not finished, so there is nothing to report yet.
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, running: true, commandCount: 4, previousCommandCount: 4 })),
    "unknown",
    "a probe still running has not compared anything yet",
  );
  // The last probe failed. A stale sidecar timestamp must not turn a failure
  // into a confident "in sync" — the row passes `errorCode` for exactly this.
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, errorCode: "probe_timeout", commandCount: 4, previousCommandCount: 4 })),
    "unknown",
  );
  // Degraded (only optional probes failed) is its own word in the drawer and
  // is not a clean comparison here either.
  assert.equal(
    freshnessDotState(freshnessOf({ ...probed, healthStatus: "degraded", commandCount: 4, previousCommandCount: 4 })),
    "unknown",
  );
});

test("the projection adds no state outside the three", () => {
  // Every combination of the projected facts lands in the same closed set. The
  // point is that the type is exhaustively three-valued: a fourth state would
  // mean the CSS and the dictionary are incomplete.
  const inputs = [
    {},
    { lastProbeAt: 1 },
    { lastProbeAt: 1, commandCount: 1 },
    { lastProbeAt: 1, commandCount: 1, previousCommandCount: 1 },
    { lastProbeAt: 1, commandCount: 1, previousCommandCount: 2 },
    { running: true },
    { errorCode: "x" },
    { healthStatus: "degraded" as const, healthCheckedAt: "2026-01-01T00:00:00Z" },
    { healthStatus: "unhealthy" as const, healthCheckedAt: "2026-01-01T00:00:00Z" },
  ];
  for (const input of inputs) {
    const state = freshnessDotState(freshnessOf(input));
    assert.ok(DOT_STATES.includes(state), `${JSON.stringify(input)} → ${state} is not one of the three`);
  }
});

// ── B. the row renders it: presence, class, and no new affordance ───────────

test("the row projects the dot through the same pure module the drawer uses", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  // Mutation lock: re-deriving the state inline (e.g. "has a probe timestamp →
  // synced") is what this rejects. Both calls have to be present.
  assert.match(row, /freshnessDotState\(freshnessOf\(\{/, "the dot must be a projection of the pure module");
  // The fields it may read are the list's own. `errorCode` is load-bearing: a
  // failed probe must not be announced as a clean comparison.
  assert.match(row, /lastProbeAt: extension\.lastProbeAt/, "the dot reads the list's probe timestamp");
  assert.match(row, /errorCode: extension\.lastErrorCode/, "a failed probe must not read as a comparison");
  assert.match(row, /commandCount: extension\.commandCount/, "the dot reads the list's command count");
  assert.match(
    row,
    /previousCommandCount: extension\.previousCommandCount/,
    "the dot reads the previous command count",
  );
});

test("the dot is present on connected rows and its class is driven by the state", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  const content = constJsxBody(row, "const rowContent = (", "\n  return (");
  // Presence is gated on `connected`: a detected row has no probe of any kind,
  // so its dot could only ever say "unknown" — three grey dots and no
  // information. `unknown` still has a real home on a connected row.
  assert.match(
    content,
    /\{extension\.connected && \(\s*<span\s+className=\{`extension-row__dot extension-row__dot--\$\{dotState\}`\}/,
    "the dot must be a connected-row mark whose class comes from the projected state",
  );
  // The class is the state, not a restatement of the inputs.
  assert.match(content, /extension-row__dot--\$\{dotState\}/, "the variant class is the state name");
  // It lives in the title line, after the version — the row's own identity
  // line, not the meta line (which is a wrapping badge soup).
  const titleAt = content.indexOf("extension-row__title");
  const dotAt = content.indexOf("extension-row__dot");
  const metaAt = content.indexOf("extension-row__meta");
  assert.ok(titleAt !== -1 && dotAt !== -1 && metaAt !== -1, "title, dot and meta must all be in the row body");
  assert.ok(titleAt < dotAt && dotAt < metaAt, "the dot belongs to the title line, before the meta line");
});

test("each of the three states has its own CSS rule and its own token mapping", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  // One rule per state: a missing rule is a state with no visual difference,
  // which is the same failure as a wrong mapping.
  for (const state of DOT_STATES) {
    const body = ruleFor(css, `.extension-row__dot--${state}`);
    assert.match(body, /background:\s*var\(--[a-z-]+\)/, `.extension-row__dot--${state} must paint one token`);
  }
  // The three have to *differ* where it matters: only `changed` earns warmth.
  const synced = ruleFor(css, ".extension-row__dot--synced");
  const changed = ruleFor(css, ".extension-row__dot--changed");
  const unknown = ruleFor(css, ".extension-row__dot--unknown");
  assert.match(changed, /var\(--text-warning\)/, "the changed state uses the existing warning token");
  assert.ok(!/--text-warning/.test(synced), "a synced row is not a warning");
  assert.ok(!/--text-warning/.test(unknown), "an unknown row is not a warning");
  assert.notEqual(synced, changed, "the two positive states must look different");
  // No new colour is invented anywhere in the dot family.
  for (const selector of [".extension-row__dot", ...DOT_STATES.map((s) => `.extension-row__dot--${s}`)]) {
    const body = ruleFor(css, selector);
    assert.ok(
      !/#|rgba?\(|hsla?\(|color-mix\(/.test(body),
      `${selector} must use an existing token, not a literal colour`,
    );
  }
});

test("the dot is a static mark: no animation, no transition, no pulse", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  for (const selector of [".extension-row__dot", ...DOT_STATES.map((s) => `.extension-row__dot--${s}`)]) {
    const body = ruleFor(css, selector);
    assert.ok(!/animation|transition|@keyframes|will-change/.test(body), `${selector} must not move on its own`);
  }
  // And it is a fixed 8px circle — "lightweight dot", not a badge that grows
  // with its label.
  const base = ruleFor(css, ".extension-row__dot");
  assert.match(base, /width:\s*8px/);
  assert.match(base, /height:\s*8px/);
  assert.match(base, /border-radius:\s*50%/);
});

test("the dot is a mark, not a budget face (accent-budget vocabulary)", async () => {
  // The shape test the accent-budget suite uses to keep indicators out of the
  // census. If the selector stopped matching MARK it would have to be listed in
  // a view's ledger — i.e. it would be counted as a tinted action.
  assert.ok(MARK.test(".extension-row__dot"), "a dot is one of the exempt indicator shapes");
  assert.ok(MARK.test(".extension-row__dot--changed"), "a state variant is the same shape");
  const css = stripCssComments(await read("src/styles/extensions.css"));
  // None of the dot rules is an accent fill, and none carries an accent-family
  // tinted pane either (the "one solid mark, not a tinted face" judgement the
  // accent-budget suite records for its own dots).
  for (const selector of [".extension-row__dot", ...DOT_STATES.map((s) => `.extension-row__dot--${s}`)]) {
    const body = ruleFor(css, selector);
    assert.ok(!ACCENT_FILL.test(body), `${selector} must not be an accent fill`);
    assert.ok(
      !/var\(--(glass-raised|accent-tint)/.test(body),
      `${selector} is one solid mark, not a tinted face`,
    );
  }
});

// ── C. accessibility: a labelled image, not a focus stop ────────────────────

test("the dot is a labelled image and not a focus target", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  const content = constJsxBody(row, "const rowContent = (", "\n  return (");
  // Mutation lock (aria removed): `role="img"` + `aria-label` is the only
  // non-visual representation of the state. Drop the label and the state
  // becomes invisible to assistive tech; drop the role and it is not
  // announced at all.
  assert.match(content, /role="img"/, "the dot must be exposed as an image");
  assert.match(content, /aria-label=\{dotLabel\}/, "the dot must carry its state as a label");
  assert.match(content, /title=\{dotLabel\}/, "the pointer tooltip must agree with the label");
  // No focus conflict: it is not a button and takes no tab stop, so the row's
  // open button / action buttons / switch keep the focus order they had.
  const dotAt = content.indexOf("extension-row__dot");
  const dotBlock = content.slice(content.lastIndexOf("<span", dotAt), content.indexOf("/>", dotAt));
  assert.ok(!/tabIndex/.test(dotBlock), "the dot must not enter the tab order");
  assert.ok(!/onClick|<button/.test(dotBlock), "the dot is read-only and wires nothing");
  const accessibleName = dotLabelSource(row);
  assert.match(accessibleName, /t\("settings\.extensions\.freshnessDot",\s*\{/, "the label is translated, not a literal");
  assert.match(
    accessibleName,
    /freshnessDot\.\$\{dotState\}/,
    "the label's state word is the same projection the class uses",
  );
});

/** The `dotLabel` declaration, by brace matching from its own statement. */
const dotLabelSource = (row: string) => {
  const at = row.indexOf("const dotLabel =");
  assert.notEqual(at, -1, "the row must compute a dot label");
  const end = row.indexOf(";", at);
  assert.notEqual(end, -1, "the dot label must be a complete statement");
  return row.slice(at, end);
};

test("the state words are translated in both languages and differ per state", () => {
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  const labels = DOT_STATES.map((state) => ({
    state,
    en: en(`settings.extensions.freshnessDot.${state}` as Parameters<typeof en>[0]),
    zh: zh(`settings.extensions.freshnessDot.${state}` as Parameters<typeof zh>[0]),
  }));
  for (const { state, en: english, zh: chinese } of labels) {
    assert.ok(english.length > 0, `${state} must have en text`);
    assert.ok(chinese.length > 0, `${state} must have zh text`);
    assert.notEqual(chinese, english, `${state} must be a real zh translation`);
  }
  // Three states, three distinct words: "changed" reading the same as "synced"
  // would be the text-level version of the mapping bug.
  assert.equal(new Set(labels.map((l) => l.en)).size, 3, "the three states must read differently in en");
  assert.equal(new Set(labels.map((l) => l.zh)).size, 3, "the three states must read differently in zh");
  // The composed label interpolates the state word rather than leaving braces.
  for (const translate of [en, zh]) {
    const text = translate("settings.extensions.freshnessDot", { state: "X" });
    assert.ok(!text.includes("{"), "the dot label must interpolate its state");
    assert.ok(text.includes("X"), "the dot label must carry the state word");
  }
  assert.match(zh("settings.extensions.freshnessDot", { state: "X" }), /[\u4e00-\u9fff]/, "the zh label is Chinese");
});

const DOT_KEYS = [
  "settings.extensions.freshnessDot",
  "settings.extensions.freshnessDot.synced",
  "settings.extensions.freshnessDot.changed",
  "settings.extensions.freshnessDot.unknown",
];

test("every dot key is declared exactly once per language", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of DOT_KEYS) {
    assert.equal(i18n.split(`"${key}":`).length - 1, 2, `${key} must be declared for en and zh`);
  }
});

// ── D. one change, one interruption (the G2 red line) ───────────────────────

test("the list dot reports state; it never announces anything itself", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  // No toast, no notice decision, no notification callback in the row: the
  // one-shot announcement is the drawer/panel's job (R7-7a), and duplicating it
  // per row would be the double-report G2 forbids.
  for (const banned of ["decideDriftNotice", "reprobeNotice", "onNotify", "createReprobeNoticeGate"]) {
    assert.ok(!row.includes(banned), `the row must not carry the announcement path (${banned})`);
  }
});

test("the panel keeps exactly one announcement path and syncs the list from it", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // One announce site, in `handleDriftNotice`. A second `onNotify` carrying the
  // reprobe notice would be a double toast.
  assert.equal(
    panel.split('t("settings.extensions.reprobeNotice"').length - 1,
    1,
    "the drift notice must be announced from exactly one place",
  );
  // Mutation lock: the dot is only as fresh as the list it reads, so the notice
  // frame must write the list row too — otherwise the row keeps saying "in
  // sync" until an unrelated mutation refreshes.
  assert.match(
    panel,
    /setExtensions\(\(current\) => current\.map\(\(entry\) => entry\.id === payload\.extensionId/,
    "the notice frame must update the list row the dot is projected from",
  );
  assert.match(panel, /lastProbeAt: atSeconds/, "the row's probe timestamp is refreshed");
  // Absent counts keep the list's value instead of being zeroed into a fake
  // comparison.
  const sync = declarationBody(panel, "const handleDriftNotice = (payload: OperationProgressPayload) =>");
  assert.match(
    sync,
    /typeof notice\.commandCount === "number" \? notice\.commandCount : entry\.commandCount/,
    "a frame without counts must not fabricate them",
  );
  assert.match(
    sync,
    /typeof notice\.previousCommandCount === "number"/,
    "the previous count is guarded the same way",
  );
});

test("the i18n dictionaries stay balanced overall", async () => {
  const i18n = await read("src/i18n.ts");
  const count = (source: string) => (source.match(/^ {2}"[^"]+":/gm) ?? []).length;
  const en = i18n.slice(i18n.indexOf("const en = {"), i18n.indexOf("export type MessageKey"));
  const zh = i18n.slice(i18n.indexOf("const zh: Record<MessageKey, string> = {"));
  assert.equal(count(en), count(zh), "en and zh must declare the same number of keys");
});
