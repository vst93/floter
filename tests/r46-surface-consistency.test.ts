// R46 · one window, four surfaces, one margin and one shadow language.
//
// The user's report, verbatim: 「还有，我发现搜索页面、应用设置页面、窗口以及终端页
// 面的外边距或者阴影存在不一致性。这个需要尽可能保持一致，让软件有更强的一体性」.
//
// The round is an audit: the launcher card, the settings card and the terminal
// panel are measured against each other, and every
// difference that carries no meaning is collapsed onto one number. What the
// numbers *are* is the sheets' business; what this file locks is that they are
// now the same number, per axis, and that the differences that stay are the
// ones the sheets explain (the Windows launcher's asymmetric shadow gutter).
//
// Mutations that must turn this red:
//   * giving the Linux launcher shell its old 4px margin back -> "one margin";
//   * restoring `.platform-linux .collapsed-card`'s inset-only override ->
//     "no resting launcher without a frame shadow";
//   * moving any terminal chrome strip off 12px -> "one terminal inset";
//   * putting the settings header's trailing inset back to 10u -> "one band";
//   * pulling a launcher panel block off the 16u column -> "one column".
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of stripComments(css).matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

const declarations = (body: string, property: string) =>
  [...body.matchAll(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g"))].map((m) => m[1].trim());

const rule = (css: string, selector: string) => {
  const found = rules(css).find(({ selector: s }) =>
    s.split(",").some((part) => part.trim() === selector),
  );
  assert.ok(found, `the sheet must still define ${selector}`);
  return found!;
};

const decl = (css: string, selector: string, property: string) => {
  const value = declarations(rule(css, selector).body, property)[0];
  assert.ok(value, `${selector} must declare ${property}`);
  return value;
};

/** Split a value into top-level components, respecting parentheses. */
const splitTokens = (value: string) => {
  const out: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < value.length; i += 1) {
    const c = value[i];
    if (c === "(") depth += 1;
    else if (c === ")") depth -= 1;
    else if (/\s/.test(c) && depth === 0) {
      if (i > start) out.push(value.slice(start, i));
      start = i + 1;
    }
  }
  if (start < value.length) out.push(value.slice(start));
  return out;
};

/** The numeric value of every component, whichever unit it is written in:
 *  `0`, `calc(var(--u) * N)` and `Npx` all resolve to a number. */
const comps = (value: string) =>
  splitTokens(value.trim()).map((token) => {
    if (token === "0") return 0;
    const unit = /^calc\(var\(--u\)\s*\*\s*(\d+(?:\.\d+)?)\)$/.exec(token);
    if (unit) return Number(unit[1]);
    const literal = /^(\d+(?:\.\d+)?)px$/.exec(token);
    if (literal) return Number(literal[1]);
    assert.fail(`unresolvable component "${token}"`);
  });

/** The same, but every non-zero component must be on the `--u` knob. */
const units = (value: string) => {
  for (const token of splitTokens(value.trim())) {
    if (token === "0") continue;
    assert.match(token, /^calc\(var\(--u\)\s*\*\s*\d+(?:\.\d+)?\)$/, `"${token}" must be written in units`);
  }
  return comps(value);
};

// ── 1 · the window's own margin: one number per platform ──────────────────

test("the shells' margin to the window edge is one number per platform", async () => {
  const base = await read("src/styles/base.css");

  // Linux. The three shells are the same window swapping its body, so the
  // collapsed shell carries the panels' 10u (R46: it carried 4u, which also
  // forced it to drop the frame shadow — see the next test).
  for (const shell of [
    ".platform-linux .collapsed-shell",
    ".platform-linux .settings-shell",
    ".platform-linux .terminal-shell",
  ]) {
    assert.deepEqual(units(decl(base, shell, "padding")), [10], `${shell} pads the window edge by 10u`);
  }

  // macOS reserves nothing: the window server owns the frame and the shadow
  // (`set_shadow(true)`), so every surface is flush. Asserted as the *absence*
  // of a rule, which is what "flush" means in this sheet.
  assert.ok(
    !/\.platform-macos\s+\.(?:collapsed|settings|terminal)-shell\s*\{/.test(stripComments(base)),
    "macOS must not pad a shell — the window server is the frame there",
  );

  // Windows keeps its documented asymmetric gutter for the launcher only (the
  // panel hangs from the top of the screen and lights from above-left), and the
  // panels keep the symmetric one. Both are named so an edit has to face the
  // reason rather than the number.
  assert.deepEqual(
    units(decl(base, ".platform-windows .collapsed-shell", "padding")),
    [4, 10, 12, 4],
    "the Windows launcher shell keeps its documented asymmetric shadow gutter",
  );
  for (const shell of [".platform-windows .settings-shell", ".platform-windows .terminal-shell"]) {
    assert.deepEqual(units(decl(base, shell, "padding")), [10], `${shell} keeps the symmetric 10u`);
  }
});

test("no resting launcher is left without the frame shadow its siblings cast", async () => {
  const base = await read("src/styles/base.css");

  // The frame cast is one rung per platform and every window-filling surface
  // draws it: the group rule names all three cards, so a per-surface override
  // that strips the cast is the mutation this test exists to catch.
  for (const [platform, throw_] of [["windows", "0 4px 16px"], ["linux", "0 4px 18px"]] as const) {
    const group = rule(
      base,
      `.platform-${platform} .collapsed-card`,
    );
    assert.ok(
      group.selector.includes(`.platform-${platform} .settings-card`) &&
        group.selector.includes(`.platform-${platform} .terminal-panel`),
      `the ${platform} frame shadow must be one group rule over all three surfaces`,
    );
    assert.match(group.body, new RegExp(throw_.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")), `${platform} ambient throw`);
    assert.match(group.body, /var\(--window-shadow-ambient\)/, `${platform} ambient tint is the shared token`);
    assert.match(group.body, /var\(--window-shadow-contact\)/, `${platform} contact tint is the shared token`);

    // No later rule may re-draw `.collapsed-card` with the insets alone: that
    // is exactly the R46 defect (the launcher was the one Linux surface with no
    // cast at rest), and only the focus rule may change the strokes.
    for (const { selector, body } of rules(stripComments(base))) {
      if (!/\.platform-(windows|linux)/.test(selector)) continue;
      const parts = selector.split(",").map((part) => part.trim());
      if (!parts.includes(`.platform-${platform} .collapsed-card`)) continue;
      const cast = declarations(body, "box-shadow")
        .flatMap((value) => value.split(/,(?![^(]*\))/))
        .map((part) => part.trim())
        .filter((part) => part && !part.startsWith("inset"));
      assert.ok(cast.length > 0, `${selector} strips the resting cast — the launcher is not a lesser surface`);
    }
  }

  // The launcher's *focus* is the accent ring, not the presence of a shadow:
  // both Linux and Windows keep the cast and swap a stroke, so focus adds a
  // state rather than a rung.
  const linuxFocus = rule(base, ".platform-linux .collapsed-card:focus-within");
  assert.match(linuxFocus.body, /var\(--accent-glow\)/, "the focused launcher marks focus with the accent ring");
  assert.match(linuxFocus.body, /var\(--window-shadow-ambient\)/, "…without dropping the frame cast");
});

// ── 2 · the window band: one height (R37) and one inset (R46) ─────────────

test("the 56u window band carries one inset on each side", async () => {
  const launcher = await read("src/styles/launcher.css");
  const settings = await read("src/styles/settings.css");

  // Leading: the field row's own 16u.
  assert.deepEqual(units(decl(launcher, ".collapsed-card__input-row", "padding")), [0, 16], "field row");
  // …and its trailing control is the same inset (the field row's padding is
  // symmetric), so the settings header must be symmetric too.
  assert.deepEqual(units(decl(settings, ".settings-card__header", "padding")), [0, 16], "settings header");
  // The filter subline is the field's own row and shares the 16u.
  assert.deepEqual(units(decl(launcher, ".launcher-filter", "padding")), [0, 16], "filter subline");

  // The settings sidebar's first glyph: the column (8u) plus the item's own
  // padding is the same 16u, so the sidebar rows and the title above them start
  // on one line.
  const sidebar = units(decl(settings, ".settings-sidebar", "padding"));
  const item = units(decl(settings, ".settings-sidebar__item", "padding"));
  assert.equal(sidebar[1], 8, "the sidebar column pads 8u");
  assert.equal(item[1], 8, "…and the item's own inset is 8u, so the glyph lands on 16u");
});

// ── 3 · the launcher's column: window band 16u, panel 4u + 12u ───────────

test("every launcher block lands on the query's 16u column", async () => {
  const launcher = await read("src/styles/launcher.css");
  // The sunken panel's own inset, which is what the blocks below sit in.
  const panel = units(decl(launcher, ".launcher-bottom", "padding"))[1];
  assert.equal(panel, 4, "the panel keeps its 4u inset (a selected row's tint stays off its edge)");

  // The row family and every other panel block pad 12u, so 4 + 12 is the 16u
  // column the query text and the chips start on.
  for (const selector of [
    ".launcher-result",
    ".launcher-status",
    ".launcher-action-bar",
    ".launcher-feedback",
    ".launcher-hint",
    ".launcher-system-confirm",
    ".launcher-plugin-footer",
  ]) {
    const sides = units(decl(launcher, selector, "padding"));
    assert.equal(sides[sides.length - 1], 12, `${selector} pads 12u inside the panel (16u from the card)`);
  }
  assert.deepEqual(units(decl(launcher, ".launcher-plugin-text", "padding")), [8, 12], "plugin text");

  // The group title is a body of that column too, and it is on the knob like
  // the rows it labels (a literal would keep the alignment only at the default
  // step). Its box is still its own line box plus one padding pair.
  assert.deepEqual(units(decl(launcher, ".launcher-section-title", "padding")), [6, 12, 4], "group title");

  // The two card-level blocks are full-bleed, so they carry the whole 16u.
  assert.deepEqual(units(decl(launcher, ".launcher-tip", "padding")), [4, 16], "first-run tip");
});

// ── 4 · the terminal surface's chrome: one inset ─────────────────────────

test("the terminal's chrome shares one 12px inset", async () => {
  const terminal = await read("src/styles/terminal.css");

  // The horizontal inset of every piece of terminal chrome. The canvas's own
  // inset is the user's `terminal_padding` and is deliberately not part of this
  // list (it is a content axis, not chrome).
  assert.deepEqual(comps(decl(terminal, ".terminal-bar", "padding")), [0, 12], "the terminal bar");
  assert.deepEqual(comps(decl(terminal, ".terminal-copy-notice", "padding")), [0, 12], "the copy notice strip");
  assert.deepEqual(comps(decl(terminal, ".terminal-settings-drawer", "padding")), [10, 12], "the settings drawer");
  assert.deepEqual(comps(decl(terminal, ".terminal-resident", "left")), [12], "the exit note's left edge");
  assert.deepEqual(comps(decl(terminal, ".terminal-resident", "right")), [12], "…and its right");
  assert.deepEqual(comps(decl(terminal, ".terminal-feedback", "left")), [12], "the feedback toast");
  assert.deepEqual(comps(decl(terminal, ".terminal-bar", "height")), [28], "the terminal bar's own height");
});

// ── 5 · the floaters keep their explainable rung ─────────────────────────

test("the window cards draw the shared material rim", async () => {
  for (const sheet of ["launcher.css", "settings.css", "terminal.css"]) {
    const css = await read(`src/styles/${sheet}`);
    const card = sheet === "launcher.css" ? ".collapsed-card" : sheet === "settings.css" ? ".settings-card" : ".terminal-panel";
    assert.match(decl(css, card, "box-shadow"), /var\(--glass-rim-shadow\)/, `${card} draws the shared rim`);
  }
});
