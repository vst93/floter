# Phase 4 Slice 4: Operation Progress + Cancel

Baseline: main HEAD 8164917, clean. Implementation started by dispatched
Claude (session hung on the relay after ~50% completion and was terminated;
the coordinator completed, fixed and verified the remaining work directly).

## Premise Check (audit drift)

- The audit's slice-4 sketch assumed a package (tar) install path with a
  `paths.staging` directory. Neither exists on main: `InstallSource` has only
  `Linked` and `ExtensionPaths` has no staging field. Tests therefore exercise
  progress/cancel through the real `create_custom_integration` path.
- `install`/`uninstall` were synchronous-through commands holding the mutation
  lock; progress events are emitted from inside those flows at phase
  boundaries, cancel is checked at each async yield point.

## Implementation

New module `src-tauri/src/extensions/operation.rs`:

- `CancelToken` (Arc<AtomicBool>): minted by `start_operation`, flipped by
  `cancel_operation`, cleared by `end_operation`.
- `OperationProgress { extensionId, kind, phase, percent? }` payload.

`ExtensionState` (mod.rs):

- `app_handle: OnceLock<AppHandle>` — registered at setup in lib.rs; when
  present, `emit_progress` emits `extension-op-progress` on the Tauri event
  bus.
- `progress_listener: Mutex<Option<Box<dyn Fn>>>` — in-process listener for
  unit tests (no AppHandle in tests).
- `active_cancel` + `start_operation`/`end_operation`/`cancel_operation` —
  single-slot cancel registry; `check_cancelled` is called at every major
  await point in install (Loading manifest → Validating → Resolving
  executable → ...) and uninstall (Preparing → Creating backup → Staging
  removal → Updating registry → ...), returning `Err("Operation cancelled")`.
  The mutation lock + journal-first transaction guarantee that a cancelled
  operation leaves either rolled-back state or a journal recoverable at
  startup — no torn state.

Commands (commands/extensions.rs):

- `extensions_install` / `extensions_uninstall` now bracket the operation
  with `start_operation`/`end_operation`.
- New `extensions_cancel_operation(operationId)` command, registered in
  lib.rs invoke_handler.

Frontend:

- `ExtensionsPanel.tsx` subscribes to `extension-op-progress`, keeps a
  per-extension progress map, passes `progress` + `onCancelOperation` down to
  `ExtensionRow`; cancel invokes `extensions_cancel_operation` with the
  operation id and clears the row's progress state.
- `ExtensionRow.tsx` renders an inline progress label + cancel button while
  an operation is in flight (existing UI style, no new deps, no dialogs).
- i18n: new strings for progress/cancel.

## Not done (explicit)

- repair/reprobe cancellation: both are naturally idempotent and complete in
  seconds under the probe timeout budget; adding cancel would be ceremony.
  Progress events for them can be added later on the same mechanism.

## Tests

- `install_emits_progress_events`: listener records the install progress
  stream (Validating, Resolving executable, ...), ids correct.
- `install_respects_cancel_signal`: a pre-cancelled token never persists a
  broken lock entry.
- `uninstall_respects_cancel_signal`: cancel mid-uninstall returns
  `Err("Operation cancelled")` and the entry remains in the lock (recoverable,
  not torn).

## Verification pipeline

```
cd src-tauri && cargo check --tests   → 0 errors
cd src-tauri && cargo test            → 464 passed; 0 failed; 7 ignored
npx --legacy-peer-deps tsc --noEmit   → clean
npm run build                         → success
node --experimental-strip-types --test tests/*.test.ts → 85 pass / 0 fail
```

Coordinator fixes on top of the agent's partial work: command-layer missing
method impls (start/end/cancel_operation), Emitter trait import, AppHandle
injection at setup, tests rewritten off the non-existent tar/staging API,
tsc `require` → static import + event payload typing, cancel invoke key
mismatch (`extensionId` → `operationId`).

## Known limitation

Progress percent is an estimate at phase boundaries (no granular byte
progress); good enough for the row-level busy/cancel UX this slice ships.
