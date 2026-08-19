import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);

test("production Tauri configuration keeps a restrictive WebView CSP", async () => {
  const config = JSON.parse(await readFile(new URL("src-tauri/tauri.conf.json", root), "utf8")) as {
    app?: { security?: { csp?: unknown } };
  };
  const csp = config.app?.security?.csp;
  assert.equal(typeof csp, "string");
  assert.match(csp, /default-src 'self'/);
  assert.match(csp, /object-src 'none'/);
  assert.match(csp, /connect-src[^;]*ipc:/);
  assert.doesNotMatch(csp, /script-src[^;]*unsafe-inline/);
  assert.doesNotMatch(csp, /default-src[^;]*\*/);
});

test("capabilities do not expose unused shell, opener, or store permissions", async () => {
  const capabilities = JSON.parse(
    await readFile(new URL("src-tauri/capabilities/default.json", root), "utf8"),
  ) as { permissions?: unknown };
  const permissions = capabilities.permissions;
  assert.ok(Array.isArray(permissions));
  assert.equal(permissions.some((permission) => /^(shell|opener|store):/.test(String(permission))), false);
});
