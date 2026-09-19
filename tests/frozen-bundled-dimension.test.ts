// R-FREEZE-2 · the bundled-distribution dimension was physically removed.
//
// The NPM distribution (`Distribution::Npm`), the bundled runtime
// (`Runtime::Bundled` + `platformPackages`), the `ResolvedManifest`
// `platformPackage` projection, the artifact shim chain (`artifacts.rs`), the
// legacy asset matcher (`asset_matcher.rs`) and the resolver's profile context
// (`profile.rs`) were one load-bearing dead cluster: the shim activation path
// was itself guarded to be a no-op for every local integration, and the two
// production call sites therefore did nothing. Removing only the enum would
// have orphaned the rest, so the whole dimension went at once.
//
// These are the freeze locks. They scan the real sources rather than asserting
// a constant, so reviving any removed name — in Rust or in the manifest schema
// — turns them red.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const missing = (path: string) =>
  assert.rejects(read(path), /ENOENT/, `${path} must not exist after R-FREEZE-2`);

const stripRustComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const rustSources = [
  "src-tauri/src/extensions/manifest.rs",
  "src-tauri/src/extensions/lock.rs",
  "src-tauri/src/extensions/transaction.rs",
  "src-tauri/src/extensions/registry.rs",
  "src-tauri/src/extensions/conformance.rs",
  "src-tauri/src/extensions/install.rs",
  "src-tauri/src/extensions/sync.rs",
  "src-tauri/src/extensions/resolver.rs",
  "src-tauri/src/commands/extensions.rs",
];

test("the removed modules are physically gone", async () => {
  for (const path of [
    "src-tauri/src/extensions/artifacts.rs",
    "src-tauri/src/extensions/asset_matcher.rs",
    "src-tauri/src/extensions/profile.rs",
  ]) {
    await missing(path);
  }
});

test("no source revives an npm distribution, bundled runtime, or shim path", async () => {
  const forbidden = [
    "Distribution::Npm",
    "Runtime::Bundled",
    "platform_package",
    "platformPackages",
    "platform_packages",
    "ArtifactBinary",
    "Artifacts::",
    "BinaryRole",
    "AssetSelection",
    "activate_entry_shims",
    "prepare_shim_metadata",
    "verify_binaries",
    "profile::Profile",
    "use super::profile",
  ];
  for (const path of rustSources) {
    const source = stripRustComments(await read(path));
    for (const token of forbidden) {
      // `platformPackages` legitimately survives only in the legacy-manifest
      // rejection test inside manifest.rs, where it proves the bundled runtime
      // is now rejected. Allow that one negative assertion.
      if (token === "platformPackages" && path.endsWith("manifest.rs")) {
        const uses = source.split(token).length - 1;
        assert.ok(
          uses <= 1,
          `${path} must reference platformPackages only in the rejection lock (found ${uses})`,
        );
        continue;
      }
      // lock.rs carries the serde compatibility lock: it deliberately feeds a
      // legacy `assetSelection` object and asserts it is dropped on re-serialize.
      // That Rust test (not this scan) is what turns red if the field is revived.
      if (
        path.endsWith("lock.rs") &&
        (token === "asset_selection" || token === "AssetSelection")
      ) {
        continue;
      }
      assert.ok(
        !source.includes(token),
        `${path} must not reference the removed \`${token}\``,
      );
    }
  }
});

test("the manifest schema no longer accepts npm or a bundled runtime", async () => {
  const schema = await read("docs/extensions/schemas/floter-extension.schema.json");
  assert.ok(!schema.includes('"npm"'), "the npm distribution must leave the schema enum");
  assert.ok(!schema.includes("bundledRuntime"), "the bundled runtime def must be deleted");
  assert.ok(!schema.includes('"bundled"'), "no bundled runtime variant may remain");
  assert.ok(
    !schema.includes('"artifacts"'),
    "the artifacts block must leave the schema properties",
  );
  assert.ok(schema.includes('"local"'), "the local distribution must stay");
  assert.ok(schema.includes('"built-in"'), "the built-in distribution must stay");
});

test("the shipped manifest fixtures are local system integrations", async () => {
  for (const path of [
    "docs/extensions/examples/v/floter.extension.json",
    "docs/extensions/sdk/fixtures/valid/floter.extension.json",
  ]) {
    const manifest = JSON.parse(await read(path)) as {
      distribution: { type: string };
      runtime: { type: string };
      artifacts?: unknown;
    };
    assert.equal(manifest.distribution.type, "local", `${path} must be a local distribution`);
    assert.equal(manifest.runtime.type, "system", `${path} must use a system runtime`);
    assert.equal(manifest.artifacts, undefined, `${path} must carry no artifacts block`);
  }
});
