// R7-9 · the `floter://` scheme: one allow-list, three actions, and a hard rule
// that a link never installs anything.
//
// The backend is the authority (`src-tauri/src/deep_link.rs` owns the action
// table, the manifest validation and the routing), and its unit tests drive
// the refusal matrix directly. What this file locks is the *contract across
// the boundary*, which is where the interesting failure modes live:
//
//   A. the frontend has no second allow-list and no second validator — the
//      round's mutation "CLI/scheme 处理分叉" turns this red;
//   B. the app has one path from a link to the review dialog, and that path
//      ends at the dialog (the "deep link installs directly" mutation);
//   C. a refusal is one deduped toast keyed by a real dictionary entry, and an
//      unknown action is silent;
//   D. the scheme is registered in the config the bundler reads, and the About
//      page shows the line the router actually accepts.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator, isMessageKey } from "../src/i18n.ts";
import {
  DEEP_LINK_CONNECT_EVENT,
  DEEP_LINK_EXAMPLE,
  DEEP_LINK_REGISTER_EXAMPLE,
  DEEP_LINK_REJECT_DEDUP_MS,
  DEEP_LINK_REJECT_EVENT,
  DEEP_LINK_REJECT_KEY,
  deepLinkRejectGate,
} from "../src/deep-link.ts";
import { FAILURE_NOTIFY_DEDUP_MS, createFailureDeduper } from "../src/plugin-pages.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// ── A · one allow-list, one validator ─────────────────────────────────────

// The mutation this round exists to prevent: the CLI and the scheme growing a
// second list of action names that can drift from the router's. The frontend
// therefore must not contain the strings at all — it receives an outcome, not
// a decision.
test("the frontend declares no second action allow-list", async () => {
  const deepLink = stripJsComments(await read("src/deep-link.ts"));
  for (const name of ["open", "connect", "register"]) {
    assert.ok(
      !new RegExp(`["'\`]${name}["'\`]`).test(deepLink),
      `src/deep-link.ts must not enumerate the action "${name}" — the backend owns the table`,
    );
  }
  assert.ok(
    !/ACTIONS|ALLOWED_ACTIONS|allowList|allowlist/i.test(deepLink),
    "there must be no frontend action table",
  );
});

// The backend half of the same lock: the table exists once, and both the URL
// router and the CLI normalizer read it rather than restating it. Two literal
// occurrences of the pair would be the drift.
test("the backend keeps exactly one action table and both entry points read it", async () => {
  // The production half only: the test module below legitimately spells the
  // action names out to pin the table's contents.
  const source = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  const production = source.slice(0, source.indexOf("#[cfg(test)]"));
  assert.match(
    production,
    /pub const ACTIONS: &\[&str\] = &\["open", "connect", "register"\];/,
    "the action table is declared once, verbatim",
  );
  assert.equal(
    production.split('"connect"').length - 1,
    2,
    "outside the tests, `connect` appears only in the table and the router's parameter check",
  );
  assert.equal(
    production.split('"open"').length - 1,
    2,
    "`open` appears only in the table and the router's own match arm — no second list",
  );
  // R8-3 adds `register`, R8-4 adds its fourth parameter (`confirm`, the
  // terminal spelling's `--yes`). It is named in the table once, in the
  // router's three parameter guards, in the router's own match arm, in the CLI
  // normalizer that folds trailing words into `args`, and in `wants_register`
  // (the transport question, not a second parser) — seven places, all of which
  // read the *same* action name and none of which is a second table.
  assert.equal(
    production.split('"register"').length - 1,
    7,
    "`register` is one entry in the table plus the router/normalizer call sites, never a second list",
  );
  // Both entry points consult the shared predicate instead of a literal.
  assert.match(production, /pub fn is_action\(name: &str\) -> bool \{\n\s*ACTIONS\.contains/, "is_action reads the table");
  assert.match(production, /if !is_action\(action\) \{\n\s*return Err\(Reject::UnknownAction/, "the URL router reads the table");
  assert.match(production, /if !is_action\(action\) \{\n\s*return None;/, "the CLI normalizer reads the same table");
  // And the CLI has no validator of its own: it builds a URL and hands it to
  // the router, so there is exactly one place that decides what may be carried.
  assert.ok(
    !/fn validate_manifest|ManifestTraversal|ManifestNotJson/.test(
      stripJsComments(await read("src-tauri/src/main.rs")),
    ),
    "the CLI entry point must not validate the manifest itself",
  );
  assert.match(
    stripJsComments(await read("src-tauri/src/main.rs")),
    /deep_link::canonical_argument\(&arguments\)/,
    "the CLI normalizes into the router's URL form",
  );
});

// m-1 (microfix round): the parking fork. A cold start runs before the webview
// has listeners, so its request is parked for `take_pending_deep_link`; a live
// delivery already has listeners and must NOT park, or the slot stays full and
// a webview reload replays a dismissed dialog. The Rust unit tests drive the
// fork directly; this pins that both call sites declare which one they are.
test("the router distinguishes a cold start from a live delivery", async () => {
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  assert.match(rust, /pub enum Delivery \{\s*ColdStart,\s*Live,/, "the delivery kind is explicit");
  assert.match(
    rust,
    /if delivery == Delivery::Live \{\s*return;\s*\}/,
    "a live delivery is not parked",
  );
  assert.match(
    rust,
    /pub fn dispatch_url\(app: &AppHandle, raw: &str, delivery: Delivery\)/,
    "the one entry point takes the delivery kind",
  );
  // The two call sites name their kind; neither can be silently defaulted.
  const lib = stripJsComments(await read("src-tauri/src/lib.rs"));
  assert.match(lib, /dispatch_url\(app\.handle\(\), &url, deep_link::Delivery::ColdStart\)/, "setup is the cold start");
  assert.match(lib, /dispatch_url\(&handle, &url, deep_link::Delivery::Live\)/, "a forwarded instance is live");
});

// m-2 (microfix round): the redirect target is a second URL and gets the same
// `.json` suffix rule as the input URL. The Rust unit tests drive the rule;
// this pins that `stage_remote` actually calls it.
test("a followed redirect is re-checked against the same suffix rule", async () => {
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  assert.match(
    rust,
    /fn validate_followed_manifest_url\(url: &Url\) -> Result<\(\), String>/,
    "the redirect check is a named function",
  );
  assert.match(rust, /validate_followed_manifest_url\(response\.url\(\)\)\?/, "the response URL is re-checked");
  assert.match(
    rust,
    /fn is_json_manifest_path\(url: &Url\) -> bool \{\s*url\.path\(\)\.ends_with\("\.json"\)/,
    "one suffix rule, shared by the input URL and the redirect target",
  );
});

// ── B · a deep link never installs ────────────────────────────────────────

// The backend stops at an event. There is no `extensions_install` call, no
// lock write and no approval on the deep-link path; the dialog's Connect
// button is the only way forward, and it runs the ordinary pipeline.
test("the backend deep-link path never installs, approves or enables", async () => {
  const source = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  // The production half: the test module below legitimately names the lock
  // field it asserts on.
  const production = source.slice(0, source.indexOf("#[cfg(test)]"));
  for (const forbidden of [
    "extensions_install",
    "install::install",
    "validate_permission_approval",
    "approved_permissions",
    "approvedPermissions",
    "lock.save",
    "ExtensionsLock",
  ]) {
    assert.ok(
      !production.includes(forbidden),
      `deep_link.rs must not reach ${forbidden} — a link may only open the review dialog`,
    );
  }
  // The structural check is a parse, not an install.
  assert.match(production, /ExtensionManifest::load\(&path\)\?/, "the manifest is parsed");
  assert.match(production, /app\.emit\(\s*CONNECT_EVENT/, "and the request is emitted");
});
// The frontend's only consumer of a link request turns it into the *same*
// review dialog the file picker opens — one function, two callers — and the
// install still happens only from that dialog's confirm.
test("a validated link reaches the same review dialog as the file picker", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(
    panel,
    /const reviewLocalManifest = async \(manifestPath: string\)/,
    "one review function serves both entry points",
  );
  assert.equal(
    panel.split("reviewLocalManifest(").length - 1,
    2,
    "the function is called from exactly the picker and the link",
  );
  assert.match(
    panel,
    /await reviewLocalManifest\(manifestPath\)/,
    "the file picker routes through it",
  );
  assert.match(
    panel,
    /reviewLocalManifest\(pendingDeepLink\.manifestPath\)/,
    "the deep link routes through the same function",
  );
  // The dialog is still the only installer.
  assert.match(
    panel,
    /invoke\("extensions_install", \{ request: \{ \.\.\.pending\.request, approvedPermissions/,
    "only the dialog's confirm installs, and it sends the reviewed permission set",
  );
  // A deep link must not open the dialog with a pre-approved set.
  const link = panel.slice(panel.indexOf("if (!pendingDeepLink || busyRef.current) return;"));
  const block = link.slice(0, link.indexOf("}, [pendingDeepLink])"));
  assert.ok(!/approvedPermissions/.test(block), "the link never supplies approvals");
});

// The app hands the request down as data and consumes it once. Re-opening it
// on a later render would resurrect a dialog the user closed.
test("the app consumes the link request exactly once", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /const \[pendingDeepLink, setPendingDeepLink\] = useState/, "the request is app state");
  assert.match(app, /listen<DeepLinkConnectRequest>\(\s*DEEP_LINK_CONNECT_EVENT/, "the live event opens it");
  assert.match(
    app,
    /invoke<DeepLinkConnectRequest \| null>\("take_pending_deep_link"\)/,
    "a cold start consumes the stored request",
  );
  assert.match(app, /onDeepLinkConsumed=\{\(\) => setPendingDeepLink\(null\)\}/, "the hand-off clears the slot");
  assert.match(app, /openSettings\("integrations"\)/, "the review dialog lives on the integrations page");
});

// ── B2 · register opens the review surface, and only a terminal `--yes` binds ─

// R8-4 changes what "never binds" means, and this is the contract that keeps
// the change honest. A `register` trigger binds **only** when all three hold:
//
//   1. it arrived on the terminal transport (`RegisterOrigin::Terminal`) — a
//      fact only this process' argv or a control-socket line can supply, never
//      a URL's contents;
//   2. the user already said yes (the name is curated, or `--yes` was given);
//   3. the name resolved to an available executable.
//
// And even then the write is `install::connect_tool` — the *existing* connect
// entry point — so the lock, the approval record and the catalog stay on the
// one pipeline every other connection uses. There is still no `ToolLock` write,
// no `bind_locator`, no `lock.save` and no `extensions_install` in this file.
//
// Mutation: drop the `RegisterOrigin::Terminal` conjunct (or the
// `register_may_bind` guard entirely) and the Rust tests
// `the_same_curated_name_over_a_link_still_does_not_bind` and
// `a_register_that_may_not_bind_writes_nothing` go red — as does this one,
// because `register_may_bind` is asserted to be the only gate.
test("the backend register path binds only through the gated connect path", async () => {
  const source = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  const production = source.slice(0, source.indexOf("#[cfg(test)]"));
  // The private lock write is still absent: this module cannot bind anything
  // itself, so the only binding it can cause is the shared connect pipeline.
  for (const forbidden of [
    "ToolLock",
    "bind_locator",
    "lock.save",
    "extensions_install",
    "ExtensionsLock",
    "validate_permission_approval",
    "approvedPermissions",
    "std::process::Command",
  ]) {
    assert.ok(
      !production.includes(forbidden),
      `deep_link.rs must not reach ${forbidden} — a register may only resolve, offer, or run the connect path`,
    );
  }
  // The one sanctioned write goes through the existing entry point.
  assert.match(
    production,
    /crate::extensions::install::connect_tool\(/,
    "the binding is the ordinary connect pipeline, not a private write",
  );
  // And it is unreachable without the gate: the call site sits inside a
  // function that returns early unless `register_may_bind` is true.
  const connect = production.slice(
    production.indexOf("pub(crate) fn connect_registered_command"),
  );
  const gate = connect.indexOf("register_may_bind(request)");
  const call = connect.indexOf("connect_tool(state, candidate)");
  assert.notEqual(gate, -1, "the gate must be consulted");
  assert.notEqual(call, -1, "the connect call must exist");
  assert.ok(gate < call, "the gate is checked before anything can be written");
  assert.match(connect, /if !register_may_bind\(request\) \{\s*return resolved;\s*\}/, "and a failed gate returns the untouched resolve");
  // The gate itself requires the terminal transport — a URL can never pass.
  assert.match(
    production,
    /pub fn register_may_bind\(request: &CommandRequest\) -> bool \{\s*request\.origin == RegisterOrigin::Terminal/,
    "the transport is part of the gate, not of the URL",
  );
  // The terminal transport is set by a *separate* entry point, so the URL
  // router cannot produce it.
  assert.match(
    production,
    /pub fn parse_terminal_url\(raw: &str\) -> Result<Trigger, Reject> \{\s*match parse_url\(raw\)\? \{\s*Trigger::Register\(mut request\) => \{\s*request\.origin = RegisterOrigin::Terminal;/,
    "only the terminal parser upgrades the origin, and it reuses the one router",
  );
  assert.match(
    production,
    /origin: RegisterOrigin::Link,/,
    "the URL router hard-codes the link origin",
  );
  // It *does* resolve through the one discovery + resolver chain.
  assert.match(production, /resolver::resolve_executable_names/, "the existing resolver is reused");
  assert.match(production, /inventory\.candidates\(\)/, "and the existing inventory");
  assert.match(production, /app\.emit\(REGISTER_EVENT, resolved\)/, "the offer is emitted");
  assert.match(
    production,
    /pub const REGISTER_EVENT: &str = "floter:\/\/deep-link-register"/,
    "the event name is declared once",
  );
});

// The cold-start / live fork applies to register too: the Rust test drives the
// shared parking contract, and this pins that the call site passes the kind.
test("register parks on a cold start and not on a live delivery", async () => {
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  assert.match(
    rust,
    /fn park_for_frontend<T>\(/,  
    "one generic parking contract serves both request shapes",
  );
  assert.match(
    rust,
    /pending_deep_link_register,\s*resolved\.clone\(\),\s*delivery,/,
    "the register call site names its delivery kind",
  );
});

// The frontend highlight is a highlight: it selects an existing Detected row
// and changes nothing else. No connect call, no permission review, no install.
//
// Mutation: make the register effect call `connectDetected` (or
// `extensions_connect_tool`) and this fails.
test("a register request highlights a detected row and never connects it", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The request is parked once, then resolved against the list. The split is
  // load-bearing: a cold start delivers the link before the first
  // `extensions_list` reply, so deciding in the same effect would report "not
  // found" for a tool that is right there.
  const parkAt = panel.indexOf("if (!pendingDeepLinkRegister) return;");
  assert.notEqual(parkAt, -1, "the register hand-off must exist");
  const park = panel.slice(parkAt, panel.indexOf("}, [pendingDeepLinkRegister])", parkAt));
  assert.match(park, /setRegisterPending\(pendingDeepLinkRegister\)/, "the request is parked");

  const at = panel.indexOf("if (!registerPending || loading) return;");
  assert.notEqual(at, -1, "the resolve effect must wait for the list to load");
  const block = panel.slice(at, panel.indexOf("}, [registerPending, loading])", at));
  assert.match(block, /suggestedExtensions\.find/, "it must look for the row the backend resolved");
  assert.match(block, /setRegisterTarget\(\{ id: match\.id/, "and mark that row");
  assert.match(block, /setRegisterMiss\(\{ command: request\.command, alreadyConnected \}\)/, "an unresolved name gets an inline reason");
  assert.match(block, /connectedExtensions\.some/, "an already-connected tool gets a different sentence");
  for (const forbidden of ["connectDetected", "extensions_connect_tool", "extensions_install", "approvedPermissions"]) {
    assert.ok(
      !block.includes(forbidden),
      `the register highlight must not reach ${forbidden}`,
    );
  }
  // The mark is presentational: the row keeps its own Connect button.
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /highlighted \? " extension-row--register" : ""/, "the highlight is a class");
  assert.match(row, /extension-row__register-dot/, "and a non-focusable mark, not a control");
  assert.match(row, /onClick=\{extension\.runtimeAvailable \? onConnect : onRepair\}/, "the row's own button is unchanged");
});

// The app consumes a cold-start register request exactly once, like connect.
test("the app consumes the register request exactly once", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /const \[pendingDeepLinkRegister, setPendingDeepLinkRegister\] = useState/, "it is app state");
  assert.match(app, /listen<DeepLinkRegisterRequest>\(\s*DEEP_LINK_REGISTER_EVENT/, "the live event delivers it");
  assert.match(
    app,
    /invoke<DeepLinkRegisterRequest \| null>\("take_pending_deep_link_register"\)/,
    "a cold start consumes the stored request",
  );
  assert.match(
    app,
    /onDeepLinkRegisterConsumed=\{\(\) => setPendingDeepLinkRegister\(null\)\}/,
    "the hand-off clears the slot",
  );
  // The frontend has no second action table here either.
  const deepLink = stripJsComments(await read("src/deep-link.ts"));
  assert.ok(!/ACTIONS|ALLOWED_ACTIONS|allowList|allowlist/i.test(deepLink), "still no frontend table");
});

// ── B3 · the terminal `floter register` (R8-4) ────────────────────────────

// The CLI has no parser of its own: `main.rs` asks `deep_link` to normalize the
// argv into the *same* URL a link is spelled with, and the router validates it.
// The mutation this locks is a second argv parser appearing in `main.rs` — a
// `match` on "register" with its own flag handling.
test("the CLI has no second parser: it normalizes into the router's URL", async () => {
  const main = stripJsComments(await read("src-tauri/src/main.rs"));
  assert.match(
    main,
    /deep_link::register_cli\(&arguments,/,
    "the terminal spelling is one call into deep_link",
  );
  assert.match(
    main,
    /deep_link::canonical_argument\(&arguments\)/,
    "and the generic trigger path still normalizes the same way",
  );
  // No `--yes` handling, no argument indexing, no second action name in the
  // process entry point.
  for (const forbidden of ["--yes", "clap", "clap::", "register\""]) {
    assert.ok(
      !main.includes(forbidden),
      `main.rs must not parse arguments itself (found ${forbidden})`,
    );
  }
  // The decision lives in `deep_link`, and it reuses the router.
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  assert.match(
    rust,
    /let request = match parse_terminal_url\(&url\) \{/,
    "the CLI plan routes through the one parser",
  );
  assert.match(
    rust,
    /let Some\(url\) = canonical_argument\(args\) else \{/,
    "and normalizes argv through the one normalizer",
  );
});

// The disclosure is printed for every bind, and `--yes` does not exempt it:
// the plan carries the sentence, so deleting it is a failing test rather than a
// silent loss.
//
// Mutation: remove the disclosure from `RegisterCliPlan::Bind` (or stop
// printing it) and this goes red, as does the Rust test
// `the_disclosure_names_every_permission_the_binding_gets`.
test("every terminal bind prints its disclosure, and --yes does not exempt it", async () => {
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  const production = rust.slice(0, rust.indexOf("#[cfg(test)]"));
  // The sentence is built from the one permission function, not restated.
  assert.match(
    production,
    /fn permission_disclosure\(\) -> String \{\s*crate::extensions::install::tool_binding_disclosure\(\)\s*\}/,
    "the disclosure comes from the one permission source",
  );
  assert.match(
    production,
    /let disclosure = format!\(\s*"\{\} at \{\} \(\{\}\)",/,
    "the bind plan names the tool, its path and its permissions",
  );
  // It is printed *before* anything is delivered or written, and the bind arm
  // is the only one that prints it.
  const bind = production.slice(production.indexOf("RegisterCliPlan::Bind {"));
  assert.match(
    bind,
    /println!\("floter: connecting \{\} \(\{disclosure\}\)", request\.command\);/,
    "the disclosure is the first line of a bind",
  );
  const disclosureAt = bind.indexOf("println!(\"floter: connecting");
  const deliverAt = bind.indexOf("deliver(url, RegisterOrigin::Terminal)");
  assert.notEqual(disclosureAt, -1, "the disclosure line must exist");
  assert.notEqual(deliverAt, -1, "the delivery must exist");
  assert.ok(disclosureAt < deliverAt, "disclosure is printed before the write");
  // `--yes` never short-circuits the plan: `confirmed` only reaches the gate.
  assert.ok(
    !/if request\.confirmed \{\s*return Some\(RegisterCliPlan::Bind/.test(production),
    "--yes must not bypass the plan",
  );
  // And a refusal is never silent: it has a line and a non-zero code.
  assert.match(
    production,
    /RegisterCliPlan::Refused \{ .. \} => \{\s*for line in plan\.stdout_lines\(\) \{\s*println!\("\{line\}"\);\s*\}\s*Some\(1\)/,
    "a refusal prints and exits non-zero",
  );
});

// The frontend mirrors the two new wire fields, and a completed bind is
// rendered as the "already connected" sentence rather than a highlight of a row
// that no longer exists.
test("a completed terminal bind renders as already-connected, not as a highlight", async () => {
  const deepLink = stripJsComments(await read("src/deep-link.ts"));
  assert.match(deepLink, /confirmed\?: boolean;/, "the confirmation is on the wire");
  assert.match(deepLink, /bound\?: \{ id: string \} \| null;/, "so is the completed binding");
  assert.match(deepLink, /bindError\?: string \| null;/, "and the failure");
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(
    panel,
    /if \(request\.bound\) \{\s*setRegisterTarget\(null\);\s*setRegisterMiss\(\{ command: request\.command, alreadyConnected: true \}\);\s*return;\s*\}/,
    "a bound tool has no Detected row, so it gets the accurate sentence",
  );
  // A confirmed request refreshes the list: the row moved from Detected to
  // Connected, so the panel must re-read rather than render a stale list.
  assert.match(
    panel,
    /if \(pendingDeepLinkRegister\.confirmed\) void refreshAfterMutation\(\);/,
    "a terminal bind refreshes the list",
  );
  // Still no connect call from the panel's register path: the backend owns the
  // write, and the frontend only renders what came back.
  const parkAt = panel.indexOf("if (!registerPending || loading) return;");
  const block = panel.slice(parkAt, panel.indexOf("}, [registerPending, loading])", parkAt));
  for (const forbidden of ["extensions_connect_tool", "extensions_install", "approvedPermissions"]) {
    assert.ok(!block.includes(forbidden), `the register render must not reach ${forbidden}`);
  }
});

// The "not found" reason must survive the Detected section's own contract:
// that section renders *nothing* when nothing is detected, so a reason nested
// inside it would vanish in exactly the case it exists for. The notice is a
// sibling of the section, above it.
//
// Mutation: move the `{registerMiss && …}` block inside
// `{suggestedExtensions.length > 0 && …}` and this fails.
test("a register miss is rendered even when the Detected section is absent", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const gateAt = panel.indexOf("suggestedExtensions.length > 0 &&");
  assert.notEqual(gateAt, -1, "the Detected section keeps its gate");
  const missAt = panel.indexOf("{registerMiss && (");
  assert.notEqual(missAt, -1, "the miss notice must exist");
  assert.ok(missAt < gateAt, "the miss notice must sit outside (before) the section gate");
  assert.match(panel.slice(missAt, gateAt), /role="alert"/, "and must be announced");
  // Both sentences are real, bilingual dictionary entries with the same
  // placeholder, so the two outcomes cannot drift apart.
  for (const key of ["settings.extensions.registerNotFound", "settings.extensions.registerAlreadyConnected"] as const) {
    const en = createTranslator("en")(key);
    const zh = createTranslator("zh")(key);
    assert.ok(en.includes("{name}") && zh.includes("{name}"), `${key} must name the tool`);
    assert.ok(/[\u4e00-\u9fff]/.test(zh), `${key} must be translated`);
    assert.notEqual(zh, en, `${key} must not fall back to English`);
  }
  // The highlight mark is bilingual too.
  const markEn = createTranslator("en")("settings.extensions.registerHighlight");
  const markZh = createTranslator("zh")("settings.extensions.registerHighlight");
  assert.ok(markEn.length > 0 && /[\u4e00-\u9fff]/.test(markZh));
});

// ── C · refusals are one deduped toast, unknown actions are silent ────────

// The deduper is the *same* one the plugin pages use, and the window is the
// same 30s. A second constant would be a second policy.
test("a refusal reuses the app's 30s failure deduper", async () => {
  assert.equal(DEEP_LINK_REJECT_DEDUP_MS, FAILURE_NOTIFY_DEDUP_MS);
  assert.ok(DEEP_LINK_REJECT_DEDUP_MS >= 30_000, "the window is at least 30s");
  const deepLink = stripJsComments(await read("src/deep-link.ts"));
  assert.match(deepLink, /createFailureDeduper\(DEEP_LINK_REJECT_DEDUP_MS\)/, "the gate is the shared deduper");
  assert.ok(
    !/30_000|30000/.test(deepLink),
    "the window comes from the shared constant, not a second literal",
  );
});

// Drive the policy directly: a hostile page retrying one broken link must not
// be able to fill the toast stack.
test("a repeated refusal raises one toast inside the window", () => {
  const gate = createFailureDeduper(DEEP_LINK_REJECT_DEDUP_MS);
  const raised: number[] = [];
  for (let attempt = 0; attempt < 5; attempt += 1) {
    if (gate.allow(DEEP_LINK_REJECT_KEY, 1_000)) raised.push(attempt);
  }
  assert.deepEqual(raised, [0], "five attempts inside the window raise one toast");
  assert.equal(gate.allow(DEEP_LINK_REJECT_KEY, 1_000 + DEEP_LINK_REJECT_DEDUP_MS), true, "and the relapse is news");
});

// The backend sends a dictionary key, never a sentence; the host owns the
// words (the same rule the plugin-page bridge follows). An unknown key is
// dropped rather than painted raw.
test("the refusal key is a real, bilingual dictionary entry", async () => {
  assert.equal(DEEP_LINK_REJECT_KEY, "settings.deepLinkRejected");
  assert.ok(isMessageKey(DEEP_LINK_REJECT_KEY), "the key must exist in the dictionary");
  const en = createTranslator("en")(DEEP_LINK_REJECT_KEY);
  const zh = createTranslator("zh")(DEEP_LINK_REJECT_KEY);
  assert.ok(en.length > 0 && zh.length > 0);
  assert.notEqual(zh, en, "the refusal is translated, not an English fallback");
  assert.ok(/[\u4e00-\u9fff]/.test(zh), "the zh string is Chinese");
  // The user is told it did nothing — not why. The technical reason is a log
  // line: the person who clicked cannot act on "path traversal".
  assert.ok(!/traversal|scheme|https|path/i.test(en), "the toast carries no technical reason");
  // And the backend's copy of the key matches this one.
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  assert.match(rust, /REJECT_MESSAGE_KEY: &str = "settings\.deepLinkRejected"/, "the backend sends the same key");
  assert.match(
    rust,
    /pub fn is_silent\(&self\) -> bool \{\s*matches!\(self, Self::UnknownAction\(_\) \| Self::UnsupportedScheme\(_\)\)/,
    "only an unknown action and a foreign scheme stay silent",
  );
  assert.match(
    rust,
    /if reject\.is_silent\(\) \{\s*return;/,
    "a silent refusal raises no toast",
  );
  // Every other refusal is noisy, which is what makes `is_silent` meaningful.
  assert.match(rust, /focus_main\(app\);\s*notify_reject\(app\);/, "a noisy refusal focuses and toasts");
});

test("the app routes the refusal through the gate and the dictionary", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /listen<string>\(DEEP_LINK_REJECT_EVENT/, "the refusal event is wired");
  assert.match(app, /if \(!isMessageKey\(key\)\) return;/, "an unknown key is dropped, never painted");
  assert.match(app, /if \(!deepLinkRejectGate\.allow\(key\)\) return;/, "the gate is consulted");
  assert.match(app, /notify\("error", tRef\.current\(key\)\)/, "the toast rides the app's one stack");
});

// ── D · registration and the About row ────────────────────────────────────

// Major-1 (microfix round): the packaged `.desktop` has to carry `%u`, or the
// scheme is advertised while delivery silently fails. tauri-bundler's default
// template only appends it for a bare `Exec`, so the bundle points at a
// template that always appends it — and the template file has to exist and
// name the placeholder, or the build would regress to the broken artifact.
test("the Linux bundle uses a desktop template that delivers the URL", async () => {
  const config = JSON.parse(await read("src-tauri/tauri.conf.json")) as {
    bundle?: { linux?: { deb?: { desktopTemplate?: unknown }; rpm?: { desktopTemplate?: unknown } } };
  };
  const deb = config.bundle?.linux?.deb?.desktopTemplate;
  const rpm = config.bundle?.linux?.rpm?.desktopTemplate;
  assert.equal(deb, "floter.desktop.hbs", "the deb bundle names the template");
  assert.equal(rpm, deb, "the rpm bundle uses the same template, not a second one");
  const template = await read("src-tauri/floter.desktop.hbs");
  assert.match(template, /^Exec=\{\{exec\}\} %u$/m, "the template appends %u to the launcher");
  assert.match(template, /MimeType=\{\{mime_type\}\}/, "and keeps the scheme handler it advertises");
  assert.match(template, /\{\{#if mime_type\}\}/, "the MimeType line stays conditional");
});

// The scheme is registered in the config the bundler reads for the `.desktop`
// `MimeType` (Linux), `CFBundleURLTypes` (macOS) and the NSIS/MSI registry
// entries (Windows) — one place, three platforms.
test("the scheme is registered in the Tauri config the bundler consumes", async () => {
  const config = JSON.parse(await read("src-tauri/tauri.conf.json")) as {
    plugins?: { "deep-link"?: { desktop?: { schemes?: unknown } } };
  };
  assert.deepEqual(config.plugins?.["deep-link"]?.desktop?.schemes, ["floter"], "the scheme is `floter`");
  // The runtime registration and the macOS event path are both wired.
  const rust = stripJsComments(await read("src-tauri/src/deep_link.rs"));
  assert.match(rust, /app\.deep_link\(\)\.register_all\(\)/, "runtime registration is attempted");
  assert.match(rust, /app\.deep_link\(\)\.on_open_url/, "macOS delivers links through the event loop");
  const lib = stripJsComments(await read("src-tauri/src/lib.rs"));
  assert.match(lib, /deep_link::register_scheme\(app\.handle\(\)\)/, "setup registers the scheme");
  assert.match(lib, /deep_link::listen_for_url_events\(app\.handle\(\)\)/, "setup listens for OS deliveries");
  assert.match(
    lib,
    /deep_link::dispatch_url\(&handle, &url, deep_link::Delivery::Live\)/,
    "a forwarded second instance routes through the same parser as a live delivery",
  );
});

// A second instance forwards its arguments; on Linux a live instance is
// reached over the control socket, which is where the CLI and the scheme
// genuinely share one code path.
test("the Linux control socket forwards the URL to the one router", async () => {
  const ipc = stripJsComments(await read("src-tauri/src/ipc.rs"));
  assert.match(ipc, /pub fn send_deep_link\(url: &str\)/, "the socket has a link command");
  assert.match(ipc, /command\.strip_prefix\("link "\)/, "and the server recognizes it");
  assert.match(ipc, /crate::deep_link::dispatch_url\(\s*&handle,\s*&url,\s*crate::deep_link::Delivery::Live,?\s*\)/, "the server calls the one router as a live delivery");
  // R8-4 adds the terminal verb: the same router, plus the transport fact a
  // URL cannot carry. Both verbs land in `deep_link`; neither re-parses.
  assert.match(ipc, /pub fn send_terminal_deep_link\(url: &str\)/, "the terminal verb is a separate wire command");
  assert.match(ipc, /strip_prefix\("terminal-link "\)/, "and the server recognizes it");
  assert.match(ipc, /crate::deep_link::dispatch_terminal_url\(/, "which routes through the terminal entry point");
  // The encoding round-trips, so the router sees the URL the sender wrote.
  assert.match(ipc, /fn encode_link\(url: &str\) -> String/, "the wire encoding is explicit");
  assert.match(ipc, /fn decode_link\(encoded: &str\) -> String/, "and its inverse exists");
});

// The About row is the one place the scheme is discoverable, and the line it
// shows has to be a URL the router actually accepts — an example that fails
// validation would teach the wrong shape.
test("the About page shows a valid example and copies it", async () => {
  const row = stripJsComments(await read("src/settings/DeepLinkRow.tsx"));
  assert.match(row, /DEEP_LINK_EXAMPLE/, "the example comes from the shared module");
  assert.match(row, /DEEP_LINK_REGISTER_EXAMPLE/, "so does the R8-3 register example");
  assert.match(row, /navigator\.clipboard\.writeText\(value\)/, "each line copies its own URL");
  assert.match(row, /t\("settings\.deepLinkCopy"\)/, "the button is labelled");
  assert.match(row, /settings-deep-link__value/, "the value is rendered in its own shape");
  const about = stripJsComments(await read("src/settings/AboutPage.tsx"));
  assert.match(about, /<DeepLinkRow t=\{t\} onCopied=\{onCopiedLink\} \/>/, "the About page renders the row");
  // The first example is the connect form: its manifest value is a local
  // absolute `.json` path.
  assert.equal(DEEP_LINK_EXAMPLE, "floter://connect?manifest=/path/to/tool.json");
  assert.match(DEEP_LINK_EXAMPLE, /^floter:\/\/connect\?manifest=\//);
  assert.ok(DEEP_LINK_EXAMPLE.endsWith(".json"), "the example ends in .json, the one accepted suffix");
  // The second is the register form (R8-3): a bare command name, no path and
  // no arguments — the shape the router's `validate_command` accepts.
  assert.equal(DEEP_LINK_REGISTER_EXAMPLE, "floter://register?cmd=rg");
  assert.match(DEEP_LINK_REGISTER_EXAMPLE, /^floter:\/\/register\?cmd=[A-Za-z0-9._+-]+$/);
});

test("the deep-link copy is bilingual and stays free of platform jargon", () => {
  const keys = [
    "settings.deepLinkTitle",
    "settings.deepLinkHint",
    "settings.deepLinkCopy",
    "settings.deepLinkCopied",
    "settings.deepLinkRejected",
  ] as const;
  for (const key of keys) {
    const en = createTranslator("en")(key);
    const zh = createTranslator("zh")(key);
    assert.ok(en.length > 0, `${key} must have an en string`);
    assert.ok(zh.length > 0, `${key} must have a zh string`);
    assert.notEqual(zh, en, `${key} must be translated, not fall back to English`);
    assert.ok(/[\u4e00-\u9fff]/.test(zh), `${key} must contain Chinese text`);
  }
});

// The row adds no material: it reuses the section primitive and the control
// ladder. The value line is the content recess, the button the neutral raised
// control — and neither filters, so the same-screen budget is untouched.
test("the deep-link row adds no material and no filter", async () => {
  const css = stripComments(await read("src/styles/settings.css"));
  const rule = (selector: string) => {
    const at = css.indexOf(`\n${selector} {`);
    assert.notEqual(at, -1, `${selector} must exist`);
    const open = css.indexOf("{", at);
    const close = css.indexOf("}", open);
    return css.slice(open + 1, close);
  };
  // SETTINGS-APPLE: the copy affordance is the shared blue text action, which
  // spends the accent on *text* and never on a fill — so the row still adds no
  // material and no accent face, and the one button it paints is the group's
  // rather than its own.
  const button = rule(".settings-reset");
  assert.match(button, /background:\s*transparent/, "the action is a text button, not a filled control");
  assert.ok(!/box-shadow/.test(button), "a text action casts nothing");
  // The accent is allowed on the *text* — that is the idiom. What must not
  // happen is an accent *fill*, which is what the budget census counts.
  assert.ok(
    !/(?:^|;)\s*background(?:-color)?\s*:[^;]*var\(--accent/.
      test(button),
    "the action paints no accent fill",
  );
  assert.match(button, /color:\s*var\(--accent\)/, "the accent survives as text");
  assert.ok(!/filter/.test(button), "no filter on the action");
  const value = rule(".settings-deep-link__value");
  assert.match(value, /background:\s*var\(--surface-sunken\)/, "the value line is the content recess");
  assert.ok(!/filter/.test(value), "no filter on the value line");
  // No new accent fill anywhere in the two rules: the accent budget census
  // scans every host sheet and would go red on one.
  for (const body of [button, value]) {
    assert.ok(
      !/(?:^|;)\s*background(?:-color)?\s*:\s*var\(--(accent|accent-tint|glass-raised)/.test(body),
      "the row must not add an accent face",
    );
  }
  // The About row is now a card row, so its group really is the shared card
  // primitive — asserted here so a later round cannot give it a private one.
  const about = await read("src/settings/AboutPage.tsx");
  assert.match(about, /<SettingsCard/, "the update state uses the shared grouped card");
  assert.match(about, /<SettingsRow/, "the version and its update action are one row");
});
