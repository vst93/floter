// Regression guard for "multi-word simple commands must run with shell
// semantics". The launcher builds a STRUCTURED execution plan (program + argv)
// for a simple line like `go version`, and hands a shell-syntax line (pipes,
// redirects) to the shell verbatim.
//
// Scope: these tests lock the STRUCTURED parser contract only — that
// `parseCommandLine` splits a simple multi-word line into separate tokens and
// that the catalog match carries the extra words as a per-token argument
// override. They do NOT exercise the raw-line injection path (the interactive
// shell receiving `initial_command` byte-for-byte); that path's byte-exactness
// is covered by the Rust unit test
// `broker::tests::initial_command_payload_is_verbatim_bytes_plus_carriage_return`.
//
// The contract has two halves, and the tests below fail the moment either is
// broken:
//   1. `parseCommandLine` splits a simple multi-word line into separate tokens
//      (never one fused/quoted token).
//   2. The catalog match carries the extra words as an ARGUMENT OVERRIDE
//      (a per-token array), so the plan's argv is `["version"]` — not a single
//      `["go version"]` argument.
//
// Mutation to make this fail: have `parseCommandLine` return
// `tokens: [value]` for a quote-free line (or have the override builder join
// the tail into one string). Both assertions on `tokens` / `argumentOverride`
// then break.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  parseCommandLine,
  type ExecutionPlan,
} from "../src/launcher.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** Mirror of the match-with-arguments builder in `useLauncherCatalog`: a
 *  catalog entry whose recorded plan has no user arguments gets an
 *  `argumentOverride` built from the parsed tokens AFTER the command word.
 *  This is the STRUCTURED path the search box uses. */
const planWithUserArguments = (
  plan: ExecutionPlan,
  line: string,
): ExecutionPlan => {
  const parsed = parseCommandLine(line);
  if (parsed.commandIndex === null) return plan;
  const userArgs = parsed.tokens.slice(parsed.commandIndex + 1);
  if (!userArgs.length) return plan;
  return { ...plan, argumentOverride: userArgs };
};

const basePlan = (program: string): ExecutionPlan => ({
  program,
  args: [],
  mode: "pty",
  cwd: null,
  environment: {},
  inheritEnvironment: true,
});

test("simple multi-word lines split into separate argv tokens", () => {
  for (const [line, program, args] of [
    ["go version", "go", ["version"]],
    ["cargo build", "cargo", ["build"]],
    ["npm run build", "npm", ["run", "build"]],
    ["git commit -m fix", "git", ["commit", "-m", "fix"]],
  ] as const) {
    const parsed = parseCommandLine(line);
    assert.equal(parsed.shellSyntax, false, `${line} is not shell syntax`);
    assert.equal(parsed.commandIndex, 0, `${line} command starts at token 0`);
    assert.deepEqual(parsed.tokens, [program, ...args], `${line} tokens`);
    // Nothing is quoted or fused: the shell must never receive the whole line
    // as one word.
    assert.ok(
      !parsed.tokens.some((token) => token.includes(" ")),
      `${line} must not fold words into one token`,
    );
    assert.ok(
      !parsed.tokens.some((token) => /^['"]/.test(token)),
      `${line} must not quote the command line`,
    );
  }
});

test("a catalog match carries the extra words as an argument override", () => {
  // The `go version` case: a `go` catalog entry with no recorded arguments.
  const plan = planWithUserArguments(basePlan("go"), "go version");
  assert.deepEqual(plan.args, [], "the base plan's args are untouched");
  assert.deepEqual(plan.argumentOverride, ["version"], "the extra word is argv");
  // The override is per-token: `npm run build` yields two arguments, not one
  // quoted `"run build"`.
  assert.deepEqual(
    planWithUserArguments(basePlan("npm"), "npm run build").argumentOverride,
    ["run", "build"],
  );
  // A bare command has nothing to override.
  assert.equal(
    planWithUserArguments(basePlan("go"), "go").argumentOverride,
    undefined,
  );
});

test("environment assignments keep the command index and stay out of argv", () => {
  const parsed = parseCommandLine("GOFLAGS=-mod=mod go version");
  assert.equal(parsed.commandIndex, 1, "leading NAME=value is not the command");
  assert.deepEqual(parsed.environment, { GOFLAGS: "-mod=mod" });
  // The structured plan for `go version` under an env prefix is still argv
  // `["version"]` — the assignment is carried separately, never glued on.
  assert.deepEqual(
    planWithUserArguments(basePlan("go"), "GOFLAGS=-mod=mod go version")
      .argumentOverride,
    ["version"],
  );
});

test("shell-syntax lines are flagged so they are handed to the shell intact", () => {
  // `go version | cat` cannot be a structured plan; the flag is what routes it
  // to the shell as the raw line (which the shell then parses correctly).
  assert.equal(parseCommandLine("go version | cat").shellSyntax, true);
  assert.equal(parseCommandLine("go version > out.txt").shellSyntax, true);
});

// ── R9-2 slice 5 · the terminal page stays resident after the PTY exits ─────
//
// The user's feedback: "after it runs, the output must stay on the terminal —
// it must not just exit". The chosen fix is the least invasive one: the
// terminal page keeps its last frame and shows an exit line instead of
// collapsing, so the user reads the output and closes the page themselves.
//
// Mutation: have `handleTerminalExit` call `closeTerminalSession()` and
// `setMode("collapsed")` again (the pre-slice behaviour) and every assertion
// below goes red — the view would vanish before the output could be read.

test("the terminal exit handler holds the page instead of collapsing it", async () => {
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const at = hook.indexOf("const handleTerminalExit");
  assert.notEqual(at, -1, "the exit handler must exist");
  const handler = hook.slice(at, hook.indexOf("const describeMainSession", at));
  // The frame and the mounted page survive; only the input slot is released.
  assert.match(handler, /ptyReady\.current = false/, "the exit must release the input slot");
  assert.match(handler, /setTerminalResident\(\{ code \}\)/, "the exit must enter the resident state");
  assert.ok(
    !/setTerminalMounted\(false\)/.test(handler),
    "the page must NOT unmount on exit",
  );
  assert.ok(
    !/setMode\("collapsed"\)/.test(handler),
    "the exit must NOT collapse the launcher",
  );
  assert.ok(
    !/closeTerminalSession\(\)/.test(handler),
    "the exit must not tear the session down (that would drop the last frame)",
  );
  // The exit event passes the code through so the notice can name it.
  assert.match(
    hook,
    /handleTerminalExit\(event\.payload\.code\)/,
    "the exit event must forward the real exit code",
  );
});

test("the resident notice is bilingual and colours a non-zero exit", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /terminalResident &&/, "the notice must render only while resident");
  assert.match(app, /terminal\.processExited/, "the notice must name the exit");
  assert.match(app, /terminal\.processExitedHint/, "the notice must state the close affordance");
  // A non-zero code reads in the warning colour; a clean exit does not.
  assert.match(
    app,
    /terminalResident\.code !== null && terminalResident\.code !== 0 \? " terminal-resident--warning" : ""/,
    "a non-zero exit must be visually flagged",
  );
  const css = stripJsComments(await read("src/styles/terminal.css"));
  const warning = css.slice(css.indexOf(".terminal-resident--warning .terminal-resident__title"));
  assert.match(warning.slice(0, warning.indexOf("}")), /color:\s*var\(--text-warning\)/, "the warning state must use the warning token");
  // The resident line must not become an overlay the user has to dismiss.
  const rule = css.slice(css.indexOf(".terminal-resident {"));
  assert.match(rule.slice(0, rule.indexOf("}")), /pointer-events:\s*none/, "the notice must not trap the pointer");
  assert.ok(!/role="dialog"|aria-modal/.test(app.slice(app.indexOf("terminalResident &&"), app.indexOf("terminalResident &&") + 600)), "the resident notice must not be a dialog");
  // Both dictionaries carry the new keys.
  const i18n = await read("src/i18n.ts");
  for (const key of ["terminal.processExited", "terminal.processExitedHint"]) {
    assert.equal(i18n.split(`"${key}"`).length - 1, 2, `${key} must be declared once per language`);
  }
});

test("a fresh spawn clears the resident state before reusing the view", async () => {
  // The exit notice belongs to the session that produced it; starting a new
  // one must not leave a stale "process exited" line over a live PTY.
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const at = hook.indexOf("const ensureTerminalSession");
  const ensure = hook.slice(at, hook.indexOf("const handleTerminalExit", at));
  assert.match(ensure, /setTerminalResident\(null\)/, "a new spawn must clear the resident notice");
});

test("residency does not touch the argv contract", async () => {
  // The fix is entirely frontend-side (plan b): no shell string is ever built.
  // The Rust run module must keep handing over a protected plan and the hook
  // must never assemble argv — the same guarantees the earlier slices locked.
  const run = stripJsComments(await read("src-tauri/src/extensions/run.rs"));
  assert.match(run, /fn param_arguments\(/, "the structured argv builder must remain");
  assert.match(run, /let protected = state\.protect_execution_plan\(plan\)\?;/, "the terminal route must still protect the plan");
  assert.ok(
    !/sh -c|\/bin\/sh -c|format!\("\{\}"\s*,?\s*args/.test(run),
    "residency must not introduce a shell string join",
  );
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  assert.ok(!/\.join\(/.test(hook), "the hook must not join argv into a command string");
});
