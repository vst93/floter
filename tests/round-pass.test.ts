// ROUND-PASS: the control-shape round.
//
// The user's verdict was 「按钮组件等可以更圆润，贴近苹果风格」. The round is
// deliberately *not* "add 4px everywhere": it is a two-layer shape language.
//
//   1. The ladder moves up (4/6/8/10 → 6/9/12/14) and keeps its four jobs and
//      its concentric arithmetic, so all 55 `var(--radius-*)` call sites get
//      the new shape without one of them being edited.
//   2. An independent control that floats on its own — a button, a segmented
//      control, a boxed field, a switch track — becomes a *pill* (999px), which
//      the ladder cannot express because a pill is a stadium, not a corner.
//
// The line between the two is the whole design: **a control that floats takes
// the pill; anything nested in a container follows the container.** These tests
// pin both halves — the pill list, the exception list, and the token values —
// because the failure mode of a shape pass is silent: one selector keeps an old
// radius and nothing looks broken enough to notice.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

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

const radius = (body: string) => {
  const match = body.match(/(?:^|;)\s*border-radius\s*:\s*([^;]+)/);
  return match ? match[1].trim() : null;
};

/** Every rule body in one host sheet that declares the given selector. */
const bodiesFor = async (file: string, selector: string) => {
  const css = stripComments(await read(file));
  return rules(css).filter((rule) => rule.selector === selector).map((rule) => rule.body);
};

const radiusFor = async (file: string, selector: string) => {
  const bodies = await bodiesFor(file, selector);
  assert.ok(bodies.length > 0, `${file} must define ${selector}`);
  // The last declaration wins in the cascade; a rule that only *overrides* the
  // radius (like the multi-select list box) is still the answer.
  const values = bodies.map(radius).filter((v): v is string => v !== null);
  assert.ok(values.length > 0, `${file}: ${selector} must declare a border-radius`);
  return values[values.length - 1];
};

// ── A. the ladder ─────────────────────────────────────────────────────────

test("the ROUND-PASS ladder is 6/9/12/14 and the window is its top step", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const rootBlock = base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
  const token = (name: string) => {
    const match = rootBlock.match(new RegExp(`--${name}:\\s*([^;]+);`));
    assert.ok(match, `--${name} must be defined in the dark token block`);
    return match![1].trim();
  };
  assert.equal(token("radius-xs"), "6px");
  assert.equal(token("radius-sm"), "9px");
  assert.equal(token("radius-md"), "12px");
  assert.equal(token("radius-lg"), "14px");
  // The window is the ladder's top step, not a tenth literal — the same
  // relation R7-HIG established, kept through the raise.
  assert.equal(token("window-radius"), token("radius-lg"), "--window-radius must be --radius-lg");
  // Ascending, so outer = inner + padding is still expressible.
  const px = ["radius-xs", "radius-sm", "radius-md", "radius-lg"].map((n) => Number(token(n).replace("px", "")));
  for (let i = 1; i < px.length; i += 1) {
    assert.ok(px[i] > px[i - 1], `the ladder must ascend: ${px[i - 1]} -> ${px[i]}`);
  }
});

test("every radius consumer follows the raised ladder without a literal", async () => {
  // The point of the token ladder: the raise reached the call sites without
  // editing one. Assert the mechanism (a var, not a number) across the host
  // sheets so a later round cannot pin one control back to a literal.
  for (const dir of ["src/styles/", "src/extensions/"]) {
    for (const name of (await readdir(new URL(dir, root))).filter((n) => n.endsWith(".css"))) {
      const file = `${dir}${name}`;
      if (file === "src/extensions/ComponentizedUninstallDialog.css") continue;
      for (const { selector, body } of rules(stripComments(await read(file)))) {
        const value = radius(body);
        if (value === null) continue;
        if (/^(0|50%|999px|inherit|none)$/.test(value)) continue;
        assert.match(
          value,
          /var\(--(radius|window-radius|ext-radius)/,
          `${file}: ${selector} border-radius: ${value} — a radius is either a ladder token or a pill`,
        );
      }
    }
  }
});

// ── B. the pill list ──────────────────────────────────────────────────────

test("every floating control takes the pill", async () => {
  // Each entry is a control that stands on its own (it is not nested in the
  // content flow of a list), so it is a stadium. Naming the selector rather
  // than scanning for "buttons" is deliberate: the exceptions below are just
  // as much a part of the design as this list.
  const PILLS: [string, string][] = [
    // Segmented controls: the track and the chosen slot are one stadium in
    // another. This covers the theme/language/cursor/glass-intensity pickers
    // (all four share `.settings-options--inline`) and the clipboard tabs.
    ["src/styles/settings.css", ".settings-options--inline"],
    ["src/styles/settings.css", ".settings-options--inline .settings-option"],
    ["src/styles/extensions.css", ".extension-custom-mode"],
    ["src/styles/extensions.css", ".extension-custom-mode__item"],
    ["src/styles/terminal.css", ".clipboard-panel__tabs"],
    ["src/styles/terminal.css", ".clipboard-panel__tab"],
    // Action buttons.
    ["src/styles/settings.css", ".settings-save-alert button"],
    ["src/styles/settings.css", ".update-banner__button"],
    ["src/styles/settings.css", ".session-manager__kill-confirm"],
    ["src/styles/extensions.css", ".extensions-action-button"],
    // Boxed single-line fields.
    ["src/styles/settings.css", ".terminal-setting-control select"],
    ["src/styles/extensions.css", ".extension-custom-form input, .extension-custom-form select"],
    ["src/styles/extensions.css", ".extension-config-field input, .extension-config-field select"],
    // The switch track: a 20px groove with a circular thumb. The old
    // `--radius-lg` (10px, now 14px) still left shoulders on a 20px track.
    ["src/styles/settings.css", ".settings-switch"],
  ];
  for (const [file, selector] of PILLS) {
    assert.equal(await radiusFor(file, selector), "999px", `${file}: ${selector} must be a pill`);
  }
});

test("the switch track is a pill and no longer explains itself with a ladder step", async () => {
  const settings = stripComments(await read("src/styles/settings.css"));
  const track = rules(settings).find((rule) => rule.selector === ".settings-switch");
  assert.ok(track, "settings.css must define .settings-switch");
  assert.equal(radius(track!.body), "999px");
  // The old "closest step" excuse lived in the base.css scrollbar comment
  // (never in settings.css — NIT-2). Guard both: the scrollbar comment must
  // not re-introduce it, and the switch block must never grow a new one.
  const raw = await read("src/styles/settings.css");
  const block = raw.slice(raw.indexOf("/* The switch track"), raw.indexOf(".settings-switch {"));
  assert.ok(!/closest step/.test(block), "the \"closest step\" excuse must be gone");
  const base = await read("src/styles/base.css");
  const sb = base.slice(base.indexOf("::-webkit-scrollbar-thumb"));
  if (/closest step/.test(sb)) {
    // The phrase may survive only as history (explaining the removal), never
    // as a live justification for the current radius.
    assert.ok(
      /used to excuse/.test(sb),
      "the scrollbar may mention \"closest step\" only as removed history",
    );
  }
  // The thumb stays a circle inside it.
  const thumb = rules(settings).find((rule) => rule.selector === ".settings-switch__thumb");
  assert.ok(thumb, "settings.css must define .settings-switch__thumb");
  assert.equal(radius(thumb!.body), "50%", "the switch thumb stays a circle");
});

// ── C. the exception list ─────────────────────────────────────────────────

test("nested and inline shapes keep the ladder instead of the pill", async () => {
  // The other half of the shape language. Each entry has a reason; the test
  // records the reason so a later round cannot "finish the job" by pill-ing
  // them and calling it consistency.
  const LADDER: [string, string, RegExp, string][] = [
    // Table rows: a corner on a row that touches its neighbours reads as a
    // floating card that forgot its shadow.
    ["src/styles/settings.css", ".settings-option--static", /^0$/, "table rows stay square"],
    // Content rows that scroll with their list.
    ["src/styles/launcher.css", ".launcher-result", /var\(--radius-md\)/, "result rows are content"],
    ["src/styles/launcher.css", ".launcher-action-bar", /var\(--radius-md\)/, "the action bar welds to the list above it"],
    // Message surfaces, not controls.
    ["src/styles/launcher.css", ".launcher-tip", /var\(--radius-sm\)/, "a dismissible callout is a message"],
    // Under the 28px pill threshold.
    ["src/styles/extensions.css", ".extension-discard-bar .extensions-action-button", /var\(--radius-xs\)/, "24px inline pair inside a notice bar"],
    ["src/styles/settings.css", ".shortcut-recorder", /var\(--radius-sm\)/, "27px recorder key sitting in a table row"],
    ["src/styles/settings.css", ".settings-copy-button", /var\(--radius-sm\)/, "22px icon button"],
    ["src/styles/terminal.css", ".plugin-page-host__button", /var\(--radius-sm\)/, "~23px retry affordance inside a panel"],
    ["src/styles/terminal.css", ".clipboard-panel__clear", /var\(--radius-sm\)/, "~22px inline footer action"],
    // Transparent icon button inside a bar: a pill would round nothing visible.
    ["src/styles/terminal.css", ".toolbar-button", /var\(--radius-sm\)/, "transparent icon button in the terminal bar"],
    // Multi-line boxes and panels are containers, not fields.
    ["src/styles/extensions.css", ".extension-config-field select[multiple]", /var\(--radius-sm\)/, "a multi-select is a list box"],
    ["src/styles/extensions.css", ".extension-custom-form textarea", /var\(--ext-radius\)/, "a textarea is a content box"],
    ["src/styles/extensions.css", ".extension-permission-tier", /var\(--ext-radius\)/, "a permission tier is a panel"],
  ];
  for (const [file, selector, expected, reason] of LADDER) {
    assert.match(await radiusFor(file, selector), expected, `${file}: ${selector} — ${reason}`);
  }
  // The multi-select override must actually override the pill on the shared
  // rule, or the list box would be a 66px-tall stadium.
  const multi = await radiusFor("src/styles/extensions.css", ".extension-config-field select[multiple]");
  assert.notEqual(multi, "999px", "the multi-select list box must not inherit the field pill");
});

test("the zero radius in settings.css is deliberate, not leftover drift", async () => {
  const raw = await read("src/styles/settings.css");
  // `.settings-option--static` is the shortcut table's row. Its `0` is one of
  // the three literals the radius scan allows (0, 50%, 999px) and it is
  // documented at the rule, so it reads as a decision rather than as the
  // residue of an unfinished pass.
  const index = raw.indexOf(".settings-option--static {");
  assert.notEqual(index, -1, "settings.css must define .settings-option--static");
  const before = raw.slice(Math.max(0, index - 600), index);
  assert.match(before, /table rows/, "the zero must be explained at the rule");
  const body = raw.slice(index, raw.indexOf("}", index));
  assert.match(body, /border-radius:\s*0;/);
});

// ── D. the boundary exemption ─────────────────────────────────────────────

test("the componentized uninstall sheet keeps its documented exemption", async () => {
  // ROUND-PASS does not own this file (R7-4's page boundary). Its 8px literal
  // is exempt from the radius drift scan exactly as its type ramp is, and the
  // exemption is asserted here so "it is skipped" cannot silently become "it
  // was deleted from the skip list".
  const css = stripComments(await read("src/extensions/ComponentizedUninstallDialog.css"));
  const rule = rules(css).find((r) => r.selector === ".extensions-uninstall-component");
  assert.ok(rule, "the sheet still defines its component surface");
  assert.equal(radius(rule!.body), "8px", "its literal radius is unchanged by this round");
  const test = await read("tests/hig-craft.test.ts");
  assert.match(
    test,
    /src\/extensions\/ComponentizedUninstallDialog\.css/,
    "the drift scans must still carry the exemption",
  );
});
