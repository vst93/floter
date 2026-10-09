// R161 · two levers, one for the drawer's visual weight and one for the OS's
// own accessibility switches.
//
//   1. The custom-integration editor's declaration/audit fieldsets were a
//      ceremonial form: their own hint says they do not isolate anything, and
//      they were rendered at the weight of the fields the host actually reads,
//      so a brand-new integration opened on them. They now sit behind a
//      default-closed `<details>` ("Advanced"). The guard has to prove three
//      things at once: the disclosure exists and is closed by default, the
//      fieldsets (and the boundary note) are *inside* it, and collapsing does
//      not gate submission — the checkboxes are still rendered and still write
//      to the one form state the parent submits. The real permission review
//      (`LocalInstallDialog` / `PermissionTierList`) must stay untouched.
//
//   2. The OS "Reduce motion" preference: a wildcard duration backstop next to
//      the named block R8 wrote, so a future surface sheet cannot outrun the
//      preference silently. The default path is byte-for-byte the same look —
//      the whole point is that the new rules are pure additions that only a
//      user who flipped the system switch ever sees. (The "Reduce transparency"
//      half of this round was excised in mainline review: WebKit still does not
//      ship that media feature, so the block stayed retired per R104.)
//
// Source-text for the component (Node's type stripping cannot parse JSX, so
// every drawer guard in this repo reads the file), parsed CSS for the sheet.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { decl, ruleFor, rules, stripComments } from "./css.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** Strip JSX/JS comments so prose that merely names a symbol cannot satisfy a
 *  scan (the R79 convention). */
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** Walk an `@media <query>` block by its own braces. */
const mediaBlock = (css: string, query: string) => {
  const start = css.indexOf(`@media ${query}`);
  assert.notEqual(start, -1, `missing @media ${query}`);
  const open = css.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}") {
      depth -= 1;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated @media ${query}`);
};

/** Remove every `@media <query> { … }` block (all occurrences). */
const stripMedia = (css: string, query: string) => {
  let out = css;
  for (;;) {
    const start = out.indexOf(`@media ${query}`);
    if (start === -1) return out;
    const open = out.indexOf("{", start);
    let depth = 0;
    let end = out.length;
    for (let i = open; i < out.length; i += 1) {
      if (out[i] === "{") depth += 1;
      else if (out[i] === "}") {
        depth -= 1;
        if (depth === 0) {
          end = i + 1;
          break;
        }
      }
    }
    out = out.slice(0, start) + out.slice(end);
  }
};

/** The slice of a JSX file between an opening tag and its closing tag. The
 *  drawer has no nested `<details>`, so the first close is the right one. */
const element = (source: string, open: string, close: string) => {
  const start = source.indexOf(open);
  assert.notEqual(start, -1, `missing ${open}`);
  const end = source.indexOf(close, start + open.length);
  assert.notEqual(end, -1, `unterminated ${open}`);
  return source.slice(start, end + close.length);
};

// ── 1 · the declaration/audit fieldsets are behind an Advanced disclosure ──

const drawer = async () => stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));

test("the drawer renders the permission fieldsets inside a default-closed disclosure", async () => {
  const source = await drawer();
  const details = element(source, '<details className="extension-custom-advanced">', "</details>");
  const openingTag = details.slice(0, details.indexOf(">") + 1);
  // `open` would make it expanded by default; the attribute must be absent.
  assert.ok(
    !/\bopen\b/.test(openingTag),
    `the Advanced disclosure must be closed by default, got ${openingTag}`,
  );
  // Both declaration fieldsets and the boundary note live inside it.
  assert.ok(
    details.includes("extension-custom-permission-boundary") &&
      details.includes('t("settings.extensions.permissionBoundary")'),
    "the boundary note must move into the disclosure with the fieldsets",
  );
  assert.ok(
    details.includes("customEnforcedPermissions") && details.includes("customDeclaredPermissions"),
    "both permission fieldsets must live inside the disclosure",
  );
  // The disclosure is rendered unconditionally, not only when permissions exist
  // — an existing integration with declared permissions has to keep the editor.
  assert.ok(
    !/integration\.permissions\.length\s*&&/.test(source),
    "the disclosure must not be gated on a non-empty permission set",
  );
});

test("collapsing does not gate the form's permission payload", async () => {
  const source = await drawer();
  const details = element(source, '<details className="extension-custom-advanced">', "</details>");
  // The checkboxes are rendered inside the disclosure and write straight to the
  // parent's form state — the value the submit handler serializes. `<details>`
  // hides its children without unmounting them, so a collapsed section still
  // submits exactly what an expanded one would.
  assert.match(
    details,
    /permissions:\s*event\.target\.checked\s*\?/,
    "the checkbox handler must keep writing `permissions` to the form state",
  );
  assert.ok(
    !details.includes("open &&") && !details.includes("open ?"),
    "the fieldsets must not be conditionally rendered on the disclosure's open state",
  );
  // The collapse introduces no local state that could shadow the payload.
  assert.ok(
    !/\buseState\b/.test(source),
    "the drawer must stay stateless — the disclosure's open state is the DOM's, not a second copy of the form",
  );
  // The two lists still come from the one shared tier vocabulary.
  assert.ok(
    source.includes("HOST_ENFORCED_PERMISSIONS") && source.includes("permissionTier("),
    "the editor must keep consuming the shared permission tiers",
  );
});

test("the real permission review path is untouched by the collapse", async () => {
  // The collapse is a *visual weight* change in the custom editor only. The
  // third-party install review is where permissions are actually consumed, so
  // its two surfaces must still render the tier list from the same vocabulary.
  const local = stripJsComments(await read("src/extensions/LocalInstallDialog.tsx"));
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.ok(local.includes("PermissionTierList"), "the local install dialog must keep the tier list");
  assert.ok(panel.includes("PermissionTierList"), "the review dialog must keep the tier list");
  const tiers = await read("src/extensions/permission-tiers.ts");
  assert.match(tiers, /HOST_ENFORCED_PERMISSIONS/, "the shared tier vocabulary must stay the source");
});

test("the Advanced disclosure is labelled in both dictionaries", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of ["customAdvanced", "customAdvancedPermissions"]) {
    assert.equal(
      i18n.split(`"settings.extensions.${key}"`).length - 1,
      2,
      `${key} must be declared in both the en and zh dictionaries`,
    );
  }
});

// ── 2 · the OS reduce-motion backstop ─────────────────────────────────────
//
// The reduce-*transparency* half R161 sketched was excised in mainline review:
// caniuse shows WebKit has still never shipped `prefers-reduced-transparency`
// (Safari unsupported through 26.x/TP; R104's bugzilla-175497 premise still
// holds), so that block would be the exact tombstone R104 deleted. The
// reduce-*motion* backstop below is real on every engine the app ships to.

const baseCss = async () => stripComments(await read("src/styles/base.css"));

test("the reduce-motion wildcard collapses every duration", async () => {
  const css = await baseCss();
  // The named block is the first one; the wildcard backstop is the second. The
  // wildcard is what catches the transitions a surface sheet adds later, so it
  // is asserted on its own rule rather than inferred from the named list.
  const blocks = [...css.matchAll(/@media \(prefers-reduced-motion: reduce\)/g)];
  assert.equal(blocks.length, 2, "one named block and one wildcard block");
  const wildcard = mediaBlock(css.slice(blocks[1].index), "(prefers-reduced-motion: reduce)");
  const rule = rules(wildcard).find(({ selector }) => selector.includes("*"));
  assert.ok(rule, "the wildcard block must declare a `*` rule");
  for (const selector of ["*", "*::before", "*::after"]) {
    assert.ok(
      rule!.selector.split(",").map((s) => s.trim()).includes(selector),
      `the wildcard rule must cover ${selector}`,
    );
  }
  assert.match(rule!.body, /animation-duration:\s*0\.01ms\s*!important/);
  assert.match(rule!.body, /transition-duration:\s*0\.01ms\s*!important/);
  assert.match(rule!.body, /animation-iteration-count:\s*1\s*!important/);
});

test("the default path is unchanged: every stop keeps its blur and every shell its filter", async () => {
  const css = await baseCss();
  // The three stops are still 10 / 22 / 28, and the pre-hydration default is
  // still 22 — the new block must not have moved the resting values. The
  // standalone rule is read (not `ruleFor`, which also matches the R161
  // comma-group that *deliberately* zeroes the same token inside the query).
  const soleRule = (selector: string) => {
    const rule = rules(css).find(({ selector: s }) => s === selector);
    assert.ok(rule, `${selector} must be its own rule outside the query`);
    return rule!.body;
  };
  const stepBlurs: Record<string, string> = {};
  for (const step of ["frosted", "regular", "liquid"]) {
    stepBlurs[step] = decl(soleRule(`[data-glass="${step}"]`), "--glass-step-blur")!;
  }
  assert.deepEqual(stepBlurs, { frosted: "10px", regular: "22px", liquid: "28px" });
  assert.equal(decl(soleRule("html:not([data-glass])"), "--glass-step-blur"), "22px");
  // …and each shell still filters with the aliased blur in its own sheet.
  for (const [file, selector, token] of [
    ["launcher.css", ".collapsed-card", "--glass-blur"],
    ["settings.css", ".settings-card", "--glass-blur"],
    ["terminal.css", ".terminal-panel", "--glass-blur-terminal"],
  ] as const) {
    const sheet = stripComments(await read(`src/styles/${file}`));
    const body = ruleFor(sheet, selector);
    assert.match(
      body,
      new RegExp(`backdrop-filter:\\s*blur\\(var\\(${token.replace(/[-]/g, "\\-")}\\)\\)`),
      `${file}: ${selector} must keep its blur outside the OS preference`,
    );
  }
  // The wildcard block is the only `!important` in the sheet, and it is inside
  // the media query, so the default path carries none.
  const outside = stripMedia(css, "(prefers-reduced-motion: reduce)");
  assert.ok(
    !outside.includes("!important"),
    "no `!important` may leak outside the reduce-motion query",
  );
});
