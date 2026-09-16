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

// The host sheets this round owns. The uninstall dialog's own sheet and the
// clipboard plugin page are deliberately outside it (R7-4's page boundary) and
// are listed here so the exception is visible rather than a silent gap.
const OUT_OF_SCOPE = new Set([
  "src/extensions/ComponentizedUninstallDialog.css",
  "src/plugins/clipboard/page.css",
]);

const hostSheets = async () => {
  const dir = new URL("src/styles/", root);
  const names = (await readdir(dir)).filter((n) => n.endsWith(".css"));
  return Promise.all(
    names.map(async (name) => ({
      name: `src/styles/${name}`,
      css: stripComments(await read(`src/styles/${name}`)),
    })),
  );
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

test("the elevation ladder names three depths and stays a ladder", async () => {
  const rootBlock = await rootTokens();
  assert.equal(token(rootBlock, "elev-1"), "none");
  assert.match(token(rootBlock, "elev-2"), /var\(--window-shadow-contact\)/);
  assert.match(token(rootBlock, "elev-3"), /var\(--glass-float-shadow\)/);
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
    assert.match(rule!.body, /box-shadow:\s*var\(--glass-float-shadow\)/, `${file}: ${selector} must use the floater rung`);
  }
});

// ── Liquid Glass layer discipline ─────────────────────────────────────────

// The two-layer model from `liquid-glass.md › The two layers`: glass belongs to
// the floating functional layer, and the content layer uses standard
// materials. The app's shells are the functional layer (window chrome, the
// launcher's query field, the settings header/sidebar), and the result list /
// settings body are content. Both are asserted so a later round cannot move
// the blur into the content layer.
test("the material stays on the functional layer", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));
  // Functional: the shell that floats over the desktop.
  const shell = rules(launcher).find(({ selector }) => selector === ".collapsed-card");
  assert.ok(shell, "launcher.css must define .collapsed-card");
  assert.match(shell!.body, /backdrop-filter/);
  // Content: the result field is a standard material (a tint over the shell),
  // never a second filter.
  for (const selector of [".launcher-bottom", ".launcher-results", ".launcher-result", ".launcher-action-bar"]) {
    const rule = rules(launcher).find(({ selector: s }) => s === selector);
    assert.ok(rule, `launcher.css must define ${selector}`);
    assert.ok(!/backdrop-filter/.test(rule!.body), `${selector} is content and must not filter the backdrop`);
  }
  // Functional: the settings header and sidebar sit in the floating layer.
  // Content: the body scrolls, so it takes a standard material.
  const settings = stripComments(await read("src/styles/settings.css"));
  for (const selector of [".settings-content", ".settings-sidebar", ".settings-card__body"]) {
    const rule = rules(settings).find(({ selector: s }) => s === selector);
    assert.ok(rule, `settings.css must define ${selector}`);
    assert.ok(!/backdrop-filter/.test(rule!.body), `${selector} is content and must not filter the backdrop`);
  }
  const sidebar = rules(settings).find(({ selector }) => selector === ".settings-sidebar");
  // The sidebar is a standard material: the shell's own tint, not a second
  // recess that would push its text down another contrast step.
  assert.ok(
    !/background:/.test(sidebar!.body),
    "the sidebar must sit on the shell's tint, not paint a second material",
  );
});
