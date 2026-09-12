import { describe, it } from "node:test";
import assert from "node:assert";
import { parseCommandLine } from "../src/launcher.ts";

describe("Command line parsing with arguments", () => {
  it("should parse command without arguments correctly", () => {
    const result = parseCommandLine("git", false, "posix");
    assert.strictEqual(result.shellSyntax, false);
    assert.strictEqual(result.commandIndex, 0);
    assert.deepStrictEqual(result.tokens, ["git"]);
  });

  it("should parse command with arguments correctly", () => {
    const result = parseCommandLine("git status", false, "posix");
    assert.strictEqual(result.shellSyntax, false);
    assert.strictEqual(result.commandIndex, 0);
    assert.deepStrictEqual(result.tokens, ["git", "status"]);
  });

  it("should parse command with multiple arguments", () => {
    const result = parseCommandLine("npm install --save-dev", false, "posix");
    assert.strictEqual(result.shellSyntax, false);
    assert.strictEqual(result.commandIndex, 0);
    assert.deepStrictEqual(result.tokens, ["npm", "install", "--save-dev"]);
  });

  it("should parse command with quoted arguments", () => {
    const result = parseCommandLine('echo "hello world"', false, "posix");
    assert.strictEqual(result.shellSyntax, false);
    assert.strictEqual(result.commandIndex, 0);
    assert.deepStrictEqual(result.tokens, ["echo", "hello world"]);
  });

  it("should detect shell syntax with pipes", () => {
    const result = parseCommandLine("ls | grep test", false, "posix");
    assert.strictEqual(result.shellSyntax, true);
  });

  it("should parse environment variables before command", () => {
    const result = parseCommandLine("DEBUG=true npm start", false, "posix");
    assert.strictEqual(result.shellSyntax, false);
    assert.strictEqual(result.commandIndex, 1);
    assert.deepStrictEqual(result.tokens, ["DEBUG=true", "npm", "start"]);
    assert.deepStrictEqual(result.environment, { DEBUG: "true" });
  });
});
