// R154 · the Windows frame fill: the shells' own material, with no
// `backdrop-filter` standing between the slider and the desktop.
//
// The user's report, verbatim: 「Windows 下测试发现：1. 应用的背景整个都是透明的，
// 设置当中的透明度配置完全没有作用。2. 感觉整个布局也是有问题的。」
//
// The transcription of the screenshot names the exact set of surfaces that kept
// their own fill: the settings sidebar pills, two body buttons, and one small
// white box at the top-left (the header's literal sheen band). Those are
// precisely the surfaces that carry **no `backdrop-filter` of their own** — the
// control-material ladder in base.css is explicit that "no control ever carries
// a backdrop-filter" — while the three window-filling shells do. On WebView2 the
// filtered shell is composited into a layer of its own and the shell's fill
// (`--glass-tint` / `--launcher-tint` / `--glass-tint-terminal`, i.e. the only
// place the transparency slider is visible) is dropped with it, so the card
// painted as bare desktop and the slider appeared inert. Windows also has no OS
// backdrop to blur: tao makes a transparent window with
// `DwmEnableBlurBehindWindow` and an *empty* blur region, which is the
// per-pixel-alpha trick rather than an acrylic sheet.
//
// So the Windows fallback in base.css turns the filter off for the three shells
// and nothing else. This file locks the four halves of that sentence, all of
// them source-scan (the gate has no Windows runtime — see the R154 report's
// "Windows retest" list for what only the user can confirm):
//
//   1. the shells still carry their filter and their fill in the *shared* path,
//      so "turn it off on Windows" stays a fallback and not a deletion;
//   2. the `.platform-windows` fallback covers all three shells in one group
//      rule and says `none` in both spellings;
//   3. no Windows rule re-enables the filter for a shell;
//   4. the fallback adds no alpha and no colour of its own — the transparency
//      slider stays the frame's only alpha truth on Windows, exactly as it is
//      everywhere else.
//
// Mutations that must turn this red (all three were run, see the report):
//   * deleting the `backdrop-filter: none` declaration from the Windows group
//     rule            -> "the Windows fallback turns the filter off";
//   * changing it to `blur(var(--glass-blur))` -> the same test;
//   * adding `--glass-frame-alpha: 0.86;` to that rule -> "adds no alpha".
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

interface Rule {
  selector: string;
  body: string;
}

/** Every `selector { body }` rule in a sheet, at one brace level. */
const rules = (css: string): Rule[] => {
  const out: Rule[] = [];
  for (const match of stripComments(css).matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

const parts = (selector: string) => selector.split(",").map((part) => part.trim());

/** Every value declared for `property`, in order. */
const declarations = (body: string, property: string) =>
  [...body.matchAll(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g"))].map((m) => m[1].trim());

const decl = (body: string, property: string) => declarations(body, property)[0] ?? null;

/** The one rule whose selector group contains every one of `selectors`. */
const groupRule = (css: string, selectors: readonly string[]) => {
  const found = rules(css).find((rule) => selectors.every((s) => parts(rule.selector).includes(s)));
  assert.ok(found, `the sheet must define one rule covering ${selectors.join(", ")}`);
  return found!;
};

/** The three window-filling shells, with the sheet that styles each and the
 *  token its fill actually comes from. The terminal's tint is on its veil: the
 *  panel itself is `background: transparent` because the canvas paints the
 *  panel's colour on top of it (terminal.css says so), so the veil is the
 *  surface the filter was compositing away. */
const SHELLS = [
  { shell: ".collapsed-card", sheet: "launcher.css", fill: "--launcher-tint", fillSelector: ".collapsed-card" },
  { shell: ".settings-card", sheet: "settings.css", fill: "--glass-tint", fillSelector: ".settings-card" },
  { shell: ".terminal-panel", sheet: "terminal.css", fill: "--glass-tint-terminal", fillSelector: ".terminal-panel__veil" },
] as const;

const windowsShellSelectors = SHELLS.map(({ shell }) => `.platform-windows ${shell}`);

// ── 1 · the shared path keeps the filter and the fill ─────────────────────

test("every window-filling shell keeps its filter and its fill in the shared path", async () => {
  for (const { shell, sheet, fill, fillSelector } of SHELLS) {
    const css = await read(`src/styles/${sheet}`);

    // The filter the Windows fallback switches off. Asserting it here is what
    // stops the fallback from being "the filter was deleted everywhere" — a
    // change that would pass every test below while flattening macOS and Linux.
    const filter = decl(groupRule(css, [shell]).body, "backdrop-filter");
    assert.ok(filter, `${shell} must still declare backdrop-filter in the shared path`);
    assert.notEqual(filter, "none", `${shell}'s shared filter is the material, not the fallback`);

    // …and the fill the filter used to hide. The Windows fallback is only a
    // fallback because this token still reaches the surface.
    const fillRule = groupRule(css, [fillSelector]);
    assert.match(
      decl(fillRule.body, "background") ?? "",
      new RegExp(`var\\(${fill.replace(/[-]/g, "\\-")}\\)`),
      `${fillSelector} must still paint its fill from var(${fill})`,
    );
  }
});

// ── 2 · the Windows fallback covers all three shells, in both spellings ────

test("the Windows fallback turns the filter off for all three shells at once", async () => {
  const base = await read("src/styles/base.css");
  const fallback = groupRule(base, windowsShellSelectors);

  for (const property of ["backdrop-filter", "-webkit-backdrop-filter"]) {
    assert.equal(
      decl(fallback.body, property),
      "none",
      `the Windows shell fallback must declare \`${property}: none\``,
    );
  }

  // The fallback is a *fallback*: it is the platform block's rule, and the
  // selector has to name the platform class. A bare `.collapsed-card { … }`
  // edit would leak onto macOS and Linux.
  for (const selector of parts(fallback.selector)) {
    assert.match(selector, /^\.platform-windows \./, `${selector} must be scoped to Windows`);
  }
});

// ── 3 · nothing on Windows re-enables the filter ──────────────────────────

test("no Windows rule re-enables a backdrop-filter on a shell", async () => {
  for (const sheet of ["base.css", "launcher.css", "settings.css", "terminal.css"]) {
    const css = await read(`src/styles/${sheet}`);
    for (const rule of rules(css)) {
      const selectorParts = parts(rule.selector);
      const windows = selectorParts.filter((part) => part.startsWith(".platform-windows "));
      if (windows.length === 0) continue;
      if (!windows.some((part) => SHELLS.some(({ shell }) => part === `.platform-windows ${shell}`))) {
        continue;
      }
      for (const property of ["backdrop-filter", "-webkit-backdrop-filter"]) {
        for (const value of declarations(rule.body, property)) {
          assert.equal(
            value,
            "none",
            `${rule.selector} re-enables \`${property}: ${value}\` on a Windows shell`,
          );
        }
      }
    }
  }
});

// ── 4 · the fallback adds no alpha and no colour of its own ───────────────

test("the Windows fallback adds no alpha and no colour of its own", async () => {
  const base = await read("src/styles/base.css");
  const fallback = groupRule(base, windowsShellSelectors);

  // An alpha or a colour written into the platform block is a second truth for
  // the transparency slider — exactly the failure the report describes
  // (「透明度配置完全没有作用」). The fill has to keep coming from the tokens.
  const forbidden = [
    "--glass-frame-alpha",
    "--glass-tint-alpha",
    "--glass-frame-floor",
    "--glass-solid-top",
    "--main-opacity",
    "--launcher-frame-alpha",
    "--launcher-tint-alpha",
    "--terminal-opacity",
  ];
  for (const token of forbidden) {
    assert.ok(
      !fallback.body.includes(token),
      `the Windows shell fallback must not restate ${token} — the slider is the only alpha truth`,
    );
  }
  assert.doesNotMatch(
    fallback.body,
    /(?:^|[^-\w])(?:rgba?|hsla?)\s*\(|#[0-9a-fA-F]{3,8}\b/,
    "the Windows shell fallback must not carry a colour literal of its own",
  );

  // …and the chain it defers to is still the one the sliders write. Pinned at
  // the source because the Windows fallback is the newest thing standing next
  // to it: a `--glass-frame-alpha` that stopped reading `--main-opacity` would
  // make the slider inert on *every* platform while every screenshot stayed
  // plausible.
  // Comments first: the file's own header names `[data-theme="light"]` in prose,
  // and a raw `indexOf` would slice the block down to nothing.
  const baseCode = stripComments(base);
  const rootBlock = baseCode.slice(
    baseCode.indexOf(":root {"),
    baseCode.indexOf('[data-theme="light"] {'),
  );
  assert.match(
    decl(rootBlock, "--glass-frame-alpha") ?? "",
    /--glass-frame-floor[\s\S]*--main-opacity/,
    "--glass-frame-alpha must clamp the slider's --main-opacity against the floor",
  );
  assert.equal(decl(rootBlock, "--glass-frame-floor"), "0", "the frame floor is 0 outside the contrast override");
  assert.match(decl(rootBlock, "--glass-tint-alpha") ?? "", /--glass-frame-alpha/, "--glass-tint-alpha rides the frame alpha");
  assert.match(
    decl(rootBlock, "--glass-tint-alpha-terminal") ?? "",
    /--glass-frame-alpha-terminal/,
    "…and the terminal's rides its own",
  );

  const launcher = await read("src/styles/launcher.css");
  assert.match(
    decl(groupRule(launcher, [".collapsed-card"]).body, "--launcher-tint-alpha") ?? "",
    /--main-opacity/,
    "the launcher's fill alpha is the slider",
  );
});
