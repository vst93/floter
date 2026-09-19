// R7-8 · permission visual tiering, the approval record, and the boundary
// note's reach.
//
// The round is UI + i18n only: the audit fields already existed in the lock
// entry (`approvedPermissions` / `approvedAt` / `approvedManifestDigest`,
// Phase 2). What was missing was the *distinction* — `plugin-system-audit.md`
// §3.1 names it the trust-boundary contradiction, where every declared
// permission rendered as the same shape, so a user could not tell a
// host-enforced check from a declared-only one.
//
// These tests are the mutation locks for the three deliverables. Each one is
// written against the *positive* fact, so deleting the render (rather than the
// prose) is what turns it red:
//
//   A. the tier split is decided by one shared vocabulary (`permission-tiers`)
//      and both dialogs render two labelled groups with their own hint;
//   B. the drawer renders an approval record with the short digest and a
//      changed-manifest note that never introduces rollback language;
//   C. the boundary note reaches the recommended/manifest connect path, not
//      just custom/local.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  HOST_ENFORCED_PERMISSIONS,
  groupPermissions,
  permissionTier,
  wireTier,
} from "../src/extensions/permission-tiers.ts";
import { APPROVAL_DIGEST_PREFIX, approvalIsStale, shortDigest } from "../src/extensions/approval-record.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** One CSS rule body by exact selector, asserting the selector is unique. */
const cssRule = (css: string, selector: string) => {
  const head = `${selector} {`;
  // Anchor on a line start so a descendant rule (`X > .selector {`) is not
  // mistaken for the selector itself.
  const matches = [...css.matchAll(new RegExp(`(?:^|\\n)${selector.replace(/[.>+*()[\]]/g, "\\$&")} \\{`, "g"))];
  assert.equal(matches.length, 1, `selector must be unique: ${head} (found ${matches.length})`);
  const at = matches[0].index! + matches[0][0].indexOf(selector);
  const open = css.indexOf("{", at);
  const close = css.indexOf("}", open);
  return css.slice(open + 1, close);
};

// ── A1 · the tier vocabulary ──────────────────────────────────────────────

// The split is a product decision, and it lives in exactly one module so the
// review list, the local dialog and the custom editor cannot drift. The two
// enforced permissions are the ones the host itself decides: whether to hand
// over its environment, and whether to start another program on a descriptor's
// behalf.
test("host-enforced permissions are exactly environment and process-spawn", () => {
  assert.deepEqual([...HOST_ENFORCED_PERMISSIONS], ["environment", "process-spawn"]);
  for (const permission of HOST_ENFORCED_PERMISSIONS) {
    assert.equal(permissionTier(permission), "enforced", `${permission} is host-enforced`);
  }
  for (const permission of [
    "filesystem-read",
    "filesystem-write",
    "network-fetch",
    "clipboard-read",
    "clipboard-write",
  ]) {
    assert.equal(permissionTier(permission), "disclosure", `${permission} is disclosure-only`);
  }
});

// Over-claiming is the failure mode that would make the UI lie. A permission
// the host does not know about (a manifest from a newer version) must fall on
// the honest side: declared, not enforced.
test("an unknown permission is disclosure, never enforced", () => {
  assert.equal(permissionTier("telepathy"), "disclosure");
  const grouped = groupPermissions([
    { permission: "process-spawn" },
    { permission: "telepathy" },
    { permission: "environment" },
  ]);
  assert.deepEqual(grouped.enforced.map((p) => p.permission), ["process-spawn", "environment"]);
  assert.deepEqual(grouped.disclosure.map((p) => p.permission), ["telepathy"]);
});

// ── A1b · the Rust authority and the TS projection cannot drift ─────────────
//
// R7-8a · The classification is a product decision, and the backend now owns
// it (`permission_enforcement` in `src-tauri/src/extensions/manifest.rs`). The
// UI keeps a synchronous projection (`HOST_ENFORCED_PERMISSIONS`) rather than
// asking the Host per checkbox: the custom editor renders the declared list on
// every keystroke and an IPC round-trip per permission would be absurd for a
// static map. Two constants, then — and this test is the contract between
// them, the same shape `window-contract.test.ts` uses for the launcher width.
// Change the Rust classifier or its canonical name list alone and this is red;
// change the TS projection alone and this is red.
test("the Rust enforced set matches the TS projection", async () => {
  const rust = stripJsComments(await read("src-tauri/src/extensions/manifest.rs"));
  const declaration = /pub const HOST_ENFORCED_PERMISSION_NAMES\s*:\s*\[&str;\s*\d+\]\s*=\s*\[([^\]]*)\]/.exec(
    rust,
  );
  assert.ok(
    declaration,
    "src-tauri/src/extensions/manifest.rs no longer declares `HOST_ENFORCED_PERMISSION_NAMES` — " +
      "the Rust side is the classification source of truth and this test reads it as text",
  );
  const rustNames = [...declaration[1].matchAll(/"([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(
    [...rustNames].sort(),
    [...HOST_ENFORCED_PERMISSIONS].sort(),
    "the Rust enforced set and the TS HOST_ENFORCED_PERMISSIONS must name the same permissions",
  );
  // The classifier, not just the list, must agree. Read the arm that resolves
  // to `Enforced` and collect the `Permission::…` names it covers, so a
  // one-line edit to the match arm that leaves the canonical list untouched is
  // still caught. The enforced arm is the first `=> PermissionEnforcement::Enforced`
  // in the function; its patterns are the text between `match permission {` and it.
  const enforcedArm = /fn permission_enforcement[\s\S]*?match permission \{([\s\S]*?)=>\s*PermissionEnforcement::Enforced/.exec(
    rust,
  );
  assert.ok(enforcedArm, "the classifier must have an arm that resolves to Enforced");
  const classified = [...enforcedArm[1].matchAll(/Permission::([A-Za-z]+)/g)].map((match) => match[1]);
  const pascal = (kebab: string) =>
    kebab
      .split("-")
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join("");
  assert.deepEqual(
    [...classified].sort(),
    rustNames.map(pascal).sort(),
    "the classifer's Enforced arm must cover exactly the canonical enforced names",
  );
  for (const name of rustNames) {
    assert.equal(permissionTier(name), "enforced", `${name} must be enforced on the TS side`);
  }
});

// The Rust authority is only authoritative if the UI actually reads it. The
// review payload carries `enforcement` per permission (pinned behaviorally on
// the Rust side by `the_review_payload_carries_the_backend_enforcement`), and
// `groupPermissions` must prefer it over the local projection — otherwise a
// backend change to the classifier would not reach the screen. The `wireTier`
// normalizer is the one place the wire word (`disclosed`) meets the UI word
// (`disclosure`), and an unknown value under-claims.
test("the review payload's wire enforcement wins over the local projection", () => {
  // The backend says `enforced`; the local projection would have said
  // disclosure for a permission the local list does not know. The wire wins.
  const grouped = groupPermissions([
    { permission: "filesystem-read", enforcement: "enforced" },
    { permission: "environment", enforcement: "disclosed" },
  ]);
  assert.deepEqual(grouped.enforced.map((p) => p.permission), ["filesystem-read"]);
  assert.deepEqual(grouped.disclosure.map((p) => p.permission), ["environment"]);
  // A payload without the field falls back to the projection rather than
  // defaulting everything to disclosure.
  const fallback = groupPermissions([{ permission: "environment" }]);
  assert.deepEqual(fallback.enforced.map((p) => p.permission), ["environment"]);
  // The normalizer maps the wire word to the UI word, and under-claims on an
  // unknown value.
  assert.equal(wireTier("disclosed"), "disclosure");
  assert.equal(wireTier("enforced"), "enforced");
  assert.equal(wireTier("telepathy"), "disclosure");
});

// ── A2/A3 · the render ────────────────────────────────────────────────────

// The shared component is the only renderer of the two groups, and it gates
// each group on its own non-empty list so a declaration with only disclosure
// permissions does not render an empty "Floter enforces" heading.
test("the shared tier component renders both groups with their own hints", async () => {
  const component = stripJsComments(await read("src/extensions/PermissionTierList.tsx"));
  assert.match(component, /groupPermissions\(permissions\)/, "the split must come from the shared helper");
  assert.match(component, /tier="enforced"/, "the enforced group must be rendered");
  assert.match(component, /tier="disclosure"/, "the disclosure group must be rendered");
  assert.match(component, /enforced\.length > 0 &&/, "the enforced group must be gated on a non-empty list");
  assert.match(component, /disclosure\.length > 0 &&/, "the disclosure group must be gated on a non-empty list");
  assert.match(component, /extension-permission-tier__hint/, "each group carries its own boundary line");
});

// The review dialog (recommended + manifest connect) and the local-install
// dialog must both use the shared component — a second, hand-rolled split in
// either file is the drift this round exists to prevent.
test("both review dialogs render the shared tier list", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const local = stripJsComments(await read("src/extensions/LocalInstallDialog.tsx"));
  assert.ok(
    panel.includes("<PermissionTierList permissions={pendingPermissionReview.review.permissions}"),
    "the recommended/manifest review must render the tier list",
  );
  assert.ok(
    local.includes("<PermissionTierList permissions={pending.review.permissions}"),
    "the local install dialog must render the tier list",
  );
  // …and neither may keep the old flat map as well.
  for (const [name, source, needle] of [
    ["ExtensionsPanel", panel, "pendingPermissionReview.review.permissions.map"],
    ["LocalInstallDialog", local, "pending.review.permissions.map"],
  ] as const) {
    assert.ok(!source.includes(needle), `${name} must not also render a flat permission list`);
  }
});

// The two groups are visually distinct *shapes* (A3) without a new material:
// the enforced pane is the neutral raised fill with the accent keyline, the
// disclosure pane is the plain neutral item. Counted by the accent-budget
// census as a keyline, not a fill.
test("the enforced group is an accent-keyline pane and the disclosure group is neutral", async () => {
  const css = stripComments(await read("src/styles/extensions.css"));
  const enforced = cssRule(css, ".extension-permission-tier--enforced .extension-permission-item");
  assert.match(enforced, /background:\s*var\(--glass-raised-quiet\)/, "enforced uses the neutral raised pane");
  assert.match(enforced, /inset 0 0 0 1px var\(--accent-edge\)/, "enforced carries the accent keyline");
  assert.match(enforced, /var\(--elev-0\)/, "the accent is a keyline on a tokenized resting shadow");
  const disclosure = cssRule(css, ".extension-permission-tier--disclosure .extension-permission-item");
  assert.match(disclosure, /background:\s*var\(--icon-surface\)/, "disclosure stays the plain neutral pane");
  assert.ok(!/accent/.test(disclosure), "the disclosure pane must not carry an accent keyline");
  // No second material: no filter, no blur anywhere in the tier rules.
  for (const selector of [".extension-permission-tier", ".extension-permission-tier--enforced .extension-permission-item"]) {
    assert.ok(!/backdrop-filter|filter/.test(cssRule(css, selector)), `${selector} must not filter`);
  }
});

// The hint text has to be honest in both languages. The red line from
// `plugin-system-audit.md` §3.1: never imply a sandbox (Phase 6 is not
// implemented), so the copy talks about *Floter refusing a request* and the
// tool running with the user's own rights.
test("the tier hints are bilingual and never promise a sandbox", () => {
  const keys = [
    "settings.extensions.permissionTierEnforced",
    "settings.extensions.permissionTierEnforcedHint",
    "settings.extensions.permissionTierDisclosure",
    "settings.extensions.permissionTierDisclosureHint",
  ] as const;
  for (const key of keys) {
    const en = createTranslator("en")(key);
    const zh = createTranslator("zh")(key);
    assert.ok(en.length > 0, `${key} must have an en string`);
    assert.ok(zh.length > 0, `${key} must have a zh string`);
    assert.notEqual(zh, en, `${key} must be translated, not fall back to English`);
    assert.ok(/[\u4e00-\u9fff]/.test(zh), `${key} must contain Chinese text`);
    // Neither language may claim isolation the product does not have.
    assert.ok(!/sandbox/i.test(en), `${key} must not claim a sandbox`);
    assert.ok(!/沙箱/.test(zh), `${key} must not claim a sandbox`);
  }
  // The enforced hint promises a refusal; the disclosure hint says the tool
  // still runs as the user.
  assert.match(createTranslator("en")("settings.extensions.permissionTierEnforcedHint"), /refus/i);
  assert.match(createTranslator("en")("settings.extensions.permissionTierDisclosureHint"), /does not intercept/i);
});

// Every permission name has a label in both languages, so an approved-set chip
// in the record can never render a raw identifier in one language and a label
// in the other.
test("every permission name has an en and zh label", () => {
  for (const permission of [
    "filesystem-read",
    "filesystem-write",
    "network-fetch",
    "process-spawn",
    "clipboard-read",
    "clipboard-write",
    "environment",
  ]) {
    const key = `settings.extensions.permission.${permission}` as const;
    const en = createTranslator("en")(key);
    const zh = createTranslator("zh")(key);
    assert.ok(en && zh, `${permission} must be labelled in both languages`);
  }
});

// ── B · the approval record ───────────────────────────────────────────────

test("the digest is shown short, and an absent digest stays absent", () => {
  // 12 hex characters after the `sha256-` prefix (git's short-hash width).
  assert.equal(shortDigest("sha256-0123456789abcdef"), "sha256-0123456789ab");
  assert.equal(shortDigest("sha256-0123456789abcdef")!.length, 19);
  assert.equal(shortDigest("sha256-0123456789abcdef")!.length, APPROVAL_DIGEST_PREFIX);
  // Two digests that differ in the 12th hex position must not truncate to the
  // same string (the review's Minor 3 regressed at 8 hex).
  assert.notEqual(shortDigest("sha256-aaaaaaaaaaa1ffff"), shortDigest("sha256-aaaaaaaaaaa9ffff"));
  assert.equal(shortDigest("abc"), "abc", "a digest shorter than the prefix renders whole");
  assert.equal(shortDigest(null), null);
  assert.equal(shortDigest("   "), null);
});

// "Changed" is only ever reported when both sides are known. Old data (no
// recorded digest) and an unreadable manifest are *unknown*, not stale — a
// false flag would send the user to re-approve nothing.
test("a changed manifest is reported only when both digests are known", () => {
  assert.equal(approvalIsStale("sha256-a", "sha256-b"), true);
  assert.equal(approvalIsStale("sha256-a", "sha256-a"), false);
  assert.equal(approvalIsStale(null, "sha256-b"), false, "old data has no recorded digest");
  assert.equal(approvalIsStale("sha256-a", null), false, "an unreadable manifest is unknown");
  assert.equal(approvalIsStale(undefined, undefined), false);
});

// The drawer renders the record from the live fields, with the graceful
// degradation the round promises: no `approvedAt` → a "no record" line; a
// missing digest → the not-recorded label; a missing permission list → the
// none-recorded label.
test("the drawer renders the approval record with graceful degradation", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf('t("settings.extensions.approvalRecord")');
  assert.notEqual(at, -1, "the drawer must render an approval-record heading");
  const block = panel.slice(at, panel.indexOf("</section>", at));
  assert.match(block, /selected\.approvedAt \?/, "the record body is gated on approvedAt");
  assert.match(block, /selected\.approvedPermissions\?\.length/, "the approved set is rendered when present");
  assert.match(block, /approvalRecordPermissionsNone/, "a missing permission list degrades to a label");
  assert.match(block, /shortDigest\(selected\.approvedManifestDigest\)/, "the digest is rendered short");
  assert.match(block, /approvalRecordDigestUnknown/, "a missing digest degrades to a label");
  // R7-8b · the record shows the on-disk digest next to the recorded one, not
  // just a boolean, so "what I approved" and "what is here now" are both
  // readable. It degrades to the same unknown label, never a false match.
  assert.match(block, /approvalRecordDigestCurrent/, "the on-disk (current) digest row is rendered");
  assert.match(block, /shortDigest\(selected\.currentManifestDigest\)/, "the on-disk digest is rendered short");
  // The short hash is a display truncation; the full value rides in `title` so
  // a user can verify it against the lock entry without a wider row.
  assert.match(block, /title=\{selected\.approvedManifestDigest/, "the recorded digest title carries the full value");
  assert.match(block, /title=\{selected\.currentManifestDigest/, "the on-disk digest title carries the full value");
  assert.match(
    block,
    /approvalIsStale\(selected\.approvedManifestDigest, selected\.currentManifestDigest\)/,
    "the changed-manifest note compares the recorded and on-disk digests",
  );
  assert.match(block, /approvalRecordChanged/, "the changed-manifest note is rendered");
  assert.match(block, /approvalRecordNone/, "a missing approvedAt degrades to a no-record line");
  // No rollback/downgrade semantics: the note is a prompt to review again.
  assert.ok(
    !/rollback|downgrade|revert/i.test(block),
    "the approval record must not introduce rollback or downgrade language",
  );
  const changed = createTranslator("en")("settings.extensions.approvalRecordChanged");
  assert.ok(!/rollback|downgrade|revert/i.test(changed), "the changed note must not promise a rollback");
});

// R7-8b · the comparison row and the changed-manifest note are only useful if
// both languages name them. The `zh: Record<MessageKey, string>` type catches
// a *missing* key at compile time; this pins that neither side is empty and
// that the copy actually tells the user what to do next (re-approve), rather
// than only reporting a diff.
test("the current-digest row and the changed note are bilingual and act on the change", () => {
  for (const language of ["en", "zh"] as const) {
    const t = createTranslator(language);
    assert.ok(t("settings.extensions.approvalRecordDigestCurrent").trim(), `${language}: current digest label`);
    assert.ok(t("settings.extensions.approvalRecordDigest").trim(), `${language}: recorded digest label`);
    // The two rows must be named differently, or the comparison reads as one
    // value printed twice.
    assert.notEqual(
      t("settings.extensions.approvalRecordDigest"),
      t("settings.extensions.approvalRecordDigestCurrent"),
      `${language}: the recorded and current digests need distinct labels`,
    );
    assert.ok(t("settings.extensions.approvalRecordChanged").trim(), `${language}: changed note`);
  }
  // "建议重新审批" / "re-approve" is the action the note promises.
  assert.match(createTranslator("zh")("settings.extensions.approvalRecordChanged"), /重新审批/);
  assert.match(createTranslator("en")("settings.extensions.approvalRecordChanged"), /re-approve/i);
});

// The backend has to hand the panel the on-disk digest, or the "changed" note
// can never fire. It is compared to `approvedManifestDigest`, which is a
// `sha256-…` string, so the field must be produced by the digest loader.
test("the list item exposes the on-disk manifest digest", async () => {
  const commands = stripJsComments(await read("src-tauri/src/commands/extensions.rs"));
  assert.match(
    commands,
    /pub current_manifest_digest: Option<String>/,
    "ExtensionListItem must carry current_manifest_digest",
  );
  assert.match(
    commands,
    /ExtensionManifest::load_with_digest\(Path::new\(&entry\.manifest_path\)\)/,
    "the on-disk digest must come from the manifest digest loader",
  );
  assert.match(
    commands,
    /current_manifest_digest =\s*\n?\s*loaded_manifest/,
    "the loaded digest must be threaded into the list item",
  );
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /currentManifestDigest\?: string \| null;/, "Extension must declare currentManifestDigest");
});

// Two keys were orphaned by this round and must stay gone from **both**
// dictionaries, or they silently drift back as dead weight: `approvedAt`
// (the peer "Permissions approved" row was replaced by the approval record)
// and `infoSecondary` (never referenced).
// The bidirectional completeness check is the `zh: Record<MessageKey, string>`
// type — `tsc` fails on a missing key — so this only pins the removals.
test("the orphaned approval keys are gone from both dictionaries", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of ["settings.extensions.approvedAt", "settings.extensions.infoSecondary"]) {
    assert.equal(
      i18n.split(`"${key}"`).length - 1,
      0,
      `${key} must be removed from every dictionary`,
    );
  }
});

// ── C · the boundary note reaches every review path ───────────────────────

// Before this round `permissionReviewBoundary` existed only in the custom and
// local paths; the recommended/manifest connect dialog had no boundary line at
// all, so the one flow a user meets first was the one that over-promised.
test("the permission boundary note is rendered on every review path", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const local = stripJsComments(await read("src/extensions/LocalInstallDialog.tsx"));
  const custom = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  assert.ok(
    panel.includes('className="extension-permission-dialog__boundary"') &&
      panel.includes('t("settings.extensions.permissionReviewBoundary")'),
    "the recommended/manifest connect dialog must render the boundary note",
  );
  assert.ok(
    local.includes('className="extension-permission-dialog__boundary"') &&
      local.includes('t("settings.extensions.permissionReviewBoundary")'),
    "the local install dialog must keep the boundary note",
  );
  assert.ok(
    custom.includes("extension-custom-permission-boundary") &&
      custom.includes('t("settings.extensions.permissionBoundary")'),
    "the custom editor must keep its boundary note",
  );
  // One note per dialog, not two: the note is a boundary statement, and a
  // duplicate would just be copy weight.
  for (const [name, source] of [["ExtensionsPanel", panel], ["LocalInstallDialog", local]] as const) {
    assert.equal(
      source.split("extension-permission-dialog__boundary").length - 1,
      1,
      `${name} renders exactly one boundary note`,
    );
  }
});

// Reuse, not a second component: the note's style is the existing one, and the
// review dialog's list gains no new filter (the same-screen budget only ever
// decreases).
test("the boundary note reuses its style and adds no filter", async () => {
  const css = stripComments(await read("src/styles/extensions.css"));
  const boundary = cssRule(css, ".extension-permission-dialog__boundary");
  assert.match(boundary, /var\(--ext-border\)/, "the note keeps its existing border");
  assert.match(boundary, /box-shadow:\s*var\(--elev-0\)/, "the note keeps its tokenized resting shadow");
  // The note is a review-surface primitive. Its margin must be pinned because
  // `.extension-permission-dialog p` outranks it at (0,1,1) and collapses the
  // top margin to 3px; its radius is pinned to the shared ladder so it stays
  // rounded even if the note is ever mounted outside the `.extensions-panel`
  // scope that defines `--ext-radius`.
  assert.match(
    boundary,
    /border-radius:\s*var\(--radius-sm\)/,
    "the boundary note must carry an explicit ladder radius, not the panel-local alias",
  );
  assert.match(boundary, /margin:\s*5px 0 8px/, "the boundary note must pin its own margin");
  assert.ok(!/filter/.test(boundary), "the boundary note must not filter");
  const dialog = cssRule(css, ".extension-permission-dialog");
  assert.ok(!/filter/.test(dialog), "the review dialog must not gain a filter");
});

// ── officialVerified · physically removed (R-FREEZE-1) ────────────────────

// `plugin-system-audit.md` §六 and `tool-binding-design.md`「冻结区」 list the
// official signature index as undisputed dead weight: it was already demoted to
// a secondary note in R7-8, and the flag is false for every local, recommended
// or PATH-discovered tool. R-FREEZE-1 deletes the index module, the sole
// command that consumed it, the UI note and both i18n keys. This test is the
// mutation lock: reviving the badge (or the key it renders) turns it red.
test("the official-index trust projection is gone, not demoted", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.ok(
    !panel.includes("officialVerified"),
    "no `officialVerified` projection may survive in the panel",
  );
  assert.ok(
    !panel.includes("extensions_refresh_official_status"),
    "the panel must not call the deleted official-status command",
  );
  assert.ok(
    !panel.includes("trustOfficial"),
    "the official note must be gone, not demoted",
  );
  assert.ok(
    panel.includes('"settings.extensions.signatureVerified"'),
    "the publisher signature stays the primary, checkable fact",
  );

  const i18n = await read("src/i18n.ts");
  assert.ok(
    !i18n.includes("settings.extensions.trustOfficial"),
    "the trustOfficial key must be removed from both dictionaries",
  );

  const command = await read("src-tauri/src/commands/extensions.rs");
  assert.ok(
    !command.includes("extensions_refresh_official_status"),
    "the backend command must be deleted with its module",
  );
  assert.ok(
    !command.includes("official_index"),
    "no backend reference to the deleted official-index module may remain",
  );
});

test("the official-index module and its signing script are physically gone", async () => {
  for (const path of [
    "src-tauri/src/extensions/official_index.rs",
    "scripts/sign-official-index.mjs",
    "extensions/official-index/index.json",
  ]) {
    await assert.rejects(
      read(path),
      /ENOENT/,
      `${path} must not exist after the freeze round`,
    );
  }
});
