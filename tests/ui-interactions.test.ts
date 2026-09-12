import assert from "node:assert/strict";
import test from "node:test";
import { parseCommandLine } from "../src/launcher.ts";

test("command with arguments should extract command name for catalog search", () => {
  // 问题6：命令带参数时应该能正确识别命令名
  const parsed1 = parseCommandLine("git status", false, "posix");
  assert.equal(parsed1.commandIndex, 0);
  assert.equal(parsed1.tokens[0], "git");
  assert.deepEqual(parsed1.tokens, ["git", "status"]);

  const parsed2 = parseCommandLine("npm install lodash", false, "posix");
  assert.equal(parsed2.commandIndex, 0);
  assert.equal(parsed2.tokens[0], "npm");
  assert.deepEqual(parsed2.tokens, ["npm", "install", "lodash"]);

  // 带环境变量
  const parsed3 = parseCommandLine("DEBUG=1 node index.js", false, "posix");
  assert.equal(parsed3.commandIndex, 1);
  assert.equal(parsed3.tokens[parsed3.commandIndex], "node");
});

test("command parser should handle shell operators correctly", () => {
  const parsed1 = parseCommandLine("git log | grep foo", false, "posix");
  assert.equal(parsed1.shellSyntax, true);

  const parsed2 = parseCommandLine("echo 'test | pipe'", false, "posix");
  assert.equal(parsed2.shellSyntax, false);
});
