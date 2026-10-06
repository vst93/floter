// R126 · two small closures, one guard.
//
// Task 1 — the `connect extension package` OS picker in `commands/extensions.rs`
// shipped a hardcoded English title, prompt and both buttons while the
// permission-review dialog in the same file (`extensions_import`) had been
// bilingual for rounds. This suite pins the picker's `is_zh` quartet and keeps
// the precedent it copies alive, so a future edit cannot quietly drop the
// branch back to English.
//
// Task 2 — the plugin configuration overlay read its stored block through
// `loadPluginValues`; a failed read was dropped and the
// overlay painted `configDefaults`, so the next edit wrote defaults over the
// real block (R113 only treated the write path). This suite pins the read
// failure state, the swallow's absence, the persist gate and the failure line.
//
// Every load-bearing literal is assembled from fragments, and the last test
// proves this guard does not spell the strings it scans for.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const EXTENSIONS = "src-tauri/src/commands/" + "extensions.rs";
const OVERLAY = "src/plugins/" + "PluginConfigOverlay.tsx";
const I18N = "src/" + "i18n.ts";
// Assembled: this guard must not spell the swallow it forbids.
const SWALLOW = ".catch(() => " + "undefined)";

// Assembled so this guard does not spell the strings it scans for.
const PICK = "pub async fn extensions_pick_" + "local_package(";
const IMPORT = "pub async fn extensions_import(";
const INSTALL = "pub async fn extensions_install(";
const LOCAL_REVIEW = "pub fn extensions_local_manifest_review(";
const PICK_ZH_TITLE = "连接" + "扩展包";
const PICK_ZH_MESSAGE = "选择一个包文件夹" + "（确定）或一个 floter.extension.json 文件（取消）。";
const PICK_ZH_APPROVE = "选择" + "文件夹";
const PICK_ZH_CANCEL = "选择" + "文件";
const PICK_EN_TITLE = "Connect extension " + "package";
const REVIEW_ZH_TITLE = "确认" + "插件权限";
const REVIEW_EN_TITLE = "Review extension " + "permissions";
const LOAD_KEY = "settings." + "pluginConfigLoadFailed";
const SAVE_KEY = "settings." + "saveFailed";
const LOAD_STATE = "const [loadFailed, " + "setLoadFailed] = useState(false)";
const LOAD_SET = "setLoadFailed(" + "true)";
const LOAD_CLEAR = "setLoadFailed(" + "false)";
const PERSIST_GATE = "if (loadFailed) " + "return false;";
const PERSIST_FAILED_SET = "setPersistFailed(" + "true)";

/** The slice of a Rust file between two signatures (end marker exclusive). */
const between = (source: string, start: string, end: string): string => {
  const from = source.indexOf(start);
  assert.ok(from >= 0, `start signature not found: ${start}`);
  const to = source.indexOf(end, from + start.length);
  assert.ok(to > from, `end signature not found after start: ${end}`);
  return source.slice(from, to);
};

/** The overlay body from `needle` to the next `end` marker. */
const overlaySlice = (overlay: string, start: string, end: string): string => {
  const from = overlay.indexOf(start);
  assert.ok(from >= 0, `overlay marker not found: ${start}`);
  const to = overlay.indexOf(end, from + start.length);
  assert.ok(to > from, `overlay end marker not found: ${end}`);
  return overlay.slice(from, to);
};

test("the connect-package picker carries the bilingual quartet", async () => {
  const source = await read(EXTENSIONS);
  const picker = between(source, PICK, LOCAL_REVIEW);

  assert.ok(picker.includes("is_zh"), "the picker must branch on is_zh");
  assert.ok(picker.includes(PICK_ZH_TITLE), "the zh title must be present");
  assert.ok(picker.includes(PICK_ZH_MESSAGE), "the zh prompt must be present");
  assert.ok(picker.includes(PICK_ZH_APPROVE), "the zh approve button must be present");
  assert.ok(picker.includes(PICK_ZH_CANCEL), "the zh cancel button must be present");
  // The English strings are not deleted, they become the other branch.
  assert.ok(picker.includes(PICK_EN_TITLE), "the English title must remain");
  // The dialog still reads its strings from the quartet rather than literals
  // passed straight to `.message`/`.title`.
  assert.ok(
    picker.includes(".message(message)") && picker.includes(".title(title)"),
    "the picker must feed the quartet into the dialog",
  );
});

test("the same-file permission dialog precedent is still there", async () => {
  const source = await read(EXTENSIONS);
  const review = between(source, IMPORT, INSTALL);

  assert.ok(review.includes("is_zh"), "the import review must still branch on is_zh");
  assert.ok(review.includes(REVIEW_ZH_TITLE), "the zh review title must remain");
  assert.ok(review.includes(REVIEW_EN_TITLE), "the English review title must remain");
});

test("the overlay distinguishes a failed read and no longer swallows it", async () => {
  const overlay = await read(OVERLAY);

  assert.ok(overlay.includes(LOAD_STATE), "the overlay must hold a loadFailed state");
  // The read effect reports the failure instead of dropping it on the floor.
  const effect = overlaySlice(
    overlay,
    "loadPluginValues(pluginId, clipboardEnabled)",
    "}, [pluginId, schema, clipboardEnabled]);",
  );
  assert.ok(effect.includes(LOAD_SET), "a failed read must set loadFailed");
  assert.ok(effect.includes(LOAD_CLEAR), "a read that answers must clear loadFailed");
  // No bare swallow survives anywhere in the overlay: the R113 read fallback
  // is gone, and R113's write path never had one.
  assert.equal(count(overlay, SWALLOW), 0, "the overlay must not swallow a read failure");
});

test("the persist path is gated on the failed read", async () => {
  const overlay = await read(OVERLAY);
  const persist = overlaySlice(overlay, "const persist = useCallback", "const handleChange = useCallback");

  assert.ok(persist.includes(PERSIST_GATE), "persist must refuse while the read failed");
  // R113's write-failure line is retained (asserted, not changed).
  assert.ok(overlay.includes(SAVE_KEY), "the R113 write-failure caption must remain");
  assert.ok(overlay.includes(PERSIST_FAILED_SET), "the R113 rollback report must remain");
  assert.ok(
    overlay.includes(`loadFailed ? "${LOAD_KEY}" : "${SAVE_KEY}"`),
    "the failure line must prefer the read failure while persistence is paused",
  );
});

test("the new load-failure key exists on both sides of the dictionary", async () => {
  const i18n = await read(I18N);
  assert.equal(
    count(i18n, `"${LOAD_KEY}":`),
    2,
    "the load-failure key must have an English and a Chinese entry",
  );
  // Both values are non-empty and distinct, so the key is not a placeholder.
  const values = [...i18n.matchAll(new RegExp(`"${LOAD_KEY}": "([^"]*)"`, "g"))].map(
    (match) => match[1],
  );
  assert.equal(values.length, 2, "both dictionary entries must carry a value");
  assert.ok(values[0].length > 0 && values[1].length > 0, "neither entry may be blank");
  assert.notEqual(values[0], values[1], "the two languages must not share one string");
});

test("this guard assembles its literals, it does not spell them", async () => {
  const self = await read("tests/r126-dialog-and-config.test.ts");
  for (const literal of [SWALLOW, PICK_ZH_TITLE, PICK_ZH_MESSAGE, LOAD_KEY, LOAD_STATE]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});
