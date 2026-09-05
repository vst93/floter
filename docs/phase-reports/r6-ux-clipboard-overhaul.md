# R6 — UX/Interaction Overhaul + Clipboard Sweep + Flaky Test Root-Cause Fix

Round: R6/R6b of 2026-09-05/06, dispatched to Codex (gpt-6-astra, effort max)
after the user switched the dev tool from Claude Code. Two dispatches died on
gateway overload (429/stream disconnects) before the third completed; the
coordinator independently verified everything below. Agent report corrections
by the coordinator: (1) its "Full Cargo: 363 passed, 5 failed" was an artifact
of its own workspace-write sandbox blocking socket binds (terminal broker
tests bind sockets; coordinator run: 368/0); (2) its "CSS delivery evidence
not claimed" — coordinator grepped dist/ directly (extension-discard-bar,
collapsed-card__input, clipboard-* all present in built CSS).

## Task 0 — flaky failing_version_probe_rejects_the_binary_set: ROOT CAUSE FIXED

First attempt (timeout 10s->30s) was disproven by the elapsed-time assertion:
real error was `Text file busy (os error 26)` at 102µs — a shared-fixture
ETXTBSY race (parallel test forked children inherit the fixture's write fd
until exec), not a timeout-budget problem. Final fix: the test no longer
writes an executable at test time at all — it uses a committed read-only
fixture (`src-tauri/tests/fixtures/failing-probe.sh`, CARGO_MANIFEST_DIR-based)
that is never written during the run. Coordinator proof: 15/15 consecutive
full-suite runs green (was ~1/10 flaky). Elapsed-time assertion kept.

## Task 1a — Dialogs unified to one pattern

- `RemovalDialog.tsx` (modal) DELETED; replaced by `RemovalConfirmation.tsx`
  inline confirm bar (role=alert, first button focused with preventScroll,
  Escape cancels with isComposing/keyCode 229 guard, focus returned to the
  prior element on unmount, busy-aware confirm). Wired through
  useExtensionActions / ExtensionRow / ExtensionsPanel.
- CustomIntegrationDrawer + LocalInstallDialog aligned with the same
  inline-confirmation language; no native dialogs anywhere (remnants were
  already zero; kept that way).
- i18n keys added for both en+zh.

## Task 1b — Button switching relationships

- New hooks `useImmediateState` (sync ref guard: double-click inside one
  event loop can no longer slip past React's async busy state) and
  `useTimedReset` (3s inline-confirm auto-reset). Applied across
  launcher/session/shortcut/settings hooks (useLauncherActions,
  useSessionManagement, useShortcutCapture, useSettings, useAppKeyboard).
- Settings toggles now reconcile with persisted state via the new
  settings-persistence hydration logic: optimistic flip reverts when the
  disk write fails, and edits made while a disk read is in flight are
  preserved (dedicated node test: "settings hydration preserves edits made
  while the disk read is in flight").

## Task 1c — Floating search box + floating terminal tool UI

- Launcher input: IME composition guard (Enter during Chinese composition
  does not commit; WebKit early isComposing-clear handled in
  useLauncherActions), focus ring/caret visibility, min-height floor kept.
- Results/scroll containment and pinned-card scrollbar hit-test adjustments
  (launcher.css / pinned-card.css), collapse animation untouched (dd67b69
  preserved).
- CSS delivery verified in dist/assets/*.css (main-Bz-n3HYi.css,
  clipboard-DlUClhuU.css).

## Task 2 — Clipboard plugin sweep

- Rust store/monitor: prune keeps favorites + image files ("clear keeps
  favorites and image files" regression test), failed persistence no longer
  publishes new in-memory state ("write failure does not publish new memory
  state" test), image write moved under the mutation lock, new `?` sites only
  on required persistence/restore/owner-lock failures (graceful-degradation
  gate respected; optional reads keep warn+fallback).
- Frontend main.ts: duplicate settings logic merged, same-loop double-click
  busy guard, IME guard on all key handlers, blob URL lifecycle
  (re-render leak) closed.
- Contracts preserved: favorites-permanent, files-by-reference-only,
  copy-to-system-clipboard restore, backend-owned panel size. Existing repo
  cap 300 non-favorites kept (spec's "200" reference was stale vs code).
- Node tests 76 -> 85 (+9: clipboard behavior + settings hydration +
  frozen-npm-ui + plugin-pages updates).

## Verification (coordinator, independent)

- cargo check: pass
- cargo test: 368 passed / 0 failed / 1 ignored, 15/15 consecutive green runs
- tsc --noEmit: pass
- npm run build: pass
- node tests: 85/85 pass
- dist grep: changed CSS rules present in built bundles
- Known non-blocking: native-only behaviors (external paste-key injection,
  PTY image paste, hotkey focus return on real WM, small-window dialog
  layout) need on-device testing; no sandboxed preview was possible (Vite
  EPERM in agent sandbox).
