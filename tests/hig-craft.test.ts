// R7-HIG: the Apple HIG / Liquid Glass craft round. R7-GLASS and
// R7-GLASS-DEEP made the material real; this round fixed the *structure* and
// the *finish* around it:
//
//   * A named token ladder for the three things the stylesheet had drifted
//     into literals: corner radius (9 unrelated values), the type scale (13
//     sizes, 23 declarations below the platform minimum), and the focus ring
//     (1px and 2px outlines at 1px and 2px offsets).
//   * A scroll edge effect where scrolling content meets a bar
//     (`scroll-views.md › Scroll edge effects`) — a gradient band, never a
//     second filter.
//   * Gradient hairlines where a 1px solid rule used to stop dead at the
//     frame, and the desktop hit-target minimum applied to the toolbar.
//
// These assertions keep the ladder from silently re-fragmenting: every radius
// and font-size in the host sheets has to resolve to a token, the focus ring
// has one width, the scroll-edge band is a background (not a filter), and the
// toolbar clears the desktop minimum. Slices are structural, never a
// fixed-width window around a match.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// The host sheets this round owns. `src/extensions/ComponentizedUninstallDialog.css`
// is a page-boundary file (R7-4): it predates the radius/type ladders and still
// carries their literals, so the *drift* scans (radius, type, focus) skip it.
// It is NOT skipped by the structural scans that matter here — the elevation
// wiring and the reduce-motion scan both read `HOST_DIRS` directly, which is
// what the reviewer's "scans every host sheet" finding was about. The clipboard
// plugin page is outside all of them (R7-4's page boundary).
const OUT_OF_SCOPE = new Set([
  "src/extensions/ComponentizedUninstallDialog.css",
  "src/plugins/clipboard/page.css",
]);

// Both directories that hold host CSS. `src/styles/` is the shared sheets;
// `src/extensions/` holds a component-private sheet that still consumes the
// host tokens (it was wired to `--elev-0` this round) and must obey the same
// structural rules.
const HOST_DIRS = ["src/styles/", "src/extensions/"];

const hostSheets = async () => {
  const out: { name: string; css: string }[] = [];
  for (const dir of HOST_DIRS) {
    const names = (await readdir(new URL(dir, root))).filter((n) => n.endsWith(".css"));
    for (const name of names) {
      const path = `${dir}${name}`;
      out.push({ name: path, css: stripComments(await read(path)) });
    }
  }
  return out;
};

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

const declarations = (body: string, property: string) =>
  [...body.matchAll(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g"))].map((m) => m[1].trim());

// Split on commas at paren depth 0, so a gradient's own arguments are not torn
// into separate layers (`linear-gradient(to bottom, a, b)` is one layer).
const splitTopLevel = (value: string) => {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < value.length; i += 1) {
    const c = value[i];
    if (c === "(") depth += 1;
    else if (c === ")") depth -= 1;
    else if (c === "," && depth === 0) {
      parts.push(value.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(value.slice(start));
  return parts.map((part) => part.trim()).filter(Boolean);
};

// The `:root` token block of base.css, up to the light-theme override.
const rootTokens = async () => {
  const base = stripComments(await read("src/styles/base.css"));
  return base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
};

const token = (block: string, name: string) => {
  const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
  assert.ok(match, `--${name} must be defined in the dark token block`);
  return match![1].trim();
};

// A media block, walked by its own braces so the assertions read exactly the
// block's content rather than whatever happens to follow it.
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

// ── Radius ladder ─────────────────────────────────────────────────────────

test("the radius ladder is four named steps, and the window is its top step", async () => {
  const rootBlock = await rootTokens();
  const steps = ["radius-xs", "radius-sm", "radius-md", "radius-lg"].map((name) => token(rootBlock, name));
  // Every step is a literal pixel value, not a reference to another token: the
  // ladder is the source, and a step that pointed at its neighbour would make
  // "which radius is this" unanswerable.
  for (const [i, value] of steps.entries()) {
    assert.match(value, /^\d+px$/, `radius step ${i} must be a literal px value, got "${value}"`);
  }
  const px = steps.map((v) => Number(v.replace("px", "")));
  // Strictly ascending, so nesting outer > inner is always expressible.
  for (let i = 1; i < px.length; i += 1) {
    assert.ok(px[i] > px[i - 1], `the radius ladder must ascend: ${px[i - 1]} -> ${px[i]}`);
  }
  // The window radius is the ladder's top step rather than a tenth literal.
  assert.equal(token(rootBlock, "window-radius"), "10px");
  assert.equal(steps[3], token(rootBlock, "window-radius"), "--radius-lg must be the window radius");
  // The extensions panel's private alias points into the ladder rather than
  // carrying its own 6px, so its 30 call sites follow the system.
  const extensions = stripComments(await read("src/styles/extensions.css"));
  const panel = rules(extensions).find(({ selector }) => selector === ".extensions-panel");
  assert.ok(panel, "extensions.css must define .extensions-panel");
  assert.match(panel!.body, /--ext-radius:\s*var\(--radius-sm\)/);
});

test("no host sheet carries a stray literal border-radius", async () => {
  // A literal radius is not a bug by itself; an *untracked* one is, because it
  // is how the nine-value drift started. The only allowed literals are the
  // ones that are not on the scale: a circle, a pill, and the zero.
  const allowed = /^(0|50%|999px|inherit|none)$/;
  for (const { name, css } of await hostSheets()) {
    if (OUT_OF_SCOPE.has(name)) continue;
    for (const { selector, body } of rules(css)) {
      for (const value of declarations(body, "border-radius")) {
        if (allowed.test(value)) continue;
        // `--ext-radius` is the panel's own alias; the test above pins it to
        // the ladder's control step, so consuming it is consuming the ladder.
        assert.ok(
          /var\(--(radius|window-radius|ext-radius)/.test(value),
          `${name}: ${selector} border-radius: ${value} — use the radius ladder ` +
            "(--radius-xs/sm/md/lg) instead of a literal",
        );
      }
      // The per-corner longhand has the same rule.
      for (const corner of ["border-top-left-radius", "border-top-right-radius", "border-bottom-left-radius", "border-bottom-right-radius"]) {
        for (const value of declarations(body, corner)) {
          if (allowed.test(value)) continue;
          assert.ok(
            /var\(--(radius|window-radius)/.test(value),
            `${name}: ${selector} ${corner}: ${value} — use the radius ladder`,
          );
        }
      }
    }
  }
});

// ── Type scale ────────────────────────────────────────────────────────────

test("the type scale is five named steps, none below the platform minimum", async () => {
  const rootBlock = await rootTokens();
  const names = ["text-caption", "text-body", "text-emphasis", "text-title", "text-display"];
  const px = names.map((name) => {
    const value = token(rootBlock, name);
    assert.match(value, /^\d+px$/, `--${name} must be a literal px value, got "${value}"`);
    return Number(value.replace("px", ""));
  });
  for (let i = 1; i < px.length; i += 1) {
    assert.ok(px[i] > px[i - 1], `the type scale must ascend: ${px[i - 1]} -> ${px[i]}`);
  }
  // HIG `typography.md › Ensuring legibility`: macOS minimum 10pt. R7-HIG
  // raised 23 declarations that sat at 9 / 9.5px — under the minimum, and
  // legible only on a 2x display.
  assert.ok(px[0] >= 10, `the smallest step must clear the 10pt platform minimum, got ${px[0]}px`);
  // The display step is the launcher query field, which is the one place the
  // app follows the macOS default (13pt) upward toward the iOS default (17pt).
  assert.equal(px[px.length - 1], 17, "the display step is the launcher query field");
});

test("no host sheet sets a font-size outside the scale", async () => {
  for (const { name, css } of await hostSheets()) {
    if (OUT_OF_SCOPE.has(name)) continue;
    for (const { selector, body } of rules(css)) {
      for (const value of declarations(body, "font-size")) {
        // Relative units already follow whatever step their parent picked, so
        // they are not new steps on the scale; a bare px is.
        if (/^(inherit|100%|[\d.]+em|[\d.]+rem)$/.test(value)) continue;
        assert.ok(
          /var\(--text-(caption|body|emphasis|title|display)\)/.test(value),
          `${name}: ${selector} font-size: ${value} — use the type scale ` +
            "(--text-caption/body/emphasis/title/display) instead of a literal",
        );
      }
    }
  }
});

// The launcher's query field is the one display-size surface: it must keep the
// 17px macOS/iOS-default treatment that makes the launcher read as an input,
// not as a toolbar item.
test("the launcher query field keeps the display step", async () => {
  const css = stripComments(await read("src/styles/launcher.css"));
  const input = rules(css).find(({ selector }) => selector === ".collapsed-card__input");
  assert.ok(input, "launcher.css must define .collapsed-card__input");
  assert.match(input!.body, /font-size:\s*var\(--text-display\)/);
});

// ── Focus ring ────────────────────────────────────────────────────────────

test("the focus ring has one width and one offset across every sheet", async () => {
  const rootBlock = await rootTokens();
  assert.equal(token(rootBlock, "focus-ring-width"), "2px");
  assert.equal(token(rootBlock, "focus-ring-offset"), "2px");
  for (const { name, css } of await hostSheets()) {
    if (OUT_OF_SCOPE.has(name)) continue;
    for (const { selector, body } of rules(css)) {
      // Every focus-visible rule that draws an outline must use the tokens. A
      // literal `1px solid` here is exactly the drift this pins down.
      if (!/:focus-visible/.test(selector)) continue;
      for (const value of declarations(body, "outline")) {
        if (value === "none") continue;
        assert.ok(
          /var\(--focus-ring-width\)/.test(value),
          `${name}: ${selector} outline: ${value} — focus rings use ` +
            "var(--focus-ring-width), never a literal width",
        );
      }
    }
  }
  // The exception that is allowed to keep its own offset is a *negative*
  // offset on a segment inside a track, where an outside ring would be
  // clipped; the width still comes from the token.
  const terminal = stripComments(await read("src/styles/terminal.css"));
  const tab = rules(terminal).find(({ selector }) => selector === ".clipboard-panel__tab:focus-visible");
  assert.ok(tab, "the clipboard tab keeps its inset focus ring");
  assert.match(tab!.body, /outline-offset:\s*-1px/);
});

// ── Scroll edge effect ────────────────────────────────────────────────────

test("the scroll edge effect is a gradient band, never a filter", async () => {
  const rootBlock = await rootTokens();
  assert.equal(token(rootBlock, "scroll-edge"), "14px");
  // The band is a gradient, not a blur: a second filter under a bar would
  // break the one-filter-per-surface performance rule the glass round set.
  assert.match(token(rootBlock, "scroll-edge-band"), /^linear-gradient\(/);
  assert.ok(
    !/backdrop-filter|filter/.test(token(rootBlock, "scroll-edge-band")),
    "the scroll edge band must not be a filter",
  );

  // The two scrolling regions the app owns both carry the band.
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const results = rules(launcher).find(({ selector }) => selector === ".launcher-results");
  assert.ok(results, "launcher.css must define .launcher-results");
  assert.match(results!.body, /background-image:\s*var\(--scroll-edge-band\)/);
  assert.match(results!.body, /background-size:\s*100% var\(--scroll-edge\)/);

  const settings = stripComments(await read("src/styles/settings.css"));
  const content = rules(settings).find(({ selector }) => selector === ".settings-content");
  assert.ok(content, "settings.css must define .settings-content");
  assert.match(content!.body, /background-image:\s*var\(--scroll-edge-band\)/);
  assert.match(content!.body, /background-size:\s*100% var\(--scroll-edge\)/);

  // Neither scroller may have grown a blur while gaining the band.
  for (const body of [results!.body, content!.body]) {
    assert.ok(!/backdrop-filter/.test(body), "a scroll edge band must not add a backdrop-filter");
  }
});

// The band is only honest if the resting state sits outside it: the scroller
// itself must reserve the band's height as top padding, so the first row is
// dimmed only once it actually scrolls under the bar.
test("the resting state starts below the scroll edge band", async () => {
  const rootBlock = await rootTokens();
  const band = Number(token(rootBlock, "scroll-edge").replace("px", ""));

  const cases: [string, string][] = [
    ["src/styles/launcher.css", ".launcher-results"],
    ["src/styles/settings.css", ".settings-content"],
    ["src/styles/terminal.css", ".clipboard-panel__list"],
  ];
  for (const [file, selector] of cases) {
    const css = stripComments(await read(file));
    const rule = rules(css).find(({ selector: s }) => s === selector);
    assert.ok(rule, `${file}: ${selector} must exist`);
    const padding = declarations(rule!.body, "padding")[0] ?? declarations(rule!.body, "padding-top")[0];
    assert.ok(padding, `${file}: ${selector} must declare padding`);
    const top = padding.split(/\s+/)[0];
    // The reservation is either the token itself (which resolves to the band
    // by definition) or a literal at least as large as the band.
    const ok = /var\(--scroll-edge\)/.test(top) || Number(top.replace("px", "")) >= band;
    assert.ok(ok, `${file}: ${selector} reserves "${top}" of top padding, inside the ${band}px band`);
  }
});

// ── Hairlines ─────────────────────────────────────────────────────────────

test("the bar/content separators are gradient hairlines, not solid rules", async () => {
  const rootBlock = await rootTokens();
  assert.match(token(rootBlock, "hairline-fade"), /^linear-gradient\(/);
  assert.match(token(rootBlock, "hairline-fade-vertical"), /^linear-gradient\(/);

  // The three bar edges the round converted. Each is now a pseudo-element
  // band painted with the token, and none of them may be a border again.
  const cases: [string, string, string][] = [
    ["src/styles/launcher.css", ".launcher-bottom::before", "var(--hairline-fade)"],
    ["src/styles/settings.css", ".settings-card__header::after", "var(--hairline-fade)"],
    ["src/styles/settings.css", ".settings-sidebar::after", "var(--hairline-fade-vertical)"],
    ["src/styles/terminal.css", ".clipboard-panel__topbar::after", "var(--hairline-fade)"],
    ["src/styles/terminal.css", ".clipboard-panel__footer::before", "var(--hairline-fade)"],
    ["src/styles/extensions.css", ".extensions-list--installed::before", "var(--hairline-fade)"],
  ];
  for (const [file, selector, expected] of cases) {
    const css = stripComments(await read(file));
    const rule = rules(css).find(({ selector: s }) => s === selector);
    assert.ok(rule, `${file}: ${selector} must exist`);
    // A horizontal band is 1px tall; the sidebar's column separator is the
    // same idea rotated (1px wide).
    assert.ok(
      /height:\s*1px/.test(rule!.body) || /width:\s*1px/.test(rule!.body),
      `${file}: ${selector} must be a 1px band`,
    );
    assert.ok(
      rule!.body.includes(expected),
      `${file}: ${selector} must paint ${expected}`,
    );
    assert.ok(
      !/border(-top|-bottom|-left|-right)?:/.test(rule!.body),
      `${file}: ${selector} must be a painted band, not a border`,
    );
  }

  // The bars those bands belong to no longer carry the border themselves.
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const bottom = rules(launcher).find(({ selector }) => selector === ".launcher-bottom");
  assert.ok(bottom, "launcher.css must define .launcher-bottom");
  assert.ok(!/border-top/.test(bottom!.body), ".launcher-bottom must not re-add a border-top");
});

// ── Hit targets ───────────────────────────────────────────────────────────

test("the terminal toolbar clears the desktop hit-target minimum", async () => {
  const css = stripComments(await read("src/styles/terminal.css"));
  const button = rules(css).find(({ selector }) => selector === ".toolbar-button");
  assert.ok(button, "terminal.css must define .toolbar-button");
  const width = Number(declarations(button!.body, "width")[0].replace("px", ""));
  const height = Number(declarations(button!.body, "height")[0].replace("px", ""));
  // HIG `accessibility.md › Mobility`: macOS default 28x28pt, minimum 20x20pt.
  // R7-HIG raised 24x22 to 28x28; this pins the default, not just the minimum,
  // because the bar has the height to give.
  assert.ok(width >= 28, `toolbar buttons must be at least 28px wide, got ${width}`);
  assert.ok(height >= 28, `toolbar buttons must be at least 28px tall, got ${height}`);
  // The bar itself must be tall enough to contain them without clipping.
  const bar = rules(css).find(({ selector }) => selector === ".terminal-bar");
  assert.ok(bar, "terminal.css must define .terminal-bar");
  const barHeight = Number(declarations(bar!.body, "height")[0].replace("px", ""));
  assert.ok(barHeight >= height, `the ${barHeight}px bar must contain the ${height}px button`);
});

// ── Accessibility states ──────────────────────────────────────────────────

test("reduce-transparency still covers every shell after the craft pass", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const block = mediaBlock(base, "(prefers-reduced-transparency: reduce)");
  const covered = new Set<string>();
  for (const { selector, body } of rules(block)) {
    if (!/(?:^|;)\s*backdrop-filter\s*:\s*none/.test(body)) continue;
    for (const part of selector.split(",")) covered.add(part.trim());
  }
  for (const shell of [".collapsed-card", ".settings-card", ".terminal-panel", ".clipboard-panel"]) {
    assert.ok(covered.has(shell), `prefers-reduced-transparency must drop the blur on ${shell}`);
  }
});

test("the craft pass did not break reduce-motion coverage", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const block = mediaBlock(base, "(prefers-reduced-motion: reduce)");
  const neutralized = new Set<string>();
  for (const { selector, body } of rules(block)) {
    if (!/(?:^|;)\s*(?:transition|animation)\s*:\s*none/.test(body)) continue;
    for (const part of selector.split(",")) neutralized.add(part.trim());
  }
  // The controls this round gave a transition to have to be neutralized too,
  // or a reduced-motion user would see the new motion and nothing else.
  for (const selector of [
    ".settings-reset",
    ".session-manager__icon-button",
    ".session-manager__sort-toggle",
    ".session-manager__kill-confirm",
    ".opacity-control__preset",
    ".clipboard-panel__tab",
    ".launcher-action-bar",
    ".launcher-result",
  ]) {
    assert.ok(neutralized.has(selector), `prefers-reduced-motion must neutralize ${selector}`);
  }
});

// ── Motion semantics ──────────────────────────────────────────────────────

// R7-1 put the easings in tokens; R7-HIG makes sure each *kind* of surface uses
// the right one (HIG `motion.md › Providing feedback`: brief and precise). A
// shell arriving over other content may overshoot; page content that replaces
// the pane it sits in must not, because the overshoot reads as the content
// itself bouncing.
test("page content arrives on the house ease-out, not the spring", async () => {
  const settings = stripComments(await read("src/styles/settings.css"));
  const content = rules(settings).find(({ selector }) => selector === ".settings-content");
  assert.ok(content, "settings.css must define .settings-content");
  assert.match(
    content!.body,
    /animation:\s*settings-page-enter var\(--dur-3\) var\(--ease-out\)/,
    "settings page content is an arrival, not a floater: it uses --ease-out",
  );
  assert.ok(
    !/--ease-out-back/.test(content!.body),
    "page content must not use the overshooting spring",
  );
  // The shells keep the spring: they are the surfaces that appear over the
  // desktop, and the small overshoot is what separates appearing from placed.
  const base = stripComments(await read("src/styles/base.css"));
  const shells = rules(base).find(({ selector }) => selector.includes(".collapsed-shell"));
  assert.ok(shells, "base.css must define the shared shell rule");
  assert.match(shells!.body, /animation:\s*shell-enter var\(--dur-4\) var\(--spring\)/);
});

// ── Elevation ─────────────────────────────────────────────────────────────

// R7-HIG named the three rungs and consumed none of them: the floaters each
// restated `--glass-float-shadow`, so "floater > card > base" was not readable
// from the code. R8 wired the ladder up and made it tint-linked, and the
// assertions below check both halves — the rungs are a ladder, and the two
// consumers that make the ladder *mean* something draw from a rung rather than
// from a literal.
test("the elevation ladder names four depths and stays a ladder", async () => {
  const rootBlock = await rootTokens();
  // Rung 0 is the control on the plane: the inset ring/rim pair, no cast.
  assert.equal(
    token(rootBlock, "elev-0").replace(/\s+/g, " "),
    "inset 0 0 0 1px var(--glass-control-edge), inset 0 1px 0 var(--glass-control-rim)",
  );
  assert.equal(token(rootBlock, "elev-1"), "none");
  assert.match(token(rootBlock, "elev-2"), /0 1px 2px/);
  assert.match(token(rootBlock, "elev-3"), /0 10px 26px/);
  // Both cast rungs scale with the frame's fill, so depth is not a constant
  // painted under a panel that may be nearly invisible or nearly solid.
  assert.match(token(rootBlock, "elev-2"), /var\(--elev-shadow-scale\)/);
  assert.match(token(rootBlock, "elev-3"), /var\(--elev-shadow-scale\)/);
  const scale = token(rootBlock, "elev-shadow-scale");
  assert.match(scale, /var\(--main-opacity\)/, "the shadow scale must track the transparency control");
  // The floater rung is strictly the deeper of the two: a bigger blur radius
  // and a longer throw, at the same tint.
  const blurOf = (value: string) => Number(value.match(/0 (\d+)px/)?.[1] ?? 0);
  assert.ok(blurOf(token(rootBlock, "elev-3")) > blurOf(token(rootBlock, "elev-2")), "rung 3 must throw further than rung 2");
  // The aliases keep the pre-R8 call sites working without being a second
  // definition of the same shadow.
  assert.equal(token(rootBlock, "glass-float-shadow"), "var(--elev-3)");
  // Raised selection panes are rung 2 plus the lit rim.
  assert.match(token(rootBlock, "glass-raised-shadow"), /^var\(--elev-2\), inset 0 1px 0 var\(--glass-raised-rim\)$/);

  // The derivations the top of the ladder needs. Each must be written *in
  // terms of* a rung, or it is a fourth shadow wearing a token's name.
  assert.match(token(rootBlock, "elev-3-edge"), /var\(--elev-3\)/);
  assert.match(token(rootBlock, "elev-3-compact"), /var\(--window-shadow-ambient\)/);
  assert.match(token(rootBlock, "elev-4"), /var\(--window-shadow-ambient\)/);
  assert.match(token(rootBlock, "elev-bar"), /var\(--elev-shadow-scale\)/);
  assert.match(token(rootBlock, "elev-track"), /var\(--window-shadow-contact\)/);
  assert.match(token(rootBlock, "elev-hover"), /var\(--window-shadow-contact\)/);

  // Floaters draw from the top rung; the drawer, the toast and every dialog
  // are the surfaces that use it.
  const floaters: [string, string][] = [
    ["src/styles/extensions.css", ".app-toast"],
    ["src/styles/extensions.css", ".extension-permission-dialog"],
    ["src/styles/extensions.css", ".extension-custom-dialog"],
    ["src/styles/extensions.css", ".extension-removal-dialog"],
  ];
  for (const [file, selector] of floaters) {
    const css = stripComments(await read(file));
    const rule = rules(css).find(({ selector: s }) => s === selector);
    assert.ok(rule, `${file}: ${selector} must exist`);
    assert.match(rule!.body, /box-shadow:\s*var\(--elev-3\)/, `${file}: ${selector} must use the floater rung`);
  }

  // ── Wiring (HIG-2) ──────────────────────────────────────────────────────
  // R7-HIG named the ladder and consumed only four rungs. HIG-2 wires the
  // rest. Two clauses, both about the *call site* in a host sheet:
  //
  //   1. no raw colour in a shadow — tints are tokens, full stop;
  //   2. any *cast* layer (a comma-part that is not `inset`) must name a rung.
  //      A hand-rolled floater shadow can no longer be slipped in, which is
  //      the mutation lock: revert one floater to a literal cast and this goes
  //      red.
  //
  // Inset strokes are allowed to keep their 1-2px width: a width is geometry,
  // not a palette, and tokenizing every `inset 0 0 0 1px` would be a token per
  // stroke rather than a ladder. What matters is that the stroke resolves to a
  // material token (`--glass-control-edge`, `--stroke-contrast`, …).
  //
  // The deliberate carve-out: the `.platform-*` window-server frame shadows,
  // which belong to the OS chrome (a different shadow authority) and are
  // already tokenized (`--window-shadow-*`).
  const CAST_RUNG = /var\(--elev-[0-9a-z-]+\)|var\(--glass-raised-shadow\)|var\(--glass-rim-shadow/;
  // A value that is nothing but token references is an *alias* to a material
  // token (`var(--glass-field-shadow)`, `var(--glass-raised-shadow)`); its
  // internals are audited at the definition in base.css, not here.
  const allTokens = (flat: string) =>
    /^(?:var\(--[a-z0-9-]+\))(?:\s*,\s*var\(--[a-z0-9-]+\))*$/.test(flat);
  for (const { name, css } of await hostSheets()) {
    for (const { selector, body } of rules(css)) {
      if (/\.platform-(windows|linux|macos)/.test(selector)) continue;
      for (const value of declarations(body, "box-shadow")) {
        const flat = value.replace(/\s+/g, " ").trim();
        if (flat === "none") continue;
        assert.ok(
          !/(?:^|[^\w-])(?:rgba?|hsla?)\(|#[0-9a-fA-F]{3,8}\b/.test(flat),
          `${name}: ${selector} box-shadow: ${flat} — a shadow tint must be a token, never a literal`,
        );
        if (allTokens(flat)) continue;
        const cast = flat
          .split(/,(?![^(]*\))/)
          .map((part) => part.trim())
          .filter((part) => part && !part.startsWith("inset"));
        if (cast.length > 0) {
          assert.match(
            flat,
            CAST_RUNG,
            `${name}: ${selector} casts a shadow (${flat}) without a rung — use var(--elev-*)`,
          );
        }
      }
    }
  }
});

// ── Liquid Glass layer discipline ─────────────────────────────────────────

// The two-layer model from `liquid-glass.md › The two layers`: glass belongs to
// the floating functional layer, and the content layer uses standard
// materials. The app's shells are the functional layer (window chrome, the
// launcher's query field, the settings header/sidebar), and the result list /
// settings body are content. Both are asserted so a later round cannot move
// the blur into the content layer.
//
// HIG-2 made this a *real* assertion. The R7-HIG version only checked that a
// content selector's rule body does not contain the string `backdrop-filter`
// — which is true of almost every rule in the sheet, content or not, so the
// test could not fail for the reason it claimed (the reviewer's tautology
// finding). The version below asserts the positive fact the rule is really
// about: a content surface is a *standard material*, so its rule body must
// paint itself with a `--surface-*` / `--glass-control*` / `--glass-raised*`
// token (or be told to inherit the shell's tint by painting nothing), and it
// must not carry a filter, a `--glass-tint*` / `--glass-float` (the frame and
// floater tints) or a raw colour. Injecting a `backdrop-filter` *or* moving a
// content background onto the shell tint now trips it.
test("the material stays on the functional layer", async () => {
  // The content layer, per file. Each entry is a selector that owns body
  // content — rows, a scrolling list, a recess — not a shell.
  const CONTENT: [string, string][] = [
    ["launcher.css", ".launcher-bottom"],
    ["launcher.css", ".launcher-results"],
    ["launcher.css", ".launcher-result"],
    ["launcher.css", ".launcher-action-bar"],
    ["settings.css", ".settings-content"],
    ["settings.css", ".settings-card__body"],
    ["extensions.css", ".extension-tool-results"],
  ];
  // The two materials a *standard* content surface may paint:
  //   * the content recess (`--surface-sunken*`) — the result field, the
  //     settings body, the tool-result scroll;
  //   * the control/raised ladder or `transparent` — a row or a pane that
  //     reads as glass *over* the recess rather than as a second recess.
  // A shell/floater tint (`--glass-tint*`, `--glass-float`) is deliberately
  // not on the list: those are the functional layer's own fills.
  const STANDARD_MATERIAL =
    /var\(--(surface-(sunken|sunken-soft|opaque|control|control-hover|control-press)|glass-(control|control-hover|control-press|raised|raised-hover|raised-quiet|raised-warm|raised-warm-hover)|icon-surface)\)|^(transparent|none|inherit)$/;
  // The one tokenized image a content surface may paint: the scroll edge band.
  // It is a gradient *by design* — the whole point is that it is not a second
  // filter — so it is allowed by name rather than as a raw `linear-gradient`.
  const CONTENT_IMAGE = /^var\(--scroll-edge-band(?:-soft)?\)$/;
  for (const [file, selector] of CONTENT) {
    const css = stripComments(await read(`src/styles/${file}`));
    // Every rule that *is* this content face, not just the first one: a later
    // duplicate rule must not be able to repaint the face behind the test's
    // back (the reviewer's appended-selector mutation).
    const matching = rules(css).filter(({ selector: s }) => s === selector);
    assert.ok(matching.length > 0, `${file}: ${selector} must exist`);
    for (const { body } of matching) {
      // 1. Functional-layer material must not appear in a content rule.
      assert.ok(
        !/backdrop-filter/.test(body),
        `${selector} is content and must not filter the backdrop (the one-sheet rule)`,
      );
      assert.ok(
        !/var\(--glass-(tint|float|saturate|blur)/.test(body),
        `${selector} is content — it must use a standard material, not the frame/floater tint`,
      );
      // 2. Whatever it *does* paint has to be a named standard material (or an
      //    explicit transparent/inherit), never a raw colour and never a shell
      //    tint handed in through a raw value.
      const backgrounds = [
        ...declarations(body, "background"),
        ...declarations(body, "background-color"),
      ];
      // A multi-layer background (`linear-gradient(...), var(--token)`) is judged
      // by its token layer only; a rule that paints nothing is skipped.
      for (const value of backgrounds) {
        const tokenLayers = value
          .split(/,(?![^(]*\))/)
          .map((part) => part.trim())
          .filter((part) => !/^(linear-gradient|radial-gradient|none)/.test(part));
        if (tokenLayers.length === 0) continue;
        for (const layer of tokenLayers) {
          assert.ok(
            STANDARD_MATERIAL.test(layer),
            `${file}: ${selector} background: ${layer} — a content surface paints a standard ` +
              "material (--surface-* / the control ladder), never the frame's tint or a raw colour",
          );
        }
      }
      // 3. `background-image` is the hole the reviewer found: a content face
      //    could paint raw gradient stops (or any raw colour) through it while
      //    the `background`/`background-color` scan stayed green. The band is
      //    the only image a content face may paint, and it must be the token,
      //    not an inlined `linear-gradient(#hex, …)` that merely looks the same.
      for (const value of declarations(body, "background-image")) {
        for (const layer of splitTopLevel(value)) {
          assert.ok(
            CONTENT_IMAGE.test(layer),
            `${file}: ${selector} background-image: ${layer} — a content surface paints the ` +
              "tokenized band (var(--scroll-edge-band*)), never a raw colour or an inline gradient",
          );
        }
      }
    }
  }

  // Functional: the shell that floats over the desktop is the only filter.
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const shell = rules(launcher).find(({ selector }) => selector === ".collapsed-card");
  assert.ok(shell, "launcher.css must define .collapsed-card");
  assert.match(shell!.body, /backdrop-filter/);
  // Functional: the settings sidebar sits in the floating layer and is a
  // standard material — the shell's own tint, not a second recess that would
  // push its text down another contrast step. "Paints nothing" is the positive
  // assertion here, so a later `background:` on it is caught.
  const settings = stripComments(await read("src/styles/settings.css"));
  const sidebar = rules(settings).find(({ selector }) => selector === ".settings-sidebar");
  assert.ok(sidebar, "settings.css must define .settings-sidebar");
  assert.ok(
    !/background:/.test(sidebar!.body),
    "the sidebar must sit on the shell's tint, not paint a second material",
  );
});

// Minor 2 / the reviewer's finding: clause 3 used to read `background` and
// `background-color` only, so a content face could paint `#ff00ff` (or the
// shell's tint) through `background-image` and stay green. This is the
// dedicated lock for that clause: the band is the only image a content face may
// paint, and the check is proved to fire on an injected raw gradient.
test("the content layer's background-image is tokenized, not a raw gradient", async () => {
  const CONTENT_IMAGE = /^var\(--scroll-edge-band(?:-soft)?\)$/;
  const imageViolations = (body: string) =>
    declarations(body, "background-image").flatMap((value) =>
      splitTopLevel(value).filter((layer) => !CONTENT_IMAGE.test(layer)),
    );

  // The three scrollers that legitimately paint the band must do so by token.
  const banded: [string, string][] = [
    ["src/styles/launcher.css", ".launcher-results"],
    ["src/styles/settings.css", ".settings-content"],
    ["src/styles/terminal.css", ".clipboard-panel__list"],
  ];
  for (const [file, selector] of banded) {
    const css = stripComments(await read(file));
    const rule = rules(css).find(({ selector: s }) => s === selector);
    assert.ok(rule, `${file}: ${selector} must exist`);
    assert.match(rule!.body, /background-image:\s*var\(--scroll-edge-band\)/);
    assert.deepEqual(imageViolations(rule!.body), [], `${file}: ${selector}'s band must be the token`);
  }

  // The guard really fires: an inline raw gradient, a raw hex stop, and a shell
  // tint are all rejected (the reviewer's `#ff00ff` injection is case one).
  for (const bad of [
    "linear-gradient(180deg, #ff00ff, #00ff00)",
    "url(#ff00ff)",
    "var(--glass-float)",
  ]) {
    assert.ok(
      imageViolations(`background-image: ${bad};`).length > 0,
      `the content-image guard must reject "${bad}"`,
    );
  }
  // …while the tokenized band (and its soft sibling) pass.
  for (const good of ["var(--scroll-edge-band)", "var(--scroll-edge-band-soft)"]) {
    assert.deepEqual(imageViolations(`background-image: ${good};`), [], `the guard must accept ${good}`);
  }
});
