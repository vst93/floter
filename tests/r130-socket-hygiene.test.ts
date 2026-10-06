// R130 · the vendored qscreen daemon's Unix socket node is owner-only.
//
// R105 missed this one: the daemon binds a Unix domain socket whose protocol
// accepts `SpawnCommand` (spawn an arbitrary program as this user), `Attach` and
// `Input` (write into a PTY), with no peer-credential check anywhere. The node's
// own mode is the only line between another local user and the daemon. R106
// landed the identical `0600` on the app's own control socket
// (`src-tauri/src/ipc.rs`); this round lands it on the daemon's node, in the one
// `bind_socket` helper every successful bind (fresh or reclaimed) funnels
// through.
//
// This guard pins the chmod's position relative to the bind and the accept loop,
// keeps the R106 precedent a live anchor rather than a dead name, and pins the
// two vendor tests that prove the mode at run time. Every scanned token is
// assembled from fragments so the guard never spells the literals it looks for —
// its own last test proves that.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;
const lines = (source: string) => source.split("\n");

/** The 1-based line number carrying the first `needle`. */
const firstLine = (source: string, needle: string) => {
  const index = lines(source).findIndex((line) => line.includes(needle));
  assert.notEqual(index, -1, `expected to find ${needle}`);
  return index + 1;
};

const DAEMON = "src-tauri/vendor/qscreen-daemon/src/lib.rs";
const IPC = "src-tauri/src/ipc.rs";

// Assembled so this guard does not spell what it scans for.
const BIND_CALL = "UnixListener::" + "bind(";
const RESTRICT = "restrict_" + "to_owner";
const FROM_MODE = "from_" + "mode(";
const OWNER_MODE = "0o6" + "00";
const WIDE_MODE = "0o6" + "66";
const OWNER_TEST = "the_daemon_socket_node_" + "is_owner_only";
const RECLAIM_TEST = "a_reclaimed_socket_node_" + "is_owner_only_too";
const IPC_TEST = "the_socket_node_" + "is_owner_only";

test("the daemon binds through exactly one call, then restricts the node", async () => {
  const source = await read(DAEMON);
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");

  const bind = firstLine(source, BIND_CALL);
  const chmod = firstLine(source, FROM_MODE);

  // A second bind success path would be a way around the chmod; there is none,
  // so the fresh bind and the reclaim (`remove_file` then bind) both end here.
  assert.equal(count(source, BIND_CALL), 1, "the daemon must bind through one call only");
  assert.ok(chmod > bind, `the chmod (line ${chmod}) must follow the bind (line ${bind})`);
  assert.ok(chmod - bind <= 4, "the chmod must sit immediately after the bind");

  // The owner-only literal lives in the bind region, not somewhere else.
  const region = lines(source).slice(bind - 1, chmod).join("\n");
  assert.ok(region.includes(FROM_MODE), "the bind region must call the mode helper");
  assert.ok(region.includes(OWNER_MODE), "the bind region must pull the node to owner-only");
});

test("the chmod lands before the accept loop, not inside one branch", async () => {
  const source = await read(DAEMON);
  const bind = firstLine(source, BIND_CALL);
  const chmod = firstLine(source, FROM_MODE);

  const afterBind = lines(source).slice(bind - 1);
  const loopOffset = afterBind.findIndex((line) => line.includes("loop {"));
  assert.notEqual(loopOffset, -1, "the accept loop must follow the bind");
  const loop = bind + loopOffset;

  assert.ok(chmod < loop, `the chmod (line ${chmod}) must land before the loop (line ${loop})`);
});

test("the reclaim path unlinks before it rebinds, so it takes the same chmod", async () => {
  const source = await read(DAEMON);
  const remove = firstLine(source, "remove_file(&pipe)");
  const call = firstLine(source, "bind_socket(&pipe)");

  assert.ok(remove < call, `the reclaim unlink (line ${remove}) must precede the rebind (line ${call})`);
});

test("the R106 precedent on the app's own socket is still live", async () => {
  const source = await read(IPC);
  assert.ok(source.includes("mod tests"), "the precedent must be a live file");
  assert.ok(source.includes(RESTRICT), "the R106 helper must still exist");
  assert.ok(source.includes(OWNER_MODE), "the R106 helper must still pull the node to owner-only");
  assert.ok(source.includes(IPC_TEST), "the R106 owner-only test must still exist");
});

test("the two vendor tests that prove the mode at run time are present", async () => {
  const source = await read(DAEMON);
  assert.ok(source.includes(OWNER_TEST), `the fresh-bind test ${OWNER_TEST} must exist`);
  assert.ok(source.includes(RECLAIM_TEST), `the reclaim test ${RECLAIM_TEST} must exist`);
  // The reclaim test widens the stale node first, so it cannot pass by
  // inheriting the original bind's owner-only mode.
  assert.ok(source.includes(WIDE_MODE), "the reclaim test must widen the stale node first");
});

test("this guard assembles its scanned tokens, it does not spell them", async () => {
  const self = await read("tests/r130-socket-hygiene.test.ts");
  for (const token of [
    BIND_CALL,
    RESTRICT,
    FROM_MODE,
    OWNER_MODE,
    WIDE_MODE,
    OWNER_TEST,
    RECLAIM_TEST,
    IPC_TEST,
  ]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token}`);
  }
});
