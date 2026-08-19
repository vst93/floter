import assert from "node:assert/strict";
import test from "node:test";
import {
  classifyActionBar,
  completedCommandLine,
  executionWithCompletion,
  nextLauncherSelection,
  normalizeSearch,
  parseCommandLine,
  scoreApp,
  scoreNormalized,
  shouldDefaultToActionBar,
  type CompletionItem,
  type ExecutionPlan,
} from "../src/launcher.ts";

const completion = (value: string): CompletionItem => ({ value, label: value, description: "" });

test("command parsing preserves quoted and escaped arguments", () => {
  assert.deepEqual(parseCommandLine(`git commit -m "hello world" path\\ with\\ spaces`).tokens, [
    "git",
    "commit",
    "-m",
    "hello world",
    "path with spaces",
  ]);
  assert.deepEqual(parseCommandLine(`tool 'single \\ slash' "nested ' quote"`).tokens, [
    "tool",
    "single \\ slash",
    "nested ' quote",
  ]);
});

test("completion parsing exposes an empty trailing argument and its insertion point", () => {
  assert.deepEqual(parseCommandLine("git checkout ", true), {
    tokens: ["git", "checkout", ""],
    fragmentStart: 13,
  });
  assert.deepEqual(parseCommandLine("git checkout fe", true), {
    tokens: ["git", "checkout", "fe"],
    fragmentStart: 13,
  });
});

test("completion replaces only the active fragment and quotes shell-sensitive values", () => {
  assert.equal(completedCommandLine("tool open old", 10, completion("My File.txt")), `tool open "My File.txt" `);
  assert.equal(completedCommandLine("tool open ", 10, completion("My Folder/")), `tool open "My Folder/"`);
  const special = `a"b\\c$HOME;next`;
  const completed = completedCommandLine("tool open ", 10, completion(special));
  assert.equal(completed, `tool open "a\\"b\\\\c\\$HOME;next" `);
  assert.deepEqual(parseCommandLine(completed).tokens, ["tool", "open", special]);
});

test("completion updates argument overrides without mutating the protected plan", () => {
  const plan: ExecutionPlan = {
    program: "/usr/bin/tool",
    args: ["base"],
    mode: "pty",
    cwd: null,
    environment: {},
    inheritEnvironment: false,
    planToken: "one-shot",
  };
  const completed = executionWithCompletion({ execution: plan }, ["tool", "open", "ol"], completion("new"));
  assert.deepEqual(completed?.argumentOverride, ["open", "new"]);
  assert.equal(completed?.planToken, "one-shot");
  assert.equal(plan.argumentOverride, undefined);
});

test("search normalization and scoring keep visible names ahead of hidden aliases", () => {
  assert.equal(normalizeSearch("  Ｖ-Space.app  "), "v space app");
  assert.equal(scoreNormalized("code", "code"), 1000);
  assert.ok(scoreNormalized("code", "code runner") > scoreNormalized("code", "my code runner"));
  assert.ok(scoreNormalized("vsc", "visual studio code") > 0);
  assert.equal(scoreNormalized("xyz", "visual studio code"), 0);

  assert.equal(scoreApp("wxwork", ["企业微信"], "qywx", ["wxwork"]), 690);
  assert.equal(scoreApp("qywx", ["企业微信"], "qywx", ["wxwork"]), 950);
  assert.equal(scoreApp("wok", ["企业微信"], "qywx", ["wxwork"]), 0);
});

test("action bar classification is conservative about URLs and paths", () => {
  assert.equal(classifyActionBar("https://example.com"), "url");
  assert.equal(classifyActionBar("mailto:user@example.com"), "shell");
  assert.equal(classifyActionBar("~/Documents"), "path");
  assert.equal(classifyActionBar("C:\\Users\\name"), "path");
  assert.equal(classifyActionBar("Documents"), "shell");
});

test("fresh selection favors structured results unless the query is clearly a command", () => {
  assert.equal(shouldDefaultToActionBar("code", "shell", 1, 1, false), false);
  assert.equal(shouldDefaultToActionBar("git", "shell", 1, 1, false), true);
  assert.equal(shouldDefaultToActionBar("git status", "shell", 1, 1, false), true);
  assert.equal(shouldDefaultToActionBar("git status", "shell", 2, 2, true), false);
  assert.equal(shouldDefaultToActionBar("https://example.com", "url", 1, 1, false), true);
  assert.equal(shouldDefaultToActionBar("unknown", "shell", 0, 0, false), true);
});

test("unavailable catalog commands never capture the default Enter action", () => {
  assert.equal(shouldDefaultToActionBar("tool", "shell", 1, 0, false), true);
  // A runnable application can still win for an ordinary name.
  assert.equal(shouldDefaultToActionBar("code", "shell", 2, 1, false), false);
  // A known shell command stays a command even if an application also matched.
  assert.equal(shouldDefaultToActionBar("git", "shell", 2, 1, false), true);
});

test("keyboard navigation skips unavailable results in both directions", () => {
  const runnable = [false, true, false, true];
  assert.deepEqual(nextLauncherSelection(runnable, 1, false, true, 1), {
    actionBar: false,
    resultIndex: 3,
  });
  assert.deepEqual(nextLauncherSelection(runnable, 3, false, true, -1), {
    actionBar: false,
    resultIndex: 1,
  });
  assert.deepEqual(nextLauncherSelection(runnable, 3, false, true, 1), {
    actionBar: true,
    resultIndex: 3,
  });
  assert.deepEqual(nextLauncherSelection(runnable, 3, true, true, 1), {
    actionBar: false,
    resultIndex: 1,
  });
});

test("keyboard navigation recovers when the selected result becomes unavailable", () => {
  const runnable = [true, false, true];
  assert.deepEqual(nextLauncherSelection(runnable, 1, false, true, 1), {
    actionBar: false,
    resultIndex: 2,
  });
  assert.deepEqual(nextLauncherSelection(runnable, 1, false, true, -1), {
    actionBar: false,
    resultIndex: 0,
  });
  assert.deepEqual(nextLauncherSelection([false], 0, false, true, -1), {
    actionBar: true,
    resultIndex: 0,
  });
});
