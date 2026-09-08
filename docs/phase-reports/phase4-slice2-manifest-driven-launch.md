# Phase 4 Slice 2: Manifest-Driven Launch

Baseline: `main`, HEAD `f5976c9`, initially clean. All changes remain in the working tree; no commit or push.

## Step 0: Premise Check

These references are to the actual **baseline HEAD**, before this slice. The old audit references do not describe the current implementation.

| Actual file:line | Observed behavior |
| --- | --- |
| `src-tauri/src/commands/extensions.rs:1699`, `:1710`, `:1816` | `extensions_launch` reads the repository entry for identity/version/approved filesystem grants, creates session metadata, and returns a JSON plan. It does **not** spawn, resolve a program, or verify the runtime. Caller `argv` is returned unchanged. |
| `src-tauri/src/commands/extensions.rs:1718`, `:1721` | Missing, unreadable, corrupt, or schema-invalid manifests already enter an explicit `match Err` that logs WARN and selects defaults. The bare manifest-load `?` introduced by a02dab0 is **not present at HEAD**. This slice preserves that contract and replaces the sham tests at `:2185` and `:2269` with calls to the real resolver. |
| `src-tauri/src/extensions/manifest.rs:45`; `src-tauri/src/extensions/lifecycle.rs:27`, `:132`, `:134`, `:136`, `:138`, `:172` | Manifest lifecycle contains optional `launch`, with `cwdPolicy`, `terminal`, and `restorePolicy`. Terminal is a requirements object (required, color, unicode, mouse, bracketedPaste, synchronizedOutput, keyboardProtocol), not an OS terminal name. There is **no launch command/args declaration**. |
| `docs/extensions/schemas/floter-extension.schema.json:403`, `:407`, `:438`, `:446`, `:464`, `:511` | JSON schema declares launch, cwd, camelCase `maxDepth`, fixed `{policy,path}`, terminal, and restore. These are existing declarations, not wholly ignored features. |
| `src-tauri/src/commands/extensions.rs:1750`, `:1754`, `:1759`, `:1790`, `:1824` | Cwd, restore, and terminal requirements are already read into the returned plan. Invalid cwd parsing silently falls back through `.ok()`; successfully parsed cwd resolution can still fail. The terminal environment is always hardcoded to floter-256color/truecolor/floter, even for other declared colors. |
| `src-tauri/src/extensions/cwd_policy.rs:44`, `:57`; `src-tauri/src/commands/extensions.rs:1846` | Structured cwd deserialization disagrees with the schema: `maxDepth` is ignored in favor of Rust's `max_depth`, and internally tagged newtype `Fixed(PathBuf)` cannot read the schema's fixed object. |
| `src-tauri/src/extensions/session_restore.rs:127`, `:133`, `:135`, `:171`, `:194`; `src-tauri/src/commands/extensions.rs:1765`, `:1773` | Restore is part of the same plan-building path, not an independent relaunch/spawn implementation. `find_session(...)?` at **line 194** propagates optional session read/parse errors and can abort launch. Reattach reuses metadata; restart creates metadata from the current request; none always creates new metadata. Broker liveness integration remains explicitly simplified. |
| `src-tauri/src/commands/extensions.rs:1734`, `:1754`, `:1787` | Other baseline failures are tool-data directory creation, cwd resolution, and session persistence. The repository load/get at `:1710` is required state. Optional session loading is the remaining uncaught optional-data load within this command's path; manifest loading is already caught. |
| `src-tauri/src/extensions/catalog.rs:369`, `:495`, `:520`; `src-tauri/src/extensions/provider.rs:526`, `:532`, `:565`, `:571` | The active launcher catalog builds execution plans from provider command descriptors. Program comes from the repository runtime or a permitted bundled relative program; script prefixes and environment come from registry resolution; cwd/mode come from the execution descriptor. Catalog manifest/descriptor failures are handled while loading commands. It does not call `extensions_launch`. |
| `src/hooks/useLauncherActions.ts:141`; `src/hooks/useTerminalView.ts:241`; baseline `src-tauri/src/commands/terminal.rs:59`, `:64`, `:100`; `src-tauri/src/terminal/session.rs:304`, `:315`; `src-tauri/src/terminal/broker.rs:488`, `:502` | Full actual spawn path: launcher execution -> `term_spawn` -> protected plan consumption -> TerminalManager/TerminalSession -> broker New request -> qscreen structured command/PTY. Without execution it uses the default shell and an initial command; absent cwd defaults to home. No frontend caller of `extensions_launch` exists at baseline. |
| `src-tauri/src/extensions/registry.rs:13`, `:29`, `:48`; `src-tauri/src/extensions/manifest.rs:285`; `src-tauri/src/extensions/probe_runner.rs:72` | Slice 1's registry resolves identity, current platform overrides, provider environment, and script interpreter/prefix. `run_invocation_probe` uses that executable/prefix/environment and literal probe args, excluding the provider protocol prefix. |
| `src-tauri/src/extensions/install.rs:1297`, `:1304`, `:1311`, `:1323`; `src-tauri/src/commands/extensions.rs:1488`, `:1675`, `:1690`; `src-tauri/src/extensions/probe_executor.rs:13`, `:24` | Install verification/repair and reprobe use the unified probe executor. There is **no launch verification call today**. The strict manifest load in explicit verify/repair is required verification input, not an optional load reached by `extensions_launch`. |

## Implementation

- `src-tauri/src/commands/extensions.rs:1698` now delegates to `extensions::launch::resolve` (`src-tauri/src/extensions/launch.rs:22`), the common resolver for new, reattached, and restarted extension launch plans.
- `src-tauri/src/extensions/lifecycle.rs:132`, `:164` and schema `floter-extension.schema.json:407` add optional `launch.command = { program, args }`. Program defaults to `self`; args default to empty and precede caller argv. These are literal argv elements, with no shell parsing.
- `src-tauri/src/extensions/launch.rs:140` uses `registry::provider_invocation_with_manifest` and the existing `provider::execution_plan`. This preserves script interpreter prefixes, current platform resolution, configured provider environment, execution host handling, and relative bundled program restrictions. Provider protocol `argsPrefix` is deliberately excluded, just as for lifecycle probes. Repository-approved permissions govern launch execution.
- `src-tauri/src/extensions/cwd_policy.rs:75` reads fixed directory objects correctly; `maxDepth` now uses schema casing while accepting the previous Rust `max_depth` alias. Omitted cwd defaults to inheritActiveSession, including command-only and empty launch objects.
- Terminal requirements remain the existing response object. `src-tauri/src/extensions/launch.rs:188` maps color to the actual spawn environment: none -> dumb, 8 -> xterm, 256 -> floter-256color, truecolor -> floter-256color plus COLORTERM=truecolor. Other color modes explicitly clear COLORTERM. This implements requirements/environment, not terminal capability negotiation or a new external-terminal selector.
- Declared launches add an `execution` object compatible with the existing `term_spawn` input. It uses the existing protected, single-use plan token; the provider environment stays in the backend cache. The top-level legacy envelope retains caller argv and terminal environment. `src-tauri/src/commands/terminal.rs:26` extracts its existing token-consumption logic into a shared method used unchanged by `term_spawn` and the real spawn tests.
- Valid declared launches with lifecycle probes now verify through `probe_executor::execute_capability_probes` -> `run_invocation_probe` (`launch.rs:77`). Required failure blocks the plan; optional failure permits it. Existing report/state helpers persist the result only in the repository. No probes are invented, and undeclared/default launches retain the baseline's no-verification behavior.
- Session selection still runs inside the same launch resolver (`launch.rs:105`). Restart resolves the current manifest again before session selection; recorded caller args are not mistaken for already-prefixed runtime args. `session_restore.rs:194` now catches optional session read/parse errors, logs WARN, and creates new metadata.
- No frontend, Windows-specific, macOS-specific, broker transport, launch-count, or metrics code changed. Catalog command behavior is unchanged. This slice keeps `extensions_launch` as the existing planning API; it does not introduce a second direct-spawn IPC API or rewire the unrelated catalog UI. All new OS-dependent tests are inside `#[cfg(all(test, unix))]`.
- No cancel, operation-progress, error-code taxonomy, or capability-negotiation work was added; probe state/error handling reuses slice 1's existing helpers.

Example opt-in declaration:

```json
{
  "lifecycle": {
    "launch": {
      "command": { "program": "self", "args": ["ui"] },
      "cwdPolicy": "toolData",
      "restorePolicy": "restart",
      "terminal": { "color": "256", "bracketedPaste": true }
    }
  }
}
```

## V1 And Graceful-Degradation Contract

Here, v1 compatibility means the pre-slice launch behavior for any manifest without `lifecycle.launch`, independent of the manifest document's schema-version label.

- No declaration returns the same serialized plan fields/values as f5976c9: unchanged caller argv, package version, inheritActiveSession cwd fallback, Reattach session policy, original terminal defaults, and the exact original three environment entries. No additional execution field or probe invocation appears. The generated UUID is the only normalized value in the byte comparison.
- A missing/unreadable/corrupt/schema-invalid manifest logs WARN with extension id, manifest path, and load error, then follows those defaults. There is no bare manifest-load `?` in the launch resolver.
- An invalid launch declaration is treated as wholly undeclared: WARN plus defaults for command, cwd, restore, and terminal. This avoids combining a declared command with an invalid working directory or unsupported terminal requirement, and preserves the caller's pre-slice launch contract. Structural errors are caught at manifest load; validly parsed but unusable cwd/program/permission declarations are caught at `launch.rs:56`. Schema errors now include their JSON instance path (`manifest.rs:496`).
- Explicit required lifecycle probes are not optional-data loads: when a valid declared launch requests them and they fail, their existing verification error blocks launch. Falling back because a launch declaration is absent or invalid does not invent verification.
- Existing session metadata files are reused at their existing paths. There are no new state files or parallel extension state sources. Session write failures and repository failures remain real errors, as before.

### Every Launch-Path Load And Question Mark

| Current path | Why it cannot silently abort on optional data |
| --- | --- |
| `launch.rs:31`, manifest load | Explicit `match`; all read, JSON/schema, and semantic load failures WARN and use defaults. The inner `manifest.rs:232` read and `:265` parse APIs still return errors to this catch boundary. No optional manifest load is propagated. |
| `launch.rs:147`, `:150`, `:180`, declared cwd/registry/execution resolution | These `?` operators return only to `declared_execution`'s explicit catch at `:56`. Bad cwd, unsupported platform/identity, missing script/interpreter/runtime, and invalid command cause WARN/defaults, not launch failure. Registry `provider_invocation_with_manifest` does not reload the manifest. |
| `session_restore.rs:133`, `:135`, session file read/parse | The low-level `?` operators preserve diagnostics for callers. Launch's `SessionResolver::resolve` catches their result at `:194` and warns/creates new metadata. The `resolve(...)?` at `launch.rs:106` preserves the existing Result API but no longer propagates optional session-load errors. |
| `launch.rs:29`, `:30` | Repository load and entry lookup are required authoritative installation state, not optional metadata. Launch cannot safely manufacture an installed entry. |
| `launch.rs:44`, `:98` | Tool data directory creation and legacy cwd resolution are required execution prerequisites, preserving baseline errors. Neither loads optional manifest/session data. |
| `launch.rs:79`, `:80` | Required recording/save of a declared verification result in the existing repository. Errors must propagate to avoid reporting successful verification when its state was not committed. |
| `launch.rs:121`; `session_restore.rs:140` | Existing session persistence is a required write, not an optional read. Directory/temp/write/fsync/rename errors retain the baseline failure behavior. |
| `launch.rs:135`; `commands/terminal.rs:33` | Protected execution-plan storage/consumption is required for the execution contract. Cache poisoning, expiration, reuse, and prohibited argument overrides remain errors; no disk metadata is loaded. |

## Behavioral Tests

All 19 tests below are in `src-tauri/src/extensions/launch/tests.rs`, gated at the parent module by `#[cfg(all(test, unix))]`. The old three parser-only tests and two sham fallback tests were replaced, for a net increase of 14 tests.

Spawn tests use the immutable fake executable `src-tauri/tests/fixtures/launch-record.sh` (or an interpreter-read copy). They call the production launch resolver, production TerminalExecutionPlan consumer, and qscreen's actual structured-command PTY spawn API. The child writes NUL-delimited records of program, cwd, TERM, COLORTERM, TERM_PROGRAM, resolved provider value, and all arguments. No spawn mocks, global environment mutation, global tracing subscriber, serialized test runner, or socket substitution is used. Warning capture is scoped to the future via `WithSubscriber`.

| Test | Concrete assertion |
| --- | --- |
| declared_system_command_cwd_and_terminal_reach_real_pty_spawn | Exact child record includes repository runtime, declared args before literal caller args, toolData cwd, 256-color environment, and platform environment override. All terminal response fields asserted; repository bytes unchanged for no-probe launch; shell metacharacters stay literal. |
| script_launch_and_verification_share_registry_invocation | Launch and ordered health/optional probes succeed despite a deliberately nonexistent raw repository executable. Child script prefix and platform environment are recorded; no provider protocol flags appear; repository stores Healthy report and no health sidecar exists. |
| undeclared_launch_is_byte_identical_and_does_not_probe | Serialized f5976c9 golden plan bytes match, including absent execution/mouse fields; repeat launch reuses session id; configured probes do not run and no warning occurs. |
| undeclared_launch_preserves_cwd_fallback_chain | No active cwd uses tool data; a missing active cwd finds a fixture-local project root before tool data. |
| missing_manifest_warns_and_returns_real_v1_plan | Delete manifest on disk; actual resolver returns golden default plan and captured WARN names the extension/load failure. |
| corrupt_manifest_warns_and_returns_real_v1_plan | Write invalid JSON on disk; actual resolver returns golden default plan and captured WARN identifies JSON failure. |
| invalid_cwd_on_loaded_manifest_warns_and_discards_entire_declaration | Manifest successfully loads with a nonexistent fixed path; launch WARN names cwdPolicy and the entire output matches defaults. |
| unknown_terminal_and_malformed_commands_warn_and_use_defaults | Unknown color/terminal shape, traversal program, NUL arg, non-array args, and invalid restore policy each produce field-specific WARN and golden defaults. |
| unapproved_command_permission_on_loaded_manifest_falls_back | A loaded declaration of a helper cannot acquire process-spawn from manifest edits; repository grants control the decision and warning names the command/permission. |
| bundled_command_uses_declared_program_and_approved_grants | Approved bundled relative helper is the program actually executed; removing it causes a command-specific warning and v1 fallback. |
| protected_launch_plan_ignores_ipc_tampering_and_is_single_use | Tampered program/args/cwd in IPC do not alter the spawned child; consuming the token a second time fails. |
| session_restart_resolves_current_manifest_before_spawning_again | Second launch from existing session metadata observes edited command, cwd, and terminal in the real child record; new id/isRestart and stored unprefixed caller argv are asserted. |
| reattach_and_none_policies_share_the_launch_resolver | Reattach reuses id; none generates distinct ids; both still deliver the same declared command to the real spawn consumer. |
| corrupt_optional_session_warns_and_launches_with_manifest_settings | Corrupt existing session JSON; captured WARN plus new id and successful declared-command spawn. |
| declared_required_probe_failure_blocks_launch_and_is_recorded_in_repository | Exact declared probe record; existing verification error returned; no session or launch-child record created; Unhealthy stored in repository. |
| declared_optional_probe_failure_still_launches | Actual child still runs; Degraded report is in repository and extension remains enabled. |
| project_root_max_depth_and_fixed_directory_follow_schema | Actual child cwd proves maxDepth=1 versus 2 and schema-form fixed path inside tool data. |
| command_only_declaration_defaults_cwd_restore_and_terminal | Omitted fields use inherited cwd, Reattach, self runtime, and original truecolor environment; child args contain no extra prefixes. |
| cwd_only_declaration_uses_registry_runtime_and_caller_args | No command declaration inside launch uses script runtime and caller args; home cwd and 8-color environment reach the child. |

## Verification

Targeted final run: `cargo test extensions::launch::tests`: **19 passed, 0 failed, 0 ignored, 444 filtered out**. Two earlier fixture failures were corrected: ambient project-root discovery and an invalid local/bundled fixture combination.

All commands ran in the sandbox with default test concurrency. Rust commands ran in `src-tauri`; JavaScript commands ran at the repository root.

| Command | Exact result |
| --- | --- |
| `cargo check` | PASS, exit 0; 0 errors, 0 warnings. |
| `cargo test` | **451 passed, 5 failed, 7 ignored, 0 measured, 0 filtered out**; 463 library tests total; exit 101 solely from the known socket-related failures below. All **19/19** new launch tests and **9/9** existing slice 1 command lifecycle-probe tests passed within this run. |
| `cargo test -- --ignored crash_simulation` | PASS, exit 0: **6 passed, 0 failed, 0 ignored, 0 measured, 457 filtered out**. Main binary, extension-check binary, and doc-test targets each had 0 tests. |
| `npx --legacy-peer-deps tsc --noEmit` | PASS, exit 0; no diagnostics. |
| `npm run build` | PASS, exit 0; Vite transformed **1860 modules** and produced the production assets. |
| `node --experimental-strip-types --test tests/*.test.ts` | PASS, exit 0: this environment's Node **v24.13.0** reports **9 passed file-level results, 0 failed, 0 skipped**. This is the exact requested command, not a claim of 85 individual results. |
| `npm test` (additional count verification) | PASS, exit 0: repository's existing `node --experimental-strip-types --test --test-isolation=none tests/*.test.ts` reports **85 passed, 0 failed, 0 cancelled, 0 skipped, 0 todo**. No package script or test-isolation setting was changed. |
| `git diff --check` | PASS, exit 0. |

### Sandbox Caveats

The five existing failures were left visible and unchanged:

1. `ipc::tests::delivers_a_toggle_line`: `src/ipc.rs:248`, bind denied with EPERM (os error 1).
2. `ipc::tests::reclaims_a_socket_left_by_a_crash`: `src/ipc.rs:267`, first bind denied with EPERM.
3. `ipc::tests::refuses_a_socket_a_live_instance_owns`: `src/ipc.rs:279`, first bind denied with EPERM.
4. `terminal::broker::tests::broker_executes_command_and_reports_exit`: daemon bind at `src/terminal/broker.rs:975` denied with EPERM; test then reports daemon did not start at `:984`.
5. `terminal::broker::tests::detached_session_can_be_listed_and_reattached`: `src/terminal/broker.rs:1056`, shared test lock poisoned by the preceding broker setup failure.

No socket test was changed, ignored, filtered, masked, or rerun with `--test-threads=1`. The Rust pass floor (451 >= 442), original 7 ignored tests, and Node individual-test floor (85) are satisfied. New spawn tests exercise the actual qscreen PTY implementation directly, so they require no socket bind.

Logs: `/tmp/floter-phase4-slice2-*.log`.
