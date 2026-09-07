# Phase 4 Slice 1: One Lifecycle Probe Set

Baseline: `main`, HEAD `260a3f8`, clean working tree. No commit or push.

## Step 0: Premise Check

All references in this section describe the **baseline HEAD**, before edits.

| Actual file:line | Current behavior and audit drift |
| --- | --- |
| `src-tauri/src/commands/extensions.rs:1622`, `:1643`, `:1698`, `:1708` | Reprobe already executes real lifecycle probes through `probe_executor`; the audit's static-only premise is stale. It first checks the raw stored executable, synthesizes a manifest on load failure, and writes `data/<id>/health.json`. It never updates repository broken state. |
| `src-tauri/src/extensions/install.rs:120`, `:1456`, `:1511`, `:1532`, `:1547`, `:1575`, `:1648` | Production install is linked-only (system or script). It discovers version metadata with declared `runtime.versionArgs`, validates/describes the provider, then runs lifecycle probes best-effort against the raw executable. An unhealthy report still produces an enabled entry; the report is written outside the repository. |
| `src-tauri/src/commands/extensions.rs:1455`, `:1459`, `:1492`, `:1495`, `:1510`; `src-tauri/src/extensions/install.rs:1288` | Repair calls `verify_installed`: manifest identity, provider describe or static descriptor/runtime availability. No lifecycle probes run. Failures use `classify_verify_error` and `mark_broken`; system repair reconnects and clears broken without lifecycle verification. |
| `src-tauri/src/extensions/artifacts.rs:19`, `:27`, `:295` | Artifact verification checks required binaries, containment and declared binary version arguments. It has **no production callers** at this HEAD; managed NPM installation was removed earlier. Its injectable timeout and load-sensitive failing-version test remain relevant regression coverage, but are not the current install path. |
| `src-tauri/src/extensions/manifest.rs:45`, `:91`, `:137`; `src-tauri/src/extensions/lifecycle.rs:25`, `:82`, `:112` | Lifecycle declarations contain id, full argument list, required flag and timeout (100-30000 ms). Runtime and artifact version arguments are separate declarations. Lifecycle entries have no independent binary target field. |
| `src-tauri/src/extensions/probe_executor.rs:16`, `:23`, `:33`, `:78`; `src-tauri/src/extensions/probe_runner.rs:58` | Shared install/reprobe executor runs lifecycle entries in order with their timeouts; empty declarations invent required `--version` and optional `--help`. The runner uses only a raw path, without script prefixes or resolved provider environment. |
| `src-tauri/src/extensions/capability_probe.rs:98`, `:202`, `:261`; `src-tauri/src/extensions/provider.rs:271`, `:628` | Generic capability scanning is a separate facility. Provider describe can discover version metadata using stored runtime arguments. Lifecycle health reports currently leave capability summaries at defaults. |
| `src-tauri/src/extensions/registry.rs:8`, `:23`; `src-tauri/src/extensions/manifest.rs:277` | Registry resolution validates identity, resolves current platform overrides, and supplies interpreter/script prefixes for script runtimes. Reprobe bypasses that invocation resolution. |
| `src-tauri/src/extensions/lock.rs:142`, `:151`, `:159`, `:192`, `:278`, `:308`; `src-tauri/src/extensions/repository.rs:23`, `:117` | Repository stores last error code/detail/time, first broken reason and prior enabled intent, but no probe report. `save()` uses the repository writer. `mark_broken` disables execution and preserves intent; `clear_broken` restores it. |
| `src-tauri/src/commands/extensions.rs:1559`, `:1609`; `src-tauri/src/extensions/catalog.rs:444`; `src/ExtensionsPanel.tsx:739`, `:772`, `:1554` | Health UI reads the sidecar command; repair affordances read repository broken/error fields. Describe and catalog binding checks can clear broken state without probes, which must not erase a required-probe failure. Command signatures can stay unchanged. |

### Gaps

1. Install validates permission approval, compatibility, minimum OS, executable usability, script existence and provider data before probing. Reprobe does none of these validation steps, apart from checking raw path existence and attempting a manifest load; repair performs identity/provider/runtime checks but no lifecycle probes.
2. Install and reprobe share declared lifecycle order and timeouts today, but neither persists required failures as broken. Repair does persist verification failures, yet can report success without running any lifecycle checks.
3. Install discovers version metadata via declared runtime arguments; reprobe's invented fallback ignores those arguments. Metadata discovery is best-effort and separate from lifecycle verification. No fixed version/help arguments should be invented for old manifests.
4. Both lifecycle callers bypass resolved script prefixes and environment. Reprobe additionally accepts missing/corrupt manifests by synthesizing declarations, concealing manifest errors.
5. Reprobe refreshes the health sidecar; repair does not. The repository has error state but no report. Reports and error transitions must be saved together through the repository writer, without a new state file.
6. Binding/describe success can erase broken state even while the last required lifecycle probe failed. Reconnect also needs to verify the declared probes on its replacement target before clearing failure.

## Implementation And Verification

In progress. Final path references, behavioral test assertions and exact verification counts will be recorded after implementation.

## Completed By Coordinator (agent relay died mid-report)

The dispatching agent's relay connection died (account concurrency limit)
after implementation and tests were written but before this section could be
recorded. The uncommitted tree was audited and verified independently:

- Baseline on the interrupted tree: cargo test 442 passed / 0 failed /
  7 ignored (8 new tests above the 434 floor) — implementation was already
  green when the relay died.
- Full pipeline re-run: crash simulations 6/6 PASS; tsc --noEmit PASS;
  node tests 85/85 PASS; npm run build PASS.
- Diff audit: reprobe now routes through registry invocation resolution
  (provider_invocation_with_manifest) instead of raw paths; the old
  missing/corrupt-manifest fallback that synthesized --version/--help probes
  is REMOVED (unreadable or mismatched manifests are rejected, gap 4);
  required probe failures persist via mark_broken with the install error
  taxonomy through the repository writer (gap 2/5); binding/describe/reconnect
  success can no longer clear a failed required probe (gap 6); optional
  failures stay degraded-but-enabled on all three surfaces; empty lifecycle
  declarations remain a no-op reprobe (v1 fallback).

Phase 4 slice 1 acceptance (audit criterion 1): install, repair, and reprobe
execute the same ordered lifecycle probe set; marker-fixture tests assert
identical invocation order/count across surfaces.
