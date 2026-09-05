import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);

// The management panel was collapsed to Connected / Detected-on-this-device.
// NPM discovery, permission-review installs, updates/rollback/reinstall and
// channel/pin controls were removed; their backend commands stay registered as
// frozen legacy. These checks keep the frontend from drifting back onto them.
const panelSources = [
  "src/ExtensionsPanel.tsx",
  "src/extensions/ExtensionRow.tsx",
  "src/extensions/LocalInstallDialog.tsx",
  "src/extensions/RemovalConfirmation.tsx",
  "src/extensions/CustomIntegrationDrawer.tsx",
];

const forbiddenCommands = [
  "extensions_search\"",
  "extensions_check_updates",
  "extensions_update",
  "extensions_rollback",
  "extensions_reinstall",
  "extensions_set_pinned",
   "extensions_set_channel",
];

test("panel frontend no longer invokes frozen NPM maintenance commands", async () => {
  for (const file of panelSources) {
    const source = await readFile(new URL(file, root), "utf8");
    for (const command of forbiddenCommands) {
      if (source.includes(command.trim())) {
        throw new Error(`${file} references frozen command ${command.trim()}`);
      }
    }
  }
});

test("panel keeps local connection paths working", async () => {
  const source = await readFile(new URL("src/ExtensionsPanel.tsx", root), "utf8");
  for (const command of [
    "extensions_list",
    "extensions_enable",
    "extensions_disable",
    "extensions_uninstall",
    // extensions_connect_tool was retired from the panel: PATH discoveries are
    // connected through the create-custom-integration drawer
    // (extensions_create_custom), recommendations through
    // extensions_connect_recommended.
    "extensions_connect_recommended",
    "extensions_create_custom",
    "extensions_search_tools",
    "extensions_export",
    "extensions_import",
    "extensions_describe",
    "extensions_diagnose",
    "extensions_health",
    "extensions_config_get",
  ]) {
    if (!source.includes(`"${command}"`)) {
      throw new Error(`ExtensionsPanel.tsx lost the ${command} wiring`);
    }
  }
});
