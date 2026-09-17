// R7-9 · the `floter://` scheme: one allow-list, two actions, and a hard rule
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
  for (const name of ["open", "connect"]) {
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
    /pub const ACTIONS: &\[&str\] = &\["open", "connect"\];/,
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
      !source.includes(forbidden),
      `deep_link.rs must not reach ${forbidden} — a link may only open the review dialog`,
    );
  }
  // The structural check is a parse, not an install.
  assert.match(source, /ExtensionManifest::load\(&path\)\?/, "the manifest is parsed");
  assert.match(source, /app\.emit\(\s*CONNECT_EVENT/, "and the request is emitted");
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
  assert.match(ipc, /crate::deep_link::dispatch_url\(&handle, &url, crate::deep_link::Delivery::Live\)/, "the server calls the one router as a live delivery");
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
  assert.match(row, /navigator\.clipboard\.writeText\(DEEP_LINK_EXAMPLE\)/, "the button copies it");
  assert.match(row, /t\("settings\.deepLinkCopy"\)/, "the button is labelled");
  assert.match(row, /settings-deep-link__value/, "the value is rendered in its own shape");
  const about = stripJsComments(await read("src/settings/AboutPage.tsx"));
  assert.match(about, /<DeepLinkRow t=\{t\} onCopied=\{onCopiedLink\} \/>/, "the About page renders the row");
  // The example is the connect form: the only action with a parameter worth
  // copying. Its manifest value is a local absolute `.json` path.
  assert.equal(DEEP_LINK_EXAMPLE, "floter://connect?manifest=/path/to/tool.json");
  assert.match(DEEP_LINK_EXAMPLE, /^floter:\/\/connect\?manifest=\//);
  assert.ok(DEEP_LINK_EXAMPLE.endsWith(".json"), "the example ends in .json, the one accepted suffix");
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
  const button = rule(".settings-copy-button");
  assert.match(button, /background:\s*var\(--glass-control\)/, "the button is the neutral control");
  assert.match(button, /box-shadow:\s*var\(--elev-0\)/, "on the resting rung");
  assert.ok(!/accent[^-]/.test(button.replace(/--accent-ring/g, "")), "the button paints no accent fill");
  assert.ok(!/filter/.test(button), "no filter on the button");
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
});
