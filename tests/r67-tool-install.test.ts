// R67 · the frontend half of the install hand-off: the command string and the
// terminal session that types it.
//
// Two pure pieces, each with the mutation that turns it back into the wrong
// behaviour:
//
//   1. `installCommand` picks the first recipe whose manager the machine
//      actually has. A regression that always took `recipes[0]` would hand a
//      Debian user `pacman`, and one that always took the last would hand an
//      Arch user `apt` — both are pinned below.
//   2. `openInstallSession` opens a **bare** session and types afterwards, so
//      R62's `isCommandStartedSession` stays false. A regression that passed
//      the command as `initialCommand` would make the page auto-close when the
//      install finished; the fake records the exact argument.
//
// The deps are injected, so this suite needs no DOM, no Tauri host and no
// network.

import assert from "node:assert/strict";
import test from "node:test";

import {
  installCommand,
  installSessionInput,
  MANAGER_INSTALL_COMMANDS,
  openInstallSession,
  RECIPE_PLATFORMS,
  TOOL_CATALOG_IDS,
  type InstallPlatform,
  type PackageManagerId,
  type ToolCatalogEntry,
  type ToolRecipe,
} from "../src/extensions/tool-install.ts";

const entryWith = (
  recipes: Partial<Record<InstallPlatform, ToolRecipe[]>>,
): Pick<ToolCatalogEntry, "recipes"> => ({
  recipes: {
    macos: recipes.macos ?? [],
    linux: recipes.linux ?? [],
    windows: recipes.windows ?? [],
  },
});

// ── 1 · the command generation truth table ────────────────────────────────

test("every manager has exactly one command template", () => {
  // Mutation: add a manager id to the Rust table without a template here, and
  // the guard test fails; delete a template and this does.
  assert.deepEqual(Object.keys(MANAGER_INSTALL_COMMANDS).sort(), [
    "apt",
    "brew",
    "cargo",
    "dnf",
    "npm",
    "pacman",
    "pipx",
    "winget",
  ]);
  const expected: Record<PackageManagerId, string> = {
    brew: "brew install jq",
    winget: "winget install --id jqlang.jq -e",
    pacman: "sudo pacman -S jq",
    apt: "sudo apt install jq",
    dnf: "sudo dnf install jq",
    npm: "npm install -g tldr",
    cargo: "cargo install ripgrep",
    pipx: "pipx install httpie",
  };
  assert.equal(MANAGER_INSTALL_COMMANDS.brew("jq"), expected.brew);
  assert.equal(MANAGER_INSTALL_COMMANDS.winget("jqlang.jq"), expected.winget);
  assert.equal(MANAGER_INSTALL_COMMANDS.pacman("jq"), expected.pacman);
  assert.equal(MANAGER_INSTALL_COMMANDS.apt("jq"), expected.apt);
  assert.equal(MANAGER_INSTALL_COMMANDS.dnf("jq"), expected.dnf);
  assert.equal(MANAGER_INSTALL_COMMANDS.npm("tldr"), expected.npm);
  assert.equal(MANAGER_INSTALL_COMMANDS.cargo("ripgrep"), expected.cargo);
  assert.equal(MANAGER_INSTALL_COMMANDS.pipx("httpie"), expected.pipx);
});

test("the detected manager wins over the table's order", () => {
  const entry = entryWith({
    linux: [
      { manager: "pacman", package: "jq" },
      { manager: "apt", package: "jq" },
      { manager: "dnf", package: "jq" },
    ],
  });
  assert.equal(installCommand(entry, ["pacman"], "linux"), "sudo pacman -S jq");
  assert.equal(installCommand(entry, ["apt"], "linux"), "sudo apt install jq");
  assert.equal(installCommand(entry, ["dnf"], "linux"), "sudo dnf install jq");
  // Several present: the earliest recipe in the table still wins.
  assert.equal(
    installCommand(entry, ["dnf", "apt"], "linux"),
    "sudo apt install jq",
  );
  // A manager that is present but not a recipe for this tool is ignored.
  assert.equal(
    installCommand(entry, ["brew"], "linux"),
    "sudo pacman -S jq",
  );
});

test("with nothing detected the preferred recipe is still returned", () => {
  // Mutation: return `null` here and a minimal host sees no command at all.
  const entry = entryWith({
    macos: [{ manager: "brew", package: "flameshot" }],
    windows: [{ manager: "winget", package: "Flameshot.Flameshot" }],
  });
  assert.equal(installCommand(entry, [], "macos"), "brew install flameshot");
  assert.equal(
    installCommand(entry, [], "windows"),
    "winget install --id Flameshot.Flameshot -e",
  );
});

test("a platform with no recipe yields no command", () => {
  const entry = entryWith({ macos: [{ manager: "brew", package: "jq" }] });
  assert.equal(installCommand(entry, ["brew"], "linux"), null);
  assert.equal(installCommand(entry, ["brew"], "windows"), null);
});

test("a recipe naming a manager the frontend does not know yields no command", () => {
  // The Rust table is the source of truth for manager ids; if it grows ahead of
  // this module, a half-built command would be worse than none.
  const entry = entryWith({
    linux: [{ manager: "snap" as PackageManagerId, package: "jq" }],
  });
  assert.equal(installCommand(entry, ["snap"], "linux"), null);
});

test("the fallback manager (cargo/pipx/npm) is used when the system one is absent", () => {
  // fd's real Linux table: pacman/apt/dnf then cargo. A cargo-only box must
  // still get a command.
  const fd = entryWith({
    linux: [
      { manager: "pacman", package: "fd" },
      { manager: "apt", package: "fd-find" },
      { manager: "dnf", package: "fd-find" },
      { manager: "cargo", package: "fd-find" },
    ],
  });
  assert.equal(installCommand(fd, ["cargo"], "linux"), "cargo install fd-find");
  assert.equal(
    installCommand(fd, ["apt", "cargo"], "linux"),
    "sudo apt install fd-find",
  );
});

test("the platform key set is the frontend's own vocabulary", () => {
  assert.deepEqual([...RECIPE_PLATFORMS], ["macos", "linux", "windows"]);
});

// ── 2 · the terminal hand-off ─────────────────────────────────────────────

test("the session input is the command plus the submitting break", () => {
  assert.equal(installSessionInput("brew install jq"), "brew install jq\r");
  assert.equal(installSessionInput("  brew install jq  "), "brew install jq\r");
  assert.equal(installSessionInput(""), null);
  assert.equal(installSessionInput("   "), null);
});

test("openInstallSession spawns bare, then types the command", async () => {
  const calls: string[] = [];
  const typed: { command: string; args: { id: string; data: number[] } }[] = [];
  const opened = await openInstallSession("brew install jq", {
    ensureTerminalSession: async (initialCommand) => {
      // Mutation: pass the command here instead of null and R62's predicate
      // becomes true — the page would auto-close when the install ends.
      calls.push(`spawn:${String(initialCommand)}`);
    },
    invoke: async (command, args) => {
      calls.push(`invoke:${command}`);
      typed.push({ command, args });
    },
  });
  assert.equal(opened, true);
  assert.deepEqual(calls, ["spawn:null", "invoke:term_input"]);
  assert.equal(typed.length, 1);
  assert.equal(typed[0].command, "term_input");
  assert.equal(typed[0].args.id, "main", "the main surface is the target");
  assert.equal(
    new TextDecoder().decode(new Uint8Array(typed[0].args.data)),
    "brew install jq\r",
    "the PTY receives the command and the break, byte for byte",
  );
});

test("openInstallSession does nothing for a blank command", async () => {
  let spawns = 0;
  let invokes = 0;
  const opened = await openInstallSession("   ", {
    ensureTerminalSession: async () => {
      spawns += 1;
    },
    invoke: async () => {
      invokes += 1;
    },
  });
  assert.equal(opened, false);
  assert.equal(spawns, 0, "an empty install must not open a session");
  assert.equal(invokes, 0);
});

test("the id mirror is the catalog's own id list", () => {
  // The guard test compares this to the Rust table; here it is pinned as a set
  // of unique, non-empty ids so a typo cannot slip through as a "new tool".
  assert.equal(new Set(TOOL_CATALOG_IDS).size, TOOL_CATALOG_IDS.length);
  for (const id of TOOL_CATALOG_IDS) {
    assert.match(id, /^[a-z0-9][a-z0-9-]*$/, `${id} is a stable slug`);
  }
});
