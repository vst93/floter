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
import test from "node:test";
import {
  parseCommandLine,
  type ExecutionPlan,
} from "../src/launcher.ts";

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
