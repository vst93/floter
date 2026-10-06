// R117 · `stage_fixture` census — the A-class migration left no run-time
// staging behind, and the surviving call sites are pinned.
//
// R116 classified the 41 remaining staging call sites into A (a fixed fixture
// is exec'd and never rewritten → migrate to exec the committed fixture in
// place), B (the test needs a per-test writable path: in-place upgrade, delete,
// or two same-basename copies), C (rewritten but never exec'd) and D (never
// exec'd). R117 migrated all 19 A sites, converged the two `stage_fixture`
// bodies into one, and left B/C/D alone. This guard pins the survivors as exact
// line lists: any new run-time staging call — or any moved one — must be
// re-registered here first. B is out of scope this round, so its sites are
// pinned by position only, never re-interpreted.
//
// Every banned/expected token is assembled from fragments so this guard does
// not spell the very tokens it scans for — its own last test proves that.
import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const STAGE_FIXTURE = "stage_" + "fixture(";
const STAGE_FIXTURE_DEF = "fn " + STAGE_FIXTURE;
const EXEMPTION = "R117 " + "exemption:";

const INSTALL = "src-tauri/src/extensions/install.rs";
const RUN = "src-tauri/src/extensions/run.rs";
const COMMANDS = "src-tauri/src/commands/extensions.rs";

/** The 1-based line numbers carrying `needle`, in file order. */
const sites = (source: string, needle: string) =>
  source
    .split("\n")
    .flatMap((line, index) => (line.includes(needle) ? [index + 1] : []));

// Measured after the R117 landing (19 − 9 A = 10). Every survivor is B-class:
// the test rewrites or deletes the staged inode (in-place upgrade / drift /
// removal), which a single committed fixture cannot express.
const INSTALL_SITES = [3342, 3380, 3960, 4005, 4054, 4087, 4178, 4225, 4360, 4505];
// All nine run.rs sites were A-class, so none may stage at run time again.
const RUN_SITES: number[] = [];
// 13 − 1 A = 12 survivors: 5 B + 4 C + 3 D. The C/D sites carry the exemption
// comment asserted below; the B sites are pinned by position only.
const COMMANDS_SITES = [
  2789, 2882, 2957, 3047, 3115, 3163, 3282, 3598, 3646, 3735, 3770, 3837,
];
// The A-class targets: five new fixtures (same bytes, basename-load-bearing)
// plus the external-tool fixture that replaced the last run-time `fs::write`.
const SH = "." + "sh";
const NEW_FIXTURES = ["findable", "mytool", "plain" + SH, "order" + SH, "term" + SH, "external-tool" + SH];

test("install.rs stages only its registered B-class sites", async () => {
  const source = await read(INSTALL);
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.deepEqual(sites(source, STAGE_FIXTURE), INSTALL_SITES);
});

test("run.rs stages nothing: every A-class site execs a committed fixture", async () => {
  const source = await read(RUN);
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.deepEqual(sites(source, STAGE_FIXTURE), RUN_SITES);
  // The migrated paths really point at the repo fixtures, not at a copy.
  for (const name of [
    "run-capture" + SH,
    "run-args" + SH,
    "order" + SH,
    "term" + SH,
    "run-argv" + SH,
    "run-env" + SH,
    "run-slow" + SH,
    "run-kill" + SH,
  ]) {
    assert.ok(
      source.includes("tests/fixtures/" + name),
      `run.rs must exec tests/fixtures/${name} in place`,
    );
  }
});

test("commands/extensions.rs stages only its registered B/C/D sites", async () => {
  const source = await read(COMMANDS);
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.deepEqual(sites(source, STAGE_FIXTURE), COMMANDS_SITES);
});

test("the C/D exemptions are registered, one per non-B survivor", async () => {
  const source = await read(COMMANDS);
  const exemptions = sites(source, EXEMPTION);
  assert.equal(exemptions.length, 7, "4 C + 3 D sites must carry the token");
  // Each exemption sits on the line directly above a pinned staging call.
  for (const line of exemptions) {
    assert.ok(
      COMMANDS_SITES.includes(line + 1),
      `exemption at line ${line} must guard the staging call at line ${line + 1}`,
    );
  }
});

test("the new fixtures are committed and executable", async () => {
  for (const name of NEW_FIXTURES) {
    const info = await stat(new URL("src-tauri/tests/fixtures/" + name, root));
    assert.ok(info.isFile(), `${name} must be a committed fixture`);
    assert.notEqual(info.mode & 0o111, 0, `${name} must be executable`);
  }
});

test("one staging body survives, in extensions/mod.rs", async () => {
  const canonical = await read("src-tauri/src/extensions/mod.rs");
  assert.equal(count(canonical, STAGE_FIXTURE_DEF), 1, "the shared helper must exist once");
  const commands = await read(COMMANDS);
  assert.equal(count(commands, STAGE_FIXTURE_DEF), 0, "the local duplicate must be gone");
});

test("capability_probe and probe_runner keep zero run-time staging (R115 stays green)", async () => {
  for (const path of [
    "src-tauri/src/extensions/capability_probe.rs",
    "src-tauri/src/extensions/probe_runner.rs",
  ]) {
    const source = await read(path);
    assert.ok(source.includes("mod tests"), `${path} must be a live file`);
    assert.equal(count(source, STAGE_FIXTURE), 0, `${path} may not stage a fixture at run time`);
  }
});

test("this guard assembles its banned tokens, it does not spell them", async () => {
  const self = await read("tests/r117-fixture-hygiene.test.ts");
  for (const token of [STAGE_FIXTURE, STAGE_FIXTURE_DEF, EXEMPTION]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token}`);
  }
});
