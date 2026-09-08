# Phase 4 Slice 3: No Orphan Children on Provider Timeout/Cancel

Baseline: main HEAD d01bf91, clean. Implementation by dispatched Codex
(session hit a relay 429 after implementation+tests were complete but before
its report was written — 309k tokens consumed; tree audited and pipeline
verified independently by the coordinator; this report was written by the
coordinator from that audit).

## Premise Check (spawn-site inventory at baseline)

| Spawn site | Baseline state |
| --- | --- |
| `probe_executor.rs` (lifecycle probes, slice 1) | tokio::process + tokio::time::timeout around wait; timeout branch only STOPPED WAITING — child kept running. |
| `capability_probe.rs` (capability scan) | Same pattern: `.ok()?` on a 2s timeout — child leaked on timeout. |
| `provider.rs` (protocol I/O) | `tokio::time::timeout(2s, command.output())` with `.ok()?` — no kill on timeout, no kill_on_drop. |
| `install.rs` verification | Routed through probe_executor (slice 1), inherited its leak. |
| Broker/PTY terminal spawns | Intentionally untouched (user-facing terminal sessions, out of scope per spec). |

Gap: on any probe/scan/protocol timeout, the direct child (and any
grandchildren — provider entry points are scripts that spawn children)
survived the operation. kill_on_drop was absent, so cancellation/drop of the
awaiting future also leaked the child.

## Implementation

New module `src-tauri/src/extensions/process_cleanup.rs`:

- `configure_command(&mut Command)`: puts the child in its own process group
  (`process_group(0)`, unix-only; documented no-op equivalent on Windows where
  kill_on_drop + direct kill cover the direct child) so script-interpreter
  entry points can be cleaned up as a group.
- `command_output(command, timeout)`: kill_on_drop(true) + explicit timeout
  branch that calls `kill_and_reap` before returning the timeout error; waits
  for stdout/stderr reader tasks after cleanup; distinguishes
  `TimedOut`/`Failed` in `CommandOutputError`.
- `ChildCleanup::kill_and_reap`: kill_group → kill child → wait (reap, no
  zombie) → kill_group again to close the spawn race; all kill errors ignored
  (already-exited is an expected race, never surfaces to the operation).

All five provider/extension spawn sites now route through these helpers
(probe_executor, capability_probe, provider describe/protocol, install
verification paths). Timeout VALUES and probe semantics unchanged from
slice 1/2. No new IPC/API surface, no new state files, broker/qscreen
untouched. Unix-gated bits are cfg(unix); no windows/macos-specific code
was edited.

## Behavioral Tests (cargo 461 passed / 0 failed / 7 ignored)

New tests spawn REAL processes and assert actual process death:

- `scan_timeout_kills_the_real_probe_process` — trap-ignoring fixture + tiny
  injected timeout: operation errors AND the child pid is gone (pid-file
  handshake + /proc polling with generous bounds).
- `timeout_kills_parent_and_grandchild_process_group` — fixture spawns a
  background sleep grandchild then ignores signals: after cleanup BOTH are
  gone (validates process_group(0) + killpg).
- `aborting_probe_future_kills_the_process_group` — tokio task abort
  mid-probe: child killed via kill_on_drop (group gone).
- `already_exited_probe_does_not_report_cleanup_error` — immediate-exit
  fixture: kill path produces no error/panic.
- Zombie/reap discipline asserted by the polling helpers (no fixture-named
  processes survive, no zombies accumulate).

## Verification (coordinator, out-of-sandbox)

| Command | Result |
| --- | --- |
| `cargo check` | PASS |
| `cargo test` | PASS: **461 passed / 0 failed / 7 ignored** (all 5 known sandbox socket EPERM failures PASS outside the sandbox) |
| `cargo test -- --ignored crash_simulation` | PASS: 6/6 |
| `npx --legacy-peer-deps tsc --noEmit` | PASS |
| `node --experimental-strip-types --test tests/*.test.ts` | PASS: 85/85 |
| `npm run build` | PASS |

Phase 4 acceptance criterion 3 (provider timeout/cancel leaves no orphan
children): met — cleanup is structural (process group + kill_on_drop +
explicit kill-and-reap), not best-effort.
