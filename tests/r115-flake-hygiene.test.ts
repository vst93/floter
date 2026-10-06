// R115 · flake root-cause slice 1 — committed fixtures, not run-time writes.
//
// R114 measured two registered flakes (probe_runner 14/340, capability_probe
// 19/200 under a parallel `cargo test`) down to one root cause: a test writes
// an executable and execs it, and a sibling test's fork inherits the write fd
// until its own exec, so Linux refuses the exec with ETXTBSY. The fix is to
// stop writing executables at run time: every helper is a committed fixture
// under `tests/fixtures/`, exec'd in place. This suite pins the files the slice
// touched, plus the two realism fixes (R-C) that ride along.
//
// Every banned literal is assembled from fragments so this guard does not
// contain the very tokens it bans — its own last test proves that.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const STAGE_FIXTURE = "stage_" + "fixture(";
const ONCE_LOCK_FIXTURE = "Once" + "Lock<Fixture>";
const OLD_CLAIM = "prevents parallel test " + "processes";
const OLD_TAUTOLOGY = "assert_eq!(before, discovery_" + "signature";
const INJECTION_SEAM = "fn signature_" + "of(";
const DROP_GUARD = "impl Drop for Restore" + "PermissionParent";
const CLEANUP_FIXTURE = "tests/fixtures/" + "provider-cleanup.sh";
const CAPABILITY_TOOL_FIXTURE = "tests/fixtures/" + "capability-probe-tool.sh";
const TIMEOUT_FIXTURE = "tests/fixtures/" + "capability-timeout.sh";

test("probe_runner execs the committed cleanup fixture, never a staged copy", async () => {
  const source = await read("src-tauri/src/extensions/probe_runner.rs");
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.ok(
    source.includes(CLEANUP_FIXTURE),
    "the committed cleanup fixture must be referenced by path",
  );
  assert.equal(count(source, STAGE_FIXTURE), 0, "no run-time fixture staging may return");
});

test("capability_probe execs committed fixtures and retires the OnceLock copy", async () => {
  const source = await read("src-tauri/src/extensions/capability_probe.rs");
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.ok(
    source.includes(CAPABILITY_TOOL_FIXTURE),
    "the committed probe tool must be referenced by path",
  );
  assert.ok(
    source.includes(TIMEOUT_FIXTURE),
    "the committed timeout fixture must be referenced by path",
  );
  assert.equal(count(source, ONCE_LOCK_FIXTURE), 0, "the OnceLock fixture copy must be gone");
  assert.equal(count(source, STAGE_FIXTURE), 0, "no run-time fixture staging may return");
  assert.equal(count(source, OLD_CLAIM), 0, "the falsified ETXTBSY claim must be gone");
});

test("discover's no-op signature invariance hashes an injected directory list", async () => {
  const source = await read("src-tauri/src/browser_data/discover.rs");
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.ok(source.includes(INJECTION_SEAM), "the signature must expose an injection seam");
  assert.equal(count(source, OLD_TAUTOLOGY), 0, "the real-$HOME tautology must be gone");
});

test("provider's permission test restores the host environment on every exit", async () => {
  const source = await read("src-tauri/src/extensions/provider.rs");
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.ok(source.includes(DROP_GUARD), "a Drop guard must restore FLOTER_PERMISSION_PARENT");
});

test("this guard assembles its banned tokens, it does not spell them", async () => {
  const self = await read("tests/r115-flake-hygiene.test.ts");
  for (const token of [STAGE_FIXTURE, ONCE_LOCK_FIXTURE, OLD_CLAIM, OLD_TAUTOLOGY]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token}`);
  }
});
