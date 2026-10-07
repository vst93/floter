// R118 · `stage_fixture` census, part two — the B-class migration.
//
// R116 classified the remaining staging call sites into A (a fixed fixture is
// exec'd and never rewritten → exec the committed fixture in place), B (the
// test needs a per-test writable path: in-place upgrade, delete, or two
// same-basename copies), C (rewritten but never exec'd) and D (never exec'd).
// R117 migrated the 19 A sites and pinned the survivors. R118 migrates the 15
// B sites: `build.rs` now pre-stages every committed fixture into
// `OUT_DIR/fixtures/`, and a B-class test points a **symlink** at one of those
// presets and "upgrades" by repointing the link (an atomic rename of a fresh
// link). No run-time write ever touches an inode a child may be about to exec,
// which is the ETXTBSY window R114 measured.
//
// This guard pins both censuses by exact line list: any new run-time staging
// call, any moved one, or any B-class site that slips back to a run-time write
// must be re-registered here first. C and D stay out of scope — they stage a
// writable copy the list path never execs — and are pinned by position only.
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
const LINK_FIXTURE = "link_" + "fixture(";
const LINK_FIXTURE_DEF = "fn " + LINK_FIXTURE;
const EXEMPTION = "R117 " + "exemption:";
const OUT_DIR = "OUT_" + "DIR";
const WRITE_THROUGH = "fs::write(&fixture." + "executable";

const INSTALL = "src-tauri/src/extensions/install.rs";
const RUN = "src-tauri/src/extensions/run.rs";
const COMMANDS = "src-tauri/src/commands/extensions.rs";
const BUILD = "src-tauri/build.rs";
const MOD = "src-tauri/src/extensions/mod.rs";

/** The 1-based line numbers carrying `needle`, in file order. */
const sites = (source: string, needle: string) =>
  source
    .split("\n")
    .flatMap((line, index) => (line.includes(needle) ? [index + 1] : []));

// Measured after the R118 landing. R117 pinned 10 B-class survivors here; every
// one now links a build-time preset instead, so the run-time staging list is
// empty. A revival of any of them — or a new one — is a red guard.
const INSTALL_SITES: number[] = [];
// All nine run.rs sites were A-class, so none may stage at run time again.
const RUN_SITES: number[] = [];
// 12 − 5 B = 7 survivors: 4 C + 3 D. The C/D sites carry the exemption comment
// asserted below; no B site stages here any more. (R121 deleted four emits above
// these lines, so every pin here moved up by 7 from the R118 numbering; R126
// grew the connect-package picker above them by 24 lines, moving every pin here
// down by 24 from the R121 numbering. R135 moved the three export bodies'
// blocking IO into `spawn_blocking` and added their explanatory comments (then
// rustfmt wrapped the export's result tuple onto four lines), so every pin here
// moved down by 32 from the R126 numbering.)
const COMMANDS_SITES = [2847, 2940, 3015, 3105, 3173, 3343, 3902];
// The B-class upgrade path: one `link_fixture` call per former B site in
// install.rs, and per former B site plus the `:3536` rebuild rewrite in
// commands/extensions.rs (which may no longer write through the link).
// R128 grew the `cannotDeriveCommand` refusal from one line to a four-line
// `format!` above every one of these, moving each pin down by 2. R135's +32
// (see above) applies here too.
const INSTALL_LINK_SITES = [3353, 3394, 3974, 4022, 4074, 4110, 4204, 4252, 4390, 4535];
const COMMANDS_LINK_SITES = [3221, 3574, 3663, 3711, 3800, 3835];
// The A-class targets: five new fixtures (same bytes, basename-load-bearing)
// plus the external-tool fixture that replaced the last run-time `fs::write`.
const SH = "." + "sh";
const NEW_FIXTURES = ["findable", "mytool", "plain" + SH, "order" + SH, "term" + SH, "external-tool" + SH];

test("install.rs stages nothing at run time: every B-class site links a preset", async () => {
  const source = await read(INSTALL);
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.deepEqual(sites(source, STAGE_FIXTURE), INSTALL_SITES);
  assert.deepEqual(sites(source, LINK_FIXTURE), INSTALL_LINK_SITES);
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

test("commands/extensions.rs stages only its registered C/D sites", async () => {
  const source = await read(COMMANDS);
  assert.ok(source.includes("mod tests"), "the guard must scan a live file");
  assert.deepEqual(sites(source, STAGE_FIXTURE), COMMANDS_SITES);
  assert.deepEqual(sites(source, LINK_FIXTURE), COMMANDS_LINK_SITES);
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

test("build.rs pre-stages the committed fixtures into OUT_DIR", async () => {
  const source = await read(BUILD);
  assert.ok(source.includes(OUT_DIR), "build.rs must resolve the crate's OUT_DIR");
  assert.ok(source.includes("fixtures"), "the presets must live under a fixtures/ directory");
  assert.ok(source.includes("rerun-if-changed"), "the presets must be re-staged when a fixture moves");
  assert.ok(
    source.includes("preset_test_fixtures"),
    "the preset logic must be one named step, not inlined into main",
  );
});

test("the two migrated files resolve their presets through OUT_DIR", async () => {
  // The executable wiring lives in `test_support::link_fixture`; each file
  // declares the preset root it links into, so a revert to a run-time copy has
  // to delete a live OUT_DIR reference, not just move a call.
  for (const path of [INSTALL, COMMANDS]) {
    const source = await read(path);
    assert.ok(source.includes(OUT_DIR), `${path} must resolve its presets from OUT_DIR`);
  }
});

test("one link body survives, in extensions/mod.rs", async () => {
  const canonical = await read(MOD);
  assert.equal(count(canonical, LINK_FIXTURE_DEF), 1, "the shared link helper must exist once");
  for (const path of [INSTALL, COMMANDS]) {
    const source = await read(path);
    assert.equal(count(source, LINK_FIXTURE_DEF), 0, `${path} must not re-define the helper`);
  }
});

test("no B-class destination is the target of an fs::write", async () => {
  // A symlinked destination points at a build-time preset shared by every test
  // in the run: writing through it would corrupt that preset for everyone. The
  // pre-R118 rebuild form (a direct `fs::write` onto the fixture link) is banned
  // outright, so the write-through mutation is a red guard even though the
  // write itself is otherwise invisible.
  for (const path of [INSTALL, COMMANDS]) {
    const source = await read(path);
    assert.equal(count(source, WRITE_THROUGH), 0, `${path} must not write through a fixture link`);
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
  const canonical = await read(MOD);
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
  for (const token of [STAGE_FIXTURE, STAGE_FIXTURE_DEF, LINK_FIXTURE, LINK_FIXTURE_DEF, EXEMPTION, WRITE_THROUGH]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token}`);
  }
});
