// R7-2 · The two surface tables, pinned against the pre-R7-2 behaviour.
//
// The Esc/⌘W tests below do not assert the table against itself. They carry a
// verbatim transcription of the hand-written branches that `useAppKeyboard`,
// `App.tsx` and `ExtensionsPanel` used before this round (`preR72Dismiss`,
// copied from `git show c9deb29:src/hooks/useAppKeyboard.ts` lines 242-294) and
// require the table-driven resolver to agree with it for every surface × every
// press. Changing a table entry therefore turns these red unless the frozen
// reference is changed too — which is the point: the reference is history, the
// table is the new single source.
//
// Mutations that must turn this red:
//   * delete any `DISMISS_TABLE[surface][trigger]` cell -> the completeness
//     test fails on the missing key, and the golden comparison fails on the
//     press that used to resolve through it;
//   * swap `close-settings` for `close-plugin` in the settings row -> golden
//     comparison fails;
//   * drop `stopPropagation` from the settings/plugin `mod-w` rule -> golden
//     comparison fails;
//   * change `DISMISS_TRIGGER_ORDER` so `escape` precedes `mod-w` -> the
//     settings Cmd+W press still resolves (both cells close), so that one is
//     caught by the order assertion instead;
//   * remove `focusSettingsSidebar(...)` from App's settings branch -> the
//     "settings entry focuses the sidebar" source-shape test fails;
//   * empty `SURFACE_FOCUS_POLICY.settings.beats`/owner -> the policy test
//     fails.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  DISMISS_TABLE,
  DISMISS_TRIGGER_ORDER,
  MODAL_GUARDED_SURFACES,
  SURFACE_FOCUS_POLICY,
  applySurfaceFocusOnEntry,
  focusSettingsSidebar,
  isDismissKey,
  resolveDismissRule,
  settingsSidebarTabIndex,
  surfaceFocusBeats,
  surfaceYieldsToModal,
  type AppSurface,
  type DismissTrigger,
} from "../src/surface-policy.ts";
import { COLLAPSED_FOCUS_BEATS_MS } from "../src/collapsed-focus.ts";
import {
  DEFAULT_SHORTCUTS,
  IS_WINDOWS,
  matchesShortcut,
  type ShortcutMap,
} from "../src/shortcuts.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const SURFACES: AppSurface[] = ["collapsed", "terminal", "settings", "plugin"];
const TRIGGERS: DismissTrigger[] = ["mod-w", "escape", "new-command"];

type KeySpec = {
  key: string;
  code: string;
  ctrl?: boolean;
  meta?: boolean;
  alt?: boolean;
  shift?: boolean;
};

const keyEvent = (spec: KeySpec): KeyboardEvent =>
  ({
    key: spec.key,
    code: spec.code,
    ctrlKey: spec.ctrl ?? false,
    metaKey: spec.meta ?? false,
    altKey: spec.alt ?? false,
    shiftKey: spec.shift ?? false,
  }) as KeyboardEvent;

const PRESSES: Array<{ name: string; event: KeyboardEvent }> = [
  { name: "Escape", event: keyEvent({ key: "Escape", code: "Escape" }) },
  { name: "Ctrl+W", event: keyEvent({ key: "w", code: "KeyW", ctrl: true }) },
  { name: "Cmd+W", event: keyEvent({ key: "w", code: "KeyW", meta: true }) },
  { name: "Ctrl+Shift+W", event: keyEvent({ key: "W", code: "KeyW", ctrl: true, shift: true }) },
  { name: "bare w", event: keyEvent({ key: "w", code: "KeyW" }) },
  { name: "Ctrl+Q", event: keyEvent({ key: "q", code: "KeyQ", ctrl: true }) },
  { name: "ArrowDown", event: keyEvent({ key: "ArrowDown", code: "ArrowDown" }) },
  { name: "Enter", event: keyEvent({ key: "Enter", code: "Enter" }) },
  { name: "plain a", event: keyEvent({ key: "a", code: "KeyA" }) },
];

/** The shortcut map the app runs with on a fresh profile. */
const shortcuts: ShortcutMap = DEFAULT_SHORTCUTS;

type OldRule = { action: string; stop: boolean };

/**
 * Frozen transcription of the pre-R7-2 branches. Do not "fix" this to match
 * the table: it exists to disagree when the table drifts.
 *
 * Source: `git show c9deb29:src/hooks/useAppKeyboard.ts` lines 242-294 and
 * `git show c9deb29:src/App.tsx` lines 318-322 (collapsed).
 */
function preR72Dismiss(surface: AppSurface, event: KeyboardEvent): OldRule | null {
  if (surface === "settings") {
    // if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w")
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
      return { action: "close-settings", stop: true };
    }
    // if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command))
    if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
      return { action: "close-settings", stop: false };
    }
    return null;
  }
  if (surface === "plugin") {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
      return { action: "close-plugin", stop: true };
    }
    if (event.key === "Escape") {
      return { action: "close-plugin", stop: false };
    }
    return null;
  }
  if (surface === "collapsed") {
    // if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command))
    if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
      return { action: "hide-window", stop: false };
    }
    return null;
  }
  // terminal: the first branch of the terminal handler.
  if (matchesShortcut(event, shortcuts.new_command)) {
    return { action: "return-to-input", stop: false };
  }
  return null;
}

const normalize = (rule: ReturnType<typeof resolveDismissRule>): OldRule | null =>
  rule === null ? null : { action: rule.action, stop: Boolean(rule.stopPropagation) };

test("the dismiss table covers every surface × every trigger (no holes)", () => {
  for (const surface of SURFACES) {
    const row = DISMISS_TABLE[surface];
    assert.ok(row, `missing row for surface ${surface}`);
    for (const trigger of TRIGGERS) {
      assert.ok(
        trigger in row,
        `DISMISS_TABLE.${surface}.${trigger} must exist (null is an explicit "no rule")`,
      );
      assert.notEqual(
        row[trigger],
        undefined,
        `DISMISS_TABLE.${surface}.${trigger} must be a rule or null, never undefined`,
      );
    }
    assert.deepEqual(
      Object.keys(row).sort(),
      [...TRIGGERS].sort(),
      `${surface} must declare exactly the known triggers`,
    );
  }
});

test("the trigger order keeps the pre-R7-2 precedence (mod-w before escape)", () => {
  assert.deepEqual([...DISMISS_TRIGGER_ORDER], ["mod-w", "escape", "new-command"]);
});

test("resolving a press equals the frozen pre-R7-2 behaviour, for every surface", () => {
  for (const surface of SURFACES) {
    for (const press of PRESSES) {
      assert.deepEqual(
        normalize(resolveDismissRule(surface, press.event, shortcuts)),
        preR72Dismiss(surface, press.event),
        `${surface} + ${press.name}`,
      );
    }
  }
});

test("each surface's rule set is the one the pre-R7-2 branches encoded", () => {
  // Readable form of the same golden, so a reviewer sees the contract without
  // running the comparison.
  assert.equal(DISMISS_TABLE.collapsed["mod-w"], null);
  assert.deepEqual(DISMISS_TABLE.collapsed.escape, { action: "hide-window" });
  assert.deepEqual(DISMISS_TABLE.collapsed["new-command"], { action: "hide-window" });

  assert.equal(DISMISS_TABLE.terminal["mod-w"], null);
  assert.equal(DISMISS_TABLE.terminal.escape, null);
  assert.deepEqual(DISMISS_TABLE.terminal["new-command"], {
    action: "return-to-input",
    reassertOnKeyUp: true,
  });

  assert.deepEqual(DISMISS_TABLE.settings["mod-w"], {
    action: "close-settings",
    stopPropagation: true,
  });
  assert.deepEqual(DISMISS_TABLE.settings.escape, { action: "close-settings" });
  assert.deepEqual(DISMISS_TABLE.settings["new-command"], { action: "close-settings" });

  assert.deepEqual(DISMISS_TABLE.plugin["mod-w"], {
    action: "close-plugin",
    stopPropagation: true,
  });
  assert.deepEqual(DISMISS_TABLE.plugin.escape, { action: "close-plugin" });
  assert.equal(DISMISS_TABLE.plugin["new-command"], null);
});

test("a rebound new-command follows the same table on every surface", () => {
  const rebound: ShortcutMap = { ...shortcuts, new_command: "Ctrl+Q" };
  const ctrlQ = keyEvent({ key: "q", code: "KeyQ", ctrl: true });
  assert.equal(resolveDismissRule("settings", ctrlQ, rebound)?.action, "close-settings");
  assert.equal(resolveDismissRule("collapsed", ctrlQ, rebound)?.action, "hide-window");
  assert.equal(resolveDismissRule("terminal", ctrlQ, rebound)?.action, "return-to-input");
  assert.equal(resolveDismissRule("plugin", ctrlQ, rebound), null);
  // And the old chord no longer dismisses where it used to, exactly as before.
  const ctrlW = keyEvent({ key: "w", code: "KeyW", ctrl: true });
  assert.equal(resolveDismissRule("terminal", ctrlW, rebound), null);
});

test("the literal mod-W rule wins over an overlapping new-command binding", () => {
  // On macOS `new_command` defaults to Cmd+W, so settings Cmd+W matches BOTH
  // the literal chord and the configurable binding. Pre-R7-2 the settings
  // branch tested the literal first, and that is why the press carries
  // `stopPropagation` — the modal layer must not also see it. The trigger
  // order must keep that precedence even though both actions are the same.
  const macLike: ShortcutMap = { ...shortcuts, new_command: "Cmd+W" };
  const cmdW = keyEvent({ key: "w", code: "KeyW", meta: true });
  const rule = resolveDismissRule("settings", cmdW, macLike);
  assert.deepEqual(rule, { action: "close-settings", stopPropagation: true });
  assert.equal(preR72Dismiss("settings", cmdW)?.stop, true, "and this is the old behaviour too");
});

test("only settings yields the whole surface handler to an open modal", () => {
  assert.deepEqual([...MODAL_GUARDED_SURFACES], ["settings"]);
  assert.equal(surfaceYieldsToModal("settings"), true);
  for (const surface of SURFACES.filter((s) => s !== "settings")) {
    assert.equal(surfaceYieldsToModal(surface), false, `${surface} must keep handling keys`);
  }
});

test("isDismissKey is exactly Esc-or-mod-W (the chord the modal layer owns)", () => {
  assert.equal(isDismissKey(keyEvent({ key: "Escape", code: "Escape" })), true);
  assert.equal(isDismissKey(keyEvent({ key: "w", code: "KeyW", meta: true })), true);
  assert.equal(isDismissKey(keyEvent({ key: "w", code: "KeyW", ctrl: true })), true);
  assert.equal(isDismissKey(keyEvent({ key: "w", code: "KeyW" })), false);
  assert.equal(isDismissKey(keyEvent({ key: "Enter", code: "Enter" })), false);
});

test("every surface declares a keyboard owner", () => {
  for (const surface of SURFACES) {
    const policy = SURFACE_FOCUS_POLICY[surface];
    assert.ok(policy, `missing focus policy for ${surface}`);
    assert.ok(policy.owner.length > 0, `${surface} must name an owner`);
  }
  assert.equal(SURFACE_FOCUS_POLICY.settings.owner, "settings-sidebar");
  assert.equal(SURFACE_FOCUS_POLICY.collapsed.owner, "collapsed-input");
  assert.equal(SURFACE_FOCUS_POLICY.terminal.owner, "terminal-canvas");
  assert.equal(SURFACE_FOCUS_POLICY.plugin.owner, "plugin-iframe");
});

test("the declared beats reproduce the pre-R7-2 schedules", () => {
  // Collapsed: the shared collector beat list, plus the Windows retry.
  assert.deepEqual(
    [...SURFACE_FOCUS_POLICY.collapsed.beats],
    [...COLLAPSED_FOCUS_BEATS_MS],
    "the collapsed row must reference the collector's beat list, not a copy",
  );
  assert.equal(SURFACE_FOCUS_POLICY.collapsed.windowsRetry, 180);
  assert.deepEqual(
    surfaceFocusBeats("collapsed"),
    IS_WINDOWS ? [0, 90, 140, 180] : [0, 90, 140],
  );

  // Terminal: focusTerminalView(80), chased once more on Windows.
  assert.deepEqual([...SURFACE_FOCUS_POLICY.terminal.beats], [80]);
  assert.deepEqual(surfaceFocusBeats("terminal"), IS_WINDOWS ? [80, 180] : [80]);

  // Settings claims the keyboard synchronously; plugin's guest document does.
  assert.deepEqual(surfaceFocusBeats("settings"), []);
  assert.deepEqual(surfaceFocusBeats("plugin"), []);
});

test("entering a surface asks exactly its declared owner for the keyboard", () => {
  const calls: string[] = [];
  const seams = {
    focusCollapsedInput: (delay: number) => calls.push(`collapsed:${delay}`),
    focusTerminalView: (delay: number) => calls.push(`terminal:${delay}`),
    focusSettingsSidebar: () => {
      calls.push("settings");
      return true;
    },
  };

  assert.equal(applySurfaceFocusOnEntry("settings", seams), "settings-sidebar");
  assert.deepEqual(calls, ["settings"], "settings must focus its sidebar, and nothing else");
  calls.length = 0;

  assert.equal(applySurfaceFocusOnEntry("terminal", seams), "terminal-canvas");
  assert.deepEqual(calls, surfaceFocusBeats("terminal").map((b) => `terminal:${b}`));
  calls.length = 0;

  assert.equal(applySurfaceFocusOnEntry("collapsed", seams), "collapsed-input");
  assert.deepEqual(calls, surfaceFocusBeats("collapsed").map((b) => `collapsed:${b}`));
  calls.length = 0;

  // The plugin iframe claims its own keyboard; the host must not fight it.
  assert.equal(applySurfaceFocusOnEntry("plugin", seams), "plugin-iframe");
  assert.deepEqual(calls, []);
});

test("focusSettingsSidebar focuses the current page's button and reports it", () => {
  const focused: string[] = [];
  const buttons = new Map(
    (["general", "sessions", "shortcuts", "integrations", "about"] as const).map((page) => [
      page,
      {
        focus: (options?: { preventScroll?: boolean }) => {
          focused.push(`${page}:${options?.preventScroll === true ? "prevent" : "plain"}`);
        },
      } as unknown as HTMLElement,
    ]),
  );

  assert.equal(focusSettingsSidebar(buttons, "shortcuts"), true);
  assert.deepEqual(focused, ["shortcuts:prevent"]);

  // A page whose button is not mounted yet must not throw.
  assert.equal(
    focusSettingsSidebar({ get: () => undefined }, "about"),
    false,
  );
});

test("the sidebar is a roving tab stop: only the active page is tabbable", () => {
  assert.equal(settingsSidebarTabIndex("general", "general"), 0);
  assert.equal(settingsSidebarTabIndex("about", "general"), -1);
  for (const page of ["general", "sessions", "shortcuts", "integrations", "about"] as const) {
    for (const active of ["general", "about"] as const) {
      assert.equal(settingsSidebarTabIndex(page, active), page === active ? 0 : -1);
    }
  }
});

// ---------------------------------------------------------------------------
// Source shape: the call sites actually consult the tables.
// ---------------------------------------------------------------------------

const readCode = async (path: string) =>
  (await read(path))
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/[^\n]*/g, "");

test("App's mode effect focuses the settings sidebar through the policy", async () => {
  const app = await readCode("src/App.tsx");
  assert.match(
    app,
    /applySurfaceFocusOnEntry\("settings"/,
    "entering settings must run the declared focus policy",
  );
  assert.match(
    app,
    /focusSettingsSidebar\(settingsSidebarButtons\.current/,
    "the settings owner must be the sidebar button for the current page",
  );
  // The mode effect must still return before the collapsed/terminal work, and
  // must not reach for any native focus command.
  const start = app.indexOf('if (mode === "settings")');
  assert.ok(start > -1, "the settings branch must exist");
  const branch = app.slice(start, app.indexOf("if (mode === \"plugin\")", start));
  assert.doesNotMatch(branch, /refocus_webview|set_focus|make_key|show_input/);
});

test("the settings sidebar buttons carry the roving tabIndex", async () => {
  const app = await readCode("src/App.tsx");
  assert.match(
    app,
    /tabIndex=\{settingsSidebarTabIndex\(page,\s*settingsPage\)\}/,
    "the sidebar must derive its tabIndex from the shared roving helper",
  );
});

test("useAppKeyboard no longer hand-writes the dismiss branches", async () => {
  const keyboard = await readCode("src/hooks/useAppKeyboard.ts");
  assert.match(
    keyboard,
    /resolveDismissRule\(/,
    "the window handler must resolve Esc/⌘W through the table",
  );
  assert.equal(
    /event\.key\.toLowerCase\(\) === "w"/.test(keyboard),
    false,
    "the literal Cmd/Ctrl+W test must live in the table module only",
  );
  assert.equal(
    /if \(mode === "settings"\) \{\s*if \(target\?\.closest/.test(keyboard),
    false,
    "the settings branch must not open with its own modal test",
  );
});

test("the modal layer shares the same dismiss predicate", async () => {
  const panel = await readCode("src/ExtensionsPanel.tsx");
  assert.match(
    panel,
    /isDismissKey\(event\)/,
    "the dialog must use the shared Esc/Cmd+W predicate",
  );
  assert.equal(
    /event\.metaKey \|\| event\.ctrlKey\)\s*&&\s*event\.key\.toLowerCase\(\) === "w"/.test(panel),
    false,
    "the dialog must not carry its own copy of the chord",
  );
});

test("surface-policy keeps the collapsed collector as the collapsed enforcer", async () => {
  const policy = await readCode("src/surface-policy.ts");
  assert.match(
    policy,
    /beats:\s*COLLAPSED_FOCUS_BEATS_MS/,
    "the collapsed row must reference the collector's beat list",
  );
  assert.doesNotMatch(
    policy,
    /shouldReclaimCollapsedFocus|createCollapsedFocusController/,
    "the table declares policy; collapsed-focus.ts keeps enforcing it",
  );
});

test("a focused sidebar item is visible (existing :focus-visible token rule)", async () => {
  // The new initial focus must never be invisible. The rule predates this
  // round; this assertion keeps the round from removing it while making the
  // sidebar the surface's keyboard home. R7-HIG moved the ring's width and
  // offset onto shared tokens so every control uses one focus language; the
  // assertion follows the token and pins its value in base.css below.
  const css = await read("src/styles/settings.css");
  assert.match(
    css,
    /\.settings-sidebar__item:focus-visible\s*\{[^}]*outline:\s*var\(--focus-ring-width\)\s*solid\s*var\(--accent-ring\)/,
    "the sidebar item needs a token-based :focus-visible outline",
  );
  // The token really is a 2px ring, so the sidebar's focus is as visible as
  // it was before the convergence.
  const base = await read("src/styles/base.css");
  assert.match(base, /--focus-ring-width:\s*2px;/, "the shared focus ring must stay 2px");
});
